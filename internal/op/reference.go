package op

import (
	"fmt"
	"strings"
)

// Reference represents an op://Vault/Item[/Field] secret reference.
type Reference struct {
	Vault string
	Item  string
	Field string // optional; may include nested path "section/field"
}

// ParseReference parses a string in the form op://Vault/Item[/Field].
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

// String returns the canonical op:// form.
func (r Reference) String() string {
	if r.Field == "" {
		return fmt.Sprintf("op://%s/%s", r.Vault, r.Item)
	}
	return fmt.Sprintf("op://%s/%s/%s", r.Vault, r.Item, r.Field)
}

// ItemRef returns the reference stripped of any Field component.
func (r Reference) ItemRef() Reference {
	return Reference{Vault: r.Vault, Item: r.Item}
}
