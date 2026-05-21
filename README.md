# sshsyncer

CLI for deploying a git branch to a remote server via rsync over SSH, with
credentials stored in a dedicated 1Password vault and SSH auth pinned to a
specific 1Password-held key per server.

## Why

Most "deploy via SSH" scripts have at least one of these problems:

- Keys live on every developer's laptop, copied around manually.
- Credentials live in a shared file (or worse, environment variables).
- SSH auth offers every key in the agent until one works — slow and prompts
  for biometric approval on each attempt.

sshsyncer puts every credential — host, user, port, and the SSH key itself —
inside a single 1Password vault. The vault is shared via 1Password's normal
sharing model. Auth is pinned to one specific key per server, so the
1Password SSH agent prompts you exactly once per deploy.

Only the last commit pushed to `origin/<branch>` is ever deployed. Local
uncommitted changes never reach a server.

## Requirements

- macOS or Linux
- git
- **GNU rsync** (macOS openrsync is not compatible — `brew install rsync`)
- 1Password CLI (`op`)
- One of:
  - 1Password 8 desktop app with the SSH agent enabled (recommended)
  - A service account token (`OP_SERVICE_ACCOUNT_TOKEN`) plus a separate
    SSH agent at `SSH_AUTH_SOCK` for CI use

## Install

For now, build from source:

```bash
git clone https://github.com/davidjonidev/sshsyncer.git
cd sshsyncer
go install .
```

A Homebrew tap and pre-built binaries will be added once the project hits
v1.0.

## Quick start

```bash
sshsyncer setup              # one-time per machine: creates the sshsyncer vault
sshsyncer server add         # creates a Server item + generates an SSH key
sshsyncer init               # in your project repo: pick server + remote path
git add .sshsyncer.json .deployignore && git commit -m "Add sshsyncer config"
sshsyncer push --dry         # preview
sshsyncer push               # deploy
sshsyncer log                # view deploy history
```

## Command reference

| Command | What it does |
| --- | --- |
| `sshsyncer setup` | One-time per-machine setup (creates or claims the sshsyncer vault) |
| `sshsyncer doctor` | Diagnose environment + 1Password + project state |
| `sshsyncer vault` | Show current vault + ownership status |
| `sshsyncer server add` | Create a Server item (and an SSH Key) in the vault |
| `sshsyncer server list` | List Server items in the vault |
| `sshsyncer server test NAME` | Open an SSH connection to verify auth works |
| `sshsyncer server remove NAME` | Delete a Server item |
| `sshsyncer init` | Initialise the current project (writes `.sshsyncer.json` + `.deployignore`) |
| `sshsyncer target` | Show the current branch's target |
| `sshsyncer target edit` | Create or update the current branch's target |
| `sshsyncer target list` | List every branch → target mapping |
| `sshsyncer target remove` | Remove the current branch's target |
| `sshsyncer push` | Deploy the current branch |
| `sshsyncer push --dry` | Preview a deploy without applying |
| `sshsyncer log` | View deploy history for the current branch |
| `sshsyncer docs` | Print the full reference |

## Config files

| File | Location | Committed | Purpose |
| --- | --- | --- | --- |
| `config.json` | `~/.sshsyncer/` | no | Local: which 1Password vault to use |
| `.sshsyncer.json` | project root | **yes** | Branch → target mappings (contains only `op://` references, no secrets) |
| `.deployignore` | project root | **yes** | rsync exclude patterns |
| `.sshsyncer-log` | remote `<remote_path>` | n/a | Append-only deploy history on the server |

## Sharing with a team

1. Owner shares the sshsyncer vault in 1Password.
2. Teammate runs `sshsyncer setup` and picks the shared vault name.
3. sshsyncer verifies the vault is owned (every item is `sshsyncer`-tagged)
   before saving the choice.

No SSH keys to email around, no `credentials.json` to sync.

## Architecture

See [`docs/architecture.md`](docs/architecture.md) for the trust model,
package layout, and design decisions.

## License

MIT — see [LICENSE](LICENSE).
