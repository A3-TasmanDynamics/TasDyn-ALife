package dashboard

import (
	"reflect"
	"testing"
)

func TestBand(t *testing.T) {
	for total, want := range map[int64]string{
		0: "Under $25k", 24_999: "Under $25k", 25_000: "$25k–$50k", 118_400: "$100k–$250k", 654_500: "$500k–$1M", 2_000_000: "$1M+",
	} {
		if _, got := Band(total); got != want {
			t.Errorf("Band(%d) = %q, want %q", total, got, want)
		}
	}
}

func TestRankRowsTies(t *testing.T) {
	es := []entry{{1, "A", 0, ""}, {2, "B", 1, ""}, {3, "C", 2, ""}, {4, "D", 2, ""}, {5, "E", 2, ""}, {6, "F", 3, ""}}
	rows, you := rankRows(es, func(a, b entry) bool { return a.score == b.score }, 4, 10)
	var got []string
	for _, r := range rows {
		got = append(got, r.Rank)
	}
	if want := []string{"1", "2", "3=", "3=", "3=", "6"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ranks = %v, want %v", got, want)
	}
	if you != "3=" || !rows[3].You {
		t.Errorf("your rank = %q", you)
	}
}

func TestHumanize(t *testing.T) {
	for in, want := range map[string]string{"C_Offroad_01_F": "Offroad 01", "kavala_townhouse": "Kavala townhouse", "license_driver": "Driver"} {
		if got := humanize(in); got != want {
			t.Errorf("humanize(%q) = %q, want %q", in, got, want)
		}
	}
}
