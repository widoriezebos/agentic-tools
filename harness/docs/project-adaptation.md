# Project Adaptation

These steps assume a repository without existing agent instruction assets. If the repository already has agent contracts, skills, prompts, or rule files, follow `docs/harness-reconciliation.md`, which wraps these steps with inventory, classification, and cutover.

1. Copy the harness contents into the new repository root, excluding `meta/` — it documents harness maintenance and must not ship with adopting projects.
2. Replace `docs/project-rules.md` with verified project facts and commands, including any project-specific delegation facts (see `docs/orchestration.md`), the team's additions to the decisions reserved for humans, the refactor acceptance gate with its cadence backstop, and — where the project chases measured goals — the improvement evaluation with its metrics and noise floor.
3. Enable optional skills only where they apply: move `optional-skills/debug-java` into `skills/` only for repositories with a JVM runtime, and configure its launcher path in the skill reference. Leave the rest of `optional-skills/` out.
4. Register subagent profiles for the runtimes in use: copy each `skills/<name>/agents/claude-profile.md` to `.claude/agents/<name>.md`, each `skills/<name>/agents/devin/AGENT.md` to `.devin/agents/<name>/AGENT.md`, and each `agents/openai.yaml` (invocation metadata) where an OpenAI-style skill launcher consumes it. Skip runtimes the team does not use.
5. Keep only design principles relevant to the system's risk. Do not weaken ownership, invariant, failure, or proof rules without documenting why.
6. Add project-specific skills only for specialist workflows used repeatedly. Initialize and validate each skill; keep detailed references one hop from `SKILL.md`.
7. Put task state in `plans/`, generated evidence in a gitignored artifact directory, and durable decisions in code/docs—not in root prompt history.
8. Run `scripts/validate-harness.sh`, then a focused project build/test. Wire `scripts/audit-harness.sh .` (and the gate scripts the project uses) into CI or a pre-merge hook — hard checks must not depend on anyone remembering to run them.
9. Record the template commit SHA adopted from as a line in `docs/project-rules.md`; future template migrations diff against it (see the README's updating section).

## What Not to Copy Forward

- `meta/` and its rationale documents.
- Old benchmark thresholds, frontier SHAs, provider/model names, fixture terminology, or incident-specific stop budgets.
- Language/framework rules irrelevant to the new repository.
- Multiple compatibility instruction files containing the same policy; compatibility files should point to `AGENTS.md`.
- Long tool manuals in root instructions; runtime subagent mechanics belong to the runtime's own manual.
- Rules already enforced by the compiler, tests, schemas, permissions, or CI unless the routing instruction is still necessary.

## Receipts and Retro

The harness is judged by shipped outcomes, not by whether its documents were followed. The evidence is collected continuously and consumed periodically:

Every task that changes the repository appends one receipt line at completion via `scripts/receipt.sh add` — task type, outcome, skills triggered, what verification showed, corrections received, stop-loss events, and a ceremony note when a rule cost more than it earned. Receipts are committed with the work. Review-only tasks propose the receipt line in the report instead of writing it; purely conversational tasks get no receipt.

When `scripts/receipt.sh check` reports a retro due (default: more than 25 receipts or 30 days since the last retro), run the retro per `skills/retro/SKILL.md`: the previous retro's changes are reviewed against evidence first and kept, amended, or reverted; period numbers are recorded for comparability; and new proposals pass the change gate below before a human accepts or vetoes them. Every adopted change carries a falsifiable expected effect in `plans/instruction-ledger.md` so the next retro can judge it.

Do not add a new global rule after a single ambiguous miss; receipts exist precisely so rules rest on repeated evidence. In the first week, run the first retro after a handful of tasks instead of waiting for the cadence — early routing errors are the cheapest to fix.

### The Change Gate

Every instruction change — retro proposal, correction capture, or any other addition — answers these before it lands:

- What observed failure does this prevent?
- Is the model unable to infer it from the task, code, or tool feedback?
- Does an existing owner already cover it?
- Must it load always, or only for one task type or phase?
- Can a schema, permission, test, linter, or script enforce it better?
- What evidence will justify keeping it after models or products change?

If the answers are weak, do not add the instruction.
