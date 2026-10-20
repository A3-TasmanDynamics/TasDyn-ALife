package lookup

import (
	"reflect"
	"testing"
)

func TestParseTrunk(t *testing.T) {
	items, cash := parseTrunk([]byte(`[["items", [["copper_ore", 12], ["water", 2], ["bad"], ["apple", 0]]], ["cash", 1500]]`))
	want := []Stack{{Name: "Copper Ore", Key: "copper_ore", Count: 12}, {Name: "Water Bottle", Key: "water", Count: 2}}
	if !reflect.DeepEqual(items, want) || cash != 1500 {
		t.Errorf("got %v $%d", items, cash)
	}
	if items, cash := parseTrunk([]byte(`[]`)); items != nil || cash != 0 {
		t.Errorf("empty: got %v $%d", items, cash)
	}
}

func TestParseDamage(t *testing.T) {
	got := parseDamage([]byte(`[["hitengine", 0.4], ["hitlfwheel", 1], ["hithull", 0]]`))
	want := []Part{{Name: "LF wheel", Pct: 100}, {Name: "Engine", Pct: 40}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseStorage(t *testing.T) {
	got := parseStorage([]byte(`[["water", 3], ["note", "hi"]]`))
	want := []StoredItem{{Name: "Water Bottle", Value: "×3"}, {Name: "Note", Value: `"hi"`}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
