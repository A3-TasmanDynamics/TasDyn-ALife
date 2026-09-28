package audit

import "testing"

func TestCategory(t *testing.T) {
	cases := map[string]string{
		"rank_change:staff_rank_id": "Permissions",
		"rank_change:cop_level":     "Permissions",
		"rank_edit:Moderator":       "Permissions",
		"faction_rank_names:ems":    "Permissions",
		"player.ban":                "Moderation",
		"player.compensate":         "Economy",
		"anticheat.dismiss":         "Moderation",
		"ticket.close":              "Tickets",
		"database.query":            "Database",
		"devlog_post":               "Other",
	}
	for action, want := range cases {
		if got := Category(action); got != want {
			t.Errorf("Category(%q) = %q, want %q", action, got, want)
		}
	}
}

func TestDisplayAction(t *testing.T) {
	cases := map[string]string{
		"rank_change:staff_status": "staff.status",
		"rank_change:medic_level":  "player.ems_level",
		"rank_create:Support":      "roles.create",
		"player.ban":               "player.ban",
	}
	for action, want := range cases {
		if got := DisplayAction(action); got != want {
			t.Errorf("DisplayAction(%q) = %q, want %q", action, got, want)
		}
	}
}
