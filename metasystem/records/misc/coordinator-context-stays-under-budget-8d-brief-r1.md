# Design brief: coordinator-context-stays-under-budget, amendment 8d

Template: scripts/agents/templates/design-brief.md (the repository's design-brief template), filled from the seat's prompt of 2026-09-15 by the design delegate itself, as the prompt instructed. Paths are relative to the metasystem module of /Users/wido/LocalStorage/GitHub/agentic-tools-m1e unless absolute.

## Revision

Revision: first draft of amendment 8d of plans/coordinator-context-stays-under-budget-design.md (revision 3 plus 8c.15), designed against plans/seats-spend-tokens-in-bounded-sessions-design.md revision 3 (the program page, landed 495ddd0f2), which supersedes the parent page's section 5 on the launcher.

Reason: the goal's next step (revision 88) names amendment 8d next: U3a the trigger with the 250K ceiling key (R-114-m1e item 3), U3e the in-turn hook (R-114-m1e item 5), U3b the open-work record with in-flight delegates (R-115-m1e item 2), U3c the automatic successor within a stated number of seconds with SSTB-301, 303, 305, 310 and the console status (R-115-m1e item 2). m1c's evidence F2: `metasystem context handoff` started no continuation three times on 2026-09-15 because handoff.go holds the intent while the predecessor pid lives and an interactive seat cannot exit itself; Wido had to /clear and paste a continuation prompt by hand. Kind: design. This round writes the brief and the design and nothing else; the Codex critique and the builds are later rounds the seat runs. A NOT LAND or critique verdict is never the designer's to override.

## Context pack

Read this pack first. Open another file only to check one of the cited lines below; do not widen the read beyond that check.

Diagnosis or prior revision:

- The parent page, revision 3 (plans/coordinator-context-stays-under-budget-design.md, 940 lines): section 5 (handoff by observed predecessor death, Wido's decision 2 of 2026-09-13), section 7 (the proof week), 8c.15 (cancellation authority C1a, C1b, C2, landed), 8c.15.7 (where an 8c.7 expiry plugs in: `handoffExpiryRule`, handoff.go:21, :50-52, :89-91).
- The program page, revision 3 (plans/seats-spend-tokens-in-bounded-sessions-design.md, 205 lines): P1 trigger, P2 successor, P3 stop-gate policy, P7 unit map rows U3a, U3e, U3b, U3c, U1; open questions 1 to 4, answered by R-114-m1e.
- The goal record: `bin/metasystem goal show --id coordinator-context-stays-under-budget` (revision 88, approved by Wido at revision 71): DONE proof lines (95th percentile under 150K, no call over 200K, no compaction), the runtime-independence clause, the next step quoted above.
- Seat rules S1 to S8: docs/orchestration.md:355-371. Rulings R-114-m1e and R-115-m1e: memory/rulings.md:173-174.

Critique findings being answered:

1. none for a first draft; the program page's moved findings SSTB-301, 303, 305, 310 are the required decisions, and SSTB-308 (U1 before U3c's fresh registration) is the ordering constraint.

Cited code excerpts (read, not pasted; each is under twenty lines at the named place):

1. `internal/steward/handoff.go:89-125` — the three holds of `decideForHandoff`: expiry (:89-91), predecessor liveness (:93-101), census with `LiveSeatMains`, `CensusComplete`, `Untracked`, `Unprovable` (:102-114), `others > 0` (:117).
2. `internal/steward/revive.go:93-160, :270-285` — `CompleteRevival` (fence, verdict, consume, launch, stamp) and the integer `others` reduction before `decideForHandoff`.
3. `internal/steward/intervene.go:219-235` — `StampLaunch` sets `LaunchStamped` with no timestamp.
4. `internal/steward/stage.go:153-164` — the staged continuation brief text.
5. `cmd/metasystem/steward_verbs.go:493-509` — the launch seam: `<engine> delegate --revive <nonce>` under `Setsid`; `cmd/metasystem/delegate.go:236-241` maps it to `dispatch --steward-intent`; `scripts/agents/dispatch.sh:1467-1486` authorizes and launches with `use_worktree=1`.
6. `internal/steward/handoff_capture.go:374-427` (`waiterInFlight` refuses `HANDOFF_WAIT_IN_FLIGHT`), `:432-494` (`captureHandoffJobs`: running engine jobs of the held goal, pending ones refuse `HANDOFF_LAUNCH_IN_FLIGHT`), `:926-1029` (`Handoff`: capture, supersede, stage, prepare; it launches nothing), `:1175` (`LiveHandoffForSession`, live intents only).
7. `internal/steward/handoff_state.go:114-137` — `HandoffState` and `HandoffManifest`: no open-work rows, no delegates, no lessons note, no engine identity.
8. `internal/goal/turnverdict.go:200-205, :393-412, :991-1010, :1037` — `TurnVerdictOptions.HandoffRecorded`, the allowance `handoff recorded: <nonce>; end this session`, the idle-backlog `refusal N of 3`, `escalateIdleBacklog`; wired at `cmd/metasystem/goal.go:710-711`.
9. `internal/steward/context.go:23-24, :63, :310-321` — `ContextBoundTokens = 150000`, `ContextCeilingTokens = 200000` as Go constants, `ContextBudgetLine`, the verdict text. `internal/config/spend.go:15-17` shows the key naming style.
10. `internal/adapter/claude.go:122-131, :352-412` — job settings install only a SessionStart hook; `claude -p ... --settings <file>`. `internal/adapter/stopoutput.go:91-97` — `systemMessage` on every allow, `decision: block` otherwise. `../.claude/settings.json` installs SessionStart (`claude start`), Stop (`claude stop`), SessionEnd (`claude end`); no PreToolUse.
11. `scripts/agents/supervision-hook.sh:1451-1475, :1581-1586` — a delegate job skips the hook through `lease hook-delegate` (`internal/lease/hook_delegate.go:22-23`, custody-based); `:1534` the session from the payload; `:2368` an empty display is unreadable.
12. `internal/run/waiter.go:57-68` — `WaitSelector` (kind, targetId, goalId, event, after, verb, question, chain, poll). `internal/steward/runner.go:104` — the tick interval, 600 seconds.
13. `cmd/metasystem/main.go:431-438` — the context family: status, report, handoff, verify, prune; no resume (observed by m1e on the 13:16 binary and confirmed in source).

Example page:

records/misc/seats-spend-tokens-in-bounded-sessions-brief-r3.md for the brief's shape; plans/coordinator-context-stays-under-budget-design.md section 8c.15 (decisions, rule-to-witness tables, reader inventory, build shape) for the design's level of detail.

## Tool-call budget

Maximum delegate tool calls: 70

Stop when this number is reached. List anything the budget did not allow you to check.

## Page-size ceiling

Maximum page size: 450 lines (the seat's ceiling; about 9,000 words)

Cut a draft that exceeds this ceiling. If cutting would make the page incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique only through the context pack above. Never resume a delegate from an earlier revision.

## Rulings to honour, verbatim

- R-115-m1e item 2: "Design questions on coordinator-context-stays-under-budget: U3b's open-work record lists every in-flight Agent delegate (id, what it was asked, output path) or the quiet-point rule adds no delegate running; U3c's successor starts within a stated number of seconds, not a tick, dead time from the handoff record to the successor's first act is recorded, and the console pane shows the worker's status (goal next and the health line)."
- R-115-m1e item 6: "Program rule: no unit of the efficiency program lowers a proof floor, removes a witness or a gate, or narrows a DONE to save tokens, and each unit's read checks it."
- R-114-m1e item 2: "the automatic launcher: the steward continuation, a headless session the steward starts. No machinery may assume or rely on tmux." Item 3: "a context ceiling of 250K in configuration, with the trigger at the ceiling minus the margin." Item 5: "build U3e, a separate tool-use hook under about 100 ms that never blocks a landing or an in-flight wait, so the DONE's stated size holds by construction." Item 6: "Never stop work because of a limit: a cap or budget triggers a handoff or a delegate that continues the work, never a stop and never a wait on Wido."
- Wido 2026-09-14 (fix upstream, not at the check): every rule the design adds names the check or test that fails if the rule is removed, so the builder proves each rule by mutation.

## Page artifact and return shape

Write the page to: /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9b7e334d-919d-47e9-9555-a9f2ed557072/scratchpad/coordinator-context-8d-design.md

Return only these two lines:

```text
/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9b7e334d-919d-47e9-9555-a9f2ed557072/scratchpad/coordinator-context-8d-design.md
DESIGN: ready (<lines> lines)
```

or:

```text
/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9b7e334d-919d-47e9-9555-a9f2ed557072/scratchpad/coordinator-context-8d-design.md
DESIGN: blocked (<reason>)
```

(The seat's prompt widens the return to under 3,000 characters with the unit list and the open questions; that return shape wins for this round.)
