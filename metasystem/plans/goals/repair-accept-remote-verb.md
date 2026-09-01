# repair-accept-remote-verb

- State: claimed
- Intent: The goal machinery's own refusal advertises a verb that does not exist: a non-descending canonical tip makes goal fetch say 'repair --accept-remote is the deliberate path', but no CLI verb wires internal/goal.RepairAcceptRemote (implemented, tested, journaling, human-attributed via --by) - cmd/metasystem has no repair verb at all. m0 hit this live executing Wido's divergence-reconciliation order and had to call the library through a temporary in-module main (written, run once, deleted; the repair itself ran with full validation, journaled by=Wido, old tip d1d607b5 new tip 44a3d93e). A machine without that workaround is stranded behind its own error message.
- Origin: main
- Next step: Wire 'goal repair --accept-remote --by <human>' to the existing library function, matching the refusal message's exact grammar; one test proving the verb reaches RepairAcceptRemote and refuses without --by. Under 4h, intuitive-use and robustness gains (R-33)
- OpenedAt: 2026-08-31T19:09:02Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=3 reservedJobMinutesLimit=120 activeJobLimit=1
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-09-01T21:09:46Z revision=3
- StopCapability: generation=3 revision=3 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-31T19:09:02Z N295X7HW27N0H7CHAFYNJB686E-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=repair-accept-remote-verb
- 2026-08-31T19:59:38Z WX366GBZAA9ZKG3QQQCP2Y47HQ-m0-c5dbf036 set-budget actor=human:Wido targets=repair-accept-remote-verb
- 2026-09-01T21:09:46Z 5N4CM848NQ327MCCZJDZS00VAX-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=repair-accept-remote-verb
Integrity: sha256=7a619fca663933e29139bbc01c52b829825b39cebaeddf50689d6d6273969063
