# Unit U0 correction 1: the fold is built with plumbing, so the real lane checkout's pre-commit guard never sees it

Working Mode: Implement. Correction of U0 (brief `plans/lane-recovers-and-pauses-u0-records-only-main-moves.md`) after its Opus read (RETURN, 2 material). Worktree uncommitted; change only what this brief names; leave fixes uncommitted; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json.

## F-1 (material, reproduced by the reader): the fold commit is refused in the real lane checkout
`internal/landing/plain/main_move.go` ~104-113 builds the fold with porcelain (`git merge --no-ff --no-commit` then `git commit --no-edit`). A porcelain commit runs the checkout's pre-commit hook; the real lane checkout's hook runs `metasystem internal pre-commit`, which refuses on a detached HEAD ("an agent commits here only through metasystem work land (HEAD names no branch)", landpath/precommit.go ~133) and, for every committer class, refuses any staged `plans/goals/` path (the ledger fence, ~140-153) — and the fold stages exactly those paths. Every test bed (`newBed`, `newBatchVerbBed`, the fake-git fold test) is a repo WITHOUT the hook, so no test saw it. Fix: build the fold with plumbing, never a porcelain commit: `git merge-tree --write-tree <batch> <main>` gives the tree (a conflict → refusal, nothing touched); compare it strictly with the proof-neutral check (`ledgerOnlyBetween`); `git commit-tree <tree> -p <batch> -p <main>` with the fold message; then `git checkout --detach <commit>` (or update the ref the lane uses) — the goal verbs already publish through plumbing that skips the hook (precommit.go ~141-142). No merge state is ever left behind.
Test (real shape): a test bed that INSTALLS the real guard hook (copy the engine's `internal pre-commit` wiring as the live lane checkout has it: `.git/hooks/pre-commit` running `metasystem internal pre-commit`) and pushes after a records-only move: the fold lands; mutation: porcelain commit → the hook refuses → the test fails. Keep the fake-git test too.

## F-2 (material): a failed fold left the checkout mid-merge
Removed by F-1 (plumbing never starts a merge). Add the assertion to the hook test: after a refused fold nothing is staged and `MERGE_HEAD` does not exist.

## Non-material folds (cheap)
- N-1 `main_move.go` ~79: set the kind after the per-commit loop so a `diff-tree` error never reports records-only in status.
- N-2 stop the classification loop at the first code commit.
- N-3 skill step 3: say plainly that a freshly prepared batch with nothing merged yet merges its members (cases 1 and 2) whatever `main_move` says; "continue the current step" applies to a step in progress.
- N-4 main with no first-parent link to HEAD (rewritten main) returns a Refusal (as today's push does), not a bare error.

## Checks
`go vet ./...`; `go test -count=1 -timeout 30m ./internal/landing/plain ./internal/landing/landpath ./internal/goal/branch ./internal/layering`; `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestLanding|TestSkill|TestAudit|TestLaneMerge|TestLandingPush|TestMainMove'`; `go run ./cmd/devgate static`. Report: diff --stat, production line count (≤250), every exit, the hook test's mutation.
