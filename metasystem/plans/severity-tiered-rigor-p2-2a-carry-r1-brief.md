Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2-2a-carry)
Date: 2026-09-04

# Carry slice 2a, the risk basis: round one, the round-six tree plus the second review's two findings

This chain carries the finished risk basis to main; its parent goal's
budget is spent and Wido granted the carry. Step one, before anything
else: from the repository toplevel (the parent of `metasystem/`) apply
the finished tree verbatim,

    git apply --directory=metasystem metasystem/artifacts/agents/str-p2-build-2a/rounds/6/diff.patch

(37 files; it applies cleanly on main at 17264410). That diff is six
rounds of built, twice-reviewed work: do not rewrite, reformat, or
"improve" any of it beyond the two items below. Then build, on that
tree, the two items of
metasystem/plans/severity-tiered-rigor-p2-build-2a-r7-brief.md exactly
as written there (design revision 4.5,
metasystem/plans/severity-tiered-rigor-p2-design.md): item 1, sweep
recovery skips a draft row whose goal already carries exactly that
row's Risk record, with TestClassifySweepRecoverySkipsRowsAlreadyApplied
feeding the draft that still carries the applied row; item 2, the
counselor register line of kind misclassification is appended by
metasystem/cmd/metasystem/goalsync_mutations.go only when the goal
package performed a raise (approved before the edit, derivation
lifted), with one new test for a queued goal's first four answers
writing nothing. Both restore existing law; neither is a law change.
Every example is illustrative: where an example contradicts the tree's
existing law, the law wins, the choice is recorded under `decisions`,
and the item is built.

Run by name: TestClassifySweepRecoverySkipsRowsAlreadyApplied,
TestClassifySweepInstallsTierLawForAnAlreadyTieredLedger,
TestSTR3MigrationBootstrap01ApprovedAndClaimedLegacyGoals,
TestSTR4R1SweepBackfill, the misclassification tests in
cmd/metasystem (`-run Misclassif`), then scripts/agents/go-gate.sh
--fast. Known on this host, not yours to fix: the complete
internal/dispatch and internal/steward packages fail temporary-repository
setup ("could not parse HEAD", bootstrap ledgers); the `dispatch`
scenario of dispatch-fixtures.sh is red on main. Do not stage or
commit; the seat lands the chain.

# Constraints

Wall-clock budget: 35 minutes; return by minute 30 whatever the state.
Return under the implementer schema with `decisions` listed and the
applied diff named under evidence.

# Gap Rule

Stop and report a gap only for a law-changing contract (a new authority,
refusal, landing bar, or fleet-read schema); a mechanical choice (a
field name, a message wording, a helper's placement) is made from what
the tree does nearest the seam, recorded under `decisions`, and built.
A choice recorded in the return is not silent.
