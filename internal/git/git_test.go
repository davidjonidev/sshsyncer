package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	return dir
}

func TestCurrentBranch(t *testing.T) {
	dir := initRepo(t)
	b, err := CurrentBranch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b != "main" {
		t.Fatalf("got %q", b)
	}
}

func TestHeadShort(t *testing.T) {
	dir := initRepo(t)
	h, err := HeadShort(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) < 4 || len(h) > 12 {
		t.Fatalf("got %q", h)
	}
}

func TestArchive_ExtractsPushedTree(t *testing.T) {
	dir := initRepo(t)
	remote := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %s: %v", out, err)
	}
	for _, args := range [][]string{
		{"remote", "add", "origin", remote},
		{"push", "origin", "main"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	out := t.TempDir()
	if err := Archive(dir, "origin/main", out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "hi") {
		t.Fatalf("got %q", data)
	}

	ok, err := HEADPushed(dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected HEAD == origin/main after push")
	}
}
