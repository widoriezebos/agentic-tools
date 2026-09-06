# claude-implementer-read-roots-writable

- State: claimed
- Risk: severity=3 novelty=1 exposure=3 accumulation=1 basis="severity 3: a builder's shell can change the live checkout that lands the fleet's work, which the envelope claims to forbid; novelty 1: the denyWrite key is proven and the settings builder already knows the roots; exposure 3: every claude implementer round on every seat; accumulation 1: nothing compounds, the hole is per round"
- Tier: 3
- Intent: A claude implementer delegate gets its extra read roots (the live repository root, everything in the record's requested readRoots beyond its worktree) as --add-dir arguments (BuildClaudeCommand, ClaudeReadRoots in metasystem/internal/adapter/claude.go), and the claude sandbox treats every added directory as a writable working directory: a live probe on 2026-09-06 (goal code-critic-runtime-has-no-shell) showed Bash touching a file inside an --add-dir root with sandbox.filesystem.allowWrite empty, and inside the cwd. An implementer's settings allow Bash, so its shell can write the live checkout, not only its worktree; the Edit and Write tools respect allowWrite, Bash does not. The same probe showed sandbox.filesystem.denyWrite is honoured. DONE means every claude delegate's settings deny writes (denyWrite) to every read root and to the workspace root when the workspace is not a write root, a settings test pins it, and a live probe shows an implementer's Bash refused a touch in the live root while its worktree stays writable.
- Origin: main
- Next step: STATE 2026-09-06 21:58Z (m1c): claimed; brief plans/claude-implementer-read-roots-writable-brief.md landed; chain cir-build1 (Sol, DESIGN-BEARING) building: BuildClaudeSettings builds denyWrite for every role (workspace root plus read roots minus anything at or under a write root), emitted only when non-empty, scratch stays in allowWrite; settings tests for an implementer (deny the live root, keep the worktree) and a nested read root. Then seat proofs, a live implementer probe from the candidate engine (touch in the worktree succeeds, touch in the live root refused), one Fable critique, fold if needed, close, land (full width: goal accumulation 1? check gateWidth), rebuild, up, done.
- OpenedAt: 2026-09-06T19:18:35Z
- Revision: 5
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T21:18:46Z revision=2 opid=10S4262XEJBA9XC00VBMTQ7WE6-m1-7cd0bd60 authority=proven digest=521d4eaf35764fb9c8585adc6144c60388fddcf0d4caf99dd53deffe70078483
- Sliced: machine=m1c lineage=main-1788680061-17829-64951c revision=3 at=2026-09-06T21:52:40Z
- Claimed: machine=m1c lineage=main-1788680061-17829-64951c at=2026-09-06T21:52:27Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m1c claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T19:18:35Z C25MKZCYCMWBBM8HVQADXX3RA2-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=claude-implementer-read-roots-writable
- 2026-09-06T21:18:46Z 10S4262XEJBA9XC00VBMTQ7WE6-m1-7cd0bd60 approve actor=human:Wido targets=adopt-bed-authority-probe-passes-tier,adopt-bed-gate-unreachable-under-load,claude-critic-sandbox-allows-loopback,claude-critic-shell-network-deny,claude-delegate-scratch-cleanup,claude-implementer-read-roots-writable
- 2026-09-06T21:52:27Z RW1VPZ1JAXD40N3JSWK9J0V9DP-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=claude-implementer-read-roots-writable
- 2026-09-06T21:52:40Z S207GRXFCW48725DG6D1BFXCYN-m1c-7cd0bd60 slice-start actor=m1c+main-1788680061-17829-64951c targets=claude-implementer-read-roots-writable
- 2026-09-06T21:53:05Z 3YCDK112JHEGPEQ471C7XWHS8J-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=claude-implementer-read-roots-writable
Integrity: sha256=3b51ea9036a5770b55f04b4994a990839bf78eb55aa0043b2e82cce4c390356c
