# Backlog ordered by priority: fourth read of slice 2 (chain bolnext-build1, round 6)

Critic bolnext-crit4 (code-critic, Opus 5) on reviewed tree d430d19508736d1ea4e50400b00b247127ab0f0f. One material finding, medium, and two notes. One of the notes corrects the orchestrator's own brief and matters more than its grading suggests.

## BOQ-01 - medium - material=True

CLAIM: A free seat can still be handed work it cannot claim, and the seat guidance this change lands does not tell it what to do about that. The frontier now applies every gate the claim verb applies to the goal record, which closes the earlier gap. But the claim verb's first act, before it looks at any goal, is an unconditional refusal on a checkout declared the fleet brain or whose brain declaration is unreadable, and the frontier never consults that state. So on such a checkout `goal next --machine <nick> --fetch` answers with a ready goal and `goal claim` then refuses, every time. This matters now because the same change rewrites the contract from one orientation read at turn end into a standing instruction to take work through the verb whenever the seat is free. The design specified the handling in the same paragraph as the rest of the seat guidance: stop a retry cycle on a transport, authority or quota error and report that cause. Two of that paragraph's three requirements landed in both guidance documents; this one landed in neither, so the seat is told to fetch and select again when it loses a race and told nothing when it is refused on authority.

EVIDENCE: internal/goal/project.go contains no reference to the brain state; a repository-wide search finds only two unrelated comments. internal/goal/verbs.go:588 opens the claim verb with the brain fence and returns its message before any goal is examined. internal/brain/brain.go:448 shows that message is unconditional for a declared checkout.

## BOQ-02 - low - material=False, and it corrects this orchestrator

CLAIM: The orchestrator's brief said an over-norm goal is invisible in the frontier exactly as before this chain. That is wrong. Before this chain the frontier admitted such a goal to its ready list, because the only extra filter was a structural budget check that an over-norm budget passes. Routing the frontier through claim admission therefore also narrowed the idle-stop refusal and the steward's idle escalation: a seat whose only remaining backlog is over-norm approved work will now be allowed to end its turn where it was previously blocked three times and then handed a continuation for a goal it could not have claimed. Reported as fact, not as work: the direction is toward consistency with the claim gate and away from a livelock, the human still sees the goal at the head of the status report, and rounds 5 and 6 left the category alone as the brief asked.

## BOQ-03 - low - material=False

CLAIM: The steward's ledger-attention watcher compares the waiting queue position by position and that queue is now in rank order rather than opened-at order, so a re-rank raises an event. Already recorded twice in this chain.

## Coordinator's reading (m1b, 2026-09-08)

BOQ-01 accepted and folded in round 7, documentation only. The
reasoning for folding rather than landing: this goal exists so that
seats take work through one verb, and shipping a version that tells the
brain checkout's seat to do exactly that and then refuses it every time,
with no instruction for the refusal, ships the loop this program has
spent the day fighting. The design already specifies the sentence, so
the fold has no code surface and no invention.

BOQ-02 is the more valuable finding even though the critic graded it not
material, because it corrects a false statement the orchestrator put in
a brief and then repeated to Wido. The over-norm goal was NOT invisible
before this chain. Narrowing the frontier to the claim gate's tests also
narrowed the idle refusal, which is a real behaviour change that this
chain made and nobody asked for. It is defensible, since the alternative
is handing a seat a continuation for a goal it cannot claim, and the
goal remains visible to the human at the head of the status report. It
is recorded here rather than actioned, and it belongs in the record of
whatever settles the missing category, because the two questions are the
same question seen from two sides.

That correction is also the second time today a read has caught the
orchestrator asserting something it had not verified. The first was
grading the stop-verb classifier findings. The rule that follows is the
one already written after the last such lapse: state what was measured,
not what is assumed, and mark an assumption as an assumption when it
reaches a brief.
