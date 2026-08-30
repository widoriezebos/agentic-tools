# host-turn-transport-scope

- State: claimed
- Intent: Extend the ACP transport selector to host turns: hosts/devin.sh still runs legacy dangerous mode; the flip covers dispatch only (bm-2dc evidence, 2026-08-24)
- Origin: main
- Next step: Add the transport selector to scripts/agents/hosts/devin.sh mirroring the dispatch adapter's devin_transport(): conf key resolves acp, host turn rides a graded ACP session; keep legacy for absent-key confs under D61. Acceptance: a bm-2dc-shaped mission's host turn records a transport pin and its session carries v1 grades; the wall stays as backstop. Evidence: docs/design/acp-transport-rationale.md, the first post-flip cohort section.
- OpenedAt: 2026-08-24T11:40:43Z
- Revision: 3
- Budget: elapsedLimit=2h attemptLimit=4 reservedJobMinutesLimit=30 activeJobLimit=1
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-30T00:03:28Z revision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-24T11:40:43Z PGR0AAAVT4C6ZJ0M2CAGJZECKV-m2-bc1be9cb open actor=m2+mac-coordinator targets=host-turn-transport-scope
- 2026-08-30T00:03:14Z Y154ZPVJ8861N3T4X2BWGQBJ1D-m2-bc1be9cb set-budget actor=human:wido targets=host-turn-transport-scope
- 2026-08-30T00:03:28Z 4QM9TM55DRACRNNH9TZZV205S5-m2-bc1be9cb claim actor=m2+mac-coordinator targets=host-turn-transport-scope
Integrity: sha256=d31aa6539cc302e538225b78358c983713dede86531a18396867246d6054adb3
