# Decisions — davidjonidev/sshsyncer

> **Frozen 2026-09-28.** Historical record, kept for past reasoning. Do not append.

Append-only. Authoritative for *why* anything in this repo is the way it is. Record corrections **as
corrections**; never edit a wrong entry into a right one.

Studio-level and fleet-wide decisions live in `~/business/ops/docs/memory/decisions.md`.

## 2026-08-27 — Joined the fleet; host-key verification fixed rather than documented

Moved from `~/dev/sshsyncer` into `~/business/`. It is a portfolio asset under active work and had
neither an instruction file nor a brain, which is precisely the gap the per-repo contract exists to
close. The remote is unchanged at `davidjonidev/sshsyncer`, which is public.

### The fix, and why "documented limitation" was the wrong framing

The case study described `InsecureIgnoreHostKey()` as *"a documented limitation, not a design
choice: first-time machines lack the data needed to verify the host."* **That framing was wrong, and
it was wrong in a way that made the bug look acceptable.**

The real shape was an inconsistency. rsync already ran with
`StrictHostKeyChecking=accept-new`, which trusts a host on first sight, **records its key**, and
refuses a later change. The Go path did not record anything, so the connection never became
verifiable: not on the tenth attempt, not on the hundredth. *A first connection that cannot be
verified is a reasonable trade. A connection that is never verifiable is a different claim.*

Two transports with two policies is the kind of defect that survives review because **each half looks
fine on its own**. Nobody reads `builder.go` and `pinkey.go` in the same sitting.

Both now share one policy: unseen host recorded and accepted, recorded host must match, changed key
always refused, unresolvable home directory fails closed. Four tests written first; the load-bearing
one asserts the refusal, and the previous implementation would have failed it because it returned
`nil` unconditionally.

### The rule this establishes

**A showcase piece is held to a higher bar than a private tool, not a lower one.** The purpose of
this repo is that someone technical looks at it. A known weakness left in place is therefore not a
tolerable backlog item; it is the thing they will find, in the exact area the case study claims
competence in. Fix it, then write about having fixed it, which is a better story than the caveat was.
