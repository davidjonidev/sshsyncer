package cmd

import "github.com/spf13/cobra"

var version = "dev"

// SetVersion injects the build-time version into the root command.
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

var rootCmd = &cobra.Command{
	Use:     "sshsyncer",
	Short:   "Deploy a git branch to a remote server via rsync over SSH",
	Long:    "sshsyncer deploys the last pushed commit of the current branch to a server defined in a dedicated 1Password vault.",
	Version: version,
}

// Execute runs the root command.
func Execute() error { return rootCmd.Execute() }
