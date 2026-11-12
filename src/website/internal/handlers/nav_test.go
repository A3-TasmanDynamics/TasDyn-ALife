package handlers

import (
	"testing"

	"website/internal/auth"
)

func TestBuildNav(t *testing.T) {
	keys := func(s AdminShell) map[string]bool {
		m := map[string]bool{}
		for _, sec := range s.Nav {
			for _, it := range sec.Items {
				m[it.Key] = true
			}
		}
		return m
	}

	// A moderator: cases and flags, not the server pages.
	s := AdminShell{AdminTab: "anticheat", OpenFlags: 3, OpenCases: 0,
		Perms: map[string]bool{"cases.view": true, "anticheat.review": true, "players.view": true}}
	buildNav(&s, &auth.Session{AdminPanelAccess: true})
	k := keys(s)
	for _, want := range []string{"dashboard", "cases", "anticheat", "bans", "watchlist", "players", "gangs", "roles", "stafflog"} {
		if !k[want] {
			t.Errorf("moderator should see %s", want)
		}
	}
	for _, not := range []string{"server", "database", "discord", "vehicles", "support", "dev-health"} {
		if k[not] {
			t.Errorf("moderator shouldn't see %s", not)
		}
	}
	if s.NavSection == nil || s.NavSection.Key != "moderation" || !s.NavSection.Active || s.NavPage == nil || s.NavPage.Label != "Anti-Cheat Flags" {
		t.Fatalf("current: %+v %+v", s.NavSection, s.NavPage)
	}
	if len(s.NavTabs) != 4 {
		t.Errorf("moderation tabs: %d", len(s.NavTabs))
	}
	for _, it := range s.NavTabs {
		if it.Key == "anticheat" && (it.Count != 3 || !it.Hot || !it.Active) {
			t.Errorf("flags tab: %+v", it)
		}
	}

	// A section that isn't tabbed: no tabs; a page in no section: no header.
	s = AdminShell{AdminTab: "server", Perms: map[string]bool{"server.logs": true}}
	buildNav(&s, &auth.Session{AdminPanelAccess: true})
	if s.NavPage == nil || s.NavTabs != nil {
		t.Errorf("server: page %v tabs %v", s.NavPage, s.NavTabs)
	}
	s = AdminShell{AdminTab: "nowhere"}
	buildNav(&s, &auth.Session{AdminPanelAccess: true})
	if s.NavPage != nil || s.NavSection != nil {
		t.Error("unknown tab matched a page")
	}

	// Support-only staff: just the Support section.
	s = AdminShell{AdminTab: "tickets", UnassignedTickets: 2, OpenTickets: 5}
	buildNav(&s, &auth.Session{SupportPanelAccess: true})
	if len(s.Nav) != 1 || s.Nav[0].Key != "support" {
		t.Fatalf("support-only nav: %+v", s.Nav)
	}
	if it := s.Nav[0].Items[1]; it.Count != 2 || !it.Hot {
		t.Errorf("unassigned tickets badge: %+v", it)
	}
	buildNav(&s, nil)
	if s.Nav != nil {
		t.Error("no session should have no nav")
	}
}
