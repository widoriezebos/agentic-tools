# Design brief: lane-first-run-fixes (follow-up of landing-takes-an-hour, from the first lane run 2026-10-09)

Wido's words: "learn from what happens when we try to land these with the new landing lane and evolve the landing lane to be perfect before we switch it on for real"; "DO NOT BUILD A NUCLEAR POWER PLANT. Build the smallest thing that works!"

Evidence: agentic-tools-evidence/lane-test-20261009/findings.md (numbered), the trunk proof log 20261009T145725, plans/machinery-open-findings-plan-2026-10-06.md "Measured 2026-10-09 17:58".

Units (each at most 250 production lines, smallest change):
1. Proof partitioning (finding 16): the repoproof gate shards only the big package (cmd/metasystem) by test name and runs every other package once, in parallel bounded by host.builds; measured target: a trunk proof of main under 25 minutes on this host (was 61). Keep "a panic loses one shard".
2. Status tells the truth about the trunk (finding 13): after a red trunk proof `landing status` line 1 names the red, the incident, and the next act (hot-fix, then landing prove --trunk); while proving it shows shards done/total and elapsed.
3. Hand-in remedies that succeed (findings 1, 4): a goal branch absent on origin -> the remedy is the push (or work land pushes it); a merge commit on the branch -> the remedy names work rebase.
4. Deploy without a wall-clock cap (finding 2): the steward's start waits on an observed event, not 10 seconds.
5. Proof environment hygiene on the lane's side (finding 15b, if main's testenv fix is not enough): the lane passes its LANDING_* context to the reporter explicitly, not through the inherited environment of every test.

Out of scope (own goal): the seat-side recovery verbs (findings 6-10). Acceptance: one goal lands through the lane end to end with a proof under 25 minutes and a status a person can act on; the first-run known limits on the design pages are re-checked against what happened.
