# answer-archive

- State: queued
- Intent: An aggregate, durable ARCHIVE of every human answer the fleet receives (Wido's word 2026-09-03, companion to fleet-channel-gateway): all of Wido's replies - to asks, stop-loss resets, approvals relayed through the channel - land in one append-only log on the transport, with the ask they answered, the provider they came from, the time, and the machine that consumed them. DONE means: one command lists every answer ever received fleet-wide in order, each joined to its ask and outcome; nothing the gateway ingests is ever lost even after the inbox rotates; the archive is readable by every machine and by Wido.
- Origin: main
- Next step: INTENT: the fleet's memory of what the human said. CONSTRAINTS: append-only, git-synced (register carriage), never rewritten; joins to ask ids and goal ids; survives inbox rotation (the inbox is the working queue, the archive is the record); no secrets (tokens, TOTP) ever land in it. FREEDOMS: file format (jsonl vs markdown table), location (records/ vs memory/), whether the gateway writes it directly or a steward phase harvests the inbox. Depends on fleet-channel-gateway landing (it produces the inbox the archive harvests) - sequence it after. Small (4h box) once the gateway exists. Budget Wido's word at approval.
- OpenedAt: 2026-09-03T15:27:59Z
- Revision: 1

History:
- 2026-09-03T15:27:59Z WR0H1SK4VDG1X4207R511WQN7B-m0-c5dbf036 open actor=human:Wido targets=answer-archive
Integrity: sha256=d842020a7dee9699a249affa420cc05931c3a9546027436e801d23318c12d17a
