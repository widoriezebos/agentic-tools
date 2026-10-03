# every-launched-agent-waits-inside-its-turn

- State: approved
- Priority: 1
- Sequence: 1
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="One instruction sentence added to launch briefs; prevents lost work"
- Tier: 1
- Intent: Every agent the machinery launches headless (seat sessions already; also builders, critics, readers and the landing agent) is told that its process ends when its turn ends and must wait inside the turn (Wido 2026-10-03, lessons from m1f: attempt 4 of root-structs ended its turn while its full test run was still going, so the run died with it; the rule exists only in the seat brief, internal/steward/seat_start.go).
- Origin: main
- Next step: Find every launch brief template (unit builds, critic and read delegates, landing agent) and add the one-sentence rule in the shared place they are assembled; keep the message and always-loaded budgets green; test that each launched kind's brief carries it.
- OpenedAt: 2026-10-03T07:55:58Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-03T07:56:28Z revision=2 opid=4X22XWN6KQ9J1G3ZXB36K95S8J-m1e-718ba0eb authority=proven digest=f7f88e08ab4f7df9d39a14155103902a7336a2a6c10043e89c4685be6fff8e24 episode=2

History:
- 2026-10-03T07:55:58Z FP9QK48TH80TX90Z65RW40T7PY-m1e-718ba0eb open actor=human:Wido targets=every-launched-agent-waits-inside-its-turn
- 2026-10-03T07:56:28Z 4X22XWN6KQ9J1G3ZXB36K95S8J-m1e-718ba0eb approve actor=human:Wido targets=every-launched-agent-waits-inside-its-turn
- 2026-10-03T07:56:57Z 74GC7GC0HN0GDXXTC1HMFS6DRB-m1e-718ba0eb set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,channel-questions-stand-alone,conflicts-resolve-unattended,critique-stops-on-convergence,cross-cutting-change-inventories-its-readers,every-launched-agent-waits-inside-its-turn,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,fleet-forgets-an-unreachable-seat,fleet-page-redesign,fleet-panel-ux,machinery-blocks-of-2026-10-02,one-folder-deployed-and-evolved,one-steward-per-checkout-on-its-own-root,overrides-state-their-impact,partner-runtime-defaults-to-an-available-one,questions-carry-what-they-ask-about,receipt-writer-follows-the-worktree-it-runs-in,rosters-are-configuration-items,round-proof-feeds-the-next-brief,seat-path-lands-without-help,seat-works-without-a-person,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,ui-connects-to-a-running-agent reason=priority-order subject=every-launched-agent-waits-inside-its-turn from=unranked to=1:1 requested-sequence=1
Integrity: sha256=3e9346977e00cd93c3a049396b9e4dfcfbebfc13852fbcff593a3ef0d7487a9c
