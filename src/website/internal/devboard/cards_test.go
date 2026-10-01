package devboard

import (
	"reflect"
	"testing"
	"time"
)

func TestDescriptionBlocks(t *testing.T) {
	body := "Intro line one\r\nline two\r\n\r\n- first\r\n* second\r\n• third\r\nAfter list\r\n\r\n- solo"
	got := Task{Body: body}.DescriptionBlocks()
	want := []Block{
		{Text: "Intro line one\nline two"},
		{Bullets: []string{"first", "second", "third"}},
		{Text: "After list"},
		{Bullets: []string{"solo"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
	if b := (Task{}).DescriptionBlocks(); len(b) != 0 {
		t.Fatalf("empty body gave %#v", b)
	}
}

func TestFeedNewestFirst(t *testing.T) {
	now := time.Now()
	c := Card{
		Comments: []Comment{{ID: 1, Author: "A", Body: "hi", At: now.Add(-time.Hour)}},
		Activity: []Activity{{Actor: "B", What: "created this card", At: now.Add(-2 * time.Hour)}, {Actor: "B", What: "moved", At: now}},
	}
	f := c.Feed()
	if len(f) != 3 || f[0].Text != "moved" || !f[1].Comment || f[2].Text != "created this card" {
		t.Fatalf("feed order wrong: %#v", f)
	}
}
