# Project Adaptation

These steps assume a repository without existing agent instruction assets. If the repository already has agent contracts, skills, prompts, or rule files, follow `docs/metasystem-reconciliation.md`, which wraps these steps with inventory, classification, and cutover.

The fast path is the adopter script, run from the template checkout: `scripts/adopt.sh <target> [--runtimes <names>] [--enable debug-java]` (the adoptable set and default come from `bin/metasystem runtime list --adoptable` and `bin/metasystem runtime adoption-default`). It performs steps 1, 3, 4, 8, and 9 below mechanically, refuses targets that need reconciliation instead of guessing at them, and prints the finish line: fill `docs/project-rules.md` (step 2), then run `scripts/validate-metasystem.sh`, which must pass with zero placeholders. A script can detect only file-shaped instruction assets; confirm by hand that no agent-directed prose, prompt directories under other names, or agent-encoding hooks and CI exist before treating a repository as fresh. The numbered steps below remain the specification and the manual fallback. The guided path through the human half — step 2's facts plus covenant v1, doctrine v1, and the first goals — is the inception interview (`skills/inception/SKILL.md`), run on the coordinator seat with the human present, for fresh and retrofit repositories alike.

1. Install with `scripts/adopt.sh <target> [--runtimes <names>] [--enable <optional-skill>]` from the template checkout. The payload is an explicit allowlist — what is not named does not ship — and the script also registers skills and subagent profiles for the selected runtimes, installs the shipped CI workflow, hooks, and the new-plan commit guard, creates the gitignored `artifacts/` directory, seeds `plans/` with its README and FRESH empty ledgers (never the template repository's own), and records the template SHA. If adoption must ever be done by hand, reproduce exactly what the script does — its source is the specification — and merge shipped dotfiles such as `.gitattributes` into existing ones instead of overwriting them.
2. Replace `docs/project-rules.md` with verified project facts and commands, including project-specific delegation facts (see `docs/orchestration.md`), the team's additions to the decisions reserved for humans, the budget facts where agents can spend real money, the durable evidence root where run evidence must survive, the refactor acceptance gate with its cadence backstop, and, where the project chases measured goals, the improvement evaluation with its metrics and noise floor.
3. Enable optional skills only where they apply: move `optional-skills/debug-java` into `skills/` only for repositories with a JVM runtime, and configure its launcher path in the skill reference. Leave the rest of `optional-skills/` out.
4. Verify skill registration for the runtimes in use (`scripts/adopt.sh` performs it; this step is the check). The registration contract is DECLARED, never remembered: `bin/metasystem runtime registration <name>` lists every artifact a runtime installs — operation, source, destination, and drift policy — and `bin/metasystem runtime dirs <name>` gives the directory view. The ONE generic repair procedure for a drifted installation: for each row of `runtime registration <name>`, recreate the destination from the source per its operation (`tree` links or copies each `skills/<skill>` child; `copy-file` copies; `json-strip-key` re-derives via `bin/metasystem json strip`; `skill-profiles` projects per staged skill), then rerun `scripts/validate-metasystem.sh`. Without registration, no runtime auto-triggers skills and routing depends entirely on the model reading `wow.md`. Skip runtimes the team does not use.
5. Keep only design principles relevant to the system's risk. Do not weaken ownership, invariant, failure, or proof rules without documenting why.
6. Add project-specific skills only for specialist workflows used repeatedly. Initialize and validate each skill; keep detailed references one hop from `SKILL.md`.
7. Put task state in `plans/`, generated evidence in the gitignored `artifacts/` directory at the repository root (create it and add it to the project's `.gitignore`), and durable decisions in code and docs. Do not keep them in root prompt history. Programs start with `goal open`: before a multi-session effort's first commit, declare its goal through the verb — the engine owns where the ledger lives (the legacy single file or the converted `plans/goals/` tree) — and adoption seeds the ledger with a Goal-free declaration (never an example goal), which the turn verdict expires when the plans world moves.
8. Run `scripts/validate-metasystem.sh`, then a focused project build and test. Verify the hard checks are wired (`scripts/adopt.sh` installs the CI workflow and the Claude Code hooks; repair from `scripts/enforcement/` if drifted: the workflow installs ripgrep, which the audit requires, and the hooks merge into `.claude/settings.json`) so a due retro surfaces as a visible notice at the end of every Claude Code turn (the Stop hook emits a systemMessage; when the cadence is not due it stays silent). Hard checks must not depend on anyone remembering to run them.
9. Replace the `<template sha>` placeholder in `docs/project-rules.md` with the template commit SHA adopted from; future template migrations diff against it (see the README's updating section). `scripts/adopt.sh` fills it automatically.

## What Not to Copy Forward

- `meta/` and its rationale documents.
- Old benchmark thresholds, frontier SHAs, provider or model names, fixture terminology, or incident-specific stop budgets.
- Language or framework rules irrelevant to the new repository.
- Multiple compatibility instruction files containing the same policy; compatibility files should point to `AGENTS.md`.
- Long tool manuals in root instructions; runtime subagent mechanics belong to the runtime's own manual.
- Rules already enforced by the compiler, tests, schemas, permissions, or CI, unless the routing instruction is still necessary.

## Receipts and Retro

The metasystem is judged by shipped outcomes rather than by whether its documents were followed. Evidence is collected continuously and consumed periodically.

Every task that changes the repository appends one receipt line at completion via `scripts/receipt.sh add`: task type, outcome, skills triggered, what verification showed, corrections received, stop-loss events, and a ceremony note when a rule cost more than it earned. Receipts are committed with the work. Review-only tasks propose the receipt line in the report instead of writing it. Purely conversational tasks get no receipt.

When `scripts/receipt.sh check` reports a retro due (default: more than 25 receipts or 30 days since the last retro), run the retro per `skills/retro/SKILL.md`. The previous retro's changes are reviewed against evidence first and kept, amended, or reverted. Period numbers are recorded so retros stay comparable. New proposals pass the change gate below before a human accepts or vetoes them. Every adopted change carries a testable expected effect in `plans/instruction-ledger.md` so the next retro can judge it.

Do not add a new global rule after a single ambiguous miss; receipts exist precisely so rules rest on repeated evidence. In the first week, run the first retro after a handful of tasks instead of waiting for the cadence. Early routing errors are the cheapest to fix.

### The Change Gate

Every instruction change (retro proposal, correction capture, or any other addition) answers these before it lands:

- What observed failure does this prevent?
- Is the model unable to infer it from the task, code, or tool feedback?
- Does an existing owner already cover it?
- Must it load always, or only for one task type or phase?
- Can a schema, permission, test, linter, or script enforce it better?
- What evidence will justify keeping it after models or products change?

If the answers are weak, do not add the instruction.
