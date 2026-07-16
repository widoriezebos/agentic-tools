# Ways of Working Index

This is the single routing layer. `AGENTS.md` owns always-on behavior; this table owns what to load and when. Do not duplicate routing elsewhere.

| Need | Canonical owner | Load when |
| --- | --- | --- |
| Always-on behavior | `AGENTS.md` | Every task |
| Project commands and constraints | `docs/project-rules.md` | Before editing or validation |
| Delegation, parallel work, peer agents | `docs/orchestration.md` | Work splits into independent parts, needs broad exploration, runs long, or shares the repo with another agent |
| Working with the human team | `docs/collaboration.md` | Committing, preparing changes for review, or receiving a correction |
| Session handoff and task-local state | `plans/README.md` | Starting or ending a session on a multi-session stream |
| Architecture/design judgment | `docs/design/design-principles.md` | Design, refactor, or consequential change |
| Completion check and design proof | `docs/design/design-obligation-gate.md` | Finishing any change; full matrix only on its listed triggers |
| End-to-end verification | `skills/verify/SKILL.md` | A change claims to work and has a runnable surface |
| Behavior-preserving refactor | `skills/refactor/SKILL.md` | Restructuring, readability, or cleanup work whose contract is unchanged behavior |
| Investigation stop-loss | `skills/take-a-step-back/SKILL.md` | Work is stuck, repetitive, expensive, or premise is uncertain |
| Worked examples | `docs/examples/` | A template above is unclear in practice |
| Adoption | `docs/project-adaptation.md` | Starting a new repository |
| Opt-in specialist skills | `optional-skills/` | Only when enabled during adaptation (e.g. `debug-java` for JVM repositories) |
| Harness maintenance and rationale | `meta/` | Changing the harness itself; never copied into adopting projects |

Task-local plans, ledgers, receipts, benchmark artifacts, and incident notes are evidence. They must not become global policy unless a stable lesson is deliberately promoted into one canonical owner above. Worked examples illustrate templates; they are not policy either.
