package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	"github.com/sshsyncer/sshsyncer/internal/deployignore"
	gitPkg "github.com/sshsyncer/sshsyncer/internal/git"
	"github.com/sshsyncer/sshsyncer/internal/op"
	"github.com/sshsyncer/sshsyncer/internal/ui"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise sshsyncer in the current project",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := config.RequireSetup()
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(cwd, config.ProjectFile)); err == nil {
			return fmt.Errorf("%s already exists in %s", config.ProjectFile, cwd)
		}

		branch, err := gitPkg.CurrentBranch(cwd)
		if err != nil {
			return err
		}

		client := op.NewClient()
		items, err := client.ListServers(g.Vault)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return fmt.Errorf("no server items in vault %q — run: sshsyncer server add", g.Vault)
		}

		options := make([]huh.Option[string], 0, len(items))
		for _, it := range items {
			options = append(options, huh.NewOption(it.Title, it.Title))
		}
		var picked string
		if err := ui.Select("Server for branch "+branch, &picked, options); err != nil {
			return err
		}

		var remotePath string
		if err := ui.Input("Remote path", &remotePath, nonEmpty); err != nil {
			return err
		}

		proj := config.Project{
			Version: 1,
			Vault:   g.Vault,
			Targets: map[string]config.Target{
				branch: {
					Server:     (op.Reference{Vault: g.Vault, Item: picked}).String(),
					RemotePath: remotePath,
				},
			},
		}
		if err := config.SaveProject(filepath.Join(cwd, config.ProjectFile), proj); err != nil {
			return err
		}

		ignorePath := filepath.Join(cwd, ".deployignore")
		if _, err := os.Stat(ignorePath); os.IsNotExist(err) {
			if err := os.WriteFile(ignorePath, []byte(deployignore.Default), 0o644); err != nil {
				return err
			}
		}

		fmt.Println("Wrote " + config.ProjectFile)
		fmt.Println("Wrote .deployignore")
		fmt.Println(ui.Dim.Render("Commit both files:  git add .sshsyncer.json .deployignore && git commit"))
		return nil
	},
}

func init() { rootCmd.AddCommand(initCmd) }
