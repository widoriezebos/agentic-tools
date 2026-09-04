Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal channel-budget-answer-binds-nothing)
Date: 2026-09-04

# Build brief: a verified channel answer to a budget question re-approves the goal

Goal `channel-budget-answer-binds-nothing` (tier 1, approved under R-76-m2, Wido's word of 2026-09-04; box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## The defect

`channel ask` (`cmd/metasystem/channel_verbs.go`) accepts the proposed budget tuple (`--elapsed-limit`, `--attempt-limit`, `--reserved-job-minutes-limit`, `--active-job-limit`, `--review-round-limit`) for a `budget-above-norm` question and records it on the question. When the human answers with the token verbatim and a valid code, `internal/channel/poll.go` (around line 260) calls the goal package's `Answer`, which records the answer in the goal's history and closes the question, but nothing re-binds the goal's box: on 2026-09-04 two such answers ("Yes" to one question, "allow two more rounds" to another) left the goals at their tier box, and `goal approve` then refused a second relayed approval on the same goal ("a further approve needs freshly observed enrolled-terminal authority"), so the seat had to open an arc-split member goal each time.

## What to build

In the poll's disposition of a matched, code-verified answer to a `budget-above-norm` question whose answer text equals the token: after `Answer` succeeds, re-approve the goal with the question's tuple through the goal package's approval path (the same function `goal approve` uses), with actor this machine, `--by` the human user id recorded on the channel, the authority being the **verified channel answer** class that the status-post binding landing added to the goal package (human user id, message reference, code step, question id), and the question's tuple as the budget; the box-above-tier norm check accepts this authority the way it accepts an enrolled-terminal approval, because the code proves the human. Reuse that class; do not add another. A `budget-above-norm` question whose tuple is within the tier box needs no norm proof. An answer that is not the token, or to a question of another kind, changes nothing (as today). The refusal "a further approve needs freshly observed enrolled-terminal authority" must not fire for this authority: it guards relayed words, and a verified channel answer is not a relay. Post "recorded: <goal> box raised to <tuple>" in the thread on success; on refusal post the refusal reason.

## Verification

`go test ./internal/channel/... ./internal/goal/...` with tests: a verified token answer to a budget question raises the goal's box to the question's tuple and reopens admission; a second such answer on the same goal is accepted; a free-text answer changes nothing; a question of kind `other` changes nothing. Run `gofmt -l`, `go vet` and `go run honnef.co/go/tools/cmd/staticcheck@2025.1` on the touched packages.

## Bounds

Touch `internal/channel/poll.go`, the goal package's approval entry points and norm check only as far as the new authority kind needs, their tests, and one line in `docs/backlog-mechanism.md` where budget raises are described. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.
