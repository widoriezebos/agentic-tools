# conformance-runtime-state-litter

- State: queued
- Intent: Conformance's control-plane check refuses ANY delegate-created file under artifacts/agents/, including GITIGNORED runtime state a fix legitimately writes there. Found repeatedly on m0's idle-with-backlog-alarm rounds (2026-09-02): the new turn-verdict persists artifacts/agents/turn-verdict-state.json (and goal.lock appears) at runtime; the delegate's test/binary run leaves them in the worktree; conformance then fails 'agent control plane contains delegate-created files' though both are gitignored. The orchestrator hand-removed them each round to re-certify - a recurring tax, and a latent trap for any future control-plane runtime state (the steward already writes there under artifacts/agents/steward/). DONE: conformance's control-plane tamper check EXEMPTS gitignored runtime state (or a declared runtime-state allowlist), so a legitimate gitignored write does not fail the gate, while a delegate sneaking TRACKED code into the control plane still refuses.
- Origin: main
- Next step: INTENT: the tamper check catches smuggled code, not benign gitignored runtime state. CONSTRAINTS: keep the real protection (a delegate must not commit code into artifacts/agents); exempt only files git would ignore, or a declared runtime-state path list; a fixture proves a gitignored runtime write passes and a tracked code file refuses. FREEDOMS: exempt-by-gitignore vs an explicit runtime-state allowlist. Budget Wido's word at claim. Small, R-33 robustness.
- OpenedAt: 2026-09-02T17:25:02Z
- Revision: 1

History:
- 2026-09-02T17:25:02Z WGH85D1WKWK7M8KMD662DRN99V-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=conformance-runtime-state-litter
Integrity: sha256=4b0a3e9e187c6cc108eacacb8946e23599a30a29e33b1b99ec1c33b945c1c932
