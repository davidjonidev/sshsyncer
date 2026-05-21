package ssh

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Conn describes an SSH endpoint.
type Conn struct {
	User      string
	Host      string
	Port      int
	PublicKey string // when set, only the matching agent key is offered
}

// Endpoint returns "host:port".
func (c Conn) Endpoint() string {
	return c.Host + ":" + strconv.Itoa(c.Port)
}

// Dial opens an SSH connection using the agent at SSH_AUTH_SOCK
// (typically the 1Password SSH agent). When c.PublicKey is set, only the
// agent signer matching that public key is offered.
func Dial(c Conn) (*ssh.Client, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set — enable the 1Password SSH agent (or a fallback agent in CI)")
	}
	a, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("dial SSH agent: %w", err)
	}
	ag := agent.NewClient(a)

	signersFn := ag.Signers
	if c.PublicKey != "" {
		signersFn = filterSigners(ag, c.PublicKey)
	}

	cfg := &ssh.ClientConfig{
		User:            c.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeysCallback(signersFn)},
		HostKeyCallback: knownHostsCallback(),
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", c.Endpoint(), cfg)
}

// RunCommand opens a session and runs cmd, returning combined output.
func RunCommand(client *ssh.Client, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	return string(out), err
}
