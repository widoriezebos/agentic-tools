Working Mode: design
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal alert-escalation-channel, overnight envelope R-38-m3)
Date: 2026-09-01

# Goal

Revision 6 of plans/alert-channel-design.md: the Sol implementer
GAP-STOPPED slice 1 with zero invented bytes and the seven gaps below
— each one a place your design references or requires something
without mechanically defining it. This is not a critique round; it is
the implementability pass the design critic could not perform. Every
gap needs a specification an implementer can code WITHOUT deciding
anything: exact fields, exact persisted representations, exact
behavior. Relayed whole per R-25b-m1.

# Workspace

Your prior worktree. Revise exactly one file:
plans/alert-channel-design.md.

# Constraints

- Wall-clock budget: 30 minutes. Order: the sender stamp and refusal
  journal first (the riskiest — persisted state and recovery), then
  the five undefined contract types, then the unconfigured-send
  outcome law, the implicit-destination conflict (remember: legacy
  stays byte-for-byte), the health-floor counting semantics, the
  health-alert composer mapping, the Telegram seam rules (request
  encoding, response validation, timeout ownership, fake-endpoint
  injection, max-message capability).
- Where slice 1 does not need a full answer (e.g. ConversationState
  for the unthreaded adapter), specify the slice-1 minimum explicitly
  and mark the remainder for its owning slice — a marked deferral is
  lawful, an unmarked reference is not.
- Update the status line and self-grade.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/alert-channel-design.md; whatWasDone maps each of the
seven gaps to its new section.

# Gap Rule

stop and report a gap; never fill it silently.

## The implementer gap-stop, verbatim (seven gaps, zero invented)

1. The highest-risk gap is the completion contract: the design requires locating a pending attempt by sequence number and sender stamp, but never defines the sender identity source, representation, or persisted stamp field. It also requires a refused completion to be durably journaled without naming that journal’s schema or location.

2. The required public contract is incomplete. MessageClass, ContentSpan, AdapterCapabilities, DestinationConfig, and ConversationState are referenced but not mechanically defined. Choosing their fields would create an application programming interface contract not authorized by the accepted design.

3. The failure laws conflict at the unconfigured boundary. Send returns a top-level error before transport, while episode receipts are defined as one AlertAttempt per ChunkOutcome. An unconfigured send therefore has no outcome to journal even though unconfigured failures and fallback attempts must remain observable.

4. Slice 1 ships only Telegram, but configuration also requires an implicit local-command destination when no alert destination exists, while the legacy NotifyCommand implementation must remain byte-for-byte untouched. The design does not specify whether the channel layer implements command and desktop adapters, receives an injected legacy transport, or bypasses its sole-engine contract.

5. The health floor is underdetermined: the design does not specify whether silent episodes, pending attempts, failed attempts, cleared episodes, or acknowledged episodes count as undelivered; whether age begins at episode opening or the latest attempt; how partial minutes round; or what the verdict says when the episode store cannot be read.

6. The alert composer requires nonempty Happened, Asked, and Answer fields, but the existing health producer persists only one precomposed Message string. No exact mapping or answering act is specified for health alerts, so implementing one would invent human-visible wording and behavior.

7. The Telegram provider seam lacks mechanical rules for request encoding, response validation, timeout ownership, fake-endpoint injection, and the advertised maximum-message capability. Revision 5 exposes a byte capability while the provider constraint is character-based, leaving the declared value ambiguous.

RISKIEST: The reload-and-merge completion cannot be implemented exactly because its sender-stamp identity and durable refusal journal are unspecified; inventing either would change persisted state and recovery behavior.