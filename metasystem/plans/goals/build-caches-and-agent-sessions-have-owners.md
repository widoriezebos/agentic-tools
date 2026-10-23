# build-caches-and-agent-sessions-have-owners

- State: parked
- Priority: 2
- Sequence: 22
- Risk: severity=2 novelty=2 exposure=3 accumulation=3 basis="Lane test finding 387 (2026-10-17): 136 GB free of 926; Go build cache 33 GB, Codex 17 GB, Claude 10 GB, none with an owner; a hand cleanup at 09:21 freed 8.8 GB of Codex sessions but the growth repeats within days. Engine pins (14.9 GB) are owned by a-proof-unit-ends-on-time U5b; this goal owns the caches and the session logs."
- Tier: 2
- Intent: The Go build cache and the agents' session logs have owners: each has a stated bound and a sweep the steward's disk pass runs, so the SSD never fills with recreatable build objects or with Codex and Claude session transcripts nobody reads.
- Origin: human
- Next step: Design: D1 the Go build cache under the steward's disk pass with a size bound and an age rule (go clean -cache only when no build or proof runs on the host; or per-entry trimming by mtime), measured against today's 33 GB; D2 Codex sessions (~/.codex/sessions 9.7 GB, thread_history sqlite 3.5 GB) and Claude session scratch (/private/tmp/claude-501 5.5 GB, ~/.claude 4.7 GB) with a retention count and a rule for logs that evidence still names; Astra critique; units by the hand route.
- OpenedAt: 2026-10-17T08:35:33Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-17T08:35:51Z revision=3 opid=X2DPPH6D4DY5FK2ZRF8PYSJTVF-m1e-c6925449 authority=proven digest=58558961a9451b9454b83809bac7a503aa0b9da195ef2cef5504b22520f36d97 episode=3
- Parked: by=human:Wido at=2026-10-23T22:31:22Z because=parked in Wido's name (decision 47e, plan-use-for-real 0.5): the ledger holds only goals that can start now; resume when its turn comes

History:
- 2026-10-17T08:35:33Z Q6RB86KW7RDB2R903284XS4SEK-m1e-c5a1e456 open actor=human:Wido targets=build-caches-and-agent-sessions-have-owners
- 2026-10-17T08:35:43Z 561DWEWVMHDBA6HX733DV36GZ7-m1e-c5a1e456 set-priority actor=human:Wido targets=build-caches-and-agent-sessions-have-owners reason=priority-order subject=build-caches-and-agent-sessions-have-owners from=unranked to=2:22 requested-sequence=append
- 2026-10-17T08:35:51Z X2DPPH6D4DY5FK2ZRF8PYSJTVF-m1e-c6925449 approve actor=human:Wido targets=build-caches-and-agent-sessions-have-owners
- 2026-10-23T22:31:22Z 0KX10017FWMH4XRBP2Y7H22HVR-m1e-cca12454 park actor=human:Wido targets=build-caches-and-agent-sessions-have-owners reason=parked in Wido's name (decision 47e, plan-use-for-real 0.5): the ledger holds only goals that can start now; resume when its turn comes
Integrity: sha256=a21d9fa33fc5691d61dd54cc33c0e4b0e465828901c7094e8985b62669eb652c
