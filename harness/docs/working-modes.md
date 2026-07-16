# Working Modes — A Plain-English Guide

The harness organizes work into modes. A mode is a promise about what will be true when you are done, and each mode has its own rules because each one fails in a different way. This guide explains every mode in plain language: why it exists, when you are in it, how it works, and how to use it.

This guide explains; it does not legislate. The binding rules live in each mode's own file, linked below. If this guide and a mode's file ever disagree, the mode's file wins.

## The Modes at a Glance

| Mode | You are in it when | "Done" means | Rules live in |
| --- | --- | --- | --- |
| **Implement** (default) | Asked to add, change, or fix behavior | The requested outcome exists and you watched it work | `AGENTS.md` + the completion check |
| **Design** | Deciding how something should be built, before building it | Decisions are written down, risks are named, and there is a plan to prove each one | `docs/design/` |
| **Refactor** | Changing structure while behavior must stay identical | The code is cleaner and provably behaves the same | `skills/refactor/SKILL.md` |
| **Improve** | Pushing a measured number up | The metric is provably better and nothing you promised to protect got worse | `skills/improve/SKILL.md` |
| **Take a step back** | The current approach is not working | You know why it failed and what to do instead | `skills/take-a-step-back/SKILL.md` |
| **Verify** (cross-cutting) | Before claiming anything works | You drove the real thing and watched it | `skills/verify/SKILL.md` |

## How the Modes Fit Together

Every task starts in **implement**. You switch modes when the shape of the work changes:

- The change is consequential — new boundaries, new failure behavior, hard-to-reverse choices? Pass through **design** first.
- The task is restructuring with no behavior change? That is **refactor**.
- The task is "make this number better"? That is **improve**.
- Any mode can get stuck. **Take a step back** is the escape hatch from all of them — it pauses the mode you were in, it does not cancel its rules.
- Every mode exits through **verify** and the completion check. No mode is allowed to end on "it should work."

One rule matters more than any other: **never mix refactor and improve in the same change.** Refactor promises the score stays identical; improve exists to move it. Mix them and a failure cannot tell you which promise was broken. The same logic says: keep mechanical cleanup and behavior changes in separate commits, always.

## Implement — the default mode

**Why it exists.** Most failures in ordinary work are not exotic: acting on a stale assumption, doing more than was asked, claiming success that was inferred rather than observed. Implement mode is the set of habits that prevents those.

**What it is for.** Features, bug fixes, configuration, documentation — the everyday work.

**How it works.** Look before you conclude (read the code, the tests, the current state). Make the smallest robust change that satisfies the actual request. Resolve ambiguity in a fixed order: check the repository first, then make the smallest reversible assumption and say so, and ask only when the choice affects contracts, scope, data, or what users see. Certain decisions are never yours: anything on the project's reserved list (deploys, schema changes, new dependencies) goes to a human first.

**How to use it.** Asked to add rate limiting to an endpoint: read how the endpoint and its middleware work today; check `docs/project-rules.md` for commands and reserved decisions; make the change; run the focused tests; then actually hit the endpoint until it returns the limit response and paste that evidence into your report. Finish with the five-question completion check in `docs/design/design-obligation-gate.md` and a receipt.

## Design — decide before you build

**Why it exists.** Consequential changes fail late and expensively when nobody decided who owns a behavior, what happens on failure, or how the change will be proven. Design mode forces those decisions while they are still cheap.

**What it is for.** New components or boundaries, changes to failure/retry/lifecycle behavior, anything with an expensive proof step, any "should we build it this way?" question.

**How it works.** `docs/design/design-principles.md` gives a priority order for conflicts (correctness first, ceremony last) and eight questions you must be able to answer before implementing — the contract, the owner, each invariant, the failure behavior, the tests, the migration. For genuinely risky changes, the obligation gate adds a tracking table: every critical requirement gets an owner, a code target, a test target, and a runtime proof, and the change is not done while any critical row is not proven. Most changes never need the table — the five-question default check is the whole gate.

**How to use it.** Read the filled example in `docs/examples/design-obligation-matrix.md` — a webhook retry queue, four obligations, and what the gate stopped from being called "done" too early.

## Refactor — change the structure, keep the behavior

**Why it exists.** Refactors fail differently from features: silently, broadly, and late. Green unit tests get mistaken for proof that behavior is preserved, failed refactors accumulate on top of each other, and by the time the damage is visible nobody can say which change caused it.

**What it is for.** Readability work, extractions, moving code, de-duplication — anything whose contract is "afterwards, it behaves exactly the same."

**How it works, in plain words.**

- *Trusted baseline*: the last version that passed the project's full behavior-preservation proof (its "acceptance gate" — full suite, benchmark, or golden run). Everything after it is unproven until the gate passes again. `scripts/refactor-baseline.sh check` blocks new work when you have drifted too far from proven ground (default: 24 hours or 40 commits).
- *Tests before restructuring*: if a unit carries real behavior and weak tests, write the tests first, then refactor against them. If a test must change for your refactor to pass, that is not a refactor anymore — stop and escalate.
- *Batch sizing*: work in clusters big enough to matter and small enough to diagnose — several related classes, one coherent package — never a one-class ritual, and never "everything at once." Each unit is one commit that can be replayed alone.
- *Spend proof wisely*: focused tests often, compiles when signatures change, the full gate only at cluster boundaries or when risk demands it.
- *Bounded failure*: if the gate rejects your candidate, you get three focused fix attempts. Then revert the whole unit, write down what failed, and move to the next one. Never build on a failed candidate.

**How to use it.** Declare the acceptance gate from `docs/project-rules.md`; record the baseline; before each batch run the check; make replayable checkpoint commits; gate at the cluster boundary; re-record the baseline on success.

## Improve — make a number go up, without fooling yourself

**Why it exists.** Chasing a score invites three specific ways of self-deception: celebrating a random fluctuation as progress, losing your best version because you kept tinkering past it, and optimizing for the test instead of the thing the test stands for.

**What it is for.** Benchmark scores, quality evaluations, latency and cost budgets — any goal a runnable evaluation can measure. If no such evaluation exists, building one is the first task.

**How it works, in plain words.**

- *The contract comes first*: before any run, write down the metric, the baseline score, the noise floor (the smallest change that means anything), the target, the guard metrics (numbers that must not get worse), and the budget.
- *The frontier*: your best version so far — exact commit, score, and the run that proved it. `scripts/frontier.sh` guards it: `challenge` refuses to call a run an improvement unless it beats the frontier by more than the noise floor, and `record` refuses to overwrite a better frontier with a worse one. When you do beat it, preserve that exact state before touching anything else.
- *One change per experiment*: change five things and the score moves — you have learned nothing. Every experiment tests one mechanism, with a stated expectation, and gets classified honestly afterwards (the classifications come from take-a-step-back).
- *Do not overfit*: a gain you cannot explain is noise until reproduced. Never change the evaluation and the system in the same commit. Do not tune against the same fixed cases forever.
- *Know when to stop*: target reached, budget spent, or three experiments in a row that beat nothing — stop, hand over the frontier and what you learned.

**How to use it.** Baseline 80, noise floor 1. An experiment scores 80.5 — `challenge` says noise; classify, revert, next mechanism. Another scores 82 — `challenge` passes; commit, record, continue from there. Three failures in a row — stop and report, with the 82 preserved.

## Take a Step Back — the escape hatch

**Why it exists.** Everyone — humans and agents — tweaks in circles when stuck. Each attempt feels one fix away; an hour later nothing new is known and the budget is gone. This mode replaces momentum with evidence, and it contains the stop-loss rules that end an investigation *for* you, because in the moment you will not want to stop.

**When you are in it.** You are repeating one mechanism with small variations. You cannot say what new fact the last attempt produced. A run is expensive and you are about to repeat it. Someone says "step back." Any of these — from any other mode.

**How it works, in plain words.** Freeze the frame first: exact symptom, exact state, what has been tried, and a budget. Keep two to four theories that evidence could actually kill, and hunt the evidence that decides between them — not the evidence that comforts you. Before every attempt, write a short contract: what question this answers, what it costs, and when you will stop. Classify every result honestly: did the contract actually improve, did a theory die usefully, or did nothing change? The stop-loss triggers (two no-progress results, one expensive run that taught nothing, a dead end) are not suggestions — when one fires, you stop, preserve what was learned, revert what failed, and take the decision up a level.

**How to use it.** Read the filled example in `docs/examples/step-back-ledger.md` — a flaky test, two blind patches already wasted, and how two disciplined cycles found a connection leak the patches never would have.

## Verify — the exit everyone shares

**Why it exists.** "Tests pass" is not "it works." The single most trust-destroying thing an agent does is claim success it never observed.

**How it works.** Name the behavior your change claims to alter, as a user would see it. Drive it through a real entrypoint — the running app, the CLI, the API — including one failure or boundary case when error handling changed. Capture the command and the output; those replace the sentence "it works." If nothing runnable exists, say plainly that runtime verification did not happen. And if making verification pass would mean weakening a test — stop; that is a contract change for a human.

## Choosing a Mode — the awkward cases

- **A cleanup that also fixes a bug** is two changes: fix the bug in implement mode, restructure in refactor mode, separate commits — whichever order is safer.
- **An improvement that needs restructuring first**: refactor first (score must hold), prove it, then improve (score may move). Never both at once.
- **Stuck mid-refactor or mid-improvement**: step back. The investigation follows step-back rules; the baseline and frontier rules of the paused mode still stand.
- **Not sure whether design is needed**: answer the five-question default check; if any answer requires deciding ownership, failure behavior, or an invariant you cannot name — that is design work, do it first.
- **Not sure at all**: `wow.md` is the routing table. When two modes seem to apply, the one with the stricter promise wins.
