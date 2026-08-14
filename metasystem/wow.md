# Ways of Working Index

This is the single routing layer. `AGENTS.md` owns always-on behavior; this table owns what to load and when. Do not duplicate routing elsewhere.

| Need | Canonical owner | Load when |
| --- | --- | --- |
| Always-on behavior | `AGENTS.md` | Every task |
| Project commands and constraints | `docs/project-rules.md` | Before editing or validation |
| Dispatching, supervising, or coordinating agents | `docs/orchestration.md` | Work splits into independent parts, a rostered role is dispatched, a run needs supervision, or peers share the repo |
| Working with the human team | `docs/collaboration.md` | Answering a question, reporting, committing, preparing changes for review, or receiving a correction |
| The human teammate's manual | `docs/working-with-agents.md` | The user asks how to work with agents under this metasystem |
| Session handoff and task-local state | `plans/README.md` | Starting or ending a session on a multi-session stream |
| Code and design standards | `docs/design/design-principles.md` | Writing or changing code beyond trivial edits; any design, refactor, or consequential change |
| Completion check and design proof | `docs/design/design-obligation-gate.md` | Finishing any change; full matrix only on its listed triggers |
| End-to-end verification | `skills/verify/SKILL.md` | A change claims to work and has a runnable surface |
| Adversarial design critique | `skills/design-critique/SKILL.md` | A design is ready to attack before implementation, or a critique loop needs a stop decision |
| Two-layer implementation critique | `skills/code-critique/SKILL.md` | Implemented work needs conformance review against its brief and computed diff, then adversarial defect review |
| Behavior-preserving refactor | `skills/refactor/SKILL.md` | Restructuring, readability, or cleanup work whose contract is unchanged behavior |
| Benchmark-driven improvement | `skills/improve/SKILL.md` | Chasing a measured improvement goal against a runnable evaluation |
| Investigation stop-loss | `skills/take-a-step-back/SKILL.md` | Work is stuck, repetitive, expensive, or premise is uncertain |
| Working modes, explained in plain English | `docs/working-modes.md` | Learning the system, or unsure which mode a task is in |
| Worked examples | `docs/examples/` | A template above is unclear in practice |
| Adoption and the change gate | `docs/project-adaptation.md` | Starting a new repository, or judging any instruction change |
| Metasystem retro | `skills/retro/SKILL.md` | `scripts/receipt.sh check` reports a retro due, or the human asks for one |
| Reconciling an existing repository | `docs/metasystem-reconciliation.md` | Installing or upgrading the metasystem where instructions, skills, or prompts already exist |
| Opt-in specialist skills | `optional-skills/` | Only when enabled during adaptation (e.g. `debug-java` for JVM repositories) |
| Metasystem maintenance and rationale | `development/` at the repository toplevel, one level above this directory (template repository only) | Changing the metasystem template itself; absent in adopting projects |

Task-local plans, ledgers, receipts, benchmark artifacts, and incident notes are evidence. They must not become global policy unless a stable lesson is deliberately promoted into one canonical owner above. Worked examples illustrate templates; they are not policy either.
