# error-checks-use-typed-errors

- State: parked
- Priority: 1
- Sequence: 4
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="In-process refactor with no process boundary; behaviour preserved; tests pin each decision; reverts cleanly"
- Tier: 1
- Intent: Go code decides on errors with errors.Is/As and typed errors, never by searching an error's text for a code or phrase
- Origin: main
- Next step: Replace the 35 in-process error-text decision points (24 sites) inventoried in /Users/wido/LocalStorage/agentic-tools-evidence/structured-output-20260930/structured-output.md with typed errors; extend the static audit to refuse new ones. Priority 2 per Wido 2026-09-30 (D4)
- OpenedAt: 2026-09-30T12:03:35Z
- Revision: 2
- BlockedBy: processes-read-structured-results
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Parked: by=human:wido at=2026-09-30T12:03:35Z blocker=processes-read-structured-results because=blocked by processes-read-structured-results; returns when it is done

History:
- 2026-09-30T12:03:35Z JMRCDJGB10YRZKEABA5JT58418-m1e-b6a4eb0a open actor=human:wido targets=error-checks-use-typed-errors,processes-read-structured-results
- 2026-09-30T12:04:18Z PB0V98VRJZ6BRTAQJY39PQ31XN-m1e-b6a4eb0a set-priority actor=human:wido targets=adoption-filled-delivery-passes-on-trunk,beds-run-under-the-oldest-supported-bash,brief-declares-the-round-boundary,builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,critique-closes-on-folded-proof,cross-cutting-change-inventories-its-readers,delegate-sandbox-runs-the-beds,design-rounds-read-less-and-repeat-less,error-checks-use-typed-errors,every-round-gets-an-independent-read,evidence-and-build-output-have-retention-and-stay-unindexed,failures-show-observed-against-expected,fixture-runners-and-fakes-bound-their-own-life,fleet-doctor-repairs-what-stops-other-seats,follow-up-brief-cannot-cite-fresh-trunk-files,machinery-runs-unattended-on-codex,one-steward-per-checkout-on-its-own-root,proof-runs-reap-what-they-armed-and-a-census-names-the-rest,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,seats-spend-tokens-in-bounded-sessions,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,test-environment-edges-are-closed,testing-surfaces-declare-their-mirror reason=priority-order subject=error-checks-use-typed-errors from=unranked to=1:4 requested-sequence=4
Integrity: sha256=ef0cbb6aa186fb45e8a9bfa910741d558c4d2d54a839cd6d9bf41f85b916f428
