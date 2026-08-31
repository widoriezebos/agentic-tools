Working Mode: design
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal alert-escalation-channel, overnight envelope R-38-m3)
Date: 2026-09-01

# Goal

Final fold of plans/alert-channel-design.md: round 4 returned exactly
ONE finding — the email References-chain trimming rule has no
implementable boundary (unspecified "header limits", misattributed
authority). Every other line of the design is now SOUND, including the
section 5a merge law. Relayed whole per R-25b-m1.

# Workspace

Your prior worktree. Revise exactly one file:
plans/alert-channel-design.md — the trimming rule only; touch nothing
else in the design.

# Constraints

- Wall-clock budget: 12 minutes. The fold must give the rule a
  CONCRETE boundary an implementer can code without deciding anything:
  a named numeric limit with its source (a fixed conservative
  constant stated in the design is acceptable; if you attribute a
  standard, cite the exact clause or do not attribute). State the
  behavior at the boundary exactly.
- Update the status line and self-grade; if this fold closes every
  open finding, say so plainly.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/alert-channel-design.md.

# Gap Rule

stop and report a gap; never fill it silently.

### AC4-EMAIL-TRIM-BOUNDARY-001 (high)

CLAIM: The email ancestry rule has no implementable trimming boundary. Section 3a says to trim when unspecified “header limits” would be exceeded and attributes permission to Request for Comments 5322, but that standard supplies no total References-field limit or trimming rule. One implementer can fold the complete chain indefinitely, another can treat the 998-character physical-line limit as the trigger, and another can apply a provider-specific cap. Those implementations emit different ancestry and assert different tests, so the supposedly common email contract remains under-specified for long conversations.

EVIDENCE: metasystem/plans/alert-channel-design.md lines 253–263 promises complete ancestry for any length, then trims at undefined header limits; line 571 calls that trimming bounded. Request for Comments 5322 section 2.2.3 says folding handles physical-line limits and an unfolded header field has no length restriction. Section 3.6.4 constructs References from the parent's References plus the parent's Message-ID and does not specify trimming. The later provider-test slice does not name a provider limit, authority, or required long-chain fixture.