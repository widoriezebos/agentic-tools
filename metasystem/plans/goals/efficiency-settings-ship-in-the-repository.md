# efficiency-settings-ship-in-the-repository

- State: approved
- Priority: 1
- Sequence: 11
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="Configuration read at launch on every seat; a wrong value costs tokens or a compaction, never product code."
- Tier: 2
- Intent: Every token-efficiency setting a seat runs with is tracked configuration in the repository, read by the verbs, not a per-machine file or a memory rule. DONE: (1) the seat context window (200K), the delegate window per launch kind (build and design by brief 200K, read 400K), the Codex model and effort for builds (gpt-5.6-sol, xhigh), the wait cap (240 s) and the compaction handoff rule live in tracked configuration under metasystem/ and .claude/settings.json, with one verb that prints each value and its source; (2) a fresh clone on a new machine runs with them without settings.local.json, ~/.codex/config.toml or a memory file; (3) context status reports the window and its source.
- Origin: human
- Next step: Evidence 2026-09-17: the 200K window sits in each seat's git-ignored settings.local.json, the Codex effort in ~/.codex/config.toml, the wait cap in a memory file; a fresh checkout would run at 16 September's cost (240M weighted tokens per 13K lines against 84M per 12K on 17 September). No design first: brief the configuration keys and the source-printing verb, build, read. The launchers of delegate-launchers-become-go-verbs read these keys; delegate-runs-stay-under-200k-context is the delegate half.
- OpenedAt: 2026-09-17T13:39:27Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-17T13:39:45Z revision=2 opid=Y3E2FR6RKCPPB6VNHR6WW0YR12-m1e-c6925449 authority=proven digest=45b3b66b26c28389fc46a79626e2ec68774ab57424baebe521653660ba4d62c9

History:
- 2026-09-17T13:39:27Z ZBDB0RDD8WWHMXBCP3ADCN2FBT-m1e-c6925449 open actor=human:Wido targets=efficiency-settings-ship-in-the-repository
- 2026-09-17T13:39:45Z Y3E2FR6RKCPPB6VNHR6WW0YR12-m1e-c6925449 approve actor=human:Wido targets=efficiency-settings-ship-in-the-repository
- 2026-09-17T13:40:04Z S6VENT6YH1RWQKMSE0C2EXTH8T-m1e-c6925449 set-priority actor=human:Wido targets=adoption-filled-delivery-passes-on-trunk,beds-run-under-the-oldest-supported-bash,brief-declares-the-round-boundary,builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,codex-jobs-run-through-a-metasystem-verb,coordinator-context-stays-under-budget,critique-closes-on-folded-proof,cross-cutting-change-inventories-its-readers,deep-sections-cannot-stay-red-unseen,degraded-stop-forms-have-one-source,delegate-sandbox-runs-the-beds,design-rounds-read-less-and-repeat-less,efficiency-settings-ship-in-the-repository,engine-policy-binding-survives-drift-and-load,every-round-gets-an-independent-read,evidence-and-build-output-have-retention-and-stay-unindexed,failures-show-observed-against-expected,fixture-runners-and-fakes-bound-their-own-life,fleet-doctor-repairs-what-stops-other-seats,follow-up-brief-cannot-cite-fresh-trunk-files,human-goal-verbs-forgiving,one-steward-per-checkout-on-its-own-root,proof-runs-reap-what-they-armed-and-a-census-names-the-rest,receipt-admission-caps-concurrent-batteries,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,stop-capability-follows-the-lease-epoch,stop-decision-surface-is-a-gate,stop-gate-sees-harness-tracked-work,stop-hook-never-forces-an-empty-turn,stop-response-carries-a-structured-report-reference,test-environment-edges-are-closed,testing-surfaces-declare-their-mirror,tests-never-wait-on-wall-time,units-run-through-one-launcher reason=priority-order subject=efficiency-settings-ship-in-the-repository from=unranked to=1:11 requested-sequence=11
Integrity: sha256=370bb0060ef7078243d68c72d791d2091016d81541b59bf704cb7dc682652f65
