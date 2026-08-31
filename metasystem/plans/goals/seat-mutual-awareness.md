# seat-mutual-awareness

- State: queued
- Intent: Wido's order 2026-08-31: seats must be aware of each other and ask each other questions directly, without the human as relay - the m3-to-m2 seam check of this day routed through Wido when it should have been seat-to-seat by default; DONE means a seat can discover what other seats have in flight and put a question to them as the normal, mechanized path
- Origin: main
- Next step: Appetite: 3h design. INTENT: mechanize inter-seat awareness and direct questioning, in both directions, FOR ANY AGENT RUNTIME (Wido's word 2026-08-31: the Claude session bridge is a Claude-specific mechanism - the design must work for any agent; claude, codex, devin, future runtimes alike). CONSTRAINTS: four proven gaps - TRANSPORT: no runtime-agnostic channel exists; the Claude bridge worked twice on 2026-08-31 (m2 load-hold, m3 seam check) but only Claude seats speak it, its cross-machine reach depends on a transient Remote Control link, and cloud sessions cannot answer - the runtime-neutral durable medium is the repository itself (the everything-is-a-file law; the ledger is already the recorded inter-seat channel), with the bridge at most a latency optimization; DISCOVERY: the ledger publishes claims, not in-flight seams, and the seat roster is nowhere mechanical (m0 exists by Wido's word yet is invisible to m3 - seat identity must be assertable by the mechanism, never inferred from display names); RESPONSE PROTOCOL: a receiving seat learns the norm only from prose inside each message - nothing binds it to answer, says what an answer commits, or that it answers from its own ledger without routing through Wido; NORM: when a seat must ask before claiming or running heavy. Compose with machine-concurrency-governor (m2's, machine load) rather than duplicating it; R-1 conflict test before any new role; R-12 composes - the human is involved only for worth, scope, priority, or budget. FREEDOMS: the concrete mechanism (seam-declaration register, question-and-answer files in the repository, pre-claim check protocol, polling cadence, bridge as optional fast path) and where the roster lives (config, ledger, or a new surface)
- OpenedAt: 2026-08-31T14:24:17Z
- Revision: 4

History:
- 2026-08-31T14:24:17Z PQVSVQQVASG56RB6DNG57JA3W9-m3-a5da21ff open actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T14:24:52Z 6XMZ029FWWWM1W7F3RQDP3KQ62-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T14:30:13Z QSR12G5EKAA1DEPBYVZXJN7E08-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T14:32:23Z AZZJY85CH39SZERMYHTZYKA2CE-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
Integrity: sha256=76f29097af2529580bcb3b4b71401efe12c3cc0d94e8b425058937ed22444d44
