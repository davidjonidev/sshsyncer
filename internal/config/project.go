package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ProjectFile is the file name committed at a project's root.
const ProjectFile = ".sshsyncer.json"

// Project is the on-disk shape of .sshsyncer.json.
type Project struct {
	Version int               `json:"version"`
	Vault   string            `json:"vault"`
	Targets map[string]Target `json:"targets"`
}

// Target maps a single git branch to a deploy destination.
type Target struct {
	Server     string   `json:"server"` // op:// reference
	RemotePath string   `json:"remote_path"`
	Excludes   []string `json:"excludes,omitempty"`
}

// LoadProject reads .sshsyncer.json from the given path.
func LoadProject(path string) (Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Project{}, err
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return Project{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if p.Targets == nil {
		p.Targets = map[string]Target{}
	}
	return p, nil
}

// SaveProject writes .sshsyncer.json to the given path.
func SaveProject(path string, p Project) error {
	if p.Version == 0 {
		p.Version = 1
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// FindProject walks up from start until it finds .sshsyncer.json.
func FindProject(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, ProjectFile)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s found. Run: sshsyncer init", ProjectFile)
		}
		dir = parent
	}
}
