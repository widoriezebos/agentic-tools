Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal race-gate-red-on-main, tier 3, hazard DESIGN-BEARING, code critique round two of chain rgr-build1)
Date: 2026-09-06

# Review brief: race-gate-red-on-main, round two

Round budget: three focused rounds for the goal's tier-3 box; this is
round two, after one fold. The orchestrator adjudicates every finding;
you edit nothing.

Threat model: unchanged from round one
(metasystem/plans/race-gate-red-on-main-critique-r1-brief.md): one seat
and its delegates on one machine, no adversaries; in scope are wrong
behavior on the stop hook's verdict path, a surviving concurrency
defect, a weakened or bypassed test or gate, a register entry that hides
a real refusal, an unrelated change, and a gate ceiling that no longer
bounds a hang. Out of scope: hostile inputs, the suites' speed, and the
steward test.

Scope: the computed diff of chain rgr-build1 after follow-up round two
(job rgr-build1-r2) against base commit 414c9dc4. Round one implemented
metasystem/plans/race-gate-red-on-main-brief.md; round two folded the
three round-one findings per
metasystem/plans/race-gate-red-on-main-fold-r2-brief.md, comments only,
in metasystem/scripts/agents/go-gate.sh and
metasystem/scripts/agents/coverage-delta.sh. The round-one findings and
the orchestrator's dispositions are in
metasystem/artifacts/agents/rgr-critic1/rounds/1/return.json and
the dispositions record you receive with this brief. The computed diff
for the whole chain after round two is
metasystem/artifacts/agents/rgr-build1/rounds/2/diff.patch and its
reviewed tree is db53bb3dc47593c219a15743bde138670e046bfe; carry that
hash into your return exactly.

# Goal

Say whether the chain now ships a defect, violates its briefs, or
damages what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

1. The fold: F-1 corrected to a 19-second (or under-twenty-second) per-
   test bound; F-2's ceiling sentence states only what the file can
   stand behind, no machine names, dates, goal names or round
   references; F-3's coverage-delta.sh comment states its own reason
   with the timeout value unchanged. Confirm no other line of either
   script moved beyond those comments.
2. Round-one work must be untouched by the fold: the Go hunks in
   metasystem/internal/goal/project.go,
   metasystem/internal/goal/turnverdict.go,
   metasystem/internal/goal/sessionstop.go and the exclusion in
   metasystem/internal/refusal/register.go are byte-identical to the
   round-one diff.
3. Anything round one missed. The Store copy pattern reassigns the
   receiver parameter to a copy's address: check closures and deferred
   calls inside the three verbs still see the copy, not the original.
4. Conformance: every changed path is inside the union of the two
   briefs' May-touch lists; no test file changed; nothing under plans.

# Evidence you may run

If your runtime gives you a shell, from the reviewed worktree root (the
metasystem directory):

- `bash -n ./scripts/agents/go-gate.sh` and `bash -n ./scripts/agents/coverage-delta.sh`
- `grep -n 'timeout' ./scripts/agents/go-gate.sh ./scripts/agents/coverage-delta.sh`
- `go test -race -count=1 -run 'TestFreshLedgerFailureAndFetchTimeoutBlockTheStop|TestWatchdogProtocol' ./internal/goal`
- `go test -count=1 ./internal/refusal`

If it does not, say so in gaps as round one did; the orchestrator has
run the focused commands on the reviewed worktree and runs the full
gate before landing.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
