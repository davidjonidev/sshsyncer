package config

import (
	"os"
	"path/filepath"
	"testing"
)

func withHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestGlobal_RoundTrip(t *testing.T) {
	withHome(t)
	if _, err := LoadGlobal(); err == nil {
		t.Fatal("expected not-set-up error")
	}
	g := Global{Version: 1, Vault: "sshsyncer"}
	if err := SaveGlobal(g); err != nil {
		t.Fatal(err)
	}
	got, err := LoadGlobal()
	if err != nil {
		t.Fatal(err)
	}
	if got.Vault != "sshsyncer" {
		t.Fatalf("got %+v", got)
	}
}

func TestGlobal_PathUnderHome(t *testing.T) {
	home := withHome(t)
	if err := SaveGlobal(Global{Version: 1, Vault: "sshsyncer"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".sshsyncer", "config.json")); err != nil {
		t.Fatalf("expected file: %v", err)
	}
}
