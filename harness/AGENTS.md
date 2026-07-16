# Repository Agent Contract

Use `wow.md` as the only routing index. Read just the guidance and skills relevant to the current task.

## Work Contract

- Inspect local instructions, code, tests, and current state before firm conclusions.
- Match the requested action: explain/review without mutation; implement when asked to change; verify in proportion to risk.
- Reproduce a reported defect before fixing it; when practical, capture the reproduction as a failing test and prove the fix flips it.
- Preserve user-owned changes. Avoid destructive Git operations and unrelated edits.
- State assumptions, blockers, verification performed, unverified areas, and residual risk.
- Resolve ambiguity by ladder: repository evidence first; then the smallest reversible assumption, stated; ask before choices that affect contracts, scope, data, or user-visible behavior. Turn the chosen interpretation into an observable acceptance criterion.
- Escalate before acting on human-reserved decisions: irreversible or outward-facing actions, API or schema contracts, new dependencies, and scope changes discovered mid-task (`docs/project-rules.md` lists the project's reserved set).
- Treat a user correction of a convention, preference, or fact as an instruction update: apply it now, and persist it to its owning document only when the task authorizes edits — otherwise propose the capture in your report (`docs/collaboration.md`).
- Instructions change only through correction capture or a retro, each through the change gate — never mid-task from a single incident.
- Prefer the smallest robust solution that satisfies a current user or production contract.
- Give each important behavior one owner; keep boundaries honest; make state, failure, and observability explicit.
- Use focused tests first. Use expensive, model-backed, debugger, or full-suite validation only when it can answer a named question.
- When the runtime provides subagents, delegate independent exploration and verifiable subtasks; keep the main context for decisions (`docs/orchestration.md`).
- Keep machine-verifiable requirements in schemas, tests, linters, permissions, or scripts—not repeated prose.
- Keep project-specific commands and policies in `docs/project-rules.md`.

## Completion

Before calling any change complete, run the default completion check in `docs/design/design-obligation-gate.md` and, when the change has a runnable surface, verify it end-to-end per `skills/verify/SKILL.md`. Work is complete only when the requested outcome exists, focused verification passes, and any critical/high design obligations are done. Report what changed, what proved it, what was not run, and remaining risk — structured for human review per `docs/collaboration.md`. On unfinished multi-session work, update the stream's handoff note in `plans/` before ending the session. If the task changed the repository or triggered a skill, append a receipt with `scripts/receipt.sh add`.
