Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal status-post-next-in-line-binds)
Date: 2026-09-04

# Build brief: carry the status-post binding onto the new main

Goal `status-post-next-in-line-binds` (tier 1, approved under R-76-m2; this is the box's last attempt). No critic in this chain.

## What happened

Chain spn-build2 built the change over two rounds (eleven files: the status post's decision line only for the seat's pinned-and-labelled next pick, the exact start token, the poll's disposition of a code-verified token reply in the post's thread as execution approval, and the verified-channel-answer proof class in the goal package, documented). While it built, another machine landed the risk-basis change on main (35 engine and command files), so the diff no longer applies: `internal/goal/file.go` conflicts around line 512. The work is preserved on branch `preserve/spn-build2-r2`.

## What to do

In your worktree run `git cherry-pick --no-commit preserve/spn-build2-r2`, resolve the conflict in `internal/goal/file.go` so both the risk-basis change and the verified-channel-answer recording survive, confirm the diff against HEAD is those eleven files only, then `go build ./...`, `gofmt -l` and `go vet` on the touched packages, `go run honnef.co/go/tools/cmd/staticcheck@2025.1` on them, and `go test ./internal/channel/... ./internal/humanauthority/... ./internal/governance/... ./cmd/metasystem/`. For `./internal/goal/` run `go test -timeout 30m ./internal/goal/` and report its wall time in the return: seat-side that package hit a ten-minute limit and the orchestrator is measuring whether that is inherent. Change nothing else. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.
