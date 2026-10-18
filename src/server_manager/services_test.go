package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Supervisor tests, with throwaway processes (cmd.exe / ping) instead of
// the real services, and a temporary manager folder (ALIFE_MANAGER_DIR) so
// the real services.json / state.json / logs are never touched.

func testService(t *testing.T, key string, restart bool, args ...string) *service {
	t.Helper()
	return &service{
		Key: key, Title: key,
		start: func() (launchSpec, error) {
			return launchSpec{Cmd: exec.Command("cmd", append([]string{"/c"}, args...)...), LogToFile: true, Hidden: true}, nil
		},
		cfg: func(ServicesConfig) ServiceConfig { return ServiceConfig{RestartOnCrash: restart} },
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func useTempManagerDir(t *testing.T) {
	t.Helper()
	t.Setenv("ALIFE_MANAGER_DIR", t.TempDir())
}

func TestServiceStopIsNotACrash(t *testing.T) {
	useTempManagerDir(t)
	s := testService(t, "sleeper", true, "ping", "-n", "60", "127.0.0.1")
	if _, err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); st.State != StateRunning || st.PID == 0 {
		t.Fatalf("after start: %+v", st)
	}
	if rec, ok := loadRunState()["sleeper"]; !ok || rec.PID != s.Status().PID {
		t.Fatalf("PID not saved to state.json: %+v", rec)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	st := s.Status()
	if st.State != StateStopped || st.RecentCrashes != 0 || st.LastError != "" {
		t.Fatalf("a stop counted as a crash: %+v", st)
	}
	if _, ok := loadRunState()["sleeper"]; ok {
		t.Fatal("state.json still lists a stopped service")
	}
}

func TestServiceLogsOutput(t *testing.T) {
	useTempManagerDir(t)
	s := testService(t, "echo", false, "echo", "hello-from-service")
	if _, err := s.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "exit", func() bool { return !s.Running() })
	got := strings.Join(tailFile(s.logPath(), 10), "\n")
	if !strings.Contains(got, "hello-from-service") {
		t.Fatalf("log missing output: %q", got)
	}
	// RestartOnCrash off: it stays stopped, with the reason.
	if st := s.Status(); st.State != StateStopped || !strings.Contains(st.LastError, "unexpectedly") {
		t.Fatalf("after exit: %+v", st)
	}
}

func TestServiceCrashLoopGuard(t *testing.T) {
	useTempManagerDir(t)
	s := testService(t, "crasher", true, "exit", "1")
	if _, err := s.Start(); err != nil {
		t.Fatal(err)
	}
	// Crashes, restarts after 5s, crashes, restarts after 10s, crashes:
	// three in the window, so it gives up.
	waitFor(t, "crash loop", func() bool { return s.Status().State == StateCrashLoop })
	st := s.Status()
	if st.RecentCrashes != maxCrashes || st.PID != 0 {
		t.Fatalf("crash loop: %+v", st)
	}
	// A manual start clears the guard.
	s.start = testService(t, "crasher", true, "ping", "-n", "60", "127.0.0.1").start
	if _, err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); st.State != StateRunning || st.RecentCrashes != 0 {
		t.Fatalf("manual start after crash loop: %+v", st)
	}
	_ = s.Stop()
}

func TestServiceReattach(t *testing.T) {
	useTempManagerDir(t)
	// The first session's own watcher sees the kill as a crash; with
	// restarts off it just reports it.
	first := testService(t, "keeper", false, "ping", "-n", "60", "127.0.0.1")
	if _, err := first.Start(); err != nil {
		t.Fatal(err)
	}
	pid := first.Status().PID
	// A new manager session: a fresh service value, same key.
	second := testService(t, "keeper", true)
	rec, ok := loadRunState()["keeper"]
	if !ok || !second.adopt(rec) {
		t.Fatal("didn't re-attach to the running process")
	}
	if second.Status().PID != pid {
		t.Fatalf("re-attached to PID %d, want %d", second.Status().PID, pid)
	}
	// A stale record (PID reused by something else) is refused.
	if second2 := testService(t, "keeper", true); second2.adopt(runRecord{PID: pid, Exe: `C:\nope\other.exe`}) {
		t.Fatal("re-attached to a PID whose program doesn't match")
	}
	if err := second.Stop(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "first to notice", func() bool { return !first.Running() })
}

func TestTS3Escaping(t *testing.T) {
	in := `a b|c/d\e`
	if got := ts3Unescape(ts3Escape(in)); got != in {
		t.Fatalf("round trip: %q", got)
	}
	kv := ts3Parse(`virtualserver_name=TasDyn\sALife virtualserver_clientsonline=3`)
	if kv["virtualserver_name"] != "TasDyn ALife" || kv["virtualserver_clientsonline"] != "3" {
		t.Fatalf("parse: %v", kv)
	}
}
