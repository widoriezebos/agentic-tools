Working Mode: code-critique
Orchestrator Identity: m1 (lineage main-1788594343-3833-fb64b9, closing round on critic chain code-critic-aa235e0ddd1ff4832da65826, goal channel-ask-fits-one-message)
Date: 2026-09-05

# What this round is

The closing review of the fold. Your round 1 raised one material finding,
F-1: questionMessageRuneLimit was 4000, the Telegram adapter's chunkLimit
restated, so the change stopped the split without stopping the wall, and
your arithmetic put the right bound at about 1600.

Round 2 of the implementation chain (implementer-1fb40275a06e386262ce7d0b,
round 2 diff at metasystem/artifacts/agents/implementer-1fb40275a06e386262ce7d0b/rounds/2/diff.patch)
reports: the limit is 1600 and documented as deliberately smaller than any
provider transport limit; a trimmed ask that dropped no facts no longer
claims a fact count; the proposed box line is back after the options and
before the recommendation; and a regression covering long options with all
short facts retained was added.

# Settle these

1. Is F-1 resolved? The constant must be a channel-level bound on what a
   human is asked to read, not derived from or equal to any provider limit,
   and the long-ask test must assert against the constant so the bound
   cannot drift back without the test noticing.
2. Do the invariants from round 1 still hold at the smaller bound: the
   reply instruction and token whole and last, every option label present,
   the trim notice reserved before facts are spent, cuts visibly
   ellipsized, and a short ask unchanged and claiming no trim?
3. Does the new no-facts-dropped wording say something true for every case
   that reaches it, including one fact dropped and all facts dropped?
4. At 1600 rather than 4000, is there any input that now exceeds the bound
   which did not before, beyond the accepted long-label case from your F-4?

# Return

Confirm or refuse each by number, material findings only with file:line
evidence and a concrete input. If F-1 is resolved and nothing new is
material, say so plainly so the register can close.
