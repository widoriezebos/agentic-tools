# ui-shows-true-facts

- State: claimed
- Priority: 1
- Sequence: 4
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Wrong facts mislead the person's decisions (severity 2); fixes in existing readers (novelty 1); every page (exposure 2); read daily (accumulation 2)"
- Tier: 2
- Intent: Every count and state the web UI shows matches the engine (UI audit 2026-10-02, agentic-tools-evidence/ui-audit-20261002/report.md, section WRONG FACT and BROKEN): Needs you counts only what truly needs the person (priority 1 / actionable), one total shared by Overview and Decisions, open questions counted from the channel's open questions, In Progress cards show the engine's phase, a claimed goal shows claimed, the fleet health and expanded-row facts agree with the engine, duplicate known-issue rows gone, unknown tabs and goals refused, ruling rows titled by their sentence.
- Origin: main
- Next step: Fix each WRONG FACT and BROKEN item in report.md with a test that reads the same fact from the engine; check with Playwright (node script in internal/ui/web/_app) before and after; ASKED QHT1F72F9GNWE9CSN9HQE45TCV (other): ui-shows-true-facts is built and green, but no route will hand it in for review. All tests pass when run from metasystem/ (Go UI packages, 1879 web tests, type check, fast gate). The unit's proof was red only because I gave a check that ran from the repository root, where there is no Go module (my mistake). Then three lawful routes refused: (1) work review G: no work waiting for review, because the attempt is red; work revise would reuse the same stored check. (2) work review G --changes, run from the goal's own worktree: "isolation audit failed: .claude/settings.json still resolves into the primary checkout". The audit compares the worktree with itself, so this route always refuses there. (3) work review G --patch from this checkout: the commit fence refused, "HEAD names no branch", although the goal worktree is on goal/ui-shows-true-facts. The change is kept as a stash in the goal worktree and as /tmp/ui-shows-true-facts.patch.; ASKED 07PYSG7DP3CT144F02N7EHYABJ (other): The UI-facts work passed review with no findings, but the engine won't close that review, so it can't land. How should I get past this?; ASKED B4SA3R6RG0K5J9SNBZ4HTH71GR (other): The UI-facts review is now closed clean, but the engine can't record that result on the goal branch, and landing needs it. Do you want to wait for an engine fix, and should this goal's 4-hour time budget be raised in the meantime?
- OpenedAt: 2026-10-02T21:54:47Z
- Revision: 9
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 1
- NormApproval: approvedRef=RHEM47C52E8V7MJQ14EEKAQ0XK-m1e-718ba0eb minutes=1200 reviewRounds=20 goalRevision=8
- Approved: by=human:Wido at=2026-10-03T02:51:04Z revision=9 opid=RHEM47C52E8V7MJQ14EEKAQ0XK-m1e-718ba0eb authority=proven digest=044314e0b948c9c3021e3ff3a9d41208c374f769cb8fd4cd00680c8622bbe27c episode=9
- Sliced: machine=m1h lineage=steward-seat revision=4 at=2026-10-03T00:35:35Z
- Claimed: machine=m1h lineage=steward-seat at=2026-10-03T02:51:04Z revision=9 accountingRevision=9 episodeAt=2026-10-02T22:28:16Z episodeRevision=4
- StopCapability: generation=9 revision=9 machine=m1h claimEpoch=9 fenceEpoch=0

History:
- 2026-10-02T21:54:47Z X6S7W8A5D6VQ483BBBRX5VSDGR-m1e-9c612d71 open actor=human:Wido targets=ui-shows-true-facts
- 2026-10-02T21:54:53Z BT9F3JXESQ027E6QZ70NTQVVDY-m1e-9c612d71 approve actor=human:Wido targets=ui-shows-true-facts
- 2026-10-02T21:54:59Z M0TSVAQNR16NW151930292CY3V-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,critique-stops-on-convergence,cross-cutting-change-inventories-its-readers,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,fleet-forgets-an-unreachable-seat,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,rosters-are-configuration-items,round-proof-feeds-the-next-brief,seat-path-lands-without-help,seat-works-without-a-person,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,ui-connects-to-a-running-agent,ui-shows-true-facts,work-review-starts-its-critic reason=priority-order subject=ui-shows-true-facts from=unranked to=1:4 requested-sequence=4
- 2026-10-02T22:28:16Z 35F1SFBERVEFTZJ3TYJCYF8472-m1h-71c5cb39 claim actor=m1h+steward-seat targets=ui-shows-true-facts
- 2026-10-02T23:08:07Z W3NV3BFTCH7WWAZA3GWBGA3J4F-m1h-71c5cb39 ask actor=m1h+steward-seat targets=ui-shows-true-facts
- 2026-10-03T00:35:35Z JZRRMRBTHD8K76BFCXY11X1G6D-m1h-71c5cb39 slice-start actor=m1h+steward-seat targets=ui-shows-true-facts
- 2026-10-03T01:51:38Z RSJHC0RRY8ENCHEAJXPNPW79QF-m1h-71c5cb39 ask actor=m1h+steward-seat targets=ui-shows-true-facts
- 2026-10-03T02:06:09Z E5T2DHBMKS3H8SMF7Z3H4T03C1-m1h-71c5cb39 ask actor=m1h+steward-seat targets=ui-shows-true-facts
- 2026-10-03T02:51:04Z RHEM47C52E8V7MJQ14EEKAQ0XK-m1e-718ba0eb set-budget actor=human:Wido targets=ui-shows-true-facts displaced=m1h+steward-seat@2026-10-02T22:28:16Z
Integrity: sha256=aeef081153ae6717d18815608473ea497d9207032e7308ca27a794c18cd4746f
