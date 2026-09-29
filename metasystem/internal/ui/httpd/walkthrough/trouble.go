package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// Asking what happened (g1-s68): the fake Partner's answer to a press on Ask
// what happened, in the three parts the skill asks for, ending with the
// recovery — one Apply card for a goal act the proposal grammar holds, and one
// link for a press elsewhere in the interface, never a card.
//
// It is narrowed to the trouble block's own first line, which only a turn a
// press asked carries: the skill's words about the block are in every first
// prompt, so the block's heading alone would narrow nothing.
const troublePhrase = "- What the screen said:"

// The goal the card proposes to resume: parked on this fixture, so Apply is an
// act the engine can take.
const troubleResumes = "g1-s22"

// troubleRoom is the review room the answer links to: the record a review
// sitting on the fixture's reviewed goal writes.
const troubleRoom = "/review/plans/reviews/review-of-" + reviewedGoal + ".md"

var troubleAnswers = []fakeacp.Answer{
	{When: troublePhrase, Chunks: []string{
		"**What happened.** You pressed the act this line is under, and the engine refused it; ",
		"the sentence on screen is the engine's own.\n\n",
		"**Why.** I read the refusal register's row for the code and the goal itself: the goal is claimed by ",
		"another pair, and pausing another pair's claim is a human act at that terminal. ",
		"Nothing in the records says the claim has gone quiet.\n\n",
		"**How to recover.** Two presses, both yours. The goal it is waiting behind, `" + troubleResumes + "`, ",
		"is paused; I have proposed resuming it, and Apply on the card below resumes it and nothing else. ",
		"The review of `" + reviewedGoal + "` is where the claim's work is examined: ",
		"[the review room on " + reviewedGoal + "](" + troubleRoom + ") — press Review there. ",
		"If the pause itself is what you want, it is `metasystem goal pause` from an enrolled terminal.\n",
	}},
}

var troubleReads = []fakeacp.Read{
	proposed(troublePhrase, uitools.ProposeUnpark, troubleResumes, nil,
		"it is paused behind the claim the refusal names; resuming it lets its own work go on"),
}
