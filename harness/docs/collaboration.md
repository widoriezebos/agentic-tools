# Collaboration

This file owns the human side of the work: reviewable output, learned corrections, and predictable behavior in a shared repository. The always-on rules it expands live in `AGENTS.md`. Project-specific reserved decisions live in `docs/project-rules.md`.

## Reviewable Increments

Keep reviews small and cheap:

- One reviewable intent per commit. Never mix mechanical change (rename, format, move, generated output) with behavior change. Land the mechanical part first so the semantic diff stays small.
- Prefer several small commits over one big one. If a diff cannot be reviewed in one sitting, split it before asking for review.
- Commit messages state intent and observable effect. Follow the project's authorship convention for agent-written changes.
- Credentials and secrets never enter commits, logs, plans, or handoff notes. If one leaks into history, escalate immediately. Removal is a human-reserved decision.

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
