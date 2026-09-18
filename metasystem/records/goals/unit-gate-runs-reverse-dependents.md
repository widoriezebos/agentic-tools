# unit-gate-runs-reverse-dependents

- State: done
- Priority: 1
- Sequence: 48
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Gate-only change on the landing lane and a new read-only verb; a wrong dependents computation shows as a missed or extra package in the gate's own tests"
- Tier: 2
- Intent: Every unit is gated as it stacks, by machinery. DONE when the batch lane's join gate and the verb metasystem gate unit --base <sha> run every reverse dependent of each changed package whole (with -tags batchtest for cmd/metasystem) and eject the unit on a red naming the failing tests.
- Origin: human
- Next step: U1 (join gate and gate unit --base run every reverse dependent; a red ejects the unit) on main d2713932efcdc9a3adee0493bd8196f3996e1bb4; nothing remains; gate unit runs after each stacked unit.
- Concluded: gate unit measured on row 165: 1550 s wall for 41 dependent packages, one load-fragile red classified to goal tests-never-wait-on-wall-time; the verb works as built and is the machinery's per-unit gate when armed; direct mode keeps gate-par v2 plus beds once per wave
- OpenedAt: 2026-09-18T14:56:10Z
- Revision: 8
- Budget: elapsedLimit=1d attemptLimit=20 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-18T14:59:39Z revision=3 opid=PA6032YQ1G064X0Z9Y1QAZWXZ9-m1e-c6925449 authority=proven digest=63ef3634356773f959dd428b7ed54f0b1e812050f057aa8ccb2166e1dbe3833a

History:
- 2026-09-18T14:56:10Z 15T6NP85HS5ADAYTWNPMWR3G0T-m1e-c6925449 open actor=human:Wido targets=unit-gate-runs-reverse-dependents
- 2026-09-18T14:58:37Z SZ75KQX9ZXQAXQ9TNKMTGYPQG3-m1e-c6925449 edit actor=human:Wido targets=unit-gate-runs-reverse-dependents
- 2026-09-18T14:59:39Z PA6032YQ1G064X0Z9Y1QAZWXZ9-m1e-c6925449 approve actor=human:Wido targets=unit-gate-runs-reverse-dependents
- 2026-09-18T15:01:01Z RTD7V1C31TB8MGA3AKEF18XZ96-m1e-c6925449 set-priority actor=human:Wido targets=unit-gate-runs-reverse-dependents reason=priority-order subject=unit-gate-runs-reverse-dependents from=unranked to=1:49 requested-sequence=49
- 2026-09-18T20:05:06Z GXZ1MMCGY1KHG1RTHGBSBV6KQC-m1e-c6925449 edit actor=human:Wido targets=unit-gate-runs-reverse-dependents
- 2026-09-18T21:51:33Z JPCGGPFJR0BZMVV5ED2BR6TEDQ-m1e-c6925449 edit actor=human:Wido targets=unit-gate-runs-reverse-dependents
- 2026-09-18T21:55:13Z JEN6XG86WCEPHZSSV0JTSVBPN9-m1e-c6925449 done actor=human:Wido targets=adoption-filled-delivery-passes-on-trunk,beds-run-under-the-oldest-supported-bash,brief-declares-the-round-boundary,build-jobs-fit-one-builder-context,builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,codex-jobs-run-through-a-metasystem-verb,coordinator-context-stays-under-budget,critique-closes-on-folded-proof,cross-cutting-change-inventories-its-readers,deep-sections-cannot-stay-red-unseen,delegate-launchers-become-go-verbs,delegate-runs-stay-under-200k-context,delegate-sandbox-runs-the-beds,design-rounds-read-less-and-repeat-less,efficiency-settings-ship-in-the-repository,engine-policy-binding-survives-drift-and-load,every-round-gets-an-independent-read,evidence-and-build-output-have-retention-and-stay-unindexed,failures-show-observed-against-expected,fixture-children-cannot-outlive-their-test,fixture-runners-and-fakes-bound-their-own-life,fleet-doctor-repairs-what-stops-other-seats,follow-up-brief-cannot-cite-fresh-trunk-files,glb-known-issues-are-fixed,go-tests-run-in-parallel,human-goal-verbs-forgiving,one-steward-per-checkout-on-its-own-root,proof-admission-fits-a-seat-proving-several-units,proof-runs-reap-what-they-armed-and-a-census-names-the-rest,reads-keep-the-whole-diff-in-context,receipt-writer-follows-the-worktree-it-runs-in,red-on-main-known-issues-are-fixed,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,seats-spend-tokens-in-bounded-sessions,spend-fence-reports-tokens-per-model-and-cause,stop-capability-follows-the-lease-epoch,stop-decision-surface-is-a-gate,stop-hook-never-forces-an-empty-turn,stop-response-carries-a-structured-report-reference,test-environment-edges-are-closed,testing-surfaces-declare-their-mirror,tests-never-wait-on-wall-time,unit-gate-runs-reverse-dependents,waits-run-outside-model-contexts reason=priority-order from=1:49 to=1:48
- 2026-09-18T21:56:01Z P8ZGPJCXV2MK92B9TKGAXD052G-m1e-c6925449 done actor=human:Wido targets=unit-gate-runs-reverse-dependents
Integrity: sha256=56b6e0cb43979bc5b875ea03f668ecc65db45569b8c88454effb1f7148f0382f
