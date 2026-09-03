# first-headless-run

- State: queued
- Intent: Run the metasystem headless for real: on m0, with no Claude session on top, the mission runner (family metasystem mission, cmd/metasystem/mission.go, so far exercised only through its fake host in scripts/agents/dispatch-fixtures.sh) picks one approved tier-2 goal, composes its brief, dispatches the build and the review, folds or defers findings, lands the result and talks to Wido through the fleet conversation channel where a severe finding needs his word; every defect the run surfaces is fixed forward as its own tier-1 or tier-2 item the same day; the run is recorded, and its landing on its own is the proof that the machines can be headless.
- Origin: main
- Next step: BLOCKED BY THE FEATURE m0 JUST LANDED (2026-09-03, working as designed): claim refuses APPROVAL_REQUIRED - first-headless-run is queued, not approved. R-66-m1 is Wido's word approving it, but under human-approval-for-execution (landed c285d5a0) the mechanical approve act is human-only and carries the budget. m0 will NOT self-approve its own next goal via the relayed --temporary-human-word form - that is exactly the self-authorization the feature prevents, and self-approving one's own execution is the forge hazard. NEEDED FROM Wido/m1: run 'goal approve --id first-headless-run --by Wido --budget <box|tuple>' (or the relayed form with review-by) to set the approved state AND a budget - R-66-m1 gave no budget tuple, so name one (tier 3). The moment it is approved with a budget, m0 claims it and starts the headless run, posting to the ledger. --- [R-66-m1 recipe below stands: find the runner entry point, pick one approved tier-2 goal, stop own session, start the runner from the shell with no session on top, fix forward, record the run.]
- OpenedAt: 2026-09-03T12:10:50Z
- Revision: 4
- Arc: headless-fleet

History:
- 2026-09-03T12:10:50Z Y3RZW9JRD4KE9WTRF7TFX4SX73-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=first-headless-run
- 2026-09-03T12:16:09Z CM7YVX33TYA4CDEFZX8W38TEC7-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=first-headless-run
- 2026-09-03T12:20:17Z 30F71GZ7RZ9BP1RJJCEBEW9D3W-m1-7bb1546e set-arc actor=m1+main-1788333680-2840-7f79f4 targets=first-headless-run
- 2026-09-03T13:57:12Z BW9A6KM6NXFJNJTFGVV8T0FA2H-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=first-headless-run
Integrity: sha256=2360efd071672272b10b91150c0878682f87c071fb263805a33e1579b77e5b53
