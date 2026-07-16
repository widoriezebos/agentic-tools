# Ideal Agent Harness

A portable, low-context engineering harness for agentic runtimes (Claude Code, Devin CLI, and others), distilled from production engineering, agent-evaluation, and runtime-debugging experience. It is judged by shipped task outcomes, not by document tidiness.

The harness is deliberately layered:

1. `AGENTS.md` is the small, always-loaded operating contract.
2. `wow.md` is the single routing index to canonical guidance and triggered skills.
3. `docs/` holds depth read only for the relevant phase, including delegation and peer-agent judgment (`docs/orchestration.md`), collaboration with the human team (`docs/collaboration.md`), and worked examples (`docs/examples/`).
4. `skills/` holds core triggered workflows (`verify`, `take-a-step-back`, `refactor`), each with per-runtime subagent profile templates under `agents/`.
5. `optional-skills/` holds specialists (e.g. `debug-java`) enabled per project during adaptation.
6. `scripts/` turns binary requirements into checks.
7. `plans/` holds task-local state and proof, not permanent global policy.
8. `meta/` holds harness maintenance and rationale; it is never copied into adopting projects.

Start with [project-adaptation.md](docs/project-adaptation.md). Keep the root contract short; add project facts by replacing placeholders, not by appending incident history.

Run:

```bash
scripts/validate-harness.sh
scripts/audit-harness.sh .
```

The maintenance change gate is in [meta/harness-architecture.md](meta/harness-architecture.md); source analysis and keep/remove decisions are in [meta/source-analysis.md](meta/source-analysis.md).
