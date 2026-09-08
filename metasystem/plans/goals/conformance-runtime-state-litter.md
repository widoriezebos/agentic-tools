# conformance-runtime-state-litter

- State: queued
- Priority: 3
- Sequence: 56
- Intent: Conformance's control-plane check refuses ANY delegate-created file under artifacts/agents/, including GITIGNORED runtime state a fix legitimately writes there. Found repeatedly on m0's idle-with-backlog-alarm rounds (2026-09-02): the new turn-verdict persists artifacts/agents/turn-verdict-state.json (and goal.lock appears) at runtime; the delegate's test/binary run leaves them in the worktree; conformance then fails 'agent control plane contains delegate-created files' though both are gitignored. The orchestrator hand-removed them each round to re-certify - a recurring tax, and a latent trap for any future control-plane runtime state (the steward already writes there under artifacts/agents/steward/). DONE: conformance's control-plane tamper check EXEMPTS gitignored runtime state (or a declared runtime-state allowlist), so a legitimate gitignored write does not fail the gate, while a delegate sneaking TRACKED code into the control plane still refuses.
- Origin: main
- Next step: INTENT: the tamper check catches smuggled code, not benign gitignored runtime state. CONSTRAINTS: keep the real protection (a delegate must not commit code into artifacts/agents); exempt only files git would ignore, or a declared runtime-state path list; a fixture proves a gitignored runtime write passes and a tracked code file refuses. FREEDOMS: exempt-by-gitignore vs an explicit runtime-state allowlist. Budget Wido's word at claim. Small, R-33 robustness. SPECIMEN 2026-09-04 01:4x (m2): chain str-build1c round 4; the implementer ran dispatch-fixtures.sh inside its worktree, the run failed on a pre-existing break and wrote a suite-failure snapshot under the worktree's artifacts/agents/suite-failures (hundreds of ignored files) plus an empty artifacts/agents/mains; conformance --stage review refused 'agent control plane contains delegate-created files' until the orchestrator removed the ignored litter by hand. The tamper check should distinguish fixture litter (ignored runtime paths the delegate's own gate wrote) from planted control-plane state, or the fixtures should write their failure snapshots outside the worktree.
- OpenedAt: 2026-09-02T17:25:02Z
- Revision: 3
- BudgetExceptions: 0

History:
- 2026-09-02T17:25:02Z WGH85D1WKWK7M8KMD662DRN99V-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=conformance-runtime-state-litter
- 2026-09-03T23:36:35Z RRA61HVV1ZJ2V3HPH8SEEE9454-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=conformance-runtime-state-litter
- 2026-09-08T16:02:24Z WF2CN1VSTRJ12Z822HTFTKWJ6J-m1-7cd0bd60 set-priority actor=human:Wido targets=conformance-runtime-state-litter reason=priority-order subject=conformance-runtime-state-litter from=unranked to=3:56 requested-sequence=56
Integrity: sha256=137d5f409a9a8d3c5d12e221ca18090de69cd2ea1df65a54f69462df9c40d374
