# land-bed-reads-build-stamps-without-a-broken-pipe

- State: approved
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: false reds in fixture legs, product behaviour unaffected; novelty 1: the fix pattern has landed twice; exposure 2: every run of the land deep section goes red; accumulation 2: five scenarios and a shared harness helper"
- Tier: 1
- Intent: Five land bed scenarios die with SIGPIPE (rc 141) within seconds on unmodified trunk 68682486 and print nothing: build-stamp, brain-land-refuses, brain-absent-node-proceeds, receipt-cutover and abandonment-refuses-every-push-route (a harness-tracked control by m1b on 2026-09-15; the row 24 candidate fails the same five). build-stamp reads a stamp with go version -m piped into sed and then into head -1 under pipefail. When the reader exits early, the producer takes SIGPIPE and set -e ends the child. The same shape is in harness_dispatch_fixture_bed_mint_capability in scripts/agents/fixture-budget.sh. Class: pipelines-never-lose-a-truncated-producer. DONE: every producer piped into an early-exiting reader in scripts/agents/land-fixtures.sh and in harness_dispatch_fixture_bed_mint_capability reads through a file, a here-string or a two-step read with checked statuses instead; the five scenarios pass on trunk; a leg proves one such read with a producer larger than a pipe buffer.
- Origin: human
- Next step: Build, tier 1: find each pipe into head, grep -q, sed with q, or awk with exit in the five scenarios and their shared helpers, and replace it with the landed patterns (here-strings as in 71b96a02, a file as in 9bc768c0). Include harness_dispatch_fixture_bed_mint_capability. Rebase over beds-report-every-failure if it has landed, since both touch fixture-budget.sh. Free for the next seat by sequence.
- OpenedAt: 2026-09-14T22:33:27Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T22:33:35Z revision=2 opid=E2DJZ3AFHSP86S5MMBS25RJXGN-m1e-c6925449 authority=proven digest=fd4b17fa7570dcf3232588b224c3df9f04f7417064e974d4543ad56de267a783

History:
- 2026-09-14T22:33:27Z FT72HNYMYDZ7N1392TJVCFFYM7-m1e-c6925449 open actor=human:Wido targets=land-bed-reads-build-stamps-without-a-broken-pipe
- 2026-09-14T22:33:35Z E2DJZ3AFHSP86S5MMBS25RJXGN-m1e-c6925449 approve actor=human:Wido targets=land-bed-reads-build-stamps-without-a-broken-pipe
Integrity: sha256=9d664ba35e7d0e9f51dcadecf11c6659959ba8691f319a69ad05ac38a07fe6fb
