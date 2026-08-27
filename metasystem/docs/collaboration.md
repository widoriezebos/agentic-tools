# Collaboration

This file owns the human side of the work: reviewable output, learned corrections, and predictable behavior in a shared repository. The always-on rules it expands live in `AGENTS.md`. Project-specific reserved decisions live in `docs/project-rules.md`.

## Write to a human

Every report a human reads — a turn summary, a delegate return, a refusal
message, a commit message — is written for the person, not for the machine
that produced it. Concretely:

- Spell out an identifier the first time it appears in a report: "KI-4, the
  slow process scan", not "KI-4". Ids are bookmarks, never explanations.
- Say what a number means, not only its value: "442ms per scan, which is
  longer than the 250ms gap between scans, so it can never catch up".
- Never let a status line be made of ids, paths and jargon alone.
- Prefer the sentence a colleague would understand without the repository
  open. If it would not survive being read aloud to them, rewrite it.
- Plain does not mean vague: keep the verdict, the evidence level, and the
  uncertainty. Say the hard thing clearly rather than hiding it in shorthand.

This is a standing rule in `AGENTS.md`, restated here because this document
owns reporting; the role preambles carry it for delegates, so it holds for
every runtime.

## Reviewable Increments

Keep reviews small and cheap:

- One reviewable intent per commit. Never mix mechanical change (rename, format, move, generated output) with behavior change. Land the mechanical part first so the semantic diff stays small.
- Prefer several small commits over one big one. If a diff cannot be reviewed in one sitting, split it before asking for review.
- Stage commits by explicit path. An add-everything commit sweeps up whatever else is in the tree, and with delegates or peer agents active that can include another stream's uncommitted work.
- Commit messages state intent and observable effect. Follow the project's authorship convention for agent-written changes.
- Credentials and secrets never enter commits, logs, plans, or handoff notes. If one leaks into history, escalate immediately. Removal is a human-reserved decision.

## The Milestone Battery, Weight-Triggered

Verification is tiered so ordinary landings stay fast and accumulated change
still receives the expensive proof. While a change is moving, and at its
landing boundary, run the TOUCHED-SURFACE tier: the changed packages' tests,
the fixture legs the diff touches, and the static proof built from the
prospective landing. The landing wrapper weighs exactly the behavior-surface
policy's `LANDING` projection. Coordination-only changes weigh zero; the due
line is a scheduling nudge, never a landing refusal.

When accumulated feature weight reaches the configured threshold, run the
FULL milestone battery once with:

```sh
scripts/agents/milestone-battery.sh
```

That command records the current commit, creates an independent local clone
detached at it, builds and validates with that clone's own engine and the
shared Go cache, and reports the exact subject commit it judged. Live goal
writes, commits, rebases, checkouts, configuration edits, and supervision use
different state and cannot change the recorded subject. The clone receives
only the committed generic battery configuration; the live `conf.local` and
the live supervision registry are not copied.

Every outcome publishes a run-scoped evidence envelope to the durable
evidence home before teardown. A green run then consumes only the accumulator
portion it checkpointed, preserving landings that arrived during validation.
A red or aborted run abandons its checkpoint and resets nothing. Evidence-copy
failure retains the clone and forbids reset; reset-appendix publication is
repairable from the accumulator on the next weight read. Findings from a
milestone battery fix forward.

The isolated validation root publishes one structural run class. `FULL` means
that root performed the complete engine proof itself; witness reuse by its
descendants is only deduplication and does not change the class.
`WITNESS-ASSISTED` means the root imported engine proof. Only `FULL` consumes
the weight checkpoint. A witness-assisted run abandons the checkpoint without
subtracting weight, and a conclusion that relies on milestone-battery
acceptance must procedurally cite a `FULL` run.

Two rules keep its wall clock and proof transfer honest:

- One suite run, not two. The engine gate (`scripts/agents/go-gate.sh`) runs
  the race suite with coverage; a separate plain `go test ./...` before it
  proves nothing the gate does not re-prove.
- One proof per byte projection. The versioned behavior-surface owner names
  `ENGINE`, `LANDING`, and `PAYLOAD` separately. Any explicit `ENGINE`
  consumer descended from the witness's exact live controller may skip only
  the policy's witness-engine family when policy version, `ENGINE` digest, and
  independent toolchain identity match. Delivery reuse is a separate policy
  scope: it additionally requires `PAYLOAD` equality and a rebuilt-binary
  stamp before an enumerated delivery family may skip. A mismatch runs the
  omitted proof instead of accepting reuse.

## Review Guide in Reports

Start every completion report with where to look first: the riskiest hunk, the decision that most needs human confirmation, and which parts are behavior change versus mechanical bulk. A report that buries the one dangerous line under twenty safe ones has failed even if the code is correct.

## Correction Capture

A user correction of a convention, preference, or fact ("we use X here", "never touch Y") means the instructions need updating, in addition to the immediate fix:

1. Apply the correction to the work at hand.
2. Persist it to its one owning document (usually `docs/project-rules.md`; a workflow lesson may belong to a skill or design doc per `wow.md`), but only when the task already authorizes repository edits. In review-only or explain-only work, do not edit files. Propose the exact capture (file and wording) in the report instead.
3. Say where it was recorded or proposed so the human can veto or refine it. If unsure whether a correction is personal preference or durable project policy, ask before persisting.

One rule, one home still applies: update the owner, do not scatter copies. A correction repeated across sessions means the capture failed. Fix the instruction as well as the code.

## Answering and Reporting

The counterpart of reviewable code is a readable answer. Detail that buries the point costs the reader the same way an unreviewable diff does.

- Answer the question first. The verdict, the number, or the yes or no goes in the opening lines, then the evidence. A reader who stops after one paragraph must leave with the right conclusion.
- Rank by what matters. The most important finding comes first and the rest follow in falling order. Never hide the one dangerous fact among ten harmless ones.
- Give honest verdicts. "No", "partially", and "I introduced this bug" are complete answers. Do not soften a finding into vagueness, and do not inflate a nothing into a finding.
- Make detail proportional to the stakes. One sentence for a small thing, depth only where a decision depends on it. When a list has thirty items and three matter, name the three and summarize the rest.
- Mark the evidence level: verified by running it, checked by reading it, or inferred. Never present an inference as an observation.
- State what was not done: not run, not read, not covered. An answer that hides its gaps is wrong even when every sentence in it is true.
- End with the decision or next step that belongs to the human, when there is one. Do not end with a summary that restates the answer.
- Use a table when facts are parallel, prose when reasoning matters, and nothing when neither helps. Skip preambles, restated questions, and filler.

Asking follows the same economy: one question, the smallest set of real options, a recommendation, and what each option costs (see Escalation Shape below).

## Escalation Shape

When a reserved or ambiguous decision blocks progress, ask with a recommendation and the smallest set of real options, stating what each costs. Do not ask about decisions the code or conventions already answer, and do not proceed on a reserved decision because asking felt expensive.

## Emergencies

The human may explicitly suspend gates and checks for a declared emergency. Suspension is always their explicit call; never infer it from urgency. Record what was skipped in the receipt or handoff note, and reconcile (run the skipped verification, backfill the ledgers) as the first task after the incident.

## Coordinator session capacity

The Claude Code harness caps each session at 200 spawned subagents
(a runaway-loop backstop). A continuous coordinator session dispatches
an agent for every build, critique, and fix round, so 200 is roughly
one working day — hitting the cap mid-queue stalls lawful work
(first sighting 2026-08-27 01:30, five idle hours). Launch
coordinator sessions with the limit raised:

    export CLAUDE_CODE_MAX_SUBAGENTS_PER_SESSION=2000

Put it in the shell profile of any machine that runs a coordinator.
The variable is read at session start; a capped session cannot raise
it from inside — there, dispatch codex work directly through the
companion CLI, which the cap does not govern.
