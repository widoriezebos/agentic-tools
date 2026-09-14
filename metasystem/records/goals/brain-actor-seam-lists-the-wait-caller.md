# brain-actor-seam-lists-the-wait-caller

- State: done
- Priority: 1
- Sequence: 57
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: the brain bed's actor-seam scenario is red on trunk, so a real new Actor or caller-classification site would be hidden; novelty 1: an allow-list line that did not follow a refactor; exposure 2: every deep run of the brain section; accumulation 2: a red allow-list stops reviewing new sites"
- Tier: 1
- Intent: The brain bed's brain-actor-seam-coverage scenario is red on trunk (m1b's control at 68682486). Its exact allow-list (scripts/agents/brain-fixtures.sh, heredoc near lines 341-383) expects cmd/metasystem/wait_verb.go 'view, err := classifyVerbCaller(stateRoot, waitCallerPID())', but m1c's e0c5bd61 (publication-owners-hint-the-waiter, 2026-09-14) made it 'classifyVerbCaller(stateRoot, callerPID)', passing the PID in as a parameter of runWaitCommand. No other site differs. DONE: every caller of runWaitCommand is shown to pass a PID that classification verifies (no caller can hand in an unverified PID), the allow-list names the live site, and brain-actor-seam-coverage passes on trunk.
- Origin: human
- Next step: m1c inspects the runWaitCommand callers, updates the allow-list line, and runs the brain bed scenario; lands after session-start-relays-the-durable-wait-line.
- Concluded: Landed 96c2fe64 (2026-09-15): the brain bed's actor-seam allow-list names the wait verb's classification site as classifyVerbCaller(stateRoot, callerPID), which e0c5bd61 introduced. runWaitCommand's one production caller passes waitCallerPID(), so the site's effect is unchanged. brain-actor-seam-coverage passes on the candidate and fails on trunk.
- OpenedAt: 2026-09-14T22:28:17Z
- Revision: 5
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T22:28:26Z revision=2 opid=Y4B9T33A4904FMGWA7VTKSM1JH-m1e-c6925449 authority=proven digest=50223f0fd3e5c8b34199952a2c420864fda0926d83a4311c49f53ffea2615bc0

History:
- 2026-09-14T22:28:17Z R46QHA5EZHGZ3K6QPMQVCEJ5DG-m1e-c6925449 open actor=human:Wido targets=brain-actor-seam-lists-the-wait-caller reason=TierOverride: derived=2 set=1 why=Opened by m1c in Wido's name under R-110-m1e: a trunk red from m1c's own landing e0c5bd61, found by m1b.
- 2026-09-14T22:28:26Z Y4B9T33A4904FMGWA7VTKSM1JH-m1e-c6925449 approve actor=human:Wido targets=brain-actor-seam-lists-the-wait-caller
- 2026-09-14T22:28:34Z X233DT7QW1P1J8JM60BW1WAYJ4-m1e-c6925449 set-priority actor=human:Wido targets=brain-actor-seam-lists-the-wait-caller reason=priority-order subject=brain-actor-seam-lists-the-wait-caller from=unranked to=1:57 requested-sequence=append
- 2026-09-14T22:33:20Z QGV2FFAQ703GE1BTHWCQYADN9R-m1c-69c9e454 claim actor=m1c+main-1789191340-90689-8975a9 targets=brain-actor-seam-lists-the-wait-caller
- 2026-09-14T22:34:14Z SDWD5654D9FBS63BPH7BR2VX44-m1e-c6925449 done actor=human:Wido targets=adoption-filled-delivery-passes-on-trunk,brain-actor-seam-lists-the-wait-caller,land-bed-reads-build-stamps-without-a-broken-pipe displaced=m1c+main-1789191340-90689-8975a9@2026-09-14T22:33:20Z
Integrity: sha256=6416eda329b42f4282c4c43522896e4f52ffe0d2afb86ce50156dfdac4faed74
