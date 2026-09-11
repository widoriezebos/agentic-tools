Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal human-carried-landing-carry, sixth round on chain hcl-build3-20260911)
Date: 2026-09-11

# Fold: the fifth read's two findings

Round 5 of this chain (reviewed tree 70c7e1ba03828f34f3ae3ae8dcbaad29d0c806b3)
was read on Opus (hcl-cc5-20260911): four findings, two material, both
low, the verdict "one fold: HCL-C-79, HCL-C-80". The host has run every
leg green on this tree, the crash-local leg included. The two decisions
below are binding and small; change nothing else. Rule of the round,
unchanged: no fixture claims what it does not drive; no waiting on the
sandbox.

# The two decisions

1. **HCL-C-79 (low, a real race), the lock file is deleted while held.**
   In metasystem/internal/counselor/register.go the shared append path
   unlocks, closes and only then deletes `<register>.lock`. A waiting
   writer that already opened the old lock file then takes the lock on an
   unlinked file while a third writer creates and locks a fresh one; two
   writers scan and append at once and the register's idempotency by id
   breaks. Fix it the standard way: after acquiring the flock, stat the
   opened file and stat the path, and if they are not the same inode
   (the path was replaced or removed) release and retry; on release,
   delete the path FIRST while still holding the lock, then unlock and
   close. The error path does the same. One test drives the race the
   critic ran: sixteen goroutines, a register seeded with a few thousand
   lines, mixed unique and repeated ids over a hundred iterations, and
   asserts no id appears twice. It runs as `TestHCL79RegisterLockSurvivesRelease`
   and is named in the group that owns internal/counselor.
2. **HCL-C-80 (low), the carried round trip of a trailing-space why.** A
   test takes `--why "paid the carried debt "` through `goal accept-risk`
   on chain `human-carried` (the command layer, not the counselor helper
   directly), then through the carried admission of the resulting
   accepted-risk line, and asserts the line is admitted. Name it
   `TestHCL80TrailingWhitespaceWhyIsAdmitted` in the group that owns it.

# Not actioned, recorded

HCL-C-81 (a failed unlock, close or delete after the line is written)
and HCL-C-82 (pin the injected author date in the crash-local leg): the
coordinator records them; no change.

# Proof before you return

`go build ./... && go vet ./...`; `go test ./internal/counselor
./internal/landing ./cmd/metasystem -run 'HCL79|HCL80|Register|AcceptRisk'
-race -count=3`; `metasystem test check --root .` (say if the sandbox
blocks the cache); `bash scripts/agents/go-gate.sh --fast` (same).

# Return

`diffBoundary` and `files` are repository-root paths; under `evidence`
the commands with their observed result; under `deviations` anything left
out with the reason. Return within 30 minutes.
