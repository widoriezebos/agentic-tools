Working Mode: design
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal alert-escalation-channel, overnight envelope R-38-m3)
Date: 2026-09-01

# Goal

Revision 7 of plans/alert-channel-design.md. Your own reject condition
fired: the second slice-1 gap-stop happened, and its four gaps (below,
verbatim) are CROSS-SECTION CONTRADICTIONS introduced by section 11a's
one-pass addition — 11a against sections 5a, 11, 2a, and the external
tick surface. The orchestrator's process changed in response: this
round has a 40-minute budget instead of 30, and its mandate is
consistency, not speed.

# Workspace

Your prior worktree. Revise exactly one file:
plans/alert-channel-design.md.

# The mandate

1. Resolve the four contradictions decisively (MessageRef retention:
   pick ONE slice and make sections 5a and 11 agree; the adapter
   context parameter: make 2a and 11a.7 one contract; the external
   steward tick command: either wire DeliverDueAlerts into that path
   in slice 1 or declare transport resident-runner-only for slice 1
   IN section 5 AND section 11 with the operational consequence
   stated; the truncation tail: exact bytes, UTF-8 boundary rule,
   and which fields may shorten).
2. Then perform an explicit SELF-CONSISTENCY PASS: for every rule in
   section 11a, verify the sections it touches agree with it, and say
   in the status line that this pass was done and over which section
   pairs. The previous revision failed precisely for skipping this.
3. Update the self-grade; the reject condition for revision 7 should
   be a THIRD gap-stop, stated plainly.

# Constraints

Wall-clock budget: 40 minutes. No other design content changes beyond
what the four resolutions and the consistency pass require.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/alert-channel-design.md; whatWasDone maps each
contradiction to its resolution and names the consistency pass's
section pairs.

# Gap Rule

stop and report a gap; never fill it silently.

## Second implementability gap-stop, verbatim (four cross-section contradictions)

1. Sections 5a and 11 define incompatible slice boundaries. Section 5a requires completion to merge AlertAttempt.MessageRef, but section 11 says attempt MessageRef retention lands in slice 4. Implementing either clause would violate the other.

2. Section 2a defines AdapterSend without a context argument, while section 11a.7 requires the channel layer to create a timeout context and every adapter to honor its cancellation. The adapter cannot mechanically honor an unpassed context, and giving it one changes the specified interface.

3. Moving transport out of UpdateAlertEpisodes changes the supported external steward tick command because it calls RunTick but not DeliverDueAlerts. Section 5 wires the new sender only into the resident runner, while section 11 requires no legacy behavior changes; the design does not specify whether or where the external command must invoke it.

4. The 1,500-byte composition rule requires a truncation tail naming the episode, but does not specify that tail's bytes, UTF-8 boundary behavior, or which required alert fields may be shortened. Implementing it would invent human-visible behavior.

RISKIEST: The completion merge cannot be implemented exactly: it must persist the provider message reference in slice 1 and must defer the same persisted field until slice 4.