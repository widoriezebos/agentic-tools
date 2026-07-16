# Project Adaptation

1. Copy the harness contents into the new repository root, excluding `meta/` — it documents harness maintenance and must not ship with adopting projects.
2. Replace `docs/project-rules.md` with verified project facts and commands, including any project-specific delegation facts (see `docs/orchestration.md`), the team's additions to the decisions reserved for humans, and the refactor acceptance gate with its cadence backstop.
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

## Receipts and Retro

The harness is judged by shipped outcomes, not by whether its documents were followed. The evidence is collected continuously and consumed periodically:

Every task that changes the repository or triggers a skill appends one receipt line at completion via `scripts/receipt.sh add` — task type, outcome, skills triggered, what verification showed, corrections received, stop-loss events, and a ceremony note when a rule cost more than it earned. Receipts are committed with the work. Purely conversational tasks get no receipt.

When `scripts/receipt.sh check` reports a retro due (default every 25 receipts or 30 days), run a harness retro:

1. Read the receipts since the last retro.
2. Look for patterns, never anecdotes: a skill that should have triggered but did not; verification repeatedly catching the same class of issue; corrections clustering around one convention; ceremony notes; rules and skills that never fired.
3. Propose instruction changes through the change gate, each routed to its one owning document — and prune with the same energy as adding. A rule whose receipts never mention it is a removal candidate.
4. Present the proposals for human veto, then record the retro with `scripts/receipt.sh retro` and a summary of what changed.

Do not add a new global rule after a single ambiguous miss; receipts exist precisely so rules rest on repeated evidence. In the first week, run the first retro after a handful of tasks instead of waiting for the cadence — early routing errors are the cheapest to fix.
