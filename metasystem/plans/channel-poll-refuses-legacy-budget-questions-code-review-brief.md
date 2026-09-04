Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal channel-poll-refuses-legacy-budget-questions)
Date: 2026-09-04

# Goal

The one code review (tier 2, box 4h/6/720m/1/2) of the legacy budget
question fix, implementer job lbq-build-1 (gpt-5.6-sol), against its
brief metasystem/plans/channel-poll-refuses-legacy-budget-questions-build-brief.md
and the goal's DONE, verbatim: "the tuple is required only when a
question is opened; loading tolerates a budget question without one
(closed or open; it simply cannot raise a box), and a test pins that a
legacy record loads." The computed diff and reviewedTree are the
conformance artefacts of that job
(artifacts/agents/lbq-build-1/rounds/<n>/diff.patch and review.json,
n=1 for this chain; reviewedTree
bc31345dd2e8b3a76022b706d12e57f6d2cc0fe6);
review that diff, never the delegate's own summary. Three files:
internal/channel/question.go, internal/channel/poll.go,
internal/channel/channel_test.go.

# Review brief

Two ordered layers per the code-critique skill. LAYER 1, conformance:
validateQuestionBudget still runs in Ask (question.go) with its body
unchanged, and TestBudgetQuestionRequiresPersistsAndRendersCompleteTuple
is untouched and green; the call is gone from ReadQuestion and from
listQuestions and from nowhere else; in poll.go the budget re-approval
branch skips goal.Approve exactly when `q.Budget == nil` with the
receipt `"recorded: " + q.Goal + " has no proposed box on this question; nothing raised"`
and the ordinary receipt path continues; the two obligation tests
TestLegacyBudgetQuestionLoadsWithoutTuple and
TestLegacyBudgetQuestionAnswerRaisesNothing are present and
discriminating (each fails against the pre-fix tree: name the
assertion that would fail); nothing outside the three files; no test
weakened. LAYER 2, adversarial: a legacy question with a PARTIAL tuple
(a `budget` object missing a field) — does loading tolerate it and
does the answer path dereference safely or refuse by name; the nil
check against `renderProposedBox(*q.Budget)` and every other
dereference of q.Budget in poll.go and question.go; an open legacy
budget question whose token answer now closes without a box raise —
the receipt text is the only word the human gets, judge whether it
says so plainly; the answer phases "matched" -> "recorded" ->
receipt are unchanged for the tuple-bearing case (m2's
channel-budget-answer-binds-nothing path, TestBudget* tests all
green).

Materiality criterion, verbatim: would the change ship a defect,
violate its brief, or damage what certifies it? Count only material
findings in the verdict. This is the ONE code review of a tier 2
goal; material findings return to the implementer as one correction
round.

Run what the sandbox allows: `go build ./...`, `go vet
./internal/channel/...`, `go test ./internal/channel/... -count=1`.
Report what could not run.

# Constraints

Wall-clock budget: 20 minutes. Return per the code-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
