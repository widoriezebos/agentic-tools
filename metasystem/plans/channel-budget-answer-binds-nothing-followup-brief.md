Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal channel-budget-answer-binds-nothing)
Date: 2026-09-04

# Follow-up: persist the question's tuple; the boundary widens to the ask verb and the question record

Decision on your gap: the brief's prerequisite was wrong, and the boundary widens to fix it. `cmd/metasystem/channel_verbs.go` accepts the five tuple flags on `channel ask` but drops them; `internal/channel/question.go` has no field for them. Add the tuple to the question record (a `Budget` with the five limits, present only for `budget-above-norm` questions; validate that a budget question carries all five, refuse otherwise), have the ask verb store it, render it in the question text as one line ("Proposed box: 2h, 5 attempts, 600 reserved minutes, 1 active job, 0 review rounds"), and then implement the disposition as the build brief says, passing that budget to the goal package's approval with the verified-channel-answer proof class the status-post binding landing added (`internal/humanauthority`, `internal/goal/approval.go`). Everything else in the build brief stands. Every path in your return is relative to the repository root (starting with `metasystem/`).
