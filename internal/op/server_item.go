package op

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Server is a resolved server credential (host/user/port + pinned SSH key).
type Server struct {
	Ref       Reference
	Title     string
	Host      string
	User      string
	Port      int
	SSHKeyRef string // op:// reference to a 1Password SSH Key item
}

// CreateServerInput is the parameter object for CreateServer.
type CreateServerInput struct {
	Vault     string
	Title     string
	Host      string
	User      string
	Port      int
	SSHKeyRef string // required: op:// reference to an existing SSH Key item
}

// ListServers returns every Server item in the vault tagged `sshsyncer`.
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

// GetServer resolves a Server item to its structured form.
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

// CreateServer creates a new Server item in the vault.
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

// DeleteServer removes a Server item.
func (c *Client) DeleteServer(ref Reference) error {
	_, err := c.runner.Run("item", "delete", ref.Item, "--vault", ref.Vault)
	return err
}
