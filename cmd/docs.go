package cmd

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
)

//go:embed docs_reference.md
var docsReference string

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Print the full sshsyncer reference",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(docsReference)
		return nil
	},
}

func init() { rootCmd.AddCommand(docsCmd) }
