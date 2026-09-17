# build-jobs-fit-one-builder-context

- State: approved
- Priority: 1
- Sequence: 12
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="A launcher check on every build; a wrong cap delays a job, never lands code."
- Tier: 2
- Intent: A build job never exceeds what its builder holds in one context. DONE: (1) the launcher estimates a job's changed lines from the unit rows of its page and refuses a job above 1,500 lines, splitting it into serial jobs on the goal branch; (2) every build return records the builder's compaction count and call count; (3) the retro reports jobs over the cap and compactions per job, zero over the cap after landing.
- Origin: human
- Next step: Evidence 2026-09-17: every 2,200-line Codex job compacted three times inside Codex's 258K window; Build B's 13 material read items were page-conformance drift, the mark of a builder that lost the page; effort high against xhigh changed neither tokens nor time (7.2M and 8.8M per job). No design first: brief the estimate, the refusal and the return fields, build, read. Lands on the launcher of delegate-launchers-become-go-verbs.
- OpenedAt: 2026-09-17T13:39:30Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-17T13:39:49Z revision=2 opid=E80BMY94DX3D1QGDWXDSME1P3E-m1e-c6925449 authority=proven digest=eb9c220c45e2adb1895b52b25afe2e9d8476adf4454042a155a15381ef1826fd

History:
- 2026-09-17T13:39:30Z J7NVPQ4V8JBT3C49N8JWA815HD-m1e-c6925449 open actor=human:Wido targets=build-jobs-fit-one-builder-context
- 2026-09-17T13:39:49Z E80BMY94DX3D1QGDWXDSME1P3E-m1e-c6925449 approve actor=human:Wido targets=build-jobs-fit-one-builder-context
- 2026-09-17T13:40:10Z 2PNSMPC8FKMMFG66PBE23059MT-m1e-c6925449 set-priority actor=human:Wido targets=adoption-filled-delivery-passes-on-trunk,beds-run-under-the-oldest-supported-bash,brief-declares-the-round-boundary,build-jobs-fit-one-builder-context,builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,codex-jobs-run-through-a-metasystem-verb,coordinator-context-stays-under-budget,critique-closes-on-folded-proof,cross-cutting-change-inventories-its-readers,deep-sections-cannot-stay-red-unseen,degraded-stop-forms-have-one-source,delegate-sandbox-runs-the-beds,design-rounds-read-less-and-repeat-less,engine-policy-binding-survives-drift-and-load,every-round-gets-an-independent-read,evidence-and-build-output-have-retention-and-stay-unindexed,failures-show-observed-against-expected,fixture-runners-and-fakes-bound-their-own-life,fleet-doctor-repairs-what-stops-other-seats,follow-up-brief-cannot-cite-fresh-trunk-files,human-goal-verbs-forgiving,one-steward-per-checkout-on-its-own-root,proof-runs-reap-what-they-armed-and-a-census-names-the-rest,receipt-admission-caps-concurrent-batteries,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,stop-capability-follows-the-lease-epoch,stop-decision-surface-is-a-gate,stop-gate-sees-harness-tracked-work,stop-hook-never-forces-an-empty-turn,stop-response-carries-a-structured-report-reference,test-environment-edges-are-closed,testing-surfaces-declare-their-mirror,tests-never-wait-on-wall-time,units-run-through-one-launcher reason=priority-order subject=build-jobs-fit-one-builder-context from=unranked to=1:12 requested-sequence=12
Integrity: sha256=de39b55bb9dcb47ad75bbaf675b1ca66533407ead65777a61b984fe7a242d724
