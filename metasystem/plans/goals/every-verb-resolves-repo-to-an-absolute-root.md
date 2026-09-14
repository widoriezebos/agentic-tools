# every-verb-resolves-repo-to-an-absolute-root

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: false health alarms that send seats chasing phantom corruption; novelty 1: path hygiene; exposure 2: any human or seat running verbs by hand; accumulation 2: each false alarm costs a diagnosis"
- Tier: 2
- Intent: A relative --repo must not change what a verb reports. On 2026-09-14 metasystem health --repo . reported context-budget unknown ('state root must be absolute') and BUDGET_UNKNOWN for three goals with a false 'record identity contradicts its path', while the same call with an absolute path reported alive. DONE: every verb resolves --repo and --root to an absolute, symlink-resolved installation at the CLI boundary before any reader sees it, and a table test runs each verb with a relative path and asserts identical output to the absolute one.
- Origin: human
- Next step: Resolve the root once at the command boundary for all verbs, remove the per-reader absoluteness assumptions, and add the relative-versus-absolute table test.
- OpenedAt: 2026-09-14T16:16:45Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T16:16:45Z 2FC3WFPAYJENAPGDPHC3D3Z11Z-m1e-c6925449 open actor=human:Wido targets=every-verb-resolves-repo-to-an-absolute-root
Integrity: sha256=2c8edc29852c50ad22d70a2d600ea66719c6b7c1aac17241f9592f4fb78bebb2
