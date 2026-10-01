// Package logbuf keeps the website's recent log lines in memory for
// Admin → Development → Website logs: slog records and the HTTP request
// log, newest last, capped so it never grows.
package logbuf

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Entry is one log line.
type Entry struct {
	ID    int64     `json:"id"`
	Time  time.Time `json:"time"`
	Level string    `json:"level"` // debug / info / warn / error / http
	Msg   string    `json:"msg"`
	Attrs string    `json:"attrs"`
}

// Buffer is a fixed-size ring of entries.
type Buffer struct {
	mu      sync.Mutex
	entries []Entry
	next    int64
	max     int
	errors  int64
	warns   int64
}

// New keeps the last max entries.
func New(max int) *Buffer { return &Buffer{max: max} }

func (b *Buffer) add(e Entry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	e.ID = b.next
	switch e.Level {
	case "error":
		b.errors++
	case "warn":
		b.warns++
	}
	b.entries = append(b.entries, e)
	if len(b.entries) > b.max {
		b.entries = b.entries[len(b.entries)-b.max:]
	}
}

// Since returns entries with an ID above after (0 = all), oldest first.
func (b *Buffer) Since(after int64) []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []Entry
	for _, e := range b.entries {
		if e.ID > after {
			out = append(out, e)
		}
	}
	return out
}

// Counts returns how many errors and warnings were logged since start.
func (b *Buffer) Counts() (errors, warns int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.errors, b.warns
}

// Handler wraps an slog handler, copying every record into the buffer.
func (b *Buffer) Handler(next slog.Handler) slog.Handler { return &handler{buf: b, next: next} }

type handler struct {
	buf   *Buffer
	next  slog.Handler
	attrs []slog.Attr
	group string
}

func (h *handler) Enabled(ctx context.Context, l slog.Level) bool { return h.next.Enabled(ctx, l) }

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	var sb strings.Builder
	write := func(a slog.Attr) {
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		key := a.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		fmt.Fprintf(&sb, "%s=%v", key, a.Value.Any())
	}
	for _, a := range h.attrs {
		write(a)
	}
	r.Attrs(func(a slog.Attr) bool { write(a); return true })
	h.buf.add(Entry{Time: r.Time, Level: strings.ToLower(r.Level.String()), Msg: r.Message, Attrs: sb.String()})
	return h.next.Handle(ctx, r)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{buf: h.buf, next: h.next.WithAttrs(attrs), attrs: append(append([]slog.Attr{}, h.attrs...), attrs...), group: h.group}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{buf: h.buf, next: h.next.WithGroup(name), attrs: h.attrs, group: name}
}

// RequestWriter is an io.Writer for the HTTP request logger: each line
// becomes an "http" entry (lines for 5xx responses count as errors).
func (b *Buffer) RequestWriter() *requestWriter { return &requestWriter{buf: b} }

type requestWriter struct {
	buf     *Buffer
	pending bytes.Buffer
	mu      sync.Mutex
}

func (w *requestWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending.Write(p)
	for {
		line, err := w.pending.ReadString('\n')
		if err != nil {
			w.pending.Reset()
			w.pending.WriteString(line)
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// The standard logger prefixes "2006/01/02 15:04:05 ".
		if len(line) > 20 && line[4] == '/' && line[19] == ' ' {
			line = line[20:]
		}
		level := "http"
		if strings.Contains(line, " - 5") {
			level = "error"
		}
		w.buf.add(Entry{Time: time.Now(), Level: level, Msg: line})
	}
	return len(p), nil
}
