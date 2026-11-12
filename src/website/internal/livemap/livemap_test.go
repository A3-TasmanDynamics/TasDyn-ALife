package livemap

import (
	"os"
	"testing"
)

func TestParsePosition(t *testing.T) {
	for in, want := range map[string][3]float64{
		`[["x",3642.1],["y",13113.0],["z",0.4]]`: {3642.1, 13113, 1},
		`[["y",100],["x",200],["z",0]]`:          {200, 100, 1},
		`[16830, 12650, 0]`:                      {16830, 12650, 1},
		`[]`:                                     {0, 0, 0},
		`[["x",0],["y",0],["z",0]]`:              {0, 0, 0}, // never placed
		`[["x",99999],["y",5],["z",0]]`:          {0, 0, 0}, // off the map
		`not json`:                               {0, 0, 0},
	} {
		x, y, ok := ParsePosition(in)
		if ok != (want[2] == 1) || (ok && (x != want[0] || y != want[1])) {
			t.Errorf("%s: %v %v %v", in, x, y, ok)
		}
	}
}

func TestParseMarkersFromMission(t *testing.T) {
	b, err := os.ReadFile("../../../ALife.Altis/mission.sqm")
	if err != nil {
		t.Skip("mission.sqm not found")
	}
	ps := ParseMarkers(string(b))
	towns := map[string]Place{}
	for _, p := range ps {
		if p.X <= 0 || p.Y <= 0 || p.X > WorldSize || p.Y > WorldSize {
			t.Errorf("off the map: %+v", p)
		}
		if p.Town {
			towns[p.Name] = p
		}
	}
	k, ok := towns["Kavala"]
	if !ok || k.X < 3000 || k.X > 4500 || k.Y < 12500 || k.Y > 13500 {
		t.Errorf("Kavala: %+v (towns %v)", k, towns)
	}
	if len(ps) < 5 {
		t.Errorf("only %d places", len(ps))
	}
	t.Logf("%d places, towns: %v", len(ps), towns)
}
