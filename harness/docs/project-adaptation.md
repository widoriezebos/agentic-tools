# Project Adaptation

1. Copy the harness contents into the new repository root, excluding `meta/` — it documents harness maintenance and must not ship with adopting projects.
2. Replace `docs/project-rules.md` with verified project facts and commands, including any project-specific delegation facts (see `docs/orchestration.md`).
3. Enable optional skills only where they apply: move `optional-skills/debug-java` into `skills/` only for repositories with a JVM runtime, and configure its launcher path in the skill reference. Leave the rest of `optional-skills/` out.
4. Register subagent profiles for the runtimes in use: copy each `skills/<name>/agents/claude.md` to `.claude/agents/<name>.md` and each `skills/<name>/agents/devin/AGENT.md` to `.devin/agents/<name>/AGENT.md`. Skip runtimes the team does not use.
5. Keep only design principles relevant to the system's risk. Do not weaken ownership, invariant, failure, or proof rules without documenting why.
6. Add project-specific skills only for specialist workflows used repeatedly. Initialize and validate each skill; keep detailed references one hop from `SKILL.md`.
7. Put task state in `plans/`, generated evidence in a gitignored artifact directory, and durable decisions in code/docs—not in root prompt history.
8. Run `scripts/validate-harness.sh`, then a focused project build/test.

## What Not to Copy Forward

- `meta/` and its rationale documents.
- Old benchmark thresholds, frontier SHAs, provider/model names, fixture terminology, or incident-specific stop budgets.
- Language/framework rules irrelevant to the new repository.
- Multiple compatibility instruction files containing the same policy; compatibility files should point to `AGENTS.md`.
- Long tool manuals in root instructions; runtime subagent mechanics belong to the runtime's own manual.
- Rules already enforced by the compiler, tests, schemas, permissions, or CI unless the routing instruction is still necessary.

## First-Week Receipt

Record the model/product, task types exercised, skills triggered, tools used, checks run, failures, fallbacks, and instruction changes. Add per-task outcome evidence:

- Did the requested outcome ship without rework? If reworked, what did the harness fail to catch?
- Did end-to-end verification (`skills/verify`) catch anything green checks missed?
- Did stop-loss or delegation demonstrably save time or cost, or did it add ceremony?

Tune routing from that evidence; judge the harness by shipped outcomes, not by whether its documents were followed. Do not add a new global rule after a single ambiguous miss.
