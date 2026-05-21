package op

import (
	"encoding/json"
	"fmt"
)

// Vault is a 1Password vault.
type Vault struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListVaults returns every vault accessible to the current op account.
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

// FindVault returns the vault with the given name, or (zero, false, nil) if not present.
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

// OwnershipReport categorises items in a vault by whether sshsyncer owns them.
type OwnershipReport struct {
	OwnedCount   int
	ForeignCount int
	Foreign      []Item // first few foreign items, for the error message
}

// AssessVaultOwnership scans the vault and counts owned vs foreign items.
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
