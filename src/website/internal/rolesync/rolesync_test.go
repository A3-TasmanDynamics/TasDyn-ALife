package rolesync

import (
	"reflect"
	"testing"
)

func TestPlanOnlyTouchesMappedGroups(t *testing.T) {
	maps := []Mapping{
		{"linked", "verified"},
		{"staff_rank:admin", "admin-role"},
		{"staff_rank:moderator", "mod-role"},
		{"staff_loa", "loa-role"},
	}
	// A moderator promoted to admin, who also has an unmanaged Nitro role.
	ents := []string{"linked", "staff", "staff_rank:admin"}
	current := []string{"verified", "mod-role", "nitro-booster"}

	add, remove := Plan(maps, ents, current)
	if !reflect.DeepEqual(add, []string{"admin-role"}) {
		t.Errorf("add = %v", add)
	}
	if !reflect.DeepEqual(remove, []string{"mod-role"}) {
		t.Errorf("remove = %v (nitro-booster must never be removed)", remove)
	}
}

func TestPlanLOASwapsRole(t *testing.T) {
	maps := []Mapping{{"staff_rank:admin", "admin-role"}, {"staff_loa", "loa-role"}}
	add, remove := Plan(maps, []string{"linked", "staff_loa"}, []string{"admin-role"})
	if !reflect.DeepEqual(add, []string{"loa-role"}) || !reflect.DeepEqual(remove, []string{"admin-role"}) {
		t.Errorf("add=%v remove=%v", add, remove)
	}
}

func TestPlanTwoEntitlementsOneRole(t *testing.T) {
	// Both police ranks map to the same "Police" role: keep it while either applies.
	maps := []Mapping{{"faction_rank:police:1", "police"}, {"faction_rank:police:2", "police"}}
	add, remove := Plan(maps, []string{"faction_rank:police:2"}, []string{"police"})
	if len(add)+len(remove) != 0 {
		t.Errorf("add=%v remove=%v, want no change", add, remove)
	}
}

func TestPlanNoMappingsNoChanges(t *testing.T) {
	add, remove := Plan(nil, []string{"linked", "staff"}, []string{"a", "b"})
	if len(add)+len(remove) != 0 {
		t.Errorf("add=%v remove=%v", add, remove)
	}
}

func TestSlug(t *testing.T) {
	if got := Slug("  Moderation Team! "); got != "moderation_team" {
		t.Errorf("got %q", got)
	}
}
