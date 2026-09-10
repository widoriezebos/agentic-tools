# up-is-the-human-start-command

- State: queued
- Risk: severity=3 novelty=1 exposure=3 accumulation=2 basis="Startup includes human authorization and process ownership, so incorrect routing can grant authority or disturb live supervision; reuse the existing owners, cover every supported runtime, and prevent recurring failed startup and recovery attempts."
- Tier: 3
- Intent: A human runs metasystem up from the repository root to authorize and bring up the MetaSystem, including refreshing an enrolled rebuilt engine, with automatic context detection and no process-identity flags; the current agent-only startup operation has a distinct name and purpose.
- Origin: human
- Next step: INTENT: focused, independently deliverable startup-command item linked to verbs-match-intent. Wido requested on 2026-09-10 that up do what arm currently does and that confusing operations be renamed. DONE: plain up works from an ordinary human terminal under the existing human-authority rules; it completes enrollment and supervision startup or refresh, reports a truthful aggregate result, and does not require a second command from an agent. The agent-only session announcement and lease operation is clearly named or internalized, and runtime hooks/adapters are updated for Claude, Codex and Devin. A live owner from an older engine is lawfully replaced when required; readiness waits report progress and actionable causes rather than a silent wait followed by the same ineffective retry. Help, documentation and refusal commands match the caller context, with an explicit compatibility or deprecation policy for old names. CONSTRAINTS: preserve human authorization, exact process ownership, checkout isolation, delegate custody and lease protection; no impersonating an agent from a human terminal, no broad CLI redesign. Verify with focused end-to-end canaries for a human terminal, an agent session, already-running services and a rebuilt-engine refresh. FREEDOMS: choose the distinct name and reuse existing owners; do not weaken authority to simplify names. EVIDENCE: m1c engine d1a47c35; human arm enrolled generation 34 but returned recovery-partial because recovery-only up retained supervision owner 2388 on old engine 225f7c9d. Human up then refused no immediate agent-signature ancestor and suggested pid/start-time flags. Ordinary agent up replaced the owner, reached generation 37 and verified armed. Preserved before/after evidence: /Users/wido/metasystem-evidence/agentic-tools/arm-recovery-20260910.Rr77Qv. Existing owners: cmd/metasystem/process_verbs.go runProcessArm, internal/up/up.go and internal/supervise/arming.go OnlyIfDown branch. Backlog intake only; implementation has not started.
- OpenedAt: 2026-09-10T06:03:24Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T06:03:24Z GFGTDQ5RH0TX7NY6KH8K03NBR9-m1c-1274caf4 open actor=m1c+main-1788759014-39092-24fa5c targets=up-is-the-human-start-command
Integrity: sha256=4d33cba2a1e5a04318b72d11c1124fad68d83ca8c7a57f0c3afd9c2b2cdbbfbe
