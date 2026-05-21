package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/davidjonidev/sshsyncer/internal/config"
	gitPkg "github.com/davidjonidev/sshsyncer/internal/git"
	"github.com/davidjonidev/sshsyncer/internal/ui"
)

var targetRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the current branch's target",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := config.RequireSetup(); err != nil {
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
		if _, ok := p.Targets[branch]; !ok {
			return fmt.Errorf("branch %s has no target", branch)
		}
		ok, err := ui.Confirm("Remove target for "+branch+"?", "Remove", "Cancel")
		if err != nil || !ok {
			return err
		}
		delete(p.Targets, branch)
		return config.SaveProject(projPath, p)
	},
}

func init() { targetCmd.AddCommand(targetRemoveCmd) }
