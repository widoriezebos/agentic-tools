# a-publication-refuses-only-what-it-adds

- State: approved
- Priority: 1
- Sequence: 124
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: A ledger publication is refused only for a dangling design reference it adds, not for ones its parent already had (finding 540; internal/project/publication.go:42-55)
- Origin: human
- DesignGate: off
- Next step: Build without a design from N6 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: on the refusal path read the parent commit problems and refuse only new ones; test the three-page sequence of finding 540; about 25 lines. Decided in Wido's name; rejected: atomic multi-goal open
- OpenedAt: 2026-11-07T09:52:30Z
- Revision: 5
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-07T09:52:49Z revision=3 opid=94CV1YAXB01FT4016HTMTES5M8-m1e-1dae276b authority=proven digest=cb572171f896227e37214c00138fd94765593ba3caeaa85d450584ddf770d494 episode=3

History:
- 2026-11-07T09:52:30Z Y34TYSP6876J4M98A72JAVGQ57-m1e-1dae276b open actor=human:Wido targets=a-publication-refuses-only-what-it-adds
- 2026-11-07T09:52:39Z K38GACFKG5G61YDD1T5TZWCRPH-m1e-1dae276b edit actor=human:Wido targets=a-publication-refuses-only-what-it-adds reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a recorded finding whose root cause is read in the fixes analysis revision 3 and critiqued twice by Astra impact=This lets goal a-publication-refuses-only-what-it-adds be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-publication-refuses-only-what-it-adds until withdrawn. To undo: metasystem goal disallow a-publication-refuses-only-what-it-adds build-without-design
- 2026-11-07T09:52:49Z 94CV1YAXB01FT4016HTMTES5M8-m1e-1dae276b approve actor=human:Wido targets=a-publication-refuses-only-what-it-adds
- 2026-11-07T09:52:59Z HESWH5NWSWWEMX5XN7SYKWBYCY-m1e-1dae276b set-priority actor=human:Wido targets=a-publication-refuses-only-what-it-adds reason=priority-order subject=a-publication-refuses-only-what-it-adds from=unranked to=1:125 requested-sequence=append
- 2026-11-07T09:57:17Z DKWF3Z6RV2420ZW19S47K2RHBT-m1e-1dae276b done actor=human:Wido targets=a-boundary-waits-only-on-its-own-questions,a-delivered-goal-and-a-needed-act-reach-the-person,a-final-prefix-red-is-repeated-before-own,a-flake-has-an-owner-from-its-first-sighting,a-lane-act-runs-from-the-phone,a-persons-terminal-is-not-a-seat-main-2,a-proof-unit-ends-on-time-2,a-publication-refuses-only-what-it-adds,a-question-reminds-and-anyone-can-ask,a-retained-read-brief-keeps-its-context,a-review-keeps-every-read,a-spent-box-asks-before-it-blocks,a-start-that-never-launched-is-collected,a-stewards-standing-defect-reaches-an-owner,a-stop-waits-for-the-unit-boundary-2,a-unit-check-runs-the-importers-too,a-unit-reservation-carries-its-deadline,flake-fix-metasystem-cmd-metasystem-testproofruncommandgovernedparentsharesonecharge,flake-fix-metasystem-cmd-metasystem-testproviderrecoveryaskpubliclifecycle,flake-fix-metasystem-internal-proofrun-testtestenvironmentstandardinventorymatchesobserved,one-person-one-truth,red-names-what-failed,steward-flake-fixer-resolves-its-machine,tests-observe-only-what-they-inject-2,the-lane-deploys-the-engine-it-runs-2,the-lane-returns-its-own-red reason=priority-order from=1:125 to=1:124
Integrity: sha256=f0a0d8c13168bd91308684525810a33697c9150fb596bb7eb28f41ffe6b0895a
