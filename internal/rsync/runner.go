package rsync

import (
	"bytes"
	"io"
	"os/exec"
)

// Run executes rsync with the given args, streaming output to `out`.
// Returns the captured stdout for downstream parsing on dry runs.
func Run(args []string, out io.Writer) (string, error) {
	cmd := exec.Command("rsync", args...)
	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(out, &buf)
	cmd.Stderr = out
	err := cmd.Run()
	return buf.String(), err
}
