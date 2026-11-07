# a-unit-reservation-carries-its-deadline

- State: approved
- Priority: 1
- Sequence: 120
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: Delivery health can read unit launches: a unit reservation carries capDeadline (finding 504; unit_launch.go:78-80 writes capMin only, delivery.go:241-245 requires both)
- Origin: human
- DesignGate: off
- Next step: Build without a design from N2 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: write capDeadline from createdAt and capMin; derive it for older unit-launch reservations in the reader; test through the delivery role on an artificial clock; about 15 lines
- OpenedAt: 2026-11-07T09:50:00Z
- Revision: 5
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-07T09:50:19Z revision=3 opid=DHN2PJXJC8RY2R4YNRCX9MD4WJ-m1e-1dae276b authority=proven digest=e4fbaf1e5076a4d35ab08b3887919790569e00ef4b0ee0cad9c8c9029e9ebff8 episode=3

History:
- 2026-11-07T09:50:00Z ABV4P0HD1XK260KM2R2CWSASP0-m1e-1dae276b open actor=human:Wido targets=a-unit-reservation-carries-its-deadline
- 2026-11-07T09:50:09Z QQCVD3N60AQH1QA8VYWV8XRKYZ-m1e-1dae276b edit actor=human:Wido targets=a-unit-reservation-carries-its-deadline reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a recorded finding whose root cause is read in the fixes analysis revision 3 and critiqued twice by Astra impact=This lets goal a-unit-reservation-carries-its-deadline be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-unit-reservation-carries-its-deadline until withdrawn. To undo: metasystem goal disallow a-unit-reservation-carries-its-deadline build-without-design
- 2026-11-07T09:50:19Z DHN2PJXJC8RY2R4YNRCX9MD4WJ-m1e-1dae276b approve actor=human:Wido targets=a-unit-reservation-carries-its-deadline
- 2026-11-07T09:50:29Z FMHRXXCTNWVF3AHZWE7VHHBR6E-m1e-1dae276b set-priority actor=human:Wido targets=a-unit-reservation-carries-its-deadline reason=priority-order subject=a-unit-reservation-carries-its-deadline from=unranked to=1:121 requested-sequence=append
- 2026-11-07T09:57:17Z DKWF3Z6RV2420ZW19S47K2RHBT-m1e-1dae276b done actor=human:Wido targets=a-boundary-waits-only-on-its-own-questions,a-delivered-goal-and-a-needed-act-reach-the-person,a-final-prefix-red-is-repeated-before-own,a-flake-has-an-owner-from-its-first-sighting,a-lane-act-runs-from-the-phone,a-persons-terminal-is-not-a-seat-main-2,a-proof-unit-ends-on-time-2,a-publication-refuses-only-what-it-adds,a-question-reminds-and-anyone-can-ask,a-retained-read-brief-keeps-its-context,a-review-keeps-every-read,a-spent-box-asks-before-it-blocks,a-start-that-never-launched-is-collected,a-stewards-standing-defect-reaches-an-owner,a-stop-waits-for-the-unit-boundary-2,a-unit-check-runs-the-importers-too,a-unit-reservation-carries-its-deadline,flake-fix-metasystem-cmd-metasystem-testproofruncommandgovernedparentsharesonecharge,flake-fix-metasystem-cmd-metasystem-testproviderrecoveryaskpubliclifecycle,flake-fix-metasystem-internal-proofrun-testtestenvironmentstandardinventorymatchesobserved,one-person-one-truth,red-names-what-failed,steward-flake-fixer-resolves-its-machine,tests-observe-only-what-they-inject-2,the-lane-deploys-the-engine-it-runs-2,the-lane-returns-its-own-red reason=priority-order from=1:121 to=1:120
Integrity: sha256=87a5d3254ce0e55608409f71e815bb37409949b14b2327167d4ea6510488cd97
