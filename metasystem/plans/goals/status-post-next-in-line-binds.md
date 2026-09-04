# status-post-next-in-line-binds

- State: queued
- Tier: 1
- Intent: The concise fleet status post's one decision line names the queued goal that sorts first alphabetically as 'next in line' (internal/channel/report.go sorts ids with sort.Strings), not the item the seat would actually pick, and a reply to that line binds nothing: on 2026-09-04 Wido replied 'approved <code>' at 11:09Z and 'Approved <code>' at 11:32Z to two status posts and both landed in artifacts/agents/channel/fleet/unmatched.jsonl while the post had named 'account provenance' only because of the alphabet. DONE means (1) the decision line appears only for a goal the seat has marked as its next pick (pinned to the machine and carrying the label next, set by the seat when it decides), never by sort order; (2) that line ends with the reply form the channel already uses for questions, a token verbatim plus the six-digit code, and the poll treats such a reply as the human's execution approval of that goal, recording it the way goal approve by relayed word does with the message reference as the word; (3) an unmatched reply to a status post is answered in its thread with what it did not bind, so the human is never silently ignored.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one composer rule, one poll disposition, a label): build, go test ./internal/channel/... ./internal/goal/..., land through a chain; box 1h/3/60m/1. Waits for human approval for execution; found when Wido replied to the post on 2026-09-04.
- OpenedAt: 2026-09-04T11:36:29Z
- Revision: 1
- Labels: communication
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0

History:
- 2026-09-04T11:36:29Z G8P4KXXSDSJR3YPNJZVM8B2VEH-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=status-post-next-in-line-binds
Integrity: sha256=990b7456f2fce2bfeabd3e1d75a698f175e77c0fac3be8b9bb7bbad6f83fb371
