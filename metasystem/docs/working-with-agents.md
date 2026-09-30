# Working With Agents

The rest of the metasystem instructs agents. This document is for the humans: how to hand work to one or more agents, what to expect back, and the few duties only a human can do. The agent-facing counterpart is `docs/collaboration.md`. The modes themselves are explained in `docs/working-modes.md`.

## What you can expect

An agent operating under this metasystem will inspect the repository before concluding anything, make the smallest change that satisfies the request, state its assumptions, stop and ask before decisions reserved for you, prove its work by running it, answer your question before giving the background, lead its report with the part you should look at first, and leave a receipt plus (for unfinished work) a handoff note so the next session starts warm.

If an agent is not doing these things, say so. Corrections about how it works are captured the same way as corrections about the code.

## Handing over work

- State the outcome instead of the steps. "Users should stop seeing duplicate emails" works better than a list of files to edit. The agent inspects; you decide what "good" means.
- Name the mode when you know it. The mode words trigger specific rules: "refactor this" promises unchanged behavior and activates the baseline rules, "improve the p95 latency" activates the frontier and noise-floor rules, "take a step back" stops a spiral, and "verify it end to end" demands observed proof.
- Give the acceptance criterion up front when there is one: the test that should pass, the score to beat, the behavior to observe.
- Name the non-goals when scope could creep, for example "do not touch the public API".
- Expect ambiguity to come back either as a stated, reversible assumption or as one question with a recommendation. Answer the question asked; you rarely need to re-explain the task.

## Decisions that come back to you

Some calls are yours by design, and the agent will stop for them: production deployments and data, API or schema contracts, new dependencies, deleting user-visible behavior, spending past a stated budget or onto a costlier resource tier, and anything on your project's reserved list in `docs/project-rules.md`. A few less obvious ones:

- A red test is a question for you. The agent will never weaken or delete a failing test to get to green; it will ask whether the contract changed.
- Changing an evaluation re-baselines the improvement frontier, so the agent asks before doing both at once.
- A budget running out arrives as one batched ask (spend so far, what it bought, the remaining options), never as silent overage and never as one question per run.
- A fired stop-loss comes to you as evidence plus a decision, never as more attempts. The agent stops when an investigation records a dead end, two cycles without progress, or an exhausted budget. That is the mechanism working. The useful responses are a decision, a redesign, or a bigger budget; "just try once more" by reflex is the thing the stop-loss exists to prevent.

When you answer, decide briefly and say why. The reason is what gets captured so the question is never asked twice.

## Reviewing agent work

The metasystem tries to keep reviews small and predictable:

- Reports start with a review guide: the riskiest hunk, the decision that needs your confirmation, and which parts are behavior change versus mechanical bulk. Read that first.
- Commits arrive one intent at a time, with mechanical churn separated from behavior change. If a diff arrives unreviewable, send it back. Splitting it is the agent's job.
- "It works" arrives as evidence: the exact command and the observed output. Treat a report that says "should work" as a defect.
- Refactor and improvement work carries its proof with it: baseline or frontier state and gate results. Review the claim against the artifact.

## Making corrections stick

Correct once, in plain words: "we use the internal client here", "never touch the generated folder". The agent applies it, records it in the owning document (or proposes the recording, if the task was review-only), and tells you where, so you can veto or refine the wording.

If you find yourself giving the same correction twice, the capture failed. Say exactly that. Fixing the instruction is worth more than fixing the code again.

## Running more than one agent

Two agents, or two sessions, in one repository are peers. Nothing coordinates them unless you do:

- One branch or worktree per agent per stream of work. Never point two agents at the same stream.
- Work streams are claimed through handoff notes in `plans/`. A claim only exists once the note is pushed to the shared default branch. An agent will not advance a stream whose note another agent owns; hand over by reassigning the note.
- Split work by stream rather than by file: one agent on a feature branch, another reconciling or refactoring on its own branch, meeting only in review.
- Merge conflicts between peer agents come to you. Neither agent resolves them by force.
- Subagents an agent spawns for itself are its own business. You mostly notice them as cost: each runs its own context and bills separately on every current runtime.

## Driving the work yourself

Commands say what you want done, not how the system is built. Run them from any directory inside the repository, or pass `--repo PATH` naming the repository top or any directory below it. `metasystem help human` is the one list of your actions, with their arguments; `metasystem help OBJECT ACTION` shows the full grammar and examples for one of them. Each line below is a separate example, not a sequence to run in order:

```sh
metasystem goal list                              # what is open
metasystem goal open faster-proof --intent 'Proof runs in half the time.' --next 'Measure the slowest step.' --risk severity=1,novelty=1,exposure=1,accumulation=1 --basis 'local test tooling only'
metasystem goal show faster-landing                # one goal: state, budget, next step, design
metasystem goal approve faster-landing             # approve it at its tier's normal budget
metasystem goal budget faster-landing              # read the budget; `budget G BOX` changes it
metasystem goal pause faster-landing --reason "waiting for the vendor fix"
metasystem goal resume faster-landing              # back on its standing approved budget
metasystem goal done faster-landing --reason "landed in 3f2a1c0, verified live"   # only once it has actually landed and been verified
```

The same pattern covers opening and editing goals (`goal open`, `goal edit`), starting, stopping and restarting this checkout, the browser interface, a mission or a new machine (`system start`, `system stop`, `system restart`, `ui start`, `mission start M`, `machine start NAME`), seeing and stopping everything MetaSystem runs on this computer (`machine list --verbose`, `machine stop --all`), enrolling your terminal (`system enroll --name NAME`), answering a question (`question answer Q [TEXT]`), accepting a finding's risk (`goal accept-risk`), diagnosing problems (`system check`), trimming the machine's Go and staticcheck caches to their caps now (`disk clean --go-cache`), bringing hand-edited goal files and the ledger into agreement (`goal sync`), and settings (`settings show`, `settings check`, `settings coordinator`). Agents use the matching agent actions (`goal claim`, `work brief`, `work build`, `work review`, `work revise`, `work wait`, `work land`, `question ask`), listed by `metasystem help agent`; `metasystem help all` lists every command.

Two things these commands never do on your behalf. They never make a decision that is yours: when a human act or a missing answer is needed, the command stops and names it rather than guessing. And a successful `build` ends awaiting review: built, proved and given a preliminary read that is feedback only; `review G` then has the result examined independently, completes it when there are no findings, and asks for your decisions when there are. Landing and concluding a goal stay separate, explicit acts.

Use `metasystem help administration` for MetaSystem setup and maintenance. Application work is listed separately in human, agent and command help.

## Running the landing lane

The landing lane is the one checkout per computer where every seat's work is proved and pushed (`docs/concepts.md`, "Landing lane", says why). A computer works without one, with each seat landing its work itself, but slower.

**Set one up on a new computer.** Clone a checkout next to the seats' checkouts, give it a machine nickname, build its engine, set it up, register it as the lane and start it, at your own terminal:

```sh
git clone https://github.com/OWNER/REPO.git ~/src/repo-landing
git -C ~/src/repo-landing config metasystem.goal.machine landing
cd ~/src/repo-landing/metasystem && go run ./cmd/devgate build
bin/metasystem system setup
bin/metasystem landing set ~/src/repo-landing
bin/metasystem system start --repo ~/src/repo-landing
```

`landing set` refuses a checkout without a machine nickname: the lane signs what it lands with that name. `system start` must be run by you, at a terminal no agent started; it starts the lane's supervision, and the lane's owner with it. No agent ever works in this checkout, and no seat needs a restart: each lands through the lane from its next `work land`.

**Check it.**

```sh
metasystem landing status             # one line: where the lane is, whether its owner runs, the batch it proves, the next one collecting
metasystem landing status --verbose   # adds who registered it, the owner's pid and restarts, and each batch's members
metasystem machine list --verbose     # every machine of this computer, the lane's checkout and its owner included
```

The browser interface's Fleet page shows the same lane line.

**Stop, restart or move it.**

```sh
metasystem landing stop                        # pause the owner for maintenance; nothing lands until landing start
metasystem landing start                       # end the pause, clear the restart count, start the owner
metasystem landing restart                     # stop and start the owner, for example when it seems stuck
metasystem landing set ~/src/repo-landing-2    # move the lane to another checkout
metasystem landing unset                       # take the lane away: its members go back to their seats, each seat lands its own work
metasystem machine stop landing                # stop MetaSystem in the lane's checkout altogether
```

`landing set`, `landing unset` and resuming a stopped lane (`landing start`, `landing restart`) are yours to run, at an enrolled terminal; an agent is refused. A stopped lane still takes the seats' work, which waits in it: `landing unset` is the way back to each seat landing its own work. It stops and lists what is left when a push or a proof still runs, or when a returned member does not read back as returned; the same command continues, and `--force` goes past state that cannot be known. `landing stop` and `landing restart` are refused while a batch is pushing to main, and moving the lane is refused while a batch proves or pushes in it; each refusal says to wait, or to pause the lane with `landing stop` first. To put the lane on a newly built engine, rebuild it in the lane's checkout and run `metasystem system restart --repo PATH` there.

**When it is down.** Seats keep going: `work land` still joins the lane's next batch, and the work waits there until the owner runs again. No seat quietly switches to landing the work itself. The steward restarts a dead owner on its own, with growing pauses in between, and gives up after five deaths in a row. `landing status` then reads, for example, `owner given-up after 5 restarts; last error: ...; to fix: ...`: the last error and the one command that fixes it. Typical causes and fixes:

- the lane's checkout is not started: `metasystem system start --repo PATH` at your terminal;
- it has no machine nickname: `git -C PATH config metasystem.goal.machine landing`, then `metasystem landing start`;
- the owner keeps dying: read the error file `landing status` names, fix the cause, then `metasystem landing start`.

## Your recurring duties

The system stays honest through a few small human acts:

1. **Answer escalations promptly.** A reserved-decision question blocks that stream until you do. The stream's handoff note keeps the standing list of everything waiting on you.
2. **Accept or veto dispositions**: reconciliation ledgers, correction captures, retro proposals. They are designed as short lists you can approve item by item.
3. **Run the retro when it is due** (`metasystem receipt status` tells you, or the agent will). The agent first reviews the previous retro's changes against evidence, keeping, amending, or reverting them, then proposes new ones from receipt patterns. You veto. This is the only mechanism by which the metasystem learns, and it costs about twenty minutes a month.
4. **Spot-check receipts against reality** now and then. A "shipped" receipt followed by three fix commits is rework the next retro should hear about.

## Day one

Fresh repository: `docs/project-adaptation.md`. Repository with existing agent instructions: point an agent at `docs/metasystem-reconciliation.md` and review its ledger. Either way, register the skills and profiles for the runtimes you actually use, and run the first retro after a handful of tasks. Early routing mistakes are the cheapest ones to fix.

## Phrases that work

| You want | Say |
| --- | --- |
| Cleanup without behavior change | "Refactor X. The acceptance gate is the full suite." |
| A metric pushed up | "Improve Y from 80 toward 90; noise floor is 1; nothing else may regress." |
| A spiral stopped | "Take a step back." |
| Proof instead of promises | "Verify it end to end and show me the output." |
| The metasystem installed here | "Reconcile this repo with the metasystem at SHA <sha>, per docs/metasystem-reconciliation.md." |
| The system tuned | "Receipts say a retro is due. Run it and bring me the proposals." |
| A second opinion | "Have a subagent review this before you call it done." |
| Emergency speed | "This is an emergency: suspend the gates, log what we skip, reconcile after." |
