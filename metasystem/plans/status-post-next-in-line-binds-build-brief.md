Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal status-post-next-in-line-binds)
Date: 2026-09-04

# Build brief: the status post's decision line names the seat's real next pick and its reply binds

Goal `status-post-next-in-line-binds` (tier 1, approved under R-76-m2, Wido's word of 2026-09-04; box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## The defect

`internal/channel/report.go` renders one "Needs you" approval line for the queued goal that sorts first (`sort.Strings` over ids), calling it "next in line"; that is the alphabet, not a pick. And a reply to a status post binds nothing: on 2026-09-04 Wido replied "approved <code>" and "Approved <code>" to two status posts; both went to the channel's unmatched file under the fleet runtime directory (`internal/channel/poll.go` files replies that match no open question there) and he was never told.

## What to build

1. The approval line appears only for a goal the seat has marked as its next pick: queued, pinned to this machine (`Pinned`) and carrying the label `next`; never by sort order. With no such goal there is no approval line. The seat sets the mark with the existing goal verbs (`goal edit --pin <machine> --label next`); document that in the one line of `docs/orchestration.md` that describes the status post.
2. The line ends with the reply form the channel uses for questions: "Reply in this thread with this token verbatim, followed by your code: start <goal id>". Give the status post's thread reference a durable record (extend `StatusState` in report.go with the goal id the line names) so the poll can match a reply in that thread.
3. In `internal/channel/poll.go`, a reply in the status thread whose text is that token and whose code verifies (use the send-time rule now in place) is the human's execution approval of that goal: call the goal package's approve the way `goal approve` by relayed word does, with the message reference and code step recorded as the word, actor this machine; on success post "recorded: <goal> approved for execution" in the thread. A reply in the status thread that is not the token, or fails the code, is answered in the thread with "not recorded: <reason>; reply with the token and your code", the way question threads already answer, and still filed as unmatched.

## Verification

`go test ./internal/channel/... ./internal/goal/...` with tests: eleven queued goals and no mark produce no approval line; a marked goal produces one line with the token; a status-thread reply with the token and a valid code approves that goal and posts the confirmation; a reply without the token is answered and filed. Run `gofmt -l`, `go vet` and `go run honnef.co/go/tools/cmd/staticcheck@2025.1 ./internal/channel/...`.

## Bounds

Touch `internal/channel/report.go`, `internal/channel/poll.go`, their tests, and one doc line. Do not change the goal package's approval law; call it. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.
