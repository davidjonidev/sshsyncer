package deploylog

import (
	"strings"
	"testing"
)

func TestFormatEntry(t *testing.T) {
	e := Entry{
		Timestamp: "2026-05-21T14:02:00Z",
		User:      "you@example.com",
		Branch:    "main",
		Commit:    "a3f9e21",
		FileCount: 184,
	}
	line := FormatEntry(e)
	for _, want := range []string{"2026-05-21T14:02:00Z", "you@example.com", "main", "a3f9e21", "184"} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %q in %q", want, line)
		}
	}
	if !strings.HasSuffix(line, "\n") {
		t.Errorf("entry must end with newline")
	}
}

func TestRemoteLogPath(t *testing.T) {
	cases := map[string]string{
		"/var/www/site":  "/var/www/site/.sshsyncer-log",
		"/var/www/site/": "/var/www/site/.sshsyncer-log",
	}
	for in, want := range cases {
		if got := remoteLogPath(in); got != want {
			t.Errorf("remoteLogPath(%q) = %q, want %q", in, got, want)
		}
	}
}
