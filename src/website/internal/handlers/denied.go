package handlers

import (
	"net/http"

	"website/internal/auth"
)

type deniedData struct {
	Base
	Info     auth.DeniedInfo
	AreaName string
}

// Denied renders the layout plan's access-denied pages (suspended, on
// leave, not staff, missing permission) with a 403. Wired into
// auth.Authenticator.Denied in main.go. Shown with the normal site header,
// even under /admin, because the admin sidebar is exactly what's refused.
func (d *Deps) Denied(w http.ResponseWriter, r *http.Request, info auth.DeniedInfo) {
	title := map[string]string{
		"suspended": "Access suspended", "loa": "You're on leave", "no_permission": "No access",
	}[info.Kind]
	if title == "" {
		title = "Staff only"
	}
	data := deniedData{Base: baseFrom(r, title), Info: info, AreaName: "admin panel"}
	data.Panel = false
	if info.Area == "support" {
		data.AreaName = "support panel"
	}
	d.Render.RenderStatus(w, http.StatusForbidden, "denied.html", data)
}
