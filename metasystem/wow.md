# Ways of Working Index

The single routing layer. `AGENTS.md` owns always-on behavior; this table owns what to load and when. Never duplicate routing elsewhere.

| Need | Canonical owner | Load when |
| --- | --- | --- |
| Always-on behavior | `AGENTS.md` | Every task |
| Project commands and constraints | `docs/project-rules.md` | Before editing or validating |
| Dispatching, supervising, or coordinating agents | `docs/orchestration.md` | Work splits into parts, a rostered role is dispatched, a run needs supervision, or peers share the repo |
| Working with the human team | `docs/collaboration.md` | Answering, reporting, committing, preparing review, or taking a correction |
| The human teammate's manual | `docs/working-with-agents.md` | The user asks how to work with agents here |
| Session handoff and task-local state | `plans/README.md` | Starting or ending a multi-session stream |
| The goal thread (`goal open`/`goal next`) | `plans/README.md` | Starting a program, ending a turn, or asking what this repository is FOR now; the ledger is standing, exempt from delete-when-shipped |
| Code and design standards | `docs/design/design-principles.md` | Any non-trivial code, design, refactor, or consequential change |
| Completion check and design proof | `docs/design/design-obligation-gate.md` | Finishing any change; full matrix on its listed triggers |
| End-to-end verification | `skills/verify/SKILL.md` | A change claims to work and can be run |
| Adversarial design critique | `skills/design-critique/SKILL.md` | A design is ready to attack, or a critique loop needs a stop decision |
| Two-layer implementation critique | `skills/code-critique/SKILL.md` | Implemented work needs conformance review against its brief and diff, then defect review |
| Behavior-preserving refactor | `skills/refactor/SKILL.md` | Restructuring or cleanup whose contract is unchanged behavior |
| Benchmark-driven improvement | `skills/improve/SKILL.md` | Chasing a measured goal against a runnable evaluation |
| Investigation stop-loss | `skills/take-a-step-back/SKILL.md` | Work is stuck, repetitive, costly, or its premise is uncertain |
| Working modes in plain English | `docs/working-modes.md` | Learning the system, or unsure of a mode |
| The Go engine's architecture | `docs/architecture.md` | Locating a decision, or judging where new logic belongs |
| Worked examples | `docs/examples/` | A template is unclear |
| Adoption and the change gate | `docs/project-adaptation.md` | Starting a repository, or judging an instruction change |
| Metasystem retro | `skills/retro/SKILL.md` | `scripts/receipt.sh check` reports one due, or the human asks |
| Reconciling an existing repository | `docs/metasystem-reconciliation.md` | Installing or upgrading where instructions or skills already exist |
| App inception | `skills/inception/SKILL.md` | Birthing an app: covenant, doctrine, thread |
| Opt-in specialist skills | `optional-skills/` | Enabled during adaptation |
| Metasystem maintenance and rationale | `development/` at the repository toplevel (template only) | Changing the metasystem template itself; absent in adopted projects |

Task-local plans, ledgers, receipts, benchmark artifacts, and incident notes are evidence, not policy. A stable lesson becomes policy only by deliberate promotion into one canonical owner above. Worked examples are illustrations, not policy.
