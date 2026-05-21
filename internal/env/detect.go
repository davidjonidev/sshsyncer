package env

import (
	"os"
	"os/exec"
	"strings"
)

// Check is a single environmental probe.
type Check struct {
	Name   string
	OK     bool
	Detail string // version string or empty
	Hint   string // remediation hint shown when !OK
}

// Result is the aggregated outcome of running every probe.
type Result struct {
	Checks []Check
}

// AllOK reports whether every check passed.
func (r Result) AllOK() bool {
	for _, c := range r.Checks {
		if !c.OK {
			return false
		}
	}
	return true
}

// Detect runs every environment probe and returns the result.
func Detect() Result {
	return Result{Checks: []Check{
		detectGit(),
		detectRsync(),
		detectOp(),
		detectAgent(),
	}}
}

func detectGit() Check {
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return Check{Name: "git", Hint: "Install git: https://git-scm.com/downloads"}
	}
	return Check{Name: "git", OK: true, Detail: strings.TrimSpace(string(out))}
}

func detectRsync() Check {
	out, err := exec.Command("rsync", "--version").Output()
	if err != nil {
		return Check{Name: "rsync", Hint: "Install GNU rsync: brew install rsync"}
	}
	s := string(out)
	if !isGNURsync(s) {
		return Check{
			Name: "rsync",
			Hint: "Detected non-GNU rsync (likely macOS openrsync). Install GNU rsync: brew install rsync",
		}
	}
	first, _, _ := strings.Cut(s, "\n")
	return Check{Name: "rsync", OK: true, Detail: strings.TrimSpace(first)}
}

func isGNURsync(versionOutput string) bool {
	v := strings.ToLower(versionOutput)
	return strings.Contains(v, "rsync") &&
		!strings.Contains(v, "openrsync") &&
		strings.Contains(v, "protocol version 3")
}

func detectOp() Check {
	out, err := exec.Command("op", "--version").Output()
	if err != nil {
		return Check{Name: "op", Hint: "Install 1Password CLI: https://1password.com/downloads/command-line"}
	}
	return Check{Name: "op", OK: true, Detail: strings.TrimSpace(string(out))}
}

func detectAgent() Check {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return Check{
			Name: "1password-agent",
			Hint: "Enable 1Password SSH agent: 1Password → Settings → Developer → Use SSH agent",
		}
	}
	if !strings.Contains(sock, "1password") && !strings.Contains(sock, ".1password") {
		return Check{
			Name:   "1password-agent",
			OK:     true,
			Detail: "SSH_AUTH_SOCK set (non-1Password agent — keys must be loaded another way)",
		}
	}
	return Check{Name: "1password-agent", OK: true, Detail: sock}
}
