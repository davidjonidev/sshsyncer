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

func TestListVaults(t *testing.T) {
	fr := &fakeRunner{stdouts: []string{`[{"id":"abc","name":"sshsyncer"},{"id":"def","name":"Private"}]`}, errs: []error{nil}}
	c := &Client{runner: fr}
	vs, err := c.ListVaults()
	if err != nil {
		t.Fatal(err)
	}
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
	if err != nil {
		t.Fatal(err)
	}
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
