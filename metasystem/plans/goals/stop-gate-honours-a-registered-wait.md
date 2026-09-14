# stop-gate-honours-a-registered-wait

- State: approved
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: this gate decides whether a seat is released while it is waiting, so a defect either wedges seats or lets them stop with work pending; novelty 2: the stop table has never held on a registered wait and its interaction with the live-job counter and human-act returns is new; exposure 3: every seat on every runtime ends every turn through this gate; accumulation 2: each wrong release costs a whole seat-idle interval and they compound across a day"
- Tier: 2
- Intent: Member 3 of coordinator-wakes-on-events-not-polls, and the piece that makes Wido's invariant of 2026-09-14 mechanical rather than instructed: a seat stops only when there is genuinely no way to continue, and a registered wait is not idleness. Today the stop gate can release a seat holding a live wait, so continuation depends on the agent remembering to arm something outside the machinery; on m1c on 2026-09-14 a finished delegate sat unread for eleven minutes for exactly that reason. No Stop text can fix it: a design critique established from the runtimes that on an ALLOWED Stop the message reaches the human and schedules no further model inference on Claude or current Codex, and Devin has no report-bearing mapping. DONE, from the design section 6: the stop table holds on fake and on Claude, the existing live-job counter effects and the human-act return are preserved, and docs/design/turn-verdict-delivery-contract.md says so. A seat with genuinely nothing to do still goes quiet and must not burn tokens waiting.
- Origin: human
- Next step: Read plans/coordinator-wakes-on-events-not-polls-design.md section 6 member three and its section 5 rows, then build: the stop gate in internal/goal/turnverdict.go holds while a registered wait is pending, the contract document is updated, and the named fixtures and bed wiring land with it. Members 1 and 2 are landed (wait verb works, events arrive unasked). Preserve every existing Stop decision: a seat with claimable work and no registered wait blocks exactly as it does today. Opened in Wido's name under the authority he relayed to m1c on 2026-09-14.
- OpenedAt: 2026-09-14T14:10:16Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T14:10:54Z revision=2 opid=FZ7VHKT5ZDHTBAWMB1RJC7ZCTJ-m1e-c6925449 authority=proven digest=6494cef9a69e12758f12b6067a9aef48d1c451b9fee19d7148e58b45c59f3fd2

History:
- 2026-09-14T14:10:16Z C7G2H4XV4KMWP20ART288M34YY-m1e-c6925449 open actor=human:Wido targets=stop-gate-honours-a-registered-wait reason=TierOverride: derived=3 set=2 why=member 3 of an existing tier-3 program, scoped to one gate with a named design and fixtures, so it carries tier 2 rigour rather than the parent's
- 2026-09-14T14:10:54Z FZ7VHKT5ZDHTBAWMB1RJC7ZCTJ-m1e-c6925449 approve actor=human:Wido targets=stop-gate-honours-a-registered-wait
Integrity: sha256=a88f41614cf2ebbff4a1046721bf086eb6800c8648347f4ade5f725fd335fb70
