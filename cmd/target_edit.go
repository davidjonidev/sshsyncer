package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/davidjonidev/sshsyncer/internal/config"
	gitPkg "github.com/davidjonidev/sshsyncer/internal/git"
	"github.com/davidjonidev/sshsyncer/internal/op"
	"github.com/davidjonidev/sshsyncer/internal/ui"
)

var targetEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Create or update the current branch's target",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := config.RequireSetup()
		if err != nil {
			return err
		}
		cwd, _ := os.Getwd()
		projPath, err := config.FindProject(cwd)
		if err != nil {
			return err
		}
		p, err := config.LoadProject(projPath)
		if err != nil {
			return err
		}
		branch, err := gitPkg.CurrentBranch(cwd)
		if err != nil {
			return err
		}

		vault := p.Vault
		if vault == "" {
			vault = g.Vault
		}
		if vault != g.Vault {
			return fmt.Errorf("project vault %q does not match this machine's sshsyncer vault %q — re-run setup or edit .sshsyncer.json", p.Vault, g.Vault)
		}

		client := op.NewClient()
		items, err := client.ListServers(vault)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return fmt.Errorf("no server items in vault %q — run: sshsyncer server add", vault)
		}

		current := p.Targets[branch]
		picked := ""
		if ref, err := op.ParseReference(current.Server); err == nil {
			picked = ref.Item
		}

		options := make([]huh.Option[string], 0, len(items))
		for _, it := range items {
			options = append(options, huh.NewOption(it.Title, it.Title))
		}
		if err := ui.Select("Server for "+branch, &picked, options); err != nil {
			return err
		}

		remotePath := current.RemotePath
		if err := ui.Input("Remote path", &remotePath, nonEmpty); err != nil {
			return err
		}

		if p.Targets == nil {
			p.Targets = map[string]config.Target{}
		}
		p.Targets[branch] = config.Target{
			Server:     (op.Reference{Vault: vault, Item: picked}).String(),
			RemotePath: remotePath,
			Excludes:   current.Excludes,
		}
		if err := config.SaveProject(projPath, p); err != nil {
			return err
		}
		fmt.Println("Saved.")
		return nil
	},
}

func init() { targetCmd.AddCommand(targetEditCmd) }
