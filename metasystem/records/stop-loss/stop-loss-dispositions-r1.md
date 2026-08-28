# Dispositions — stop-loss-last-defense, critique round 1

Critic: design-critic-20260811t063357z-7912 (codex gpt-5.6-sol, xhigh).
Verdict: 13 material findings. All 13 ACCEPTED. Design amended in the same
commit; anchors below name the amended sections.

| # | Sev | Claim (short) | Disposition |
|---|-----|---------------|-------------|
| 0 | critical | No trusted one-use identity for a closure credit; replay/fresh-chain farming | ACCEPT — D1 now mints runner-proven credits keyed (chain root, round), once each, rounds monotone per chain, chains named by the mission's own reservations, per-stream lifetime credit budget sealed in the contract. |
| 1 | high | Two incompatible counter algorithms | ACCEPT — freeze semantics chosen and stated as the single algorithm; Invariant 1 rewritten to match (count since last improvement reaches budget; loop-advanced freezes, only contract-improved resets). |
| 2 | critical | Lifetime two-no-progress fuse in assert-stop-loss.sh survives and still parks | ACCEPT — that rule is retired; ONE fuse remains (the no-gain counter, which now also counts no-progress cycles). The 2-consecutive host-failure breaker stays: different jurisdiction (host health). D2 rewritten. |
| 3 | medium | Fixed warning threshold detects nothing | ACCEPT — warning is now relative: no-gain-budget < half the cycle fence warns, naming this design. |
| 4 | high | Stop-loss unpark path does not exist; answer-vs-amendment authority undecided | ACCEPT — decided: an answer with the literal prefix `reset:` unparks and resets the runtime counter (the sealed budget line is untouched; wall clock and exposure stay the hard guards); any other answer keeps the amend-reseal-resign guidance. answer.go's refusal is replaced by this rule. |
| 5 | high | Triple record not guaranteeable; no crash-consistent transaction | ACCEPT — the LEDGER append is the single authoritative reset record (flock'd, append-first); unpark happens only after it lands. Event and ask records are best-effort echoes and the design says so. |
| 6 | critical | Post-handshake stamp contradicts the already-issued header | ACCEPT — split identities: turn.json carries announcedSession (what the prompt header said) and observedSession (stamped from the handshake signal when it arrives). Adjudication accepts a return matching either. |
| 7 | high | invalid-run vs improved measurement undefined | ACCEPT — measurement is runner-run and trusted: the cycle classifies from the measured tree (an improvement counts and resets), only the return application is skipped; the ledger entry names the identity fault. |
| 8 | high | Blanket no-debit exemption lets a broken host stall under the fuse | ACCEPT — with announced/observed both available, a return matching neither is a host protocol violation: a failed turn, debited normally, feeding the existing host-failure breaker. The blanket exemption is deleted. |
| 9 | high | reap-facts carries no process-liveness fact | ACCEPT — D5 now names the two halves explicitly: record-side facts from reap-facts, process-side proof from the supervision reaper's kernel custodian discipline (pid at recorded start AND tag in command); the mission lease authorizes, never proves. |
| 10 | critical | Conservative refusal can wedge drainJobs forever inside one cycle | ACCEPT — the drain is now bounded: deadline = latest surviving capDeadline + grace; on expiry with non-terminal unprovable records the runner parks with an ask naming them (reason drain-stalled). No unbounded loop remains. |
| 11 | high | Orphaned returns neither reachable nor provably unadjudicated | ACCEPT — applied rounds are recorded in stream state; anything landed past that is orphaned by definition. Orphan ledger entries name the artifact paths and land in the prior-context ledger tail the next prompt already carries. |
| 12 | high | Usage-after-terminalization has no race-safe owner | ACCEPT — single-writer rule: the adapter owns its round's usage.json (written atomically, best effort on caps from the streamed events); reapers never write usage; the aggregator reads round dirs independently of record status. |
