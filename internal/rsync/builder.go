package rsync

import "fmt"

// Spec is the input to BuildArgs.
type Spec struct {
	SourceDir    string
	User         string
	Host         string
	Port         int
	RemotePath   string
	DeployIgnore string   // path to project .deployignore file, may be ""
	Excludes     []string // per-target extras
	IdentityFile string   // path to a .pub file; pins agent to one key when set
	Delete       bool
	DryRun       bool
}

// BuildArgs returns the argv for `rsync` given the spec.
func BuildArgs(s Spec) []string {
	args := []string{"-azv"}
	if s.Delete {
		args = append(args, "--delete")
	}
	if s.DryRun {
		args = append(args, "--dry-run")
	}
	if s.DeployIgnore != "" {
		args = append(args, "--exclude-from="+s.DeployIgnore)
	}
	for _, e := range s.Excludes {
		args = append(args, "--exclude="+e)
	}
	args = append(args, "--exclude=.sshsyncer-log")

	port := s.Port
	if port == 0 {
		port = 22
	}
	sshCmd := fmt.Sprintf("ssh -p %d -o StrictHostKeyChecking=accept-new", port)
	if s.IdentityFile != "" {
		sshCmd += fmt.Sprintf(" -o IdentitiesOnly=yes -i %s", s.IdentityFile)
	}
	args = append(args, "-e", sshCmd)

	src := s.SourceDir
	if src != "" && src[len(src)-1] != '/' {
		src += "/"
	}
	args = append(args, src, fmt.Sprintf("%s@%s:%s", s.User, s.Host, s.RemotePath))
	return args
}
