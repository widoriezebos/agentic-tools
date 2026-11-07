# a-start-that-never-launched-is-collected

- State: approved
- Priority: 1
- Sequence: 121
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: A step saved as starting whose launch never existed is offered to its seat like an ended one (finding 500; read_sequence.go:72-76, seat_idle.go:120-127, person_wait.go:112-160)
- Origin: human
- DesignGate: off
- Next step: Build without a design from N3 and section 7 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: one never-launched predicate with the same clock and custody evidence for discovery and reconciliation; the question stays until the start advances or is cancelled; test successive ticks; about 40 lines
- OpenedAt: 2026-11-07T09:50:38Z
- Revision: 5
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-07T09:50:57Z revision=3 opid=HCWKMAJAEEDTDQV99E42VSVHKB-m1e-1dae276b authority=proven digest=bb8243195901accea46ffe411ad38bf1e73c7122d9dba441c40418353915d16a episode=3

History:
- 2026-11-07T09:50:38Z TF94QQDXGFHX5CN2RN7T4HT3SG-m1e-1dae276b open actor=human:Wido targets=a-start-that-never-launched-is-collected
- 2026-11-07T09:50:48Z YDS3RP9RD5TPZSZDZCGTZ9FY4M-m1e-1dae276b edit actor=human:Wido targets=a-start-that-never-launched-is-collected reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a recorded finding whose root cause is read in the fixes analysis revision 3 and critiqued twice by Astra impact=This lets goal a-start-that-never-launched-is-collected be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-start-that-never-launched-is-collected until withdrawn. To undo: metasystem goal disallow a-start-that-never-launched-is-collected build-without-design
- 2026-11-07T09:50:57Z HCWKMAJAEEDTDQV99E42VSVHKB-m1e-1dae276b approve actor=human:Wido targets=a-start-that-never-launched-is-collected
- 2026-11-07T09:51:06Z 52834X15WZG22DTPQ4RZMDE78T-m1e-1dae276b set-priority actor=human:Wido targets=a-start-that-never-launched-is-collected reason=priority-order subject=a-start-that-never-launched-is-collected from=unranked to=1:122 requested-sequence=append
- 2026-11-07T09:57:17Z DKWF3Z6RV2420ZW19S47K2RHBT-m1e-1dae276b done actor=human:Wido targets=a-boundary-waits-only-on-its-own-questions,a-delivered-goal-and-a-needed-act-reach-the-person,a-final-prefix-red-is-repeated-before-own,a-flake-has-an-owner-from-its-first-sighting,a-lane-act-runs-from-the-phone,a-persons-terminal-is-not-a-seat-main-2,a-proof-unit-ends-on-time-2,a-publication-refuses-only-what-it-adds,a-question-reminds-and-anyone-can-ask,a-retained-read-brief-keeps-its-context,a-review-keeps-every-read,a-spent-box-asks-before-it-blocks,a-start-that-never-launched-is-collected,a-stewards-standing-defect-reaches-an-owner,a-stop-waits-for-the-unit-boundary-2,a-unit-check-runs-the-importers-too,a-unit-reservation-carries-its-deadline,flake-fix-metasystem-cmd-metasystem-testproofruncommandgovernedparentsharesonecharge,flake-fix-metasystem-cmd-metasystem-testproviderrecoveryaskpubliclifecycle,flake-fix-metasystem-internal-proofrun-testtestenvironmentstandardinventorymatchesobserved,one-person-one-truth,red-names-what-failed,steward-flake-fixer-resolves-its-machine,tests-observe-only-what-they-inject-2,the-lane-deploys-the-engine-it-runs-2,the-lane-returns-its-own-red reason=priority-order from=1:122 to=1:121
Integrity: sha256=86e6e4e22989ca0f10e84d15d40a32d604d06626f253ac408ea9a72fece63911
