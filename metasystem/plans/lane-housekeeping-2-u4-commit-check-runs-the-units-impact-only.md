# lane-housekeeping-2 unit 4: the commit-time check of a unit runs the unit's impact plan only

Working Mode: Implement. Goal lane-housekeeping-2, design brief `plans/lane-housekeeping-2-design-brief.md` (read it first). Worktree = this checkout, off origin/main. At most 250 production lines. Leave everything uncommitted; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json by hand. Tests never READ live state that is not about the real contract, and never copy testing.json, go.mod, metasystem.conf or ledger pages into testdata (a test that certifies the real contract reads the LIVE files; fixtures hold small synthetic shapes or one saved record). Run checks sequentially with -p 4. Report: diff --stat, production line count, every exit, each mutation.

## Problem (findings 103, 118, 2026-10-11)
The committed cheap check of `work commit` ran 56 minutes of named command tests for an eight-file lane fix: it ran the whole named selection of the touched command package, where the batch proof runs the same tree in full anyway.

## Build
`work commit`'s committed check runs the UNIT's own impact selection: the named tests of the symbols the unit's diff touches (what `metasystem test impact --plan` selects for the unit's base..tree), never a package-wide or command-family run; package-wide depth stays with the batch proof. The cheap-or-skip rule of lane-speed U3 (landed today: when the selection is not cheap, static only; cmd/metasystem named is never cheap) applies here too — reuse its decision function, do not re-implement. The check record says which form ran ("impact: N tests of M packages" / "static-only: selection not cheap") and the minutes. Real shape: today's fix of eight files (the lane fix record of 2026-10-11 in the landing install, read-only at /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/fixes/) → the plan names the touched symbols' tests, not the family. Mutation: run the family → the test asserting the selection size fails.

## Checks
`go vet ./cmd/metasystem ./internal/landing/plain`; `go test -p 4 -count=1 -timeout 30m ./internal/landing/plain ./internal/landing/batch/... ./internal/layering`; `go test -p 4 -count=1 -timeout 30m ./cmd/metasystem -run 'TestWorkCommit|TestTestImpact|TestLandingCheck|TestAudit'`; `go run ./cmd/devgate static`.
