package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigDir is the directory under $HOME where sshsyncer stores its config.
const ConfigDir = ".sshsyncer"
const globalFile = "config.json"

// Global is the on-disk shape of ~/.sshsyncer/config.json.
type Global struct {
	Version int    `json:"version"`
	Vault   string `json:"vault"`
}

func globalPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ConfigDir, globalFile), nil
}

// LoadGlobal reads the global config. Returns a friendly error if not set up.
func LoadGlobal() (Global, error) {
	p, err := globalPath()
	if err != nil {
		return Global{}, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return Global{}, fmt.Errorf("sshsyncer is not set up yet. Run: sshsyncer setup")
	}
	if err != nil {
		return Global{}, err
	}
	var g Global
	if err := json.Unmarshal(data, &g); err != nil {
		return Global{}, fmt.Errorf("parse %s: %w", p, err)
	}
	return g, nil
}

// SaveGlobal writes the global config (creating the directory if needed).
func SaveGlobal(g Global) error {
	if g.Version == 0 {
		g.Version = 1
	}
	p, err := globalPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
