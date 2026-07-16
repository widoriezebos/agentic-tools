# Working With Agents — the Human's Guide

The rest of the harness instructs agents. This document is your side of the deal: how to hand work to one or more agents, what to expect back, and the few duties only a human can do. The agent-facing counterpart is `docs/collaboration.md`; the modes themselves are explained in `docs/working-modes.md`.

## What You Can Expect

An agent operating under this harness will: inspect the repository before concluding anything; make the smallest change that satisfies the request; state its assumptions instead of hiding them; stop and ask before decisions reserved for you; prove its work by running it, not by asserting it; lead its report with where you should look first; and leave a receipt and, for unfinished work, a handoff note so the next session starts warm.

If an agent is not doing these things, that is a harness violation worth saying out loud — corrections about *how* it works are captured the same way as corrections about the code.

## Handing Over Work

- **State the outcome, not the steps.** "Users should stop seeing duplicate emails" beats a list of files to edit. The agent inspects; you decide what "good" means.
- **Name the mode when you know it — mode words are load-bearing.** "Refactor this" promises unchanged behavior and triggers baseline discipline. "Improve the p95 latency" triggers frontier and noise-floor rules. "Take a step back" halts momentum. "Verify it end to end" demands observed proof. Using the word buys you the whole discipline.
- **Give the acceptance criterion up front** when there is one: the test that should pass, the score to beat, the behavior to observe.
- **Name the non-goals** when scope could creep: "do not touch the public API."
- Expect ambiguity to come back either as a stated, reversible assumption or as one question with a recommendation. Answer the question asked; you rarely need to re-explain the task.

## Decisions That Come Back to You

Some calls are yours by design, and the agent will stop for them: production deployments and data, API or schema contracts, new dependencies, deleting user-visible behavior, anything on your project's reserved list in `docs/project-rules.md`. Two less obvious ones:

- **A red test is a question for you**, not an obstacle for the agent. It will never weaken or delete a failing test to get to green; it will ask whether the contract changed.
- **Changing an evaluation re-baselines the improvement frontier** — the agent will ask before doing both at once.

When you answer, decide briefly and say *why*. The why is what gets captured so the question is never asked twice.

## Reviewing Agent Work

Your attention is the scarcest resource in the system, and the harness is built around protecting it:

- Reports lead with a review guide: the riskiest hunk, the decision needing your confirmation, what is behavior change versus mechanical bulk. Read that first; it is where your five minutes matter.
- Commits arrive one intent at a time — mechanical churn separated from behavior change. If a diff arrives unreviewable, bounce it; splitting it is the agent's job, not yours.
- "It works" arrives as evidence: the exact command and the observed output. If a report says "should work," treat it as a defect.
- Refactor and improvement work carries its proof with it — baseline or frontier state, gate results. You review the claim against the artifact, not against trust.

## Making Corrections Stick

Correct once, in plain words: "we use the internal client here," "never touch the generated folder." The agent applies it, records it in the owning document (or proposes the recording, if the task was review-only), and tells you where — so you can veto or refine the wording.

**If you find yourself giving the same correction twice, the capture failed.** Say exactly that; fixing the instruction is more valuable than fixing the code.

## Running More Than One Agent

Two agents (or two sessions) in one repository are peers, and nothing coordinates them unless you do:

- One branch or worktree per agent per stream of work. Never point two agents at the same stream.
- Work streams are claimed through handoff notes in `plans/` — an agent will not advance a stream whose note another agent owns. Hand over by reassigning the note.
- Split work by stream, not by file: one agent on a feature branch, another reconciling or refactoring on its own branch, meeting only through review.
- Merge conflicts between peer agents come to you. Neither agent resolves them by force.
- Subagents an agent spawns for itself are its own business — you will mostly notice them as cost. Each runs its own context and bills independently, on every current runtime.

## Your Recurring Duties

The system stays honest through a few small human acts. Skipping them is how it rots:

1. **Answer escalations promptly** — a reserved-decision question blocks that stream until you do.
2. **Veto or accept dispositions** — reconciliation ledgers, correction captures, retro proposals. They are designed as accept/veto lists, not essays.
3. **Run the retro when it is due** (`scripts/receipt.sh check` tells you, or the agent will). The agent first reviews the previous retro's changes against evidence — keeping, amending, or reverting them — then proposes new ones from receipt patterns; you veto. This is the only mechanism by which the harness learns — twenty minutes a month is the entire cost.
4. **Spot-check receipts against reality** occasionally: a `shipped` receipt followed by three fix commits is rework the next retro should hear about.

## Day One

Fresh repository: `docs/project-adaptation.md`. Repository with existing agent instructions: point an agent at `docs/harness-reconciliation.md` and review its ledger. Either way: register the runtime profiles for the agents you actually use, and run the first retro after a handful of tasks — early routing mistakes are the cheapest ones to fix.

## Phrases That Work

| You want | Say |
| --- | --- |
| Cleanup without behavior change | "Refactor X. The acceptance gate is the full suite." |
| A metric pushed up | "Improve Y from 80 toward 90; noise floor is 1; nothing else may regress." |
| A spiral stopped | "Take a step back." |
| Proof, not promises | "Verify it end to end and show me the output." |
| The harness installed here | "Reconcile this repo with the harness at SHA <sha>, per docs/harness-reconciliation.md." |
| The system tuned | "Receipts say a retro is due — run it and bring me the proposals." |
| A second opinion | "Have a subagent review this before you call it done." |
| Emergency speed | "This is an emergency: suspend the gates, log what we skip, reconcile after." |
