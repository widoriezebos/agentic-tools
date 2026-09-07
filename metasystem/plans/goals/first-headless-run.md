# first-headless-run

- State: approved
- Intent: Run the metasystem headless for real: on m0, with no Claude session on top, the mission runner (family metasystem mission, cmd/metasystem/mission.go, so far exercised only through its fake host in scripts/agents/dispatch-fixtures.sh) picks one approved tier-2 goal, composes its brief, dispatches the build and the review, folds or defers findings, lands the result and talks to Wido through the fleet conversation channel where a severe finding needs his word; every defect the run surfaces is fixed forward as its own tier-1 or tier-2 item the same day; the run is recorded, and its landing on its own is the proof that the machines can be headless.
- Origin: main
- Next step: INTENT: obtain the first limited real headless feature-delivery receipt from the existing mission runner on the pinned machine, without a supervising chat. SCOPE: this proves one feature only; it cannot conclude sustained headless-fleet readiness. CONSTRAINTS: preserve the approved goal budget and machine pin, use a supported host whose current health/launch preflight passes, and retain the exact goal approval, contract, birth baseline, claim, job/review and landing records. The September 6 aborted/hung attempt is retained in history; its night-specific manual command sequence is evidence to recheck, never a standing startup procedure. host-runtime-setup, metasystem-stop-verb and mission-birth-baseline-from-dirty-worktree own the corresponding machinery; do not rebuild them or steal their claims. Use their current supported verbs and explicit refusals, never terminal tricks. This diagnostic may proceed before every fleet reliability item finishes; each exposed defect has a separate owner and is not silently waived. Consecutive delivery and omitted-stage negative proofs belong to headless-continuous-delivery-proof; multi-node and brain proof belong to headless-fleet-coordination-proof.
- OpenedAt: 2026-09-03T12:10:50Z
- Revision: 21
- Labels: headless-fleet, headless-proof
- Arc: headless-fleet
- Pinned: m1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T15:46:29Z revision=17 opid=WFVEHJPWDM290E8WVX9EDB8B1S-m1-7cd0bd60 authority=proven digest=f3221db8cdf77daede3db76fbf4c861511fd0b0b0724f96c140d89180762c395

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
- 2026-09-03T15:50:30Z P82T4H9HBFQ4N7XBPM1J1KTK8Q-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=first-headless-run
- 2026-09-03T15:59:24Z QYRJFVYYFJKBE6VQ5DHXSJ7C9D-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=first-headless-run
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=first-headless-run reason=sweep
- 2026-09-06T15:43:04Z BG43FSRNGHE2E5BTRJNC90G06X-m1-7cd0bd60 set-pin actor=human:Wido targets=first-headless-run
- 2026-09-06T15:46:29Z WFVEHJPWDM290E8WVX9EDB8B1S-m1-7cd0bd60 approve actor=human:Wido targets=first-headless-run
- 2026-09-06T20:30:04Z A80N0DGAEMXRYKFFZ1CKTV433V-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=first-headless-run
- 2026-09-06T20:30:52Z P4TDTQG5PPVHYXT4M1KQ15RZNX-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=first-headless-run
- 2026-09-06T21:03:06Z Y40DTHW2ENS7D9T1SJA6AVA1BE-m1-a4f8999f edit actor=m1+main-1788594343-3833-fb64b9 targets=first-headless-run
- 2026-09-07T21:12:01Z RPKM7WBBCDYH5MC47RN3ZCNKX0-m1-76f67331 edit actor=human:Wido targets=first-headless-run
Integrity: sha256=95311caaafef43805ce1a643404a23c293237658032cae0ecd570ca27b8acccb
