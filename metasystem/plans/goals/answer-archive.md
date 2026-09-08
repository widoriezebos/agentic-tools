# answer-archive

- State: queued
- Priority: 3
- Sequence: 26
- Intent: An aggregate, durable ARCHIVE of every human answer the fleet receives (Wido's word 2026-09-03, companion to fleet-channel-gateway): all of Wido's replies - to asks, stop-loss resets, approvals relayed through the channel - land in one append-only log on the transport, with the ask they answered, the provider they came from, the time, and the machine that consumed them. DONE means: one command lists every answer ever received fleet-wide in order, each joined to its ask and outcome; nothing the gateway ingests is ever lost even after the inbox rotates; the archive is readable by every machine and by Wido.
- Origin: main
- Next step: INTENT: the fleet's memory of what the human said. CONSTRAINTS: append-only, git-synced (register carriage), never rewritten; joins to ask ids and goal ids; survives inbox rotation (the inbox is the working queue, the archive is the record); no secrets (tokens, TOTP) ever land in it. FREEDOMS: file format (jsonl vs markdown table), location (records/ vs memory/), whether the gateway writes it directly or a steward phase harvests the inbox. Depends on fleet-channel-gateway landing (it produces the inbox the archive harvests) - sequence it after. Small (4h box) once the gateway exists. Budget Wido's word at approval.
- OpenedAt: 2026-09-03T15:27:59Z
- Revision: 2
- BudgetExceptions: 0

History:
- 2026-09-03T15:27:59Z WR0H1SK4VDG1X4207R511WQN7B-m0-c5dbf036 open actor=human:Wido targets=answer-archive
- 2026-09-08T16:00:39Z 99YCBHR320V5Q81TV1RBJ87G89-m1-7cd0bd60 set-priority actor=human:Wido targets=answer-archive reason=priority-order subject=answer-archive from=unranked to=3:26 requested-sequence=26
Integrity: sha256=e01b29b282c8b884e11d09c84c431aac50cd98534d68e39f3e232d2f9d3f6815
