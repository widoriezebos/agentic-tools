# seat-mutual-awareness

- State: queued
- Intent: Wido's order 2026-08-31: seats must be aware of each other and ask each other questions directly, without the human as relay - the m3-to-m2 seam check of this day routed through Wido when it should have been seat-to-seat by default; DONE means a seat can discover what other seats have in flight and put a question to them as the normal, mechanized path
- Origin: main
- Next step: Appetite: 3h design. INTENT: mechanize inter-seat awareness and direct questioning, in both directions, FOR ANY AGENT RUNTIME (Wido's word 2026-08-31: the Claude session bridge is Claude-specific - the design must work for any agent). CONSTRAINTS: five proven gaps, all from 2026-08-31 - TRANSPORT: no runtime-agnostic channel; the repository is the runtime-neutral durable medium (everything-is-a-file law), the bridge at most a fast path; DISCOVERY: seams and the seat roster are nowhere mechanical (m0 invisible to m3; seat identity must be assertable, never inferred from display names); RESPONSE PROTOCOL: nothing binds a receiving seat to answer or says what an answer commits; WAITING DISCIPLINE: a seat that asks waits with an explicit deadline and an armed fallback, acting on silence - m3 idled hours on m2's unanswered seam check with no timeout, caught only by Wido looking; NORM: when to ask before claiming or running heavy. A seat blocked on the HUMAN is out of scope here - external delivery is alert-escalation-channel's seam (compose, do not duplicate); same for machine-concurrency-governor (m2's, machine load). R-1 conflict test before any new role; R-12 composes. FREEDOMS: mechanism (seam register, question-answer files in the repository, pre-claim protocol, polling cadence, bridge as optional fast path), roster home (config, ledger, new surface), and deadline defaults
- OpenedAt: 2026-08-31T14:24:17Z
- Revision: 5

History:
- 2026-08-31T14:24:17Z PQVSVQQVASG56RB6DNG57JA3W9-m3-a5da21ff open actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T14:24:52Z 6XMZ029FWWWM1W7F3RQDP3KQ62-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T14:30:13Z QSR12G5EKAA1DEPBYVZXJN7E08-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T14:32:23Z AZZJY85CH39SZERMYHTZYKA2CE-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T17:24:03Z 809B7APA7QNP2S3843JEJ48T3H-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
Integrity: sha256=2be1754d2f58f9db4dfbb64bae3c5bfa4d4dc48af5b4b1083ab335ceeeb08722
