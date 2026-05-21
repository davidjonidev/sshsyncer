package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/davidjonidev/sshsyncer/internal/config"
	"github.com/davidjonidev/sshsyncer/internal/env"
	gitPkg "github.com/davidjonidev/sshsyncer/internal/git"
	"github.com/davidjonidev/sshsyncer/internal/op"
	"github.com/davidjonidev/sshsyncer/internal/ui"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose installation and project state",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(ui.Title.Render("Environment"))
		for _, c := range env.Detect().Checks {
			fmt.Println("  " + ui.Checkline(c.Name, c.OK, c.Detail))
			if !c.OK && c.Hint != "" {
				fmt.Println("      " + ui.Dim.Render(c.Hint))
			}
		}

		fmt.Println("\n" + ui.Title.Render("1Password"))
		client := op.NewClient()
		ok, err := client.SignedIn()
		fmt.Println("  " + ui.Checkline("signed in", ok && err == nil, ""))
		if err != nil {
			fmt.Println("      " + ui.Dim.Render(err.Error()))
		}
		mode := op.DetectAuthMode()
		fmt.Println("  " + ui.Checkline("auth mode: "+mode.String(), true, ""))
		if mode == op.AuthModeServiceAccount {
			fmt.Println("      " + ui.Dim.Render("Service account — SSH keys must be supplied by a separate agent at SSH_AUTH_SOCK"))
		}

		g, err := config.LoadGlobal()
		if err == nil {
			report, vErr := client.AssessVaultOwnership(g.Vault)
			fmt.Println("  " + ui.Checkline("vault "+g.Vault, vErr == nil && report.ForeignCount == 0, ""))
			if vErr != nil {
				fmt.Println("      " + ui.Dim.Render(vErr.Error()))
			} else if report.ForeignCount > 0 {
				fmt.Println("      " + ui.Dim.Render(fmt.Sprintf("%d foreign items in the vault — investigate before re-using sshsyncer", report.ForeignCount)))
			}
		}

		fmt.Println("\n" + ui.Title.Render("Project"))
		cwd, _ := os.Getwd()
		projectPath, err := config.FindProject(cwd)
		fmt.Println("  " + ui.Checkline(".sshsyncer.json", err == nil, projectPath))
		if err != nil {
			return nil
		}
		p, err := config.LoadProject(projectPath)
		if err != nil {
			return err
		}

		branch, err := gitPkg.CurrentBranch(cwd)
		if err != nil {
			fmt.Println("  " + ui.Checkline("current branch", false, err.Error()))
			return nil
		}
		t, ok := p.Targets[branch]
		fmt.Println("  " + ui.Checkline("target for branch "+branch, ok, ""))
		if !ok {
			fmt.Println("      " + ui.Dim.Render("Run: sshsyncer target edit"))
			return nil
		}

		ref, err := op.ParseReference(t.Server)
		if err != nil {
			fmt.Println("  " + ui.Checkline("server reference", false, err.Error()))
			return nil
		}
		srv, err := client.GetServer(ref)
		fmt.Println("  " + ui.Checkline("resolve "+ref.String(), err == nil, fmt.Sprintf("%s@%s:%d", srv.User, srv.Host, srv.Port)))
		if err != nil {
			fmt.Println("      " + ui.Dim.Render(err.Error()))
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(doctorCmd) }
