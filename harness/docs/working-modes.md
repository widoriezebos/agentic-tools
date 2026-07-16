# Working Modes

The harness organizes work into modes. A mode is a promise about what will be true when you are done. Each mode has its own rules because each one fails in its own way. This guide explains every mode in plain language: why it exists, when you are in it, how it works, and how to use it.

This guide explains; it does not set rules. The binding rules live in each mode's own file, linked below. If this guide and a mode's file disagree, the mode's file wins.

## The modes at a glance

| Mode | You are in it when | "Done" means | Rules live in |
| --- | --- | --- | --- |
| **Implement** (default) | Asked to add, change, or fix behavior | The requested outcome exists and you watched it work | `AGENTS.md` and the completion check |
| **Design** | Deciding how something should be built, before building it | Decisions are written down, risks are named, and there is a plan to prove each one | `docs/design/` |
| **Refactor** | Changing structure while behavior must stay identical | The code is cleaner and provably behaves the same | `skills/refactor/SKILL.md` |
| **Improve** | Pushing a measured number up | The metric is provably better and nothing you promised to protect got worse | `skills/improve/SKILL.md` |
| **Take a step back** | The current approach is not working | You know why it failed and what to do instead | `skills/take-a-step-back/SKILL.md` |
| **Verify** (cross-cutting) | Before claiming anything works | You drove the real thing and watched it | `skills/verify/SKILL.md` |

## How the modes fit together

Every task starts in **implement**. You switch modes when the shape of the work changes:

- If the change is consequential (new boundaries, new failure behavior, choices that are hard to reverse), go through **design** first.
- If the task is restructuring with no behavior change, that is **refactor**.
- If the task is "make this number better", that is **improve**.
- Any mode can get stuck. **Take a step back** interrupts the mode you were in. It does not cancel that mode's rules.
- Every mode ends with **verify** and the completion check. No mode may end on "it should work".

Never mix refactor and improve in the same change. Refactor promises the score stays identical. Improve exists to move it. If you mix them, a failure cannot tell you which promise was broken. For the same reason, keep mechanical cleanup and behavior changes in separate commits.

## Implement (the default mode)

**Why it exists.** Most failures in ordinary work are mundane: acting on a stale assumption, doing more than was asked, or claiming success that was inferred instead of observed. Implement mode is the set of habits that prevents those.

**What it is for.** Features, bug fixes, configuration, documentation. The everyday work.

**How it works.** Look before you conclude: read the code, the tests, and the current state. Make the smallest change that satisfies the actual request. Resolve ambiguity in a fixed order: check the repository first; if the choice is reversible, make the smallest assumption and say so; if the choice affects contracts, scope, data, or what users see, ask first. Some decisions are never yours: anything on the project's reserved list (deploys, schema changes, new dependencies) goes to a human.

**How to use it.** Say you are asked to add rate limiting to an endpoint. Read how the endpoint and its middleware work today. Check `docs/project-rules.md` for commands and reserved decisions. Make the change and run the focused tests. Then actually hit the endpoint until it returns the limit response, and paste that evidence into your report. Finish with the five-question completion check in `docs/design/design-obligation-gate.md` and a receipt.

**Fixing a bug** adds one rule: reproduce it before you fix it. Capture the reproduction as a failing test when practical, make the fix, and show that the same test now passes. Then verify end to end as usual. A fix without a reproduction is a guess. The plausible cause and the demonstrated cause turn out to be different things more often than you would expect.

## Design (decide before you build)

**Why it exists.** Consequential changes fail late and expensively when nobody decided who owns a behavior, what happens on failure, or how the change will be proven. Design mode forces those decisions while they are still cheap.

**What it is for.** New components or boundaries, changes to failure, retry, or lifecycle behavior, anything with an expensive proof step, and any "should we build it this way?" question.

**How it works.** `docs/design/design-principles.md` gives a priority order for conflicts (correctness first, ceremony last) and eight questions you must be able to answer before implementing: the contract, the owner, each invariant, the failure behavior, the tests, the migration. The same document explains responsibility-driven design (behavior lives with the owner of the state it guards) and domain-driven design for systems with heavy business logic (domain language in code, bounded contexts, aggregates). For genuinely risky changes the obligation gate adds a tracking table: every critical requirement gets an owner, a code target, a test target, and a runtime proof, and the change is not done while any critical row is unproven. Most changes never need the table. The five-question default check is the whole gate.

**How to use it.** Read the filled example in `docs/examples/design-obligation-matrix.md`. It shows a webhook retry queue, four obligations, and what the gate stopped from being called done too early.

## Refactor (change the structure, keep the behavior)

**Why it exists.** Refactors fail differently from features. The damage is quiet, spreads wide, and shows up late. Green unit tests get mistaken for proof that behavior was preserved, failed refactors pile on top of each other, and by the time anything is visible nobody can say which change caused it.

**What it is for.** Readability work, extractions, moving code, de-duplication. Anything where the contract is "afterwards, it behaves exactly the same".

**How it works.**

- *Trusted baseline*: the last version that passed the project's full behavior-preservation proof (the acceptance gate: full suite, benchmark, or golden run). Everything after it is unproven until the gate passes again. `scripts/refactor-baseline.sh check` blocks new work when you have drifted too far from proven ground (default 24 hours or 40 commits; projects tune these, and the script's answer wins over the numbers printed here).
- *Tests before restructuring*: if a unit carries real behavior and weak tests, write the tests first, then refactor against them. If a test must change for your refactor to pass, it is no longer a refactor. Stop and escalate.
- *Batch sizing*: work in clusters big enough to matter and small enough to diagnose. Several related classes or one coherent package. Avoid single-class rituals and avoid "everything at once". Each unit is one commit that can be replayed alone.
- *Spend proof wisely*: run focused tests often, compile when signatures change, and save the full gate for cluster boundaries or real risk.
- *Bounded failure*: if the gate rejects your candidate, you get three focused fix attempts. Then revert the whole unit, write down what failed, and move on. Never build on a failed candidate.

**How to use it.** Declare the acceptance gate from `docs/project-rules.md`. Record the baseline. Run the check before each batch. Make replayable checkpoint commits. Gate at the cluster boundary and re-record the baseline on success.

## Improve (make a number go up without fooling yourself)

**Why it exists.** Chasing a score invites specific kinds of self-deception: treating random noise as progress, losing your best version by tinkering past it, and optimizing for the test instead of the thing the test stands for.

**What it is for.** Benchmark scores, quality evaluations, latency and cost budgets. Any goal a runnable evaluation can measure. If no such evaluation exists, building one is the first task.

**How it works.**

- *The contract comes first.* Before any run, write down the metric, the baseline score, the noise floor (the smallest change that means anything), the target, the guard metrics (numbers that must not get worse), and the budget.
- *The frontier* is your best version so far: exact commit, score, and the run that proved it. `scripts/frontier.sh` guards it. `challenge` refuses to call a run an improvement unless it beats the frontier by more than the noise floor, and `record` refuses to overwrite a better frontier with a worse one. When you beat the frontier, preserve that exact state before touching anything else.
- *One change per experiment.* Change five things and the score moves; you have learned nothing. Every experiment tests one mechanism, with a stated expectation, and gets classified honestly afterwards. The classifications come from take-a-step-back.
- *Do not overfit.* A gain you cannot explain is noise until it reproduces. Never change the evaluation and the system in the same commit. Do not tune against the same fixed cases forever.
- *Know when to stop*: target reached, budget spent, or three experiments in a row that beat nothing. Stop, and hand over the frontier and what you learned.

**How to use it.** Baseline 80, noise floor 1. An experiment scores 80.5: `challenge` says noise, so classify it, revert, and try the next mechanism. Another scores 82: `challenge` passes, so commit, record, and continue from there. Three failures in a row: stop and report, with the 82 preserved.

## Take a step back

**Why it exists.** Everyone tweaks in circles when stuck, humans and agents alike. Each attempt feels one fix away. An hour later nothing new is known and the budget is gone. This mode replaces momentum with evidence, and it contains the stop-loss rules that end an investigation for you, because in the moment you will not want to stop.

**When you are in it.** You are repeating one mechanism with small variations. You cannot say what new fact the last attempt produced. A run is expensive and you are about to repeat it. Someone says "step back". Any of these applies, from any other mode.

**How it works.** Freeze the frame first: exact symptom, exact state, what has been tried, and a budget. Keep two to four theories that evidence could kill, and hunt the evidence that decides between them instead of the evidence that comforts you. Before every attempt, write a short contract: what question this answers, what it costs, and when you will stop. Classify every result honestly: did the contract improve, did a theory die usefully, or did nothing change? The stop-loss triggers (two no-progress results, one expensive run that taught nothing, a dead end) are mandatory. When one fires, stop, preserve what was learned, revert what failed, and take the decision up a level.

**How to use it.** Read the filled example in `docs/examples/step-back-ledger.md`. A flaky test, two blind patches already wasted, and two disciplined cycles that found a connection leak the patches never would have.

## Verify

**Why it exists.** "Tests pass" and "it works" are different claims. The single most trust-destroying thing an agent does is claim success it never observed.

**How it works.** Name the behavior your change claims to alter, as a user would see it. Drive it through a real entrypoint: the running app, the CLI, the API. Include one failure or boundary case when error handling changed. Capture the command and the output; those replace the sentence "it works". If nothing runnable exists, say plainly that runtime verification did not happen. If making verification pass would mean weakening a test, stop. That is a contract change and a human decides it.

## Choosing a mode in awkward cases

- A cleanup that also fixes a bug is two changes: fix the bug in implement mode, restructure in refactor mode, in separate commits, in whichever order is safer.
- An improvement that needs restructuring first: refactor first (the score must hold), prove it, then improve (the score may move). Never both at once.
- Stuck mid-refactor or mid-improvement: step back. The investigation follows step-back rules. The baseline and frontier rules of the paused mode still stand.
- Not sure whether design is needed: answer the five-question default check. If any answer requires deciding ownership, failure behavior, or an invariant you cannot name, that is design work. Do it first.
- Not sure at all: `wow.md` is the routing table. When two modes seem to apply, pick the one with the stricter promise.

## The retro

One workflow sits above all the modes: the retro (`skills/retro/SKILL.md`). It is how the harness improves itself. Every repo-changing task leaves a one-line receipt. When enough accumulate, a retro runs. It treats the harness's own rules the way improve mode treats code: every rule change was adopted with a written, testable expectation ("correction X never repeats"), and the next retro checks whether that expectation came true, then keeps, amends, or reverts the change. A rule that cannot show its value after two reviews is removed by default. Humans veto every change; nothing rewrites the rules automatically. Over time this should make the harness smaller and more precise.
