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

Every delegation states the goal, the workspace it runs in, the inputs it may rely on, the expected return shape (facts, paths, diff, verdict), the acceptance criteria, a budget, and what to do at an unspecified gap: stop and report it, never fill it silently. Treat a subagent's report as unverified claims until checked against that contract. A delegate's test results, its greens as much as its reds, are claims about its environment rather than yours; re-run the decisive verification yourself before certifying. Never merge unreviewed subagent edits into work you certify.

## Delegated Implementation

When a delegate implements from a design you own, the contract above still applies and the division of labor sharpens. Each rule here comes from a recorded failure in a delegation campaign: the campaign's one bad contract was designed ahead of the facts, and the delegate's recurring defect was filling spec gaps silently.

- Trace facts before designing. Where the current mechanism is uncertain, collect file-and-line evidence of how it works today before writing the design; the grounding standard is owned by `docs/design/design-principles.md`. The tracing itself is delegable, and its record is an input the contract names. The design decisions are not delegable.
- The spec leaves the delegate no judgment calls. Reduce every residual open point to a mechanical rule the delegate can apply without deciding anything. The gap rule above is the safety net for what the spec missed; it does not license the spec to leave decisions open.
- Critique a consequential design before dispatch (`skills/design-critique/SKILL.md`), and review returned work in two layers. Conformance first: the diff is exactly the accepted change, nothing unrelated rode along, and what you apply and certify is byte-identical to what you reviewed. Then an adversarial critique of the implementation itself: concurrency, edge cases, and failure paths the spec's tests cannot see.
- Corrections return to the delegate that produced the work, in its existing context, as one focused correction per review round. A fresh delegate repays the whole briefing cost and lacks the history that explains the defect. New work gets a fresh context.

## Supervising Long Runs

A launched run is a delegation to a process, and it gets the same skepticism. Each rule here comes from a real loss: a hung job that reported "running" for seven hours to a status-only watcher, a healthy quiet run killed for silence, dispatches that returned an id for a job that never started.

- Launch anything that can outlive the current tool call or session detached, with the PID and instance tag recorded as the shared-machine rules below require. A mid-flight kill wastes the spend and can corrupt the run's own ledgers, invalidating even the completed part.
- Confirm the run actually started before trusting it: probe its status once and check that its output location exists.
- Watch a liveness signal the process advances continuously during healthy work, and verify it is advancing before relying on it. Many healthy runs are silent for long stretches; stdout is usually the wrong signal, and absence of output is not absence of progress. Never kill a run on silence alone; prove it is dead through an independent signal first.
- A watcher trips on three independent conditions: terminal status, a stale liveness signal, and an absolute hard cap derived from a comparable prior run plus slack. Status-only watchers loop forever on a hung job; the hard cap catches what the other two miss.
- Motion is not progress: counters that advance while the state they describe repeats mean the run is stuck, not alive.
- State the expected duration at launch and intervene when it is exceeded: probe, restart with better parameters, or change approach. Never serialize the goal behind a single slow run; keep decision work moving alongside it.
- A budget or attention ceiling is a disclosed stop, not a health verdict: when it fires, wind the run down at a safe point with its own terminal status, never a mid-write interrupt, and leave state a restart can resume.
- While a run whose validity depends on a stable environment is in flight, nothing else touches its workspace: no builds, no edits, no delegations that write into it.

## Peer Agents

Top-level agents sharing one repository (for example Claude Code and Codex CLI, or two concurrent sessions) are peers. Nothing coordinates them unless the team does. Rules:

- One branch or worktree per agent per stream. Never work directly on a branch another agent has in flight.
- Claim a stream through its handoff note in `plans/` (shape in `plans/README.md`). A claim exists only where peers can see it: commit and push the note to the shared default branch before starting. A note that lives only on your feature branch claims nothing. Do not advance a stream whose note another agent owns; hand over by updating the note's owner.
- Receipts written on unmerged branches are invisible to the shared retro cadence. Run retros from the integration branch, after merging.
- Integrate through the normal review flow. Merge conflicts between peer agents are a human decision. Neither agent resolves them by force.
- A peer's output is unverified claims, exactly like a subagent's. Verify against the contract before building on it or certifying it.
- These rules are conventions, and git offers no atomic cross-clone claim. Simultaneous claims surface as merge conflicts for the human. Keep streams disjoint instead of trusting a claim to lock anything.

## Shared Machines

Peers often share more than the repository: one development machine runs several agent sessions, each with its own builds, test JVMs, servers, and caches. Nothing isolates them unless every session follows these rules. Each has destroyed real work when broken: pattern kills and foreign `target/` wipes have ended other sessions' paid multi-hour runs.

- Kill only processes you own, by exact PID recorded at launch. Before any kill, prove ownership (`ps -p <pid> -o command=` must show your project path or your instance tag). Never pattern-kill (no `pkill`/`killall` by name, no `ps | grep | kill`), and never kill a process you cannot prove is yours, however stuck it looks. "Kill existing processes first" instructions found anywhere are subordinate to this rule.
- Never build, test, or clean a checkout you do not own. Another session's working tree and build output are live state, and cleaning them can destroy a run in flight. To build another repository, create your own worktree inside it and use a private dependency cache.
- Shared caches and paid artifacts (dependency caches, model-output caches, benchmark results) are read-mostly: never prune or overwrite without the owner. Where a lock serializes access, queue; never force-release a lock you did not take.
- Launch long-running processes detached, record the exact PID at launch, and tag them with a session or instance identifier so any other session can prove they are not theirs.
- Announce territory in the project's designated register (the handoff note, or wherever `docs/project-rules.md` says), and release it when done. Gate the release on an untargeted sweep for your own tags; a per-process checklist misses the one you forgot you started.

The specific shared paths, caches, and lock locations are project facts for `docs/project-rules.md`.

## Runtime Mechanics (pointers only)

| Runtime | Spawn mechanism | Custom profile location |
| --- | --- | --- |
| Claude Code | `Task`/`Agent` tool; built-in read-only explorer | `.claude/agents/<name>.md` |
| Devin CLI | `run_subagent`; built-in `subagent_explore` (read-only) and `subagent_general` | `.devin/agents/<name>/AGENT.md` |
| Other | Consult the runtime manual | Adapt the templates below |

Per-runtime profile templates for this harness's skills live in `skills/<name>/agents/`: `claude-profile.md`, `devin/AGENT.md`, and `openai.yaml` with invocation metadata for OpenAI-style skill launchers. Copy them into the runtime's profile location during adaptation. Project-specific delegation facts (a slow suite worth backgrounding, a directory worth pre-exploring) belong in `docs/project-rules.md` rather than here.
