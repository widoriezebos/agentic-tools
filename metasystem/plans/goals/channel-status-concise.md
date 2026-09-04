# channel-status-concise

- State: claimed
- Tier: 1
- Intent: The fleet Telegram status post (internal/channel/report.go ComposeReport, posted by every machine's steward every channel.status.interval-minutes) is a wall of text: up to twelve Planned lines dump the whole queued backlog every time, plus landed, under-way, spend and undelivered lines, and each machine posts its own copy. Wido (2026-09-04): 'I'm only interested in things that need my judgement/decision and in what was delivered and what will be picked up next IN A CONCISE WAY. Not a full dump of the backlog (every time!)'. DONE means the post has three short parts and nothing else: Needs you (open channel questions, goals waiting for a human word, budget raises; omitted when empty), Delivered since the last post (one line per feature, plain name), Next up (the next one or two approved items in order); no backlog dump, no spend line (cost anomalies are incidents raised on their own); at most twelve lines; nothing posted when nothing changed since the last post.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one report composer and its test): build, run go test ./internal/channel/..., land through a chain; box 1h/3/60m/1. Wido asked for this first among the bugs (2026-09-04); it runs right after the in-flight dispatcher-skew item.
- OpenedAt: 2026-09-04T10:06:55Z
- Revision: 3
- Labels: communication
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- Approved: by=human:Wido at=2026-09-04T10:07:01Z revision=2 opid=T4CM9DABHBCEVY94H33H1HDP41-m2-5fcf08ab authority=relayed digest=0434e079ced63b03294e4ceee032ec726f82456cffcf40d6e20df2f373211470 reviewBy=2026-09-06
- Claimed: machine=m2 lineage=main-1788441779-14484-82d6ed at=2026-09-04T10:10:23Z revision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=2 fenceEpoch=0

History:
- 2026-09-04T10:06:55Z 44SVAXX9AB38W8DP7HVFR0HY4A-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=channel-status-concise
- 2026-09-04T10:07:01Z T4CM9DABHBCEVY94H33H1HDP41-m2-5fcf08ab approve actor=human:Wido targets=channel-status-concise authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="Once we have landed the planned work and you are picking up bugs/issues, can you do this one first?"
- 2026-09-04T10:10:23Z NY17TX7CW5ZK7K5G134N82XJJ2-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=channel-status-concise
Integrity: sha256=6d43483b0e1bee31d90f27e3942983a8729d8e4aa0a0f05b950c3c1165027faf
