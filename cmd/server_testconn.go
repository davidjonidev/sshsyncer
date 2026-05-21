package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	"github.com/sshsyncer/sshsyncer/internal/op"
	sshpkg "github.com/sshsyncer/sshsyncer/internal/ssh"
)

var serverTestCmd = &cobra.Command{
	Use:   "test <title>",
	Short: "Open an SSH connection to verify credentials work",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := config.RequireSetup()
		if err != nil {
			return err
		}
		c := op.NewClient()
		srv, err := c.GetServer(op.Reference{Vault: g.Vault, Item: args[0]})
		if err != nil {
			return err
		}

		keyRef, err := op.ParseReference(srv.SSHKeyRef)
		if err != nil {
			return err
		}
		pub, err := c.ReadPublicKey(keyRef)
		if err != nil {
			return err
		}

		fmt.Printf("Connecting to %s@%s:%d (key %s)…\n", srv.User, srv.Host, srv.Port, keyRef.Item)
		client, err := sshpkg.Dial(sshpkg.Conn{
			User: srv.User, Host: srv.Host, Port: srv.Port,
			PublicKey: pub,
		})
		if err != nil {
			return err
		}
		defer client.Close()
		out, err := sshpkg.RunCommand(client, "echo ok")
		if err != nil {
			return fmt.Errorf("connected but command failed: %w", err)
		}
		fmt.Println(out)
		return nil
	},
}

func init() { serverCmd.AddCommand(serverTestCmd) }
