package rules

import (
	"errors"
	"reflect"
	"strings"
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

func TestDocIntroAndNumbering(t *testing.T) {
	intro, secs, err := ParseDoc("> Welcome.\n> Read these.\n# General rules\n1.1 Respect everyone.\n1.2 No cheating.\n# Gameplay rules\n2.1 Safe zones:\n- Kavala")
	if err != nil {
		t.Fatal(err)
	}
	if intro != "Welcome.\nRead these." || secs[1].Rules[0].Disp != "2.1" || secs[0].Rules[1].Disp != "1.2" {
		t.Errorf("intro %q, numbering %+v", intro, secs)
	}
	body := FormatDoc(intro, secs)
	if i2, s2, err := ParseDoc(body); err != nil || i2 != intro || FormatDoc(i2, s2) != body {
		t.Errorf("round trip: %v\n%s", err, body)
	}
	d := Doc{Sections: secs, Changed: []string{"2.1", "1.2"}}
	if got := d.ChangedLabels(); !reflect.DeepEqual(got, []string{"1.2", "2.1"}) {
		t.Errorf("labels = %v", got)
	}
}

func TestSubsections(t *testing.T) {
	body := "# General rules\n1.1 Be respectful.\n## Chain of command\n> Who answers to whom.\n1.2.1 Follow senior staff.\n- Unless it breaks a rule\n1.2.2 Report up the chain.\n## Discipline\n1.3.1 Points expire.\n\n# Gameplay rules\n## Safe zones\n2.1.1 No combat in:\n- Kavala"
	intro, secs, err := ParseDoc(body)
	if err != nil {
		t.Fatal(err)
	}
	g := secs[0]
	if len(g.Rules) != 1 || len(g.Subs) != 2 || g.Subs[0].Disp != "1.2" || g.Subs[0].Intro != "Who answers to whom." || g.Subs[0].Rules[1].Disp != "1.2.2" || g.Count() != 4 {
		t.Fatalf("general = %+v", g)
	}
	if g.Subs[0].Rules[0].Points[0].Text != "Unless it breaks a rule" || secs[1].Subs[0].Disp != "2.1" || secs[1].Subs[0].Anchor != "r2-1" {
		t.Errorf("points/anchors: %+v", secs[1])
	}
	if FormatDoc(intro, secs) != body+"\n" {
		t.Errorf("format:\n%s", FormatDoc(intro, secs))
	}
	if got := Filter(secs, "chain"); len(got) != 1 || len(got[0].Subs) != 1 || len(got[0].Subs[0].Rules) != 2 || len(got[0].Rules) != 0 {
		t.Errorf("subsection title search: %+v", got)
	}
	if got := Filter(secs, "1.3"); len(got) != 1 || len(got[0].Subs) != 1 || got[0].Subs[0].Title != "Discipline" {
		t.Errorf("number search: %+v", got)
	}
	prev, _ := Parse(body)
	next, _ := Parse(strings.Replace(body, "Points expire.", "Points expire after 90 days.", 1))
	if got := Changed(prev, next); !reflect.DeepEqual(got, []string{"1.3.1"}) {
		t.Errorf("changed = %v", got)
	}
	if _, err := Parse("## Orphan\n1.1 x"); !errors.Is(err, ErrInvalid) {
		t.Errorf("subsection before a section: %v", err)
	}
}
