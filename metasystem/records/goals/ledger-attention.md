# ledger-attention

- State: done
- Intent: A machine notices when the shared ledger changes under it - the steward tick fetches and surfaces new claimable goals, pins addressed to this machine, and queue reorderings, so no human relays routine nudges between machines (Wido 2026-08-30: 'should we build a mechanism for you to be able to do that yourself?'). ABSORBS fleet-pull (parked, R-33 merge): the same tick lets an idle machine pick up claimable shared-backlog work by itself - fleet liveness is the steward's duty, never a human's memory. Design landed; released for implementation
- Origin: human
- Next step: DESIGN LANDED 5ed2a823 (plans/ledger-attention-design.md): Fable-authored, Sol-verified across three rounds (10->7->7 material), the two requirement-substitution findings CLOSED - the design serves the goal text as written (queue reorderings surfaced, nudge per change). Seven technical findings remain as SS10 residue, each binding on implementation (closed or refuted with evidence; LA-R1-006/LA-R3-001/LA-R3-002 mark parts of SS7 unimplementable as written and take precedence over contradicted plan text). BLOCKED ON BUDGET: 8 jobs against attemptLimit 6, reserved minutes at/past 960 - the implementation slice (Sol lane, DESIGN-BEARING, ~2h, plus Fable code-critique) cannot dispatch until the human raises (~+2 attempts / +300 job-minutes). Three revision runs died on the native dollar cap after completing their product - recorded as goal:budget-death-on-return. Claim released 2026-08-31 so m2 can work goal-scope-bounds serially per Wido's idle-capacity assignment.
- Concluded: Landed 52845ae3: each steward tick performs a bounded isolated fetch of the shared ledger and surfaces every new claimable goal, machine-addressed pin, and queue reordering as one digest entry and one notification per change - attention never authority, staleness a health condition on the configured threshold, retired state honest through every recovery path. Design landed 5ed2a823 after six verification rounds; implementation chain la-impl ran three build and three critique rounds to zero material with every earlier confirmation re-checked on the final tree; one contrived-trace note stays recorded in the chain record (la-verify2). The goal was born from the stale-reads incident - 'M1 sees nothing' - and ends with the machines watching the ledger themselves.
- OpenedAt: 2026-08-30T13:38:36Z
- Revision: 10
- Budget: elapsedLimit=3d attemptLimit=8 reservedJobMinutesLimit=1260 activeJobLimit=2

History:
- 2026-08-30T13:38:36Z GXV12W8X9D21QAQR97ECX52CCA-m1-bf243850 open actor=m1+coordinator targets=ledger-attention
- 2026-08-30T15:17:00Z KVRXBHRWRM6KM2SBR5J8RQPNZF-m1-bf243850 set-budget actor=m1+coordinator targets=ledger-attention
- 2026-08-30T16:52:28Z CZGRTJK4KQY57EXHNGCDF0A9TB-m2-bc1be9cb claim actor=m2+mac-coordinator targets=ledger-attention
- 2026-08-30T20:49:53Z MVHWBPCAX0JTBH9D720F2X3FXW-m2-bc1be9cb set-budget actor=human:wido targets=ledger-attention
- 2026-08-30T21:35:12Z NBFGS7A2K0BGNBBNQQP177YJ93-m2-bc1be9cb edit actor=m2+mac-coordinator targets=ledger-attention
- 2026-08-30T21:35:52Z JX99TT90TJBS38T6CAHVY1A3VY-m2-bc1be9cb release actor=m2+mac-coordinator targets=ledger-attention
- 2026-08-31T10:13:26Z BDFYJK5X79CX03TQXXD4P8ZSPA-m2-bc1be9cb set-budget actor=human:wido targets=ledger-attention
- 2026-08-31T19:09:50Z B058Q7DHZV6VDGGQHZ22DNPHPV-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=ledger-attention
- 2026-09-01T02:44:00Z 5X74AJ9MJ4MA6P6CSDQK4GN8AK-m2-bc1be9cb claim actor=m2+mac-coordinator targets=ledger-attention
- 2026-09-01T06:58:53Z G0CA786E1PYAXZEJZ8852NP65H-m2-bc1be9cb done actor=human:wido targets=ledger-attention
Integrity: sha256=f9f79f88adbf7d50dc629c43648d7e3275f6372b4e37cf1b4c5556222b1ec791
