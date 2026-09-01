# Alert channel design critique — revision 7, round 1 (Sol)

Chain: design round implementer-0d40e4f087fbb016d455fd35 (Fable lane,
recovery-certified) -> critic design-critic-563ae99ff0a1c5e082e659fb
(codex gpt-5.6-sol, xhigh, fresh context), 2026-09-01, reviewed commit
7544c9310fc4443fadee4062f865c417eca5b3ec. Nine material findings — one
critical, eight high; every finding quoted verbatim from the critic's
return (durable at artifacts/agents/design-critic-563ae99ff0a1c5e082e659fb/
rounds/1/return.json). Revision 8 of plans/alert-channel-design.md is owed
and must fold or refute each by id.

## AC7-PRODUCER-STATE-001 — high, material=True

CLAIM: The two new slice-1 producers have no durable producer discriminator or payload representation, so their send-time composition and class-scoped lifecycle cannot be implemented from the specification without inventing an episode schema.

EVIDENCE: In metasystem/plans/alert-channel-design.md section 7 says “the episode store is the only durable delivery state for the alert class” and “resolution is class-scoped.” Sections 11a.8 and 11a.9 require carried facts and “Composition at send time.” In contrast, section 11a.1 specifies only three persisted sender-stamp fields. The shipped metasystem/internal/steward/alert_episode.go lines 49–64 contain only Digest and Message for content, with no producer class, goal identifier, job identifier, revision, stop identifier, state, or failure reason; lines 246–265 clear every episode on a healthy observation. An implementer must therefore choose among adding an unspecified payload schema, precomposing contrary to the send-time law, or rereading job/goal state contrary to the episode-store source-of-truth law.

## AC7-PRODUCER-ATOMICITY-001 — critical, material=True

CLAIM: Neither new producer has a recoverable handoff into the episode store; a crash or second-store write failure after the source transition can permanently suppress the required alert.

EVIDENCE: Section 11a.8 says “The transition writer journals in the same operation,” but the job record and alert episode are separate files under separate locks, and no ordering, outbox, or reconciliation scan is specified. metasystem/internal/dispatch/record.go lines 540–550 durably write the terminal job record before returning; metasystem/internal/supervise/reaper.go lines 122–123 then ignore terminal records. Section 11a.9 likewise journals after the custodian report, while metasystem/internal/dispatch/stop.go lines 293–296 skip a completed stop batch on every later scan. Thus source-first can lose the alert forever, while alert-first can publish an alert for a transition that later fails. The design's at-least-once crash law begins only after a pending attempt exists and does not close either pre-journal gap.

## AC7-STOP-OUTCOME-001 — high, material=True

CLAIM: The stop-awaiting-resume producer includes outcomes for which its composed alert is false: failed or indeterminate stop work does not prove that a budget fence closed or that goal resume is required.

EVIDENCE: Section 11a.9 requires “one episode per report carrying a goal id and revision, completed and failed stop states alike,” but its fixed message says “the goal waits for resume” and its fixed ask says “The budget fence closed this revision and nothing will move it without you.” metasystem/internal/dispatch/stop.go lines 315–331 produces goal-bearing INDETERMINATE routes when the budget is unknown or stop capability is absent, before closing a fence; metasystem/internal/steward/tick.go lines 77–80 preserves that state in the report. A FAILED stop command can also fail before closure. Following the design literally would send a false state assertion and direct the human to the wrong authority verb.

## AC7-JOB-WRITER-001 — high, material=True

CLAIM: The delegate-job-failed wiring omits a shipped terminal-failure writer, so protocol failures under a claimed goal can remain silent.

EVIDENCE: Section 1 of metasystem/plans/alert-channel-design.md claims a job “reaches terminal failure only through the record CAS's transition table,” and section 11a.8 consequently wires “every other path that moves such a record through the record CAS's transition table.” That premise is contradicted by metasystem/internal/dispatch/record.go lines 417–464: RecordProtocolError directly writes status failed and error protocol_error without calling RecordCAS. An implementer following the enumerated wiring points will miss this required failure class.

## AC7-MESSAGEREF-PERSISTENCE-001 — high, material=True

CLAIM: Provider message-reference retention is assigned to slice 1 but its exact persisted AlertAttempt field is still unspecified, so the first of revision 7's four repairs is not mechanically complete.

EVIDENCE: Section 5a requires merging the attempt's “MessageRef”; section 7 requires each AlertAttempt to carry the returned MessageRef; and section 11 correctly assigns that retention to slice 1. However, section 11a.1's persisted AlertAttempt change lists only “Three additive fields” for senderPid, senderPidStartedAt, and stampedAt. The shipped AlertAttempt in metasystem/internal/steward/alert_episode.go lines 27–34 has no MessageRef or Channel. Neither a JSON field name, nesting, omission rule, nor compatibility rule is supplied for the newly slice-1 provider reference. Different implementers would produce incompatible durable episode bytes and fixtures.

## AC7-SEND-OUTCOME-001 — high, material=True

CLAIM: Sections 2 and 11a.3 still prescribe mutually exclusive results for an unconfigured destination.

EVIDENCE: Section 2 says: “Send's top-level error covers only pre-transport failure (unknown destination, unconfigured).” Section 11a.3 says the opposite: “Unconfigured destination … is NOT a top-level error” and requires a nil top-level error plus exactly one ChunkOutcome containing ErrUnconfigured. This changes caller control flow, fallback journaling, and what tests assert; the section pair explicitly claimed as checked in revision 7 still disagrees.

## AC7-COMPOSER-BYTES-001 — high, material=True

CLAIM: The truncation repair cannot be byte-exact because the bytes of the never-cut message portions are not defined anywhere in the slice-1 specification.

EVIDENCE: Section 9 calculates the Happened budget as “cap − bytes(all never-cut parts) − bytes(tail)” and calls the rule exact. Yet section 6 defines only the concepts “WHAT HAPPENED / WHAT IS ASKED / the exact ANSWERING ACT” and says an unspecified “acknowledgment line” is appended. Section 11a.6 fixes the three field values but again says only that the acknowledgment line is appended; it never fixes labels, separators, newline placement, the acknowledgment command text, or repository-path placement. A fresh implementer must invent human-visible bytes, and those choices change both the Telegram request and the truncation boundary.

## AC7-DEDUP-ENCODING-001 — high, material=True

CLAIM: The two new deduplication keys name tuples but never define how those tuples become the episode store's required digest value.

EVIDENCE: Section 11a.8 calls the key the pair “(delegate-job-failed, job id),” and section 11a.9 calls it “(stop-awaiting-resume, goal id, revision).” The shipped episode schema in metasystem/internal/steward/alert_episode.go lines 119–120 accepts only a valid evidence digest; metasystem/internal/steward/component_evidence.go lines 436–443 defines that as a lowercase SHA-256 digest. The design supplies no canonical tuple encoding, delimiter/length framing, revision rendering, or hash rule. Different choices create different episode identifiers and break deduplication compatibility across upgrades.

## AC7-TICK-ERROR-PATH-001 — high, material=True

CLAIM: The external tick driver's failed-RunTick branch is not assigned a DeliverDueAlerts behavior, so the third contradiction repair still leaves a control-flow choice.

EVIDENCE: Section 5 says DeliverDueAlerts is called “AFTER RunTick returns” by both drivers and “alongside” each existing DeliverPending call; section 11 says each driver gains one additive call. But metasystem/cmd/metasystem/steward_verbs.go lines 234–239 has an early error branch that calls DeliverPending and returns, while lines 270–285 contain the separately traced success-path delivery and report. Revision 7's section 1 cites only lines 231 and 270. An implementer can lawfully place the new call only on success, in both branches, or before the branch; those builds differ when an earlier pending alert exists or RunTick journals before a later error.

## Critic-declared gaps

- The runtime classified context isolation and independent-examination proof as advisory. I performed a fresh read of the landed tree, but the harness could not prove that isolation property.
