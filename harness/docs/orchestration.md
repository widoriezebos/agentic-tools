# Orchestration

This file owns delegation judgment. Each runtime's own manual owns tool mechanics. Do not copy runtime instructions here.

## When to Delegate

- Broad exploration across many files, directories, or naming conventions: use a read-only explorer subagent and keep only its conclusion in the main context.
- Independent, separately verifiable subtasks: run them as parallel workers when neither needs the other's intermediate output.
- Long or expensive runs whose output is only needed later: run them in the background and continue decision work.

## When Not to Delegate

- A single lookup where the file, symbol, or command is already known.
- Sequentially dependent edits, or work that needs the judgment built up in the main conversation.
- When coordination plus billing outweighs the saving. Subagents run their own context windows and inference calls and bill separately on every current runtime.

## Delegation Contract

Every delegation states the goal, the inputs it may rely on, the expected return shape (facts, paths, diff, verdict), and a budget. Treat a subagent's report as unverified claims until checked against that contract. Never merge unreviewed subagent edits into work you certify.

## Peer Agents

Top-level agents sharing one repository (for example Claude Code and Codex CLI, or two concurrent sessions) are peers. Nothing coordinates them unless the team does. Rules:

- One branch or worktree per agent per stream. Never work directly on a branch another agent has in flight.
- Claim a stream through its handoff note in `plans/` (shape in `plans/README.md`). A claim exists only where peers can see it: commit and push the note to the shared default branch before starting. A note that lives only on your feature branch claims nothing. Do not advance a stream whose note another agent owns; hand over by updating the note's owner.
- Receipts written on unmerged branches are invisible to the shared retro cadence. Run retros from the integration branch, after merging.
- Integrate through the normal review flow. Merge conflicts between peer agents are a human decision. Neither agent resolves them by force.
- A peer's output is unverified claims, exactly like a subagent's. Verify against the contract before building on it or certifying it.
- These rules are conventions, and git offers no atomic cross-clone claim. Simultaneous claims surface as merge conflicts for the human. Keep streams disjoint instead of trusting a claim to lock anything.

## Runtime Mechanics (pointers only)

| Runtime | Spawn mechanism | Custom profile location |
| --- | --- | --- |
| Claude Code | `Task`/`Agent` tool; built-in read-only explorer | `.claude/agents/<name>.md` |
| Devin CLI | `run_subagent`; built-in `subagent_explore` (read-only) and `subagent_general` | `.devin/agents/<name>/AGENT.md` |
| Other | Consult the runtime manual | Adapt the templates below |

Per-runtime profile templates for this harness's skills live in `skills/<name>/agents/`: `claude-profile.md`, `devin/AGENT.md`, and `openai.yaml` with invocation metadata for OpenAI-style skill launchers. Copy them into the runtime's profile location during adaptation. Project-specific delegation facts (a slow suite worth backgrounding, a directory worth pre-exploring) belong in `docs/project-rules.md` rather than here.
