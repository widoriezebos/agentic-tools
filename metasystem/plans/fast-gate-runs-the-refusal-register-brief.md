Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal fast-gate-runs-the-refusal-register, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

The refusal register test (TestHCL03EveryCodeRowed in
metasystem/internal/refusal/register_test.go) walks the engine's source
for refusal-shaped tokens and fails when one has no row or exclusion in
metasystem/internal/refusal/register.go. It runs only in the full race
gate, never in the fast gate every landing runs, so three landings
today named a new token and left the full gate red for the next seat
that ran it: VERIFIED_CHANNEL_ANSWER (fixed at 6c79648a),
DEADLINE_EXPIRED (fixed at 6ab703f7) and, right now on main,
`ledger-unreadable`, the constant unreadableIdleBacklogDigest at line
291 of metasystem/internal/goal/turnverdict.go, a backlog-digest
sentinel the steward's alert episode compares against (lines 157 and
235 of metasystem/internal/steward/alert_episode.go); it refuses
nothing.

When you are done, the fast gate runs the refusal package's tests as one
of its stages, `ledger-unreadable` has its exclusion, and the test is
green on main.

# The design

1. In metasystem/scripts/agents/go-gate.sh, fast mode (the block that
   begins "Fast mode stops here", around line 453) gains one stage
   before the build: `go test -count=1 ./internal/refusal`, reported
   like the other static stages ("refusal register" as its name in the
   red block and the passed line). It costs under a second. The full
   gate already runs it inside the race run; do not run it twice there.
   Update the header comment's list of fast stages and the passed line
   text ("gofmt, vet, staticcheck, refusal register, build").
2. In register.go add the exclusion
   `{Pattern: "ledger-unreadable", Reason: "a turn-verdict digest sentinel that refuses nothing"}`
   beside the other sentinel-style exclusions (after the steward
   component observations). The collector matches hyphen codes exactly,
   so the pattern is the full token.
3. No test changes; the register test is the proof.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/scripts/agents/go-gate.sh
May touch: metasystem/internal/refusal/register.go
Must not touch: anything else.

# Constraints

- Bash 3.2 clean. Never weaken a test. At most 30 minutes of wall clock.
  Hazard DESIGN-BEARING: the landing gate's stages change; an
  independent critique follows.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/refusal` (expected: ok)
- `bash scripts/agents/go-gate.sh --fast` (expected: passes, and its passed line names the refusal register stage)
- `grep -n 'internal/refusal' ./scripts/agents/go-gate.sh` (expected: the one fast stage)
- `git diff --stat` (expected: the two files)

# Acceptance Criteria

1. `go-gate.sh --fast` runs the refusal package's tests before the
   build and fails the gate when they fail.
2. TestHCL03EveryCodeRowed passes on the candidate.
3. Nothing else changed.

# Gap Rule

stop and report a gap; never fill it silently.
