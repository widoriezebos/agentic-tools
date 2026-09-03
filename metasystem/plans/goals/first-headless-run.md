# first-headless-run

- State: approved
- Intent: Run the metasystem headless for real: on m0, with no Claude session on top, the mission runner (family metasystem mission, cmd/metasystem/mission.go, so far exercised only through its fake host in scripts/agents/dispatch-fixtures.sh) picks one approved tier-2 goal, composes its brief, dispatches the build and the review, folds or defers findings, lands the result and talks to Wido through the fleet conversation channel where a severe finding needs his word; every defect the run surfaces is fixed forward as its own tier-1 or tier-2 item the same day; the run is recorded, and its landing on its own is the proof that the machines can be headless.
- Origin: main
- Next step: STEP 1 DONE (plan landed 75613a85); instruments + draft contract landed 512c0226, tag first-headless-run-instruments pushed; contract amended with the tier-3 ladder and R-42-m0's THREE-ROUND STOP (park and ask rather than a fourth round; runner enforces it as no-gain-budget=3), re-sealed - hash to sign 7e53aa1977e3f43148174ecb51a07701a04f4060ca5ed194f788fc1bccef7559. FINDING FOR THE RUN RECORD (Wido's question 2026-09-03): the mission contract is a SECOND human gate beside the ledger - its fences/exposure are a separate budget from the goal's ledger tuple; the contract limits should DERIVE from the ledger tuple, not be a second number (follow-up to open after the run). WAITING ON WIDO: (1) sign - relayed Approval line with the hash above; (2) goal approve job-record-birth-token (4h/6/240m). Then: land signed contract, ledger-init, baseline, state-init, release this goal, claim the target, shut down m0's owner, stop this session, start the runner from a plain shell, watch from outside, fix forward, record.
- OpenedAt: 2026-09-03T12:10:50Z
- Revision: 9
- Arc: headless-fleet
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1
- Approved: by=human:Wido at=2026-09-03T14:16:24Z revision=5 opid=JY54AQT8101VK9353WT9FQYGA9-m0-c5dbf036 authority=relayed digest=0c9d3b9a12438d4b0c36b5798c98f1e73e3bdf3312756c4b09088d8921cecf5f reviewBy=2026-09-06

History:
- 2026-09-03T12:10:50Z Y3RZW9JRD4KE9WTRF7TFX4SX73-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=first-headless-run
- 2026-09-03T12:16:09Z CM7YVX33TYA4CDEFZX8W38TEC7-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=first-headless-run
- 2026-09-03T12:20:17Z 30F71GZ7RZ9BP1RJJCEBEW9D3W-m1-7bb1546e set-arc actor=m1+main-1788333680-2840-7f79f4 targets=first-headless-run
- 2026-09-03T13:57:12Z BW9A6KM6NXFJNJTFGVV8T0FA2H-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=first-headless-run
- 2026-09-03T14:16:24Z JY54AQT8101VK9353WT9FQYGA9-m0-c5dbf036 approve actor=human:Wido targets=first-headless-run authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="approved (start work on first-headless-run; relayed from Wido through m0, 2026-09-03)"
- 2026-09-03T14:16:47Z MEE0QGPYWT297RMHS6H60AQ1FB-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=first-headless-run
- 2026-09-03T15:32:24Z 78R0B6QJSS6M464YJAGAYTFJ30-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=first-headless-run
- 2026-09-03T15:40:15Z 5WY2SXR5RVZ3YJ2C7RF92H5RSE-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=first-headless-run
- 2026-09-03T15:46:37Z XVAC123RXPZ7SNXWK254YNDPFK-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=first-headless-run
Integrity: sha256=3d7376c9631aefa361f09332251ea3fafc445d7ef302d591793a953e816b92de
