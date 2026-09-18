# batch-landing-known-issues-are-fixed

- State: approved
- Priority: 1
- Sequence: 2
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="Fixes to landed batch-landing code, each with a witness; a wrong fix is caught by its witness and the goal's proof."
- Tier: 2
- Intent: The known issues that batch landing (units-land-in-batches-under-one-proof) merges with are fixed forward right after the merge. DONE: (1) the four breaking items of the C+D read (MATERIAL 1 prefix receipt exit 76 aborts the landing, 2 Seal re-applies ejected units, 3 a prefix red loses its groups, 4 a moved trunk re-pushes on every tick) and their riders are fixed with witnesses, by blb FIX 1 unless it rode the merge; (2) C+D read MATERIAL 5-10 and its twelve minors are fixed with witnesses or recorded as not a defect with the reason; (3) Build B's read minors are fixed or recorded the same way; (4) BA14c, the two-tip landing-branch publication of the r3 amendment (candidate branch pushed before the proof, receipt-bearing series after green), is built; (5) while item 1 is open no seat routes a real landing through landing batch; (6) the goal's Next lists each open item until it is fixed. Read files: hact-20260912/m1e-tools-0917/blb-buildcd2-read-items.md and blb-buildb-read-items.md.
- Origin: human
- Next step: fix-bed-abandonment on main d2713932efcdc9a3adee0493bd8196f3996e1bb4; remaining: the closing re-read of the dm-g2read breaking items, then conclude; buildcd 30 s targetMs stays a stable-state item.
- OpenedAt: 2026-09-17T13:57:10Z
- Revision: 10
- Labels: known-issue
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-17T13:57:13Z revision=2 opid=42GYA2RA2V15419GPQNH1H23CY-m1e-c6925449 authority=proven digest=599da8e41a5f2f608230bf5bba199521e26671f736f04322515373351937d941

History:
- 2026-09-17T13:57:10Z F5AR6ZDRZ3CKX78BNRC8Y7FMG2-m1e-c6925449 open actor=human:Wido targets=batch-landing-known-issues-are-fixed
- 2026-09-17T13:57:13Z 42GYA2RA2V15419GPQNH1H23CY-m1e-c6925449 approve actor=human:Wido targets=batch-landing-known-issues-are-fixed
- 2026-09-17T13:57:17Z XP35NF64FBY21AH3JGB8Y5DNXD-m1e-c6925449 set-priority actor=human:Wido targets=adoption-filled-delivery-passes-on-trunk,batch-landing-known-issues-are-fixed,beds-run-under-the-oldest-supported-bash,brief-declares-the-round-boundary,build-jobs-fit-one-builder-context,builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,codex-jobs-run-through-a-metasystem-verb,coordinator-context-stays-under-budget,critique-closes-on-folded-proof,cross-cutting-change-inventories-its-readers,deep-sections-cannot-stay-red-unseen,degraded-stop-forms-have-one-source,delegate-launchers-become-go-verbs,delegate-runs-stay-under-200k-context,delegate-sandbox-runs-the-beds,design-rounds-read-less-and-repeat-less,efficiency-settings-ship-in-the-repository,engine-policy-binding-survives-drift-and-load,every-round-gets-an-independent-read,evidence-and-build-output-have-retention-and-stay-unindexed,failures-show-observed-against-expected,fixture-children-cannot-outlive-their-test,fixture-runners-and-fakes-bound-their-own-life,fleet-doctor-repairs-what-stops-other-seats,follow-up-brief-cannot-cite-fresh-trunk-files,goals-live-on-branches,human-goal-verbs-forgiving,one-steward-per-checkout-on-its-own-root,proof-admission-fits-a-seat-proving-several-units,proof-runs-reap-what-they-armed-and-a-census-names-the-rest,read-items-are-tracked-until-the-goal-closes,reads-keep-the-whole-diff-in-context,receipt-admission-caps-concurrent-batteries,receipt-writer-follows-the-worktree-it-runs-in,red-on-main-gets-an-owner-on-the-ledger,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,seats-spend-tokens-in-bounded-sessions,spend-fence-reports-tokens-per-model-and-cause,stop-capability-follows-the-lease-epoch,stop-decision-surface-is-a-gate,stop-gate-sees-harness-tracked-work,stop-hook-never-forces-an-empty-turn,stop-response-carries-a-structured-report-reference,test-environment-edges-are-closed,testing-json-merges-by-surface,testing-surfaces-declare-their-mirror,tests-never-wait-on-wall-time,units-run-through-one-launcher,waits-run-outside-model-contexts reason=priority-order subject=batch-landing-known-issues-are-fixed from=unranked to=1:2 requested-sequence=2
- 2026-09-17T16:47:54Z S4HSFEV9D25CFRY3JJCC8P43W3-m1e-c6925449 edit actor=human:Wido targets=batch-landing-known-issues-are-fixed
- 2026-09-17T18:10:38Z B7KWRNKSHPQ0AV0XC7HKCSDH9A-m1e-c6925449 edit actor=human:Wido targets=batch-landing-known-issues-are-fixed
- 2026-09-18T07:07:37Z CYD08HTRXQVGADXYFH4HEXSEZ2-m1e-c6925449 edit actor=human:Wido targets=batch-landing-known-issues-are-fixed
- 2026-09-18T14:09:05Z VX1W40WNY90QQDD2GAQGNXNYPZ-m1b-d0e95724 claim actor=m1b+main-1789560571-22295-8d907a targets=batch-landing-known-issues-are-fixed
- 2026-09-18T14:56:44Z SH2SY5T2G3Q2F1CGGTBDSBS3CH-m1e-c6925449 edit actor=human:Wido targets=batch-landing-known-issues-are-fixed displaced=m1b+main-1789560571-22295-8d907a@2026-09-18T14:09:05Z
- 2026-09-18T14:59:51Z J0TXX6390B5A5DM7JST552KQT7-m1e-c6925449 release actor=human:Wido targets=batch-landing-known-issues-are-fixed displaced=m1b+main-1789560571-22295-8d907a@2026-09-18T14:09:05Z
- 2026-09-18T20:05:43Z T72PQHQHAWS9F15FFZF44CEXE9-m1e-c6925449 edit actor=human:Wido targets=batch-landing-known-issues-are-fixed
Integrity: sha256=f28ad62b397653529fe5450190030354e229f4406297ae493da77ae3eebf5cb9
