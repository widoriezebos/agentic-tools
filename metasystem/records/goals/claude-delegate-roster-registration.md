# claude-delegate-roster-registration

- State: done
- Intent: Register a benchmark configuration whose roster actually pins Claude delegates: devin-host-claude-delegate@1 provisions an all-Devin roster despite its name (KI-41)
- Origin: main
- Next step: Mint a new registered configuration version (versions.lock is append-only) with role.implementer.runtime=claude and role.design-critic.runtime=claude, contract prose matching; alias or rename so the name tells the truth. Then the actual claude-delegate question bm-2dc was named for can be asked.
- Concluded: Landed 1396d1d, KI-41 closed: devin-host-claude-delegate@2 registered (append-only versions.lock, blob b0cee2e) with roster.delegateRoles pinning implementer/design-critic/code-critic to claude:claude-opus-5 - the machine projection version 1 carried as prose only, which is exactly how bm-2dc provisioned all-Devin. Kit object-model validation green over the new version (7 configurations, registry OK); version-1 cohorts remain devin-host-devin-delegate evidence; the issue-8 unsealable-fences warning stays a human ruling with fences reproduced verbatim. Note: validate-kit's provisioning leg cannot run on m2 - it arms supervision and hits the standing ENROLLMENT_DRIFT gate awaiting Wido's steward arm from the agent-free terminal (the same human-blocked item parked since the severity arc); first provision on an enrolled machine is the remaining live confirmation.
- OpenedAt: 2026-08-24T11:41:09Z
- Revision: 4
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=15 activeJobLimit=1

History:
- 2026-08-24T11:41:09Z 46NE8S3AG3XSFPQVR5YZX4ATTE-m2-bc1be9cb open actor=m2+mac-coordinator targets=claude-delegate-roster-registration
- 2026-08-30T04:56:50Z 62QNV7RVVX6F9XQYMSJCQAS776-m2-bc1be9cb set-budget actor=human:wido targets=claude-delegate-roster-registration
- 2026-08-30T04:57:06Z EFK8ZPG7ES2XRANV7104472GHJ-m2-bc1be9cb claim actor=m2+mac-coordinator targets=claude-delegate-roster-registration
- 2026-08-30T06:22:00Z 77J5X16A3Z63PFQGFGD0CKT45C-m2-bc1be9cb done actor=human:wido targets=claude-delegate-roster-registration
Integrity: sha256=9554fa43abbb1b4ac2c15d7a9cf7b566391c424a3065be8eba22a25d208b89eb
