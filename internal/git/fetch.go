package git

// Fetch runs `git fetch <remote>` (default "origin").
func Fetch(repo, remote string) error {
	if remote == "" {
		remote = "origin"
	}
	_, err := run(repo, "fetch", remote)
	return err
}
