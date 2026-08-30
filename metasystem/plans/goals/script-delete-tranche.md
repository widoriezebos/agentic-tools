# script-delete-tranche

- State: queued
- Intent: Execute the L15 disposition ledger's DELETE verdicts that carry no pending INTERNALIZE dependency and whose caller sweep stays off m1's active surfaces (Wido's speed-up package via m1, 2026-08-30). Ruling N: clean cuts, no stubs, survivor named per deletion; Ruling R: every deletion sweeps its callers at the gate.
- Origin: main
- Next step: Appetite: 4h. Execute 13 of 18 DELETEs: assert-conformance, check-preamble-quotes, dispatch.sh.orig, dispatch.sh.rej, mission-runner.sh, assert-critique-closed, assert-design-obligation-gate, assert-mission, assert-plan-consistency, assert-stop-loss, assert-turn-prompt, audit-metasystem, frontier. SKIPPED with reasons: battery.sh + milestone-battery.sh (DELETE-pending-implementation, m1's), arm-supervision.sh + metasystem-config.sh (caller sweep would edit dispatch.sh - m1's active surface tonight), sync-transport.sh (caller sweep edits land.sh/commit.sh + IL-30 migration depends on the land internalize). Verify every survivor verb exists before the first cut.
- OpenedAt: 2026-08-30T09:40:49Z
- Revision: 1

History:
- 2026-08-30T09:40:49Z 7WVS0GMX0JPC925P5KXC5NXW64-m2-bc1be9cb open actor=m2+mac-coordinator targets=script-delete-tranche
Integrity: sha256=0c41930d0aa4662f305b971b04d3de13b8e9dc4b16897f4696a2d1fd8beb2baa
