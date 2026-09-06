# fast-gate-runs-the-refusal-register

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="Every landing that names a new token breaks the next full gate for another seat; twice today; the fix is one gate stage and one register line."
- Tier: 3
- Intent: The refusal register test (TestHCL03EveryCodeRowed in internal/refusal) runs only in the full race gate, not in the fast gate every landing runs, so a landing that adds a refusal-shaped token without a register row turns the full gate red hours later for whoever runs it next. Twice on 2026-09-06: VERIFIED_CHANNEL_ANSWER (landed 09-04, fixed by goal race-gate-red-on-main at 6c79648a) and DEADLINE_EXPIRED (landed 19b14a9b, a steward health observation, found by the governed validation on m1c at 15:07Z). The register test takes under a second. DONE means scripts/agents/go-gate.sh --fast runs the refusal package's tests (go test ./internal/refusal) as one of its static stages so a landing cannot introduce an unrowed token, DEADLINE_EXPIRED has its row or exclusion (an observation that refuses nothing, with its owner's confirmation), and the full gate is green on that count.
- Origin: main
- Next step: Read go-gate.sh's fast stages (gofmt, vet, staticcheck, build) and add the refusal package test after the build; add the DEADLINE_EXPIRED exclusion beside the other steward health observations after confirming with the landing's author (19b14a9b) that it admits rather than refuses; one small chain. THIRD instance the same day (m1c, 15:5xZ): after 6ab703f7 excluded DEADLINE_EXPIRED, main is red again on TestHCL03EveryCodeRowed with the token 'ledger-unreadable' (a hyphen-form refusal code) from 35c9e4df; it needs a row (it looks like a refusal) or an exclusion, and the fast gate must run this sub-second test or this repeats with every landing that names a code.
- OpenedAt: 2026-09-06T15:20:47Z
- Revision: 4
- Labels: robustness
- Pinned: m1c
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T15:32:11Z revision=2 opid=EPXR6VP5RR0M6NF9BARZHBZF2W-m1-7cd0bd60 authority=proven digest=d153ccbc2fed33f60f7aa6bff2e74f9e55abe2d12d27abe1c448f91641c635ab

History:
- 2026-09-06T15:20:47Z SNQ1VT6Q378Q6GC522VSQFQR9Z-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=fast-gate-runs-the-refusal-register
- 2026-09-06T15:32:11Z EPXR6VP5RR0M6NF9BARZHBZF2W-m1-7cd0bd60 approve actor=human:Wido targets=fast-gate-runs-the-refusal-register
- 2026-09-06T15:43:35Z 0ZMF520EZ6S29EEXPZ0PFAX17P-m1-7cd0bd60 set-pin actor=human:Wido targets=fast-gate-runs-the-refusal-register
- 2026-09-06T15:49:19Z T2V655JEMNH7TRJRNDCC2AGVRW-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=fast-gate-runs-the-refusal-register
Integrity: sha256=f195ad4e652a287d45c1fbef42ff596fc52411843178eda2e77004a33ed8b31a
