package git

import "fmt"

// HEADPushed reports whether HEAD equals origin/<branch>.
func HEADPushed(repo, branch string) (bool, error) {
	local, err := run(repo, "rev-parse", "HEAD")
	if err != nil {
		return false, err
	}
	remote, err := run(repo, "rev-parse", "origin/"+branch)
	if err != nil {
		return false, fmt.Errorf("origin/%s not found — push the branch first", branch)
	}
	return local == remote, nil
}
