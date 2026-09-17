# reads-keep-the-whole-diff-in-context

- State: approved
- Priority: 1
- Sequence: 13
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="A launcher choice per read; a wrong choice costs one rerun."
- Tier: 2
- Intent: An independent read judges the whole diff without compacting. DONE: (1) a read over 1,200 diff lines runs per package, or headless in its own window (400K), chosen by the launcher from the diff size; (2) the read's return records its compaction count, and a read that compacted is rerun split before its verdict counts; (3) the retro reports zero compacted reads.
- Origin: human
- Next step: Evidence 2026-09-17: the reads of blb Build B (2,219 lines) and rom R2 compacted twice each under the 200K delegate window, so the reader judged the last files against a summary of the first; the C+D read (3,187 lines) ran headless at 400K as the workaround (scripts/agents/headless-design-launch.sh with CLAUDE_CODE_AUTO_COMPACT_WINDOW). No design first: brief the size rule, the window and the return field, build, read. Lands on the launcher of delegate-launchers-become-go-verbs; the window keys come from efficiency-settings-ship-in-the-repository.
- OpenedAt: 2026-09-17T13:39:34Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-17T13:39:53Z revision=2 opid=ZSTP02XSE2GK11Q40DRD4REDM1-m1e-c6925449 authority=proven digest=55fb7d3f4724ba8b78f2b9460d111ccc6164c2c0b85ab8c4d55ba8f91d871fdb

History:
- 2026-09-17T13:39:34Z JKXEBQE08X0WT13KJBAYGDS4TZ-m1e-c6925449 open actor=human:Wido targets=reads-keep-the-whole-diff-in-context
- 2026-09-17T13:39:53Z ZSTP02XSE2GK11Q40DRD4REDM1-m1e-c6925449 approve actor=human:Wido targets=reads-keep-the-whole-diff-in-context
- 2026-09-17T13:40:15Z J5GJQBKKZXZRM7R88X7Z2ZZXEJ-m1e-c6925449 set-priority actor=human:Wido targets=adoption-filled-delivery-passes-on-trunk,beds-run-under-the-oldest-supported-bash,brief-declares-the-round-boundary,builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,codex-jobs-run-through-a-metasystem-verb,coordinator-context-stays-under-budget,critique-closes-on-folded-proof,cross-cutting-change-inventories-its-readers,deep-sections-cannot-stay-red-unseen,degraded-stop-forms-have-one-source,delegate-sandbox-runs-the-beds,design-rounds-read-less-and-repeat-less,engine-policy-binding-survives-drift-and-load,every-round-gets-an-independent-read,evidence-and-build-output-have-retention-and-stay-unindexed,failures-show-observed-against-expected,fixture-runners-and-fakes-bound-their-own-life,fleet-doctor-repairs-what-stops-other-seats,follow-up-brief-cannot-cite-fresh-trunk-files,human-goal-verbs-forgiving,one-steward-per-checkout-on-its-own-root,proof-runs-reap-what-they-armed-and-a-census-names-the-rest,reads-keep-the-whole-diff-in-context,receipt-admission-caps-concurrent-batteries,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,stop-capability-follows-the-lease-epoch,stop-decision-surface-is-a-gate,stop-gate-sees-harness-tracked-work,stop-hook-never-forces-an-empty-turn,stop-response-carries-a-structured-report-reference,test-environment-edges-are-closed,testing-surfaces-declare-their-mirror,tests-never-wait-on-wall-time,units-run-through-one-launcher reason=priority-order subject=reads-keep-the-whole-diff-in-context from=unranked to=1:13 requested-sequence=13
Integrity: sha256=2333736d6fd50c3d0ff2f36b44f68d1b57f3a3440ab52d1d9c5f1debda22dd4a
