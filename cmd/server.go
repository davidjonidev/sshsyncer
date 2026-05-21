package cmd

import "github.com/spf13/cobra"

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage server credentials stored in the sshsyncer 1Password vault",
}

func init() { rootCmd.AddCommand(serverCmd) }
