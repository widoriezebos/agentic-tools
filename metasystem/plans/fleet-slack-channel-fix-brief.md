Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fleet-slack-channel)
Date: 2026-09-03

# Goal

The fix build of the fleet conversation channel, folding the one code
review of tier 3 (R-54-m1). Your branch starts at origin/main; first
cherry-pick the build commit a3929f3e ("Build the authenticated fleet
conversation channel", 21 files, job fsc-build-2; reachable by hash in
this repository). Precondition: it applies cleanly and `git diff HEAD~1
--stat` lists exactly its 21 files; otherwise stop and report. Then ONE
fix commit on top, touching only what the findings below name. The
findings and the orchestrator's dispositions are landed in
metasystem/records/misc/fleet-slack-channel-code-critique-r1.md; the design
is metasystem/plans/fleet-slack-channel-design.md revision 4. Under
R-60-m1 there is no further review: your commit lands through the chain
after the orchestrator's host gate, so every fold must be exact.

# The folds (each names its file, function and test)

1. F-1, crash inside the RECORDED transaction. `goal.Answer` in
   internal/goal/verbs.go journals a COMPLETE intent (question id, text,
   wants token, provider, user id, message ref, step) and
   `requestForEntry` in internal/goal/recover.go gains an `answer` case
   rebuilding that request, so a replayed op id after a crash inside the
   transaction recovers to the same operation and lands once. Test:
   `TestPollCrashRecoveryExactlyOnce` adds an injected failure point
   between the ledger commit and the phase write (today every point fires
   after the phase's durable write); after re-poll: one history op, one
   ANSWERED line, the ledger parses, thread closed, cursor advanced.
2. F-2, newlines. `SplitTOTP` in internal/channel/totp.go returns the
   answer as its whitespace-split fields joined by single spaces (the
   code is the last field). Test: `TestPollRecordsAuthenticatedReply`
   with a reply whose text contains a newline; the history line and the
   next-step line stay single lines and the commit validates.
3. F-3, the answer as a bearer credential. `AuthenticatedChannelApproval`
   in internal/goal/verbs.go takes the act's strict token and requires it
   in the operation's reason as a contiguous run of whitespace fields
   (the `StrictApprovalTriple` idiom in internal/goal/norm.go), and
   refuses an operation a `resume` or `set-obligation` on the same goal
   already consumed (the consuming history line records the op id under
   a key `approvedRef`; the refusal scans for it). The tokens, decided by
   the orchestrator: for resume `goal=<id> resume elapsed=<elapsedLimit>
   attempts=<attemptLimit> minutes=<reservedJobMinutesLimit>
   active=<activeJobLimit>`, every value equal to the tuple the act
   installs, rendered as the budget's intent args render them; for
   set-obligation `goal=<id> set-obligation state=<state> owner=<owner>`.
   `channel ask` builds the `stop` kind's `--wants` from that resume form
   (the asker passes the proposed tuple) and the question text tells Wido
   to reply with it verbatim plus his code. Tests:
   `TestAuthenticatedChannelAuthorityAfterTemporaryHorizon` asserts that
   an answer to a budget-above-norm question is REFUSED by resume, that an
   answer carrying the resume token is accepted once and refused the
   second time, and the same for set-obligation with its token.
4. F-4, a failing receipt post. In `Poll` (internal/channel/poll.go) the
   resume loop counts an `advanceAnswer` post failure as undelivered and
   continues to the next question and to Receive, as the undelivered-root
   loop already does; a ledger error still returns. Test:
   `TestPollRecordsAuthenticatedReply` gains a case with the receipt post
   failing once: the pass still receives, the next pass receipts.
5. F-9, empty secret or human id. `Poll` refuses attribution with reason
   `unconfigured` and no consumption row when `channel.human.totp-secret`
   or `channel.human.<provider>.user-id` resolves empty (present but
   blank counts as empty). Test: `TestPollRejectsWrongUserNoCodeBadCodeReplay`
   gains the empty-secret and empty-id cases: no ledger op, no
   consumption row, one rejection post.
6. F-5, `TestPollAtomicallyConsumesTOTP` becomes a real test: two `Poll`
   calls, each delivering a reply from the human user with the SAME valid
   code on different message refs; exactly one answer operation and one
   "replayed code" rejection; the consumption file holds one row.
7. F-6, `TestReportComposesFromLedgerJobsAndLandings` builds the design's
   fixture (a ledger with a goal claimed by the machine, two job records,
   three commits with `Goal-Item:` trailers, usage facts for one runtime)
   and asserts the EXACT report text, section by section.
8. F-7, `TestInboundCheckpointSurvivesCrashAndDeduplicates` becomes the
   design's scenario: one unmatched update, then a valid reply, a crash
   injected before the cursor write, a re-poll that attributes once and
   advances the cursor; the unmatched journal holds one line.
9. Cheap notes, folded: F-11 `TestSecretsScrubbedFromErrors` also asserts
   the token absent from one Slack adapter error produced against the
   fake; F-14 `Ask` in internal/channel/question.go returns the goal
   transaction's error instead of discarding it.

Nothing else changes. No refusal weakened, no assertion loosened, no
sleep for ordering (R-35), no benchmarks (R-31).

# Gate

gofmt, go vet, go build; go test -count=1 over internal/channel/...,
internal/goal, internal/humanauthority, internal/governance,
internal/steward and cmd/metasystem green; `bash -n` and one run of the
channel fixture script the build commit adds under scripts/agents (it is
not on main yet) and of metasystem/scripts/agents/goal-cli-fixtures.sh
there. The repository-wide run's
known sandbox failure (TestHolderProbeUnreadableArgvIsNeverDead) is not
yours. Paste the final lines. The diffBoundary of the fix commit is the
files named above and nothing else.

# Constraints

Wall-clock budget: 60 minutes. Version-2 implementer JSON.

# Gap Rule

stop and report a gap; never fill it silently.
