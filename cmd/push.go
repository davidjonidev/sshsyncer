package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/davidjonidev/sshsyncer/internal/config"
	"github.com/davidjonidev/sshsyncer/internal/deploylog"
	gitPkg "github.com/davidjonidev/sshsyncer/internal/git"
	"github.com/davidjonidev/sshsyncer/internal/op"
	rsyncPkg "github.com/davidjonidev/sshsyncer/internal/rsync"
	sshpkg "github.com/davidjonidev/sshsyncer/internal/ssh"
	"github.com/davidjonidev/sshsyncer/internal/ui"
)

var pushDry bool

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Deploy the current branch to its target",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := config.RequireSetup(); err != nil {
			return err
		}
		cwd, _ := os.Getwd()
		projPath, err := config.FindProject(cwd)
		if err != nil {
			return err
		}
		repo := filepath.Dir(projPath)

		p, err := config.LoadProject(projPath)
		if err != nil {
			return err
		}
		branch, err := gitPkg.CurrentBranch(repo)
		if err != nil {
			return err
		}
		t, ok := p.Targets[branch]
		if !ok {
			return fmt.Errorf("no target for branch %q. Run: sshsyncer target edit", branch)
		}
		ref, err := op.ParseReference(t.Server)
		if err != nil {
			return err
		}

		fmt.Println(ui.Title.Render("sshsyncer push"))
		fmt.Printf("  branch:  %s\n", branch)
		fmt.Printf("  server:  %s\n", ref.Item)
		fmt.Printf("  remote:  %s\n", t.RemotePath)
		fmt.Println()

		fmt.Print("Fetching origin… ")
		if err := gitPkg.Fetch(repo, "origin"); err != nil {
			return err
		}
		fmt.Println(ui.OK.String())

		fmt.Print("Checking branch is pushed… ")
		pushed, err := gitPkg.HEADPushed(repo, branch)
		if err != nil {
			return err
		}
		if !pushed {
			return fmt.Errorf("HEAD is not pushed to origin/%s — push first", branch)
		}
		fmt.Println(ui.OK.String())

		fmt.Print("Resolving credentials from 1Password… ")
		client := op.NewClient()
		srv, err := client.GetServer(ref)
		if err != nil {
			return err
		}
		keyRef, err := op.ParseReference(srv.SSHKeyRef)
		if err != nil {
			return err
		}
		pub, err := client.ReadPublicKey(keyRef)
		if err != nil {
			return err
		}
		fmt.Println(ui.OK.String())

		pubPath, cleanupPub, err := sshpkg.PinPublicKey(pub)
		if err != nil {
			return err
		}
		defer cleanupPub()

		fmt.Print("Exporting origin/" + branch + " via git archive… ")
		tmp, err := os.MkdirTemp("", "sshsyncer-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		if err := gitPkg.Archive(repo, "origin/"+branch, tmp); err != nil {
			return err
		}
		fmt.Println(ui.OK.String())

		deployIgnore := filepath.Join(repo, ".deployignore")
		if _, err := os.Stat(deployIgnore); os.IsNotExist(err) {
			deployIgnore = ""
		}

		spec := rsyncPkg.Spec{
			SourceDir:    tmp,
			User:         srv.User,
			Host:         srv.Host,
			Port:         srv.Port,
			RemotePath:   t.RemotePath,
			DeployIgnore: deployIgnore,
			Excludes:     t.Excludes,
			IdentityFile: pubPath,
			Delete:       true,
			DryRun:       pushDry,
		}
		rsyncArgs := rsyncPkg.BuildArgs(spec)
		fmt.Println("Running rsync…")
		out, err := rsyncPkg.Run(rsyncArgs, os.Stdout)
		if err != nil {
			return err
		}

		if pushDry {
			plan := rsyncPkg.ParsePlan(out)
			fmt.Printf("\n(dry run) %d new, %d modified, %d deleted\n",
				len(plan.Added), len(plan.Modified), len(plan.Deleted))
			return nil
		}

		fmt.Print("Connecting to log destination… ")
		sshClient, err := sshpkg.Dial(sshpkg.Conn{
			User: srv.User, Host: srv.Host, Port: srv.Port,
			PublicKey: pub,
		})
		if err != nil {
			return err
		}
		defer sshClient.Close()
		fmt.Println(ui.OK.String())

		head, _ := gitPkg.HeadShort(repo)
		whoami := os.Getenv("USER")
		err = deploylog.Append(sshClient, t.RemotePath, deploylog.Entry{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			User:      whoami,
			Branch:    branch,
			Commit:    head,
			FileCount: countTransferred(out),
		})
		if err != nil {
			return err
		}
		fmt.Println("Done.")
		return nil
	},
}

func countTransferred(out string) int {
	plan := rsyncPkg.ParsePlan(out)
	return len(plan.Added) + len(plan.Modified)
}

func init() {
	pushCmd.Flags().BoolVar(&pushDry, "dry", false, "Preview without applying")
	rootCmd.AddCommand(pushCmd)
}
