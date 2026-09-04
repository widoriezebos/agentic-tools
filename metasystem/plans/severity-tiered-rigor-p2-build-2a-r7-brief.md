Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Build slice 2a, the risk basis: round seven, the second closing review's two findings

The second closing review (job str-p2-build-2a-cc2, tree cee8c2ea)
returned two material findings; its return.json under that chain's
round 1 is the authority for their text and evidence. The design
answers them in revision 4.5 of
metasystem/plans/severity-tiered-rigor-p2-design.md; build that
revision on the tree you inherit. Both restore existing law; neither
is a law change. Every example here is illustrative: where an example
contradicts the tree's existing law, the law wins, the choice is
recorded under `decisions`, and the item is built.

1. STR2P2A-10: sweep recovery skips applied rows. In
   metasystem/internal/goal/approval.go the classification listing and
   the confirm skip a draft row whose goal already carries exactly the
   row's Risk record (same four scores, same basis); the listing counts
   it as applied; a row whose goal carries a different Risk record
   keeps today's SWEEP_UNKNOWN_GOAL refusal. Rewrite
   TestClassifySweepRecoverySkipsRowsAlreadyApplied to feed the draft
   that still carries the applied row (the shape it had before round
   six: the applied goal's row plus one remaining row) and assert one
   proposal, the applied goal's Risk record unchanged, and the
   remaining goal's record written on confirm.
2. STR2P2A-11: the register line only for a raise. In
   metasystem/cmd/metasystem/goalsync_mutations.go the counselor
   register append (the condition near line 1244) fires only when the
   goal package performed a raise, that is the goal was approved before
   the edit and the edit lifted the derivation; a queued or unapproved
   goal's risk edit, including a tierless goal's first four answers,
   appends nothing and demands no evidence. Add one test in
   goalsync_mutations_test.go beside the existing misclassification
   test: a queued goal given its first four answers leaves the register
   file absent or unchanged; the existing approved-goal test still
   reads the line it wrote.

Run by name: TestClassifySweepRecoverySkipsRowsAlreadyApplied,
TestClassifySweepInstallsTierLawForAnAlreadyTieredLedger,
TestSTR3MigrationBootstrap01ApprovedAndClaimedLegacyGoals, the
misclassification tests in cmd/metasystem, then
scripts/agents/go-gate.sh --fast. Do not stage or commit; the seat lands
the chain.

# Constraints

Wall-clock budget: 25 minutes; return by minute 20 whatever the state.
Return under the implementer schema with `decisions` listed.

# Gap Rule

Stop and report a gap only for a law-changing contract (a new authority,
refusal, landing bar, or fleet-read schema); a mechanical choice (a
field name, a message wording, a helper's placement) is made from what
the tree does nearest the seam, recorded under `decisions`, and built.
A choice recorded in the return is not silent.
