# a-live-read-proves-its-subject

- State: claimed
- Priority: 1
- Sequence: 92
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="One git flag so two existing digests of the same bytes agree; a mistake is the refusal we have today"
- Tier: 1
- Intent: A unit run's live read proves the committed subject: the worktree diff the read digests is written with the same git flags as the commit's patch digest (--full-index), so a clean run-read is promoted at collection instead of refused and replaced by a committed critic; a tier-1 goal can complete its read (finding 436)
- Origin: main
- Next step: On m1e's build lane after a-settled-reply-names-its-machine and before a-ready-goal-behind-a-claim-is-a-row (one line, minutes): opus-builder adds --full-index to worktreeDiff (internal/launch/unit_run.go:1182) and a test that computes both digests from one committed unit through the production paths (worktreeDiff and read_subject_compute) and asserts they are equal, failing first; Read each round: yes; the steward submits with work review --changes --last
- OpenedAt: 2026-10-27T17:41:56Z
- FirstClaimAt: 2026-10-27T18:52:48Z
- Revision: 6
- Pinned: m1e
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=TGKGSFA630HZSQSQKQNQWP9877-m1e-55cdfc19 minutes=720 reviewRounds=20 goalRevision=2
- Approved: by=human:Wido at=2026-10-27T17:42:15Z revision=3 opid=TGKGSFA630HZSQSQKQNQWP9877-m1e-55cdfc19 authority=proven digest=5cb30766684127890e6524486dd98855ceb83f5c1be36a00a2948841925893b7 episode=3
- Claimed: machine=m1e lineage=main-1792389436-6018-51cc28 at=2026-10-27T18:52:48Z areas="[]" areas-source="" areas-known=false areas-warnings="[\"areas unknown for goal a-live-read-proves-its-subject: no accepted design declares areas\",\"areas unknown for goal a-capacity-refusal-is-an-environment-stop\",\"areas unknown for goal a-settled-reply-names-its-machine\",\"areas unknown for goal lane-first-run-fixes\",\"areas unknown for goal lane-recovers-and-pauses\",\"areas unknown for goal a-capacity-refusal-is-an-environment-stop\",\"areas unknown for goal a-settled-reply-names-its-machine\"]" revision=6 accountingRevision=6 episodeAt=2026-10-27T18:52:48Z episodeRevision=6
- StopCapability: generation=6 revision=6 machine=m1e claimEpoch=10 fenceEpoch=0

History:
- 2026-10-27T17:41:56Z 87D50ZJR623QVE7YY5N17HMQ1X-m1e-55cdfc19 open actor=human:Wido targets=a-live-read-proves-its-subject
- 2026-10-27T17:42:05Z BHMXNYRAVRWT0DA3EV4C3566R6-m1e-55cdfc19 approve actor=human:Wido targets=a-live-read-proves-its-subject
- 2026-10-27T17:42:15Z TGKGSFA630HZSQSQKQNQWP9877-m1e-55cdfc19 approve actor=human:Wido targets=a-live-read-proves-its-subject
- 2026-10-27T17:42:25Z 5NXJW9053AFPMZGKWZ50RFYH8W-m1e-55cdfc19 set-pin actor=human:Wido targets=a-live-read-proves-its-subject
- 2026-10-27T17:42:33Z 1N2YZCEW8WGWTCQSYZY56RBM9E-m1e-55cdfc19 set-priority actor=human:Wido targets=a-live-read-proves-its-subject reason=priority-order subject=a-live-read-proves-its-subject from=unranked to=1:92 requested-sequence=append
- 2026-10-27T18:52:48Z CDTY6ASKNQ27VJ2NADZV217X98-m1e-cc2e8fc8 claim actor=m1e+main-1792389436-6018-51cc28 targets=a-live-read-proves-its-subject reason=areas unknown for goal a-live-read-proves-its-subject: no accepted design declares areas; areas unknown for goal a-capacity-refusal-is-an-environment-stop; areas unknown for goal a-settled-reply-names-its-machine; areas unknown for goal lane-first-run-fixes; areas unknown for goal lane-recovers-and-pauses; areas unknown for goal a-capacity-refusal-is-an-environment-stop; areas unknown for goal a-settled-reply-names-its-machine
Integrity: sha256=5b139f9bbf844103103ae0236b0379109a481be4043f8ce0471dc9d5d7147b2d
