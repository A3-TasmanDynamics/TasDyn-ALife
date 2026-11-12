package chart

import (
	"strings"
	"testing"
)

func TestBuildSpark(t *testing.T) {
	if s := BuildSpark([]float64{5}, 120, 28, Count); s.Path != "" {
		t.Errorf("one value should draw nothing: %+v", s)
	}
	s := BuildSpark([]float64{2, 8, 4, 9}, 120, 28, Count)
	if !strings.HasPrefix(s.Path, "M2.0 ") || strings.Count(s.Path, "L") != 3 {
		t.Errorf("path: %q", s.Path)
	}
	if s.LastX != 118 || s.LastY != 2 { // the last value is the peak: top right
		t.Errorf("last point: %v,%v", s.LastX, s.LastY)
	}
	if !strings.HasSuffix(s.Area, "Z") || s.Label != "from 2 to 9, peak 9" {
		t.Errorf("area %q label %q", s.Area, s.Label)
	}
	flat := BuildSpark([]float64{3, 3, 3}, 120, 28, Count)
	if flat.Path == "" || strings.Contains(flat.Path, "NaN") {
		t.Errorf("flat: %q", flat.Path)
	}
}
