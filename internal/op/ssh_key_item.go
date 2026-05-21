package op

import (
	"encoding/json"
	"fmt"
)

// SSHKey is a resolved 1Password SSH Key item.
type SSHKey struct {
	Ref       Reference
	Title     string
	PublicKey string
	KeyType   string
}

// GenerateSSHKey creates a new SSH Key item with 1Password generating the
// private key. The private key never leaves 1Password.
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

// ListSSHKeys returns every sshsyncer-tagged SSH Key item in the vault.
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

// GetSSHKey resolves an SSH Key item to its structured form.
func (c *Client) GetSSHKey(ref Reference) (SSHKey, error) {
	it, err := c.ReadItem(ref)
	if err != nil {
		return SSHKey{}, err
	}
	return sshKeyFromItem(it), nil
}

// ReadPublicKey is the fast path used at deploy time — fetches just the
// public_key field of an SSH Key item via `op read`.
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
