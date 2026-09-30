package rules

import (
	"errors"
	"reflect"
	"testing"
)

const doc = `# General conduct
1.1 Stay in character.
1.2 Be respectful.
    Hate speech means a permanent ban.

# Vehicles
4.3 No VDM.
`

func TestParse(t *testing.T) {
	secs, err := Parse(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 2 || secs[1].Title != "Vehicles" || secs[1].Anchor != "r2" {
		t.Fatalf("sections = %+v", secs)
	}
	if got := secs[0].Rules[1].Text; got != "Be respectful. Hate speech means a permanent ban." {
		t.Errorf("continuation line: %q", got)
	}
}

func TestParseRejects(t *testing.T) {
	for name, body := range map[string]string{
		"rule before heading": "1.1 Hello",
		"duplicate number":    "# A\n1.1 x\n1.1 y",
		"empty":               "# Only a heading",
		"text before a rule":  "# A\nloose text",
		"empty heading":       "#\n1.1 x",
	} {
		if _, err := Parse(body); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestChanged(t *testing.T) {
	prev, _ := Parse(doc)
	next, _ := Parse("# General conduct\n1.1 Stay in character.\n1.2 Be kind.\n# Vehicles\n4.3 No VDM.\n4.4 New rule.")
	if got := Changed(prev, next); !reflect.DeepEqual(got, []string{"1.2", "4.4"}) {
		t.Errorf("changed = %v", got)
	}
}

func TestFilter(t *testing.T) {
	secs, _ := Parse(doc)
	if got := Filter(secs, "vdm"); len(got) != 1 || got[0].Rules[0].Number != "4.3" {
		t.Errorf("text search: %+v", got)
	}
	if got := Filter(secs, "conduct"); len(got) != 1 || len(got[0].Rules) != 2 {
		t.Errorf("section title search keeps the whole section: %+v", got)
	}
	if got := Filter(secs, "1.2"); len(got) != 1 || got[0].Rules[0].Number != "1.2" {
		t.Errorf("number search: %+v", got)
	}
	if got := Filter(secs, "zzz"); len(got) != 0 {
		t.Errorf("no match: %+v", got)
	}
}

func TestParsePointsAndIntro(t *testing.T) {
	secs, err := Parse("# Safe zones\n> Where combat is never allowed.\n4 Safe zones are:\n- Kavala Markets\n  - Including the car park.\n    Even at night.\n- Kavala Hospital\n4.1 Next rule.")
	if err != nil {
		t.Fatal(err)
	}
	s := secs[0]
	if s.Intro != "Where combat is never allowed." || len(s.Rules) != 2 {
		t.Fatalf("section = %+v", s)
	}
	want := []Point{{Text: "Kavala Markets", Sub: []string{"Including the car park. Even at night."}}, {Text: "Kavala Hospital"}}
	if !reflect.DeepEqual(s.Rules[0].Points, want) {
		t.Errorf("points = %+v", s.Rules[0].Points)
	}
	for name, body := range map[string]string{
		"point before a rule": "# A\n- x\n1.1 y",
		"intro after a rule":  "# A\n1.1 y\n> late intro",
	} {
		if _, err := Parse(body); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestFormatRoundTrip(t *testing.T) {
	secs, _ := Parse("# General\n> Intro   text\n7 Be   nice.\n- a\n  - b\n# TeamSpeak\n9.9 Use push to talk.")
	body := Format(secs)
	want := "# General\n> Intro text\n1.1 Be nice.\n- a\n  - b\n\n# TeamSpeak\n2.1 Use push to talk.\n"
	if body != want {
		t.Fatalf("Format:\n%s\nwant:\n%s", body, want)
	}
	again, err := Parse(body)
	if err != nil || Format(again) != body {
		t.Errorf("round trip: %v\n%s", err, Format(again))
	}
}

func TestChangedIgnoresRenumbering(t *testing.T) {
	prev, _ := Parse("# A\n1.1 One.\n1.2 Two.\n- x")
	next, _ := Parse("# A\n1.1 New first.\n1.2 One.\n1.3 Two.\n- y")
	if got := Changed(prev, next); !reflect.DeepEqual(got, []string{"1.1", "1.3"}) {
		t.Errorf("changed = %v (a moved rule isn't new; a changed dot point is)", got)
	}
}
