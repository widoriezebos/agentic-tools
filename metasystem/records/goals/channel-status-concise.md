# channel-status-concise

- State: done
- Tier: 1
- Intent: The fleet Telegram status post (internal/channel/report.go ComposeReport, posted by every machine's steward every channel.status.interval-minutes) is a wall of text: up to twelve Planned lines dump the whole queued backlog every time, plus landed, under-way, spend and undelivered lines, and each machine posts its own copy. Wido (2026-09-04): 'I'm only interested in things that need my judgement/decision and in what was delivered and what will be picked up next IN A CONCISE WAY. Not a full dump of the backlog (every time!)'. DONE means the post has three short parts and nothing else: Needs you (open channel questions, goals waiting for a human word, budget raises; omitted when empty), Delivered since the last post (one line per feature, plain name), Next up (the next one or two approved items in order); no backlog dump, no spend line (cost anomalies are incidents raised on their own); at most twelve lines; nothing posted when nothing changed since the last post.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one report composer and its test): build, run go test ./internal/channel/..., land through a chain; box 1h/3/60m/1. Wido asked for this first among the bugs (2026-09-04); it runs right after the in-flight dispatcher-skew item.
- Concluded: Landed 4dd26cfc and aa116c12: the fleet status post has needs-you, delivered and next-up parts only, twelve lines at most, one decision line, silent when unchanged.
- OpenedAt: 2026-09-04T10:06:55Z
- Revision: 8
- Labels: communication
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- Approved: by=human:Wido at=2026-09-04T10:07:01Z revision=2 opid=T4CM9DABHBCEVY94H33H1HDP41-m2-5fcf08ab authority=relayed digest=0434e079ced63b03294e4ceee032ec726f82456cffcf40d6e20df2f373211470 reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=3 at=2026-09-04T10:10:33Z

History:
- 2026-09-04T10:06:55Z 44SVAXX9AB38W8DP7HVFR0HY4A-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=channel-status-concise
- 2026-09-04T10:07:01Z T4CM9DABHBCEVY94H33H1HDP41-m2-5fcf08ab approve actor=human:Wido targets=channel-status-concise authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="Once we have landed the planned work and you are picking up bugs/issues, can you do this one first?"
- 2026-09-04T10:10:23Z NY17TX7CW5ZK7K5G134N82XJJ2-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=channel-status-concise
- 2026-09-04T10:10:33Z GBPE5FF9BPVTJFTJW889BRQ4DS-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=channel-status-concise
- 2026-09-04T10:30:58Z GHKZSTAB90D81D8J1DFRN7ZQM6-m2-5fcf08ab release actor=m2+main-1788441779-14484-82d6ed targets=channel-status-concise
- 2026-09-04T11:00:44Z 9Q8P2A5RVM2W7FKF1BE24018RT-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=channel-status-concise
- 2026-09-04T11:10:44Z HSZHXAT5YBD98499G9X5VBSA5J-m2-5fcf08ab release actor=m2+main-1788441779-14484-82d6ed targets=channel-status-concise
- 2026-09-04T11:12:09Z KZ0R5Q0FPAE1941VVYKH4JTGNT-m2-5fcf08ab done actor=m2+main-1788441779-14484-82d6ed targets=channel-status-concise
Integrity: sha256=bbc5426c4d864e94bbd4811528d9cd9fc70f7e803e2eb0d3dbf67ee155ed76d0
