// Package chart draws small server-side SVG line charts for the admin
// panel: one y-axis, thin 2px lines, a recessive grid, direct labels at the
// line ends, and the data for a hover tooltip (web/static/chart.js).
package chart

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// Series is one line: its name (legend, tooltip, direct label) and a CSS
// class that colours it.
type Series struct {
	Name   string
	Class  string
	Values []float64
}

type Tick struct {
	Pos   float64
	Label string
}

type Line struct {
	Name, Class string
	Path        string
	EndX, EndY  float64 // where its direct label goes
	EndLabel    string
}

// Chart is ready for the template (partial_line_chart.html).
type Chart struct {
	W, H                     int
	Left, Right, Top, Bottom float64 // the plot area
	LabelX                   float64 // where the y-axis labels end
	Lines                    []Line
	YTicks, XTicks           []Tick
	ZeroY                    float64 // the y of 0 when the range crosses it, else -1
	Hover                    string  // JSON for chart.js: {"x":[...], "t":[...], "s":[[...]...]}
	Points                   int
}

// Build lays out a chart of the series over times (same length as each
// series' values). fmtY formats an axis / tooltip value. fromZero keeps 0
// on the axis (amounts); otherwise the range hugs the data with 0 shown
// if it's crossed (percentages).
func Build(times []time.Time, series []Series, fmtY func(float64) string, fromZero bool) Chart {
	c := Chart{W: 720, H: 240, Left: 72, Right: 720 - 92, Top: 12, Bottom: 240 - 26, ZeroY: -1, Points: len(times)}
	c.LabelX = c.Left - 8
	if len(times) < 2 {
		return c
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, s := range series {
		for _, v := range s.Values {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}
	if fromZero {
		lo = math.Min(lo, 0)
	} else if lo > 0 && hi > 0 {
		lo = math.Min(lo, 0)
	}
	if hi == lo {
		hi = lo + 1
	}
	step := niceStep((hi - lo) / 4)
	lo = math.Floor(lo/step) * step
	hi = math.Ceil(hi/step) * step
	y := func(v float64) float64 { return c.Bottom - (v-lo)/(hi-lo)*(c.Bottom-c.Top) }
	t0, t1 := times[0], times[len(times)-1]
	span := t1.Sub(t0).Seconds()
	if span <= 0 {
		span = 1
	}
	x := func(t time.Time) float64 { return c.Left + t.Sub(t0).Seconds()/span*(c.Right-c.Left) }

	for v := lo; v <= hi+step/2; v += step {
		c.YTicks = append(c.YTicks, Tick{round1(y(v)), fmtY(v)})
	}
	if lo < 0 && hi > 0 {
		c.ZeroY = round1(y(0))
	}
	// About five time ticks, on a round number of hours or days.
	var every time.Duration
	for _, h := range []int{1, 2, 3, 6, 12, 24, 48, 72, 168} {
		every = time.Duration(h) * time.Hour
		if t1.Sub(t0)/every <= 6 {
			break
		}
	}
	layout := "2 Jan"
	switch {
	case t1.Sub(t0) <= 36*time.Hour:
		layout = "15:04"
	case every < 24*time.Hour:
		layout = "2 Jan 15:04"
	}
	// Ticks on local time: midnight, then every `every` (00:00, 06:00, ...).
	first := time.Date(t0.Year(), t0.Month(), t0.Day(), 0, 0, 0, 0, t0.Location())
	for !first.After(t0) {
		first = first.Add(every)
	}
	for t := first; t.Before(t1); t = t.Add(every) {
		if x(t)-c.Left >= 20 { // not on top of the y labels
			c.XTicks = append(c.XTicks, Tick{round1(x(t)), t.Format(layout)})
		}
	}

	hover := struct {
		X []float64  `json:"x"`
		T []string   `json:"t"`
		N []string   `json:"n"`
		S [][]string `json:"s"`
	}{}
	for _, t := range times {
		hover.X = append(hover.X, round1(x(t)))
		hover.T = append(hover.T, t.Format("2 Jan 15:04"))
	}
	for _, s := range series {
		var b strings.Builder
		vals := make([]string, len(s.Values))
		for i, v := range s.Values {
			cmd := "L"
			if i == 0 {
				cmd = "M"
			}
			fmt.Fprintf(&b, "%s%.1f %.1f", cmd, x(times[i]), y(v))
			vals[i] = fmtY(v)
		}
		last := s.Values[len(s.Values)-1]
		c.Lines = append(c.Lines, Line{Name: s.Name, Class: s.Class, Path: b.String(),
			EndX: round1(c.Right + 8), EndY: round1(y(last)), EndLabel: s.Name})
		hover.N = append(hover.N, s.Name)
		hover.S = append(hover.S, vals)
	}
	// Keep direct labels from overlapping: at least 14px apart.
	for i := 1; i < len(c.Lines); i++ {
		for j := 0; j < i; j++ {
			if d := c.Lines[i].EndY - c.Lines[j].EndY; math.Abs(d) < 14 {
				if d >= 0 {
					c.Lines[i].EndY = c.Lines[j].EndY + 14
				} else {
					c.Lines[i].EndY = c.Lines[j].EndY - 14
				}
			}
		}
	}
	j, _ := json.Marshal(hover)
	c.Hover = string(j)
	return c
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// niceStep rounds a step up to 1, 2, 2.5 or 5 times a power of ten.
func niceStep(raw float64) float64 {
	if raw <= 0 {
		return 1
	}
	p := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if raw <= m*p {
			return m * p
		}
	}
	return 10 * p
}

// Money formats dollars compactly for an axis: $1.2M, $350k, $900.
func Money(v float64) string {
	a := math.Abs(v)
	sign := ""
	if v < 0 {
		sign = "-"
	}
	switch {
	case a >= 1e9:
		return fmt.Sprintf("%s$%sB", sign, trim(a/1e9))
	case a >= 1e6:
		return fmt.Sprintf("%s$%sM", sign, trim(a/1e6))
	case a >= 1e3:
		return fmt.Sprintf("%s$%sk", sign, trim(a/1e3))
	}
	return fmt.Sprintf("%s$%.0f", sign, a)
}

// Percent formats a signed percentage: +4.5%, -12%, 0%.
func Percent(v float64) string {
	if v > 0 {
		return "+" + trim(v) + "%"
	}
	return trim(v) + "%"
}

func trim(v float64) string {
	s := fmt.Sprintf("%.1f", v)
	return strings.TrimSuffix(s, ".0")
}
