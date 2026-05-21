# Architecture

## Trust model

sshsyncer never holds private SSH keys on its own. Three pieces of state matter:

1. **1Password vault (dedicated)** — host, user, port, and SSH Key items.
   sshsyncer creates this vault at `setup` time and tags every item it
   creates with `sshsyncer`. It refuses to operate on a vault containing
   items it did not create (the **ownership guard**).
2. **SSH agent** — at deploy time, sshsyncer talks to whatever agent
   `SSH_AUTH_SOCK` points at. Normally that's the 1Password SSH agent,
   which signs auth challenges using keys still held in 1Password.
3. **`.sshsyncer.json` in the repo** — committed, contains only `op://`
   references. Resolving them requires the user to be signed in to
   1Password with access to the vault.

There are no on-disk credentials managed by sshsyncer.

## Pinned-key auth

Without pinning, an SSH client offers every key the agent serves until one
matches. That has two failure modes:

- 1Password prompts (biometric) for every key offer until one succeeds.
- SSH's default `MaxAuthTries=6` truncates the attempt list — auth fails
  even when the right key is in 1Password.

sshsyncer pins auth by reading the public key from the SSH Key item the
Server item points at, then:

- For rsync: passes `-o IdentitiesOnly=yes -i /tmp/sshsyncer-pub-XXX.pub`
  in the ssh command. SSH only offers that one key from the agent.
- For the Go SSH path (`server test`, `log`): filters
  `agent.Signers()` to the one whose public key matches.

Result: exactly one biometric prompt per deploy, regardless of how many
keys the user has in 1Password.

## Auth modes

`internal/op/auth_mode.go` detects two modes:

| Mode | Trigger | SSH agent | Read credentials | Create credentials |
| --- | --- | --- | --- | --- |
| **Desktop app** | 1Password 8 installed, no service-account env | 1Password agent | ✓ | ✓ |
| **Service account** | `OP_SERVICE_ACCOUNT_TOKEN` set | Must come from elsewhere | ✓ | ✓ (in granted vaults) |

CI typically uses a service account to resolve credentials, and a runner-
provided SSH agent (e.g. GitHub Actions' `webfactory/ssh-agent`) at
`SSH_AUTH_SOCK`. sshsyncer doesn't care which agent it talks to as long as
the pinned public key is loaded.

## Host-key handling

- rsync uses `-o StrictHostKeyChecking=accept-new`, which writes new host
  keys to `~/.ssh/known_hosts` on first connection and refuses changes.
- The Go SSH path (`server test`, `log`) uses
  `knownhosts.New(~/.ssh/known_hosts)` when the file exists, falling back
  to `InsecureIgnoreHostKey()` when it doesn't (first-time machines).
  Documented limitation.

## Package layout

```
cmd/                         # Cobra command handlers (thin)
  root.go                    # root cmd + version
  setup.go                   # one-time setup + ownership guard
  doctor.go                  # diagnostics
  vault.go                   # show current vault
  server.go + server_*.go    # Server item CRUD
  init.go                    # project init
  target.go + target_*.go    # branch → target CRUD
  push.go                    # deploy pipeline
  log.go                     # remote log tail
  docs.go                    # embed cmd/docs_reference.md

internal/
  config/                    # local + project config (.sshsyncer.json)
  op/                        # `op` CLI wrapper
    client.go                # base client
    reference.go             # parse op://Vault/Item[/Field]
    vault.go                 # vault create/list + ownership guard
    server_item.go           # Server CRUD
    ssh_key_item.go          # SSH Key generate/list/read
    auth_mode.go             # desktop vs service-account detection
    signin.go                # signin status
  git/                       # branch detection, fetch, push-check, archive
  rsync/                     # build argv, parse dry-run, run
  ssh/                       # net/ssh client with key pinning
  deploylog/                 # append/tail remote .sshsyncer-log
  deployignore/              # embedded default .deployignore
  env/                       # detect git, rsync, op, agent
  ui/                        # lipgloss styles + huh prompt wrappers
```

Files use one responsibility each. Anything that shells out (`op`, `git`,
`rsync`, SSH) is gated behind a small interface (or accepts an `io.Writer`
in the case of rsync) so tests can fake it without subprocesses.

## Testing

- All packages have `_test.go` files alongside the code.
- `op` tests use a `fakeRunner` interface — no real `op` CLI required.
- `git` tests create a real temp repo with `git init` — requires git on
  PATH (a build requirement anyway).
- `ssh` tests are limited to pure logic (`Endpoint`, missing agent, temp
  file writing). End-to-end SSH is in the Task 24 manual smoke checklist.
