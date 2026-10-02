# blocked-agent-asks-the-human

- State: approved
- Priority: 1
- Sequence: 1
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Decides whether unattended agents stop safely and get unblocked (severity 2); builds on existing question and channel verbs (novelty 1); every seat and the lane (exposure 2); every block of every run (accumulation 2)"
- Tier: 2
- Intent: Every agent (seat agents, the lane agent, delegates) that hits a block it cannot resolve in one obvious step asks the human instead of fighting or spinning, and the human unblocks it quickly from Telegram or the UI (Wido 2026-10-02: rely on the human to get in the loop easily; no perfection). This is what lets the machinery switch on. Step 1: the ask-on-block rule in every agent skill and brief incl. the landing skill (refusal line 1, what it tried, the decision needed) via metasystem question ask; the agent waits cheaply on question wait (no turn burn, no Stop-hook loop); the answer is a decision, the human doing the act, or a bypass; the steward asks on an agent's behalf when a seat or the lane is silent without progress (the lane sooner, it blocks every seat); spend-fence and budget alerts reach the human through the same channel; every unblock is logged (question, answer, time) as the backlog of blocks to fix by frequency. Step 2: the UI shows and answers questions per seat and carries free-form human messages to a seat's agent.
- Origin: main
- Next step: Inventory what question ask/answer/wait, the Telegram channel, the UI and the peer-message bridge already do; design step 1; critique; build
- OpenedAt: 2026-10-02T12:24:05Z
- Revision: 8
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-02T12:29:42Z revision=7 opid=YG7J36XV18ENV1D8TR55CX9E9A-m1e-9c612d71 authority=proven digest=48b04d7686eaa4569893a5283340bb53ebc04468b30b581531164cc89416a033 episode=7

History:
- 2026-10-02T12:24:05Z FV4HRFVE842X06TWH326PZXQHJ-m1e-9c612d71 open actor=human:Wido targets=blocked-agent-asks-the-human
- 2026-10-02T12:24:12Z 4XYQCHAHJH99A9J7AZAM569JG5-m1e-9c612d71 approve actor=human:Wido targets=blocked-agent-asks-the-human
- 2026-10-02T12:24:19Z 26ZZJC6E5YSPFZ2ZM308BKBMS3-m1e-9c612d71 set-pin actor=human:Wido targets=blocked-agent-asks-the-human
- 2026-10-02T12:24:25Z KS49YJBH3D4MGEAWVK9G8Y150S-m1e-9c612d71 set-priority actor=human:Wido targets=blocked-agent-asks-the-human,builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,critique-stops-on-convergence,cross-cutting-change-inventories-its-readers,design-rounds-read-less-and-repeat-less,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,host-setup-from-scratch,landing-deploys-the-engine,landing-lane-runtime-redesign,old-lane-plumbing-is-deleted,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,reviewers-check-the-rulings,round-proof-feeds-the-next-brief,seat-path-lands-without-help,seat-successor-continues-a-handoff-without-a-human,seat-works-without-a-person,seats-spend-tokens-in-bounded-sessions,spend-fence-reports-tokens-per-model-and-cause,steward-acts-on-behaviour-patterns,stop-hook-never-forces-an-empty-turn,system-start-launches-your-agent,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,work-review-starts-its-critic reason=priority-order subject=blocked-agent-asks-the-human from=unranked to=1:2 requested-sequence=2
- 2026-10-02T12:29:31Z 6ZM9CX3F84JCZ26H99G3SMCS33-m1e-9c612d71 unapprove actor=human:Wido targets=blocked-agent-asks-the-human reason=add the two switch-on conditions
- 2026-10-02T12:29:37Z EXXWF9H3V2NVPYGB2DDCK8Q4VD-m1e-9c612d71 edit actor=human:Wido targets=blocked-agent-asks-the-human
- 2026-10-02T12:29:42Z YG7J36XV18ENV1D8TR55CX9E9A-m1e-9c612d71 approve actor=human:Wido targets=blocked-agent-asks-the-human
- 2026-10-02T12:49:24Z EQFW4CYPEQWW7Z9Z92KDJGB0YP-m1e-f456f182 done actor=human:Wido targets=blocked-agent-asks-the-human,builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,cross-cutting-change-inventories-its-readers,design-rounds-read-less-and-repeat-less,evidence-and-build-output-have-retention-and-stay-unindexed,flaky-leftovers,fleet-doctor-repairs-what-stops-other-seats,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,seat-works-without-a-person,seats-spend-tokens-in-bounded-sessions,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror reason=priority-order from=1:2 to=1:1
Integrity: sha256=58fa2e8ced4a1bae15d0056e448d897a8b9f5a58af2842dc77941df277cd20c3
