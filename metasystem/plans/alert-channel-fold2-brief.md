Working Mode: design
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal alert-escalation-channel, overnight envelope R-38-m3)
Date: 2026-09-01

# Goal

Round-2 fold of plans/alert-channel-design.md. Sol's second critique
(job design-critic-e17645332f616ee62bcc806f) returned five findings,
two critical. Relayed whole per R-25b-m1; every fold or refusal is
yours. One of the criticals (AC2-CONTRACT-001) is a FOLD-FIDELITY
finding: your revision's disposition table claims the receive half
folded while the contract still lacks it — address the table's honesty
as well as the gap.

# Workspace

Your prior worktree. Revise exactly one file:
plans/alert-channel-design.md.

# Constraints

- Wall-clock budget: 30 minutes. Severity order: the lock split
  (AC2-LOCK-001 — pick ONE implementation and prove its laws, or
  change the laws honestly), the receive contract (AC2-CONTRACT-001),
  then threading reply-mapping, receipts, slice arithmetic. Honest
  partial beats rushed whole; the status line names anything unfolded.
- Slice 1 must come back to at most 4 hours independently deployable,
  even if that means slice 1 narrows (AC2-SLICE-001).
- Keep the R-24-m1 self-grade current.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/alert-channel-design.md; whatWasDone names each
finding folded, refuted, or left.

# Gap Rule

stop and report a gap; never fill it silently.

## Round-2 verdicts and self-grade

### git rev-parse HEAD
The exact reviewed commit is 34021825b7c18dbe7453b2d282e02b471d4ff013.

### Read the complete alert-channel revision, its round-one findings and dispositions, and the seat-mutu
The review covered metasystem/plans/alert-channel-design.md, metasystem/plans/alert-channel-revision-brief.md, metasystem/plans/alert-channel-recritique-brief.md, and metasystem/plans/goals/seat-mutual-awareness.md. The disposition table claims every round-one finding was folded.

### Trace the shipped tick, alert-attempt, arbitration, lease, and revival locking paths.
metasystem/internal/steward/tick.go acquires the repository arbitration lock at lines 102–112 and retains it through UpdateAlertEpisodes at lines 237–265. metasystem/internal/lease/verbs.go lines 106–110 and metasystem/internal/steward/revive.go lines 32–36 and 68–74 use the same lock. metasystem/internal/steward/alert_episode.go lines 324–356 currently serializes pending creation, sending, and completion under the alert lock.

### Read the official provider threading and receive specifications.
[Slack’s official webhook documentation](https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/) says an incoming webhook does not return the message timestamp needed for a thread and requires Events or another retrieval API. [Telegram’s Bot API](https://core.telegram.org/bots/api/) makes polling acknowledgment depend on an update offset, makes polling mutually exclusive with webhooks, and uses message identifiers for replies. [RFC 5322](https://www.rfc-editor.org/rfc/rfc5322.html) defines email replies through Message-ID, In-Reply-To, and References. [Meta’s official WhatsApp collection](https://www.postman.com/meta/whatsapp-business-platform/request/73yi2uj/send-reply-to-text-message) requires the prior message identifier for a reply, while [its webhook reference](https://www.postman.com/meta/whatsapp-business-platform/folder/vzaxn16/webhook-payload-reference) defines inbound delivery as an HTTPS callback.

### Per-line threat-model verdicts
Line 1, lock split: fail—the crash retry preserves the minimal at-least-once lower bound, but the design leaves an impossible choice between retaining a machinery-blocking outer lock and allowing duplicate concurrent sends with dishonest completion state. Line 2, round-one fold fidelity: fail—the receive contract, per-try receipt cardinality, and four-hour independently deployable slice requirements were weakened rather than fully folded. Line 3, two-consumer contract: fail—the deferred receive half contains transport ingress, checkpoint, and sender-provenance contract decisions. Line 4, threading degradation: fail—the design does not own the provider-message-reference mapping needed by all four adapters. Line 5, slice-one reality: fail—the four-hour cut explicitly excludes the gate and failure floor and may not be enabled with a live token.

### Required critic self-grade under repository ruling R-24-m1
Confidence is 0.96. The weakest finding is the multipart-result finding because “channel layer” could be interpreted as an unshown orchestration layer above Channel; that ambiguity still makes implementations diverge. Reject the lock finding only if one proved single-flight owner covers every sender without retaining the arbitration lock across network work. Reject the receive finding only if the bridge is explicitly reduced to a send-only advisory fast path and the first-class receive claim is withdrawn; both conditions contradict this revision.

## Round-2 findings, verbatim

### AC2-LOCK-001 (critical)

CLAIM: The lock split cannot satisfy its stated laws in both possible implementations. This refutes “the crash-gap law is unchanged” and “no goal transition, dispatch, or decision waits on a send.” If the shipped outer arbitration lock remains around the delivery phase, concurrent ticks are serialized, but lease announcement, revival, and every other arbitration contender still wait for as many as three 15-second sends. If delivery moves outside that outer lock, a second tick can observe and reuse the first tick’s PENDING attempt while its send is still running. Both then perform the external side effect under one attempt. Rejecting the later completion loses a real try’s receipt; accepting both lets the last completion overwrite a success with failure or overwrite concurrent acknowledgment or clearing. Crash recovery still gives a duplicate-prone at-least-once lower bound, but ordinary concurrency breaks one-try-one-receipt and one-alert-per-episode.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 220–233 releases only the alert lock and never accounts for the enclosing arbitration lock; lines 283–293 require every try to have its own attempt while directing a matching PENDING attempt to be reused. metasystem/internal/steward/tick.go lines 102–112 and 237–265 prove the outer lock currently surrounds delivery. metasystem/internal/lease/verbs.go lines 106–110 and metasystem/internal/steward/revive.go lines 32–36 and 68–74 prove machinery shares that lock. The alternative interleaving follows directly from two writers both applying the specified matching-PENDING reuse rule before either completion is journaled.

### AC2-CONTRACT-001 (critical)

CLAIM: The seat-to-seat receive half is still not part of the contract. This refutes “what THIS design fixes is the shared contract those loops ride” and the disposition claim that receive was folded. Channel has no Receive operation; the prose supplies no destination argument, checkpoint or acknowledgment token, ordering and duplicate semantics, typed receive errors, or authenticated origin. Declaring a receive boolean does not fill those interfaces. A generic poll operation also cannot directly represent Slack Events or WhatsApp webhook ingress without a listener and durable handoff, while the design separately forbids adapter queues and state. The bridge designer must therefore change the contract or invent a second transport seam.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 112–130 defines only Send, Ready, and Capabilities; lines 133–142 relegates an unspecified Receive poll operation to prose; lines 144–145 forbids adapter state and queues. metasystem/plans/goals/seat-mutual-awareness.md lines 3–6 requires runtime-neutral durable transport and assertable seat identity, but Message.Sender at design lines 95–105 is merely an asserted string. Telegram’s official API requires an owned update offset and makes polling mutually exclusive with webhooks; Meta’s official WhatsApp reference defines inbound delivery as HTTPS callbacks; Slack’s official documentation requires Events or retrieval APIs for inbound/thread information.

### AC2-THREAD-001 (high)

CLAIM: ConversationID alone does not implement the promised reply mapping for the four adapters. This refutes “callers set ConversationID and nothing else” and “losing [the Slack map] degrades threading, never content.” Telegram, email, and WhatsApp replies require a prior provider message identifier, yet only Slack is assigned a conversation map; AlertAttempt receipts do not retain MessageRef; and adapters are otherwise declared stateless. Slack’s configured webhook cannot even return the initial timestamp needed to populate its map. Starting a fresh Slack thread after map loss is not rebuilding the mapping: a delayed reply on the old thread can no longer be mapped to its exchange. An implementer must either add owned durable reference mapping or make callers retain and supply references, producing a different state and interface design.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 107–110 introduces MessageRef, lines 163–176 gives only Slack a map while saying callers provide only ConversationID, and lines 283–293 lists attempt fields without MessageRef. The example at lines 188–190 configures Slack with a webhook URL. Slack’s official documentation says that webhook sends no timestamp; Telegram replies use a message identifier scoped to the chat; RFC 5322 requires Message-ID plus In-Reply-To and References; Meta’s WhatsApp API requires context.message_id. These are opaque provider references, but the design names no common durable owner for them.

### AC2-RECEIPT-001 (high)

CLAIM: The multipart fold still cannot represent the receipts it promises. This refutes “each chunk is its own send with its own receipt.” Channel.Send returns one MessageRef and one error for one Message, while the channel layer is said to split that message into several sends. Message carries no digest-entry span, and AlertAttempt carries no returned MessageRef. If Channel.Send performs the split, the episode and digest owners cannot observe every chunk outcome or advance the cursor per accepted chunk. If the caller performs it, the stated Channel behavior and ownership are different. This quietly leaves the original per-try receipt and multipart-outcome choice unresolved.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 112–118 defines a single-reference, single-error result; lines 283–293 requires one durable attempt per transport try; lines 355–361 requires several independently receipted chunk sends and per-chunk cursor advancement. No interface connects a chunk with its source-entry span or returns multiple outcomes.

### AC2-SLICE-001 (medium)

CLAIM: Slice 1 no longer meets the four-hour independently deployable requirement. This refutes “Each slice independently deployable” and weakens the round-one failure-floor disposition. The revision limits four hours to sub-slice 1a, excludes the launch gate and undelivered health and Stop-hook surfaces, and explicitly forbids enabling 1a with a live token until unestimated sub-slice 1b is complete. Therefore the four-hour result is neither a working live Telegram adapter nor independently deployable with the required loudly-but-harmless failure floor.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 405–424 withdraws the four-hour claim for the whole slice, assigns the gate and failure floor to 1b, and says 1b is required before live-token enablement. The disposition at lines 453–455 says the round-one slice finding was folded, while the round-two brief requires core plus working Telegram within four hours with the failure floor inside that deployable result.