# a-flake-has-an-owner-from-its-first-sighting

- State: queued
- Priority: 1
- Sequence: 125
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: A flake gets an owner at its first sighting, and an owner whose repair failed again is reopened (steward_flake.go:42, flakefix.go:43-49)
- Origin: human
- DesignGate: off
- Next step: Build without a design from N7 and section 7 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: three states (repair pending, landed awaiting closure passes, recurring after repair); only the last reopens; abandoned owners keep the person route. Approve only after the incident dispositions of row 33 are done
- OpenedAt: 2026-11-07T09:53:08Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T09:53:08Z 5C28T70NWK63AYYY33RFSA32T1-m1e-1dae276b open actor=human:Wido targets=a-flake-has-an-owner-from-its-first-sighting
- 2026-11-07T09:53:17Z MVH1847W5ZWHJVG8C866V47VRG-m1e-1dae276b edit actor=human:Wido targets=a-flake-has-an-owner-from-its-first-sighting reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a recorded finding whose root cause is read in the fixes analysis revision 3 and critiqued twice by Astra impact=This lets goal a-flake-has-an-owner-from-its-first-sighting be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-flake-has-an-owner-from-its-first-sighting until withdrawn. To undo: metasystem goal disallow a-flake-has-an-owner-from-its-first-sighting build-without-design
- 2026-11-07T09:53:26Z 5TBWB3S26F01FCFJTNMN45GKKF-m1e-1dae276b set-priority actor=human:Wido targets=a-flake-has-an-owner-from-its-first-sighting reason=priority-order subject=a-flake-has-an-owner-from-its-first-sighting from=unranked to=1:126 requested-sequence=append
- 2026-11-07T09:57:17Z DKWF3Z6RV2420ZW19S47K2RHBT-m1e-1dae276b done actor=human:Wido targets=a-boundary-waits-only-on-its-own-questions,a-delivered-goal-and-a-needed-act-reach-the-person,a-final-prefix-red-is-repeated-before-own,a-flake-has-an-owner-from-its-first-sighting,a-lane-act-runs-from-the-phone,a-persons-terminal-is-not-a-seat-main-2,a-proof-unit-ends-on-time-2,a-publication-refuses-only-what-it-adds,a-question-reminds-and-anyone-can-ask,a-retained-read-brief-keeps-its-context,a-review-keeps-every-read,a-spent-box-asks-before-it-blocks,a-start-that-never-launched-is-collected,a-stewards-standing-defect-reaches-an-owner,a-stop-waits-for-the-unit-boundary-2,a-unit-check-runs-the-importers-too,a-unit-reservation-carries-its-deadline,flake-fix-metasystem-cmd-metasystem-testproofruncommandgovernedparentsharesonecharge,flake-fix-metasystem-cmd-metasystem-testproviderrecoveryaskpubliclifecycle,flake-fix-metasystem-internal-proofrun-testtestenvironmentstandardinventorymatchesobserved,one-person-one-truth,red-names-what-failed,steward-flake-fixer-resolves-its-machine,tests-observe-only-what-they-inject-2,the-lane-deploys-the-engine-it-runs-2,the-lane-returns-its-own-red reason=priority-order from=1:126 to=1:125
Integrity: sha256=5dbda35820445c476b24b5c6f0cd56c6328d4b07ba62add4eef034e6e38a80aa
