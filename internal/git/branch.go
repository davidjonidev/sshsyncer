package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// CurrentBranch returns the current git branch in the repo.
func CurrentBranch(repo string) (string, error) {
	out, err := run(repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	b := strings.TrimSpace(out)
	if b == "HEAD" {
		return "", fmt.Errorf("detached HEAD — check out a branch before deploying")
	}
	return b, nil
}

// HeadShort returns the short HEAD commit hash.
func HeadShort(repo string) (string, error) {
	out, err := run(repo, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func run(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
