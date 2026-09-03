# first-headless-run

- State: approved
- Intent: Run the metasystem headless for real: on m0, with no Claude session on top, the mission runner (family metasystem mission, cmd/metasystem/mission.go, so far exercised only through its fake host in scripts/agents/dispatch-fixtures.sh) picks one approved tier-2 goal, composes its brief, dispatches the build and the review, folds or defers findings, lands the result and talks to Wido through the fleet conversation channel where a severe finding needs his word; every defect the run surfaces is fixed forward as its own tier-1 or tier-2 item the same day; the run is recorded, and its landing on its own is the proof that the machines can be headless.
- Origin: main
- Next step: RUN IN PROGRESS (m0, 2026-09-03). FINDINGS: (1) state-init-born missions launch with RESUME not start; (2) an engine rebuild after the last steward arm makes the runner's 'up' refuse ENROLLMENT_DRIFT - re-arm before launch; (3) the runner's 'up' arms supervision as itself (owner pid 1206366) but returns advisor/read-only because the CHECKOUT LEASE is held by m0's live Claude session (main-1788344772-1154480) - 'stop your own session' is mechanical: the runner cannot hold writing authority until the seat's session lease is retired. Fix-forward: retire the seat session's lease (up --retire), then relaunch resume; the seat keeps read-only watching, exactly the recipe's 'watch from outside'.
- OpenedAt: 2026-09-03T12:10:50Z
- Revision: 12
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
- 2026-09-03T15:47:33Z 2D1CAHMMPR05R16ZJTSPJ3D5HT-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=first-headless-run
- 2026-09-03T15:48:19Z G3AQ46MRPXCERMN99SWBFPK3D9-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=first-headless-run
- 2026-09-03T15:49:13Z C8PEDYAERS0GRYJPQEGV922TRC-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=first-headless-run
Integrity: sha256=f405aa8e1299ca0a2435efa3c69404009b3c50560b435c18617eeb76257e9170
