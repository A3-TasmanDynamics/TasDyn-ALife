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
		"suspended": "Access suspended", "loa": "You're on leave", "no_permission": "No access", "not_command": "Command only", "awaiting": "Application in review",
	}[info.Kind]
	if title == "" {
		title = "Staff only"
	}
	data := deniedData{Base: baseFrom(r, title), Info: info, AreaName: "admin panel"}
	data.Panel = false
	switch info.Area {
	case "support":
		data.AreaName = "support panel"
	case "police", "ems":
		data.AreaName = map[string]string{"police": "Police", "ems": "EMS"}[info.Area]
	case "":
		if info.Kind == "not_command" {
			data.AreaName = ""
		}
	}
	d.Render.RenderStatus(w, http.StatusForbidden, "denied.html", data)
}
