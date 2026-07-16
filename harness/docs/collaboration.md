# Collaboration

This file owns the human side of the work: reviewable output, learned corrections, and predictable behavior in a shared repository. The always-on rules it expands live in `AGENTS.md`; project-specific reserved decisions live in `docs/project-rules.md`.

## Reviewable Increments

Human review attention is the scarcest resource in agent-assisted work. Protect it:

- One reviewable intent per commit. Never mix mechanical change (rename, format, move, generated output) with behavior change; land the mechanical part first so the semantic diff stays small.
- Prefer several small commits over one omnibus. If a diff cannot be reviewed in one sitting, split it before asking for review.
- Commit messages state intent and observable effect, not file lists. Follow the project's authorship convention for agent-written changes.
- Credentials and secrets never enter commits, logs, plans, or handoff notes. If one leaks into history, escalate immediately — removal is a human-reserved decision.

## Review Guide in Reports

Lead every completion report with where to look first: the riskiest hunk, the decision that most needs human confirmation, and which parts are behavior change versus mechanical bulk. A report that buries the one dangerous line under twenty safe ones has failed even if the code is correct.

## Correction Capture

A user correction of a convention, preference, or fact ("we use X here", "never touch Y") is an instruction defect, not just a task fix:

1. Apply the correction to the work at hand.
2. Persist it to its one owning document — usually `docs/project-rules.md`; a workflow lesson may belong to a skill or design doc per `wow.md` — but only when the task already authorizes repository edits. In review-only or explain-only work, do not edit files: propose the exact capture (file and wording) in the report instead.
3. Say where it was recorded or proposed so the human can veto or refine it. If unsure whether a correction is personal preference or durable project policy, ask before persisting.

One rule, one home still applies: update the owner, do not scatter copies. A correction repeated across sessions means the capture failed — fix the instruction, not just the code.

## Escalation Shape

When a reserved or ambiguous decision blocks progress, ask with a recommendation and the smallest set of real options, stating what each costs. Do not ask about decisions the code or conventions already answer, and do not proceed on a reserved decision because asking felt expensive.

## Emergencies

The human may explicitly suspend gates and checks for a declared emergency; suspension is always their explicit call, never inferred from urgency. Record what was skipped in the receipt or handoff note, and reconcile — run the skipped verification, backfill the ledgers — as the first task after the incident.
