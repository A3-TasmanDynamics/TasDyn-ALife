package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// runtimeDeps mirrors tools/test_local_server.ps1's $runtimeDeps exactly --
// the C++ extension DLL plus the dependency DLLs build.ps1 copies next to
// it (discovered as a real missing-dependency bug via dumpbin /dependents
// during that script's own development). Kept in one place conceptually,
// duplicated here because this is Go and that's PowerShell -- see that
// script if this list ever needs to change, and change both.
var runtimeDeps = []string{
	"tasdyn_alife_x64.dll", "libpq.dll", "libssl-3-x64.dll",
	"libcrypto-3-x64.dll", "libintl-9.dll", "libwinpthread-1.dll", "libiconv-2.dll",
}

// findRepoRoot locates the repository root relative to this running
// executable, by walking up from its own directory until it finds
// database/schema.sql -- a file that's always at the repo root and
// unlikely to ever move. Needed because server_manager.exe lives at
// src/server_manager/build/bin/server_manager.exe, and the mission source
// (src/ALife.Altis) and C++ extension build output
// (src/cpp_extension/build) it needs to deploy live elsewhere in the same
// repo, not somewhere fixed relative to the install location of a
// packaged app -- this tool is still run from inside a repo checkout, not
// installed standalone.
func findRepoRoot() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate own executable: %w", err)
	}

	dir := filepath.Dir(exe)
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "database", "schema.sql")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not find repo root (looked for database/schema.sql above %s)", filepath.Dir(exe))
}

// deployMission copies src/ALife.Altis into
// <armaServerPath>/mpmissions/<missionTemplate>, replacing whatever's
// there -- this is the step that was missing before: server.cfg named the
// mission template, but nothing had ever actually put a folder by that
// name into mpmissions, so Arma had nothing to load. Deployed fresh on
// every launch (not just when missing), same as
// tools/test_local_server.ps1, so a source change always takes effect on
// the next launch rather than silently testing a stale copy.
func deployMission(repoRoot, armaServerPath, missionTemplate string) error {
	missionSrc := filepath.Join(repoRoot, "src", "ALife.Altis")
	if _, err := os.Stat(missionSrc); err != nil {
		return fmt.Errorf("mission source not found at %s", missionSrc)
	}

	missionDest := filepath.Join(armaServerPath, "mpmissions", missionTemplate)
	if err := os.RemoveAll(missionDest); err != nil {
		return fmt.Errorf("clear existing mpmissions/%s: %w", missionTemplate, err)
	}
	if err := os.CopyFS(missionDest, os.DirFS(missionSrc)); err != nil {
		return fmt.Errorf("copy mission to mpmissions/%s: %w", missionTemplate, err)
	}

	return nil
}

// deployExtension best-effort copies the built C++ extension DLL, its
// runtime dependencies, and config.ini into the Arma 3 Server install.
// Unlike deployMission, this never blocks a launch -- a user testing
// mission/mission-config changes shouldn't be stopped by not having run
// cpp_extension/build.ps1 yet, since the mission still loads without it
// (only ALife_fnc_load/save calls would fail once a player actually
// joins). Returns human-readable warnings for anything it couldn't copy,
// for the caller to surface rather than fail on.
func deployExtension(repoRoot, armaServerPath string) []string {
	var warnings []string

	extensionBuild := filepath.Join(repoRoot, "src", "cpp_extension", "build")
	for _, dep := range runtimeDeps {
		src := filepath.Join(extensionBuild, dep)
		if _, err := os.Stat(src); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s not found in src/cpp_extension/build -- run src/cpp_extension/build.ps1 if you need database save/load", dep))
			continue
		}
		dest := filepath.Join(armaServerPath, dep)
		if err := copyFile(src, dest); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to copy %s: %v", dep, err))
		}
	}

	configSrc := filepath.Join(repoRoot, "config.ini")
	if _, err := os.Stat(configSrc); err != nil {
		warnings = append(warnings, "config.ini not found at repo root -- copy config.ini.example and fill in real values if you need database save/load")
	} else if err := copyFile(configSrc, filepath.Join(armaServerPath, "config.ini")); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to copy config.ini: %v", err))
	}

	return warnings
}

func copyFile(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}
