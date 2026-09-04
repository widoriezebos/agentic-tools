Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal channel-poll-refuses-legacy-budget-questions)
Date: 2026-09-04

# Build brief: a legacy budget question loads; the tuple is required only at ask time

Goal: channel-poll-refuses-legacy-budget-questions (Wido, approved
2026-09-04 "approve channel-poll-refuses-legacy-budget-questions for
me"; tier 2, box 4h/6/720m/1/2). Its DONE, verbatim: "the tuple is
required only when a question is opened; loading tolerates a budget
question without one (closed or open; it simply cannot raise a box),
and a test pins that a legacy record loads."

Why: since landing 3615da7a, `validateQuestionBudget` runs in
ReadQuestion (internal/channel/question.go:113) and in listQuestions
(question.go:130) as well as in Ask (question.go:190). Every machine
with a budget-above-norm question recorded before that landing (m2 has
two, m3 has one: closed, no `budget` field) now fails every channel
verb with "a budget-above-norm question requires a complete proposed
budget tuple". The fleet channel is unread on both machines.

# Workspace

Existing files only:
- internal/channel/question.go
- internal/channel/poll.go
- internal/channel/channel_test.go

# Mandate

1. In question.go, keep `validateQuestionBudget` exactly as it is for
   Ask (question.go:190): a budget-above-norm ask without a complete
   valid tuple is refused, and a non-budget ask with a tuple is refused
   (TestBudgetQuestionRequiresPersistsAndRendersCompleteTuple stays
   green unchanged). Remove the call from ReadQuestion and from
   listQuestions. Loading a record never validates the tuple.
2. In poll.go at the budget re-approval branch (poll.go:358-371): when
   `q.Kind == "budget-above-norm"` and the text equals the token but
   `q.Budget == nil`, do not call goal.Approve; set
   `a.Receipt = "recorded: " + q.Goal + " has no proposed box on this question; nothing raised"`
   and continue the normal receipt path. A legacy budget question thus
   answers and closes, and raises nothing.
3. Tests in channel_test.go, in-package, using the existing fakes:
   (a) TestLegacyBudgetQuestionLoadsWithoutTuple: write a
   budget-above-norm question file by hand with no `budget` field
   (state "closed") into the question directory of a temp root, plus
   one ordinary open question; ReadQuestion of the legacy id succeeds
   with Budget nil, and listQuestions (through whatever exported path
   lists open questions for Poll, or the unexported function directly)
   returns without error and includes the open question.
   (b) TestLegacyBudgetQuestionAnswerRaisesNothing: an open
   budget-above-norm question with no tuple receives a verified token
   answer through Poll; the answer is recorded, the question closes,
   no approve row appears on the goal, and the receipt contains
   "nothing raised". Model it on
   TestBudgetFreeTextAndOtherQuestionDoNotRaiseBox (channel_test.go:972).
4. Run `go test ./internal/channel/...` green. Nothing outside the
   three files changes; no new files.

# Constraints

Wall-clock budget: 25 minutes. Return per the implementer schema with
the `decisions` list. Do not run the full suite or the fixture script.

# Gap Rule

Stop only on a law-changing gap (a mandate that cannot be met without
changing what the goal's DONE says); record a mechanical choice in
`decisions` and build on.
