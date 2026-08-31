Working Mode: design
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal alert-escalation-channel, overnight envelope R-38-m3)
Date: 2026-08-31

# Goal

An independent critique of plans/alert-channel-design.md (authored
this hour by the Fable design lane, job
implementer-a9d33850396bacbb50b07184, landed 578eba43): the external
alert channel — one transport contract, pluggable adapters, two
message classes, episode store as sole state, never-blocking delivery.

# Workspace

The dispatch-created job worktree, branched from main. Read
everything; write nothing but your return. The design is at
plans/alert-channel-design.md.

# New human word since the design was authored (test against it)

Wido, after the design round: (1) Telegram is CONFIRMED as the first
example implementation — the design's flagged assumption is resolved
in its favor; (2) verbatim: "We can use the same mechanism for the
session bridge too, so there is a bit of reuse there" — the adapter
contract should bear a SECOND CONSUMER later: runtime-agnostic
seat-to-seat messaging (the seat-mutual-awareness goal). Critique the
contract as a two-consumer contract: would inter-seat delivery over
the same adapters require contract changes, or only a new caller?

# Threat model (findings outside it close as out-of-scope)

1. CONTRACT SOUNDNESS: is the adapter contract (send one message of a
   class, report delivery or failure) sufficient for all four named
   targets (email, Slack, Telegram, WhatsApp) by configuration alone,
   and for the future seat-to-seat consumer, without per-adapter
   leakage into call sites?
2. NEVER-BLOCKING LAW: can any code path make machinery wait on a
   send? Judge the design's handling of the one legacy delivery-gated
   launch behavior it preserves — is the split sound and bounded?
3. STATE HONESTY: does any part of the channel become a second state
   beside the episode store (queues, retries, receipts)? Where do
   receipts live and can they drift from episodes?
4. DEDUPLICATION: the design claims every blocked-on-human state
   reduces to a stable (class, subject-id) key WITHOUT having traced
   the producer call sites — the design's own declared weak claim.
   Test it against the four named producers (claim awaiting approval,
   stop awaiting resume, decision-ask, enrollment drift).
5. CREDENTIAL AND FAILURE STORY: credentials outside the repository,
   unconfigured channel degrades loudly-but-harmlessly, failed sends
   surface without becoming an unread log — complete and enforceable?
6. SLICE PLAN: is slice 1 (Telegram + core) at most 4 hours and
   independently deployable; are later adapters truly
   configuration-only?

# Constraints

- Round budget: ONE round. Findings ranked by severity, each carrying
  the design text it refutes and evidence.
- Do not redesign — the designer revises (R-25b-m1).
- Wall-clock budget: 25 minutes.

# Expected Return

The design-critic role's version-2 return per its schema: findings
with severity and evidence, a verdict per threat-model line, and your
R-24-m1 self-grade.

# Gap Rule

If the design file or a cited authority is absent from your worktree,
stop and report the gap; never critique from memory.
