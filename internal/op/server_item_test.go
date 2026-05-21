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
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "myproject-prod" {
		t.Fatalf("bad list: %+v", items)
	}

	srv, err := c.GetServer(Reference{Vault: "sshsyncer", Item: "myproject-prod"})
	if err != nil {
		t.Fatal(err)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	args := fr.args[0]
	want := []string{"item", "create", "--category", "Server", "--vault", "sshsyncer", "--title", "new",
		"--tags", "sshsyncer", "host=h", "username=u", "port=22",
		"ssh_key[text]=op://sshsyncer/new-key", "--format", "json"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("\n got: %v\nwant: %v", args, want)
	}
	var it Item
	if err := json.Unmarshal([]byte(fr.stdouts[0]), &it); err != nil {
		t.Fatal(err)
	}
}

func TestServerItem_CreateRequiresKeyRef(t *testing.T) {
	c := &Client{runner: &fakeRunner{}}
	_, err := c.CreateServer(CreateServerInput{Vault: "v", Title: "t", Host: "h", User: "u"})
	if err == nil {
		t.Fatal("expected error: ssh_key is required")
	}
}
