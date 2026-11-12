package chart

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuild(t *testing.T) {
	t0 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	var times []time.Time
	var a, b []float64
	for i := 0; i < 48; i++ {
		times = append(times, t0.Add(time.Duration(i)*time.Hour))
		a = append(a, float64(1000000+i*10000))
		b = append(b, float64(50000+i*100))
	}
	c := Build(times, []Series{{"Bank", "lc-bank", a}, {"Cash", "lc-cash", b}}, Money, true)
	if len(c.Lines) != 2 || !strings.HasPrefix(c.Lines[0].Path, "M72.0 ") || strings.Count(c.Lines[0].Path, "L") != 47 {
		t.Fatalf("lines: %+v", c.Lines[0].Path[:40])
	}
	if c.YTicks[0].Label != "$0" || c.YTicks[0].Pos != c.Bottom {
		t.Errorf("from zero: first tick %+v", c.YTicks[0])
	}
	if len(c.XTicks) < 2 {
		t.Errorf("x ticks: %+v", c.XTicks)
	}
	var h struct {
		X []float64
		S [][]string
	}
	if err := json.Unmarshal([]byte(c.Hover), &h); err != nil || len(h.X) != 48 || h.S[1][0] != "$50k" {
		t.Errorf("hover: %v %+v", err, h.S)
	}
	// Percent chart crossing zero gets a zero line.
	p := Build(times[:3], []Series{{"Market", "lc-market", []float64{-5, 2, 7}}}, Percent, false)
	if p.ZeroY < p.Top || p.ZeroY > p.Bottom {
		t.Errorf("zero line %v", p.ZeroY)
	}
	if Build(times[:1], []Series{{"x", "", []float64{1}}}, Money, true).Lines != nil {
		t.Error("one point should draw nothing")
	}
}

func TestFormats(t *testing.T) {
	for v, want := range map[float64]string{0: "$0", 950: "$950", 1500: "$1.5k", 2000000: "$2M", 2450000: "$2.5M", -3000: "-$3k"} {
		if got := Money(v); got != want {
			t.Errorf("Money(%v) = %q, want %q", v, got, want)
		}
	}
	if Percent(4.5) != "+4.5%" || Percent(-12) != "-12%" || Percent(0) != "0%" {
		t.Error("Percent")
	}
	if niceStep(0.3) != 0.5 || niceStep(230000) != 250000 || niceStep(7) != 10 {
		t.Errorf("niceStep %v %v %v", niceStep(0.3), niceStep(230000), niceStep(7))
	}
}
