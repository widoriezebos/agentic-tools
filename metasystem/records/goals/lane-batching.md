# lane-batching

- State: done
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Finding 150 of 2026-10-11: two goals handed in 20 s apart cost two full proofs (45 min each) because the batch sealed on the first hand-in; batches hold one member by policy; a restart has no clean cancel (a killed child reads as a lost-process red); every proof reruns every package"
- Tier: 3
- Intent: The landing lane batches what arrives together and restarts an early proof for a newcomer, with the person's verbs for every situation
- Origin: main
- Next step: Design plans/designs/lane-batching.md (Astra critique in parallel), then four Codex units: multi-member admission window, restart rule with a clean cancel, green-package reuse, verbs and status
- Concluded: all five units on main: U1-U3 and U5 in hot-fix 4 (9f869cbcc), U4 in hot-fix 3; D5 parked by design
- OpenedAt: 2026-10-11T20:42:19Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-11T20:42:30Z revision=2 opid=2Q3CMAWS3W4WXENVVBTED5XVH8-m1e-9c08b0e5 authority=proven digest=44bb21fcfef895d9782b097adc9af1a24909cbf1b247250e578f0277ec003a86 episode=2

History:
- 2026-10-11T20:42:19Z 2R9NFWD36J7D0D6AZ49C4YN10V-m1e-9c08b0e5 open actor=human:Wido targets=lane-batching
- 2026-10-11T20:42:30Z 2Q3CMAWS3W4WXENVVBTED5XVH8-m1e-9c08b0e5 approve actor=human:Wido targets=lane-batching
- 2026-10-12T08:39:45Z 8H8VN6H3YN4BC4RG5VGP7GX5TS-m1e-c6925449 done actor=m1e+main-1788680071-18713-e76d5d targets=lane-batching
Integrity: sha256=85fddd925e4b2f5d7aed347396a403f7d46fae8523981036c28e6ba708f328de
