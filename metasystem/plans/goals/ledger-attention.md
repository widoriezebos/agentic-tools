# ledger-attention

- State: claimed
- Intent: A machine notices when the shared ledger changes under it - the steward tick fetches and surfaces new claimable goals, pins addressed to this machine, and queue reorderings, so no human relays routine nudges between machines (Wido 2026-08-30: 'should we build a mechanism for you to be able to do that yourself?'). ABSORBS fleet-pull (parked, R-33 merge): the same tick lets an idle machine pick up claimable shared-backlog work by itself - fleet liveness is the steward's duty, never a human's memory. Design landed; released for implementation
- Origin: human
- Next step: DESIGN LANDED 5ed2a823 (plans/ledger-attention-design.md): Fable-authored, Sol-verified across three rounds (10->7->7 material), the two requirement-substitution findings CLOSED - the design serves the goal text as written (queue reorderings surfaced, nudge per change). Seven technical findings remain as SS10 residue, each binding on implementation (closed or refuted with evidence; LA-R1-006/LA-R3-001/LA-R3-002 mark parts of SS7 unimplementable as written and take precedence over contradicted plan text). BLOCKED ON BUDGET: 8 jobs against attemptLimit 6, reserved minutes at/past 960 - the implementation slice (Sol lane, DESIGN-BEARING, ~2h, plus Fable code-critique) cannot dispatch until the human raises (~+2 attempts / +300 job-minutes). Three revision runs died on the native dollar cap after completing their product - recorded as goal:budget-death-on-return. Claim released 2026-08-31 so m2 can work goal-scope-bounds serially per Wido's idle-capacity assignment.
- OpenedAt: 2026-08-30T13:38:36Z
- Revision: 9
- Budget: elapsedLimit=3d attemptLimit=8 reservedJobMinutesLimit=1260 activeJobLimit=2
- Claimed: machine=m2 lineage=mac-coordinator at=2026-09-01T02:44:00Z revision=9
- StopCapability: generation=9 revision=9 machine=m2 claimEpoch=1 fenceEpoch=0

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
Integrity: sha256=0d76f2c806ec89625078091096140bb45ee40734d1780cbf8d25efa2edce3b4b
