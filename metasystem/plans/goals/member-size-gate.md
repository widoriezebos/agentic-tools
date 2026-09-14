# member-size-gate

- State: approved
- Priority: 2
- Sequence: 26
- Risk: severity=1 novelty=2 exposure=2 accumulation=1 basis="severity 1: a refused slice is re-sliced, nothing lands wrongly; novelty 2: goal slice gains a measured refusal; exposure 2: every slice; accumulation 1: nothing builds on the count"
- Tier: 2
- Intent: Delivery-efficiency phase D, delegate loop. Goal slice refuses a member whose design names more than a set number of files or moves more than one owner, with the count in the refusal, so a member is split before its build rather than after nineteen rounds. Why: the stop-decisions member touched twenty-two files and five thousand lines and replaced the deadline parent in one build; Wido's rule of 2026-09-10 is one mechanism per member. DONE: a fixture slices an oversized member draft and is refused with the count; a compliant draft passes; the limits live in metasystem.conf. every mechanism in this goal lives in the Go engine, the ledger verbs and the adapter contract, never in one runtime; it is proven on at least two runtimes.
- Origin: human
- Next step: Design page: the two limits, where they are read, the refusal text, the fixture; then build.
- OpenedAt: 2026-09-14T15:31:38Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T18:54:11Z revision=3 opid=YYX0BJJ1AW3MSF8A9C00KP2RQX-m1e-c6925449 authority=proven digest=c279ab9fba4a1840722b4f58b0d0caed4c66663515df38827daf0e5a1f3c8ae0

History:
- 2026-09-14T15:31:38Z X7XDQG0BN01B994R3C38Q3X585-m1e-c6925449 open actor=human:Wido targets=member-size-gate
- 2026-09-14T15:32:32Z BZAZZEKFHN4M0N12ECB46FKAYC-m1e-c6925449 set-priority actor=human:Wido targets=member-size-gate reason=priority-order subject=member-size-gate from=unranked to=2:26 requested-sequence=append
- 2026-09-14T18:54:11Z YYX0BJJ1AW3MSF8A9C00KP2RQX-m1e-c6925449 approve actor=human:Wido targets=member-size-gate
Integrity: sha256=3d7822f97e476f4807ed439c88646ed982e6032fbe794999a1ec379ce0f7de90
