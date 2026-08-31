Working Mode: design
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal alert-escalation-channel, overnight envelope R-38-m3)
Date: 2026-09-01

# Goal

Round-3 fold of plans/alert-channel-design.md. Three findings remain
(from eleven, then five): one critical — the sender's completion
transition can erase acknowledgment or clearing state written while
transport was in flight (a lost-update on the completion merge) — and
two highs (email reply ancestry beyond two messages; slice 1's cutover
while the legacy pending-notification queue still rides its old
transport). Two lines are SOUND and closed: the receive-half
reservation and multipart receipt outcomes. Relayed whole per
R-25b-m1.

# Workspace

Your prior worktree. Revise exactly one file:
plans/alert-channel-design.md.

# Constraints

- Wall-clock budget: 25 minutes. The critical first: the completion
  write must merge against re-read current state (or an equivalent
  compare-and-swap law) such that an acknowledgment recorded mid-
  transport survives; state the law precisely enough to test. Then
  email ancestry (the critic's own self-grade says MessageRef.ThreadID
  carrying the full references chain may already be your intent — if
  so, SAY it in the contract), then the cutover: slice 1 may narrow
  again (gate cutover can be its own slice behind a configuration
  default) rather than absorb compatibility work.
- Keep the R-24-m1 self-grade current.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/alert-channel-design.md; whatWasDone names each
finding folded, refuted, or left.

# Gap Rule

stop and report a gap; never fill it silently.

## Round-3 verdicts and self-grade

### git rev-parse HEAD && git status --short
The reviewed commit is b0b397c01b1bebbe39ad0ca274207e43484fce6d. The design file was unchanged relative to that commit. Unrelated worktree changes were present and were not touched.

### nl -ba metasystem/plans/alert-channel-design.md
Read all 482 lines of alert-channel design revision 3, including its sender laws, channel interfaces, receipt rules, slice plan, dispositions, and self-grade.

### nl -ba metasystem/plans/alert-channel-fold2-brief.md; nl -ba metasystem/plans/goals/alert-escalation
Read the complete round-two findings and the two governing goals. All cited repository authorities needed for this closure critique were present.

### nl -ba metasystem/internal/steward/alert_episode.go; nl -ba metasystem/internal/steward/runner.go; n
The shipped episode implementation keeps one in-memory episode snapshot because its alert lock currently spans submission and completion. Acknowledgment and healthy clearing independently read and save the episode under that lock. The shipped resident and command-line tick paths also continue to feed and drain the legacy pending-notification queue through the legacy notifier.

### Per-line verdict — chosen single-flight sender
FAIL. The non-blocking exclusive flock correctly prevents concurrent senders, a dead holder leaves no stale kernel lock, and a stale on-disk sender stamp can be reused by the next exclusive holder. However, the design does not define a current-state merge for completion after acknowledgment or clearing writes occur during transport, so the chosen split has not proved state-safe receipt completion. A live but stopped holder remains a disclosed liveness limitation rather than a stale-lock takeover case.

### Per-line verdict — round-two threading and multipart-receipt folds
FAIL for threading; SOUND for multipart outcomes. Ordered ChunkOutcome values and source spans can represent every attempted multipart submission and longest-prefix cursor advancement. The first-and-latest MessageRef state does not define how an email adapter retains the complete References ancestry required for a multi-message reply chain.

### Per-line verdict — four-hour first slice
FAIL. The four one-hour estimates omit the compatibility work needed when the Ready gate accepts Telegram while legacy queue-backed producers still deliver only through NotifyCommand. The slice is therefore not independently deployable under Telegram-only configuration as written.

### Per-line verdict — receive-half reservation
SOUND. The five receive obligations, read together with section 3's explicit exchange-join decision, reserve ingress ownership, durable handoff and acknowledgment, ordering and duplication, typed failures, authenticated provenance, and provider-reference correlation. The seat-mutual-awareness design can add its discovery, response, deadline, and conduct policies above this outbound interface without changing it.

### R-24-m1 self-grade
Confidence is 0.91. The weakest finding is the email ancestry finding because MessageRef.ThreadID could have been intended to carry the complete References chain, but the contract does not say that and equally permits incompatible implementations. Reject the sender finding if completion is already required to reload and conditionally merge the current episode under the alert lock rather than save the pre-send snapshot. Reject the slice finding if a Telegram-only Ready result is not permitted until every still-live legacy queue producer has a working delivery route.

## Round-3 findings, verbatim

### AC3-SENDER-MERGE-001 (critical)

CLAIM: The sender completion transition can erase acknowledgment or clearing state written while transport is in flight. This refutes the claim that re-taking the alert lock is sufficient to “journal each ChunkOutcome as its attempt result” and complete the round-two lock finding. Revision 3 says to read and stamp a pending attempt, release the alert lock, send, and later re-take the lock, but it never requires reloading the current episode, matching the stamped attempt, and merging only the receipt fields. An implementer mechanically splitting the shipped function can retain its pre-send episode snapshot and save it after completion. If AcknowledgeAlert or a healthy tick saves acknowledgment, resolution, or clearing during the send, that stale completion save overwrites the newer state.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 271–283 specifies lock release and later completion but no reload, comparison, or merge transition; lines 291–304 argues only sender-to-sender exclusion. metasystem/internal/steward/alert_episode.go lines 341–359 currently completes and saves the already-loaded episode snapshot because the alert lock never left, while lines 246–267 and 373–390 show healthy clearing and acknowledgment are separate read-modify-save writers. The new lock split makes those writers concurrent with transport completion, so mutual exclusion at each individual write does not prevent a lost update.

### AC3-THREAD-ANCESTRY-001 (high)

CLAIM: The common conversation state does not fully specify email reply mapping after more than two messages. This refutes “ConversationState (the conversation's first and latest known MessageRef) ... is how ... email sets In-Reply-To/References” and the claim that the threading fold is complete for every named adapter. An email reply's References field is derived from the parent message's existing References ancestry followed by the parent Message-ID. Retaining only first and latest MessageRef values cannot reconstruct an arbitrary parent chain unless ThreadID is explicitly defined to carry that ancestry or the store retains more references; neither rule exists, leaving materially different implementations.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 146–151 limits ConversationState to first and latest references while promising email In-Reply-To and References; lines 201–206 gives the store the same two-value shape. [Request for Comments 5322 section 3.6.4](https://www.rfc-editor.org/rfc/rfc5322.html) specifies that a reply's References field contains the parent's References field followed by the parent's Message-ID. A three-or-more-message chain therefore needs ancestry not represented by the stated store contract.

### AC3-SLICE-GATE-CUTOVER-001 (high)

CLAIM: Slice 1 cannot safely replace the launch gate while the legacy pending-notification queue remains on its old transport. This refutes “live-token deployable,” “No live-token precondition outside the slice remains,” and the four-hour independent-deployment claim. Slice 1 lets Ready pass for a Telegram destination and implements the new episode sender, but queue retirement and caller migration are deferred to later slices. Existing verdict, revival-failure, and other queue-backed notifications still call the legacy Deliver and NotifyCommand path. With Telegram configured and no legacy command, the new gate admits the runner while those existing notifications cannot use Telegram and remain undelivered. An implementer must add compatibility or migration work, or defer the gate cutover, changing both the slice and its arithmetic.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 319–323 puts the Ready gate in the new path; lines 407–421 calls that slice independently deployable in four hours; lines 422–432 defers noticings, queue retirement, caller migration, and blocked-on-human producers. metasystem/internal/steward/tick.go lines 200–210 still queues notify verdicts; metasystem/internal/steward/runner.go lines 113–133 still queues revival failures and drains pending notifications; metasystem/internal/steward/notify.go lines 24–58 and 61–98 proves that queue drains only through NotifyCommand or the platform desktop notifier, not a Telegram destination.