Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal fixture-suite-drift-after-approval-gate)
Date: 2026-09-04

# Follow-up: the serving goal must be approved and claimed

Seat-side, after your third round, the channel suite is green and the dispatch scenario now fails at the serving-goal leg with: "no serving goal to project: a converted checkout serves this machine's claimed goal, a legacy checkout its Current goal" and "no current goal to project (--serving-goal)" (exit 3). The leg's comment says the goal is deliberately open but unclaimed; in a converted checkout that is no longer a lawful serving goal. Make the leg approve the fixture-serving goal (goal approve with the fixture's tier box, `--by Wido`, a temporary human word of at least three words and `--review-by`), then claim it bare, then dispatch with `--serving-goal`; update the comment to say why (a converted checkout serves the machine's claimed goal). Keep the earlier refusal sub-leg (no usable goal refuses exit 3 without a job record) as it is, since it runs before the goal exists. Every path in your return is relative to the repository root (starting with `metasystem/`). The orchestrator reruns the suites seat-side.
