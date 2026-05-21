package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Archive exports `ref` from the repo into outDir using `git archive | tar -x`.
func Archive(repo, ref, outDir string) error {
	out, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}
	archive := exec.Command("git", "archive", "--format=tar", ref)
	archive.Dir = repo
	var stderr bytes.Buffer
	archive.Stderr = &stderr

	tar := exec.Command("tar", "-x", "-C", out)
	tar.Stderr = &stderr

	pipe, err := archive.StdoutPipe()
	if err != nil {
		return err
	}
	tar.Stdin = pipe

	if err := tar.Start(); err != nil {
		return err
	}
	if err := archive.Start(); err != nil {
		return err
	}
	if err := archive.Wait(); err != nil {
		_ = tar.Process.Kill()
		return wrapPipeErr(stderr.String(), err)
	}
	if err := tar.Wait(); err != nil {
		return wrapPipeErr(stderr.String(), err)
	}
	return nil
}

func wrapPipeErr(stderr string, err error) error {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return err
	}
	return fmt.Errorf("git archive: %s", stderr)
}
