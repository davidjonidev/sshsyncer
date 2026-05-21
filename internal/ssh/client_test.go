package ssh

import (
	"os"
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
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != pub+"\n" {
		t.Fatalf("got %q", data)
	}
}
