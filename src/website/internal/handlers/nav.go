package handlers

// The admin panel's navigation, in one place: the sidebar sections, which
// pages each holds, who can see them (permission keys), their badges, and
// which sections show their pages as tabs across the top of each page.
// partial_admin_sidebar.html draws Nav; partial_admin_header.html draws
// the breadcrumb and tabs from Section / Page / Tabs.

import "website/internal/auth"

// NavItem is one page link.
type NavItem struct {
	Key    string // AdminTab value of the page
	Label  string
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
		{key: "online", label: "Online Now", url: "/admin/online", perm: "players.view"},
	}},
	{key: "moderation", name: "Moderation", tabs: true, items: []navDef{
		{key: "cases", label: "Cases", url: "/admin/cases", perm: "cases.view", badge: func(s *AdminShell) (int, bool) { return s.OpenCases, true }},
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
		{key: "rules", label: "Server Rules", url: "/admin/rules", perm: "rules.edit"},
	}},
	{key: "dev", name: "Development", tabs: true, items: []navDef{
		{key: "dev-health", label: "System Health", url: "/admin/dev", perm: "dev.tools"},
		{key: "dev-logs", label: "Website Logs", url: "/admin/dev/logs", perm: "dev.tools"},
		{key: "dev-board", label: "Project Board", url: "/admin/dev/board", perm: "dev.tools"},
	}},
}

// buildNav fills s.Nav, s.Section, s.Page and s.Tabs for the viewer.
func buildNav(s *AdminShell, sess *auth.Session) {
	s.Nav, s.Tabs, s.Section, s.Page = nil, nil, nil, nil
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
			it := NavItem{Key: d.key, Label: d.label, URL: d.url, New: d.isNew, Active: d.key == s.AdminTab}
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
				s.Section, s.Page = sec, &sec.Items[j]
				if sec.Tabs && len(sec.Items) > 1 {
					s.Tabs = sec.Items
				}
			}
		}
	}
}
