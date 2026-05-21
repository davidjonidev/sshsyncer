package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/davidjonidev/sshsyncer/internal/config"
	gitPkg "github.com/davidjonidev/sshsyncer/internal/git"
	"github.com/davidjonidev/sshsyncer/internal/op"
	"github.com/davidjonidev/sshsyncer/internal/ui"
)

var targetCmd = &cobra.Command{
	Use:   "target",
	Short: "Show or manage the current branch's deploy target",
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
		t, ok := p.Targets[branch]
		if !ok {
			fmt.Printf("Branch %s has no target. Use `sshsyncer target edit` to add one.\n", branch)
			return nil
		}
		ref, _ := op.ParseReference(t.Server)
		client := op.NewClient()
		srv, err := client.GetServer(ref)
		fmt.Printf("Branch:       %s\n", branch)
		fmt.Printf("Server:       %s    (%s)\n", ref.Item, ref.String())
		if err == nil {
			fmt.Printf("Connects to:  %s@%s:%d\n", srv.User, srv.Host, srv.Port)
		} else {
			fmt.Println(ui.Dim.Render("Connects to:  (could not resolve item)"))
		}
		fmt.Printf("Remote path:  %s\n", t.RemotePath)
		if len(t.Excludes) == 0 {
			fmt.Println("Excludes:     (uses .deployignore only)")
		} else {
			fmt.Printf("Excludes:     %v\n", t.Excludes)
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(targetCmd) }
