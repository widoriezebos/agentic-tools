# fast-gate-runs-the-refusal-register

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="Every landing that names a new token breaks the next full gate for another seat; twice today; the fix is one gate stage and one register line."
- Tier: 3
- Intent: The refusal register test (TestHCL03EveryCodeRowed in internal/refusal) runs only in the full race gate, not in the fast gate every landing runs, so a landing that adds a refusal-shaped token without a register row turns the full gate red hours later for whoever runs it next. Twice on 2026-09-06: VERIFIED_CHANNEL_ANSWER (landed 09-04, fixed by goal race-gate-red-on-main at 6c79648a) and DEADLINE_EXPIRED (landed 19b14a9b, a steward health observation, found by the governed validation on m1c at 15:07Z). The register test takes under a second. DONE means scripts/agents/go-gate.sh --fast runs the refusal package's tests (go test ./internal/refusal) as one of its static stages so a landing cannot introduce an unrowed token, DEADLINE_EXPIRED has its row or exclusion (an observation that refuses nothing, with its owner's confirmation), and the full gate is green on that count.
- Origin: main
- Next step: Chain fgr-build1 (codex, DESIGN-BEARING, started 16:3xZ) builds from plans/fast-gate-runs-the-refusal-register-brief.md (e66ba132): go-gate.sh --fast gains a refusal-register stage before the build; ledger-unreadable (a turn-verdict digest sentinel, not a refusal) gets its exclusion. Then one Fable critique, close, records, receipt (the new verb in the live root), land through land.sh --chain --test-receipt, conclude (agent-opened).
- OpenedAt: 2026-09-06T15:20:47Z
- Revision: 7
- Labels: robustness
- Pinned: m1c
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T15:32:11Z revision=2 opid=EPXR6VP5RR0M6NF9BARZHBZF2W-m1-7cd0bd60 authority=proven digest=d153ccbc2fed33f60f7aa6bff2e74f9e55abe2d12d27abe1c448f91641c635ab
- Sliced: machine=m1c lineage=main-1788680061-17829-64951c revision=5 at=2026-09-06T16:27:16Z
- Claimed: machine=m1c lineage=main-1788680061-17829-64951c at=2026-09-06T16:25:39Z revision=5 accountingRevision=5
- StopCapability: generation=5 revision=5 machine=m1c claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T15:20:47Z SNQ1VT6Q378Q6GC522VSQFQR9Z-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=fast-gate-runs-the-refusal-register
- 2026-09-06T15:32:11Z EPXR6VP5RR0M6NF9BARZHBZF2W-m1-7cd0bd60 approve actor=human:Wido targets=fast-gate-runs-the-refusal-register
- 2026-09-06T15:43:35Z 0ZMF520EZ6S29EEXPZ0PFAX17P-m1-7cd0bd60 set-pin actor=human:Wido targets=fast-gate-runs-the-refusal-register
- 2026-09-06T15:49:19Z T2V655JEMNH7TRJRNDCC2AGVRW-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=fast-gate-runs-the-refusal-register
- 2026-09-06T16:25:39Z PVCHY1AEX6NWRFRD18203DZKYE-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=fast-gate-runs-the-refusal-register
- 2026-09-06T16:27:16Z WZZYCE6W7NHQ7KERNRRFND6K91-m1c-7cd0bd60 slice-start actor=m1c+main-1788680061-17829-64951c targets=fast-gate-runs-the-refusal-register
- 2026-09-06T16:27:32Z Y50ZNNXG6JCGKXW3WA00GZ0WW1-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=fast-gate-runs-the-refusal-register
Integrity: sha256=370f7a6d0e966afaee1d44239d45af07e9542be2037fb9cba778efadd846b413
