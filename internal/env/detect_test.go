package env

import "testing"

func TestDetect_ReturnsAllChecks(t *testing.T) {
	r := Detect()
	names := map[string]bool{}
	for _, c := range r.Checks {
		names[c.Name] = true
	}
	for _, want := range []string{"git", "rsync", "op", "1password-agent"} {
		if !names[want] {
			t.Errorf("missing check %q", want)
		}
	}
}

func TestIsGNURsync(t *testing.T) {
	cases := map[string]bool{
		"rsync  version 3.2.7  protocol version 31\n": true,
		"openrsync: protocol version 29\n":            false,
		"":                                            false,
	}
	for in, want := range cases {
		if got := isGNURsync(in); got != want {
			t.Errorf("isGNURsync(%q) = %v, want %v", in, got, want)
		}
	}
}
