package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Integration test against a real local Arma 3 Server install -- verifies
// LaunchServer's deploy step (deploy.go) actually puts a mission folder
// where Arma will look for it. Skipped by default (needs a real install);
// run explicitly:
//
//	ALIFE_TEST_ARMA_PATH="G:\SteamLibrary\steamapps\common\Arma 3 Server" go test ./... -run TestDeployMission -v
func TestDeployMission(t *testing.T) {
	armaPath := os.Getenv("ALIFE_TEST_ARMA_PATH")
	if armaPath == "" {
		t.Skip("set ALIFE_TEST_ARMA_PATH to a real Arma 3 Server install to run this")
	}

	// findRepoRoot() itself isn't testable here: it locates the repo via
	// os.Executable(), which under `go test` is a temp-built test binary
	// under a Go build cache dir, not the real server_manager.exe location
	// -- that's a real difference between test and production, not a test
	// bug to work around. Verified separately in the deployed-app section
	// below. deployMission/deployExtension take repoRoot as a plain
	// parameter and don't care how the caller found it, so they're
	// testable directly with a repo root derived from `go test`'s own
	// working directory (always the package dir, src/server_manager).
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	repoRoot := filepath.Join(wd, "..", "..")
	t.Logf("repo root: %s", repoRoot)

	if _, err := os.Stat(filepath.Join(repoRoot, "src", "ALife.Altis", "mission.sqm")); err != nil {
		t.Fatalf("sanity check failed: repo root doesn't look right (%v)", err)
	}

	missionDest := filepath.Join(armaPath, "mpmissions", "ALife.Altis")
	defer os.RemoveAll(missionDest) // leave the install clean afterward, this is a test run

	if err := deployMission(repoRoot, armaPath, "ALife.Altis"); err != nil {
		t.Fatalf("deployMission: %v", err)
	}

	for _, want := range []string{"mission.sqm", "description.ext", "CfgFunctions.hpp", "functions", "config"} {
		if _, err := os.Stat(filepath.Join(missionDest, want)); err != nil {
			t.Errorf("expected %s to have been deployed, but: %v", want, err)
		}
	}

	warnings := deployExtension(repoRoot, armaPath, "")
	t.Logf("deployExtension warnings: %v", warnings)
}

func TestWriteExtensionConfig(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.ini")
	dest := filepath.Join(dir, "out.ini")
	in := "[database]\r\nhost=127.0.0.1\r\ndbname=alife_db\r\n\r\n[server]\r\ncommand_password=old\r\n"
	if err := os.WriteFile(src, []byte(in), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeExtensionConfig(src, dest, "s3cret"); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(dest)
	got := string(out)
	if !strings.Contains(got, "dbname=alife_db") {
		t.Errorf("database section lost: %q", got)
	}
	if strings.Contains(got, "old") || strings.Count(got, "[server]") != 1 || !strings.Contains(got, "command_password=s3cret") {
		t.Errorf("server section not replaced: %q", got)
	}
	if err := writeExtensionConfig(src, dest, ""); err != nil {
		t.Fatal(err)
	}
	out, _ = os.ReadFile(dest)
	if strings.Contains(string(out), "[server]") {
		t.Errorf("no password should mean no [server] section: %q", out)
	}
}
