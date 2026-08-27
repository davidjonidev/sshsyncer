package ssh

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// hostKeyCallback verifies a host key against knownHostsPath using the same
// policy the rsync path already applies via StrictHostKeyChecking=accept-new:
//
//   - an unseen host is accepted on first connection AND recorded
//   - a host already on file must present the same key
//   - a changed key is always refused
//
// There is deliberately no branch that skips verification. The previous
// implementation returned InsecureIgnoreHostKey() whenever known_hosts was
// missing or unreadable, which accepted every key and recorded none, so the
// connection never became verifiable on any later attempt. Two transports with
// two different host-key policies is the bug; this makes them one policy.
func hostKeyCallback(knownHostsPath string) xssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key xssh.PublicKey) error {
		if err := ensureKnownHosts(knownHostsPath); err != nil {
			return fmt.Errorf("prepare known_hosts: %w", err)
		}

		verify, err := knownhosts.New(knownHostsPath)
		if err != nil {
			return fmt.Errorf("read known_hosts: %w", err)
		}

		err = verify(hostname, remote, key)
		if err == nil {
			return nil
		}

		// A KeyError with no Want entries means the host is simply unknown.
		// Anything else, including a Want that disagrees with what the server
		// presented, is a refusal.
		var ke *knownhosts.KeyError
		if errors.As(err, &ke) && len(ke.Want) == 0 {
			return appendKnownHost(knownHostsPath, hostname, key)
		}
		return err
	}
}

// ensureKnownHosts makes sure the file exists so knownhosts.New has something
// to parse. An empty known_hosts is valid and simply knows no hosts yet.
func ensureKnownHosts(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	return f.Close()
}

// appendKnownHost records a host key so the NEXT connection is verified rather
// than trusted. This is the half that InsecureIgnoreHostKey() never did.
func appendKnownHost(path, hostname string, key xssh.PublicKey) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("open known_hosts for append: %w", err)
	}
	defer f.Close()

	line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("record host key: %w", err)
	}
	return nil
}

// knownHostsCallback resolves the default known_hosts path and verifies
// against it. If the home directory cannot be resolved it fails closed: a host
// key that cannot be checked is not a host key that should be trusted.
func knownHostsCallback() xssh.HostKeyCallback {
	home, err := os.UserHomeDir()
	if err != nil {
		return func(string, net.Addr, xssh.PublicKey) error {
			return fmt.Errorf("cannot locate a home directory to verify the host key against: %w", err)
		}
	}
	return hostKeyCallback(filepath.Join(home, ".ssh", "known_hosts"))
}
