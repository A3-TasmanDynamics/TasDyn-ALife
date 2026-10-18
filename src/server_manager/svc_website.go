package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// The website and the Discord bot are one Go program (src/website) run
// twice: `-mode web` and `-mode bot` (see src/website/main.go), so the bot
// can restart without taking the site down and a bot crash can't kill it.
//
// Each start rebuilds the binary from source first -- like Arma's mission
// deploy, a source change always takes effect on the next start. If the
// build fails (or Go isn't installed) the previous build runs instead, with
// a warning. Each mode has its own .exe so one can be rebuilt while the
// other is running.

// websiteDir is src/website: its .env, web/templates and web/static are
// read relative to it, so it's also the working directory.
func websiteDir() (string, error) {
	cfg, _ := loadServicesConfig()
	if cfg.WebsiteDir != "" {
		return cfg.WebsiteDir, nil
	}
	root, err := findRepoRoot()
	if err != nil {
		return "", fmt.Errorf("locate the website source: %w", err)
	}
	return filepath.Join(root, "src", "website"), nil
}

// readDotEnv reads KEY=VALUE lines from the website's .env. Used only to
// find addresses and check settings exist -- values are never shown.
func readDotEnv(dir string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(filepath.Join(dir, ".env"))
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}

// websiteExe is where buildWebsite puts the binary for a mode.
func websiteExe(mode string) string {
	appDir, err := appDataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(appDir, "bin", "website-"+mode+".exe")
}

// buildWebsite compiles src/website to <AppData>/bin/website-<mode>.exe.
func buildWebsite(dir, mode string) (exe string, warnings []string, err error) {
	exe = websiteExe(mode)
	if exe == "" {
		return "", nil, fmt.Errorf("no AppData folder for the website build")
	}
	_ = os.MkdirAll(filepath.Dir(exe), 0o755)

	_, haveOld := os.Stat(exe)
	goExe, lookErr := exec.LookPath("go")
	if lookErr != nil {
		if haveOld == nil {
			return exe, []string{"Go isn't installed, so the website wasn't rebuilt -- running the previous build"}, nil
		}
		return "", nil, fmt.Errorf("Go isn't installed (needed to build the website): %w", lookErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, goExe, "build", "-o", exe, ".")
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if len(msg) > 600 {
			msg = msg[:600] + "..."
		}
		if haveOld == nil {
			return exe, []string{"Website build failed, running the previous build: " + msg}, nil
		}
		return "", nil, fmt.Errorf("website build failed: %s", msg)
	}
	return exe, nil, nil
}

func websiteStart(mode string) func() (launchSpec, error) {
	return func() (launchSpec, error) {
		dir, err := websiteDir()
		if err != nil {
			return launchSpec{}, err
		}
		if _, err := os.Stat(filepath.Join(dir, "main.go")); err != nil {
			return launchSpec{}, fmt.Errorf("no website source at %s", dir)
		}
		env := readDotEnv(dir)
		if mode == "bot" && env["DISCORD_BOT_TOKEN"] == "" && os.Getenv("DISCORD_BOT_TOKEN") == "" {
			return launchSpec{}, fmt.Errorf("DISCORD_BOT_TOKEN isn't set in %s", filepath.Join(dir, ".env"))
		}
		exe, warnings, err := buildWebsite(dir, mode)
		if err != nil {
			return launchSpec{}, err
		}
		cmd := exec.Command(exe, "-mode", mode)
		cmd.Dir = dir
		return launchSpec{Cmd: cmd, Warnings: warnings, LogToFile: true, Hidden: true}, nil
	}
}

var healthClient = &http.Client{Timeout: 3 * time.Second}

// websiteHealth: does the site answer on its LISTEN_ADDR?
func websiteHealth() string {
	dir, err := websiteDir()
	if err != nil {
		return ""
	}
	addr := readDotEnv(dir)["LISTEN_ADDR"]
	if addr == "" {
		addr = ":8080"
	}
	url := "http://" + addr + "/"
	if strings.HasPrefix(addr, ":") {
		url = "http://127.0.0.1" + addr + "/"
	}
	start := time.Now()
	resp, err := healthClient.Get(url)
	if err != nil {
		return "Not answering on " + strings.TrimPrefix(url, "http://")
	}
	resp.Body.Close()
	return fmt.Sprintf("HTTP %d on %s · %d ms", resp.StatusCode, strings.TrimSuffix(strings.TrimPrefix(url, "http://"), "/"), time.Since(start).Milliseconds())
}

// botHealth reads the bot process's health endpoint (BOT_HEALTH_ADDR).
func botHealth() string {
	dir, err := websiteDir()
	if err != nil {
		return ""
	}
	addr := readDotEnv(dir)["BOT_HEALTH_ADDR"]
	if addr == "" {
		addr = "127.0.0.1:8096"
	}
	resp, err := healthClient.Get("http://" + addr + "/health")
	if err != nil {
		return "Starting up, or health endpoint not answering"
	}
	defer resp.Body.Close()
	var h struct {
		Ready     bool  `json:"ready"`
		LatencyMS int64 `json:"latencyMs"`
	}
	if json.NewDecoder(resp.Body).Decode(&h) != nil {
		return "Unexpected health reply"
	}
	if !h.Ready {
		return "Not connected to Discord"
	}
	return fmt.Sprintf("Connected to Discord · %d ms", h.LatencyMS)
}
