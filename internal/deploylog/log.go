package deploylog

import (
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// Entry is a single line in the remote .sshsyncer-log.
type Entry struct {
	Timestamp string `json:"ts"`
	User      string `json:"user"`
	Branch    string `json:"branch"`
	Commit    string `json:"commit"`
	FileCount int    `json:"files"`
}

// FormatEntry serialises an Entry to a single newline-terminated JSON line.
func FormatEntry(e Entry) string {
	b, _ := json.Marshal(e)
	return string(b) + "\n"
}

func remoteLogPath(remotePath string) string {
	return strings.TrimSuffix(remotePath, "/") + "/.sshsyncer-log"
}

// Append writes a single Entry to the remote .sshsyncer-log.
func Append(client *ssh.Client, remotePath string, e Entry) error {
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	cmd := fmt.Sprintf("cat >> %s", remoteLogPath(remotePath))
	sess.Stdin = strings.NewReader(FormatEntry(e))
	return sess.Run(cmd)
}

// Tail returns the last n lines of the remote .sshsyncer-log.
func Tail(client *ssh.Client, remotePath string, n int) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	cmd := fmt.Sprintf("tail -n %d %s 2>/dev/null || true", n, remoteLogPath(remotePath))
	out, err := sess.CombinedOutput(cmd)
	return string(out), err
}
