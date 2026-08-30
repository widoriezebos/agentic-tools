# script-delete-tranche

- State: claimed
- Intent: Execute the L15 disposition ledger's DELETE verdicts that carry no pending INTERNALIZE dependency and whose caller sweep stays off m1's active surfaces (Wido's speed-up package via m1, 2026-08-30). Ruling N: clean cuts, no stubs, survivor named per deletion; Ruling R: every deletion sweeps its callers at the gate.
- Origin: main
- Next step: Appetite: 4h. Execute 13 of 18 DELETEs: assert-conformance, check-preamble-quotes, dispatch.sh.orig, dispatch.sh.rej, mission-runner.sh, assert-critique-closed, assert-design-obligation-gate, assert-mission, assert-plan-consistency, assert-stop-loss, assert-turn-prompt, audit-metasystem, frontier. SKIPPED with reasons: battery.sh + milestone-battery.sh (DELETE-pending-implementation, m1's), arm-supervision.sh + metasystem-config.sh (caller sweep would edit dispatch.sh - m1's active surface tonight), sync-transport.sh (caller sweep edits land.sh/commit.sh + IL-30 migration depends on the land internalize). Verify every survivor verb exists before the first cut.
- OpenedAt: 2026-08-30T09:40:49Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=5 reservedJobMinutesLimit=45 activeJobLimit=1
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-30T09:41:21Z revision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-30T09:40:49Z 7WVS0GMX0JPC925P5KXC5NXW64-m2-bc1be9cb open actor=m2+mac-coordinator targets=script-delete-tranche
- 2026-08-30T09:41:05Z A796KNP9SVQW17XSP5D3XJ5QP5-m2-bc1be9cb set-budget actor=human:wido targets=script-delete-tranche
- 2026-08-30T09:41:21Z NFCPFHRK8S835EHT28T1BPG5VP-m2-bc1be9cb claim actor=m2+mac-coordinator targets=script-delete-tranche
Integrity: sha256=7197646fd9fb4658a8a87134059b7ab5a9be67277900e4206a06a454117c7252
