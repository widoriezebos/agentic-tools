# Sol design critique of the fleet conversation channel — round 1 (2026-09-03)

Job fsc-design-crit1 (design-critic, codex gpt-5.6-sol, cap 40) reviewed
plans/fleet-slack-channel-design.md revision 1 at main 371b9749 under
plans/fleet-slack-channel-critique-r1-brief.md. Eight material findings,
three notes; every fold is in revision 2 of the design (its §12).

## FSC-R1-001 (critical, material)

Section 5, artifacts channel.Poll, artifacts/agents/channel/totp-
consumed.json, and the per-question record: the check that a time-based one-
time-password step is unconsumed and the act that consumes it are not an
atomic, serialized operation. Concurrent manual and steward polls can both
observe the same step as unused and attribute two inbound messages to actor
human:wido. Proposed change: define one channel authentication lock and a
durable consumption row keyed by matched step and inbound reference, written
before attribution and recoverable by question identifier and operation
identifier; add TestPollAtomicallyConsumesTOTP.

Evidence: The design states only that the secret-and-step pair must not have
been consumed and names a replay file. It neither places consumption in steps
(a) through (d) nor names a lock, compare-and-swap, or recovery record, while
section 7 exposes both automatic and manual poll entry points.

Disposition (m0b): folded §5: one flock, consumption row before attribution.

## FSC-R1-002 (high, material)

Section 5, artifacts Question.state, Question.cursor, answer.opid,
channel.Poll, and TestPollRecordsAuthenticatedReply: the durable sequence is
not crash-safe. A crash after step (a) leaves state answered, which future
polls skip, so the goal history operation and thread receipt never happen; a
cursor persisted before judgment can lose the inbound message, and no phase
resumes steps (b) through (d). Proposed change: specify a resumable per-
inbound phase machine whose stable operation identifier is allocated before
the ledger write, advance the provider cursor only after the inbound
disposition is durable, and test crashes at every phase plus re-poll with
exactly one history operation.

Evidence: Section 5 says Poll visits every open question, then step (a)
changes the question to answered before the goal-history operation in step
(b). The existing goal transaction engine can make a stable operation
identifier idempotent, but the design does not define an answer verb or
recovery path that invokes it after the question ceases to be open.

Disposition (m0b): folded §5: resumable phase machine, op id allocated at
MATCHED, cursor persisted after disposition.

## FSC-R1-003 (high, material)

Section 5, artifacts goal.HistoryLine,
goal.validateRecordedTemporaryAuthority, the new answer verb, and
TestAnswerCarryingStrictTokenSatisfiesNormApproval:
authorityOutcome=AUTHENTICATED_CHANNEL_WORD does not fit the existing readable
history grammar as validated today. The strict approval token does fit
unchanged when stored in HistoryLine.Reason. Proposed change: extend the goal
history authority validator and round-trip grammar tests for the
authenticated-channel outcome and its required proof coordinates, while
keeping the answer and strict token in reason.

Evidence: ParseHistoryLine accepts the authorityOutcome key but always calls
validateRecordedTemporaryAuthority;
governance.RecordedTemporaryAuthority.ValidateRecorded rejects every nonempty
outcome except TEMPORARY_HUMAN_WORD. RecordedNormApproval already scans
HistoryLine.Reason with StrictApprovalTriple and requires actor to begin with
human:.

Disposition (m0b): folded §5: ParseHistoryLine authority validation accepts
the new outcome; token stays in reason.

## FSC-R1-004 (high, material)

Section 2, artifact Provider inbound interface: Replies(thread, after) is
insufficient for the promised later Telegram and WhatsApp adapters and
contradicts adopted alert-channel section 2b. Those transports require
destination-wide ingress ownership and a durable acknowledgment after handoff,
not independent consumption per open thread. Proposed change: add a named
acknowledge/checkpoint operation such as AcknowledgeInbound and make receive
return destination-wide inbound envelopes carrying thread correlation and an
acknowledgment token; add TestInboundCheckpointSurvivesCrashAndDeduplicates.

Evidence: Adopted section 2b explicitly reserves a durable inbound handoff
store with checkpoint and acknowledgment tokens, ordering and duplicate
semantics, Slack Events, Telegram getUpdates with one owned update offset, and
WhatsApp callbacks. The proposed interface has only Post, Replies, and Whoami,
with a cursor scoped to each question thread and no acknowledgment.

Disposition (m0b): AMENDED §2: Receive is destination-wide with thread
correlation; the persisted cursor is the acknowledgment;
TestInboundCheckpointSurvivesCrashAndDeduplicates stands.

## FSC-R1-005 (medium, material)

Section 3, artifact Spend today report line and
TestReportComposesFromLedgerJobsAndLandings: the asserted durable United
States dollar cost source does not exist on main. internal/usage supplies
token or provider-unit facts but sets cost to null for every current runtime.
Proposed change: omit the dollar line until a named durable price or cost
source exists, or name and test the new pricing-derived source this build will
add.

Evidence: Search of metasystem/internal/usage found production cost fields
only as nil in usage.go, acp.go, devin.go, and the Codex/Claude event-stream
path. Therefore events.jsonl does not already contain the cost lines claimed
by section 1.4.

Disposition (m0b): folded §3: usage units per runtime, no dollars.

## FSC-R1-006 (high, material)

Section 7, artifacts channel.Poll, both steward tick-driver call sites, and a
new TestTickChannelPassBound: a 15-second bound per transport call does not
bound the poll pass. Many open questions, Slack pages, rejection posts, and
the status post can consume 15 seconds each sequentially, delaying revival,
pending-notification delivery, and the resident runner's next observation.
Proposed change: give the whole channel phase one 15-second context and a
bounded per-pass work budget, then carry remaining work to the next tick.

Evidence: The design bounds each call, not the full pass. The resident runner
performs revival and DeliverPending after RunTick, and the standalone command
does the same; channel work placed outside arbitration still blocks those
sequential driver duties from running.

Disposition (m0b): folded §7: one 15 s context for the whole phase, work
budget per pass, last duty in both drivers.

## FSC-R1-007 (high, material)

Section 5, artifacts humanauthority.Proof, AuthorizesSetObligation, the resume
authorization path, and the answer-consumption tests: the claim that the
recorded answer can use today's relayed-word path is false as a durable
contract. That path recognizes only enrolled-terminal proof or
TEMPORARY_HUMAN_WORD under ruling R-32-m1, whose horizon is 2026-09-06; it
does not consume an authenticated-channel operation identifier. Proposed
change: explicitly name the authorization consumers that validate
AUTHENTICATED_CHANNEL_WORD and its operation identifier for only the already-
permitted resume and set-obligation acts, with tests after the temporary
ruling horizon.

Evidence: humanauthority.Proof.Valid accepts only HUMAN_AUTHORITY_PROVEN,
while temporaryValidFor is the separate R-32-m1 departure. The recorded
validator and goal consumers have no authenticated-channel outcome branch or
approved-reference input for resume and set-obligation.

Disposition (m0b): folded §5 and D5: named consumers resume and set-obligation
with --approved-ref, R-32-m1 set only, horizon-independent.

## FSC-R1-008 (medium, material)

Section 2, artifact Provider.Whoami and TestWhoami: the method comment says it
identifies the configured human, but Slack auth.test identifies the token
holder, which is the bot. This leaves implementers choosing between an always-
failing human comparison and an unused method. Proposed change: rename and
specify it as credential identity/readiness only, and state that inbound
UserID plus the configured human identifier is the sole provider-identity
check.

Evidence: The Slack mapping names auth.test while section 5 authenticates the
human using Inbound.UserID and channel.human.slack.user-id. No step consumes
ProviderIdentity from Whoami.

Disposition (m0b): folded §2: Credential = token identity/readiness only.

## FSC-R1-N01 (low, note)

Note — Section 2 Slack Replies: using oldest equal to the cursor, filtering
returned timestamps strictly greater than the cursor, and skipping the bot-
owned root correctly accommodates Slack's inclusive oldest behavior and root-
first response. No separate finding is raised for that pagination rule.

Evidence: The design explicitly provides all three behaviors and pages
response_metadata.next_cursor until exhaustion.

## FSC-R1-N02 (low, note)

Note — Section 6 configuration: no proposed key collides with a key currently
read by metasystem/internal/config. Secret resolution remains safe only if the
new channel layer implements the adopted committed-file skip and literal
scrub; the generic config resolver does not do that automatically.

Evidence: Repository-wide key search found the proposed names only in the
reviewed design. config.Get otherwise permits an arbitrary valid dotted key to
resolve from environment, metasystem.conf.local, or the committed file.

## FSC-R1-N03 (low, note)

Note — Section 3 report sources other than monetary cost are present: the goal
ledger, origin/main history and Goal-Item trailers, structured job records
under artifacts/agents/jobs, and internal/report scanners exist on main.

Evidence: The repository contains the named goal files and report packages;
git log of origin/main returned Goal-Item trailers, and report.Scan reads
artifacts/agents/jobs JSON records including startedAt and status.

## Verdict line

Disputed points become these named test obligations:
TestPollAtomicallyConsumesTOTP; TestPollCrashRecoveryExactlyOnce;
TestAuthenticatedChannelHistoryRoundTrip;
TestInboundCheckpointSurvivesCrashAndDeduplicates;
TestReportHasDurableSpendSource; TestTickChannelPassBound;
TestAuthenticatedChannelAuthorityAfterTemporaryHorizon;
TestWhoamiIsCredentialIdentity.
