package dashboard

import "testing"

func TestCommas(t *testing.T) {
	for in, want := range map[int64]string{0: "0", 999: "999", 1000: "1,000", 1234567: "1,234,567", -45000: "-45,000"} {
		if got := commas(in); got != want {
			t.Errorf("commas(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFinishRanksAndYou(t *testing.T) {
	es := []entry{{1, "bob", 5, "5"}, {2, "amy", 9, "9"}, {3, "cat", 5, "5"}, {4, "dan", 1, "1"}}
	b := finish("x", "X", "", es, false, 3, 2, map[int64]string{2: "note"})
	if len(b.Rows) != 2 || b.Rows[0].Name != "amy" || b.Rows[0].Note != "note" || b.Rows[1].Rank != "2=" || b.Count != 4 {
		t.Fatalf("rows = %+v", b)
	}
	if b.You != "#2 of 4" {
		t.Errorf("you = %q (tied second, below the cut)", b.You)
	}
	bands := finish("w", "W", "", []entry{{1, "z", 2, ""}, {2, "a", 0, ""}}, true, 0, 10, nil)
	if bands.Rows[0].Name != "a" || bands.You != "" {
		t.Errorf("bands: lower index ranks first: %+v", bands)
	}
}
