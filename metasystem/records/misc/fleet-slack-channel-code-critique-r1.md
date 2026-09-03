# Fleet conversation channel — code review round 1 (job fsc-code-crit1, Fable)

Reviewed: build fsc-build-2 commit a3929f3e against design revision 4. Material: 8. Gaps named by the reviewer: (1) no computed diff artifact, read from the worktree at the commit; (2) the strict token for the stop/resume and set-obligation consumers was undecided — decided below under F-3. Dispositions by m0b, folded in the fix build under plans/fleet-slack-channel-fix-brief.md.

## F-1 (material, critical)

A crash inside the RECORDED goal transaction permanently wedges the question and stops
all receiving. In metasystem/internal/channel/poll.go the function advanceAnswer re-runs
the answer verb with the operation id persisted at MATCHED, but the transaction engine's
Publish refuses a replayed operation id whose journal entry is not terminal-confirmed,
and journal recovery cannot rebuild an answer entry (the answer intent stores no
arguments and the rebuild switch in metasystem/internal/goal/recover.go has no answer
case), so it terminalizes as rejected. From then on every poll pass hits this question
in the resume loop and returns the error before Receive. One-hunk fix: goal.Answer in
metasystem/internal/goal/verbs.go journals a complete intent (question id, text, wants
token, provider, user, reference, step) and requestForEntry gains an answer case. Test
obligation: TestPollCrashRecoveryExactlyOnce adds a failure point between the ledger
commit and the phase write; today each injected crash fires only after the phase's
durable write, so the repeated-operation-id no-op is never exercised.

Evidence: poll.go lines 85-96 return on any advanceAnswer error before Receive; txn.go
line 542 refuses a replay with a non-confirmed existing entry; recover.go lines 236-346
rebuild only the listed verbs and reject the rest; verbs.go line 89 builds the answer
intent with no Args; channel_test.go lines 236-274 inject crashes only at the four post-
write points.

Disposition: FOLD as proposed: goal.Answer journals the complete intent; recover.go
gains the answer case; the crash test adds the point between ledger commit and phase
write.

## F-2 (material, high)

A reply containing a newline is accepted, consumes the TOTP step, and then fails the
ledger, producing the same permanent wedge as F-1. SplitTOTP in
metasystem/internal/channel/totp.go keeps interior newlines in the answer text;
goal.Answer puts that text verbatim into the history reason and the next-step line, both
rendered as single lines, and the parser reports an unparseable line so ValidateCommit
refuses the commit. Slack mobile inserts newlines on Enter. One-hunk fix: SplitTOTP
returns the answer as the space-joined whitespace-split fields before the code. Test:
TestPollRecordsAuthenticatedReply with a reply whose text contains a newline.

Evidence: totp.go lines 52-66 trim only the ends; verbs.go lines 85-110 store text raw
in Reason and NextStep; file.go lines 706 and 1003 render both as one line; file.go
lines 257-259 treat a stray line as a problem; validate.go lines 410-427 refuse the
commit on problems.

Disposition: FOLD as proposed: SplitTOTP joins the answer's whitespace-split fields with
single spaces.

## F-3 (material, high)

Any authenticated answer on a goal becomes a permanent, reusable bearer credential for
both goal resume and goal set-obligation. AuthenticatedChannelApproval in
metasystem/internal/goal/verbs.go checks only that the operation is an answer with the
channel outcome on the same goal; it does not check that the text answers the act's
question as the design requires, and the channel proof class skips the repeated-act
refusal that the temporary relay path enforces, so one answer to a budget question
authorizes unlimited resumes with arbitrary budgets. One-hunk fix:
AuthenticatedChannelApproval takes the act's strict token and requires it in the
operation's reason, and refuses an operation already consumed by that act. Test
obligation: TestAuthenticatedChannelAuthorityAfterTemporaryHorizon asserts that an
answer to a budget-above-norm question is refused by resume.

Evidence: verbs.go lines 58-77 match on opid, verb answer, and outcome only; stop.go
lines 384-388 and verbs.go lines 673-677 run repeatedRelayedActError only when
TemporaryResumeFor or TemporarySetObligationFor is true, which authority.go lines
128-136 and 159-161 make false for the channel class; design section 5 says the consumer
validates an operation 'whose text answers the question'.

Disposition: FOLD, token decided by m0b (design lane): the consumer requires the act's
strict token in the op's reason and refuses an op id a resume or set-obligation on that
goal already consumed. Tokens: resume `goal=<id> resume elapsed=<d> attempts=<n>
minutes=<n> active=<n>`; set-obligation `goal=<id> set-obligation state=<s> owner=<o>`;
`ask --wants` for kind stop carries the resume token.

## F-4 (material, medium)

A receipt post that keeps failing for one question (for example an archived or deleted
thread) aborts every poll pass before Receive until someone runs channel close. In
metasystem/internal/channel/poll.go the resume loop returns the error from
advanceAnswer, whose recorded-phase branch returns the Post error, whereas the
undelivered-root loop counts the failure and continues. One-hunk fix: in the resume loop
count the failure as undelivered and continue to the next question.

Evidence: poll.go lines 85-96 return on error; lines 239-245 return the Post error after
incrementing undelivered; lines 71-83 handle the same failure for root posts by
counting.

Disposition: FOLD as proposed: the resume loop counts a receipt post failure as
undelivered and continues.

## F-5 (material, medium)

TestPollAtomicallyConsumesTOTP in metasystem/internal/channel/channel_test.go is a one-
line alias of TestTOTPResumeExceptionIsEnvelopeScoped and never runs a poll; the
design's obligation 'two polls, one step, one attribution' is uncertified. Test
obligation: two Poll calls each delivering a reply from the human user carrying the same
valid code on different message references, asserting one answer operation and one
'replayed code' rejection.

Evidence: channel_test.go line 83 defines the test as a call to the other test; design
section 8 names the obligation.

Disposition: FOLD as a real test: two polls, one step, one attribution, one replay
rejection.

## F-6 (material, medium)

TestReportComposesFromLedgerJobsAndLandings in
metasystem/internal/channel/channel_test.go composes from an empty temporary directory
and asserts only the header line and the undelivered line; the design requires a fixture
ledger, two jobs, and three trailered commits with exact text, so the Landed, Under way,
Planned, and Spend sections of report.go are uncertified. Test obligation: build the
fixture and assert the exact text.

Evidence: channel_test.go lines 151-156; report.go lines 25-142 contain the untested
composition; design section 8 names the inputs.

Disposition: FOLD as a real test: fixture ledger, two jobs, three trailered commits,
exact text.

## F-7 (material, medium)

TestInboundCheckpointSurvivesCrashAndDeduplicates in
metasystem/internal/channel/channel_test.go contains no crash, no valid reply, and no
deduplication assertion: it delivers one stray envelope and checks the cursor file
exists. Test obligation per design section 8: an unmatched update before a valid reply,
a crash before the cursor write, and a re-poll that attributes once and advances the
cursor.

Evidence: channel_test.go lines 175-186; design section 8 names the scenario.

Disposition: FOLD as a real test: unmatched update, valid reply, crash before the cursor
write, re-poll attributes once and advances the cursor.

## F-9 (material, medium)

Poll in metasystem/internal/channel/poll.go attributes a reply when the configured TOTP
secret or human user id is empty. A blank value in the local configuration file passes
the secret loader as an empty string, and VerifyTOTP with an empty secret decodes an
empty key and computes a public code sequence, so authentication degenerates to the user
id alone, which design decision D1 rejects. One-hunk fix: Poll refuses attribution
(reason 'unconfigured', no consumption) when either value is empty.

Evidence: phase.go lines 25-37 and channel_verbs.go lines 29-41 return a present blank
value from ConfLookup; totp.go lines 20-33 accept an empty secret; poll.go lines 153-159
compare against the possibly empty id.

Disposition: FOLD as proposed: Poll refuses attribution without consumption when the
secret or the human id is empty.

## F-10 (note, low)

TestCredentialIsTokenIdentity in channel_test.go exercises the in-test provider stub,
not the adapter; the real proof is TestCredential in the slack package against the fake.

Evidence: channel_test.go lines 90-96; slack_test.go lines 90-96.

Disposition: NOTE, no change: TestCredential in the slack package is the adapter proof.

## F-11 (note, low)

TestSecretsScrubbedFromErrors tests only the Scrub helper on a literal string; by
reading, the Slack adapter scrubs request, transport, and provider-error strings with
the destination secrets and token, so the behavior is present but not certified by that
test.

Evidence: channel_test.go lines 84-89; slack.go lines 43, 49, 56.

Disposition: NOTE, folded cheaply: the fix build asserts the token absent from one
adapter error.

## F-12 (note, low)

matchedRefs in Poll is built once per pass and not updated as envelopes are matched, so
a message reference duplicated inside one Receive batch would be attributed twice with
two operation ids; neither the Slack adapter nor the fake produces duplicates within a
batch.

Evidence: poll.go lines 99-102 and 124-127 versus 185-201.

Disposition: NOTE, no change: neither provider duplicates a ref within a batch.

## F-13 (note, low)

A rejection whose post failed is journaled with posted true and never retried, and still
counts toward the three-post cap.

Evidence: poll.go lines 170-183.

Disposition: NOTE, no change.

## F-14 (note, low)

Ask discards the result of the goal ask transaction, so the ASKED next-step line can be
silently missing while the question record and thread exist.

Evidence: question.go lines 181-189.

Disposition: NOTE, folded cheaply: Ask returns the transaction error.
