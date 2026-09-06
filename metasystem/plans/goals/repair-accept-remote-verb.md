# repair-accept-remote-verb

- State: approved
- Intent: The goal machinery's own refusal advertises a verb that does not exist: a non-descending canonical tip makes goal fetch say 'repair --accept-remote is the deliberate path', but no CLI verb wires internal/goal.RepairAcceptRemote (implemented, tested, journaling, human-attributed via --by) - cmd/metasystem has no repair verb at all. m0 hit this live executing Wido's divergence-reconciliation order and had to call the library through a temporary in-module main (written, run once, deleted; the repair itself ran with full validation, journaled by=Wido, old tip d1d607b5 new tip 44a3d93e). A machine without that workaround is stranded behind its own error message.
- Origin: main
- Next step: DONE, landed by m0 (account Wido@M0): the verb exists matching the advertised grammar, refuses without --by, test-pinned.
- OpenedAt: 2026-08-31T19:09:02Z
- Revision: 7
- Budget: elapsedLimit=1d attemptLimit=3 reservedJobMinutesLimit=120 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=7 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=35c3c455257dbbe3015e84081701bca144de227bf0ef834e40c46d66c8152145
- Sliced: machine=m0 lineage=main-1788178136-1684505-4ffe42 revision=3 at=2026-09-01T21:10:13Z

History:
- 2026-08-31T19:09:02Z N295X7HW27N0H7CHAFYNJB686E-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=repair-accept-remote-verb
- 2026-08-31T19:59:38Z WX366GBZAA9ZKG3QQQCP2Y47HQ-m0-c5dbf036 set-budget actor=human:Wido targets=repair-accept-remote-verb
- 2026-09-01T21:09:46Z 5N4CM848NQ327MCCZJDZS00VAX-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=repair-accept-remote-verb
- 2026-09-01T21:10:13Z NRQR31QAWRH13E5WK27XS5QBB2-m0-c5dbf036 slice-start actor=m0+main-1788178136-1684505-4ffe42 targets=repair-accept-remote-verb
- 2026-09-01T21:15:39Z G72ZKDXRWTQQDT6H6ZHJY8YMZY-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=repair-accept-remote-verb
- 2026-09-01T21:15:42Z K6MV6B6Z0ESXAQ9Q6EYKQZVXXR-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=repair-accept-remote-verb
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=repair-accept-remote-verb reason=sweep
Integrity: sha256=329d6ad077510367709b3e557563263ad35ed3f0f3232a93873383520e4886c2
