package ssh

import (
	"bytes"
	"fmt"
	"os"

	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// PinPublicKey writes pub to a temp .pub file (used by rsync's -i option)
// and returns the file path plus a cleanup function.
func PinPublicKey(pub string) (string, func(), error) {
	f, err := os.CreateTemp("", "sshsyncer-pub-*.pub")
	if err != nil {
		return "", nil, err
	}
	if _, err := f.WriteString(pub + "\n"); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", nil, err
	}
	cleanup := func() { os.Remove(f.Name()) }
	return f.Name(), cleanup, nil
}

// filterSigners returns a Signers function that yields only the agent signer
// whose marshalled public key matches `pub`.
func filterSigners(ag agent.Agent, pub string) func() ([]xssh.Signer, error) {
	return func() ([]xssh.Signer, error) {
		want, _, _, _, err := xssh.ParseAuthorizedKey([]byte(pub))
		if err != nil {
			return nil, fmt.Errorf("parse pinned public key: %w", err)
		}
		signers, err := ag.Signers()
		if err != nil {
			return nil, err
		}
		for _, s := range signers {
			if bytes.Equal(s.PublicKey().Marshal(), want.Marshal()) {
				return []xssh.Signer{s}, nil
			}
		}
		return nil, fmt.Errorf("pinned key not loaded in the SSH agent (is the 1Password vault holding it, and the agent enabled?)")
	}
}
