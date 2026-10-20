package players

import (
	"reflect"
	"testing"
)

func TestParseGear(t *testing.T) {
	raw := []byte(`[["loadout", [["arifle_MX_F", "", "acc_flashlight", "optic_Aco", ["30Rnd_65x39_caseless_mag", 30], [], ""], [], [],
		["U_C_Poloshirt_blue", [["FirstAidKit", 1], ["30Rnd_65x39_caseless_mag", 2, 30]]], [], [], "H_Cap_red", "", [],
		["ItemMap", "ItemGPS", "", "ItemCompass", "ItemWatch", ""]]], ["food", 80.4], ["water", 55]]`)
	var inv Inventory
	parseGear(raw, &inv)
	if inv.Food != 80 || inv.Water != 55 {
		t.Errorf("food/water %d %d", inv.Food, inv.Water)
	}
	if len(inv.Slots) != 3 {
		t.Fatalf("slots %+v", inv.Slots)
	}
	if inv.Slots[0].Label != "Primary" || len(inv.Slots[0].Extra) != 3 {
		t.Errorf("primary %+v", inv.Slots[0])
	}
	if inv.Slots[1].Label != "Uniform" || len(inv.Slots[1].Items) != 2 || inv.Slots[1].Items[1].Count != 2 {
		t.Errorf("uniform %+v", inv.Slots[1])
	}
	if inv.Slots[2].Label != "Headgear" {
		t.Errorf("headgear %+v", inv.Slots[2])
	}
	if want := []string{"Map", "GPS", "Compass", "Watch"}; len(inv.Links) != 4 {
		t.Errorf("links %v, want like %v", inv.Links, want)
	}
	var empty Inventory
	parseGear([]byte(`[]`), &empty)
	parseGear([]byte(`not json`), &empty)
	if !reflect.DeepEqual(empty, Inventory{}) {
		t.Errorf("empty: %+v", empty)
	}
}

func TestVItems(t *testing.T) {
	got := vItems([]byte(`[["water", 2], ["apple", 0], ["bad"], ["copper_ore", 5]]`))
	want := []Stack{{Name: "Water Bottle", Count: 2}, {Name: "Copper Ore", Count: 5}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}
}
