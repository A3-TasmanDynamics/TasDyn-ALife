package factions

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The faction drive, on EMS. Runs only with TEST_DATABASE_URL and only when
// no EMS ranks are configured (it sets its own and removes them).
func TestDrive(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var n int
	pool.QueryRow(ctx, `SELECT count(*) FROM faction_rank_names WHERE faction = 'ems'`).Scan(&n)
	if n > 0 {
		t.Skip("EMS ranks already configured on this database")
	}
	const f = "ems"
	must := func(what string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	denied := func(what string, err error) {
		t.Helper()
		if !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%s: expected refusal, got %v", what, err)
		}
	}
	must("ranks", func() error {
		_, err := pool.Exec(ctx, `INSERT INTO faction_rank_names (faction, level, name, promote_up_to, is_command) VALUES
			('ems', 1, 'Trainee', 0, false), ('ems', 2, 'Paramedic', 0, false), ('ems', 4, 'Supervisor', 2, true)`)
		return err
	}())
	must("division", func() error {
		_, err := pool.Exec(ctx, `INSERT INTO faction_divisions (faction, key, name, roles) VALUES ('ems', 'TD1', 'Test Div', '{Lead,Second,Member}')`)
		return err
	}())
	ids := map[string]int64{}
	for name, lvl := range map[string]int{"cmdr": 4, "lead": 2, "member": 1, "other": 1, "outsider": 0} {
		uid := map[string]string{"cmdr": "76561190000000281", "lead": "76561190000000282", "member": "76561190000000283",
			"other": "76561190000000284", "outsider": "76561190000000285"}[name]
		var id int64
		must("player", pool.QueryRow(ctx, `INSERT INTO players (uid, name, medic_level) VALUES ($1, $2, $3) RETURNING id`, uid, "t_"+name, lvl).Scan(&id))
		ids[name] = id
	}
	pool.Exec(ctx, `INSERT INTO faction_member_divisions (faction, player_id, division_key, role) VALUES ('ems', $1, 'TD1', 'Lead'), ('ems', $2, 'TD1', 'Member')`, ids["lead"], ids["member"])
	t.Cleanup(func() {
		all := []int64{}
		for _, id := range ids {
			all = append(all, id)
		}
		pool.Exec(ctx, `DELETE FROM faction_drive_items WHERE faction = 'ems'`)
		pool.Exec(ctx, `DELETE FROM faction_drive_folders WHERE faction = 'ems'`)
		pool.Exec(ctx, `DELETE FROM faction_log WHERE actor_id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM players WHERE id = ANY($1)`, all)
		pool.Exec(ctx, `DELETE FROM faction_divisions WHERE faction = 'ems' AND key = 'TD1'`)
		pool.Exec(ctx, `DELETE FROM faction_rank_names WHERE faction = 'ems'`)
	})
	viewer := func(who string, audit, configure bool) DriveViewer {
		v, err := DriveViewerFor(ctx, pool, f, ids[who], audit, configure)
		must("viewer "+who, err)
		return v
	}
	cmdr, lead, member, other, outsider := viewer("cmdr", false, false), viewer("lead", false, false), viewer("member", false, false), viewer("other", false, false), viewer("outsider", false, false)
	mgmt := viewer("outsider", true, true)

	// Cleaning: scripts and handlers go; formatting, tables and alignment stay.
	dirty := `<h2 style="text-align:center">SOP</h2><p onclick="x()">Hi <script>alert(1)</script><strong>bold</strong></p><table><tr><td colspan="2">a</td></tr></table><img src=x onerror=alert(1)><a href="javascript:alert(1)">x</a>`
	clean := SanitizeDoc(dirty)
	for _, bad := range []string{"<script", "onclick", "onerror", "javascript:"} {
		if strings.Contains(clean, bad) {
			t.Errorf("sanitised HTML still contains %q: %s", bad, clean)
		}
	}
	for _, good := range []string{`text-align:center`, "<strong>bold</strong>", `colspan="2"`, "<table>"} {
		if !strings.Contains(strings.ReplaceAll(clean, " ", ""), strings.ReplaceAll(good, " ", "")) {
			t.Errorf("sanitised HTML lost %q: %s", good, clean)
		}
	}

	// Who can create what.
	sop, err := CreateDoc(ctx, pool, cmdr, ItemInput{Title: "Radio SOP", Category: "sop", Visibility: "members", Body: dirty})
	must("command creates a faction-wide SOP", err)
	secret, err := CreateDoc(ctx, pool, cmdr, ItemInput{Title: "Command notes", Visibility: "command", Body: "<p>x</p>"})
	must("command creates a command-only doc", err)
	_, err = CreateDoc(ctx, pool, lead, ItemInput{Title: "x", Body: "<p>x</p>"})
	denied("division command creates a faction-wide doc", err)
	divDoc, err := CreateDoc(ctx, pool, lead, ItemInput{Title: "TD1 training", Category: "training", Division: "TD1", Body: "<p>train</p>"})
	must("division command creates in their division", err)
	_, err = CreateDoc(ctx, pool, member, ItemInput{Title: "x", Division: "TD1", Body: "<p>x</p>"})
	denied("member creates", err)
	_, err = CreateDoc(ctx, pool, mgmt, ItemInput{Title: "Management memo", Body: "<p>x</p>"})
	must("Management creates", err)

	// Who can read what.
	if _, body, err := GetItem(ctx, pool, member, sop); err != nil || strings.Contains(body, "<script") {
		t.Errorf("member reads the SOP (sanitised on save): %v %q", err, body)
	}
	_, _, err = GetItem(ctx, pool, member, secret)
	denied("member reads a command-only doc", err)
	_, _, err = GetItem(ctx, pool, member, divDoc)
	must("division member reads their division's doc", err)
	_, _, err = GetItem(ctx, pool, other, divDoc)
	denied("non-division member reads a division doc", err)
	_, _, err = GetItem(ctx, pool, outsider, sop)
	denied("non-member reads", err)
	l, err := ListDrive(ctx, pool, other, 0, "")
	must("list", err)
	if len(l.Items) != 2 { // the SOP and the Management memo
		t.Errorf("other member should see 2 items, got %d", len(l.Items))
	}

	// Saving keeps a version.
	must("save", SaveDoc(ctx, pool, cmdr, sop, ItemInput{Title: "Radio SOP v2", Category: "sop", Visibility: "members", Body: "<p>new</p>"}))
	vers, err := Versions(ctx, pool, cmdr, sop)
	must("versions", err)
	if len(vers) != 1 || vers[0].Title != "Radio SOP" {
		t.Fatalf("want one earlier version titled Radio SOP, got %+v", vers)
	}
	old, err := GetVersion(ctx, pool, cmdr, sop, vers[0].ID)
	must("get version", err)
	if !strings.Contains(old.Body, "SOP") {
		t.Errorf("earlier version should hold the old text, got %q", old.Body)
	}
	denied("member edits", SaveDoc(ctx, pool, member, sop, ItemInput{Title: "x", Body: "x"}))
	denied("division command edits a faction-wide doc", SaveDoc(ctx, pool, lead, sop, ItemInput{Title: "x", Body: "x"}))

	// Folders and uploads.
	folder, err := CreateFolder(ctx, pool, cmdr, 0, "Training", "", "members")
	must("folder", err)
	_, err = UploadFile(ctx, pool, cmdr, ItemInput{FolderID: folder}, "evil.exe", []byte("MZ"))
	denied("upload an .exe", err)
	_, err = UploadFile(ctx, pool, cmdr, ItemInput{FolderID: folder}, "fake.png", []byte("<html>not a png</html>"))
	denied("upload a fake image", err)
	fileID, err := UploadFile(ctx, pool, cmdr, ItemInput{FolderID: folder}, "notes.txt", []byte("hello"))
	must("upload a text file", err)
	if it, data, err := FileData(ctx, pool, member, fileID); err != nil || string(data) != "hello" || it.Title != "notes" {
		t.Errorf("member downloads the file: %v %q %+v", err, data, it)
	}
	denied("remove a non-empty folder", ArchiveFolder(ctx, pool, cmdr, folder))
	must("remove the file", ArchiveItem(ctx, pool, cmdr, fileID))
	must("remove the empty folder", ArchiveFolder(ctx, pool, cmdr, folder))
	_, _, err = GetItem(ctx, pool, cmdr, fileID)
	denied("removed items are gone from the drive", err)
	var logged int
	pool.QueryRow(ctx, `SELECT count(*) FROM faction_log WHERE faction = 'ems' AND kind = 'drive'`).Scan(&logged)
	if logged < 6 {
		t.Errorf("drive changes should be in the Command log, got %d", logged)
	}
}
