Working Mode: build
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-open-work-refusal-repeats)
Date: 2026-09-07

# Fold round one: chain show-build1d-20260907

The code critic (job show-cc1-20260907, reviewed tree
d512426b64fe1103aff6c45bf38b497efa6c53bb) returned five material
findings on your round-one diff and two notes; its return is at
metasystem/artifacts/agents/show-cc1-20260907/rounds/1/return.json
(read it in full; the critic ran the reviewed binary on scratch
checkouts and its evidence names the exact commands). The contract is
unchanged: metasystem/plans/stop-hook-open-work-refusal-repeats-build-brief.md
and the goal record
metasystem/plans/goals/stop-hook-open-work-refusal-repeats.md. This is
a follow-up on your own job: start from your round-one tree.

# The change

1. SHO-01 (high). The verdict never applies the open-chain rule: the
   new open-chain test is reachable only from the report open-work
   verb, while the verdict's scanner (scanPlans and busyJobs in
   metasystem/internal/report/scan.go) still builds its busy list from
   pending or running job records alone. Make the VERDICT count an open
   chain (a root with chainClosed false whose newest round is not
   terminal) as work in flight; prove it with a test on the verdict
   (report turn-verdict), not the report verb.
2. SHO-02 (high). An unreadable or malformed seen-state must fail OPEN
   to today's behaviour with one visible line, never exit 1: today bad
   JSON, a foreign schemaVersion, a nil plans map or a digest mismatch
   make runReportTurnVerdict exit 1 with no verdict, and the hook turns
   that into a blocking "turn-verdict unavailable" refusal on every
   turn end until a human deletes the file. Treat every read failure as
   "no seen-state" (so the line refuses once, as on a fresh checkout),
   print one line naming the file and the reason, and rewrite the file
   fresh on the next write. Test each failure shape.
3. SHO-03 (high). The deadline path must not WRITE seen-state: when the
   deadline parent calls report stop-block --open-work-root, every open
   line is marked seen though the published refusal carries only the
   fixed deadline sentence, so a line that first appears during an
   expiry never blocks. The worker already writes the marker durably
   before deciding; the deadline path only reads. Remove the write from
   the deadline path and fix the fixture leg that currently pins the
   defect (the leg deleting the seen-state, running the deadline
   stop-block and asserting the next verdict does not block): the right
   assertion is that after an expiry the line still blocks once.
4. SHO-04 (medium). The digest covers only the first 200 bytes of the
   rendered line (scanPlans clips Detail to 200 bytes before the key is
   computed), so an edit past the clip leaves the old entry matching.
   Key the seen-state on the digest of the FULL plan line (the whole
   Next step text of the plan) with the plan path, computed before any
   clipping; the displayed line may stay clipped.
5. SHO-05 (medium). The seen-state grows without bound. On each write,
   drop entries for plans no longer in the current scan and entries
   whose plan line no longer matches the current digest; test that a
   plan's edit leaves exactly one entry and a removed plan leaves none.
6. Notes SHO-06 (mark-before-publish race in the deadline window) and
   SHO-07 (terminal status vocabulary duplicated from the dispatch
   package): fold SHO-07 by reusing the dispatch package's exported
   vocabulary if it is exported, otherwise leave a one-line comment
   naming the shared source; SHO-06 stays a note unless item 3 makes
   it trivial.

Do not widen: no new verbs or flags beyond what round one added; the
file set stays the round-one set.

# Gate

As in the build brief: build, vet, gofmt clean;
`go test ./internal/report/ ./cmd/metasystem/ -count=1`;
`bash scripts/agents/supervision-hook-fixtures.sh` (the seat replays
it outside the sandbox).

# Constraints

Wall-clock budget: 50 minutes; return before it ends even if something
is red, naming it. MECHANICAL reach (tier 3). Declare the boundary as
every file that differs from main. Gap rule: stop and report a gap
with your proposed contract written out.
