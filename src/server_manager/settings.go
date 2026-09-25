package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings holds machine-local configuration: where the Arma 3 Server
// install lives and how to reach Postgres. Persisted to a JSON file under
// the OS config dir (not the repo, not the Arma 3 Server install) -- this
// app is a tool a server host runs locally, its own settings are local too.
type Settings struct {
	ArmaServerPath  string `json:"armaServerPath"`
	PostgresHost    string `json:"postgresHost"`
	PostgresPort    string `json:"postgresPort"`
	PostgresDB      string `json:"postgresDb"`
	PostgresUser    string `json:"postgresUser"`
	PostgresPassword string `json:"postgresPassword"`
}

func defaultSettings() Settings {
	return Settings{
		PostgresHost: "127.0.0.1",
		PostgresPort: "5432",
		PostgresDB:   "alife_db",
		PostgresUser: "alife_admin",
	}
}

func settingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "TasDyn-ALife-ServerManager")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "settings.json"), nil
}

func loadSettings() (Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return defaultSettings(), err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaultSettings(), nil
	}
	if err != nil {
		return defaultSettings(), err
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return defaultSettings(), err
	}
	return s, nil
}

func saveSettingsToDisk(s Settings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
