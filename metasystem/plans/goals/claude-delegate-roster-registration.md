# claude-delegate-roster-registration

- State: claimed
- Intent: Register a benchmark configuration whose roster actually pins Claude delegates: devin-host-claude-delegate@1 provisions an all-Devin roster despite its name (KI-41)
- Origin: main
- Next step: Mint a new registered configuration version (versions.lock is append-only) with role.implementer.runtime=claude and role.design-critic.runtime=claude, contract prose matching; alias or rename so the name tells the truth. Then the actual claude-delegate question bm-2dc was named for can be asked.
- OpenedAt: 2026-08-24T11:41:09Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=15 activeJobLimit=1
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-30T04:57:06Z revision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-24T11:41:09Z 46NE8S3AG3XSFPQVR5YZX4ATTE-m2-bc1be9cb open actor=m2+mac-coordinator targets=claude-delegate-roster-registration
- 2026-08-30T04:56:50Z 62QNV7RVVX6F9XQYMSJCQAS776-m2-bc1be9cb set-budget actor=human:wido targets=claude-delegate-roster-registration
- 2026-08-30T04:57:06Z EFK8ZPG7ES2XRANV7104472GHJ-m2-bc1be9cb claim actor=m2+mac-coordinator targets=claude-delegate-roster-registration
Integrity: sha256=83afc88517e488164cba07f3362e1c793c21f2028023d2f8b85f66a577166420
