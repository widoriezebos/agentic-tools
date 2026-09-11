Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal human-carried-landing-carry, fourth round on chain hcl-build3-20260911)
Date: 2026-09-11

# Fold: the fourth read's four findings, and the host's run of round 3

Round 3 of this chain (reviewed tree 17a09a326a80fba66af49ef4bebb52ec0b075658)
was read on Opus (hcl-cc4-20260911): five findings, four material, the
verdict "one fold: HCL-C-74, -75, -76, -77". The coordinator ran every bed
and the affected packages on the host: every Go package green, the
goal-cli bed green, 22 of 25 land legs green; the three red legs are the
three your sandbox cannot run, and two of them fail exactly as the read
predicted. The decisions below are binding and small; change nothing
else. Rule of the round, unchanged: no fixture claims what it does not
drive; no waiting on the sandbox. Context files under artifacts/ are not
in your worktree; every decision you need is here.

# The five decisions

1. **HCL-C-74 (high), the crash-local row patterns.** In the
   `carried-crash-local` leg the two closing counts grep for lines
   starting with `- History: `; no goal file has such lines (RenderFile
   writes a `History:` header, then `- <time> <opid> <verb> actor=...`
   rows). The host run fails there: "goal does not hold exactly one
   carrying row" on a goal that holds exactly one. Count the rows in the
   rendered form (for example `^- [^ ]+ [^ ]+ carrying .*approvedRef=<word>
   .*reason=open ` and the `carried ... reason=landed` twin).
2. **HCL-C-75 (medium), a kept commit is not a restamp.** The leg must
   fail on a recovery that resets and commits again. Before the rerun,
   the leg moves origin's code (the peer pushes an unrelated code commit
   on main, the way the existing `ledger-move-lands` and forward legs
   move origin), so the `local:` branch really rebases. Then it asserts:
   the pushed commit's parent is the moved origin tip; its author date
   equals the crashed local commit's author date (a rebase keeps it, a
   restamp does not); its `Carry:` trailer and `Landing-Provenance` line
   equal the crashed commit's; and the crashed commit's own sha is no
   longer on any branch. Keep the existing five checks.
3. **HCL-C-76 (high), the counselor lock files.** `appendRegisterLine`
   creates `<register>.lock` beside each register and never removes it,
   so after the first carried landing and `goal accept-risk` two untracked
   lock files sit in the checkout and the next landing refuses
   "untracked paths remain after staging" (the host's `carried-second`
   failure, and the same on any real seat: this checkout ignores them
   only through its private `.git/info/exclude`). Decision: the writer
   removes its lock file when it releases the lock (the lock is transient;
   use the same acquire and release path for both registers), AND
   `records/counselor/*.lock` is added to the installation's `.gitignore`
   as the safety net for a crashed writer. `carried-second` then stages
   only payload and registers, and lands.
4. **HCL-C-77 (low), whitespace in the why.** `goal accept-risk` trims
   leading and trailing whitespace from `--why` before writing either the
   ledger row or the register line, and refuses a why that is blank after
   trimming; a unit test round-trips a why with a trailing space through
   the row, the line and the carried admission.
5. **The host's `carried-prefixed` failure (fixture construction).** The
   leg now fails at the supporting green battery with "delivery impact is
   unresolved: no surface owns changed path metasystem/payload.txt". Under
   a prefixed installation the testing contract's surface paths and group
   inputs are repository-relative, as this repository's own testing.json
   writes them (`metasystem/...`). Write the prefixed fixture's contract
   with the prefix on every path, then make the leg prove what its name
   says: the same fresh carried landing as `carried-fresh`, installed under
   `metasystem/`, lands with its seven trailers.

# Not actioned, recorded

HCL-C-78 (a hand-made duplicate accepted-risk line): recorded by the
coordinator, no change.

# Proof before you return

`go build ./... && go vet ./...`; `go test ./internal/counselor
./internal/landing ./internal/goal -run 'HCL|Counselor|AcceptRisk|Register'`
(sandbox-bound tests named, not waited for); `bash scripts/agents/go-build.sh`
then `metasystem test check --root .`; `bash scripts/agents/go-gate.sh --fast`;
`bash -n` on every changed script. The three legs need the host; say so
and return.

# Return

`diffBoundary` and `files` are repository-root paths. Under `evidence`
every command with its observed result; under `deviations` anything left
out with the reason. Return BEFORE the cap: this round is five small items;
at 45 minutes run the proof list and return.
