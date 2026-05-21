package ui

import (
	"strings"
	"testing"
)

func TestCheckline_OK(t *testing.T) {
	out := Checkline("git", true, "2.43.0")
	if !strings.Contains(out, "git") || !strings.Contains(out, "2.43.0") {
		t.Fatalf("unexpected: %q", out)
	}
}

func TestCheckline_Fail(t *testing.T) {
	out := Checkline("op", false, "")
	if !strings.Contains(out, "op") {
		t.Fatalf("unexpected: %q", out)
	}
}
