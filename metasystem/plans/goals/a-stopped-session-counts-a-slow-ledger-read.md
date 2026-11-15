# a-stopped-session-counts-a-slow-ledger-read

- State: approved
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="U4 of a-slow-ledger-read-is-a-wait-2, carried forward unchanged: that goal was handed in whole at U3 on 2026-11-14 and goal split refuses a done parent (internal/goal/split.go:134, :251, :266). Decided in /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/the-roles-act-without-a-proxy.md amendment A5 (Astra r2 finding 1). The source goal's delivery ended at U3."
- Tier: 1
- Intent: A stopped session counts a slow ledger read as a wait: the Stop hook's timed-out ledger read is a counted wait with a persisted count; a successful read resets the count; the third timeout notifies; independent blocks are preserved.
- Origin: main
- Next step: Build one unit (estimate 60, --last) without a new design from /Users/wido/agentic-tools-evidence/lane-test-20261009/briefs/a-slow-ledger-read-is-a-wait-brief.md, its U4 kept whole; seams internal/goal/turnverdict.go:1281 and internal/hooks/runtime_hook_stop.go:845. The brief starts with an inventory. Done when a timed-out Stop hook ledger read is counted and persisted, a successful read resets the count, the third timeout notifies once, and independent blocks still block.
- OpenedAt: 2026-11-15T20:26:16Z
- Revision: 2
- Budget: elapsedLimit=2d attemptLimit=12 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=BGKRMX1HMWHE33VSHBYHZJZA07-landing-ccee003e minutes=1200 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-11-15T20:26:27Z revision=2 opid=BGKRMX1HMWHE33VSHBYHZJZA07-landing-ccee003e authority=proven digest=837436493d2d5310080c9536b70d4581d1419865866dc7f516e41995a19e3287 episode=2

History:
- 2026-11-15T20:26:16Z WW12WT3SC9Y3V54H45PR98WTHF-landing-ccee003e open actor=human:Wido targets=a-stopped-session-counts-a-slow-ledger-read
- 2026-11-15T20:26:27Z BGKRMX1HMWHE33VSHBYHZJZA07-landing-ccee003e approve actor=human:Wido targets=a-stopped-session-counts-a-slow-ledger-read
Integrity: sha256=d6cce9f0d1066568ae3169845f9164c207291202cdc02d074042f78c03bd87c9
