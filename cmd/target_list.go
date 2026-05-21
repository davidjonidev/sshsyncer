package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/davidjonidev/sshsyncer/internal/config"
)

var targetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all branch → target mappings",
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
		if len(p.Targets) == 0 {
			fmt.Println("No targets.")
			return nil
		}
		branches := make([]string, 0, len(p.Targets))
		for b := range p.Targets {
			branches = append(branches, b)
		}
		sort.Strings(branches)
		for _, b := range branches {
			t := p.Targets[b]
			fmt.Printf("%-20s  %-30s  %s\n", b, t.Server, t.RemotePath)
		}
		return nil
	},
}

func init() { targetCmd.AddCommand(targetListCmd) }
