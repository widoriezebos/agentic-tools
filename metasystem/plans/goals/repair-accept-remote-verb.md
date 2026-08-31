# repair-accept-remote-verb

- State: queued
- Intent: The goal machinery's own refusal advertises a verb that does not exist: a non-descending canonical tip makes goal fetch say 'repair --accept-remote is the deliberate path', but no CLI verb wires internal/goal.RepairAcceptRemote (implemented, tested, journaling, human-attributed via --by) - cmd/metasystem has no repair verb at all. m0 hit this live executing Wido's divergence-reconciliation order and had to call the library through a temporary in-module main (written, run once, deleted; the repair itself ran with full validation, journaled by=Wido, old tip d1d607b5 new tip 44a3d93e). A machine without that workaround is stranded behind its own error message.
- Origin: main
- Next step: Wire 'goal repair --accept-remote --by <human>' to the existing library function, matching the refusal message's exact grammar; one test proving the verb reaches RepairAcceptRemote and refuses without --by. Under 4h, intuitive-use and robustness gains (R-33)
- OpenedAt: 2026-08-31T19:09:02Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=3 reservedJobMinutesLimit=120 activeJobLimit=1

History:
- 2026-08-31T19:09:02Z N295X7HW27N0H7CHAFYNJB686E-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=repair-accept-remote-verb
- 2026-08-31T19:59:38Z WX366GBZAA9ZKG3QQQCP2Y47HQ-m0-c5dbf036 set-budget actor=human:Wido targets=repair-accept-remote-verb
Integrity: sha256=44b293d9e8472e10b72a69907240a58440cea0dbf1452e05c59af9708b1cfa09
