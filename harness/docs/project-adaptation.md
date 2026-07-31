# Project Adaptation

These steps assume a repository without existing agent instruction assets. If the repository already has agent contracts, skills, prompts, or rule files, follow `docs/harness-reconciliation.md`, which wraps these steps with inventory, classification, and cutover.

1. Copy the harness contents into the new repository root, excluding `meta/` (template maintenance) and the template's own ledger history (`plans/receipts.log` if present; every project starts its own). Merge shipped dotfiles such as `.gitattributes` into existing ones instead of overwriting them.
2. Replace `docs/project-rules.md` with verified project facts and commands, including project-specific delegation facts (see `docs/orchestration.md`), the team's additions to the decisions reserved for humans, the budget facts where agents can spend real money, the durable evidence root where run evidence must survive, the refactor acceptance gate with its cadence backstop, and, where the project chases measured goals, the improvement evaluation with its metrics and noise floor.
3. Enable optional skills only where they apply: move `optional-skills/debug-java` into `skills/` only for repositories with a JVM runtime, and configure its launcher path in the skill reference. Leave the rest of `optional-skills/` out.
4. Register the skills with the runtimes in use. Without this, no runtime auto-triggers them and routing depends entirely on the model reading `wow.md`. For Claude Code, copy or symlink each `skills/<name>/` into `.claude/skills/<name>/` so the SKILL.md description drives triggering; consult other runtimes' manuals for their skill locations. Then register the subagent profiles: copy each `skills/<name>/agents/claude-profile.md` to `.claude/agents/<name>.md`, each `skills/<name>/agents/devin/AGENT.md` to `.devin/agents/<name>/AGENT.md`, and each `agents/openai.yaml` where an OpenAI-style skill launcher consumes it. Skip runtimes the team does not use.
5. Keep only design principles relevant to the system's risk. Do not weaken ownership, invariant, failure, or proof rules without documenting why.
6. Add project-specific skills only for specialist workflows used repeatedly. Initialize and validate each skill; keep detailed references one hop from `SKILL.md`.
7. Put task state in `plans/`, generated evidence in the gitignored `artifacts/` directory at the repository root (create it and add it to the project's `.gitignore`), and durable decisions in code and docs. Do not keep them in root prompt history.
8. Run `scripts/validate-harness.sh`, then a focused project build and test. Wire the hard checks into machinery, starting from the shipped examples: copy `scripts/enforcement/github-actions-harness.yml` to `.github/workflows/harness.yml` (it installs ripgrep, which the audit requires) and merge `scripts/enforcement/claude-code-hooks.json` into `.claude/settings.json` so a due retro surfaces as a visible notice at the end of every Claude Code turn (the Stop hook emits a systemMessage; when the cadence is not due it stays silent). Hard checks must not depend on anyone remembering to run them.
9. Replace the `<template sha>` placeholder in `docs/project-rules.md` with the template commit SHA adopted from; future template migrations diff against it (see the README's updating section). `scripts/adopt.sh` fills it automatically.

## What Not to Copy Forward

- `meta/` and its rationale documents.
- Old benchmark thresholds, frontier SHAs, provider or model names, fixture terminology, or incident-specific stop budgets.
- Language or framework rules irrelevant to the new repository.
- Multiple compatibility instruction files containing the same policy; compatibility files should point to `AGENTS.md`.
- Long tool manuals in root instructions; runtime subagent mechanics belong to the runtime's own manual.
- Rules already enforced by the compiler, tests, schemas, permissions, or CI, unless the routing instruction is still necessary.

## Receipts and Retro

The harness is judged by shipped outcomes rather than by whether its documents were followed. Evidence is collected continuously and consumed periodically.

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
