package op

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Runner abstracts execution of the `op` CLI so tests can fake it.
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

// Client wraps the `op` CLI.
type Client struct {
	runner Runner
}

// NewClient returns a Client backed by the real `op` binary on PATH.
func NewClient() *Client { return &Client{runner: execRunner{}} }

// Read fetches a single value via a secret reference (`op read`).
func (c *Client) Read(reference string) (string, error) {
	out, err := c.runner.Run("read", reference)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Item is the JSON shape `op item get` returns.
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

// ItemField is a single field on an op Item.
type ItemField struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// Field returns the value of the named field on the item, case-insensitively.
func (it Item) Field(label string) (string, bool) {
	for _, f := range it.Fields {
		if strings.EqualFold(f.Label, label) {
			return f.Value, true
		}
	}
	return "", false
}

// ReadItem returns the full JSON shape of a single op item.
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

// Tag is the 1Password item tag that marks items sshsyncer owns.
const Tag = "sshsyncer"

// ErrNotSignedIn is returned when no 1Password account is signed in.
var ErrNotSignedIn = errors.New("not signed in to 1Password (run: eval $(op signin))")
