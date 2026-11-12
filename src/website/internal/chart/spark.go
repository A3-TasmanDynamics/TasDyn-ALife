package chart

import (
	"fmt"
	"math"
	"strings"
)

// Spark is a tiny trend line for a stat tile: no axes, a thin line, a dot
// on the latest value. The tile's own number and label say what it is.
type Spark struct {
	W, H         int
	Path         string
	Area         string // the same line closed down to the baseline, for a faint fill
	LastX, LastY float64
	Label        string // accessible summary, e.g. "from 3 to 8, peak 9"
}

// BuildSpark lays values (oldest first) into a w x h box. Nothing to draw
// (fewer than 2 values) gives a zero Spark (Path "").
func BuildSpark(values []float64, w, h int, fmtV func(float64) string) Spark {
	s := Spark{W: w, H: h}
	if len(values) < 2 {
		return s
	}
	lo, hi := values[0], values[0]
	for _, v := range values {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	if lo > 0 && hi > 0 && (hi-lo) > hi*0.5 {
		lo = 0 // a big swing reads truer from zero
	}
	if hi == lo {
		hi = lo + 1
	}
	pad := 2.0 // room for the dot and the 2px line
	x := func(i int) float64 { return pad + float64(i)/float64(len(values)-1)*(float64(w)-2*pad) }
	y := func(v float64) float64 { return pad + (1-(v-lo)/(hi-lo))*(float64(h)-2*pad) }
	var b strings.Builder
	for i, v := range values {
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		fmt.Fprintf(&b, "%s%.1f %.1f", cmd, x(i), y(v))
	}
	s.Path = b.String()
	s.Area = fmt.Sprintf("%sL%.1f %.1fL%.1f %.1fZ", s.Path, x(len(values)-1), float64(h), x(0), float64(h))
	s.LastX, s.LastY = round1(x(len(values)-1)), round1(y(values[len(values)-1]))
	peak := values[0]
	for _, v := range values {
		peak = math.Max(peak, v)
	}
	s.Label = fmt.Sprintf("from %s to %s, peak %s", fmtV(values[0]), fmtV(values[len(values)-1]), fmtV(peak))
	return s
}
