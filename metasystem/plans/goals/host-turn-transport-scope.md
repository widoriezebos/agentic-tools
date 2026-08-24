# host-turn-transport-scope

- State: queued
- Intent: Extend the ACP transport selector to host turns: hosts/devin.sh still runs legacy dangerous mode; the flip covers dispatch only (bm-2dc evidence, 2026-08-24)
- Origin: main
- Next step: Add the transport selector to scripts/agents/hosts/devin.sh mirroring the dispatch adapter's devin_transport(): conf key resolves acp, host turn rides a graded ACP session; keep legacy for absent-key confs under D61. Acceptance: a bm-2dc-shaped mission's host turn records a transport pin and its session carries v1 grades; the wall stays as backstop. Evidence: docs/design/acp-transport-rationale.md, the first post-flip cohort section.
- OpenedAt: 2026-08-24T11:40:43Z
- Revision: 1

History:
- 2026-08-24T11:40:43Z PGR0AAAVT4C6ZJ0M2CAGJZECKV-m2-bc1be9cb open actor=m2+mac-coordinator targets=host-turn-transport-scope
Integrity: sha256=f115397c0941201aca08e50583e8643f03f45c2374b13b1983a4f2e13240c82f
