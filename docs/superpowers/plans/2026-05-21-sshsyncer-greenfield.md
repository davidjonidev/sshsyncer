# sshsyncer Greenfield Go Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a fresh, public Go CLI (`sshsyncer`) that deploys a git branch to a remote SSH host via rsync, with all server credentials (host/user/port/SSH key) stored in a dedicated 1Password vault that sshsyncer creates and owns, and SSH auth pinned to a specific 1Password-held key per server.

**Architecture:** Cobra-based CLI. sshsyncer requires a dedicated 1Password vault (default name `sshsyncer`) that it creates on first run and owns thereafter — it refuses to operate on vaults containing items it did not create. `.sshsyncer.json` (committed) maps git branches to 1Password secret references (`op://<vault>/<server-item>`). Each Server item carries a reference to a specific SSH Key item, so deploys pin auth to one key (no shotgun-offering every key in the agent and tripping `MaxAuthTries`). At deploy time, the binary shells out to `op read` to resolve host/user/port and the public key for the current branch's target, then runs GNU rsync against a temp directory produced by `git archive origin/<branch>`, passing `-o IdentitiesOnly=yes -i <pubkey-path>` so SSH only offers that one key from the agent. No private keys ever live on disk under sshsyncer's control.

**Two auth modes for the `op` CLI** — sshsyncer supports both and `doctor` detects which is in use:
- **Desktop app + biometric** (interactive dev laptops): full feature support, including the 1Password SSH agent.
- **Service account** (`OP_SERVICE_ACCOUNT_TOKEN`, for CI/headless): can read/create vault items, but cannot serve SSH keys via the agent. CI must supply a separate SSH agent (e.g. `ssh-agent` with a runner-provided key) at `SSH_AUTH_SOCK`.

**Tech Stack:**
- Go 1.25
- `github.com/spf13/cobra` — CLI framework
- `github.com/charmbracelet/huh` — interactive forms
- `github.com/charmbracelet/lipgloss` — output styling
- `golang.org/x/crypto/ssh` — SSH client (talks to 1Password agent via `SSH_AUTH_SOCK`)
- 1Password CLI `op` — secrets resolution (subprocess)
- GNU rsync — file transfer (subprocess)
- `testify` for assertions

---

## File structure

```
sshsyncer/
├── go.mod
├── go.sum
├── main.go                          # entrypoint, version injection
├── README.md                        # public-facing docs
├── LICENSE                          # MIT
├── CLAUDE.md                        # AI-agent guidance (kept from current)
├── .gitignore
├── .goreleaser.yaml                 # homebrew distribution
│
├── cmd/                             # cobra command handlers (thin controllers)
│   ├── root.go                      # root cmd + version + global flags
│   ├── setup.go                     # one-time env check + default vault
│   ├── doctor.go                    # diagnostic command
│   ├── vault.go                     # show/set default vault
│   ├── server.go                    # parent: server add/list/remove/test
│   ├── server_add.go
│   ├── server_list.go
│   ├── server_remove.go
│   ├── server_test.go
│   ├── init.go                      # project init: .sshsyncer.json + .deployignore
│   ├── target.go                    # parent + show current branch target
│   ├── target_edit.go
│   ├── target_remove.go
│   ├── target_list.go
│   ├── push.go                      # main deploy command
│   ├── log.go                       # view remote .sshsyncer-log
│   └── docs.go                      # print full reference
│
├── internal/
│   ├── config/
│   │   ├── global.go                # ~/.sshsyncer/config.json (vault name only)
│   │   ├── project.go               # .sshsyncer.json read/write
│   │   └── require.go               # setup-required gate
│   ├── op/
│   │   ├── client.go                # `op` CLI wrapper (run, parse JSON)
│   │   ├── reference.go             # parse op://Vault/Item[/field]
│   │   ├── vault.go                 # create/list/inspect vaults; ownership guard
│   │   ├── server_item.go           # CRUD for Server items (with linked ssh_key)
│   │   ├── ssh_key_item.go          # CRUD + public-key read for SSH Key items
│   │   ├── auth_mode.go             # detect desktop-app vs service-account
│   │   └── signin.go                # signin status + helpful error
│   ├── git/
│   │   ├── branch.go                # current branch name
│   │   ├── fetch.go                 # git fetch origin
│   │   ├── pushed.go                # is HEAD pushed to origin/<branch>?
│   │   └── archive.go               # git archive origin/<branch> → tmpdir
│   ├── rsync/
│   │   ├── builder.go               # build argv
│   │   ├── parser.go                # parse --dry-run output for preview
│   │   └── runner.go                # exec rsync, stream output
│   ├── ssh/
│   │   ├── client.go                # net/ssh client using SSH_AUTH_SOCK
│   │   └── pinkey.go                # write pubkey to tempfile; filter agent signers
│   ├── deploylog/
│   │   └── log.go                   # append/fetch remote .sshsyncer-log
│   ├── deployignore/
│   │   └── template.go              # embedded default .deployignore
│   ├── env/
│   │   └── detect.go                # detect git, rsync (GNU), op, agent
│   └── ui/
│       ├── style.go                 # lipgloss palette + helpers
│       └── prompt.go                # confirm/select wrappers around huh
│
└── docs/
    └── architecture.md              # for contributors
```

---

## Data shapes

### `~/.sshsyncer/config.json`

```json
{
  "version": 1,
  "vault": "sshsyncer"
}
```

`vault` is the dedicated 1Password vault sshsyncer manages on this machine. The default name is `sshsyncer` but the user may pick another name at `setup` time (e.g. `sshsyncer-team` for a shared one).

### `.sshsyncer.json` (committed at project root)

```json
{
  "version": 1,
  "vault": "sshsyncer",
  "targets": {
    "main": {
      "server": "op://sshsyncer/myproject-prod",
      "remote_path": "/var/www/site/wp-content/themes/mytheme",
      "excludes": []
    },
    "staging": {
      "server": "op://sshsyncer/myproject-staging",
      "remote_path": "/var/www/staging/wp-content/themes/mytheme",
      "excludes": ["wp-config-staging.php"]
    }
  }
}
```

`vault` records which 1Password vault this project's references point at so a fresh clone fails loudly if the local machine's sshsyncer is pointed at a different vault. Per-target `server` references are fully qualified `op://` strings.

### 1Password Server item (created by `sshsyncer server add`)

- Category: **Server**
- Vault: the dedicated sshsyncer vault
- Title: user-chosen, e.g. `myproject-prod`
- Tag: `sshsyncer` (every item sshsyncer creates carries this tag — used by the ownership guard)
- Fields:
  - `host` (string)
  - `username` (string)
  - `port` (string, default `22`)
  - `ssh_key` (string, **required**) — a secret reference like `op://sshsyncer/myproject-key` pointing at the SSH Key item to pin auth to.

### 1Password SSH Key item (created by `sshsyncer server add` or pre-existing)

- Category: **SSH Key**
- Vault: the dedicated sshsyncer vault
- Title: user-chosen, e.g. `myproject-key`
- Tag: `sshsyncer`
- Generated via `op item create --category=ssh-key --ssh-generate-key=ed25519 --vault=<vault> --title=<title> --tags=sshsyncer`. The private key never leaves 1Password.
- Read by sshsyncer via `op read op://<vault>/<title>/public_key`.

---

## Vault ownership model

sshsyncer treats its 1Password vault as **owned by the tool**:

1. `sshsyncer setup` creates a new vault (default name `sshsyncer`) via `op vault create`. If the user picks an existing vault name, setup refuses unless the vault is empty or every item in it carries the `sshsyncer` tag.
2. Every item sshsyncer creates is tagged `sshsyncer`. This is how the ownership guard works.
3. `sshsyncer server list`, `init`, `target edit`, etc. only ever look in the configured vault. There is no fallback to `Private` or any other vault.
4. Sharing with teammates = share the vault in 1Password. The teammate runs `sshsyncer setup`, picks the existing shared vault when prompted, and sshsyncer verifies all items are sshsyncer-owned before saving the choice.

This model means a user's personal 1Password content is never touched, scoped service account tokens can grant CI access to just this vault, and listing/searching is fast and unambiguous.

---

## Conventions

- **Errors:** every error returned to the user names the failing component and the next command to run. No silent failures.
- **No `--no-verify`-style escape hatches.** If something blocks deploy, surface it.
- **TDD:** every package gets a `_test.go` written before the implementation. External calls (`op`, `git`, `rsync`, SSH) are gated behind small interfaces so tests can fake them.
- **No emojis in output** — lipgloss colors and unicode box drawing only.
- **Frequent commits:** one commit per task.

---

## Task sequence

The plan is bottom-up: leaves (env detection, op wrapper, config) first, then mid-layer (git/rsync/ssh), then commands. Each task is independently committable. Adjacent tasks may be batched by an executor.

---

### Task 1: Repo skeleton + module init

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`
- Create: `.gitignore`
- Create: `LICENSE` (MIT, year 2026, holder "sshsyncer contributors")
- Create: `README.md` (1-line placeholder; full docs later)

- [ ] **Step 1:** `go mod init github.com/sshsyncer/sshsyncer`

- [ ] **Step 2:** Add cobra dependency

```bash
go get github.com/spf13/cobra@latest
```

- [ ] **Step 3:** Write `main.go`

```go
package main

import (
	"fmt"
	"os"

	"github.com/sshsyncer/sshsyncer/cmd"
)

var version = "dev"

func main() {
	cmd.SetVersion(version)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4:** Write `cmd/root.go`

```go
package cmd

import "github.com/spf13/cobra"

var version = "dev"

func SetVersion(v string) { version = v }

var rootCmd = &cobra.Command{
	Use:     "sshsyncer",
	Short:   "Deploy a git branch to a remote server via rsync over SSH",
	Long:    "sshsyncer deploys the last pushed commit of the current branch to a server defined in 1Password.",
	Version: version,
}

func Execute() error { return rootCmd.Execute() }
```

- [ ] **Step 5:** `.gitignore`

```
/sshsyncer
/dist/
*.test
*.out
.DS_Store
```

- [ ] **Step 6:** Verify build

Run: `go build -o sshsyncer . && ./sshsyncer --help`
Expected: usage output, exit 0.

- [ ] **Step 7:** Commit

```bash
git add .
git commit -m "chore: scaffold module and root command"
```

---

### Task 2: UI helpers (lipgloss + huh wrappers)

**Files:**
- Create: `internal/ui/style.go`
- Create: `internal/ui/prompt.go`
- Create: `internal/ui/style_test.go`

Centralise styling so every command has consistent output. Wrappers around huh keep `cmd/` free of UI library imports.

- [ ] **Step 1:** Add deps

```bash
go get github.com/charmbracelet/lipgloss github.com/charmbracelet/huh
```

- [ ] **Step 2:** Write `internal/ui/style.go`

```go
package ui

import "github.com/charmbracelet/lipgloss"

var (
	Title   = lipgloss.NewStyle().Bold(true)
	OK      = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).SetString("✓")
	Fail    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).SetString("✗")
	Dim     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	Warn    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	KeyCol  = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
)

func Checkline(label string, ok bool, detail string) string {
	mark := OK.String()
	if !ok {
		mark = Fail.String()
	}
	if detail == "" {
		return mark + "  " + label
	}
	return mark + "  " + label + "  " + Dim.Render(detail)
}
```

- [ ] **Step 3:** Write `internal/ui/prompt.go`

```go
package ui

import "github.com/charmbracelet/huh"

func Confirm(title, affirmative, negative string) (bool, error) {
	var v bool
	err := huh.NewConfirm().
		Title(title).
		Affirmative(affirmative).
		Negative(negative).
		Value(&v).Run()
	return v, err
}

func Input(title string, value *string, validate func(string) error) error {
	return huh.NewInput().Title(title).Value(value).Validate(validate).Run()
}

func Select[T comparable](title string, value *T, options []huh.Option[T]) error {
	return huh.NewSelect[T]().Title(title).Options(options...).Value(value).Run()
}
```

- [ ] **Step 4:** Smoke test in `internal/ui/style_test.go`

```go
package ui

import (
	"strings"
	"testing"
)

func TestCheckline_OK(t *testing.T) {
	out := Checkline("git", true, "2.43.0")
	if !strings.Contains(out, "git") || !strings.Contains(out, "2.43.0") {
		t.Fatalf("unexpected: %q", out)
	}
}

func TestCheckline_Fail(t *testing.T) {
	out := Checkline("op", false, "")
	if !strings.Contains(out, "op") {
		t.Fatalf("unexpected: %q", out)
	}
}
```

- [ ] **Step 5:** `go test ./internal/ui/...`

- [ ] **Step 6:** Commit

```bash
git add .
git commit -m "feat(ui): add lipgloss styles and huh prompt wrappers"
```

---

### Task 3: Environment detection

**Files:**
- Create: `internal/env/detect.go`
- Create: `internal/env/detect_test.go`

Detect git, GNU rsync, `op` CLI, and the 1Password SSH agent. Used by `setup` and `doctor`.

- [ ] **Step 1:** Write the failing test `internal/env/detect_test.go`

```go
package env

import "testing"

func TestDetect_ReturnsAllChecks(t *testing.T) {
	r := Detect()
	names := map[string]bool{}
	for _, c := range r.Checks {
		names[c.Name] = true
	}
	for _, want := range []string{"git", "rsync", "op", "1password-agent"} {
		if !names[want] {
			t.Errorf("missing check %q", want)
		}
	}
}

func TestIsGNURsync(t *testing.T) {
	cases := map[string]bool{
		"rsync  version 3.2.7  protocol version 31\n": true,
		"openrsync: protocol version 29\n":            false,
		"":                                            false,
	}
	for in, want := range cases {
		if got := isGNURsync(in); got != want {
			t.Errorf("isGNURsync(%q) = %v, want %v", in, got, want)
		}
	}
}
```

- [ ] **Step 2:** Implement `internal/env/detect.go`

```go
package env

import (
	"os"
	"os/exec"
	"strings"
)

type Check struct {
	Name    string
	OK      bool
	Detail  string  // version string or empty
	Hint    string  // remediation hint shown when !OK
}

type Result struct {
	Checks []Check
}

func (r Result) AllOK() bool {
	for _, c := range r.Checks {
		if !c.OK {
			return false
		}
	}
	return true
}

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
```

- [ ] **Step 3:** `go test ./internal/env/...`

- [ ] **Step 4:** Commit

```bash
git add .
git commit -m "feat(env): detect git, rsync, op, and SSH agent availability"
```

---

### Task 4: 1Password reference parser

**Files:**
- Create: `internal/op/reference.go`
- Create: `internal/op/reference_test.go`

Pure-function parser for `op://Vault/Item[/Section/Field]` strings. No subprocesses, no I/O.

- [ ] **Step 1:** Write failing tests

```go
package op

import "testing"

func TestParseReference(t *testing.T) {
	cases := []struct {
		in    string
		ok    bool
		vault string
		item  string
		field string
	}{
		{"op://Private/server-prod", true, "Private", "server-prod", ""},
		{"op://DevOps/my-server/password", true, "DevOps", "my-server", "password"},
		{"op://Private/server-prod/section/field", true, "Private", "server-prod", "section/field"},
		{"not-a-reference", false, "", "", ""},
		{"op://Private", false, "", "", ""},
		{"op:///item", false, "", "", ""},
		{"", false, "", "", ""},
	}
	for _, tc := range cases {
		r, err := ParseReference(tc.in)
		if (err == nil) != tc.ok {
			t.Errorf("ParseReference(%q) ok=%v want=%v err=%v", tc.in, err == nil, tc.ok, err)
			continue
		}
		if !tc.ok {
			continue
		}
		if r.Vault != tc.vault || r.Item != tc.item || r.Field != tc.field {
			t.Errorf("ParseReference(%q) = %+v", tc.in, r)
		}
	}
}

func TestReference_String(t *testing.T) {
	r := Reference{Vault: "Private", Item: "x"}
	if r.String() != "op://Private/x" {
		t.Fatalf("got %q", r.String())
	}
	r.Field = "password"
	if r.String() != "op://Private/x/password" {
		t.Fatalf("got %q", r.String())
	}
}
```

- [ ] **Step 2:** Implement

```go
package op

import (
	"fmt"
	"strings"
)

type Reference struct {
	Vault string
	Item  string
	Field string // optional; may include nested path "section/field"
}

func ParseReference(s string) (Reference, error) {
	const prefix = "op://"
	if !strings.HasPrefix(s, prefix) {
		return Reference{}, fmt.Errorf("not an op:// reference: %q", s)
	}
	rest := strings.TrimPrefix(s, prefix)
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return Reference{}, fmt.Errorf("op reference must be op://Vault/Item[/Field]: %q", s)
	}
	r := Reference{Vault: parts[0], Item: parts[1]}
	if len(parts) == 3 {
		r.Field = parts[2]
	}
	return r, nil
}

func (r Reference) String() string {
	if r.Field == "" {
		return fmt.Sprintf("op://%s/%s", r.Vault, r.Item)
	}
	return fmt.Sprintf("op://%s/%s/%s", r.Vault, r.Item, r.Field)
}

func (r Reference) ItemRef() Reference {
	return Reference{Vault: r.Vault, Item: r.Item}
}
```

- [ ] **Step 3:** `go test ./internal/op/...`

- [ ] **Step 4:** Commit

```bash
git add .
git commit -m "feat(op): parse op:// secret references"
```

---

### Task 5: `op` CLI wrapper

**Files:**
- Create: `internal/op/client.go`
- Create: `internal/op/client_test.go`
- Create: `internal/op/signin.go`

Thin wrapper around the `op` CLI. Uses `--format json` everywhere. Behind an interface so callers can fake it.

- [ ] **Step 1:** Write failing test

```go
package op

import (
	"errors"
	"testing"
)

type fakeRunner struct {
	args    [][]string
	stdouts []string
	errs    []error
	calls   int
}

func (f *fakeRunner) Run(args ...string) ([]byte, error) {
	f.args = append(f.args, args)
	i := f.calls
	f.calls++
	if i >= len(f.stdouts) {
		return nil, errors.New("no canned response")
	}
	return []byte(f.stdouts[i]), f.errs[i]
}

func TestClient_Read(t *testing.T) {
	fr := &fakeRunner{stdouts: []string{"staging.example.com\n"}, errs: []error{nil}}
	c := &Client{runner: fr}
	v, err := c.Read("op://Private/foo/host")
	if err != nil {
		t.Fatal(err)
	}
	if v != "staging.example.com" {
		t.Fatalf("got %q", v)
	}
	if got := fr.args[0]; got[0] != "read" || got[1] != "op://Private/foo/host" {
		t.Fatalf("unexpected args: %v", got)
	}
}

func TestClient_ReadItem(t *testing.T) {
	const itemJSON = `{
	  "id": "abc",
	  "title": "server-prod",
	  "vault": {"name": "Private"},
	  "category": "SERVER",
	  "tags": ["sshsyncer"],
	  "fields": [
	    {"label": "host", "value": "h"},
	    {"label": "username", "value": "u"},
	    {"label": "port", "value": "22"}
	  ]
	}`
	fr := &fakeRunner{stdouts: []string{itemJSON}, errs: []error{nil}}
	c := &Client{runner: fr}
	it, err := c.ReadItem(Reference{Vault: "Private", Item: "server-prod"})
	if err != nil {
		t.Fatal(err)
	}
	if it.Title != "server-prod" || it.Vault.Name != "Private" {
		t.Fatalf("bad item: %+v", it)
	}
}
```

- [ ] **Step 2:** Implement `internal/op/client.go`

```go
package op

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(args ...string) ([]byte, error) {
	cmd := exec.Command("op", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("op %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

type Client struct {
	runner Runner
}

func NewClient() *Client { return &Client{runner: execRunner{}} }

// Read fetches a single value via a secret reference.
func (c *Client) Read(reference string) (string, error) {
	out, err := c.runner.Run("read", reference)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type Item struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
	Vault    struct {
		Name string `json:"name"`
	} `json:"vault"`
	Fields []ItemField `json:"fields"`
}

type ItemField struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	Type   string `json:"type"`
}

func (it Item) Field(label string) (string, bool) {
	for _, f := range it.Fields {
		if strings.EqualFold(f.Label, label) {
			return f.Value, true
		}
	}
	return "", false
}

func (c *Client) ReadItem(ref Reference) (Item, error) {
	args := []string{"item", "get", ref.Item, "--vault", ref.Vault, "--format", "json"}
	out, err := c.runner.Run(args...)
	if err != nil {
		return Item{}, err
	}
	var it Item
	if err := json.Unmarshal(out, &it); err != nil {
		return Item{}, fmt.Errorf("parse op item json: %w", err)
	}
	return it, nil
}

var ErrNotSignedIn = errors.New("not signed in to 1Password (run: eval $(op signin))")
```

- [ ] **Step 3:** Implement `internal/op/vault.go`

```go
package op

import (
	"encoding/json"
	"fmt"
)

type Vault struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *Client) ListVaults() ([]Vault, error) {
	out, err := c.runner.Run("vault", "list", "--format", "json")
	if err != nil {
		return nil, err
	}
	var vs []Vault
	if err := json.Unmarshal(out, &vs); err != nil {
		return nil, fmt.Errorf("parse op vault list: %w", err)
	}
	return vs, nil
}

func (c *Client) FindVault(name string) (Vault, bool, error) {
	vs, err := c.ListVaults()
	if err != nil {
		return Vault{}, false, err
	}
	for _, v := range vs {
		if v.Name == name {
			return v, true, nil
		}
	}
	return Vault{}, false, nil
}

// CreateVault creates a new personal vault with the given name and returns it.
func (c *Client) CreateVault(name, description string) (Vault, error) {
	args := []string{"vault", "create", name, "--format", "json"}
	if description != "" {
		args = append(args, "--description", description)
	}
	out, err := c.runner.Run(args...)
	if err != nil {
		return Vault{}, err
	}
	var v Vault
	if err := json.Unmarshal(out, &v); err != nil {
		return Vault{}, fmt.Errorf("parse op vault create: %w", err)
	}
	return v, nil
}

// ListAllItems lists every item in a vault (any category, any tags).
// Used by the ownership guard at setup time.
func (c *Client) ListAllItems(vault string) ([]Item, error) {
	out, err := c.runner.Run("item", "list", "--vault", vault, "--format", "json")
	if err != nil {
		return nil, err
	}
	var items []Item
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parse op item list: %w", err)
	}
	return items, nil
}

// OwnershipReport categorises items in the vault by whether sshsyncer owns them.
type OwnershipReport struct {
	OwnedCount   int
	ForeignCount int
	Foreign      []Item // first few foreign items, for the error message
}

func (c *Client) AssessVaultOwnership(vault string) (OwnershipReport, error) {
	items, err := c.ListAllItems(vault)
	if err != nil {
		return OwnershipReport{}, err
	}
	var r OwnershipReport
	for _, it := range items {
		owned := false
		for _, t := range it.Tags {
			if t == Tag {
				owned = true
				break
			}
		}
		if owned {
			r.OwnedCount++
		} else {
			r.ForeignCount++
			if len(r.Foreign) < 5 {
				r.Foreign = append(r.Foreign, it)
			}
		}
	}
	return r, nil
}
```

- [ ] **Step 4:** Implement `internal/op/signin.go`

```go
package op

import "strings"

func (c *Client) SignedIn() (bool, error) {
	out, err := c.runner.Run("account", "list", "--format", "json")
	if err != nil {
		if strings.Contains(err.Error(), "not signed in") {
			return false, nil
		}
		return false, err
	}
	return len(strings.TrimSpace(string(out))) > 2, nil // more than "[]"
}
```

- [ ] **Step 5:** Implement `internal/op/auth_mode.go`

```go
package op

import "os"

type AuthMode int

const (
	AuthModeUnknown AuthMode = iota
	AuthModeDesktopApp
	AuthModeServiceAccount
)

func (m AuthMode) String() string {
	switch m {
	case AuthModeDesktopApp:
		return "desktop app"
	case AuthModeServiceAccount:
		return "service account"
	}
	return "unknown"
}

// DetectAuthMode infers which auth mode the op CLI will use.
// Service-account mode is selected when OP_SERVICE_ACCOUNT_TOKEN is set,
// regardless of whether the desktop app is also installed.
func DetectAuthMode() AuthMode {
	if os.Getenv("OP_SERVICE_ACCOUNT_TOKEN") != "" {
		return AuthModeServiceAccount
	}
	if _, err := os.Stat(os.Getenv("HOME") + "/Applications/1Password.app"); err == nil {
		return AuthModeDesktopApp
	}
	if _, err := os.Stat("/Applications/1Password.app"); err == nil {
		return AuthModeDesktopApp
	}
	// Linux + manual signin: also treat as desktop-app-equivalent for capability purposes.
	return AuthModeDesktopApp
}
```

- [ ] **Step 6:** Extend the test file with vault + auth-mode assertions

```go
func TestListVaults(t *testing.T) {
	fr := &fakeRunner{stdouts: []string{`[{"id":"abc","name":"sshsyncer"},{"id":"def","name":"Private"}]`}, errs: []error{nil}}
	c := &Client{runner: fr}
	vs, err := c.ListVaults()
	if err != nil { t.Fatal(err) }
	if len(vs) != 2 || vs[0].Name != "sshsyncer" {
		t.Fatalf("got %+v", vs)
	}
}

func TestAssessVaultOwnership(t *testing.T) {
	listJSON := `[
	  {"id":"1","title":"a","tags":["sshsyncer"]},
	  {"id":"2","title":"b","tags":["other"]},
	  {"id":"3","title":"c","tags":[]}
	]`
	fr := &fakeRunner{stdouts: []string{listJSON}, errs: []error{nil}}
	c := &Client{runner: fr}
	r, err := c.AssessVaultOwnership("v")
	if err != nil { t.Fatal(err) }
	if r.OwnedCount != 1 || r.ForeignCount != 2 {
		t.Fatalf("got %+v", r)
	}
}

func TestDetectAuthMode_ServiceAccount(t *testing.T) {
	t.Setenv("OP_SERVICE_ACCOUNT_TOKEN", "ops_xxx")
	if DetectAuthMode() != AuthModeServiceAccount {
		t.Fatalf("expected service account")
	}
}
```

- [ ] **Step 7:** `go test ./internal/op/...`

- [ ] **Step 8:** Commit

```bash
git add .
git commit -m "feat(op): client, vault ops, ownership guard, auth-mode detection"
```

---

### Task 6: Server-item CRUD on top of `op`

**Files:**
- Create: `internal/op/server_item.go`
- Create: `internal/op/server_item_test.go`

Wraps `op item create/list/delete` for sshsyncer-tagged Server items. Every server item carries an `ssh_key` field whose value is an `op://` reference to a 1Password SSH Key item — this is what pins SSH auth to a specific key at deploy time.

- [ ] **Step 1:** Write failing test

```go
package op

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestServerItem_Roundtrip(t *testing.T) {
	listJSON := `[{
	  "id": "abc",
	  "title": "myproject-prod",
	  "vault": {"name": "sshsyncer"},
	  "category": "SERVER",
	  "tags": ["sshsyncer"]
	}]`
	itemJSON := `{
	  "id": "abc",
	  "title": "myproject-prod",
	  "vault": {"name": "sshsyncer"},
	  "category": "SERVER",
	  "tags": ["sshsyncer"],
	  "fields": [
	    {"label": "host", "value": "h.example.com"},
	    {"label": "username", "value": "deploy"},
	    {"label": "port", "value": "22"},
	    {"label": "ssh_key", "value": "op://sshsyncer/myproject-key"}
	  ]
	}`
	fr := &fakeRunner{
		stdouts: []string{listJSON, itemJSON},
		errs:    []error{nil, nil},
	}
	c := &Client{runner: fr}

	items, err := c.ListServers("sshsyncer")
	if err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].Title != "myproject-prod" {
		t.Fatalf("bad list: %+v", items)
	}

	srv, err := c.GetServer(Reference{Vault: "sshsyncer", Item: "myproject-prod"})
	if err != nil { t.Fatal(err) }
	if srv.Host != "h.example.com" || srv.User != "deploy" || srv.Port != 22 {
		t.Fatalf("bad server: %+v", srv)
	}
	if srv.SSHKeyRef != "op://sshsyncer/myproject-key" {
		t.Fatalf("bad ssh_key: %q", srv.SSHKeyRef)
	}
}

func TestServerItem_CreateArgs(t *testing.T) {
	fr := &fakeRunner{stdouts: []string{`{"id":"x","title":"new","vault":{"name":"sshsyncer"}}`}, errs: []error{nil}}
	c := &Client{runner: fr}
	_, err := c.CreateServer(CreateServerInput{
		Vault:     "sshsyncer",
		Title:     "new",
		Host:      "h",
		User:      "u",
		Port:      22,
		SSHKeyRef: "op://sshsyncer/new-key",
	})
	if err != nil { t.Fatal(err) }
	args := fr.args[0]
	want := []string{"item", "create", "--category", "Server", "--vault", "sshsyncer", "--title", "new",
		"--tags", "sshsyncer", "host=h", "username=u", "port=22",
		"ssh_key[text]=op://sshsyncer/new-key", "--format", "json"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("\n got: %v\nwant: %v", args, want)
	}
	// confirm we can still parse the returned json into Item
	var it Item
	if err := json.Unmarshal([]byte(fr.stdouts[0]), &it); err != nil {
		t.Fatal(err)
	}
}

func TestServerItem_CreateRequiresKeyRef(t *testing.T) {
	c := &Client{runner: &fakeRunner{}}
	_, err := c.CreateServer(CreateServerInput{Vault: "v", Title: "t", Host: "h", User: "u"})
	if err == nil { t.Fatal("expected error: ssh_key is required") }
}
```

- [ ] **Step 2:** Implement `internal/op/server_item.go`

```go
package op

import (
	"encoding/json"
	"fmt"
	"strconv"
)

const Tag = "sshsyncer"

type Server struct {
	Ref       Reference
	Title     string
	Host      string
	User      string
	Port      int
	SSHKeyRef string // op:// reference to a 1Password SSH Key item
}

type CreateServerInput struct {
	Vault     string
	Title     string
	Host      string
	User      string
	Port      int
	SSHKeyRef string // required: op:// reference to an existing SSH Key item
}

func (c *Client) ListServers(vault string) ([]Item, error) {
	args := []string{"item", "list", "--categories", "Server", "--tags", Tag, "--format", "json"}
	if vault != "" {
		args = append(args, "--vault", vault)
	}
	out, err := c.runner.Run(args...)
	if err != nil {
		return nil, err
	}
	var items []Item
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parse op item list: %w", err)
	}
	return items, nil
}

func (c *Client) GetServer(ref Reference) (Server, error) {
	it, err := c.ReadItem(ref)
	if err != nil {
		return Server{}, err
	}
	host, _ := it.Field("host")
	user, _ := it.Field("username")
	portStr, _ := it.Field("port")
	keyRef, _ := it.Field("ssh_key")
	port, _ := strconv.Atoi(portStr)
	if port == 0 {
		port = 22
	}
	if host == "" || user == "" {
		return Server{}, fmt.Errorf("server item %s is missing host or username", ref)
	}
	if keyRef == "" {
		return Server{}, fmt.Errorf("server item %s is missing ssh_key reference — re-create with `sshsyncer server add`", ref)
	}
	return Server{
		Ref:       Reference{Vault: it.Vault.Name, Item: it.Title},
		Title:     it.Title,
		Host:      host,
		User:      user,
		Port:      port,
		SSHKeyRef: keyRef,
	}, nil
}

func (c *Client) CreateServer(in CreateServerInput) (Item, error) {
	if in.SSHKeyRef == "" {
		return Item{}, fmt.Errorf("ssh_key reference is required when creating a server item")
	}
	if in.Port == 0 {
		in.Port = 22
	}
	args := []string{
		"item", "create",
		"--category", "Server",
		"--vault", in.Vault,
		"--title", in.Title,
		"--tags", Tag,
		"host=" + in.Host,
		"username=" + in.User,
		"port=" + strconv.Itoa(in.Port),
		"ssh_key[text]=" + in.SSHKeyRef,
		"--format", "json",
	}
	out, err := c.runner.Run(args...)
	if err != nil {
		return Item{}, err
	}
	var it Item
	if err := json.Unmarshal(out, &it); err != nil {
		return Item{}, fmt.Errorf("parse op item create json: %w", err)
	}
	return it, nil
}

func (c *Client) DeleteServer(ref Reference) error {
	_, err := c.runner.Run("item", "delete", ref.Item, "--vault", ref.Vault)
	return err
}
```

- [ ] **Step 3:** `go test ./internal/op/...`

- [ ] **Step 4:** Commit

```bash
git add .
git commit -m "feat(op): server item CRUD with ssh_key reference"
```

---

### Task 6b: SSH Key item CRUD on top of `op`

**Files:**
- Create: `internal/op/ssh_key_item.go`
- Create: `internal/op/ssh_key_item_test.go`

Generate, list, and read 1Password SSH Key items. Generation is the primary path — `op item create --category=ssh-key --ssh-generate-key=ed25519` makes 1Password produce the key, and the private half never leaves 1Password.

- [ ] **Step 1:** Failing test

```go
package op

import (
	"strings"
	"testing"
)

func TestGenerateSSHKey_Args(t *testing.T) {
	itemJSON := `{
	  "id": "k1",
	  "title": "myproject-key",
	  "vault": {"name": "sshsyncer"},
	  "category": "SSH_KEY",
	  "tags": ["sshsyncer"],
	  "fields": [
	    {"label": "public_key", "value": "ssh-ed25519 AAAA... sshsyncer:myproject-key"},
	    {"label": "key_type", "value": "ed25519"}
	  ]
	}`
	fr := &fakeRunner{stdouts: []string{itemJSON}, errs: []error{nil}}
	c := &Client{runner: fr}

	key, err := c.GenerateSSHKey("sshsyncer", "myproject-key")
	if err != nil { t.Fatal(err) }
	if !strings.HasPrefix(key.PublicKey, "ssh-ed25519 ") {
		t.Fatalf("bad public key: %q", key.PublicKey)
	}
	if key.Ref.String() != "op://sshsyncer/myproject-key" {
		t.Fatalf("bad ref: %q", key.Ref)
	}

	args := fr.args[0]
	want := []string{
		"item", "create",
		"--category", "SSH Key",
		"--vault", "sshsyncer",
		"--title", "myproject-key",
		"--tags", "sshsyncer",
		"--ssh-generate-key", "ed25519",
		"--format", "json",
	}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("\n got: %v\nwant: %v", args, want)
	}
}

func TestListSSHKeys(t *testing.T) {
	listJSON := `[{"id":"k1","title":"myproject-key","vault":{"name":"sshsyncer"},"tags":["sshsyncer"]}]`
	fr := &fakeRunner{stdouts: []string{listJSON}, errs: []error{nil}}
	c := &Client{runner: fr}
	items, err := c.ListSSHKeys("sshsyncer")
	if err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].Title != "myproject-key" {
		t.Fatalf("bad list: %+v", items)
	}
}
```

- [ ] **Step 2:** Implement `internal/op/ssh_key_item.go`

```go
package op

import (
	"encoding/json"
	"fmt"
)

type SSHKey struct {
	Ref       Reference
	Title     string
	PublicKey string
	KeyType   string
}

func (c *Client) GenerateSSHKey(vault, title string) (SSHKey, error) {
	args := []string{
		"item", "create",
		"--category", "SSH Key",
		"--vault", vault,
		"--title", title,
		"--tags", Tag,
		"--ssh-generate-key", "ed25519",
		"--format", "json",
	}
	out, err := c.runner.Run(args...)
	if err != nil {
		return SSHKey{}, err
	}
	var it Item
	if err := json.Unmarshal(out, &it); err != nil {
		return SSHKey{}, fmt.Errorf("parse op ssh-key create: %w", err)
	}
	return sshKeyFromItem(it), nil
}

func (c *Client) ListSSHKeys(vault string) ([]Item, error) {
	args := []string{"item", "list", "--categories", "SSH Key", "--tags", Tag, "--format", "json"}
	if vault != "" {
		args = append(args, "--vault", vault)
	}
	out, err := c.runner.Run(args...)
	if err != nil {
		return nil, err
	}
	var items []Item
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parse op ssh-key list: %w", err)
	}
	return items, nil
}

func (c *Client) GetSSHKey(ref Reference) (SSHKey, error) {
	it, err := c.ReadItem(ref)
	if err != nil {
		return SSHKey{}, err
	}
	return sshKeyFromItem(it), nil
}

// ReadPublicKey is the fast path used at deploy time.
func (c *Client) ReadPublicKey(ref Reference) (string, error) {
	r := Reference{Vault: ref.Vault, Item: ref.Item, Field: "public_key"}
	pk, err := c.Read(r.String())
	if err != nil {
		return "", fmt.Errorf("read public_key for %s: %w", ref, err)
	}
	if pk == "" {
		return "", fmt.Errorf("public_key field is empty on %s", ref)
	}
	return pk, nil
}

func sshKeyFromItem(it Item) SSHKey {
	pk, _ := it.Field("public_key")
	kt, _ := it.Field("key_type")
	return SSHKey{
		Ref:       Reference{Vault: it.Vault.Name, Item: it.Title},
		Title:     it.Title,
		PublicKey: pk,
		KeyType:   kt,
	}
}
```

- [ ] **Step 3:** `go test ./internal/op/...`

- [ ] **Step 4:** Commit

```bash
git add .
git commit -m "feat(op): SSH Key item generate/list/read"
```

---

### Task 7: Global config (`~/.sshsyncer/config.json`)

**Files:**
- Create: `internal/config/global.go`
- Create: `internal/config/global_test.go`
- Create: `internal/config/require.go`

Stores the name of the dedicated 1Password vault sshsyncer manages on this machine. Simple JSON read/write with `os.UserHomeDir()`.

- [ ] **Step 1:** Write failing test

```go
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
	if err := SaveGlobal(g); err != nil { t.Fatal(err) }
	got, err := LoadGlobal()
	if err != nil { t.Fatal(err) }
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
```

- [ ] **Step 2:** Implement `internal/config/global.go`

```go
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const ConfigDir = ".sshsyncer"
const globalFile = "config.json"

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
```

- [ ] **Step 3:** Implement `internal/config/require.go`

```go
package config

func RequireSetup() (Global, error) {
	return LoadGlobal()
}
```

- [ ] **Step 4:** `go test ./internal/config/...`

- [ ] **Step 5:** Commit

```bash
git add .
git commit -m "feat(config): global config read/write"
```

---

### Task 8: Project config (`.sshsyncer.json`)

**Files:**
- Create: `internal/config/project.go`
- Create: `internal/config/project_test.go`

Branch-keyed targets. Each target carries an `op://` server reference, a remote path, and optional per-target excludes.

- [ ] **Step 1:** Failing test

```go
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
	if err := SaveProject(path, p); err != nil { t.Fatal(err) }

	got, err := LoadProject(path)
	if err != nil { t.Fatal(err) }
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
	if err := os.MkdirAll(nested, 0o755); err != nil { t.Fatal(err) }
	got, err := FindProject(nested)
	if err != nil { t.Fatal(err) }
	if got != filepath.Join(dir, ProjectFile) {
		t.Fatalf("got %q", got)
	}
}
```

- [ ] **Step 2:** Implement `internal/config/project.go`

```go
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const ProjectFile = ".sshsyncer.json"

type Project struct {
	Version int               `json:"version"`
	Vault   string            `json:"vault"`
	Targets map[string]Target `json:"targets"`
}

type Target struct {
	Server     string   `json:"server"`      // op:// reference
	RemotePath string   `json:"remote_path"`
	Excludes   []string `json:"excludes,omitempty"`
}

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
```

- [ ] **Step 3:** `go test ./internal/config/...`

- [ ] **Step 4:** Commit

```bash
git add .
git commit -m "feat(config): project config with branch targets"
```

---

### Task 9: Git helpers

**Files:**
- Create: `internal/git/branch.go`
- Create: `internal/git/fetch.go`
- Create: `internal/git/pushed.go`
- Create: `internal/git/archive.go`
- Create: `internal/git/git_test.go`

Each helper shells out to `git`. Tests use a real temp git repo via `t.TempDir()` and `exec.Command("git", ...)` — they require git to be installed, which is documented as a build requirement.

- [ ] **Step 1:** Failing test `internal/git/git_test.go`

```go
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
	if err != nil { t.Fatal(err) }
	if b != "main" { t.Fatalf("got %q", b) }
}

func TestArchive_ExtractsPushedTree(t *testing.T) {
	dir := initRepo(t)
	// Make a bare remote and push so origin/main exists.
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
	if err := Archive(dir, "origin/main", out); err != nil { t.Fatal(err) }
	data, err := os.ReadFile(filepath.Join(out, "hello.txt"))
	if err != nil { t.Fatal(err) }
	if !strings.HasPrefix(string(data), "hi") {
		t.Fatalf("got %q", data)
	}
}
```

- [ ] **Step 2:** Implement `branch.go`

```go
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

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
```

- [ ] **Step 3:** Implement `fetch.go`

```go
package git

func Fetch(repo, remote string) error {
	if remote == "" {
		remote = "origin"
	}
	_, err := run(repo, "fetch", remote)
	return err
}
```

- [ ] **Step 4:** Implement `pushed.go`

```go
package git

import "fmt"

// HEADPushed reports whether HEAD equals origin/<branch>.
func HEADPushed(repo, branch string) (bool, error) {
	local, err := run(repo, "rev-parse", "HEAD")
	if err != nil {
		return false, err
	}
	remote, err := run(repo, "rev-parse", "origin/"+branch)
	if err != nil {
		return false, fmt.Errorf("origin/%s not found — push the branch first", branch)
	}
	return local == remote, nil
}
```

- [ ] **Step 5:** Implement `archive.go`

```go
package git

import (
	"bytes"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

// Archive exports `ref` from the repo into outDir.
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
		return errPipe(stderr.String(), err)
	}
	if err := tar.Wait(); err != nil {
		return errPipe(stderr.String(), err)
	}
	return nil
}

func errPipe(stderr string, err error) error {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return err
	}
	return io.EOF // placeholder so users see something — replace with fmt.Errorf in real impl
}
```

- [ ] **Step 6:** Replace the `errPipe` placeholder with a real error wrapper before committing

```go
func errPipe(stderr string, err error) error {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return err
	}
	return fmt.Errorf("git archive: %s", stderr)
}
```

(Update imports: add `"fmt"`.)

- [ ] **Step 7:** `go test ./internal/git/...`

- [ ] **Step 8:** Commit

```bash
git add .
git commit -m "feat(git): branch, fetch, push-check, and archive helpers"
```

---

### Task 10: rsync builder + parser + runner

**Files:**
- Create: `internal/rsync/builder.go`
- Create: `internal/rsync/parser.go`
- Create: `internal/rsync/runner.go`
- Create: `internal/rsync/rsync_test.go`

Builder takes structured input → argv. Parser turns `--dry-run` output into a `Plan{Added, Modified, Deleted}` summary. Runner streams stdout/stderr.

- [ ] **Step 1:** Failing test (builder + parser only — runner is integration-tested manually)

```go
package rsync

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildArgs(t *testing.T) {
	args := BuildArgs(Spec{
		SourceDir:    "/tmp/src/",
		User:         "deploy",
		Host:         "example.com",
		Port:         722,
		RemotePath:   "/var/www/site",
		DeployIgnore: "/tmp/src/.deployignore",
		Excludes:     []string{"node_modules"},
		IdentityFile: "/tmp/sshsyncer-pub-abc.pub",
		Delete:       true,
		DryRun:       true,
	})

	wantContains := []string{
		"-azv",
		"--delete",
		"--dry-run",
		"--exclude-from=/tmp/src/.deployignore",
		"--exclude=node_modules",
		"--exclude=.sshsyncer-log",
		"-e", "ssh -p 722 -o StrictHostKeyChecking=accept-new -o IdentitiesOnly=yes -i /tmp/sshsyncer-pub-abc.pub",
		"/tmp/src/",
		"deploy@example.com:/var/www/site",
	}
	joined := strings.Join(args, " ")
	for _, w := range wantContains {
		if !strings.Contains(joined, w) {
			t.Errorf("missing %q in args: %s", w, joined)
		}
	}
}

func TestBuildArgs_NoIdentityFile(t *testing.T) {
	args := BuildArgs(Spec{User: "u", Host: "h", Port: 22, RemotePath: "/x", SourceDir: "/y/"})
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "IdentitiesOnly") {
		t.Fatalf("should not pin identity without IdentityFile: %s", joined)
	}
}

func TestParsePlan(t *testing.T) {
	sample := `sending incremental file list
>f+++++++++ functions.php
>f.st...... assets/main.css
*deleting   old.txt
sent 1,234 bytes  received 5 bytes  ...
`
	plan := ParsePlan(sample)
	wantAdded := []string{"functions.php"}
	wantMod := []string{"assets/main.css"}
	wantDel := []string{"old.txt"}
	if !reflect.DeepEqual(plan.Added, wantAdded) { t.Errorf("added: %v", plan.Added) }
	if !reflect.DeepEqual(plan.Modified, wantMod) { t.Errorf("modified: %v", plan.Modified) }
	if !reflect.DeepEqual(plan.Deleted, wantDel) { t.Errorf("deleted: %v", plan.Deleted) }
}
```

- [ ] **Step 2:** Implement `builder.go`

```go
package rsync

import (
	"fmt"
)

type Spec struct {
	SourceDir    string
	User         string
	Host         string
	Port         int
	RemotePath   string
	DeployIgnore string   // path to project .deployignore file, may be ""
	Excludes     []string // per-target extras
	IdentityFile string   // path to a .pub file; pins agent to one key when set
	Delete       bool
	DryRun       bool
}

func BuildArgs(s Spec) []string {
	args := []string{"-azv"}
	if s.Delete {
		args = append(args, "--delete")
	}
	if s.DryRun {
		args = append(args, "--dry-run")
	}
	if s.DeployIgnore != "" {
		args = append(args, "--exclude-from="+s.DeployIgnore)
	}
	for _, e := range s.Excludes {
		args = append(args, "--exclude="+e)
	}
	args = append(args, "--exclude=.sshsyncer-log")

	port := s.Port
	if port == 0 { port = 22 }
	sshCmd := fmt.Sprintf("ssh -p %d -o StrictHostKeyChecking=accept-new", port)
	if s.IdentityFile != "" {
		sshCmd += fmt.Sprintf(" -o IdentitiesOnly=yes -i %s", s.IdentityFile)
	}
	args = append(args, "-e", sshCmd)

	src := s.SourceDir
	if src != "" && src[len(src)-1] != '/' {
		src += "/"
	}
	args = append(args, src, fmt.Sprintf("%s@%s:%s", s.User, s.Host, s.RemotePath))
	return args
}
```

- [ ] **Step 3:** Implement `parser.go`

```go
package rsync

import (
	"bufio"
	"strings"
)

type Plan struct {
	Added    []string
	Modified []string
	Deleted  []string
}

func ParsePlan(output string) Plan {
	var p Plan
	sc := bufio.NewScanner(strings.NewReader(output))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "*deleting"):
			rest := strings.TrimSpace(strings.TrimPrefix(line, "*deleting"))
			if rest != "" { p.Deleted = append(p.Deleted, rest) }
		case len(line) > 12 && (line[0] == '>' || line[0] == '<') && line[1] == 'f':
			path := strings.TrimSpace(line[12:])
			if strings.Contains(line[2:11], "+++++++++") {
				p.Added = append(p.Added, path)
			} else {
				p.Modified = append(p.Modified, path)
			}
		}
	}
	return p
}
```

- [ ] **Step 4:** Implement `runner.go`

```go
package rsync

import (
	"bytes"
	"io"
	"os/exec"
)

// Run executes rsync with the given args. stdout and stderr are streamed to out.
// Returns combined stdout for parsing on dry-runs.
func Run(args []string, out io.Writer) (string, error) {
	cmd := exec.Command("rsync", args...)
	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(out, &buf)
	cmd.Stderr = out
	err := cmd.Run()
	return buf.String(), err
}
```

- [ ] **Step 5:** `go test ./internal/rsync/...`

- [ ] **Step 6:** Commit

```bash
git add .
git commit -m "feat(rsync): build args, parse plan, exec runner"
```

---

### Task 11: SSH client (uses 1Password agent via SSH_AUTH_SOCK, with key pinning)

**Files:**
- Create: `internal/ssh/client.go`
- Create: `internal/ssh/pinkey.go`
- Create: `internal/ssh/client_test.go`

A thin wrapper over `golang.org/x/crypto/ssh` and `agent`. No keygen, no passphrase prompts — we only ever talk to the agent. When `Conn.PublicKey` is set, the dialer filters the agent's signers down to the one matching that public key, so SSH only ever offers (and prompts the user to approve) that one key. This avoids `MaxAuthTries` issues for users with many keys in 1Password and gives clear "which key authed" semantics.

- [ ] **Step 1:** Add dependency

```bash
go get golang.org/x/crypto/ssh
go get golang.org/x/crypto/ssh/agent
```

- [ ] **Step 2:** Failing test (limited — auth must be integration-tested manually)

```go
package ssh

import (
	"strings"
	"testing"
)

func TestEndpoint(t *testing.T) {
	c := Conn{User: "deploy", Host: "h.example.com", Port: 22}
	if got := c.Endpoint(); got != "h.example.com:22" {
		t.Fatalf("got %q", got)
	}
}

func TestDial_NoAgent(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	_, err := Dial(Conn{User: "u", Host: "h", Port: 22})
	if err == nil || !strings.Contains(err.Error(), "SSH_AUTH_SOCK") {
		t.Fatalf("expected SSH_AUTH_SOCK error, got %v", err)
	}
}

func TestPinPublicKey_WritesTempFile(t *testing.T) {
	pub := "ssh-ed25519 AAAAC3Nz... sshsyncer:test"
	path, cleanup, err := PinPublicKey(pub)
	if err != nil { t.Fatal(err) }
	defer cleanup()
	data, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	if string(data) != pub+"\n" {
		t.Fatalf("got %q", data)
	}
}
```

- [ ] **Step 3:** Implement `internal/ssh/client.go`

```go
package ssh

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Conn struct {
	User      string
	Host      string
	Port      int
	PublicKey string // when set, only the matching agent key is offered
}

func (c Conn) Endpoint() string {
	return c.Host + ":" + strconv.Itoa(c.Port)
}

// Dial opens an SSH connection using the agent at SSH_AUTH_SOCK
// (typically the 1Password SSH agent). If c.PublicKey is set, only the
// agent signer matching that public key is offered.
func Dial(c Conn) (*ssh.Client, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set — enable the 1Password SSH agent (or a fallback agent in CI)")
	}
	a, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("dial SSH agent: %w", err)
	}
	ag := agent.NewClient(a)

	signersFn := ag.Signers
	if c.PublicKey != "" {
		signersFn = filterSigners(ag, c.PublicKey)
	}

	cfg := &ssh.ClientConfig{
		User:            c.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeysCallback(signersFn)},
		HostKeyCallback: knownHostsCallback(),
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", c.Endpoint(), cfg)
}

// RunCommand opens a session and runs cmd. Returns combined output.
func RunCommand(client *ssh.Client, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	return string(out), err
}
```

- [ ] **Step 4:** Implement `internal/ssh/pinkey.go`

```go
package ssh

import (
	"bytes"
	"fmt"
	"os"

	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// PinPublicKey writes pub to a temp .pub file (used by rsync's -i option)
// and returns the file path and a cleanup function.
func PinPublicKey(pub string) (string, func(), error) {
	f, err := os.CreateTemp("", "sshsyncer-pub-*.pub")
	if err != nil {
		return "", nil, err
	}
	if _, err := f.WriteString(pub + "\n"); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", nil, err
	}
	cleanup := func() { os.Remove(f.Name()) }
	return f.Name(), cleanup, nil
}

// filterSigners returns a Signers function that yields only the agent signer
// whose marshalled public key matches pub.
func filterSigners(ag agent.Agent, pub string) func() ([]xssh.Signer, error) {
	return func() ([]xssh.Signer, error) {
		want, _, _, _, err := xssh.ParseAuthorizedKey([]byte(pub))
		if err != nil {
			return nil, fmt.Errorf("parse pinned public key: %w", err)
		}
		signers, err := ag.Signers()
		if err != nil {
			return nil, err
		}
		for _, s := range signers {
			if bytes.Equal(s.PublicKey().Marshal(), want.Marshal()) {
				return []xssh.Signer{s}, nil
			}
		}
		return nil, fmt.Errorf("pinned key not loaded in the SSH agent (does the 1Password vault hold it, and is the agent enabled?)")
	}
}

func knownHostsCallback() xssh.HostKeyCallback {
	home, err := os.UserHomeDir()
	if err != nil {
		return xssh.InsecureIgnoreHostKey()
	}
	cb, err := knownhosts.New(home + "/.ssh/known_hosts")
	if err != nil {
		// known_hosts missing or unreadable — first-time connection.
		// Falling back to InsecureIgnoreHostKey only for the Go SSH path;
		// rsync still uses StrictHostKeyChecking=accept-new which writes
		// the host key on first use.
		return xssh.InsecureIgnoreHostKey()
	}
	return cb
}
```

- [ ] **Step 5:** `go test ./internal/ssh/...`

- [ ] **Step 6:** Commit

```bash
git add .
git commit -m "feat(ssh): agent-backed client with pinned-key auth"
```

---

### Task 12: Deploy log (remote `.sshsyncer-log`)

**Files:**
- Create: `internal/deploylog/log.go`
- Create: `internal/deploylog/log_test.go`

Append a single-line JSON record to `<remote_path>/.sshsyncer-log` after each successful deploy. Fetch returns the last N lines.

- [ ] **Step 1:** Failing test

```go
package deploylog

import (
	"strings"
	"testing"
)

func TestFormatEntry(t *testing.T) {
	e := Entry{
		Timestamp: "2026-05-21T14:02:00Z",
		User:      "you@example.com",
		Branch:    "main",
		Commit:    "a3f9e21",
		FileCount: 184,
	}
	line := FormatEntry(e)
	for _, want := range []string{"2026-05-21T14:02:00Z", "you@example.com", "main", "a3f9e21", "184"} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %q in %q", want, line)
		}
	}
	if !strings.HasSuffix(line, "\n") {
		t.Errorf("entry must end with newline")
	}
}
```

- [ ] **Step 2:** Implement `log.go`

```go
package deploylog

import (
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

type Entry struct {
	Timestamp string `json:"ts"`
	User      string `json:"user"`
	Branch    string `json:"branch"`
	Commit    string `json:"commit"`
	FileCount int    `json:"files"`
}

func FormatEntry(e Entry) string {
	b, _ := json.Marshal(e)
	return string(b) + "\n"
}

func remoteLogPath(remotePath string) string {
	return strings.TrimSuffix(remotePath, "/") + "/.sshsyncer-log"
}

func Append(client *ssh.Client, remotePath string, e Entry) error {
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	cmd := fmt.Sprintf("cat >> %s", remoteLogPath(remotePath))
	sess.Stdin = strings.NewReader(FormatEntry(e))
	return sess.Run(cmd)
}

func Tail(client *ssh.Client, remotePath string, n int) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	cmd := fmt.Sprintf("tail -n %d %s 2>/dev/null || true", n, remoteLogPath(remotePath))
	out, err := sess.CombinedOutput(cmd)
	return string(out), err
}
```

- [ ] **Step 3:** `go test ./internal/deploylog/...`

- [ ] **Step 4:** Commit

```bash
git add .
git commit -m "feat(deploylog): append/tail remote .sshsyncer-log"
```

---

### Task 13: Embedded `.deployignore` template

**Files:**
- Create: `internal/deployignore/template.go`
- Create: `internal/deployignore/default.ignore`

Sensible defaults that match common WordPress and Node projects.

- [ ] **Step 1:** Write `internal/deployignore/default.ignore`

```
# version control
.git/
.gitignore

# OS / IDE
.DS_Store
.idea/
.vscode/

# dependency bundles
node_modules/

# sshsyncer artifacts
.sshsyncer.json
.sshsyncer-log

# common build inputs (uncomment if applicable)
# src/
# _src/
# *.scss
```

- [ ] **Step 2:** Implement `internal/deployignore/template.go`

```go
package deployignore

import _ "embed"

//go:embed default.ignore
var Default string
```

- [ ] **Step 3:** Commit

```bash
git add .
git commit -m "feat(deployignore): embedded default template"
```

---

### Task 14: `sshsyncer setup`

**Files:**
- Create: `cmd/setup.go`

Runs env detection, then provisions the dedicated sshsyncer vault. Three paths:

1. **No existing config and no vault named `sshsyncer`** → offer to create a new vault (default name `sshsyncer`, user can override).
2. **Existing config** → re-validate ownership of the configured vault.
3. **User picks an existing vault by name** → run the ownership guard; refuse unless the vault is empty or contains only `sshsyncer`-tagged items.

Idempotent: re-running with valid state just re-prints the status.

- [ ] **Step 1:** Implement `cmd/setup.go`

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	"github.com/sshsyncer/sshsyncer/internal/env"
	"github.com/sshsyncer/sshsyncer/internal/op"
	"github.com/sshsyncer/sshsyncer/internal/ui"
)

const defaultVaultName = "sshsyncer"

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "One-time per-machine setup (creates or claims the sshsyncer vault)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(ui.Title.Render("Checking environment…"))
		res := env.Detect()
		for _, c := range res.Checks {
			fmt.Println("  " + ui.Checkline(c.Name, c.OK, c.Detail))
			if !c.OK && c.Hint != "" {
				fmt.Println("      " + ui.Dim.Render(c.Hint))
			}
		}
		if !res.AllOK() {
			return fmt.Errorf("setup blocked — fix the items above and re-run")
		}

		client := op.NewClient()
		signedIn, err := client.SignedIn()
		if err != nil { return err }
		if !signedIn {
			return fmt.Errorf("not signed in to 1Password (run: eval $(op signin))")
		}

		mode := op.DetectAuthMode()
		fmt.Println("\n" + ui.Title.Render("1Password auth"))
		fmt.Println("  " + ui.Checkline(mode.String(), true, ""))
		if mode == op.AuthModeServiceAccount {
			fmt.Println("  " + ui.Dim.Render("Service account detected — note: SSH key items cannot be served via the 1Password agent under a service account."))
		}

		// Resolve target vault name.
		var name string
		if g, err := config.LoadGlobal(); err == nil && g.Vault != "" {
			name = g.Vault
			fmt.Printf("\nExisting config points at vault: %s\n", ui.KeyCol.Render(name))
		} else {
			name = defaultVaultName
			if err := ui.Input("Name for the sshsyncer vault", &name, nonEmpty); err != nil { return err }
		}

		v, found, err := client.FindVault(name)
		if err != nil { return err }
		if !found {
			ok, err := ui.Confirm(fmt.Sprintf("Vault %q does not exist. Create it now?", name), "Create", "Cancel")
			if err != nil || !ok { return fmt.Errorf("setup cancelled") }
			v, err = client.CreateVault(name, "Managed by sshsyncer")
			if err != nil { return err }
			fmt.Println("Created vault " + ui.KeyCol.Render(v.Name))
		} else {
			// Ownership guard — must be empty or fully sshsyncer-tagged.
			report, err := client.AssessVaultOwnership(name)
			if err != nil { return err }
			if report.ForeignCount > 0 {
				fmt.Println(ui.Warn.Render("Vault has items not created by sshsyncer:"))
				for _, it := range report.Foreign {
					fmt.Printf("  - %s\n", it.Title)
				}
				return fmt.Errorf("refusing to claim vault %q — sshsyncer requires a dedicated vault. Either pick a different name (e.g. %q) or delete the foreign items first", name, defaultVaultName)
			}
			fmt.Printf("Claimed existing vault %s (%d sshsyncer-owned items)\n",
				ui.KeyCol.Render(v.Name), report.OwnedCount)
		}

		if err := config.SaveGlobal(config.Global{Version: 1, Vault: v.Name}); err != nil {
			return err
		}
		fmt.Println("\nVault saved to ~/.sshsyncer/config.json")
		fmt.Println(ui.Dim.Render("Next: sshsyncer server add"))
		return nil
	},
}

func init() { rootCmd.AddCommand(setupCmd) }
```

- [ ] **Step 2:** Build and smoke test

```bash
go build -o sshsyncer . && ./sshsyncer setup --help
```

- [ ] **Step 3:** Commit

```bash
git add .
git commit -m "feat(cmd): sshsyncer setup with dedicated vault provisioning"
```

---

### Task 15: `sshsyncer doctor`

**Files:**
- Create: `cmd/doctor.go`

Same env checks as `setup`, plus 1Password signin, plus project-level checks (does `.sshsyncer.json` exist, is current branch in it, do referenced items resolve).

- [ ] **Step 1:** Implement

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	"github.com/sshsyncer/sshsyncer/internal/env"
	"github.com/sshsyncer/sshsyncer/internal/git"
	"github.com/sshsyncer/sshsyncer/internal/op"
	"github.com/sshsyncer/sshsyncer/internal/ui"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose installation and project state",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(ui.Title.Render("Environment"))
		for _, c := range env.Detect().Checks {
			fmt.Println("  " + ui.Checkline(c.Name, c.OK, c.Detail))
			if !c.OK && c.Hint != "" {
				fmt.Println("      " + ui.Dim.Render(c.Hint))
			}
		}

		fmt.Println("\n" + ui.Title.Render("1Password"))
		client := op.NewClient()
		ok, err := client.SignedIn()
		fmt.Println("  " + ui.Checkline("signed in", ok && err == nil, ""))
		if err != nil {
			fmt.Println("      " + ui.Dim.Render(err.Error()))
		}
		mode := op.DetectAuthMode()
		fmt.Println("  " + ui.Checkline("auth mode: "+mode.String(), true, ""))
		if mode == op.AuthModeServiceAccount {
			fmt.Println("      " + ui.Dim.Render("Service account — SSH keys must be supplied by a separate agent at SSH_AUTH_SOCK"))
		}

		g, err := config.LoadGlobal()
		if err == nil {
			report, vErr := client.AssessVaultOwnership(g.Vault)
			fmt.Println("  " + ui.Checkline("vault "+g.Vault, vErr == nil && report.ForeignCount == 0, ""))
			if vErr != nil {
				fmt.Println("      " + ui.Dim.Render(vErr.Error()))
			} else if report.ForeignCount > 0 {
				fmt.Println("      " + ui.Dim.Render(fmt.Sprintf("%d foreign items in the vault — investigate before re-using sshsyncer", report.ForeignCount)))
			}
		}

		fmt.Println("\n" + ui.Title.Render("Project"))
		cwd, _ := os.Getwd()
		projectPath, err := config.FindProject(cwd)
		fmt.Println("  " + ui.Checkline(".sshsyncer.json", err == nil, projectPath))
		if err != nil {
			return nil
		}
		p, err := config.LoadProject(projectPath)
		if err != nil { return err }

		branch, err := git.CurrentBranch(cwd)
		if err != nil {
			fmt.Println("  " + ui.Checkline("current branch", false, err.Error()))
			return nil
		}
		t, ok := p.Targets[branch]
		fmt.Println("  " + ui.Checkline("target for branch "+branch, ok, ""))
		if !ok {
			fmt.Println("      " + ui.Dim.Render("Run: sshsyncer target"))
			return nil
		}

		ref, err := op.ParseReference(t.Server)
		if err != nil {
			fmt.Println("  " + ui.Checkline("server reference", false, err.Error()))
			return nil
		}
		srv, err := client.GetServer(ref)
		fmt.Println("  " + ui.Checkline("resolve "+ref.String(), err == nil, fmt.Sprintf("%s@%s:%d", srv.User, srv.Host, srv.Port)))
		if err != nil {
			fmt.Println("      " + ui.Dim.Render(err.Error()))
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(doctorCmd) }
```

- [ ] **Step 2:** Build and smoke test

```bash
go build -o sshsyncer . && ./sshsyncer doctor
```

- [ ] **Step 3:** Commit

```bash
git add .
git commit -m "feat(cmd): sshsyncer doctor"
```

---

### Task 16: `sshsyncer vault`

**Files:**
- Create: `cmd/vault.go`

Show which vault sshsyncer is configured to use. Switching vaults is intentionally not supported here — re-run `sshsyncer setup` to point at a different vault (which re-runs the ownership guard).

- [ ] **Step 1:** Implement

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	"github.com/sshsyncer/sshsyncer/internal/op"
)

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Show the sshsyncer 1Password vault",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := config.RequireSetup()
		if err != nil { return err }
		fmt.Println(g.Vault)
		report, err := op.NewClient().AssessVaultOwnership(g.Vault)
		if err != nil { return err }
		fmt.Printf("  %d sshsyncer items, %d foreign\n", report.OwnedCount, report.ForeignCount)
		if report.ForeignCount > 0 {
			return fmt.Errorf("vault contains foreign items — investigate or re-run setup with a new vault")
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(vaultCmd) }
```

- [ ] **Step 2:** Commit

```bash
git add .
git commit -m "feat(cmd): sshsyncer vault (show + ownership check)"
```

---

### Task 17: `sshsyncer server` (parent + add/list/remove/test)

**Files:**
- Create: `cmd/server.go`
- Create: `cmd/server_add.go`
- Create: `cmd/server_list.go`
- Create: `cmd/server_remove.go`
- Create: `cmd/server_test.go`

- [ ] **Step 1:** `cmd/server.go`

```go
package cmd

import "github.com/spf13/cobra"

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage server credentials stored in 1Password",
}

func init() { rootCmd.AddCommand(serverCmd) }
```

- [ ] **Step 2:** `cmd/server_add.go`

The flow:
1. Prompt for title, host, user, port.
2. Pick SSH key: **generate a new one in 1Password** (recommended) or **reuse an existing sshsyncer-tagged SSH Key item from the vault**.
3. If generated: create the SSH Key item and print the public key the user needs to add to the remote server's `authorized_keys`.
4. Create the Server item with `ssh_key=op://<vault>/<key-title>`.

```go
package cmd

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	"github.com/sshsyncer/sshsyncer/internal/op"
	"github.com/sshsyncer/sshsyncer/internal/ui"
)

var serverAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Create a Server credential item in the sshsyncer vault",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := config.RequireSetup()
		if err != nil { return err }
		client := op.NewClient()

		var title, host, user, portStr string
		portStr = "22"
		if err := ui.Input("Item title (e.g. myproject-prod)", &title, nonEmpty); err != nil { return err }
		if err := ui.Input("Host", &host, nonEmpty); err != nil { return err }
		if err := ui.Input("SSH user", &user, nonEmpty); err != nil { return err }
		if err := ui.Input("SSH port", &portStr, isPort); err != nil { return err }
		port, _ := strconv.Atoi(portStr)

		keyRef, err := pickOrGenerateKey(client, g.Vault, title)
		if err != nil { return err }

		ok, err := ui.Confirm(fmt.Sprintf("Create Server item %q in vault %q?", title, g.Vault), "Create", "Cancel")
		if err != nil || !ok { return err }

		it, err := client.CreateServer(op.CreateServerInput{
			Vault: g.Vault, Title: title,
			Host: host, User: user, Port: port,
			SSHKeyRef: keyRef,
		})
		if err != nil { return err }
		fmt.Println("Saved as " + (op.Reference{Vault: it.Vault.Name, Item: it.Title}).String())
		fmt.Println(ui.Dim.Render("Verify the server accepts the key: sshsyncer server test " + it.Title))
		return nil
	},
}

func pickOrGenerateKey(client *op.Client, vault, serverTitle string) (string, error) {
	existing, err := client.ListSSHKeys(vault)
	if err != nil { return "", err }

	const (
		actGenerate = "generate"
		actPick     = "pick"
	)
	action := actGenerate
	options := []huh.Option[string]{
		huh.NewOption("Generate a new SSH key in 1Password (recommended)", actGenerate),
	}
	if len(existing) > 0 {
		options = append(options, huh.NewOption(fmt.Sprintf("Reuse an existing SSH Key item from the vault (%d available)", len(existing)), actPick))
	}
	if err := ui.Select("SSH key for this server", &action, options); err != nil { return "", err }

	if action == actPick {
		titles := make([]huh.Option[string], 0, len(existing))
		for _, it := range existing {
			titles = append(titles, huh.NewOption(it.Title, it.Title))
		}
		var picked string
		if err := ui.Select("SSH key", &picked, titles); err != nil { return "", err }
		return (op.Reference{Vault: vault, Item: picked}).String(), nil
	}

	// Generate
	keyTitle := serverTitle + "-key"
	if err := ui.Input("New SSH key title", &keyTitle, nonEmpty); err != nil { return "", err }
	key, err := client.GenerateSSHKey(vault, keyTitle)
	if err != nil { return "", err }

	fmt.Println("\n" + ui.Title.Render("Public key — add this to the remote server's authorized_keys"))
	fmt.Println()
	fmt.Println("  " + key.PublicKey)
	fmt.Println()
	fmt.Println(ui.Dim.Render("One-liner (run on the remote, replacing $USER if needed):"))
	fmt.Printf(ui.Dim.Render("  mkdir -p ~/.ssh && echo %q >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys\n"), key.PublicKey)
	if _, err := ui.Confirm("Have you added the public key to the server?", "Yes, continue", "Cancel"); err != nil {
		return "", err
	}
	return key.Ref.String(), nil
}

func nonEmpty(s string) error {
	if s == "" { return fmt.Errorf("required") }
	return nil
}

func isPort(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("must be 1-65535")
	}
	return nil
}

func init() { serverCmd.AddCommand(serverAddCmd) }
```

- [ ] **Step 3:** `cmd/server_list.go`

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	"github.com/sshsyncer/sshsyncer/internal/op"
	"github.com/sshsyncer/sshsyncer/internal/ui"
)

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sshsyncer server items in the default vault",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := config.RequireSetup()
		if err != nil { return err }
		c := op.NewClient()
		items, err := c.ListServers(g.Vault)
		if err != nil { return err }
		fmt.Println("Vault: " + g.Vault)
		if len(items) == 0 {
			fmt.Println(ui.Dim.Render("  (no server items — run: sshsyncer server add)"))
			return nil
		}
		for _, it := range items {
			srv, err := c.GetServer(op.Reference{Vault: it.Vault.Name, Item: it.Title})
			if err != nil {
				fmt.Printf("  %-24s  %s\n", it.Title, ui.Dim.Render("(failed to resolve)"))
				continue
			}
			fmt.Printf("  %-24s  %s@%s:%d\n", it.Title, srv.User, srv.Host, srv.Port)
		}
		return nil
	},
}

func init() { serverCmd.AddCommand(serverListCmd) }
```

- [ ] **Step 4:** `cmd/server_remove.go`

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	"github.com/sshsyncer/sshsyncer/internal/op"
	"github.com/sshsyncer/sshsyncer/internal/ui"
)

var serverRemoveCmd = &cobra.Command{
	Use:   "remove <title>",
	Short: "Delete an sshsyncer server item from the default vault",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := config.RequireSetup()
		if err != nil { return err }
		ok, err := ui.Confirm(fmt.Sprintf("Delete %q from vault %q?", args[0], g.Vault), "Delete", "Cancel")
		if err != nil || !ok { return err }
		c := op.NewClient()
		return c.DeleteServer(op.Reference{Vault: g.Vault, Item: args[0]})
	},
}

func init() { serverCmd.AddCommand(serverRemoveCmd) }
```

- [ ] **Step 5:** `cmd/server_test.go`

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	sshpkg "github.com/sshsyncer/sshsyncer/internal/ssh"
	"github.com/sshsyncer/sshsyncer/internal/config"
	"github.com/sshsyncer/sshsyncer/internal/op"
)

var serverTestCmd = &cobra.Command{
	Use:   "test <title>",
	Short: "Open an SSH connection to verify credentials work",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := config.RequireSetup()
		if err != nil { return err }
		c := op.NewClient()
		srv, err := c.GetServer(op.Reference{Vault: g.Vault, Item: args[0]})
		if err != nil { return err }

		keyRef, err := op.ParseReference(srv.SSHKeyRef)
		if err != nil { return err }
		pub, err := c.ReadPublicKey(keyRef)
		if err != nil { return err }

		fmt.Printf("Connecting to %s@%s:%d (key %s)…\n", srv.User, srv.Host, srv.Port, keyRef.Item)
		client, err := sshpkg.Dial(sshpkg.Conn{
			User: srv.User, Host: srv.Host, Port: srv.Port,
			PublicKey: pub,
		})
		if err != nil { return err }
		defer client.Close()
		out, err := sshpkg.RunCommand(client, "echo ok")
		if err != nil { return fmt.Errorf("connected but command failed: %w", err) }
		fmt.Println(out)
		return nil
	},
}

func init() { serverCmd.AddCommand(serverTestCmd) }
```

- [ ] **Step 6:** Build and smoke test

```bash
go build -o sshsyncer . && ./sshsyncer server --help
```

- [ ] **Step 7:** Commit

```bash
git add .
git commit -m "feat(cmd): sshsyncer server add/list/remove/test"
```

---

### Task 18: `sshsyncer init`

**Files:**
- Create: `cmd/init.go`

Walks the user through: pick a server from the vault, type a remote path, write `.sshsyncer.json` + `.deployignore`.

- [ ] **Step 1:** Implement

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	"github.com/sshsyncer/sshsyncer/internal/deployignore"
	gitPkg "github.com/sshsyncer/sshsyncer/internal/git"
	"github.com/sshsyncer/sshsyncer/internal/op"
	"github.com/sshsyncer/sshsyncer/internal/ui"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise sshsyncer in the current project",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := config.RequireSetup()
		if err != nil { return err }
		cwd, err := os.Getwd()
		if err != nil { return err }
		if _, err := os.Stat(filepath.Join(cwd, config.ProjectFile)); err == nil {
			return fmt.Errorf("%s already exists in %s", config.ProjectFile, cwd)
		}

		branch, err := gitPkg.CurrentBranch(cwd)
		if err != nil { return err }

		client := op.NewClient()
		items, err := client.ListServers(g.Vault)
		if err != nil { return err }
		if len(items) == 0 {
			return fmt.Errorf("no server items in vault %q — run: sshsyncer server add", g.Vault)
		}

		options := make([]huh.Option[string], 0, len(items))
		for _, it := range items {
			options = append(options, huh.NewOption(it.Title, it.Title))
		}
		var picked string
		if err := ui.Select("Server for branch "+branch, &picked, options); err != nil { return err }

		var remotePath string
		if err := ui.Input("Remote path", &remotePath, nonEmpty); err != nil { return err }

		proj := config.Project{
			Version: 1,
			Vault:   g.Vault,
			Targets: map[string]config.Target{
				branch: {
					Server:     (op.Reference{Vault: g.Vault, Item: picked}).String(),
					RemotePath: remotePath,
				},
			},
		}
		if err := config.SaveProject(filepath.Join(cwd, config.ProjectFile), proj); err != nil { return err }

		ignorePath := filepath.Join(cwd, ".deployignore")
		if _, err := os.Stat(ignorePath); os.IsNotExist(err) {
			if err := os.WriteFile(ignorePath, []byte(deployignore.Default), 0o644); err != nil { return err }
		}

		fmt.Println("Wrote " + config.ProjectFile)
		fmt.Println("Wrote .deployignore")
		fmt.Println(ui.Dim.Render("Commit both files:  git add .sshsyncer.json .deployignore && git commit"))
		return nil
	},
}

func init() { rootCmd.AddCommand(initCmd) }
```

- [ ] **Step 2:** Commit

```bash
git add .
git commit -m "feat(cmd): sshsyncer init"
```

---

### Task 19: `sshsyncer target` (show/edit/remove/list)

**Files:**
- Create: `cmd/target.go`
- Create: `cmd/target_edit.go`
- Create: `cmd/target_remove.go`
- Create: `cmd/target_list.go`

No-arg `target` shows current branch's target; subcommands cover the CRUD.

- [ ] **Step 1:** `cmd/target.go`

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	gitPkg "github.com/sshsyncer/sshsyncer/internal/git"
	"github.com/sshsyncer/sshsyncer/internal/op"
	"github.com/sshsyncer/sshsyncer/internal/ui"
)

var targetCmd = &cobra.Command{
	Use:   "target",
	Short: "Show or manage the current branch's deploy target",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := config.RequireSetup(); err != nil { return err }
		cwd, _ := os.Getwd()
		projPath, err := config.FindProject(cwd)
		if err != nil { return err }
		p, err := config.LoadProject(projPath)
		if err != nil { return err }
		branch, err := gitPkg.CurrentBranch(cwd)
		if err != nil { return err }
		t, ok := p.Targets[branch]
		if !ok {
			fmt.Printf("Branch %s has no target. Use `sshsyncer target edit` to add one.\n", branch)
			return nil
		}
		ref, _ := op.ParseReference(t.Server)
		client := op.NewClient()
		srv, err := client.GetServer(ref)
		fmt.Printf("Branch:       %s\n", branch)
		fmt.Printf("Server:       %s    (%s)\n", ref.Item, ref.String())
		if err == nil {
			fmt.Printf("Connects to:  %s@%s:%d\n", srv.User, srv.Host, srv.Port)
		} else {
			fmt.Println(ui.Dim.Render("Connects to:  (could not resolve item)"))
		}
		fmt.Printf("Remote path:  %s\n", t.RemotePath)
		if len(t.Excludes) == 0 {
			fmt.Println("Excludes:     (uses .deployignore only)")
		} else {
			fmt.Printf("Excludes:     %v\n", t.Excludes)
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(targetCmd) }
```

- [ ] **Step 2:** `cmd/target_edit.go` — same flow as `init` but on an existing project file, for the current branch

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	gitPkg "github.com/sshsyncer/sshsyncer/internal/git"
	"github.com/sshsyncer/sshsyncer/internal/op"
	"github.com/sshsyncer/sshsyncer/internal/ui"
)

var targetEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Create or update the current branch's target",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := config.RequireSetup()
		if err != nil { return err }
		cwd, _ := os.Getwd()
		projPath, err := config.FindProject(cwd)
		if err != nil { return err }
		p, err := config.LoadProject(projPath)
		if err != nil { return err }
		branch, err := gitPkg.CurrentBranch(cwd)
		if err != nil { return err }

		vault := p.Vault
		if vault == "" { vault = g.Vault }
		if vault != g.Vault {
			return fmt.Errorf("project vault %q does not match this machine's sshsyncer vault %q — re-run setup or edit .sshsyncer.json", p.Vault, g.Vault)
		}

		client := op.NewClient()
		items, err := client.ListServers(vault)
		if err != nil { return err }
		if len(items) == 0 {
			return fmt.Errorf("no server items in vault %q — run: sshsyncer server add", vault)
		}

		current := p.Targets[branch]
		picked := ""
		if ref, err := op.ParseReference(current.Server); err == nil { picked = ref.Item }

		options := make([]huh.Option[string], 0, len(items))
		for _, it := range items {
			options = append(options, huh.NewOption(it.Title, it.Title))
		}
		if err := ui.Select("Server for "+branch, &picked, options); err != nil { return err }

		remotePath := current.RemotePath
		if err := ui.Input("Remote path", &remotePath, nonEmpty); err != nil { return err }

		p.Targets[branch] = config.Target{
			Server:     (op.Reference{Vault: vault, Item: picked}).String(),
			RemotePath: remotePath,
			Excludes:   current.Excludes,
		}
		if err := config.SaveProject(projPath, p); err != nil { return err }
		fmt.Println("Saved.")
		return nil
	},
}

func init() { targetCmd.AddCommand(targetEditCmd) }
```

- [ ] **Step 3:** `cmd/target_remove.go`

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	gitPkg "github.com/sshsyncer/sshsyncer/internal/git"
	"github.com/sshsyncer/sshsyncer/internal/ui"
)

var targetRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the current branch's target",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := config.RequireSetup(); err != nil { return err }
		cwd, _ := os.Getwd()
		projPath, err := config.FindProject(cwd)
		if err != nil { return err }
		p, err := config.LoadProject(projPath)
		if err != nil { return err }
		branch, err := gitPkg.CurrentBranch(cwd)
		if err != nil { return err }
		if _, ok := p.Targets[branch]; !ok {
			return fmt.Errorf("branch %s has no target", branch)
		}
		ok, err := ui.Confirm("Remove target for "+branch+"?", "Remove", "Cancel")
		if err != nil || !ok { return err }
		delete(p.Targets, branch)
		return config.SaveProject(projPath, p)
	},
}

func init() { targetCmd.AddCommand(targetRemoveCmd) }
```

- [ ] **Step 4:** `cmd/target_list.go`

```go
package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
)

var targetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all branch → target mappings",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := config.RequireSetup(); err != nil { return err }
		cwd, _ := os.Getwd()
		projPath, err := config.FindProject(cwd)
		if err != nil { return err }
		p, err := config.LoadProject(projPath)
		if err != nil { return err }
		if len(p.Targets) == 0 {
			fmt.Println("No targets.")
			return nil
		}
		branches := make([]string, 0, len(p.Targets))
		for b := range p.Targets { branches = append(branches, b) }
		sort.Strings(branches)
		for _, b := range branches {
			t := p.Targets[b]
			fmt.Printf("%-20s  %-30s  %s\n", b, t.Server, t.RemotePath)
		}
		return nil
	},
}

func init() { targetCmd.AddCommand(targetListCmd) }
```

- [ ] **Step 5:** Commit

```bash
git add .
git commit -m "feat(cmd): sshsyncer target show/edit/remove/list"
```

---

### Task 20: `sshsyncer push`

**Files:**
- Create: `cmd/push.go`

The hot path. Orchestrates: load config → fetch → push-check → resolve server → archive → rsync (dry/real) → log.

- [ ] **Step 1:** Implement

```go
package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	"github.com/sshsyncer/sshsyncer/internal/deploylog"
	gitPkg "github.com/sshsyncer/sshsyncer/internal/git"
	"github.com/sshsyncer/sshsyncer/internal/op"
	rsyncPkg "github.com/sshsyncer/sshsyncer/internal/rsync"
	sshpkg "github.com/sshsyncer/sshsyncer/internal/ssh"
	"github.com/sshsyncer/sshsyncer/internal/ui"
)

var pushDry bool

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Deploy the current branch to its target",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := config.RequireSetup(); err != nil { return err }
		cwd, _ := os.Getwd()
		projPath, err := config.FindProject(cwd)
		if err != nil { return err }
		repo := strings.TrimSuffix(projPath, "/"+config.ProjectFile)

		p, err := config.LoadProject(projPath)
		if err != nil { return err }
		branch, err := gitPkg.CurrentBranch(repo)
		if err != nil { return err }
		t, ok := p.Targets[branch]
		if !ok {
			return fmt.Errorf("no target for branch %q. Run: sshsyncer target edit", branch)
		}
		ref, err := op.ParseReference(t.Server)
		if err != nil { return err }

		fmt.Println(ui.Title.Render("sshsyncer push"))
		fmt.Printf("  branch:  %s\n", branch)
		fmt.Printf("  server:  %s\n", ref.Item)
		fmt.Printf("  remote:  %s\n", t.RemotePath)
		fmt.Println()

		fmt.Print("Fetching origin… ")
		if err := gitPkg.Fetch(repo, "origin"); err != nil { return err }
		fmt.Println(ui.OK.String())

		fmt.Print("Checking branch is pushed… ")
		ok, err = gitPkg.HEADPushed(repo, branch)
		if err != nil { return err }
		if !ok { return fmt.Errorf("HEAD is not pushed to origin/%s — push first", branch) }
		fmt.Println(ui.OK.String())

		fmt.Print("Resolving credentials from 1Password… ")
		client := op.NewClient()
		srv, err := client.GetServer(ref)
		if err != nil { return err }
		keyRef, err := op.ParseReference(srv.SSHKeyRef)
		if err != nil { return err }
		pub, err := client.ReadPublicKey(keyRef)
		if err != nil { return err }
		fmt.Println(ui.OK.String())

		pubPath, cleanupPub, err := sshpkg.PinPublicKey(pub)
		if err != nil { return err }
		defer cleanupPub()

		fmt.Print("Exporting origin/" + branch + " via git archive… ")
		tmp, err := os.MkdirTemp("", "sshsyncer-*")
		if err != nil { return err }
		defer os.RemoveAll(tmp)
		if err := gitPkg.Archive(repo, "origin/"+branch, tmp); err != nil { return err }
		fmt.Println(ui.OK.String())

		spec := rsyncPkg.Spec{
			SourceDir:    tmp,
			User:         srv.User,
			Host:         srv.Host,
			Port:         srv.Port,
			RemotePath:   t.RemotePath,
			DeployIgnore: repo + "/.deployignore",
			Excludes:     t.Excludes,
			IdentityFile: pubPath,
			Delete:       true,
			DryRun:       pushDry,
		}
		args2 := rsyncPkg.BuildArgs(spec)
		fmt.Println("Running rsync…")
		out, err := rsyncPkg.Run(args2, os.Stdout)
		if err != nil { return err }

		if pushDry {
			plan := rsyncPkg.ParsePlan(out)
			fmt.Printf("\n(dry run) %d new, %d modified, %d deleted\n",
				len(plan.Added), len(plan.Modified), len(plan.Deleted))
			return nil
		}

		fmt.Print("Connecting to log destination… ")
		sshClient, err := sshpkg.Dial(sshpkg.Conn{
			User: srv.User, Host: srv.Host, Port: srv.Port,
			PublicKey: pub,
		})
		if err != nil { return err }
		defer sshClient.Close()
		fmt.Println(ui.OK.String())

		head, _ := gitPkg.HeadShort(repo) // tiny helper, see below
		whoami := os.Getenv("USER")
		err = deploylog.Append(sshClient, t.RemotePath, deploylog.Entry{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			User:      whoami,
			Branch:    branch,
			Commit:    head,
			FileCount: countTransferred(out),
		})
		if err != nil { return err }
		fmt.Println("Done.")
		return nil
	},
}

func countTransferred(out string) int {
	plan := rsyncPkg.ParsePlan(out)
	return len(plan.Added) + len(plan.Modified)
}

func init() {
	pushCmd.Flags().BoolVar(&pushDry, "dry", false, "Preview without applying")
	rootCmd.AddCommand(pushCmd)
}
```

- [ ] **Step 2:** Add the small helper `HeadShort` in `internal/git/branch.go`

```go
func HeadShort(repo string) (string, error) {
	out, err := run(repo, "rev-parse", "--short", "HEAD")
	if err != nil { return "", err }
	return strings.TrimSpace(out), nil
}
```

- [ ] **Step 3:** Smoke build

```bash
go build -o sshsyncer . && ./sshsyncer push --help
```

- [ ] **Step 4:** Commit

```bash
git add .
git commit -m "feat(cmd): sshsyncer push (dry + real)"
```

---

### Task 21: `sshsyncer log`

**Files:**
- Create: `cmd/log.go`

Tails the remote `.sshsyncer-log` over SSH.

- [ ] **Step 1:** Implement

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	"github.com/sshsyncer/sshsyncer/internal/deploylog"
	gitPkg "github.com/sshsyncer/sshsyncer/internal/git"
	"github.com/sshsyncer/sshsyncer/internal/op"
	sshpkg "github.com/sshsyncer/sshsyncer/internal/ssh"
)

var logLines int

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "View deploy history for the current branch",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := config.RequireSetup(); err != nil { return err }
		cwd, _ := os.Getwd()
		projPath, err := config.FindProject(cwd)
		if err != nil { return err }
		p, err := config.LoadProject(projPath)
		if err != nil { return err }
		branch, err := gitPkg.CurrentBranch(cwd)
		if err != nil { return err }
		t, ok := p.Targets[branch]
		if !ok {
			return fmt.Errorf("no target for branch %q", branch)
		}
		ref, err := op.ParseReference(t.Server)
		if err != nil { return err }
		opc := op.NewClient()
		srv, err := opc.GetServer(ref)
		if err != nil { return err }
		keyRef, err := op.ParseReference(srv.SSHKeyRef)
		if err != nil { return err }
		pub, err := opc.ReadPublicKey(keyRef)
		if err != nil { return err }
		client, err := sshpkg.Dial(sshpkg.Conn{
			User: srv.User, Host: srv.Host, Port: srv.Port,
			PublicKey: pub,
		})
		if err != nil { return err }
		defer client.Close()
		out, err := deploylog.Tail(client, t.RemotePath, logLines)
		if err != nil { return err }
		fmt.Print(out)
		return nil
	},
}

func init() {
	logCmd.Flags().IntVarP(&logLines, "lines", "n", 20, "Number of entries")
	rootCmd.AddCommand(logCmd)
}
```

- [ ] **Step 2:** Commit

```bash
git add .
git commit -m "feat(cmd): sshsyncer log"
```

---

### Task 22: `sshsyncer docs`

**Files:**
- Create: `cmd/docs.go`
- Create: `cmd/docs_reference.md` (embedded)

Prints the full reference. Use `//go:embed`.

- [ ] **Step 1:** Write `cmd/docs_reference.md` — a single markdown file covering every command and config file (see README section).

- [ ] **Step 2:** Implement `cmd/docs.go`

```go
package cmd

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
)

//go:embed docs_reference.md
var docsReference string

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Print the full sshsyncer reference",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(docsReference)
		return nil
	},
}

func init() { rootCmd.AddCommand(docsCmd) }
```

- [ ] **Step 3:** Commit

```bash
git add .
git commit -m "feat(cmd): sshsyncer docs"
```

---

### Task 23: README + LICENSE + GoReleaser

**Files:**
- Modify: `README.md` (replace placeholder)
- Create: `LICENSE` (MIT)
- Create: `.goreleaser.yaml`
- Create: `docs/architecture.md`

- [ ] **Step 1:** README sections: What it is, How it differs from copy-paste deploys, Prereqs (1Password + op + GNU rsync), Install, Quick start, Command reference, Config file shapes, Troubleshooting, Contributing.

- [ ] **Step 2:** GoReleaser config for cross-platform binaries + Homebrew tap (`sshsyncer/homebrew-tap` placeholder repo name — adjust at release time).

- [ ] **Step 3:** `docs/architecture.md` covering the trust model (key material lives in 1Password agent, never on sshsyncer-managed disk; host key verification via `~/.ssh/known_hosts`).

- [ ] **Step 4:** Commit

```bash
git add .
git commit -m "docs: README, LICENSE, GoReleaser, architecture"
```

---

### Task 24: End-to-end smoke test against a real server

Not an automated test — a manual checklist the contributor runs before tagging a release.

- [ ] `sshsyncer setup` on a clean machine → creates a new `sshsyncer` vault in 1Password
- [ ] Re-run `sshsyncer setup` → idempotent, ownership guard passes
- [ ] `sshsyncer setup` and pick the name of an existing populated personal vault → refused with the ownership guard message
- [ ] `sshsyncer server add` → generates a new SSH Key item, prints the public key, creates Server item with `ssh_key` reference. Paste the printed public key into a test server's `authorized_keys`.
- [ ] `sshsyncer server test <name>` → connects via pinned key, prints `ok`. 1Password biometric-prompts exactly once.
- [ ] In a sample repo: `sshsyncer init` → writes `.sshsyncer.json` (containing the vault name and an `op://` server reference) and `.deployignore`
- [ ] `sshsyncer push --dry` → shows expected file list
- [ ] `sshsyncer push` → succeeds, files appear on server, deploy log entry written
- [ ] `sshsyncer log` → shows the new entry
- [ ] `sshsyncer doctor` → all checks pass, auth mode shown
- [ ] `sshsyncer doctor` on a fresh machine without `op` → fails loudly with the install link
- [ ] `sshsyncer doctor` with `OP_SERVICE_ACCOUNT_TOKEN` set → detects service-account mode and warns about SSH agent

- [ ] **Commit:** N/A (this is a release-gate task)

---

## Self-review

**Spec coverage** — every command from the agreed surface (`setup`, `vault`, `server add/list/remove/test`, `init`, `target` + `edit/remove/list`, `push` + `--dry`, `log`, `doctor`, `docs`) maps to a task. The shared building blocks (`internal/op`, `internal/git`, `internal/rsync`, `internal/ssh`, `internal/deploylog`, `internal/deployignore`, `internal/config`, `internal/env`, `internal/ui`) are each covered before the commands that consume them. The newly required pieces — dedicated-vault provisioning + ownership guard (Tasks 5, 14), pinned-key SSH (Tasks 6, 6b, 10, 11, 17, 20, 21), and auth-mode detection (Tasks 5, 14, 15) — each have a dedicated home.

**Placeholder scan** — Task 9 Step 5 deliberately leaves an `errPipe` placeholder that Step 6 immediately replaces, to keep the worked example concrete. No other TBDs.

**Type consistency** — `Conn`, `Server` (now with `SSHKeyRef`), `SSHKey`, `Vault`, `Reference`, `Project` (now with `Vault`), `Target`, `Entry`, `Plan`, `Spec` (now with `IdentityFile`), and `Item` are defined once and reused consistently across packages. `op.Reference` is the single string-form of `op://Vault/Item[/Field]`. `config.Global.Vault` and `config.Project.Vault` both refer to the dedicated sshsyncer vault.

**Vault ownership invariant** — sshsyncer never operates on a 1Password vault that contains items it did not create. Enforced at three points: setup (refuses to claim a foreign-populated vault), `doctor` (reports ForeignCount), and `vault` (errors when ForeignCount > 0).

**Auth-mode invariant** — `op.DetectAuthMode()` is the single source of truth. Setup, doctor, and the README all surface the same desktop-app-vs-service-account distinction.

**Open items deferred by design:**
- Importing an existing private key into 1Password from the CLI — out of scope; users do this in the desktop app, then pick the resulting SSH Key item via `server add`.
- Host-key bootstrap UX (`StrictHostKeyChecking=accept-new` is used for rsync; the Go SSH path uses `~/.ssh/known_hosts`, falling back to insecure-ignore on first connection). Documented in `architecture.md`.
- Sharing the sshsyncer vault with a team — done entirely in 1Password's UI; sshsyncer's setup flow handles claim-by-second-machine via the ownership guard.
