# lane-housekeeping-2 unit 3: the engine's own commands run under its PATH

Working Mode: Implement. Goal lane-housekeeping-2, design brief `plans/lane-housekeeping-2-design-brief.md` (read it first). Worktree = this checkout, off origin/main. At most 250 production lines. Leave everything uncommitted; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json by hand. Tests never READ live state that is not about the real contract, and never copy testing.json, go.mod, metasystem.conf or ledger pages into testdata (a test that certifies the real contract reads the LIVE files; fixtures hold small synthetic shapes or one saved record). Run checks sequentially with -p 4. Report: diff --stat, production line count, every exit, each mutation.

## Problem (finding 117, 2026-10-11)
`work commit`'s committed cheap check (`metasystem test impact`) failed with "command not found" because the caller's shell had no `metasystem` on PATH, while the proof runner prepends the running engine's directory to PATH for declared commands.

## Build
Every place the engine runs a DECLARED shell command (the cheap check of `work commit`, the resolution/fix job's `--check`, the keeper's static step, the classify replay, anything that shells out a `metasystem …` command from testing.json or a fix record) prepends the running engine's own directory (`filepath.Dir(os.Executable())`) to PATH the way the proof runner does, through ONE shared helper (find the proof runner's version and reuse it; no second copy). A declared command never fails for want of the caller's PATH. The helper is covered by a test that runs a declared `metasystem` command from a shell whose PATH lacks the engine (mutation: drop the prefix → "command not found" → test fails). Audit every shell-out site with grep (exec.Command with "sh" or "/bin/sh", LaunchDetached, runCheck) and list them in the report with "uses the helper: yes/no (why)".

## Checks
`go vet ./cmd/metasystem ./internal/landing/plain ./internal/launch ./internal/gaterun`; `go test -p 4 -count=1 -timeout 30m ./internal/landing/plain ./internal/launch ./internal/gaterun ./internal/layering`; `go test -p 4 -count=1 -timeout 30m ./cmd/metasystem -run 'TestWorkCommit|TestLanding|TestLaneFix|TestAudit'`; `go run ./cmd/devgate static`.
