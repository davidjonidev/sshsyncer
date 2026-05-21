package rsync

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildArgs(t *testing.T) {
	args := BuildArgs(Spec{
		SourceDir:    "/tmp/src/",
		User:         "deploy",
		Host:         "example.com",
		Port:         722,
		RemotePath:   "/var/www/site",
		DeployIgnore: "/tmp/src/.deployignore",
		Excludes:     []string{"node_modules"},
		IdentityFile: "/tmp/sshsyncer-pub-abc.pub",
		Delete:       true,
		DryRun:       true,
	})

	wantContains := []string{
		"-azv",
		"--delete",
		"--dry-run",
		"--exclude-from=/tmp/src/.deployignore",
		"--exclude=node_modules",
		"--exclude=.sshsyncer-log",
		"-e", "ssh -p 722 -o StrictHostKeyChecking=accept-new -o IdentitiesOnly=yes -i /tmp/sshsyncer-pub-abc.pub",
		"/tmp/src/",
		"deploy@example.com:/var/www/site",
	}
	joined := strings.Join(args, " ")
	for _, w := range wantContains {
		if !strings.Contains(joined, w) {
			t.Errorf("missing %q in args: %s", w, joined)
		}
	}
}

func TestBuildArgs_NoIdentityFile(t *testing.T) {
	args := BuildArgs(Spec{User: "u", Host: "h", Port: 22, RemotePath: "/x", SourceDir: "/y/"})
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "IdentitiesOnly") {
		t.Fatalf("should not pin identity without IdentityFile: %s", joined)
	}
}

func TestParsePlan(t *testing.T) {
	sample := `sending incremental file list
>f+++++++++ functions.php
>f.st...... assets/main.css
*deleting   old.txt
sent 1,234 bytes  received 5 bytes  ...
`
	plan := ParsePlan(sample)
	wantAdded := []string{"functions.php"}
	wantMod := []string{"assets/main.css"}
	wantDel := []string{"old.txt"}
	if !reflect.DeepEqual(plan.Added, wantAdded) {
		t.Errorf("added: %v", plan.Added)
	}
	if !reflect.DeepEqual(plan.Modified, wantMod) {
		t.Errorf("modified: %v", plan.Modified)
	}
	if !reflect.DeepEqual(plan.Deleted, wantDel) {
		t.Errorf("deleted: %v", plan.Deleted)
	}
}
