Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-14

# Goal

Round 5 of chain gawr-build3, the landing side. Fix the three material
findings of its rostered read (job gawr-build3-read1). Specification:
metasystem/plans/goal-abandoned-with-a-reason-design.md, revision 7. Build
on the worktree as round 4 left it; do not commit.

# The findings, as the reader wrote them

## GAWR-C3-01 (high)

The push-time re-check `landing held` stops judging a push range when it meets a commit that names no goal above a Goal-free ledger. It returns the goal-free verdict with exit 0 and never looks at the commits above that one. A higher commit in the same push that is bound to an abandoned goal, or that carries two Machine trailers (a refusal the page says is never softened), is then pushed. This breaks section 4a's rule that every commit a push introduces is judged at its own parent, and so lets a straggler land above an abandonment.

Cited: internal/landing/held.go (the file this chain adds):163-166 sets Outcome goal-free and returns from inside the per-commit loop instead of going on to the next commit. A probe test, run through go test -overlay on the reviewed landing files, built this range: a Goal-free base with ship-widget archived; L1 with only `Machine: m9+L1`; L2 above it with `Goal-Item: ship-widget` and `Goal-Revision: 3`. Held(base, L2) returned goal-free, 2 commits, exit 0. Held(L1, L2) refused goal-item-not-held, exit 1. A third commit with two Machine lines also passed. A reachable sequence: a goal-free landing commit A is left unpushed (its push was rejected or its proof step failed). Goal G is opened and claimed, and chain work

## GAWR-C3-02 (high)

The static re-proof fixture bed drives the real commit wrapper. It fails on the reviewed tree and passes on trunk, because this slice changed the wrapper contract the bed pins and the bed was not updated. It breaks three ways. (1) Its agent commits run under `env -i` with claim epoch 1 and no owner lineage, so the new owner-lineage refusal stops the first commit. (2) With a lineage supplied, rule (b)'s goal-binding-missing, which the version-2 policy record now promotes to a refusal, fires before the unclassified-path refusal the bed expects. (3) The bed still asserts the promotion-record repair sentence that revision 7 replaced. The testing contract lists `section/static-reproof-fixtures` in the gate-plumbing and residual deep plans and in the cadence list. So the full-width landing's battery fails on it, or, if the battery skips it, the bed is red on trunk after landing.

Cited: Reviewed-tree export: `bash scripts/agents/static-reproof-fixtures.sh --real-observer-only` exited 2 with 'agent commit refused: the lease holder has a claim epoch but no owner lineage'. The call is metasystem/scripts/agents/static-reproof-fixtures.sh:90-91 and the refusal is metasystem/scripts/agents/commit.sh:55-57. The same command on a trunk 770f9693 export exited 0 with 'TestRealCommitWrapperStampsParseableObservation: PASSED'. A temporary copy with METASYSTEM_OWNER_LINEAGE=human added then failed at static-reproof-fixtures.sh:144 ('unclassified refusal lost its base-manifest detail'), because the wrapper refused 'this landing names no goal and the ledger is not Goal-free (would-refuse 

## GAWR-C3-03 (low)

The Go tests for held still take the goal out of the claimed state by writing a done record, not an abandoned one, although slices 1 and 2 are on trunk. The page's fixtures TestHeldRecheckReadsTheParent and TestHeldChecksEveryCommitIntroducedByPush need a parent where the goal is archived as abandoned, and a refusal that names 'abandoned'. The chain brief allowed the done substitution only until the ledger slices landed. The review brief says the switch now calls goal abandon, but that is true only of the shell bed. The helper's own comment ('When the ledger slice lands, this body changes') is now false.

Cited: internal/landing/held_test.go (the file this chain adds):257-266 renders `State: goal.StateDone`, and the assertions at :30 and :184 expect 'goal ship-widget is done'. goal.StateAbandoned exists on trunk (metasystem/internal/goal/file.go:464) and is already used in metasystem/internal/landing/hcl_carried_test.go:142. The shell helper in metasystem/scripts/agents/land-fixtures.sh was switched to `goal abandon` and asserts 'is abandoned at'. So the behaviour is proven end to end, but the Go fixtures the page names do not match what it describes.

# What the round must satisfy

- GAWR-C3-01: the push-time re-check judges EVERY commit the push
  introduces, at its own parent, as section 4a says. A goal-free commit
  above a Goal-free ledger is not a reason to stop looking at the commits
  above it. Test a push whose lower commit is goal-free and whose higher
  commit is bound to an abandoned goal, and one whose higher commit
  carries two Machine trailers.
- GAWR-C3-02: bring the static re-proof bed back to green on this tree
  without weakening what it proves. It drives the real commit wrapper, so
  it must supply an owner lineage, meet the version-2 policy record's
  refusals in the order the page states, and assert the repair sentence
  revision 7 actually writes. Run
  `metasystem/scripts/agents/static-reproof-fixtures.sh` if your sandbox
  allows it; the seat runs it outside the sandbox otherwise.
- GAWR-C3-03: the held Go fixtures take the goal out of the claimed state
  by abandoning it, not by writing a done record, now that slices 1 and 2
  are on trunk; the refusal names 'abandoned'. Correct the helper's stale
  comment.
- `go test ./internal/landing/ -count=1` and
  `go test ./cmd/metasystem/ -run 'Land|Landing|Carry|Held' -count=1` pass,
  the land bed's 27 scenarios stay green, and the fast gate passes.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json)
with `{command, observed, level}` evidence replayable from the worktree's
repository root, one per command above, plus `git -C metasystem status --short`.
whatWasDone names each finding and the change that answers it. Every path
in your return starts with `metasystem/`.

# Constraints

Wall clock: 75 minutes. A partial round returns with its tests green for
what exists and names what is left.

# Gap Rule

stop and report a gap; never fill it silently.
