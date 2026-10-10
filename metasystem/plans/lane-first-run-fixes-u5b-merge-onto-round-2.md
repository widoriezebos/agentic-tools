# Fix round 2: merge unit u5b onto the integration tree

Worktree: `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e-fixround2b/metasystem` (branch `fix/round-2b-20261010`).
A merge is IN PROGRESS there: `MERGE_HEAD` = 382dc9431 (branch `fix/lane-resolves-conflicts-20261010`, unit u5b "the landing agent resolves a merge conflict as the seat", built on 6908395a3).
`HEAD` = 32a1162f8 = main + five fix branches; among them `fix/lane-acts-as-seat-20261010` whose units u5c (d0f54ef6a) and u5d (fd5f6983b) also build on 6908395a3 and changed the same files.

## What each side added (both must survive whole)

HEAD (u5c + u5d): the lane fix round's engine side. Fix record `fixes/<attempt>.json` with attempt, round, parent, verdict; batch custody for member goals at lane-fix call sites only; `CheckBatch` admits the recorded repair commit; the attempt-keyed checkpoint is retained after a red; the fix record follows main-refresh merges; `readFixRecords(install, closeStale, goals...)` may close stale attempts only while the caller holds the lane lock; readers never write; the guard's rule: a `Goal-Unit: <member>/lane-fix-N` trailer must name the unit the fix record expects (`fix.Unit`).

THEIRS (u5b): conflict resolution as the seat. The record also carries the conflict (paths by class as `[]conflict.Path`, both sides' tips, reason, brief, declared generated paths); `ReadFix(install)` exported, `readFix(install, includeClosed)`; the "abandoned" state (resolving record whose MERGE_HEAD vanished) bound to the current batch; the guard admits `Goal-Unit: <member>/lane-merge-N` for a merge of a batch member only and refuses a lane-fix trailer while a merge is pending; the resolution check runs with `LANDING_PROOF_BASE` outside the queue lock; the read's checkout is the merge commit.

## Conflicted paths (git diff --name-only --diff-filter=U)

- `cmd/metasystem/landing_fix_test.go` (one hunk: `plain.Running` literal; take the union of fields, `Tree: "batch-tree"` and the attempt the surrounding assertions expect; run the test)
- `internal/landing/landpath/lane_fix_test.go` (one hunk, 46 lines: keep BOTH sides' test cases)
- `internal/landing/landpath/precommit.go` (one hunk: the regexp admits `lane-(fix|merge)-N`; HEAD's `fix.Unit` match applies to lane-fix commits; THEIRS' merge rules stay)
- `internal/landing/plain/fix.go` (five hunks: imports; the `Fix` struct takes the union of both sides' fields; one reader family serving every caller on both sides — keep HEAD's goal filtering and close-stale-under-lock AND THEIRS' includeClosed/abandoned semantics and the exported `ReadFix`; the 128-line HEAD block at the last hunk and the 12-line THEIRS block are both kept)
- `internal/landing/plain/fix_test.go` (one hunk: imports; keep every test)

## Rules

- Keep both sides whole. Never drop a test, a field, a function or a rule. Where both sides changed one function's signature, make one function that serves every caller on both sides; update callers, not behaviour.
- Never edit `testing.json`. Never open any `metasystem.conf.local`. Never push.
- No new behaviour beyond what the two sides already have.

## Checks (all exit codes in the report)

1. `gofmt -l ./cmd ./internal` prints nothing.
2. `go build ./...`
3. `go vet ./internal/landing/... ./internal/launch ./cmd/metasystem`
4. `go test -count=1 -timeout 30m ./internal/landing/... ./internal/launch ./internal/conflict`
5. `go test -count=1 -timeout 30m ./cmd/metasystem -run 'Landing|Lane|Fix|Resolve|Merge|Batch|Land|Status|Review|Guard|Precommit'` (every test whose name touches a changed seam)

## Commit

Exactly one commit: the merge, with git's default message (`git commit --no-edit`). Nothing else is committed.

## Report

Per conflicted file 1-3 lines on what was kept from each side and how signatures were unified; the five check exit codes; the merge commit sha (`git rev-parse HEAD`).
