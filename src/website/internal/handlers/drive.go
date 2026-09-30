package handlers

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"website/internal/auth"
	"website/internal/factions"
)

// The faction drive (internal/factions/drive.go): folders, documents written
// in the built-in editor, and uploaded files. The command panel has the full
// drive; faction members read what's shown to them from the Factions page.

func (d *Deps) driveViewer(r *http.Request, faction string) (factions.DriveViewer, error) {
	sess, _ := auth.FromContext(r.Context())
	return factions.DriveViewerFor(r.Context(), d.Pool, faction, sess.PlayerID, d.can(r, "factions.audit"), d.can(r, "factions.configure"))
}

type driveDivision struct{ Key, Name string }

type driveData struct {
	commandBase
	Viewer     factions.DriveViewer
	List       factions.DriveListing
	FolderID   int64
	Recent     []factions.Item
	Divisions  []driveDivision
	Categories []struct{ Key, Label string }
	Accept     string
	MaxMB      int
	CanAdd     bool // add here (this folder's division, or faction-wide)
	Division   string
}

func (d *Deps) loadDriveDivisions(r *http.Request, faction string) []driveDivision {
	divs, _ := factions.Divisions(r.Context(), d.Pool, faction)
	out := []driveDivision{}
	for _, dv := range divs {
		out = append(out, driveDivision{dv.Key, dv.Name})
	}
	return out
}

// CommandDrive is the drive browser.
func (d *Deps) CommandDrive(w http.ResponseWriter, r *http.Request) {
	cb, ok := d.commandAccess(w, r, "drive")
	if !ok {
		return
	}
	v, err := d.driveViewer(r, cb.Faction)
	if err != nil {
		http.Error(w, "Couldn't check your access.", http.StatusInternalServerError)
		return
	}
	folder, _ := strconv.ParseInt(r.URL.Query().Get("folder"), 10, 64)
	data := driveData{commandBase: cb, Viewer: v, FolderID: folder, Categories: factions.DriveCategories,
		Accept: factions.AllowedExtensions(), MaxMB: factions.MaxFileSize >> 20}
	data.Title = "Drive · " + cb.FactionName + " command"
	if data.List, err = factions.ListDrive(r.Context(), d.Pool, v, folder, ""); err != nil {
		finishCommandAction(w, r, "/command/"+cb.Faction+"/drive", err, "")
		return
	}
	data.Division = data.List.Folder.Division
	data.CanAdd = v.CanEdit(data.Division)
	data.Divisions = d.loadDriveDivisions(r, cb.Faction)
	if folder == 0 {
		data.Recent, _ = factions.RecentDrive(r.Context(), d.Pool, v, 6)
	}
	d.Render.Render(w, "command_drive.html", data)
}

type docData struct {
	commandBase
	InPanel     bool
	Base2       string // URL prefix for this item's links
	Item        factions.Item
	Body        template.HTML
	CanEdit     bool
	Versions    []factions.Version
	Version     *factions.Version
	ShowHistory bool
}

// docBase is where a document's links point: the panel or the member view.
func docBase(faction string, inPanel bool) string {
	if inPanel {
		return "/command/" + faction + "/drive"
	}
	return "/factions/" + faction + "/docs"
}

func (d *Deps) renderDoc(w http.ResponseWriter, r *http.Request, cb commandBase, inPanel bool) {
	v, err := d.driveViewer(r, cb.Faction)
	if err != nil {
		http.Error(w, "Couldn't check your access.", http.StatusInternalServerError)
		return
	}
	id := urlID(r, "id")
	it, body, err := factions.GetItem(r.Context(), d.Pool, v, id)
	if err != nil {
		d.driveNotFound(w, r, cb.Faction, inPanel, err)
		return
	}
	if it.Kind == "file" {
		http.Redirect(w, r, fmt.Sprintf("%s/file/%d", docBase(cb.Faction, inPanel), id), http.StatusSeeOther)
		return
	}
	data := docData{commandBase: cb, InPanel: inPanel, Base2: docBase(cb.Faction, inPanel), Item: it,
		Body: template.HTML(factions.SanitizeDoc(body)), CanEdit: inPanel && v.CanEdit(it.Division)}
	data.Title = it.Title
	if vid, _ := strconv.ParseInt(chi.URLParam(r, "vid"), 10, 64); vid > 0 {
		ver, err := factions.GetVersion(r.Context(), d.Pool, v, id, vid)
		if err != nil {
			d.driveNotFound(w, r, cb.Faction, inPanel, err)
			return
		}
		data.Version, data.Body = &ver, template.HTML(factions.SanitizeDoc(ver.Body))
	}
	if inPanel && strings.HasSuffix(r.URL.Path, "/history") {
		data.ShowHistory = true
		data.Versions, _ = factions.Versions(r.Context(), d.Pool, v, id)
	}
	d.Render.Render(w, "drive_doc.html", data)
}

func (d *Deps) driveNotFound(w http.ResponseWriter, r *http.Request, faction string, inPanel bool, err error) {
	if errors.Is(err, factions.ErrNotAllowed) {
		finishCommandAction(w, r, docBase(faction, inPanel), err, "")
		return
	}
	slog.Error("drive: load failed", "error", err)
	http.Error(w, "Failed to load that.", http.StatusInternalServerError)
}

// CommandDriveDoc shows a document (and its history) in the panel.
func (d *Deps) CommandDriveDoc(w http.ResponseWriter, r *http.Request) {
	cb, ok := d.commandAccess(w, r, "drive")
	if !ok {
		return
	}
	d.renderDoc(w, r, cb, true)
}

type editorData struct {
	commandBase
	ID         int64
	Item       factions.ItemInput
	Body       template.HTML
	Folders    []factions.Folder
	Divisions  []driveDivision
	Categories []struct{ Key, Label string }
	Viewer     factions.DriveViewer
}

// CommandDriveEditor is the document editor, for a new document (/new) or an
// existing one (/doc/{id}/edit).
func (d *Deps) CommandDriveEditor(w http.ResponseWriter, r *http.Request) {
	cb, ok := d.commandAccess(w, r, "drive")
	if !ok {
		return
	}
	v, err := d.driveViewer(r, cb.Faction)
	if err != nil {
		http.Error(w, "Couldn't check your access.", http.StatusInternalServerError)
		return
	}
	q := r.URL.Query()
	data := editorData{commandBase: cb, Categories: factions.DriveCategories, Viewer: v, Divisions: d.loadDriveDivisions(r, cb.Faction)}
	if id := urlID(r, "id"); id > 0 {
		it, body, err := factions.GetItem(r.Context(), d.Pool, v, id)
		if err != nil || it.Kind != "doc" {
			d.driveNotFound(w, r, cb.Faction, true, err)
			return
		}
		if !v.CanEdit(it.Division) {
			finishCommandAction(w, r, fmt.Sprintf("/command/%s/drive/doc/%d", cb.Faction, id), fmt.Errorf("%w: you can't edit that document", factions.ErrNotAllowed), "")
			return
		}
		data.ID = id
		data.Item = factions.ItemInput{FolderID: it.FolderID, Title: it.Title, Category: it.Category, Division: it.Division, Visibility: it.Visibility}
		data.Body = template.HTML(factions.SanitizeDoc(body))
	} else {
		tpl, ok := factions.DocTemplates[q.Get("template")]
		if !ok {
			tpl = factions.DocTemplates["blank"]
		}
		folder, _ := strconv.ParseInt(q.Get("folder"), 10, 64)
		data.Item = factions.ItemInput{FolderID: folder, Title: tpl.Title, Category: tpl.Category, Division: q.Get("division"), Visibility: "members"}
		data.Body = template.HTML(tpl.Body)
		if !v.CanEdit(data.Item.Division) {
			finishCommandAction(w, r, "/command/"+cb.Faction+"/drive", fmt.Errorf("%w: only command, Administration, cabinet or that division's command can add documents", factions.ErrNotAllowed), "")
			return
		}
	}
	data.Title = data.Item.Title + " · editor"
	data.Folders = d.allFolders(r, v)
	d.Render.Render(w, "drive_editor.html", data)
}

// allFolders lists every folder the viewer can see, for the folder picker.
func (d *Deps) allFolders(r *http.Request, v factions.DriveViewer) []factions.Folder {
	var out []factions.Folder
	var walk func(id int64, depth int)
	walk = func(id int64, depth int) {
		if depth > 6 {
			return
		}
		l, err := factions.ListDrive(r.Context(), d.Pool, v, id, "")
		if err != nil {
			return
		}
		for _, f := range l.Folders {
			f.Name = strings.Repeat("— ", depth) + f.Name
			out = append(out, f)
			walk(f.ID, depth+1)
		}
	}
	walk(0, 0)
	return out
}

func driveInput(r *http.Request) factions.ItemInput {
	folder, _ := strconv.ParseInt(r.FormValue("folder"), 10, 64)
	return factions.ItemInput{FolderID: folder, Title: r.FormValue("title"), Category: r.FormValue("category"),
		Division: r.FormValue("division"), Visibility: r.FormValue("visibility"), Body: r.FormValue("body")}
}

// CommandDriveSave creates (/docs) or saves (/doc/{id}) a document.
func (d *Deps) CommandDriveSave(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	v, err := d.driveViewer(r, faction)
	if err != nil {
		http.Error(w, "Couldn't check your access.", http.StatusInternalServerError)
		return
	}
	in := driveInput(r)
	id := urlID(r, "id")
	if id > 0 {
		err = factions.SaveDoc(r.Context(), d.Pool, v, id, in)
		finishCommandAction(w, r, fmt.Sprintf("/command/%s/drive/doc/%d", faction, id), err, "Saved. The previous version is in the history.")
		return
	}
	id, err = factions.CreateDoc(r.Context(), d.Pool, v, in)
	if err != nil {
		finishCommandAction(w, r, "/command/"+faction+"/drive", err, "")
		return
	}
	finishCommandAction(w, r, fmt.Sprintf("/command/%s/drive/doc/%d", faction, id), nil, "Document created.")
}

// CommandDriveUpload stores an uploaded file.
func (d *Deps) CommandDriveUpload(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	in := driveInput(r)
	back := "/command/" + faction + "/drive"
	if in.FolderID > 0 {
		back += fmt.Sprintf("?folder=%d", in.FolderID)
	}
	v, err := d.driveViewer(r, faction)
	if err != nil {
		http.Error(w, "Couldn't check your access.", http.StatusInternalServerError)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		finishCommandAction(w, r, back, fmt.Errorf("%w: choose a file to upload", factions.ErrNotAllowed), "")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, factions.MaxFileSize+1))
	if err != nil {
		finishCommandAction(w, r, back, err, "")
		return
	}
	_, err = factions.UploadFile(r.Context(), d.Pool, v, in, hdr.Filename, data)
	finishCommandAction(w, r, back, err, "Uploaded.")
}

// CommandDriveRemove archives a document or file.
func (d *Deps) CommandDriveRemove(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	v, err := d.driveViewer(r, faction)
	if err != nil {
		http.Error(w, "Couldn't check your access.", http.StatusInternalServerError)
		return
	}
	err = factions.ArchiveItem(r.Context(), d.Pool, v, urlID(r, "id"))
	finishCommandAction(w, r, "/command/"+faction+"/drive", err, "Removed from the drive.")
}

// CommandDriveFolder creates a folder, or removes an empty one.
func (d *Deps) CommandDriveFolder(w http.ResponseWriter, r *http.Request) {
	faction, ok := postFaction(w, r)
	if !ok {
		return
	}
	v, err := d.driveViewer(r, faction)
	if err != nil {
		http.Error(w, "Couldn't check your access.", http.StatusInternalServerError)
		return
	}
	parent, _ := strconv.ParseInt(r.FormValue("parent"), 10, 64)
	back := "/command/" + faction + "/drive"
	if id := urlID(r, "id"); id > 0 {
		err = factions.ArchiveFolder(r.Context(), d.Pool, v, id)
		if parent > 0 {
			back += fmt.Sprintf("?folder=%d", parent)
		}
		finishCommandAction(w, r, back, err, "Folder removed.")
		return
	}
	id, err := factions.CreateFolder(r.Context(), d.Pool, v, parent, r.FormValue("name"), r.FormValue("division"), r.FormValue("visibility"))
	if err == nil {
		back += fmt.Sprintf("?folder=%d", id)
	} else if parent > 0 {
		back += fmt.Sprintf("?folder=%d", parent)
	}
	finishCommandAction(w, r, back, err, "Folder created.")
}

// serveWord sends a document as a Word file (Word opens HTML .doc files).
func (d *Deps) serveWord(w http.ResponseWriter, r *http.Request, faction string, inPanel bool) {
	v, err := d.driveViewer(r, faction)
	if err != nil {
		http.Error(w, "Couldn't check your access.", http.StatusInternalServerError)
		return
	}
	it, body, err := factions.GetItem(r.Context(), d.Pool, v, urlID(r, "id"))
	if err != nil || it.Kind != "doc" {
		d.driveNotFound(w, r, faction, inPanel, err)
		return
	}
	name := safeFileName(it.Title) + ".doc"
	w.Header().Set("Content-Type", "application/msword")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"; filename*=UTF-8''`+url.PathEscape(name))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	fmt.Fprintf(w, `<html xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:w="urn:schemas-microsoft-com:office:word" xmlns="http://www.w3.org/TR/REC-html40"><head><meta charset="utf-8"><title>%s</title>
<style>
body{font-family:"Segoe UI",Calibri,Arial,sans-serif;font-size:11pt;line-height:1.5;color:#1e293b}
h1{font-size:22pt;color:#0b1220;margin:0 0 10pt}
h2{font-size:15pt;color:#0b1220;margin:18pt 0 6pt;padding-bottom:3pt;border-bottom:1.5pt solid #f59e0b}
h3{font-size:12pt;color:#b45309;margin:12pt 0 4pt}
p{margin:0 0 7pt}
table{border-collapse:collapse;width:100%%;margin:8pt 0}
td,th{border:1px solid #cbd5e1;padding:5pt 7pt;vertical-align:top;text-align:left}
th{background:#1e293b;color:#ffffff;font-weight:bold}
blockquote{border-left:3pt solid #f59e0b;background:#fff7e6;margin:8pt 0;padding:6pt 10pt;color:#44403c}
a{color:#b45309}
.doc-kicker{font-size:9pt;color:#64748b;text-transform:uppercase;letter-spacing:1pt;margin:0 0 4pt}
</style></head><body><p class="doc-kicker">%s · %s</p>%s</body></html>`,
		template.HTMLEscapeString(it.Title), template.HTMLEscapeString(factions.Name(faction)), template.HTMLEscapeString(it.CategoryLabel()), factions.SanitizeDoc(body))
}

func safeFileName(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, r) || r < 32 {
			return '-'
		}
		return r
	}, strings.TrimSpace(s))
	if s == "" {
		s = "document"
	}
	return s
}

// serveFile sends an uploaded file. Images and PDFs open in the browser;
// everything else downloads.
func (d *Deps) serveFile(w http.ResponseWriter, r *http.Request, faction string, inPanel bool) {
	v, err := d.driveViewer(r, faction)
	if err != nil {
		http.Error(w, "Couldn't check your access.", http.StatusInternalServerError)
		return
	}
	it, data, err := factions.FileData(r.Context(), d.Pool, v, urlID(r, "id"))
	if err != nil {
		d.driveNotFound(w, r, faction, inPanel, err)
		return
	}
	disp := "attachment"
	if it.Inline() {
		disp = "inline"
	}
	w.Header().Set("Content-Type", it.FileType)
	w.Header().Set("Content-Disposition", disp+`; filename="`+safeFileName(it.FileName)+`"; filename*=UTF-8''`+url.PathEscape(it.FileName))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if it.FileType != "application/pdf" {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; sandbox")
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write(data)
}

func (d *Deps) CommandDriveWord(w http.ResponseWriter, r *http.Request) {
	if f, ok := postFaction(w, r); ok {
		d.serveWord(w, r, f, true)
	}
}

func (d *Deps) CommandDriveFile(w http.ResponseWriter, r *http.Request) {
	if f, ok := postFaction(w, r); ok {
		d.serveFile(w, r, f, true)
	}
}

// ---- Members: reading the drive from the Factions page ----

type memberDocsData struct {
	Base
	Faction     string
	FactionName string
	List        factions.DriveListing
	FolderID    int64
}

// memberBase builds the member view's page data, or refuses non-members.
func (d *Deps) memberBase(w http.ResponseWriter, r *http.Request) (commandBase, factions.DriveViewer, bool) {
	faction := chi.URLParam(r, "faction")
	var cb commandBase
	if !factions.Valid(faction) {
		http.NotFound(w, r)
		return cb, factions.DriveViewer{}, false
	}
	v, err := d.driveViewer(r, faction)
	if err != nil {
		http.Error(w, "Couldn't check your access.", http.StatusInternalServerError)
		return cb, v, false
	}
	if !v.Member && !v.Panel {
		http.Redirect(w, r, "/factions?faction="+faction+"&error="+errMsg("Documents are for "+factions.Name(faction)+" members."), http.StatusSeeOther)
		return cb, v, false
	}
	// Members read as members, even if they also have the panel.
	v.Panel, v.Editor, v.Leads = false, false, nil
	cb.Base = baseFrom(r, factions.Name(faction)+" documents")
	cb.Faction, cb.FactionName = faction, factions.Name(faction)
	return cb, v, true
}

// FactionDocs lists the documents a member can read.
func (d *Deps) FactionDocs(w http.ResponseWriter, r *http.Request) {
	cb, v, ok := d.memberBase(w, r)
	if !ok {
		return
	}
	folder, _ := strconv.ParseInt(r.URL.Query().Get("folder"), 10, 64)
	data := memberDocsData{Base: cb.Base, Faction: cb.Faction, FactionName: cb.FactionName, FolderID: folder}
	var err error
	if data.List, err = factions.ListDrive(r.Context(), d.Pool, v, folder, ""); err != nil {
		finishCommandAction(w, r, "/factions/"+cb.Faction+"/docs", err, "")
		return
	}
	d.Render.Render(w, "faction_docs.html", data)
}

// FactionDoc shows one document to a member.
func (d *Deps) FactionDoc(w http.ResponseWriter, r *http.Request) {
	cb, v, ok := d.memberBase(w, r)
	if !ok {
		return
	}
	it, body, err := factions.GetItem(r.Context(), d.Pool, v, urlID(r, "id"))
	if err != nil {
		d.driveNotFound(w, r, cb.Faction, false, err)
		return
	}
	if it.Kind == "file" {
		http.Redirect(w, r, fmt.Sprintf("/factions/%s/files/%d", cb.Faction, it.ID), http.StatusSeeOther)
		return
	}
	data := docData{commandBase: cb, Base2: docBase(cb.Faction, false), Item: it, Body: template.HTML(factions.SanitizeDoc(body))}
	data.Title = it.Title
	d.Render.Render(w, "drive_doc.html", data)
}

func (d *Deps) FactionDocWord(w http.ResponseWriter, r *http.Request) {
	if cb, _, ok := d.memberBase(w, r); ok {
		d.serveWord(w, r, cb.Faction, false)
	}
}

func (d *Deps) FactionFile(w http.ResponseWriter, r *http.Request) {
	if cb, _, ok := d.memberBase(w, r); ok {
		d.serveFile(w, r, cb.Faction, false)
	}
}
