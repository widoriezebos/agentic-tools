Working Mode: design
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal alert-escalation-channel, overnight envelope R-38-m3)
Date: 2026-08-31

# Goal

Round-2 critique of plans/alert-channel-design.md (revision 2, landed
34021825): the Fable designer folded all eleven of your round-1
findings plus Wido's three words (Telegram first; the session bridge
as a second first-class consumer; Slack threading via conversation
identity in the contract). The revision widened the design
substantially in one pass — rebuilt contract (named destinations,
sender identity, conversation and reply identity, transport
MessageRef, declared adapter capabilities), the alert lock narrowed to
journaling only, the retry queue retired. The designer's own
self-grade names the weakest claim: the lock split against the
crash-gap at-least-once delivery law.

# Workspace

The dispatch-created job worktree, branched from main. Read
everything; write nothing but your return.

# Threat model (findings outside it close as out-of-scope)

1. THE LOCK SPLIT (the designer's own named weak point): with the
   exclusive lock covering only journaling and the transport send
   running lock-free between the pending-attempt write and the
   completion write, prove or refute the at-least-once law across
   crash, concurrent-tick, and duplicate-send interleavings.
2. ROUND-1 FOLD FIDELITY: spot-check the disposition table (section
   12) against your round-1 findings — any fold that quietly weakens
   what the finding required?
3. TWO-CONSUMER CONTRACT: with bridge fields now first-class, would
   the seat-to-seat consumer (send, receive capability, conversation
   threading) work over the same adapters with zero contract change?
   Judge the deliberately-deferred receive loop boundary: is the
   contract/loop split clean, or does a contract gap hide in the
   deferred half?
4. THREADING DEGRADATION: does the conversation-identity mapping hold
   for all four adapters — Slack threads, Telegram reply-chains, email
   reply headers, WhatsApp — without per-adapter leakage?
5. SLICE 1 REALITY: core plus working Telegram adapter within 4
   hours, independently deployable, with the loudly-but-harmless
   failure floor now owned inside slice 1 (your round-1 AC-SLICE-001)?

# Constraints

- Round budget: ONE round. Findings ranked by severity with refuted
  text and evidence. Do not redesign (R-25b-m1).
- Wall-clock budget: 25 minutes.

# Expected Return

The design-critic version-2 return: findings, per-line verdicts, your
R-24-m1 self-grade.

# Gap Rule

If the design file or a cited authority is absent from your worktree,
stop and report the gap; never critique from memory.
