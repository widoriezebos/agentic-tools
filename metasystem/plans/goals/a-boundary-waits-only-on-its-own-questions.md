# a-boundary-waits-only-on-its-own-questions

- State: approved
- Priority: 1
- Sequence: 119
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: A seat unit boundary waits only on questions about the goals it holds (finding 538: ObserveUnitBoundary returns early on any open channel question, internal/steward/unit_boundary.go:77-83)
- Origin: human
- DesignGate: off
- Next step: Build without a design from N1 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: filter the question walk to held goals (work.Claimed, work.Landing) and machine questions whose subject is a held goal; test both cases; about 15 lines
- OpenedAt: 2026-11-07T09:49:10Z
- Revision: 5
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-07T09:49:42Z revision=3 opid=WNJRB35PXB8QARWZ7AMNW08XK7-m1e-1dae276b authority=proven digest=2878dc6a4f8d1659ec5afd9b84c64b5c27d77c2f7be4e8836ba76edc19ae2981 episode=3

History:
- 2026-11-07T09:49:10Z VY1D9MEXXK0T71JXK98RN18M0A-m1e-1dae276b open actor=human:Wido targets=a-boundary-waits-only-on-its-own-questions
- 2026-11-07T09:49:21Z EXEVQWSHTS5TDXM4PYNS7BS3MQ-m1e-1dae276b edit actor=human:Wido targets=a-boundary-waits-only-on-its-own-questions reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a recorded finding whose root cause is read in the fixes analysis revision 3 and critiqued twice by Astra impact=This lets goal a-boundary-waits-only-on-its-own-questions be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-boundary-waits-only-on-its-own-questions until withdrawn. To undo: metasystem goal disallow a-boundary-waits-only-on-its-own-questions build-without-design
- 2026-11-07T09:49:42Z WNJRB35PXB8QARWZ7AMNW08XK7-m1e-1dae276b approve actor=human:Wido targets=a-boundary-waits-only-on-its-own-questions
- 2026-11-07T09:49:51Z 075EGD168D20XS7Y8DXP82J4SH-m1e-1dae276b set-priority actor=human:Wido targets=a-boundary-waits-only-on-its-own-questions reason=priority-order subject=a-boundary-waits-only-on-its-own-questions from=unranked to=1:120 requested-sequence=append
- 2026-11-07T09:57:17Z DKWF3Z6RV2420ZW19S47K2RHBT-m1e-1dae276b done actor=human:Wido targets=a-boundary-waits-only-on-its-own-questions,a-delivered-goal-and-a-needed-act-reach-the-person,a-final-prefix-red-is-repeated-before-own,a-flake-has-an-owner-from-its-first-sighting,a-lane-act-runs-from-the-phone,a-persons-terminal-is-not-a-seat-main-2,a-proof-unit-ends-on-time-2,a-publication-refuses-only-what-it-adds,a-question-reminds-and-anyone-can-ask,a-retained-read-brief-keeps-its-context,a-review-keeps-every-read,a-spent-box-asks-before-it-blocks,a-start-that-never-launched-is-collected,a-stewards-standing-defect-reaches-an-owner,a-stop-waits-for-the-unit-boundary-2,a-unit-check-runs-the-importers-too,a-unit-reservation-carries-its-deadline,flake-fix-metasystem-cmd-metasystem-testproofruncommandgovernedparentsharesonecharge,flake-fix-metasystem-cmd-metasystem-testproviderrecoveryaskpubliclifecycle,flake-fix-metasystem-internal-proofrun-testtestenvironmentstandardinventorymatchesobserved,one-person-one-truth,red-names-what-failed,steward-flake-fixer-resolves-its-machine,tests-observe-only-what-they-inject-2,the-lane-deploys-the-engine-it-runs-2,the-lane-returns-its-own-red reason=priority-order from=1:120 to=1:119
Integrity: sha256=e2f92805fdeaea07df7830d9031075d45d4794b35ad3a13c84723eb21db17bf6
