# Backlog ordered by priority: closing read of slice 2 (chain bolnext-build1, round 1)

Critic bolnext-crit1 (code-critic, Opus 5) on reviewed tree c59478d97550ccd78d37b971c286d0ef097fb18e. Two material findings, both high, and three notes. The orchestrator independently reproduced the first by running the bed.

## BON-01 - high - material=True

CLAIM: The change rewrites the sentence `goal next` prints when nothing qualifies and leaves the shipped fixture bed asserting the old sentence word for word, so the goal-command bed named in this design's own gate fails.

EVIDENCE: scripts/agents/goal-cli-fixtures.sh step 10 requires the output to be exactly "no goal matches --label absent". The orchestrator ran the bed on the reviewed tree: it exits 1 with "the empty filtered candidate message is not distinct: no claimable goal for machine fixture-machine; no matching eligible work for --label absent". The new sentence also conflates two different empty conditions in one line.

## BON-02 - high - material=True

CLAIM: The verb can hand a machine the one goal it is not allowed to claim, and because this change makes the verb the only sanctioned way to take work, the machine has nowhere to go. The frontier admits a goal after checking labels, approval expiry, dependencies and pins. The claim verb applies three further tests the frontier does not: the goal must carry a budget, its approval record must be structurally valid, and its budget must fit the machine's tier box or carry a recorded norm approval. A goal that passes the first set and fails the second sits at the head of the rank and is returned to every free machine, forever. The new seat guidance tells a seat to claim only its returned ready goal and to fetch and select again rather than use a remembered fallback, so the seat cannot legitimately reach past it. Because the rank is global, every free machine converges on the same unclaimable head.

EVIDENCE: Proven by execution in a scratch copy. Two approved goals, one ranked first whose approved budget reserves 2400 job minutes and one ranked second reserving 120, against a tier-three box of 1200. Next returned both and selected the over-norm head; the claim admission gate refused that same goal with GOAL_NORM_REFUSED naming minutes=2400 against minutes=1200, and accepted the second.

## BON-03 - low - material=False

CLAIM: The seat guidance widens the fetching form beyond the page. The page says the fetching form is required when the seat is free; the new sentence requires it at turn end and whenever the seat is free, and a fetch failure is a hard error.

## BON-04 - low - material=False

CLAIM: Re-ranking now produces a steward ledger-attention event it did not produce before, because the waiting-queue snapshot is ordered by rank rather than by opened-at, and the comparison is position by position. An event saying the queue changed right after a human reordered it is truthful rather than misleading.

## BON-05 - low - material=False

CLAIM: The map of pinned goals on the claimable-work record is still built but has no production reader after pin promotion was removed from the idle diagnostic.

## Coordinator's reading (m1b, 2026-09-08)

Both material findings accepted. BON-03 and BON-05 are folded with them
because each is one edit and both are drift from the page rather than
judgement calls. BON-04 is recorded and not actioned: the event is true,
and suppressing a truthful change notice to keep a diagnostic quiet
would be the wrong trade.

BON-02 is the one that matters and it is worth naming precisely. Slice 2
makes `goal next` the sanctioned way to take work, so the verb's
eligibility became load-bearing in a way it was not when seats read
records. Two different places now decide what a machine may take, the
frontier and the claim admission gate, and they disagree. The
consequence is not one machine stalling: the rank is global, so every
free machine in the fleet converges on the same unclaimable head while
claimable work waits behind it.

The repair follows the lesson this program has already learned twice
about two spellings of one rule: the frontier must not re-implement the
claim gate's tests, it must call the same predicate, so the two cannot
drift again. The critic deliberately did not choose the repair, which
was correct, because the design page names the frontier's checks and is
silent on the claim gate's three.

BON-01 is the second time in this goal that the goal-command bed caught
something no canary could, after the listing shape in slice 1. The
canaries prove the behaviour they name; the beds catch what else was
reading the old shape. That is an argument for running both in that
order, not for replacing one with the other.
