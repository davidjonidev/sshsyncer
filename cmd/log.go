package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/davidjonidev/sshsyncer/internal/config"
	"github.com/davidjonidev/sshsyncer/internal/deploylog"
	gitPkg "github.com/davidjonidev/sshsyncer/internal/git"
	"github.com/davidjonidev/sshsyncer/internal/op"
	sshpkg "github.com/davidjonidev/sshsyncer/internal/ssh"
)

var logLines int

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "View deploy history for the current branch",
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
			return fmt.Errorf("no target for branch %q", branch)
		}
		ref, err := op.ParseReference(t.Server)
		if err != nil {
			return err
		}
		opc := op.NewClient()
		srv, err := opc.GetServer(ref)
		if err != nil {
			return err
		}
		keyRef, err := op.ParseReference(srv.SSHKeyRef)
		if err != nil {
			return err
		}
		pub, err := opc.ReadPublicKey(keyRef)
		if err != nil {
			return err
		}
		client, err := sshpkg.Dial(sshpkg.Conn{
			User: srv.User, Host: srv.Host, Port: srv.Port,
			PublicKey: pub,
		})
		if err != nil {
			return err
		}
		defer client.Close()
		out, err := deploylog.Tail(client, t.RemotePath, logLines)
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	},
}

func init() {
	logCmd.Flags().IntVarP(&logLines, "lines", "n", 20, "Number of entries")
	rootCmd.AddCommand(logCmd)
}
