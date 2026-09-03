Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fleet-slack-channel)
Date: 2026-09-03

# Goal

In-package tests for the fleet conversation channel's goal and governance
code, so the landing boundary admits the chain. This is a follow-up round
of your fix build in the same worktree: HEAD is the build commit 07c2934b
(the cherry-pick of a3929f3e) and your fix sits in the WORKING TREE,
uncommitted and unstaged, in 13 modified files (`git status --short`
lists them as modified). Leave them exactly as they are: do not stage,
revert or alter any of those bytes; product code does not change in this
round.

The commit wrapper's coverage boundary judges every staged Go package
against its floor in scripts/agents/coverage-ratchet-linux.json:
internal/goal stands at 78.7 percent against a floor of 80.0 and
internal/governance at 95.8 against 100.0, because the new functions are
exercised only from the tests in internal/channel. Lowering a floor is a
governance change and is not yours. Add tests inside those two packages.

# The uncovered code (go tool cover -func on the fix tree)

internal/goal/verbs.go: `ResumeApprovalToken`, `SetObligationApprovalToken`,
`Asked`, `AuthenticatedChannelApproval`, `containsContiguousFields`,
`Answer`, `answerRequest` and the `Error` method near line 202, all at
zero; internal/goal/recover.go: the `answer` case of `requestForEntry`;
internal/goal/file.go: the `approvedRef=` branches of `ParseHistoryLine`
(accepted only on `resume` and `set-obligation`, refused elsewhere) and
its renderer; internal/goal/stop.go: the resume history line carrying
`ApprovedRef`. internal/governance/types.go:
`RecordedChannelAuthority.ValidateRecorded` at zero.

# The tests, by name (each asserts what its name says)

In one new test file in internal/goal (name it for the channel authority),
using the package's own helpers (`verbReq`, `seedLedger`, `testBudget` in
internal/goal/verbs_test.go; the proof helper in
internal/goal/temporary_authority_test.go):

1. `TestApprovalTokensRenderTheInstalledTuple`: `ResumeApprovalToken`
   renders `goal=<id> resume elapsed=... attempts=... minutes=... active=...`
   with the values `budgetIntentArgs` renders for that budget, and
   `SetObligationApprovalToken` renders `goal=<id> set-obligation
   state=<state> owner=<owner>`, both compared as exact strings.
2. `TestContainsContiguousFieldsMatchesExactlyOnce`: true for one
   contiguous run across arbitrary whitespace; false for zero matches, for
   two matches, for the fields split by another field, and for an empty
   token.
3. `TestAskedAppendsTheAskedMarkerOnce`: on an empty next step the marker
   becomes the next step; on a filled next step it is appended after `; `;
   the same op id replayed is a no-op (one history line); a goal that is
   not live is refused by name.
4. `TestAnswerRecordsAuthenticatedChannelWordOnce`: `Answer` refuses an
   incomplete proof and blank text; a complete answer lands one history
   line with actor `human:wido`, outcome `AUTHENTICATED_CHANNEL_WORD`, the
   four channel keys, the text in the reason and the `wants` token
   appended when the text does not already contain it; the next step
   carries the ANSWERED marker; the same op id replayed is a no-op.
5. `TestAuthenticatedChannelApprovalRequiresTheTokenOnce`: refused for a
   goal that is not live, for an op id that is not an answer, for an answer
   whose reason lacks the strict token; accepted (the returned tuple equal
   to the recorded proof) for an answer carrying it; refused once a
   `resume` or `set-obligation` history line records that op id under
   `approvedRef`, and the consuming `Resume` and `SetObligation` verbs
   write that key (the existing consumers in internal/goal, exercised
   through `AuthenticatedChannelApproval` after each).
6. `TestHistoryLineApprovedRefRoundTripsOnConsumersOnly`: a `resume` and a
   `set-obligation` line with `ApprovedRef` survive `RenderFile` then
   `ParseHistoryLine` unchanged; the same key on any other verb is refused
   by the parser, and the error names the key.
7. `TestRecoveryRebuildsAnswer` in internal/goal/recover_test.go, in that
   file's idiom: a journaled `answer` intent with the seven args
   (question, text, wants, provider, user, ref, step) is rebuilt by
   `requestForEntry` into the same operation, recovery lands it once, and
   the ledger parses.

In internal/governance/types_test.go:

8. `TestRecordedChannelAuthorityRequiresACompleteWireTuple`: the complete
   tuple validates; a wrong outcome, an empty provider, user id or message
   ref, and a step below one are each refused.

Cover the `Error` method by asserting its exact string where the type is
constructed. If reaching 80.0 needs one more test, add it beside these,
named for what it asserts, on the same files; never touch product code.

# Gate

gofmt on the test files, go vet, then go test -count=1 for internal/goal
and internal/governance green, then for each of the two packages
`go test -coverprofile` and `go tool cover -func`; paste the two total
lines. Required: internal/goal at or above 80.0 percent, internal/governance
at 100.0. Leave the new and changed test files in the working tree and
STOP: stage nothing and do not run the commit wrapper (the fix build's
lease refusal repeats here; the orchestrator reads the working tree and
carries the bytes through the chain). The diffBoundary is the test files
named above and nothing else; `git status --short` must show only the 13
fix paths as modified plus your test files (modified or untracked).

# Constraints

Wall-clock budget: 30 minutes. The previous round stopped at the
precondition (it expected the fix staged); this round's precondition is
the working tree as described. Version-2 implementer JSON. No assertion
loosened, no sleep for ordering (R-35), no benchmarks (R-31).

# Gap Rule

stop and report a gap; never fill it silently.
