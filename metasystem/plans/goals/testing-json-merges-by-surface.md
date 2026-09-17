# testing-json-merges-by-surface

- State: approved
- Priority: 1
- Sequence: 15
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="A merge helper for one file; a wrong merge is caught by the contract tests at the gate."
- Tier: 2
- Intent: testing.json merges without a hand rebase. DONE: (1) two branches that each add surfaces or groups merge by surface name, not by line, with the residual lists recomputed; (2) the merge lane and the branch verbs use it; (3) no unit lands with a hand-rebased testing.json hunk after this lands.
- Origin: human
- Next step: Evidence 2026-09-17: every batch merge today needed rebase-testing-json3.py from a seat scratchpad because the contract's arrays conflict on lines, and the three-way merge has no owner. No design first: brief the surface-keyed union, build, read. Lands beside the branch verbs of goals-live-on-branches.
- OpenedAt: 2026-09-17T13:39:42Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-17T13:40:00Z revision=2 opid=3TGNQ0V47XSAP3XK7PTYRBC585-m1e-c6925449 authority=proven digest=f093d05b4355aa8885f19c12a1d69eaaf9a6ab9a7c9229c38154591773e87589

History:
- 2026-09-17T13:39:42Z V6KR2GQBNYEPY0MZZ2JBCFTFXJ-m1e-c6925449 open actor=human:Wido targets=testing-json-merges-by-surface
- 2026-09-17T13:40:00Z 3TGNQ0V47XSAP3XK7PTYRBC585-m1e-c6925449 approve actor=human:Wido targets=testing-json-merges-by-surface
- 2026-09-17T13:40:24Z 7AAPQJPFSWQ4YNRWWBH3BXKC25-m1e-c6925449 set-priority actor=human:Wido targets=adoption-filled-delivery-passes-on-trunk,beds-run-under-the-oldest-supported-bash,brief-declares-the-round-boundary,builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,codex-jobs-run-through-a-metasystem-verb,coordinator-context-stays-under-budget,critique-closes-on-folded-proof,cross-cutting-change-inventories-its-readers,deep-sections-cannot-stay-red-unseen,degraded-stop-forms-have-one-source,delegate-sandbox-runs-the-beds,design-rounds-read-less-and-repeat-less,engine-policy-binding-survives-drift-and-load,every-round-gets-an-independent-read,evidence-and-build-output-have-retention-and-stay-unindexed,failures-show-observed-against-expected,fixture-runners-and-fakes-bound-their-own-life,fleet-doctor-repairs-what-stops-other-seats,follow-up-brief-cannot-cite-fresh-trunk-files,human-goal-verbs-forgiving,one-steward-per-checkout-on-its-own-root,proof-runs-reap-what-they-armed-and-a-census-names-the-rest,receipt-admission-caps-concurrent-batteries,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,stop-capability-follows-the-lease-epoch,stop-decision-surface-is-a-gate,stop-gate-sees-harness-tracked-work,stop-hook-never-forces-an-empty-turn,stop-response-carries-a-structured-report-reference,test-environment-edges-are-closed,testing-json-merges-by-surface,testing-surfaces-declare-their-mirror,tests-never-wait-on-wall-time,units-run-through-one-launcher reason=priority-order subject=testing-json-merges-by-surface from=unranked to=1:15 requested-sequence=15
Integrity: sha256=7c7ff6670e7d55833032f92c5076ce5aa7a2ead7e817208444826726e29f1c7c
