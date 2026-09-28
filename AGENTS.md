# sshsyncer agent instructions

**`docs/memory/` is frozen as of 2026-09-28**: read it for past reasoning, never append to it.

A Go CLI that deploys a git branch to a remote server over rsync+SSH, with every credential held in
a dedicated 1Password vault and SSH auth pinned to one key per server.

**Read `docs/memory/state.md` first.** Then this file.

## 🔴 What this repo is for, and it changes how you treat it

**This is a showcase piece, not the studio's deploy tooling.** It is an independent reimplementation
of an idea, built to be shown: it is a public repo and a case study at
`davidjoni.dev/work/sshsyncer`. The fleet's own WordPress deploys do **not** run through it; those go
through CI, and the `wp-base` case study describes that pipeline.

Two consequences:

- **The bar is demonstration quality, not "works on my machine".** A known weakness left in a
  showcase piece is worse than the same weakness in a private tool, because the whole point is that
  someone is looking. This is why the host-key inconsistency was fixed rather than documented.
- **It shares no source with any employer's internal tooling.** Keep it that way. The idea is
  reimplemented; the code is this repo's own.

## Hard rules

1. **Tests first, and the failing test must fail for the right reason.** Every package has
   `_test.go` files alongside the code. Anything that shells out (`op`, `git`, `rsync`, SSH) sits
   behind a small interface or takes an `io.Writer`, so tests fake it without subprocesses.
2. **Never weaken host-key verification.** Both transports share one accept-new policy: unseen hosts
   are recorded, recorded hosts must match, a changed key is always refused, and an unresolvable home
   directory fails closed. `InsecureIgnoreHostKey()` must not come back. See `internal/ssh/hostkey.go`
   and the four tests beside it.
3. **No credentials on disk.** Host, user, port and the SSH key itself live in the 1Password vault.
   `.sshsyncer.json` is committed and carries only `op://` references. If a change would put a secret
   in a file, it is the wrong change.
4. **The ownership guard stays.** sshsyncer tags every item it creates and refuses to operate on a
   vault holding items it did not create. A deploy tool that can reach the rest of a personal vault
   is a different and worse tool.
5. **Only what is pushed gets deployed.** The tool deploys the last commit on `origin/<branch>`.
   Local uncommitted work must never reach a server.
6. **`gofmt`, `go vet` and the full suite must be clean before a commit.** There is no CI here yet.

## Layout

```
cmd/                    Cobra handlers, thin
internal/config/        local + project config (.sshsyncer.json)
internal/op/            1Password CLI wrapper, vault CRUD, ownership guard, auth-mode detection
internal/ssh/           pinned-key auth, host-key verification, the Go SSH path
internal/rsync/         argv construction, dry-run parsing, execution
internal/git/           branch detection, fetch, push-check, archive
internal/deploylog/     append and tail the remote .sshsyncer-log
internal/env/           detects git, rsync, op, agent
internal/ui/            lipgloss styles, huh prompts
```

## Dev commands

| Command | Purpose |
|---|---|
| `go build ./...` | Build |
| `go test ./...` | Full suite |
| `go vet ./...` | Vet |
| `gofmt -l .` | Formatting; must print nothing |

## Do NOT

- Do NOT let the case study at `~/business/portfolio/content/projects/sshsyncer.mdx` drift from this
  code. It is public and it makes specific claims. **If you change behaviour it describes, change the
  case study in the same session.**
- Do NOT add a `hooks` key. User-scope hooks already fire for every repo under `~/business/`.
- Do NOT lift code from any employer's internal tooling.
