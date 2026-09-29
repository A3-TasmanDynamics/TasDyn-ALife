package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestControlAPIAuthAndStatus(t *testing.T) {
	a := NewApp()
	srv := httptest.NewServer(a.controlMux(func() string { return "secret-token" }))
	defer srv.Close()

	do := func(method, path, token, body string) *http.Response {
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	for _, tok := range []string{"", "wrong"} {
		if resp := do("GET", "/api/status", tok, ""); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("token %q: status %d, want 401", tok, resp.StatusCode)
		}
	}
	resp := do("GET", "/api/status", "secret-token", "")
	var st ControlStatus
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil || resp.StatusCode != 200 || st.Running {
		t.Fatalf("status: %d %+v %v", resp.StatusCode, st, err)
	}
	// Restarting a server that isn't running is refused, not attempted.
	resp = do("POST", "/api/restart", "secret-token", `{"delaySeconds": 60}`)
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode != http.StatusConflict || out["ok"] != false {
		t.Errorf("restart while stopped: %d %v", resp.StatusCode, out)
	}
	if resp := do("POST", "/api/restart/cancel", "secret-token", ""); resp.StatusCode != http.StatusConflict {
		t.Errorf("cancel with nothing scheduled: %d", resp.StatusCode)
	}
	if resp := do("GET", "/api/logs?lines=5", "secret-token", ""); resp.StatusCode != 200 {
		t.Errorf("logs: %d", resp.StatusCode)
	}
}
