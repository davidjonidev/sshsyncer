package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/davidjonidev/sshsyncer/internal/config"
	"github.com/davidjonidev/sshsyncer/internal/env"
	"github.com/davidjonidev/sshsyncer/internal/op"
	"github.com/davidjonidev/sshsyncer/internal/ui"
)

const defaultVaultName = "sshsyncer"

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "One-time per-machine setup (creates or claims the sshsyncer vault)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(ui.Title.Render("Checking environment…"))
		res := env.Detect()
		for _, c := range res.Checks {
			fmt.Println("  " + ui.Checkline(c.Name, c.OK, c.Detail))
			if !c.OK && c.Hint != "" {
				fmt.Println("      " + ui.Dim.Render(c.Hint))
			}
		}
		if !res.AllOK() {
			return fmt.Errorf("setup blocked — fix the items above and re-run")
		}

		client := op.NewClient()
		signedIn, err := client.SignedIn()
		if err != nil {
			return err
		}
		if !signedIn {
			return fmt.Errorf("not signed in to 1Password (run: eval $(op signin))")
		}

		mode := op.DetectAuthMode()
		fmt.Println("\n" + ui.Title.Render("1Password auth"))
		fmt.Println("  " + ui.Checkline(mode.String(), true, ""))
		if mode == op.AuthModeServiceAccount {
			fmt.Println("  " + ui.Dim.Render("Service account detected — note: SSH key items cannot be served via the 1Password agent under a service account."))
		}

		// Resolve target vault name.
		var name string
		if g, err := config.LoadGlobal(); err == nil && g.Vault != "" {
			name = g.Vault
			fmt.Printf("\nExisting config points at vault: %s\n", ui.KeyCol.Render(name))
		} else {
			name = defaultVaultName
			if err := ui.Input("Name for the sshsyncer vault", &name, nonEmpty); err != nil {
				return err
			}
		}

		v, found, err := client.FindVault(name)
		if err != nil {
			return err
		}
		if !found {
			ok, err := ui.Confirm(fmt.Sprintf("Vault %q does not exist. Create it now?", name), "Create", "Cancel")
			if err != nil || !ok {
				return fmt.Errorf("setup cancelled")
			}
			v, err = client.CreateVault(name, "Managed by sshsyncer")
			if err != nil {
				return err
			}
			fmt.Println("Created vault " + ui.KeyCol.Render(v.Name))
		} else {
			report, err := client.AssessVaultOwnership(name)
			if err != nil {
				return err
			}
			if report.ForeignCount > 0 {
				fmt.Println(ui.Warn.Render("Vault has items not created by sshsyncer:"))
				for _, it := range report.Foreign {
					fmt.Printf("  - %s\n", it.Title)
				}
				return fmt.Errorf("refusing to claim vault %q — sshsyncer requires a dedicated vault. Either pick a different name (e.g. %q) or delete the foreign items first", name, defaultVaultName)
			}
			fmt.Printf("Claimed existing vault %s (%d sshsyncer-owned items)\n",
				ui.KeyCol.Render(v.Name), report.OwnedCount)
		}

		if err := config.SaveGlobal(config.Global{Version: 1, Vault: v.Name}); err != nil {
			return err
		}
		fmt.Println("\nVault saved to ~/.sshsyncer/config.json")
		fmt.Println(ui.Dim.Render("Next: sshsyncer server add"))
		return nil
	},
}

func init() { rootCmd.AddCommand(setupCmd) }
