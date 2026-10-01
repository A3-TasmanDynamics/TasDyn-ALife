package logbuf

import (
	"io"
	"log/slog"
	"testing"
)

func TestBuffer(t *testing.T) {
	b := New(3)
	l := slog.New(b.Handler(slog.NewTextHandler(io.Discard, nil)))
	l.Info("one", "k", 1)
	l.With("player", 7).Warn("two")
	l.Error("three")
	l.Info("four")
	got := b.Since(0)
	if len(got) != 3 || got[0].Msg != "two" || got[0].Attrs != "player=7" || got[2].Msg != "four" {
		t.Fatalf("ring: %+v", got)
	}
	if e, w := b.Counts(); e != 1 || w != 1 {
		t.Errorf("counts %d errors %d warns", e, w)
	}
	if after := b.Since(got[1].ID); len(after) != 1 || after[0].Msg != "four" {
		t.Errorf("since: %+v", after)
	}
	w := b.RequestWriter()
	w.Write([]byte("2026/10/01 22:00:00 \"GET http://x/ HTTP/1.1\" from [::1]:1 - 200 5B in 1ms\n\"GET /boom\" - 500 0B\n"))
	last := b.Since(0)
	if last[len(last)-2].Level != "http" || last[len(last)-1].Level != "error" || last[len(last)-2].Msg[0] != '"' {
		t.Errorf("request lines: %+v", last)
	}
}
