# channel-status-concise

- State: queued
- Tier: 1
- Intent: The fleet Telegram status post (internal/channel/report.go ComposeReport, posted by every machine's steward every channel.status.interval-minutes) is a wall of text: up to twelve Planned lines dump the whole queued backlog every time, plus landed, under-way, spend and undelivered lines, and each machine posts its own copy. Wido (2026-09-04): 'I'm only interested in things that need my judgement/decision and in what was delivered and what will be picked up next IN A CONCISE WAY. Not a full dump of the backlog (every time!)'. DONE means the post has three short parts and nothing else: Needs you (open channel questions, goals waiting for a human word, budget raises; omitted when empty), Delivered since the last post (one line per feature, plain name), Next up (the next one or two approved items in order); no backlog dump, no spend line (cost anomalies are incidents raised on their own); at most twelve lines; nothing posted when nothing changed since the last post.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one report composer and its test): build, run go test ./internal/channel/..., land through a chain; box 1h/3/60m/1. Wido asked for this first among the bugs (2026-09-04); it runs right after the in-flight dispatcher-skew item.
- OpenedAt: 2026-09-04T10:06:55Z
- Revision: 1
- Labels: communication
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0

History:
- 2026-09-04T10:06:55Z 44SVAXX9AB38W8DP7HVFR0HY4A-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=channel-status-concise
Integrity: sha256=41f9d46a3d0c24a0aed6e2412c22e8cb128c9d26eb232f7e5b1dbcb1a062dac3
