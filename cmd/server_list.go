package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/davidjonidev/sshsyncer/internal/config"
	"github.com/davidjonidev/sshsyncer/internal/op"
	"github.com/davidjonidev/sshsyncer/internal/ui"
)

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sshsyncer server items in the vault",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := config.RequireSetup()
		if err != nil {
			return err
		}
		c := op.NewClient()
		items, err := c.ListServers(g.Vault)
		if err != nil {
			return err
		}
		fmt.Println("Vault: " + g.Vault)
		if len(items) == 0 {
			fmt.Println(ui.Dim.Render("  (no server items — run: sshsyncer server add)"))
			return nil
		}
		for _, it := range items {
			srv, err := c.GetServer(op.Reference{Vault: it.Vault.Name, Item: it.Title})
			if err != nil {
				fmt.Printf("  %-24s  %s\n", it.Title, ui.Dim.Render("(failed to resolve)"))
				continue
			}
			fmt.Printf("  %-24s  %s@%s:%d\n", it.Title, srv.User, srv.Host, srv.Port)
		}
		return nil
	},
}

func init() { serverCmd.AddCommand(serverListCmd) }
