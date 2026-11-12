package handlers

// The admin panel's navigation, in one place: the sidebar sections, which
// pages each holds, who can see them (permission keys), their badges, and
// which sections show their pages as tabs across the top of each page.
// partial_admin_sidebar.html draws Nav; partial_admin_header.html draws
// the breadcrumb and tabs from NavSection / NavPage / NavTabs.

import "website/internal/auth"

// NavItem is one page link.
type NavItem struct {
	Key    string // AdminTab value of the page
	Label  string
	Icon   string // SVG path data (24x24, stroked), from navIcons
	URL    string
	Count  int  // badge; 0 = none
	Hot    bool // badge needs action (amber)
	New    bool // "New" tag
	Active bool
}

// NavSection is one collapsible sidebar group.
type NavSection struct {
	Key    string // stable id, for remembering it's collapsed
	Name   string
	Items  []NavItem
	Active bool // holds the current page (always drawn open)
	Tabs   bool // its pages are tabs on each page's header
}

type navDef struct {
	key, label, url string
	perm            string // "" = anyone with the panel; "-support" = support panel access
	badge           func(s *AdminShell) (int, bool)
	isNew           bool
}

type navSectionDef struct {
	key, name string
	tabs      bool
	support   bool // shown for support panel access (not admin panel)
	items     []navDef
}

var navDefs = []navSectionDef{
	{key: "overview", name: "Overview", items: []navDef{
		{key: "dashboard", label: "Dashboard", url: "/admin"},
		{key: "inbox", label: "Inbox", url: "/admin/inbox"},
		{key: "online", label: "Online Now", url: "/admin/online", perm: "players.view"},
		{key: "map", label: "Live Map", url: "/admin/map", perm: "game.markers"},
	}},
	{key: "moderation", name: "Moderation", tabs: true, items: []navDef{
		{key: "cases", label: "Cases", url: "/admin/cases", perm: "cases.view", badge: func(s *AdminShell) (int, bool) { return s.OpenCases, true }},
		{key: "reports", label: "Reports", url: "/admin/reports", perm: "cases.view", badge: func(s *AdminShell) (int, bool) { return s.NewReports, true }},
		{key: "anticheat", label: "Anti-Cheat Flags", url: "/admin/anticheat", perm: "anticheat.review", badge: func(s *AdminShell) (int, bool) { return s.OpenFlags, true }},
		{key: "bans", label: "Bans", url: "/admin/bans", perm: "cases.view", badge: func(s *AdminShell) (int, bool) { return s.ActiveBans, false }},
		{key: "watchlist", label: "Watchlist", url: "/admin/watchlist", perm: "players.view"},
	}},
	{key: "players", name: "Players", tabs: true, items: []navDef{
		{key: "players", label: "Player Lookup", url: "/admin/players", perm: "players.view"},
		{key: "events", label: "Event Log", url: "/admin/events", perm: "players.view"},
		{key: "vehicles", label: "Vehicles", url: "/admin/vehicles", perm: "players.vehicles"},
		{key: "houses", label: "Houses & Garages", url: "/admin/houses", perm: "players.view"},
	}},
	{key: "world", name: "World", tabs: true, items: []navDef{
		{key: "economy", label: "Economy & Crime", url: "/admin/economy", perm: "players.view"},
		{key: "gangs", label: "Gangs", url: "/admin/gangs", perm: "players.view"},
		{key: "wanted", label: "Wanted & Jail", url: "/admin/wanted", perm: "players.view"},
	}},
	{key: "support", name: "Support", tabs: true, support: true, items: []navDef{
		{key: "support", label: "Support Overview", url: "/admin/support", perm: "-support"},
		{key: "tickets", label: "Tickets", url: "/admin/tickets", perm: "-support", badge: func(s *AdminShell) (int, bool) {
			if s.UnassignedTickets > 0 {
				return s.UnassignedTickets, true
			}
			return s.OpenTickets, false
		}},
	}},
	{key: "staff", name: "Staff", tabs: true, items: []navDef{
		{key: "staff", label: "Staff Directory", url: "/admin/staff", perm: "staff.view"},
		{key: "staffactivity", label: "Staff Activity", url: "/admin/staff-activity", perm: "staff.view"},
		{key: "applications", label: "Applications", url: "/admin/applications", perm: "applications.view", badge: func(s *AdminShell) (int, bool) { return s.OpenApps, true }},
		{key: "roles", label: "Roles & Permissions", url: "/admin/roles"},
		{key: "rolesync", label: "Discord Role Sync", url: "/admin/role-sync", perm: "roles.manage"},
		{key: "stafflog", label: "Staff Log", url: "/admin/staff-log"},
	}},
	{key: "server", name: "Server", items: []navDef{
		{key: "server", label: "Server Control", url: "/admin/server", perm: "server.logs"},
		{key: "discord", label: "Discord Bot", url: "/admin/discord", perm: "bot.admin"},
		{key: "prices", label: "Item Prices", url: "/admin/prices", perm: "players.compensate"},
		{key: "database", label: "Database", url: "/admin/database", perm: "database.query"},
	}},
	{key: "content", name: "Content", tabs: true, items: []navDef{
		{key: "devlog", label: "New Devlog Post", url: "/admin/devlog/new", perm: "announce.post"},
		{key: "notices", label: "Essential Notices", url: "/admin/notices", perm: "announce.post"},
		{key: "schedule", label: "Scheduled Messages", url: "/admin/schedule", perm: "announce.post"},
		{key: "rules", label: "Server Rules", url: "/admin/rules", perm: "rules.edit"},
		{key: "replies", label: "Saved Replies", url: "/admin/replies"},
	}},
	{key: "dev", name: "Development", tabs: true, items: []navDef{
		{key: "dev-health", label: "System Health", url: "/admin/dev", perm: "dev.tools"},
		{key: "dev-logs", label: "Website Logs", url: "/admin/dev/logs", perm: "dev.tools"},
		{key: "dev-board", label: "Project Board", url: "/admin/dev/board", perm: "dev.tools"},
	}},
}

// buildNav fills s.Nav, s.NavSection, s.NavPage and s.NavTabs for the viewer.
func buildNav(s *AdminShell, sess *auth.Session) {
	s.Nav, s.NavTabs, s.NavSection, s.NavPage = nil, nil, nil, nil
	if sess == nil {
		return
	}
	for _, sd := range navDefs {
		if sd.support && !sess.SupportPanelAccess {
			continue
		}
		if !sd.support && !sess.AdminPanelAccess {
			continue
		}
		sec := NavSection{Key: sd.key, Name: sd.name, Tabs: sd.tabs}
		for _, d := range sd.items {
			switch d.perm {
			case "":
			case "-support":
				if !sess.SupportPanelAccess {
					continue
				}
			default:
				if !s.Perms[d.perm] {
					continue
				}
			}
			it := NavItem{Key: d.key, Label: d.label, Icon: navIcons[d.key], URL: d.url, New: d.isNew, Active: d.key == s.AdminTab}
			if d.badge != nil {
				it.Count, it.Hot = d.badge(s)
			}
			if it.Active {
				sec.Active = true
			}
			sec.Items = append(sec.Items, it)
		}
		if len(sec.Items) > 0 {
			s.Nav = append(s.Nav, sec)
		}
	}
	for i := range s.Nav {
		sec := &s.Nav[i]
		for j := range sec.Items {
			if sec.Items[j].Active {
				s.NavSection, s.NavPage = sec, &sec.Items[j]
				if sec.Tabs && len(sec.Items) > 1 {
					s.NavTabs = sec.Items
				}
			}
		}
	}
}

// navIcons are simple 24x24 stroked line icons, one per page.
var navIcons = map[string]string{
	"dashboard":     "M3 3h7v9H3zM14 3h7v5h-7zM14 12h7v9h-7zM3 16h7v5H3z",
	"online":        "M22 12h-4l-3 9L9 3l-3 9H2",
	"cases":         "M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z",
	"anticheat":     "M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z",
	"bans":          "M12 3a9 9 0 1 0 0 18a9 9 0 1 0 0-18M5.6 5.6l12.8 12.8",
	"watchlist":     "M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12zM12 9a3 3 0 1 0 0 6a3 3 0 1 0 0-6",
	"players":       "M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2M12 3a4 4 0 1 0 0 8a4 4 0 1 0 0-8",
	"events":        "M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01",
	"vehicles":      "M3 16v-4l2.5-5h13l2.5 5v4zM3 16v2h3v-2M18 16v2h3v-2M7 12.5h.01M17 12.5h.01",
	"houses":        "M3 11l9-7 9 7v9a1 1 0 0 1-1 1h-5v-6h-6v6H4a1 1 0 0 1-1-1z",
	"economy":       "M12 2v20M17 6H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6",
	"gangs":         "M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2M9 3a4 4 0 1 0 0 8a4 4 0 1 0 0-8M23 21v-2a4 4 0 0 0-3-3.9M16 3.1a4 4 0 0 1 0 7.8",
	"wanted":        "M12 3a9 9 0 1 0 0 18a9 9 0 1 0 0-18M12 8a4 4 0 1 0 0 8a4 4 0 1 0 0-8M12 11.5v1",
	"support":       "M12 3a9 9 0 1 0 0 18a9 9 0 1 0 0-18M9.1 9a3 3 0 0 1 5.8 1c0 2-3 3-3 3M12 17h.01",
	"tickets":       "M3 9a2 2 0 0 0 0 6v3a1 1 0 0 0 1 1h16a1 1 0 0 0 1-1v-3a2 2 0 0 0 0-6V6a1 1 0 0 0-1-1H4a1 1 0 0 0-1 1zM14 5v14",
	"staff":         "M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M9 3a4 4 0 1 0 0 8a4 4 0 1 0 0-8M19 8v6M22 11h-6",
	"staffactivity": "M3 3v18h18M7 15l4-4 3 3 5-6",
	"map":           "M9 3L3 6v15l6-3 6 3 6-3V3l-6 3zM9 3v15M15 6v15",
	"reports":       "M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1zM4 22v-7",
	"inbox":         "M22 12h-6l-2 3h-4l-2-3H2M5.5 5h13l3.5 7v6a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2v-6z",
	"schedule":      "M12 3a9 9 0 1 0 0 18a9 9 0 1 0 0-18M12 7v5l3 2",
	"replies":       "M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2zM8 9h8M8 13h5",
	"applications":  "M9 3h6v4H9zM9 5H6a1 1 0 0 0-1 1v14a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V6a1 1 0 0 0-1-1h-3M9 12h6M9 16h4",
	"roles":         "M5 11h14v10H5zM8 11V7a4 4 0 0 1 8 0v4",
	"rolesync":      "M21 12a9 9 0 0 1-15.5 6.3M3 12A9 9 0 0 1 18.5 5.7M21 3v5h-5M3 21v-5h5",
	"stafflog":      "M14 3H6a1 1 0 0 0-1 1v16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V8zM14 3v5h5M9 13h6M9 17h6",
	"server":        "M3 4h18v7H3zM3 13h18v7H3zM7 7.5h.01M7 16.5h.01",
	"discord":       "M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z",
	"prices":        "M20.6 13.4l-7.2 7.2a2 2 0 0 1-2.8 0L3 13V3h10l7.6 7.6a2 2 0 0 1 0 2.8zM7.5 7.5h.01",
	"database":      "M12 3c4.4 0 8 1.3 8 3s-3.6 3-8 3-8-1.3-8-3 3.6-3 8-3zM4 6v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3",
	"devlog":        "M12 20h9M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z",
	"notices":       "M3 11v2a1 1 0 0 0 1 1h3l5 4V6L7 10H4a1 1 0 0 0-1 1zM16 9a3 3 0 0 1 0 6",
	"rules":         "M4 19.5A2.5 2.5 0 0 1 6.5 17H20V3H6.5A2.5 2.5 0 0 0 4 5.5zM4 19.5A2.5 2.5 0 0 0 6.5 22H20v-5",
	"dev-health":    "M3 12h4l2-5 4 10 2-5h6",
	"dev-logs":      "M4 17l6-6-6-6M12 19h8",
	"dev-board":     "M4 3h4v18H4zM10 3h4v12h-4zM16 3h4v8h-4z",
}
