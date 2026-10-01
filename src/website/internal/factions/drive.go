package factions

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/microcosm-cc/bluemonday"
)

// The faction drive: folders, documents written in the built-in editor
// (SOPs, training documents, policies) and uploaded files.
//
// Reading: the command panel (command, cabinet, Administration, division
// command, staff with factions.audit) sees everything; faction members see
// items shown to members, and a division's items only if they're in it.
// Editing: command ranks, Administration, cabinet and Management edit
// everything; a division's command edits that division's items.

// DriveCategories are the kinds of document.
var DriveCategories = []struct{ Key, Label string }{
	{"sop", "SOP"}, {"training", "Training"}, {"policy", "Policy"}, {"other", "Other"},
}

// CategoryLabel is a category's display name.
func CategoryLabel(k string) string {
	for _, c := range DriveCategories {
		if c.Key == k {
			return c.Label
		}
	}
	return "Other"
}

// MaxFileSize is the largest file the drive accepts.
const MaxFileSize = 15 << 20

// maxBody caps a document's HTML.
const maxBody = 512 << 10

// allowedFiles maps accepted extensions to their content type.
var allowedFiles = map[string]string{
	".pdf": "application/pdf", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".doc":  "application/msword", ".txt": "text/plain", ".md": "text/plain", ".csv": "text/csv",
}

// AllowedExtensions lists accepted file extensions, for the upload form.
func AllowedExtensions() string {
	var out []string
	for k := range allowedFiles {
		out = append(out, k)
	}
	slices.Sort(out)
	return strings.Join(out, ",")
}

var docPolicy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowElements("u", "s", "strike", "del", "ins", "mark", "hr", "br", "div", "span", "sub", "sup", "figure", "figcaption")
	p.AllowStyles("text-align").Matching(regexp.MustCompile(`^(left|right|center|justify)$`)).
		OnElements("p", "div", "h1", "h2", "h3", "h4", "td", "th", "li", "blockquote")
	p.AllowAttrs("colspan", "rowspan").Matching(regexp.MustCompile(`^[0-9]{1,2}$`)).OnElements("td", "th")
	// The editor's Subtitle style, and text and highlight colours.
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^doc-subtitle$`)).OnElements("p")
	p.AllowStyles("color", "background-color").
		Matching(regexp.MustCompile(`^(#[0-9a-fA-F]{3,8}|rgba?\(\s*\d{1,3}\s*,\s*\d{1,3}\s*,\s*\d{1,3}\s*(,\s*[0-9.]+\s*)?\)|transparent)$`)).
		OnElements("span", "font", "mark")
	p.RequireNoFollowOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return p
}()

// SanitizeDoc makes editor HTML safe to store and show.
func SanitizeDoc(html string) string { return docPolicy.Sanitize(html) }

// DriveViewer is who is looking at the drive, and what they may do.
type DriveViewer struct {
	PlayerID   int64
	Faction    string
	Member     bool     // in the faction
	Panel      bool     // sees everything
	Editor     bool     // edits everything
	Leads      []string // divisions they command (edit those items)
	Divisions  []string // divisions they're in (read those items)
	Management bool     // edits as a staff override
}

// DriveViewerFor works out a viewer's drive access. audit and configure are
// the viewer's factions.audit and factions.configure staff permissions.
func DriveViewerFor(ctx context.Context, q dbtx, faction string, playerID int64, audit, configure bool) (DriveViewer, error) {
	v := DriveViewer{PlayerID: playerID, Faction: faction}
	if !Valid(faction) {
		return v, notAllowed("unknown faction")
	}
	c, ok, err := CommandIn(ctx, q, playerID, faction)
	if err != nil {
		return v, err
	}
	v.Member = c.Level > 0
	v.Panel = ok || audit || configure
	v.Editor = (ok && (c.Rank.IsCommand || c.Admin)) || configure
	v.Management = configure && !(ok && (c.Rank.IsCommand || c.Admin))
	v.Leads = c.Leads
	if v.Member {
		rows, err := q.Query(ctx, `SELECT division_key FROM faction_member_divisions WHERE faction = $1 AND player_id = $2`, faction, playerID)
		if err != nil {
			return v, err
		}
		for rows.Next() {
			var k string
			if err := rows.Scan(&k); err != nil {
				rows.Close()
				return v, err
			}
			v.Divisions = append(v.Divisions, k)
		}
		rows.Close()
	}
	return v, nil
}

// CanSee reports whether they may read an item or folder.
func (v DriveViewer) CanSee(division, visibility string) bool {
	if v.Panel {
		return true
	}
	return v.Member && visibility == "members" && (division == "" || slices.Contains(v.Divisions, division))
}

// CanEdit reports whether they may change an item or folder, or create one
// in division ("" = faction-wide).
func (v DriveViewer) CanEdit(division string) bool {
	return v.Editor || (division != "" && slices.Contains(v.Leads, division))
}

// CanCreateAnywhere reports whether they may create faction-wide items.
func (v DriveViewer) CanCreateAnywhere() bool { return v.Editor }

func (v DriveViewer) via() string {
	if v.Management {
		return ViaStaffOverride
	}
	return ViaCommand
}

// Folder is one drive folder.
type Folder struct {
	ID         int64
	ParentID   int64
	Name       string
	Division   string
	Visibility string
	Items      int
}

// Item is one document or file (without its contents).
type Item struct {
	ID         int64
	FolderID   int64
	Kind       string
	Title      string
	Category   string
	Division   string
	Visibility string
	FileName   string
	FileType   string
	FileSize   int
	CreatedBy  string
	UpdatedBy  string
	Created    time.Time
	Updated    time.Time
}

// CategoryLabel is the item's category name.
func (i Item) CategoryLabel() string { return CategoryLabel(i.Category) }

// SizeText is a file's size, e.g. "1.2 MB".
func (i Item) SizeText() string {
	switch {
	case i.FileSize >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(i.FileSize)/(1<<20))
	case i.FileSize >= 1<<10:
		return fmt.Sprintf("%d KB", i.FileSize>>10)
	}
	return fmt.Sprintf("%d B", i.FileSize)
}

// Inline reports whether the file is safe to show in the browser.
func (i Item) Inline() bool {
	return i.FileType == "application/pdf" || strings.HasPrefix(i.FileType, "image/")
}

const itemCols = `
	SELECT i.id, COALESCE(i.folder_id, 0), i.kind, i.title, i.category, COALESCE(i.division_key, ''), i.visibility,
	       i.file_name, i.file_type, i.file_size,
	       COALESCE(NULLIF(c.name, ''), NULLIF(c.steam_name, ''), 'Player #' || c.id, 'Unknown'), COALESCE(NULLIF(u.name, ''), NULLIF(u.steam_name, ''), 'Player #' || u.id, 'Unknown'),
	       i.created_at, i.updated_at
	FROM faction_drive_items i LEFT JOIN players c ON c.id = i.created_by LEFT JOIN players u ON u.id = i.updated_by`

func scanItems(rows pgx.Rows) ([]Item, error) {
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var x Item
		if err := rows.Scan(&x.ID, &x.FolderID, &x.Kind, &x.Title, &x.Category, &x.Division, &x.Visibility,
			&x.FileName, &x.FileType, &x.FileSize, &x.CreatedBy, &x.UpdatedBy, &x.Created, &x.Updated); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// DriveListing is one folder's contents.
type DriveListing struct {
	Folder  Folder // zero = the top level
	Path    []Folder
	Folders []Folder
	Items   []Item
}

// ListDrive lists a folder (0 = top level) as the viewer may see it.
// division filters to one division's items ("" = everything visible).
func ListDrive(ctx context.Context, q dbtx, v DriveViewer, folderID int64, division string) (DriveListing, error) {
	var out DriveListing
	if folderID > 0 {
		f, err := getFolder(ctx, q, v.Faction, folderID)
		if err != nil {
			return out, err
		}
		if !v.CanSee(f.Division, f.Visibility) {
			return out, notAllowed("you can't open that folder")
		}
		out.Folder = f
		for p := f; p.ParentID > 0; {
			parent, err := getFolder(ctx, q, v.Faction, p.ParentID)
			if err != nil {
				break
			}
			out.Path = append([]Folder{parent}, out.Path...)
			p = parent
		}
	}
	rows, err := q.Query(ctx, `
		SELECT f.id, COALESCE(f.parent_id, 0), f.name, COALESCE(f.division_key, ''), f.visibility,
		       (SELECT count(*) FROM faction_drive_items i WHERE i.folder_id = f.id AND i.archived_at IS NULL)
		FROM faction_drive_folders f
		WHERE f.faction = $1 AND f.archived_at IS NULL AND COALESCE(f.parent_id, 0) = $2 AND ($3 = '' OR f.division_key = $3)
		ORDER BY f.name`, v.Faction, folderID, division)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var f Folder
		if err := rows.Scan(&f.ID, &f.ParentID, &f.Name, &f.Division, &f.Visibility, &f.Items); err != nil {
			rows.Close()
			return out, err
		}
		if v.CanSee(f.Division, f.Visibility) {
			out.Folders = append(out.Folders, f)
		}
	}
	rows.Close()
	cond := `COALESCE(i.folder_id, 0) = $2`
	if division != "" && folderID == 0 {
		cond = `($2 = 0)` // a division's documents, wherever they're filed
	}
	rows, err = q.Query(ctx, itemCols+`
		WHERE i.faction = $1 AND i.archived_at IS NULL AND `+cond+` AND ($3 = '' OR i.division_key = $3)
		ORDER BY i.category, lower(i.title)`, v.Faction, folderID, division)
	if err != nil {
		return out, err
	}
	items, err := scanItems(rows)
	if err != nil {
		return out, err
	}
	for _, it := range items {
		if v.CanSee(it.Division, it.Visibility) {
			out.Items = append(out.Items, it)
		}
	}
	return out, nil
}

// RecentDrive lists recently updated items the viewer may see.
func RecentDrive(ctx context.Context, q dbtx, v DriveViewer, limit int) ([]Item, error) {
	rows, err := q.Query(ctx, itemCols+` WHERE i.faction = $1 AND i.archived_at IS NULL ORDER BY i.updated_at DESC LIMIT $2`, v.Faction, limit*3)
	if err != nil {
		return nil, err
	}
	items, err := scanItems(rows)
	if err != nil {
		return nil, err
	}
	var out []Item
	for _, it := range items {
		if v.CanSee(it.Division, it.Visibility) && len(out) < limit {
			out = append(out, it)
		}
	}
	return out, nil
}

func getFolder(ctx context.Context, q dbtx, faction string, id int64) (Folder, error) {
	var f Folder
	err := q.QueryRow(ctx, `SELECT id, COALESCE(parent_id, 0), name, COALESCE(division_key, ''), visibility FROM faction_drive_folders
		WHERE id = $1 AND faction = $2 AND archived_at IS NULL`, id, faction).Scan(&f.ID, &f.ParentID, &f.Name, &f.Division, &f.Visibility)
	if errors.Is(err, pgx.ErrNoRows) {
		return f, notAllowed("that folder doesn't exist")
	}
	return f, err
}

// GetItem loads an item and (for documents) its HTML.
func GetItem(ctx context.Context, q dbtx, v DriveViewer, id int64) (Item, string, error) {
	rows, err := q.Query(ctx, itemCols+` WHERE i.id = $1 AND i.faction = $2 AND i.archived_at IS NULL`, id, v.Faction)
	if err != nil {
		return Item{}, "", err
	}
	items, err := scanItems(rows)
	if err != nil {
		return Item{}, "", err
	}
	if len(items) == 0 {
		return Item{}, "", notAllowed("that item doesn't exist")
	}
	it := items[0]
	if !v.CanSee(it.Division, it.Visibility) {
		return it, "", notAllowed("you can't open that")
	}
	var body string
	if it.Kind == "doc" {
		if err := q.QueryRow(ctx, `SELECT body_html FROM faction_drive_items WHERE id = $1`, id).Scan(&body); err != nil {
			return it, "", err
		}
	}
	return it, body, nil
}

// FileData returns a file's bytes.
func FileData(ctx context.Context, q dbtx, v DriveViewer, id int64) (Item, []byte, error) {
	it, _, err := GetItem(ctx, q, v, id)
	if err != nil {
		return it, nil, err
	}
	if it.Kind != "file" {
		return it, nil, notAllowed("that isn't a file")
	}
	var data []byte
	err = q.QueryRow(ctx, `SELECT file_data FROM faction_drive_items WHERE id = $1`, id).Scan(&data)
	return it, data, err
}

// ItemInput is what's saved for a document or file.
type ItemInput struct {
	FolderID   int64
	Title      string
	Category   string
	Division   string
	Visibility string
	Body       string // documents
}

func (v DriveViewer) checkPlacement(ctx context.Context, q dbtx, in *ItemInput) error {
	var err error
	if in.Title, err = cleanText(in.Title, 120, "a title", true); err != nil {
		return err
	}
	if CategoryLabel(in.Category) == "Other" {
		in.Category = "other"
	}
	if in.Visibility != "command" {
		in.Visibility = "members"
	}
	if in.FolderID > 0 {
		f, err := getFolder(ctx, q, v.Faction, in.FolderID)
		if err != nil {
			return err
		}
		if in.Division == "" {
			in.Division = f.Division // items in a division's folder belong to it
		}
	}
	if in.Division != "" {
		var ok bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM faction_divisions WHERE faction = $1 AND key = $2)`, v.Faction, in.Division).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return notAllowed("unknown division")
		}
	}
	if !v.CanEdit(in.Division) {
		if in.Division == "" {
			return notAllowed("only command, Administration and cabinet can add faction-wide documents")
		}
		return notAllowed("only command, Administration, cabinet or that division's command can do that")
	}
	return nil
}

func driveLog(ctx context.Context, q dbtx, v DriveViewer, detail string) error {
	return logEvent(ctx, q, v.Faction, Actor{PlayerID: v.PlayerID, Via: v.via()}, 0, 0, "drive", detail, "")
}

// CreateDoc saves a new document and returns its ID.
func CreateDoc(ctx context.Context, pool *pgxpool.Pool, v DriveViewer, in ItemInput) (int64, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if err := v.checkPlacement(ctx, tx, &in); err != nil {
		return 0, err
	}
	body := SanitizeDoc(in.Body)
	if len(body) > maxBody {
		return 0, notAllowed("the document is too long")
	}
	var id int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO faction_drive_items (faction, folder_id, kind, title, category, division_key, visibility, body_html, created_by, updated_by)
		VALUES ($1, NULLIF($2, 0), 'doc', $3, $4, NULLIF($5, ''), $6, $7, $8, $8) RETURNING id`,
		v.Faction, in.FolderID, in.Title, in.Category, in.Division, in.Visibility, body, v.PlayerID).Scan(&id); err != nil {
		return 0, err
	}
	if err := driveLog(ctx, tx, v, fmt.Sprintf("Created %s %q", strings.ToLower(CategoryLabel(in.Category)), in.Title)); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

// SaveDoc updates a document, keeping the previous text as a version.
func SaveDoc(ctx context.Context, pool *pgxpool.Pool, v DriveViewer, id int64, in ItemInput) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var oldTitle, oldBody, oldDiv string
	err = tx.QueryRow(ctx, `SELECT title, body_html, COALESCE(division_key, '') FROM faction_drive_items WHERE id = $1 AND faction = $2 AND kind = 'doc' AND archived_at IS NULL FOR UPDATE`,
		id, v.Faction).Scan(&oldTitle, &oldBody, &oldDiv)
	if errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("that document doesn't exist")
	} else if err != nil {
		return err
	}
	if !v.CanEdit(oldDiv) {
		return notAllowed("you can't edit that document")
	}
	if err := v.checkPlacement(ctx, tx, &in); err != nil {
		return err
	}
	body := SanitizeDoc(in.Body)
	if len(body) > maxBody {
		return notAllowed("the document is too long")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO faction_drive_versions (item_id, title, body_html, edited_by) VALUES ($1, $2, $3, $4)`, id, oldTitle, oldBody, v.PlayerID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE faction_drive_items SET folder_id = NULLIF($2, 0), title = $3, category = $4, division_key = NULLIF($5, ''), visibility = $6,
		       body_html = $7, updated_by = $8, updated_at = now() WHERE id = $1`,
		id, in.FolderID, in.Title, in.Category, in.Division, in.Visibility, body, v.PlayerID); err != nil {
		return err
	}
	if err := driveLog(ctx, tx, v, fmt.Sprintf("Edited %q", in.Title)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UploadFile stores an uploaded file and returns its ID.
func UploadFile(ctx context.Context, pool *pgxpool.Pool, v DriveViewer, in ItemInput, name string, data []byte) (int64, error) {
	name = filepath.Base(strings.TrimSpace(name))
	ext := strings.ToLower(filepath.Ext(name))
	ctype, ok := allowedFiles[ext]
	if !ok {
		return 0, notAllowed("that file type isn't allowed (%s)", AllowedExtensions())
	}
	if len(data) == 0 {
		return 0, notAllowed("the file is empty")
	}
	if len(data) > MaxFileSize {
		return 0, notAllowed("files can be up to %d MB", MaxFileSize>>20)
	}
	// Check the bytes match the extension for types a browser would render.
	sniff := http.DetectContentType(data)
	if (strings.HasPrefix(ctype, "image/") || ctype == "application/pdf") && !strings.HasPrefix(sniff, ctype) {
		return 0, notAllowed("that file doesn't look like a %s", strings.TrimPrefix(ext, "."))
	}
	if in.Title == "" {
		in.Title = strings.TrimSuffix(name, filepath.Ext(name))
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if err := v.checkPlacement(ctx, tx, &in); err != nil {
		return 0, err
	}
	var id int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO faction_drive_items (faction, folder_id, kind, title, category, division_key, visibility, file_name, file_type, file_size, file_data, created_by, updated_by)
		VALUES ($1, NULLIF($2, 0), 'file', $3, $4, NULLIF($5, ''), $6, $7, $8, $9, $10, $11, $11) RETURNING id`,
		v.Faction, in.FolderID, in.Title, in.Category, in.Division, in.Visibility, name, ctype, len(data), data, v.PlayerID).Scan(&id); err != nil {
		return 0, err
	}
	if err := driveLog(ctx, tx, v, fmt.Sprintf("Uploaded %q", name)); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

// ArchiveItem removes an item from the drive (kept, not deleted).
func ArchiveItem(ctx context.Context, pool *pgxpool.Pool, v DriveViewer, id int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var title, div string
	err = tx.QueryRow(ctx, `SELECT title, COALESCE(division_key, '') FROM faction_drive_items WHERE id = $1 AND faction = $2 AND archived_at IS NULL FOR UPDATE`,
		id, v.Faction).Scan(&title, &div)
	if errors.Is(err, pgx.ErrNoRows) {
		return notAllowed("that item doesn't exist")
	} else if err != nil {
		return err
	}
	if !v.CanEdit(div) {
		return notAllowed("you can't remove that")
	}
	if _, err := tx.Exec(ctx, `UPDATE faction_drive_items SET archived_at = now(), updated_by = $2 WHERE id = $1`, id, v.PlayerID); err != nil {
		return err
	}
	if err := driveLog(ctx, tx, v, fmt.Sprintf("Removed %q", title)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CreateFolder adds a folder and returns its ID.
func CreateFolder(ctx context.Context, pool *pgxpool.Pool, v DriveViewer, parentID int64, name, division, visibility string) (int64, error) {
	var err error
	if name, err = cleanText(name, 80, "a folder name", true); err != nil {
		return 0, err
	}
	in := ItemInput{FolderID: parentID, Title: name, Division: division, Visibility: visibility}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if err := v.checkPlacement(ctx, tx, &in); err != nil {
		return 0, err
	}
	var id int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO faction_drive_folders (faction, parent_id, name, division_key, visibility, created_by)
		VALUES ($1, NULLIF($2, 0), $3, NULLIF($4, ''), $5, $6) RETURNING id`,
		v.Faction, parentID, name, in.Division, in.Visibility, v.PlayerID).Scan(&id); err != nil {
		return 0, err
	}
	if err := driveLog(ctx, tx, v, fmt.Sprintf("Created folder %q", name)); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

// ArchiveFolder removes an empty folder.
func ArchiveFolder(ctx context.Context, pool *pgxpool.Pool, v DriveViewer, id int64) error {
	f, err := getFolder(ctx, pool, v.Faction, id)
	if err != nil {
		return err
	}
	if !v.CanEdit(f.Division) {
		return notAllowed("you can't remove that folder")
	}
	var n int
	if err := pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM faction_drive_items WHERE folder_id = $1 AND archived_at IS NULL)
		     + (SELECT count(*) FROM faction_drive_folders WHERE parent_id = $1 AND archived_at IS NULL)`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return notAllowed("the folder isn't empty")
	}
	if _, err := pool.Exec(ctx, `UPDATE faction_drive_folders SET archived_at = now() WHERE id = $1`, id); err != nil {
		return err
	}
	return driveLog(ctx, pool, v, fmt.Sprintf("Removed folder %q", f.Name))
}

// Version is an earlier copy of a document.
type Version struct {
	ID    int64
	Title string
	By    string
	At    time.Time
	Body  string
}

// Versions lists a document's earlier copies, newest first (without text).
func Versions(ctx context.Context, q dbtx, v DriveViewer, id int64) ([]Version, error) {
	if _, _, err := GetItem(ctx, q, v, id); err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, `
		SELECT dv.id, dv.title, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id, 'Unknown'), dv.created_at
		FROM faction_drive_versions dv LEFT JOIN players p ON p.id = dv.edited_by
		WHERE dv.item_id = $1 ORDER BY dv.id DESC LIMIT 100`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		var x Version
		if err := rows.Scan(&x.ID, &x.Title, &x.By, &x.At); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// GetVersion loads one earlier copy of a document.
func GetVersion(ctx context.Context, q dbtx, v DriveViewer, id, versionID int64) (Version, error) {
	var x Version
	if _, _, err := GetItem(ctx, q, v, id); err != nil {
		return x, err
	}
	err := q.QueryRow(ctx, `
		SELECT dv.id, dv.title, COALESCE(NULLIF(p.name, ''), NULLIF(p.steam_name, ''), 'Player #' || p.id, 'Unknown'), dv.created_at, dv.body_html
		FROM faction_drive_versions dv LEFT JOIN players p ON p.id = dv.edited_by WHERE dv.id = $1 AND dv.item_id = $2`,
		versionID, id).Scan(&x.ID, &x.Title, &x.By, &x.At, &x.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return x, notAllowed("that version doesn't exist")
	}
	return x, err
}

// DocTemplates are starting points for new documents.
var DocTemplates = map[string]struct {
	Title, Category, Body string
}{
	"sop": {"New SOP", "sop", `<h1>Standard Operating Procedure</h1>
<p><strong>Applies to:</strong> </p><p><strong>Owner:</strong> </p><p><strong>Last reviewed:</strong> </p>
<h2>1. Purpose</h2><p>What this procedure is for.</p>
<h2>2. Scope</h2><p>Who and what it covers.</p>
<h2>3. Procedure</h2><ol><li>First step.</li><li>Second step.</li><li>Third step.</li></ol>
<h2>4. Radio and communication</h2><p>Callouts, channels and wording.</p>
<h2>5. Dos and don'ts</h2><ul><li>Do …</li><li>Don't …</li></ul>
<h2>6. Breaches</h2><p>Breaking this SOP is handled under Discipline (“Not following faction or department SOPs”).</p>`},
	"training": {"New training document", "training", `<h1>Training: </h1>
<p><strong>Qualification:</strong> </p><p><strong>Trainer:</strong> </p><p><strong>Duration:</strong> </p>
<h2>Learning outcomes</h2><ul><li>By the end, the trainee can …</li></ul>
<h2>Before the session</h2><p>Kit, location and anything the trainee should read.</p>
<h2>Session plan</h2><table><thead><tr><th>Part</th><th>Content</th><th>Time</th></tr></thead><tbody><tr><td>1</td><td>Briefing</td><td>5 min</td></tr><tr><td>2</td><td>Practical</td><td>20 min</td></tr><tr><td>3</td><td>Assessment</td><td>10 min</td></tr></tbody></table>
<h2>Assessment</h2><p>What counts as a pass. Record passes on the member's profile.</p>`},
	"blank": {"Untitled document", "other", `<p></p>`},
}
