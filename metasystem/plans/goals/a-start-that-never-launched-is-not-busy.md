# a-start-that-never-launched-is-not-busy

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="A wrong reading could start a second seat beside a live step (severity), bounded by the launch store check; the orphan branch already exists for the same shape (novelty); every steward tick runs it (exposure); it adds no record (accumulation)."
- Tier: 2
- Intent: The steward never mistakes a step that was saved as starting but never launched for a busy seat: a starting step with no launch record and a start older than one tick holds no process, so the busy reader logs it and the seat ladder goes on to start the successor whose brief names the pending step (finding 456: after a budget raise the ui steward ticked for 40 minutes beside a dead seat and a proved, unread unit because an admission refusal had left the read step starting; the health row said dead and starting its successor, and nothing acts on a health row)
- Origin: human
- Next step: One unit on the m1e build lane right after the-steward-sees lands, before the-steward-decides: internal/steward/seat_idle.go:113-115 treats a StepStarting step whose launch record is absent and whose StartedAt is older than one tick like the orphan branch at :126-128 (about 15 production lines plus one test with an artificial clock and an injected store, and the mutation that marks it busy again); brief by the steward, Read each round: yes, lane landing, deploy both seats
- OpenedAt: 2026-10-31T21:23:46Z
- Revision: 2
- Budget: elapsedLimit=5d attemptLimit=24 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=2NHRJDX1XQQDSV9KP2PRCEGPF7-m1e-ce2f570e minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-10-31T21:24:03Z revision=2 opid=2NHRJDX1XQQDSV9KP2PRCEGPF7-m1e-ce2f570e authority=proven digest=7302ca7b4932a6af76ec7ea2e9fa9d68a85eb1f81b99d0609c6718f615c1e003 episode=2

History:
- 2026-10-31T21:23:46Z FZ7BZ1FJ1N1TD8BB222V0KA2VW-m1e-ce2f570e open actor=human:Wido targets=a-start-that-never-launched-is-not-busy
- 2026-10-31T21:24:03Z 2NHRJDX1XQQDSV9KP2PRCEGPF7-m1e-ce2f570e approve actor=human:Wido targets=a-start-that-never-launched-is-not-busy
Integrity: sha256=7a24535021d179b78d71ed5d2df410bf80e698da5c63a670905fc0ebec39ae96
