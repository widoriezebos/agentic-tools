# every-launched-agent-waits-inside-its-turn

- State: queued
- Priority: 1
- Sequence: 1
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="The rule decides whether launched agents lose their work; it reaches every builder, critic and the landing agent, so a wrong or missing rule compounds"
- Tier: 2
- Intent: Every agent the machinery launches headless (seat sessions already; also builders, critics, readers and the landing agent) is told that its process ends when its turn ends and must wait inside the turn (Wido 2026-10-03, lessons from m1f: attempt 4 of root-structs ended its turn while its full test run was still going, so the run died with it; the rule exists only in the seat brief, internal/steward/seat_start.go).
- Origin: main
- Next step: Short design (which launch kinds, where their briefs are assembled, the exact rule, how each kind waits: work wait / question wait with bounds), Astra critique tier 2, then build with a test per launched kind; keep the message and always-loaded budgets green.
- OpenedAt: 2026-10-03T07:55:58Z
- Revision: 5
- BudgetExceptions: 0

History:
- 2026-10-03T07:55:58Z FP9QK48TH80TX90Z65RW40T7PY-m1e-718ba0eb open actor=human:Wido targets=every-launched-agent-waits-inside-its-turn
- 2026-10-03T07:56:28Z 4X22XWN6KQ9J1G3ZXB36K95S8J-m1e-718ba0eb approve actor=human:Wido targets=every-launched-agent-waits-inside-its-turn
- 2026-10-03T07:56:57Z 74GC7GC0HN0GDXXTC1HMFS6DRB-m1e-718ba0eb set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,channel-questions-stand-alone,conflicts-resolve-unattended,critique-stops-on-convergence,cross-cutting-change-inventories-its-readers,every-launched-agent-waits-inside-its-turn,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,fleet-forgets-an-unreachable-seat,fleet-page-redesign,fleet-panel-ux,machinery-blocks-of-2026-10-02,one-folder-deployed-and-evolved,one-steward-per-checkout-on-its-own-root,overrides-state-their-impact,partner-runtime-defaults-to-an-available-one,questions-carry-what-they-ask-about,receipt-writer-follows-the-worktree-it-runs-in,rosters-are-configuration-items,round-proof-feeds-the-next-brief,seat-path-lands-without-help,seat-works-without-a-person,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,ui-connects-to-a-running-agent reason=priority-order subject=every-launched-agent-waits-inside-its-turn from=unranked to=1:1 requested-sequence=1
- 2026-10-03T07:57:16Z A1173JJD040SYA8APZ0G7KA3X4-m1e-718ba0eb unapprove actor=human:Wido targets=every-launched-agent-waits-inside-its-turn reason=Raise to tier 2 so it gets design and critique (Wido 2026-10-03: these are important to get right)
- 2026-10-03T07:57:20Z 765J1XN3THA25G0X2GHJB530BF-m1e-718ba0eb edit actor=human:Wido targets=every-launched-agent-waits-inside-its-turn
Integrity: sha256=76d282834e398bd2ad27d33397f45590d1457759d745f6c14842565bb0b7f38c
