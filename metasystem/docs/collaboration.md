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

## Weight-Triggered Direct Validation

Ordinary landings keep the existing touched-package, touched-fixture, and
static checks. They add no governance command or form. The landing path only
adds behavior-surface weight; when the configured threshold is due, the
standing validator's custodian runs the retained validator directly through
the governed run boundary:

```sh
bin/metasystem run launch --root "$PWD" --id "direct-validation-<id>" \
  --kind suite --display "weight-triggered direct validation" \
  --log "artifacts/agents/runs/direct-validation-<id>.log" \
  --goal "<standing-validator-goal>" --obligation-revision "<revision>" \
  --standing-shared-process -- scripts/validate-metasystem.sh
```

Watch that exact run with the command printed by `run launch`. A green run may
discharge weight only through `gate weight-discharge` with the same goal,
obligation revision, and run id. Direct shell diagnostics remain free to run,
but cannot discharge weight or another obligation.

The retirement observation window is the next two weight-triggered direct
validations. The steward is the observer and mechanically compares their
stage-result section ids with the retained catch classes. Wido is the
custodian. Findings fix forward; no per-landing gate or retry is introduced.

Ownership of the retained overlaps is explicit. `commit.sh` alone executes the
per-landing coverage delta; `land.sh` only passes its ratchet argument through.
`go-gate.sh` owns the repository-wide coverage ratchet. Wido owns the review,
after the next declared milestone, of whether that sweep still catches debt
the landing delta cannot. `validate-metasystem.sh` owns VM delivery and
guest-runtime invariants; `adopt-fixtures.sh` owns installed-tree and update
invariants. Wido reviews those distinct owners after the same two-run
retirement window. The steward puts a due review in its single ceilinged
ruling digest; none has a per-row delivery.

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
(a runaway-loop backstop). A continuous seat session claims the
dispatch-delegate role for each build, critique, and fix-round dispatch,
so 200 is roughly
one working day — hitting the cap mid-queue stalls lawful work
(first sighting 2026-08-27 01:30, five idle hours). Launch
coordinator sessions with the limit raised:

    export CLAUDE_CODE_MAX_SUBAGENTS_PER_SESSION=2000

Put it in the shell profile of any machine that runs a coordinator.
The variable is read at session start; a capped session cannot raise
it from inside — there, dispatch codex work directly through the
companion CLI, which the cap does not govern.
