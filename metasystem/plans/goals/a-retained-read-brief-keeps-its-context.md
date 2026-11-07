# a-retained-read-brief-keeps-its-context

- State: approved
- Priority: 1
- Sequence: 123
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: A correction round read keeps its read-context.md beside the retained brief (finding 546, decision 206; unit_step_failure.go:54-76, pack.go:49)
- Origin: human
- DesignGate: off
- Next step: Build without a design from N5 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: copy read-context.md with the retained brief; test a correction-round read passes the pack check; mutation skips the copy; about 10 lines
- OpenedAt: 2026-11-07T09:51:52Z
- Revision: 5
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-07T09:52:11Z revision=3 opid=EH0155AN4HMRDPJ58D4A6C9TZG-m1e-1dae276b authority=proven digest=16c6ba251919eaadb914fb68b73f3ef54a133b47bdf020f2f3b5e191709be753 episode=3

History:
- 2026-11-07T09:51:52Z 2MB0AZSAR03CF4H0KMTWWYASFE-m1e-1dae276b open actor=human:Wido targets=a-retained-read-brief-keeps-its-context
- 2026-11-07T09:52:02Z FQWV3VCV31KHX7V1PEZN3BC0F7-m1e-1dae276b edit actor=human:Wido targets=a-retained-read-brief-keeps-its-context reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a recorded finding whose root cause is read in the fixes analysis revision 3 and critiqued twice by Astra impact=This lets goal a-retained-read-brief-keeps-its-context be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-retained-read-brief-keeps-its-context until withdrawn. To undo: metasystem goal disallow a-retained-read-brief-keeps-its-context build-without-design
- 2026-11-07T09:52:11Z EH0155AN4HMRDPJ58D4A6C9TZG-m1e-1dae276b approve actor=human:Wido targets=a-retained-read-brief-keeps-its-context
- 2026-11-07T09:52:21Z RJ3RGFWK2J85HFQS37GSDXEG3W-m1e-1dae276b set-priority actor=human:Wido targets=a-retained-read-brief-keeps-its-context reason=priority-order subject=a-retained-read-brief-keeps-its-context from=unranked to=1:124 requested-sequence=append
- 2026-11-07T09:57:17Z DKWF3Z6RV2420ZW19S47K2RHBT-m1e-1dae276b done actor=human:Wido targets=a-boundary-waits-only-on-its-own-questions,a-delivered-goal-and-a-needed-act-reach-the-person,a-final-prefix-red-is-repeated-before-own,a-flake-has-an-owner-from-its-first-sighting,a-lane-act-runs-from-the-phone,a-persons-terminal-is-not-a-seat-main-2,a-proof-unit-ends-on-time-2,a-publication-refuses-only-what-it-adds,a-question-reminds-and-anyone-can-ask,a-retained-read-brief-keeps-its-context,a-review-keeps-every-read,a-spent-box-asks-before-it-blocks,a-start-that-never-launched-is-collected,a-stewards-standing-defect-reaches-an-owner,a-stop-waits-for-the-unit-boundary-2,a-unit-check-runs-the-importers-too,a-unit-reservation-carries-its-deadline,flake-fix-metasystem-cmd-metasystem-testproofruncommandgovernedparentsharesonecharge,flake-fix-metasystem-cmd-metasystem-testproviderrecoveryaskpubliclifecycle,flake-fix-metasystem-internal-proofrun-testtestenvironmentstandardinventorymatchesobserved,one-person-one-truth,red-names-what-failed,steward-flake-fixer-resolves-its-machine,tests-observe-only-what-they-inject-2,the-lane-deploys-the-engine-it-runs-2,the-lane-returns-its-own-red reason=priority-order from=1:124 to=1:123
Integrity: sha256=f1a0a072b3fef066761badf63d1560d90d60760c211ea6319c23f37d5d9c22d3
