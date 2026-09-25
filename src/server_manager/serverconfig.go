package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ServerConfig is the subset of Arma 3's server.cfg this app actually
// exposes -- the fields the launch task named explicitly (name, password,
// admin password, slots), not a full server.cfg editor. See
// tools/test_local_server.ps1 for the throwaway dev-test config this
// project already had; this is the same shape made editable instead of
// hardcoded, for actually running a real server, not just smoke-testing.
type ServerConfig struct {
	Hostname        string `json:"hostname"`
	Password        string `json:"password"`
	PasswordAdmin   string `json:"passwordAdmin"`
	MaxPlayers      int    `json:"maxPlayers"`
	MissionTemplate string `json:"missionTemplate"` // e.g. "ALife.Altis"
	BattlEye        bool   `json:"battlEye"`
	VerifySignatures bool  `json:"verifySignatures"`
}

func defaultServerConfig() ServerConfig {
	return ServerConfig{
		Hostname:        "TasDyn-ALife",
		MaxPlayers:      32,
		MissionTemplate: "ALife.Altis",
		BattlEye:        true,  // matches docs/ANTI_CHEAT.md Layer 0 -- on by default, not off
		VerifySignatures: true,
	}
}

func serverConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "TasDyn-ALife-ServerManager")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "server_config.json"), nil
}

func loadServerConfig() (ServerConfig, error) {
	path, err := serverConfigPath()
	if err != nil {
		return defaultServerConfig(), err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaultServerConfig(), nil
	}
	if err != nil {
		return defaultServerConfig(), err
	}

	var c ServerConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return defaultServerConfig(), err
	}
	return c, nil
}

func saveServerConfigToDisk(c ServerConfig) error {
	path, err := serverConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// renderServerCfg produces the actual Arma 3 server.cfg text. Booleans
// render as 0/1 -- that's server.cfg's own convention, verified against
// the same reference used for tools/test_local_server.ps1's config.
func renderServerCfg(c ServerConfig) string {
	boolInt := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}

	return fmt.Sprintf(`hostname = "%s";
password = "%s";
passwordAdmin = "%s";
serverCommandPassword = "%s";

maxPlayers = %d;
persistent = 1;
voteMissionPlayers = 1;
voteThreshold = 0.33;

verifySignatures = %d;
BattlEye = %d;
kickDuplicate = 1;

class Missions
{
    class ALifeServer
    {
        template = "%s";
        difficulty = "Custom";
    };
};

difficulty = "Custom";
`,
		escapeCfgString(c.Hostname),
		escapeCfgString(c.Password),
		escapeCfgString(c.PasswordAdmin),
		escapeCfgString(c.PasswordAdmin), // serverCommandPassword mirrors passwordAdmin -- one field in the UI, not two to keep in sync
		c.MaxPlayers,
		boolInt(c.VerifySignatures),
		boolInt(c.BattlEye),
		escapeCfgString(c.MissionTemplate),
	)
}

// escapeCfgString guards against a value breaking out of its quoted
// server.cfg string -- the field values here come from this app's own UI,
// not an external network request, but there's no reason to skip this.
func escapeCfgString(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '"' || s[i] == '\\' {
			out = append(out, '\\')
		}
		out = append(out, s[i])
	}
	return string(out)
}
