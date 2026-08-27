package ssh

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	xssh "golang.org/x/crypto/ssh"
)

// testKey returns a deterministic host key usable as an xssh.PublicKey.
func testKey(t *testing.T, seed string) xssh.PublicKey {
	t.Helper()
	// Two distinct, valid ed25519 authorized-keys lines. Content differs by
	// seed so the mismatch case has something to disagree with.
	lines := map[string]string{
		"a": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJ1XvYQ4bLwx1qHqM0z8yqf8rWJ0k0mQhVQfE0lNn7Xy host-a",
		"b": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIB2Yb9mFf0xC5Q1kQ8YtRr6P7nJ3sW4vZ0aD9cE1gH2i host-b",
	}
	line, ok := lines[seed]
	if !ok {
		t.Fatalf("unknown seed %q", seed)
	}
	pk, _, _, _, err := xssh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		t.Fatalf("parse test key %q: %v", seed, err)
	}
	return pk
}

func addr(t *testing.T) net.Addr {
	t.Helper()
	a, err := net.ResolveTCPAddr("tcp", "203.0.113.10:22")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return a
}

// An unseen host must be ACCEPTED and RECORDED, matching the accept-new policy
// the rsync path already uses. Before this existed the Go path returned
// InsecureIgnoreHostKey(), which accepted without recording, so the connection
// stayed unverified forever instead of only on the first one.
func TestHostKeyCallback_UnknownHostIsRecordedAndAccepted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "known_hosts")
	cb := hostKeyCallback(path)

	if err := cb("example.com:22", addr(t), testKey(t, "a")); err != nil {
		t.Fatalf("first connection to an unknown host should be accepted, got: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("known_hosts should have been created: %v", err)
	}
	if !strings.Contains(string(b), "ssh-ed25519") {
		t.Fatalf("host key was not recorded; file contains: %q", string(b))
	}
}

// Having recorded a key, the same key must keep verifying.
func TestHostKeyCallback_KnownHostWithSameKeyPasses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	cb := hostKeyCallback(path)

	if err := cb("example.com:22", addr(t), testKey(t, "a")); err != nil {
		t.Fatalf("first connection: %v", err)
	}
	if err := cb("example.com:22", addr(t), testKey(t, "a")); err != nil {
		t.Fatalf("second connection with the same key should pass, got: %v", err)
	}
}

// The security-critical case. A host whose key has CHANGED must be refused,
// whatever else is going on. This is the case InsecureIgnoreHostKey() silently
// allowed.
func TestHostKeyCallback_ChangedKeyIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	cb := hostKeyCallback(path)

	if err := cb("example.com:22", addr(t), testKey(t, "a")); err != nil {
		t.Fatalf("first connection: %v", err)
	}
	err := cb("example.com:22", addr(t), testKey(t, "b"))
	if err == nil {
		t.Fatal("a changed host key MUST be refused, but the callback accepted it")
	}
}

// A different host is independent: recording one must not vouch for another.
func TestHostKeyCallback_DifferentHostIsIndependent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	cb := hostKeyCallback(path)

	if err := cb("one.example.com:22", addr(t), testKey(t, "a")); err != nil {
		t.Fatalf("first host: %v", err)
	}
	if err := cb("two.example.com:22", addr(t), testKey(t, "b")); err != nil {
		t.Fatalf("a second, unseen host should be accepted on first sight: %v", err)
	}
	b, _ := os.ReadFile(path)
	if got := strings.Count(string(b), "ssh-ed25519"); got != 2 {
		t.Fatalf("expected both hosts recorded, found %d entries in: %q", got, string(b))
	}
}
