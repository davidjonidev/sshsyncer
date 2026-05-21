package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProject_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ProjectFile)

	p := Project{
		Version: 1,
		Vault:   "sshsyncer",
		Targets: map[string]Target{
			"main": {Server: "op://sshsyncer/myproject-prod", RemotePath: "/var/www/site"},
		},
	}
	if err := SaveProject(path, p); err != nil {
		t.Fatal(err)
	}

	got, err := LoadProject(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Targets["main"].Server != "op://sshsyncer/myproject-prod" {
		t.Fatalf("bad: %+v", got)
	}
}

func TestFindProject_WalksUp(t *testing.T) {
	dir := t.TempDir()
	if err := SaveProject(filepath.Join(dir, ProjectFile), Project{Version: 1}); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindProject(nested)
	if err != nil {
		t.Fatal(err)
	}
	// On macOS, /tmp is a symlink to /private/tmp; resolve both to canonical
	// form before comparing so the test doesn't fail on macOS-only weirdness.
	wantPath := filepath.Join(dir, ProjectFile)
	gotResolved, _ := filepath.EvalSymlinks(got)
	wantResolved, _ := filepath.EvalSymlinks(wantPath)
	if gotResolved != wantResolved {
		t.Fatalf("got %q, want %q", got, wantPath)
	}
}
