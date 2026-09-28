# Dispositions: patience-satellite-4, round 12

Job: design-critic-20260811t203156z-6ca7 (codex gpt-5.6-sol, xhigh).
3 findings, 3 material, all accepted. Convergence: 3 findings, each
narrower than the last round's.

| id | disposition |
| --- | --- |
| P4-073 | accepted — sessionEstablishedSignal leaves the started predicate: it is a pre-launch capability promise, not a handshake fact, and normally true for Codex and Claude. Started for cancelled/failed = recorded handshake SUCCESS (the handshake object's completion fact, field named at implementation against the shipped struct) ∨ non-empty effectiveModel ∨ recorded usage. Completed/timeout stay structural. |
| P4-074 | accepted — row 3 triggers when no streak job QUALIFIES as an effective-model evidence record (the r8/P4-054 absence rule), not when no effective-model evidence exists anywhere; a broken record with real model evidence falls through without blocking the requested-model row. |
| P4-075 | accepted — the excluded count is scoped to what is honestly countable: mission-owned READABLE records that missionJobs yields and the participation boundary rejects (identity mismatch, unknown status). Fully unreadable or unattributable records cannot be charged to any mission — machine-global damage, the janitor's jurisdiction when it is wired — and the design now says exactly that instead of promising a count it cannot compute. The excluded line gains its verification case. |
