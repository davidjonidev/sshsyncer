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
	if err != nil {
		t.Fatal(err)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "myproject-key" {
		t.Fatalf("bad list: %+v", items)
	}
}

func TestReadPublicKey(t *testing.T) {
	fr := &fakeRunner{stdouts: []string{"ssh-ed25519 AAAA test\n"}, errs: []error{nil}}
	c := &Client{runner: fr}
	pk, err := c.ReadPublicKey(Reference{Vault: "sshsyncer", Item: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if pk != "ssh-ed25519 AAAA test" {
		t.Fatalf("got %q", pk)
	}
	got := fr.args[0]
	if got[0] != "read" || got[1] != "op://sshsyncer/k/public_key" {
		t.Fatalf("unexpected args: %v", got)
	}
}
