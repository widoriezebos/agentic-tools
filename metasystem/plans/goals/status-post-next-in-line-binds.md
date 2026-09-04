# status-post-next-in-line-binds

- State: claimed
- Tier: 1
- Intent: The concise fleet status post's one decision line names the queued goal that sorts first alphabetically as 'next in line' (internal/channel/report.go sorts ids with sort.Strings), not the item the seat would actually pick, and a reply to that line binds nothing: on 2026-09-04 Wido replied 'approved <code>' at 11:09Z and 'Approved <code>' at 11:32Z to two status posts and both landed in artifacts/agents/channel/fleet/unmatched.jsonl while the post had named 'account provenance' only because of the alphabet. DONE means (1) the decision line appears only for a goal the seat has marked as its next pick (pinned to the machine and carrying the label next, set by the seat when it decides), never by sort order; (2) that line ends with the reply form the channel already uses for questions, a token verbatim plus the six-digit code, and the poll treats such a reply as the human's execution approval of that goal, recording it the way goal approve by relayed word does with the message reference as the word; (3) an unmatched reply to a status post is answered in its thread with what it did not bind, so the human is never silently ignored.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one composer rule, one poll disposition, a label): build, go test ./internal/channel/... ./internal/goal/..., land through a chain; box 1h/3/60m/1. Waits for human approval for execution; found when Wido replied to the post on 2026-09-04.
- OpenedAt: 2026-09-04T11:36:29Z
- Revision: 4
- Labels: communication
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- Approved: by=human:Wido at=2026-09-04T14:10:11Z revision=2 opid=PHXKRTDKMX7MVPAZE7FQ7PZNWS-m2-5fcf08ab authority=relayed digest=04127afeaa2047097457fe69aa02995a698490506afe9618602fd7df81eaf19f reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=3 at=2026-09-04T14:10:56Z
- Claimed: machine=m2 lineage=main-1788441779-14484-82d6ed at=2026-09-04T14:10:16Z revision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=2 fenceEpoch=0

History:
- 2026-09-04T11:36:29Z G8P4KXXSDSJR3YPNJZVM8B2VEH-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=status-post-next-in-line-binds
- 2026-09-04T14:10:11Z PHXKRTDKMX7MVPAZE7FQ7PZNWS-m2-5fcf08ab approve actor=human:Wido targets=status-post-next-in-line-binds authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="You may pick and execute the tier-1 items opened on 2026-09-04 in my name."
- 2026-09-04T14:10:16Z ERTEB5MA7VXW8NDVB2Q59V69K5-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=status-post-next-in-line-binds
- 2026-09-04T14:10:56Z 2V932HHT6BVPBVWZYWVW97WZSB-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=status-post-next-in-line-binds
Integrity: sha256=65d1bbc85780e1f6387bf573ed03262046e07c5f0d33e1135c97dc1fad378194
