# host-turn-transport-scope

- State: done
- Intent: Extend the ACP transport selector to host turns: hosts/devin.sh still runs legacy dangerous mode; the flip covers dispatch only (bm-2dc evidence, 2026-08-24)
- Origin: main
- Next step: Add the transport selector to scripts/agents/hosts/devin.sh mirroring the dispatch adapter's devin_transport(): conf key resolves acp, host turn rides a graded ACP session; keep legacy for absent-key confs under D61. Acceptance: a bm-2dc-shaped mission's host turn records a transport pin and its session carries v1 grades; the wall stays as backstop. Evidence: docs/design/acp-transport-rationale.md, the first post-flip cohort section.
- Concluded: Landed d22cc95: hosts/devin.sh gains the transport selector on dispatch.transport.devin (absent=legacy per D61, invalid/unreadable refuses closed) and an ACP branch riding acp turn with the session mode graded from the host envelope's tools (runtime-default -> accept-edits, replacing the dangerous waiver on flipped confs). Result envelope carries the transport pin via host finish across all outcomes; legacy envelope shape unchanged. Fixture-proven end to end in acp-fixtures (ACP-H-001 happy wire with pin/session/set_mode/return, ACP-H-002 invalid-value refusal). REMAINING OBSERVATION for the record: the acceptance's live evidence - a bm-2dc-shaped mission's host turn under the flipped conf - lands with the first post-flip cohort; the machinery and refusal fences are in.
- OpenedAt: 2026-08-24T11:40:43Z
- Revision: 4
- Budget: elapsedLimit=2h attemptLimit=4 reservedJobMinutesLimit=30 activeJobLimit=1

History:
- 2026-08-24T11:40:43Z PGR0AAAVT4C6ZJ0M2CAGJZECKV-m2-bc1be9cb open actor=m2+mac-coordinator targets=host-turn-transport-scope
- 2026-08-30T00:03:14Z Y154ZPVJ8861N3T4X2BWGQBJ1D-m2-bc1be9cb set-budget actor=human:wido targets=host-turn-transport-scope
- 2026-08-30T00:03:28Z 4QM9TM55DRACRNNH9TZZV205S5-m2-bc1be9cb claim actor=m2+mac-coordinator targets=host-turn-transport-scope
- 2026-08-30T00:10:59Z JZ4DMCN939W8C2Y8YM51CMRZKT-m2-bc1be9cb done actor=human:wido targets=host-turn-transport-scope
Integrity: sha256=718e18b7b515bfcb4361487910f9117d45e3d629f3dc7953c1a7c1692f9fc821
