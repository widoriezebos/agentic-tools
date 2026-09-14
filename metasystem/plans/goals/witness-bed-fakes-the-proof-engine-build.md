# witness-bed-fakes-the-proof-engine-build

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: three witness-gate scenarios are red on trunk, so a deep landing proof fails for a reason the change did not cause; novelty 1: a fixture stub lacks one command the gate runs; exposure 2: every landing whose surfaces require a deep proof; accumulation 2: a red bed on trunk hides new reds in the same scenarios"
- Tier: 1
- Intent: The witness gate bed must be green on trunk. Since 1b12f534 (2026-09-09) scripts/agents/go-gate.sh:83-85 builds a proof engine with go build -o, but the fake go that scripts/agents/witness-gate-fixtures.sh writes in make_leg has no build branch. So authority-and-scope, equality-and-weights and frozen-consumers fail with 'fixture go: unsupported command: build -o <tmp>/metasystem-go-gate.XXXXXX/metasystem ./cmd/metasystem' ('refusal did not reach the broken full proof', 'matching ENGINE did not skip'). m1c reproduced it on 2026-09-14 at 5adc613c and at 6c43a54d. The section sits only in proof-and-landing's deep group, so standard landings never ran it. DONE: every witness-gate scenario passes on trunk with no assertion weakened or removed, and the fake go accepts only the build the gate really runs.
- Origin: human
- Next step: Build: give the bed's fake go the proof-engine build from go-gate.sh:85 without weakening any scenario; run the whole witness bed on trunk; carry into hook-root-resolver-design's landing if its proof runs deep.
- OpenedAt: 2026-09-14T21:09:59Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T21:10:05Z revision=2 opid=KXSCXAPYNC8WY4NZPA56P9NGMN-m1e-c6925449 authority=proven digest=2370fc94eda0ff3e868f9372087c82c3d07e7f8fa681b3f2270f0a6a0bf612e3

History:
- 2026-09-14T21:09:59Z WW3A9EYAZFBE9YHJ98151BXPJM-m1e-c6925449 open actor=human:Wido targets=witness-bed-fakes-the-proof-engine-build reason=TierOverride: derived=2 set=1 why=Opened by m1c in Wido's name under R-110-m1e: a trunk red found while verifying hook-root-resolver-design's landing tree.
- 2026-09-14T21:10:05Z KXSCXAPYNC8WY4NZPA56P9NGMN-m1e-c6925449 approve actor=human:Wido targets=witness-bed-fakes-the-proof-engine-build
Integrity: sha256=aeaf98b3100dcfc6e6c134404ec4cfd8d7e40564f563b5b4a5b7b8fdcece5dba
