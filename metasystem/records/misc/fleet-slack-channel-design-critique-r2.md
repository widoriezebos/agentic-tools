# Sol closing critique of the fleet conversation channel — round 2 (2026-09-03)

Job fsc-design-crit2 (design-critic, codex gpt-5.6-sol, cap 30) reviewed
plans/fleet-slack-channel-design.md revision 2 under
plans/fleet-slack-channel-critique-r2-brief.md. Five material findings, all
precise and small; under R-60-m1 they are build obligations, folded into
revision 3 of the design as law (its §5, §8, §10). No further review round.

## FSC-R2-001 (critical, material)

Section 5, artifact metasystem/artifacts/agents/channel/totp-consumed.json:
the same-inbound-reference exception is not safely scoped because MessageRef
values are provider handles, while the global consumption row records no
provider or destination and the exception compares only the reference. A
Slack, fake, or later Telegram envelope can reuse another provider's equal
handle during the same time-based one-time-password step and be treated as
recovery. One-line change: record and compare destination, provider, full
MessageRef, and question identifier for the recovery exception; add
TestTOTPResumeExceptionIsEnvelopeScoped to section 8.

Evidence: The provider contract describes MessageRef as the provider message
handle, supports switchable adapters, and places the consumed-step file above
destination scope. Section 5 stores only step, inboundRef, and question
identifier, but says equality of inboundRef alone admits reuse.

Disposition (m0b): folded §5: the consumption row and the resume exception
carry destination, provider, threadID, ref and qid;
TestTOTPResumeExceptionIsEnvelopeScoped added to §8.

## FSC-R2-002 (high, material)

Section 5, artifacts metasystem/internal/goal/HistoryLine and the question
answer operation: the allocated identifier `op-<qid>-<inbound ts>` cannot
enter the existing goal ledger because ParseHistoryLine requires
`<26-character ULID>-<machine>-<eight hexadecimal characters>`. One-line
change: allocate a stable caller ULID at MATCHED and derive the operation
identifier with goal.Opid; extend TestPollCrashRecoveryExactlyOnce to assert
that the resulting ledger parses.

Evidence: metasystem/internal/goal/file.go validates every operation
identifier with validOpidShape, whose required leading ULID and trailing
machine and lineage hash are incompatible with the literal revision-2 shape.

Disposition (m0b): folded §5: a caller ULID is allocated at MATCHED and the op
id is goal.Opid(ulid, machine, lineage); TestPollCrashRecoveryExactlyOnce
asserts the ledger parses.

## FSC-R2-003 (high, material)

Section 5, artifacts metasystem/artifacts/agents/channel/questions/<qid>.json
and metasystem/plans/goals/<goal>.md: CLOSED combines closing the question
record with appending the ANSWERED next-step line across two independently
durable stores, without ordering or idempotence. A crash can therefore leave a
closed question missing its next-step update or replay that update. One-line
change: make the next-step update conditionally idempotent and durable before
marking the question closed; require TestPollCrashRecoveryExactlyOnce to
assert exactly one ANSWERED line.

Evidence: Revision 2 assigns the history operation to RECORDED but assigns
both `state: closed` and the goal next-step mutation to CLOSED. A filesystem
rename of the question JSON cannot atomically include the goal transaction,
and a closed answer is no longer eligible to resume.

Disposition (m0b): folded §5: the history operation and the ANSWERED next-step
line land in ONE goal transaction at RECORDED; CLOSED is one rename; the test
asserts exactly one ANSWERED line.

## FSC-R2-004 (high, material)

Section 5, artifacts metasystem/internal/goal/HistoryLine, RenderHistoryLine,
and ParseHistoryLine: saying that AUTHENTICATED_CHANNEL_WORD has four proof
keys does not fix the strict grammar's exact key names or encodings, so
implementers must invent incompatible ledger formats. One-line change: specify
the four serialized key tokens and their encodings in
TestAuthenticatedChannelHistoryRoundTrip, while retaining the answer and
strict approval token only in the trailing reason field.

Evidence: The current parser rejects every unknown history key and the
renderer has explicit fields for every accepted token. Revision 2 identifies
provider, user identifier, message reference, and step semantically but
supplies no canonical serialized spellings. RecordedNormApproval does
correctly scan HistoryLine.Reason, so that part of the fold closes.

Disposition (m0b): folded §5: exact keys channelProvider, channelUser,
channelRef=<threadID>/<id>, channelStep, in that order;
TestAuthenticatedChannelHistoryRoundTrip fixes the spellings.

## FSC-R2-005 (high, material)

Sections 2 and 5, artifacts metasystem/internal/channel.Provider.Receive and
the destination cursor store: Telegram ignores the supplied open-thread list
and can return an unrelated destination update, but revision 2 defines no
durable disposition for an envelope that correlates to no open or recovering
question. Such an update prevents cursor acknowledgment forever or is silently
discarded. One-line change: define a durable per-reference `unmatched`
disposition and add an unmatched update before a valid reply to
TestInboundCheckpointSurvivesCrashAndDeduplicates; no separate acknowledge
method is then required.

Evidence: The cursor may be persisted only after every returned envelope has a
durable disposition. Section 5 defines durable outcomes only for rejected
replies and matched answers, while section 2 promises that a later Telegram
getUpdates adapter ignores the thread filter and uses one destination-wide
offset.

Disposition (m0b): folded §5: an envelope matching no open or resuming
question is journaled as unmatched by ref under the destination; the cursor
acknowledges after it; the named test gains an unmatched update.

## Next

Sol build under plans/fleet-slack-channel-build-brief.md (cap 120), one Fable
code review, land through the chain.
