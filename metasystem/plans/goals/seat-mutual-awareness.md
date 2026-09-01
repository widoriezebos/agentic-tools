# seat-mutual-awareness

- State: queued
- Intent: Wido's order 2026-08-31: seats must be aware of each other and ask each other questions directly, without the human as relay - the m3-to-m2 seam check of this day routed through Wido when it should have been seat-to-seat by default; DONE means a seat can discover what other seats have in flight and put a question to them as the normal, mechanized path
- Origin: main
- Next step: WIDO'S BINDING DESIGN WORD for the inbound/receive loop (2026-09-01, verbatim: 'I am thinking that anything inbound needs to be accompanied with an authentication token from a Google Authenticator or similar'): every inbound message on the external channel carries a TOTP code verified against a shared secret provisioned ONCE at Wido's agent-free terminal (the enrollment-law anchor). Design rules recorded with the word: codes are single-use (last accepted time-step remembered, reuse refused); a code authorizes only the message it accompanies, recorded together verbatim; tiering survives — code-verified inbound may carry authority words (budgets, resumes) but terminal-reserved acts (enrollment, provisioning this secret) stay at the terminal. Residuals accepted with eyes open: same-device collapse if the authenticator lives on the phone that runs Telegram (separate devices recommended); the machine-side secret is readable by whatever owns the VM (the token protects the channel, not the machine). The alert channel's slice-1 send path is unaffected
- OpenedAt: 2026-08-31T14:24:17Z
- Revision: 7
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-31T14:24:17Z PQVSVQQVASG56RB6DNG57JA3W9-m3-a5da21ff open actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T14:24:52Z 6XMZ029FWWWM1W7F3RQDP3KQ62-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T14:30:13Z QSR12G5EKAA1DEPBYVZXJN7E08-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T14:32:23Z AZZJY85CH39SZERMYHTZYKA2CE-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-08-31T17:24:03Z 809B7APA7QNP2S3843JEJ48T3H-m3-a5da21ff edit actor=m3+mac-m3 targets=seat-mutual-awareness
- 2026-09-01T14:41:43Z D4SYHJ782EQHP7BJ6EZ4MA0GB2-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=seat-mutual-awareness
- 2026-09-01T20:29:37Z A0JV3E94GEEWZ7HSSVQ27P9SXC-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=seat-mutual-awareness
Integrity: sha256=2ffdc73b17236f894293d187bc9a682591ec933bbe375e08c12bb39f74a9d4be
