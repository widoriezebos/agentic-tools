# claude-implementer-read-roots-writable

- State: queued
- Risk: severity=3 novelty=1 exposure=3 accumulation=1 basis="severity 3: a builder's shell can change the live checkout that lands the fleet's work, which the envelope claims to forbid; novelty 1: the denyWrite key is proven and the settings builder already knows the roots; exposure 3: every claude implementer round on every seat; accumulation 1: nothing compounds, the hole is per round"
- Tier: 3
- Intent: A claude implementer delegate gets its extra read roots (the live repository root, everything in the record's requested readRoots beyond its worktree) as --add-dir arguments (BuildClaudeCommand, ClaudeReadRoots in metasystem/internal/adapter/claude.go), and the claude sandbox treats every added directory as a writable working directory: a live probe on 2026-09-06 (goal code-critic-runtime-has-no-shell) showed Bash touching a file inside an --add-dir root with sandbox.filesystem.allowWrite empty, and inside the cwd. An implementer's settings allow Bash, so its shell can write the live checkout, not only its worktree; the Edit and Write tools respect allowWrite, Bash does not. The same probe showed sandbox.filesystem.denyWrite is honoured. DONE means every claude delegate's settings deny writes (denyWrite) to every read root and to the workspace root when the workspace is not a write root, a settings test pins it, and a live probe shows an implementer's Bash refused a touch in the live root while its worktree stays writable.
- Origin: main
- Next step: MECHANICAL after code-critic-runtime-has-no-shell lands (that chain adds denyWrite for the critic set and the scratch allowWrite): extend denyWrite to every role's read roots minus write roots; settings test; live probe with an implementer record from the candidate engine.
- OpenedAt: 2026-09-06T19:18:35Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-06T19:18:35Z C25MKZCYCMWBBM8HVQADXX3RA2-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=claude-implementer-read-roots-writable
Integrity: sha256=502eb2d0bc1032b7ddb92305f445493de7b5ad6c13b7eb8431c68fcf330955eb
