package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/davidjonidev/sshsyncer/internal/config"
	"github.com/davidjonidev/sshsyncer/internal/op"
)

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Show the sshsyncer 1Password vault",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := config.RequireSetup()
		if err != nil {
			return err
		}
		fmt.Println(g.Vault)
		report, err := op.NewClient().AssessVaultOwnership(g.Vault)
		if err != nil {
			return err
		}
		fmt.Printf("  %d sshsyncer items, %d foreign\n", report.OwnedCount, report.ForeignCount)
		if report.ForeignCount > 0 {
			return fmt.Errorf("vault contains foreign items — investigate or re-run setup with a new vault")
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(vaultCmd) }
