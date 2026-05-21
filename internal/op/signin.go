package op

import "strings"

// SignedIn reports whether at least one 1Password account is signed in.
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
