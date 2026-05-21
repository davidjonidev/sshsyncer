package cmd

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/sshsyncer/sshsyncer/internal/config"
	"github.com/sshsyncer/sshsyncer/internal/op"
	"github.com/sshsyncer/sshsyncer/internal/ui"
)

var serverAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Create a Server credential item in the sshsyncer vault",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := config.RequireSetup()
		if err != nil {
			return err
		}
		client := op.NewClient()

		var title, host, user string
		portStr := "22"
		if err := ui.Input("Item title (e.g. myproject-prod)", &title, nonEmpty); err != nil {
			return err
		}
		if err := ui.Input("Host", &host, nonEmpty); err != nil {
			return err
		}
		if err := ui.Input("SSH user", &user, nonEmpty); err != nil {
			return err
		}
		if err := ui.Input("SSH port", &portStr, isPort); err != nil {
			return err
		}
		port, _ := strconv.Atoi(portStr)

		keyRef, err := pickOrGenerateKey(client, g.Vault, title)
		if err != nil {
			return err
		}

		ok, err := ui.Confirm(fmt.Sprintf("Create Server item %q in vault %q?", title, g.Vault), "Create", "Cancel")
		if err != nil || !ok {
			return err
		}

		it, err := client.CreateServer(op.CreateServerInput{
			Vault: g.Vault, Title: title,
			Host: host, User: user, Port: port,
			SSHKeyRef: keyRef,
		})
		if err != nil {
			return err
		}
		fmt.Println("Saved as " + (op.Reference{Vault: it.Vault.Name, Item: it.Title}).String())
		fmt.Println(ui.Dim.Render("Verify the server accepts the key: sshsyncer server test " + it.Title))
		return nil
	},
}

func pickOrGenerateKey(client *op.Client, vault, serverTitle string) (string, error) {
	existing, err := client.ListSSHKeys(vault)
	if err != nil {
		return "", err
	}

	const (
		actGenerate = "generate"
		actPick     = "pick"
	)
	action := actGenerate
	options := []huh.Option[string]{
		huh.NewOption("Generate a new SSH key in 1Password (recommended)", actGenerate),
	}
	if len(existing) > 0 {
		options = append(options, huh.NewOption(fmt.Sprintf("Reuse an existing SSH Key item from the vault (%d available)", len(existing)), actPick))
	}
	if err := ui.Select("SSH key for this server", &action, options); err != nil {
		return "", err
	}

	if action == actPick {
		titles := make([]huh.Option[string], 0, len(existing))
		for _, it := range existing {
			titles = append(titles, huh.NewOption(it.Title, it.Title))
		}
		var picked string
		if err := ui.Select("SSH key", &picked, titles); err != nil {
			return "", err
		}
		return (op.Reference{Vault: vault, Item: picked}).String(), nil
	}

	// Generate
	keyTitle := serverTitle + "-key"
	if err := ui.Input("New SSH key title", &keyTitle, nonEmpty); err != nil {
		return "", err
	}
	key, err := client.GenerateSSHKey(vault, keyTitle)
	if err != nil {
		return "", err
	}

	fmt.Println("\n" + ui.Title.Render("Public key — add this to the remote server's authorized_keys"))
	fmt.Println()
	fmt.Println("  " + key.PublicKey)
	fmt.Println()
	fmt.Println(ui.Dim.Render("One-liner (run on the remote, replacing $USER if needed):"))
	fmt.Printf(ui.Dim.Render("  mkdir -p ~/.ssh && echo %q >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys\n"), key.PublicKey)
	if _, err := ui.Confirm("Have you added the public key to the server?", "Yes, continue", "Cancel"); err != nil {
		return "", err
	}
	return key.Ref.String(), nil
}

func init() { serverCmd.AddCommand(serverAddCmd) }
