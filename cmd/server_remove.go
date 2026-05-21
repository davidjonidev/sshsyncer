package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/davidjonidev/sshsyncer/internal/config"
	"github.com/davidjonidev/sshsyncer/internal/op"
	"github.com/davidjonidev/sshsyncer/internal/ui"
)

var serverRemoveCmd = &cobra.Command{
	Use:   "remove <title>",
	Short: "Delete an sshsyncer server item from the vault",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := config.RequireSetup()
		if err != nil {
			return err
		}
		ok, err := ui.Confirm(fmt.Sprintf("Delete %q from vault %q?", args[0], g.Vault), "Delete", "Cancel")
		if err != nil || !ok {
			return err
		}
		c := op.NewClient()
		return c.DeleteServer(op.Reference{Vault: g.Vault, Item: args[0]})
	},
}

func init() { serverCmd.AddCommand(serverRemoveCmd) }
