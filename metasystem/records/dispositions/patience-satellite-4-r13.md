# Dispositions: patience-satellite-4, round 13

Job: design-critic-20260811t204056z-736e (codex gpt-5.6-sol, xhigh).
2 findings, 2 material, both accepted. Convergence: 4 → 3 → 2.

| id | disposition |
| --- | --- |
| P4-076 | accepted — the started predicate for cancelled/failed stops citing a nonexistent handshake-success key and stops trusting effectiveModel alone (HandshakeEval patches it before deciding failure). Started(cancelled|failed) ⇔ (recorded usage ∨ non-empty effectiveModel) ∧ error ∉ the never-started vocabulary {abandoned-setup, handshake_timeout, launch_failed, the handshake-mismatch error}. That vocabulary is a single table in patience.go, documented against dispatch's error writers and enumerated by a verification test, so drift between the writers and the table fails loudly. Completed/timeout stay structural. |
| P4-077 | accepted — the crash contract's reliance on "a parked mission's ask" was unbacked for the all-streams-inactive park, which creates no ask. In a patience-configured mission, that park path now guarantees at least one open ask exists (creating the standard park ask if none), so a final booking's Patience lines always have a vocal successor. Unconfigured missions keep today's behavior byte-identically. |
