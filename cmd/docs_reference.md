# sshsyncer — Branch-Based SSH Deployment

sshsyncer deploys the last pushed commit of the current git branch to a
remote server via rsync over SSH. Server credentials and SSH keys live in
a dedicated 1Password vault that sshsyncer creates and owns; SSH auth is
pinned to one specific 1Password-held key per server.

## Core Concept

Deployment targets are keyed by git branch name. When you run
`sshsyncer push`, the tool:

1. Detects the current branch.
2. Looks up the target in .sshsyncer.json.
3. Runs `git fetch origin` and verifies HEAD == origin/<branch>.
4. Resolves host/user/port and the pinned SSH key from 1Password.
5. Exports origin/<branch> via `git archive` into a temp directory.
6. Runs rsync from that temp directory, telling SSH to use only the
   pinned key from the agent (no MaxAuthTries surprises).
7. Appends an entry to .sshsyncer-log on the remote server.
8. Cleans up the temp directory.

Local uncommitted changes are never deployed.

## Prerequisites

- macOS or Linux
- git
- GNU rsync (macOS openrsync is NOT compatible — `brew install rsync`)
- 1Password CLI `op`
- One of:
  - 1Password 8 desktop app with the SSH agent enabled (recommended), OR
  - A service account token (`OP_SERVICE_ACCOUNT_TOKEN`) plus a separate
    SSH agent at `SSH_AUTH_SOCK` for CI use cases.

## One-time setup (per machine)

    sshsyncer setup

Walks through environment checks, signs you into 1Password if needed,
and either creates a new "sshsyncer" vault or claims an existing one
(provided every item in it is sshsyncer-tagged).

## Server credentials

    sshsyncer server add        # Create a Server item (and an SSH Key)
    sshsyncer server list       # Show every sshsyncer-tagged Server item
    sshsyncer server test NAME  # Open an SSH connection, run `echo ok`
    sshsyncer server remove N   # Delete a Server item

Each Server item carries host/username/port plus an `ssh_key` field whose
value is an op:// reference to a 1Password SSH Key item. The 1Password
SSH agent serves that key; sshsyncer pins SSH auth to it so only one
biometric prompt happens per deploy.

## Project setup

    sshsyncer init

In the root of your git repo, this prompts for a server (from the vault)
and a remote path, then writes:

- .sshsyncer.json — branch → target mappings (commit this)
- .deployignore — rsync exclude patterns (commit this)

The .sshsyncer.json file stores only op:// references, not secrets, so
it is safe to commit.

## Deploy

    sshsyncer push --dry  # Preview file changes
    sshsyncer push        # Apply

## View history

    sshsyncer log         # Tail remote .sshsyncer-log for current branch

## Diagnose

    sshsyncer doctor      # Check env + auth mode + vault + project state

## File locations

    ~/.sshsyncer/config.json   Local config: which 1Password vault to use
    .sshsyncer.json            Per-project: branch → server + path
    .deployignore              Per-project: rsync excludes
    /<remote_path>/.sshsyncer-log  Remote: append-only deploy history

## Sharing with teammates

Share the sshsyncer 1Password vault with them in 1Password. They run
`sshsyncer setup` and pick the shared vault. sshsyncer verifies it's
owned (every item carries the `sshsyncer` tag) before claiming it.
No keys or credentials are emailed around.
