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
