package main

import (
	"os"
	"path/filepath"
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

	warnings := deployExtension(repoRoot, armaPath)
	t.Logf("deployExtension warnings: %v", warnings)
}
