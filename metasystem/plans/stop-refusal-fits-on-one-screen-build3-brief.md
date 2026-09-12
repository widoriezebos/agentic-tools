Working Mode: build
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal stop-refusal-fits-on-one-screen, reopened)
Date: 2026-09-09

# Build brief: the allow path and the health line, carried into a fresh chain

Goal stop-refusal-fits-on-one-screen (tier 3, reopened and re-approved
today; record metasystem/plans/goals/stop-refusal-fits-on-one-screen.md).
Chain one-screen-build2b-20260909 built this slice's round one and its
critic found two material defects; the fold round was cancelled by the
orchestrator to correct its brief, and a cancelled round closes a chain
to follow-ups, so this fresh chain carries round one forward by patch.
The critic's dispositions are in the records directory under misc, in
stop-refusal-fits-on-one-screen-build2-critique-r1-dispositions.md
(new, not yet committed); the original second-slice brief is
stop-refusal-fits-on-one-screen-build2-brief.md in the plans directory
(also not yet committed).

## Step one: apply round one

Before anything else, from the metasystem directory of your worktree,
run `git apply records/misc/stop-refusal-fits-on-one-screen-build2-round1.patch`
(a tracked record: the certified diff of chain
one-screen-build2b-20260909 round one, eight files, one of them new,
made against main at b95f29afa; main has moved only in ledger records
since, none of the eight). Build, vet and run the focused tests; they
were green on that tree. Then fold the items below on top.

## OSR-11: the completion must carry the health line that was emitted

metasystem/scripts/agents/supervision-hook.sh completes the attempt with
`steward hook-complete ... --health-line "$health_line" --payload-file`,
and steward.CompleteHookAttempt in
metasystem/internal/steward/component_evidence.go refuses an OK
completion whose payload does not contain that line verbatim
(hookPayloadContainsHealthLine). Round one emits the bounded message,
so whenever the display plus the health line exceed 4,000 runes the
completion is refused, the evidence is never written, and hook-freshness
reads stale on a seat that is already unhealthy. Fix: the hook passes
the health line it actually emitted (the compact preview line; if the
bound trimmed even that, the bounded text that survived), and the
engine's check compares against what the hook passes. A test in
internal/steward proves a completion succeeds for an over-bound allow.

## OSR-12: a failed evidence write must not discard the verdict

metasystem/cmd/metasystem/steward_verbs.go: `health --hook-preview`
exits 2 with empty standard output when the sibling health file cannot
be written, so the hook substitutes "HEALTH unknown", records a stop
failure, and the first failure for that cause blocks an honest allow.
Fix: the preview prints its line regardless, reports the write failure
on standard error with a non-fatal code, and the hook carries that
failure as one diagnostic line, never as a substitute verdict. A test
makes the stop-verdicts directory read-only and asserts the line still
prints and the exit code is not 2.

## OSR-15: a missing bound verb degrades, never silences

In surface_json, `report bound-message` is called without checking its
status and an empty result makes the whole response empty. Fix: when
the verb fails or returns empty, fall back to the raw text and append
one diagnostic line saying the bound was unavailable.

## The fixture: the template-layout allow scenario dies silently

Seat-side, with the round-one engine, `bash
scripts/agents/supervision-hook-fixtures.sh` exits 1 with no output.
The orchestrator traced the hook itself inside your scenario: for the
seeded running job the hook's deadline child printed the internal-skip
sentinel (METASYSTEM_INTERNAL_HOOK_SKIP_V1) and the parent exited 0
with ZERO bytes on both streams, so `template_running_message=$("$ms"
json get ... --field systemMessage)` failed and set -e ended the
script with no message. The hook skipped on purpose: it judged that
Stop to be a delegate's own, because your seeded record
template-running.json carries `pid` equal to the fixture's own pid,
which is also what the shim's `proc find-ancestor` returns as the
caller. The cause is the record, not the hook. Fix: seed the running
job the way the passing scenario at the fixture's line 492 does,
`{"jobId":"template-running","status":"running"}` with no pid and no
mainId, which yields STILL WORKING; if you want the job owned by the
caller main, give it a live pid that is not the caller's (a `sleep`
child the scenario starts and kills at its end) and say why the
unwatched-work rule then allows rather than blocks. Then add the guard
this suite lacked: before anything else, the scenario asserts the hook
produced non-empty standard output and prints both streams when it did
not, so the suite can never die silently here again.

## OSR-13, while you are in the file

Update the comment and the argument name on PreviewHealthAt so nobody
believes a repository-scoped check still exists; no behaviour change.

## Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/steward/ -run 'Health|Hook|Preview|Complete' -count=1`
and `go test ./internal/report/ ./cmd/metasystem/ -run 'Bound|Report|Health' -count=1`
green; `bash -n` on both shell files; say plainly what else your
sandbox could run. The orchestrator replays the full packages, the
coverage floors and the hook suite.

## Constraints

Wall-clock budget: 45 minutes; return before it ends even if something
is red, naming it. MECHANICAL reach (tier 3). Declare the boundary as
every file that differs from main. Stop adding at a gap that needs a
decision no page has made; keep what is built and green; report the
gap with the resolution you propose. Never delete built work.
