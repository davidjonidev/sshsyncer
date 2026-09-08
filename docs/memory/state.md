---
project: davidjonidev/sshsyncer
stack: go
role: tool
status: |-
  **A showcase piece, not the studio's deploy tooling.** A Go CLI that deploys a git branch over rsync+SSH with every credential in a dedicated 1Password vault and auth pinned to one key per server. Public at `davidjonidev/sshsyncer`, written up at `davidjoni.dev/work/sshsyncer`. Moved into the fleet 2026-08-27 because it is a portfolio asset under active work and had neither an instruction file nor a brain. **The fleet's WordPress deploys do NOT run through it**; those go through CI, described in the `wp-base` case study.
next: |-
  **Bootstrap host keys into the 1Password vault.** Host-key verification now shares one accept-new policy across both transports, but recording on first use is trust-on-first-use, not verification. Carrying host keys in the vault alongside the SSH Key item makes even the first connection checkable against something already trusted. It needs a vault schema change, not a change to `internal/ssh/`.
blocked_on: |-
  Nothing. v1.0 gating on the Homebrew tap and GoReleaser artefacts is a release decision, not a blocker.
---

# Current state — sshsyncer

> A showcase piece is held to a higher bar than a private tool, not a lower one. The whole point is
> that someone is looking, so a known weakness left in place is the thing they will find.

## What is here

A Cobra CLI in Go. `setup` claims a 1Password vault, `server add` creates a Server item and generates
an SSH key, `init` writes the committed `.sshsyncer.json` and `.deployignore`, `push` deploys, `log`
tails an append-only deploy history kept on the server.

**The trust model is the interesting part.** No private key is ever held by sshsyncer. Credentials
live in a dedicated 1Password vault protected by an ownership guard; the committed config carries
only `op://` references; auth is pinned to one key so the agent offers exactly that key, which is
what makes it one biometric prompt per deploy rather than one per key offered.

## Recent work

**2026-08-27: host-key verification fixed.** Two transports had two policies. rsync used
`StrictHostKeyChecking=accept-new`; the Go path fell back to `InsecureIgnoreHostKey()` when
`known_hosts` was missing, which accepts every key **and records none**, so the connection never
became verifiable. Both now share one policy in `internal/ssh/hostkey.go`, and it fails closed when
the home directory cannot be resolved. Four tests, written first.

## Next steps

1. **Vault-held host keys** (see `next`). Turns trust-on-first-use into verification.
2. **v1.0 release gating.** GoReleaser targets and a Homebrew tap at `davidjonidev/homebrew-tap` are
   wired but held. Releasing acquires users, and users acquire support expectations.
3. **No CI.** `gofmt`, `go vet` and the suite are run by hand. A GitHub Actions workflow would make
   that a fact rather than a habit.

## Do NOT

- Do NOT reintroduce `InsecureIgnoreHostKey()`, or any path that skips host-key verification.
- Do NOT put a credential in a file. The vault holds them; the repo holds references.
- Do NOT let the public case study drift from this code. Change both in the same session.
- Do NOT lift source from any employer's internal tooling. This is an independent reimplementation.
