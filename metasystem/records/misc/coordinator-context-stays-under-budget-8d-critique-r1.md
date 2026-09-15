# Amendment 8d design critique, round 1

## CC8D-101

Severity: critical  
Material: yes

Claim: The successor cannot become the live seat main while the predecessor remains alive. The design has no lease succession mechanism that can make its proposed launch sequence work.

Evidence: The design says, "The launcher starts the continuation, which announces as the new steward main" and waits for that announcement before allowing the predecessor to stop (`metasystem/artifacts/reports/ctx-8d-design-r1.md:67-68`). A new main whose lineage differs from the live lease holder is classified as an advisor, not as the holder (`metasystem/internal/up/up.go:659-686`, `metasystem/internal/up/up.go:703-709`). Claim resolution preserves a different live holder except for the existing mission-runner child edge (`metasystem/internal/lease/claim.go:78-128`). The handoff recorder also requires the caller's main id to equal the current holder main id (`metasystem/internal/steward/handoff_capture.go:322-330`). U3c-2 does not name either lease claim or startup classification as an implementation surface (`metasystem/artifacts/reports/ctx-8d-design-r1.md:163`).

Design change: D6 and U3c-2 need an explicit, fenced lease succession operation. It must let the launched continuation become the sole holder while the predecessor stays alive only as a non-acting liveness guard.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: a named lease transition and witness prove that exactly one main has mutation authority throughout launch and acknowledgement.

## CC8D-102

Severity: critical  
Material: yes

Claim: The unit order activates a launcher before its required continuation command, seat behavior, claim transfer, and U1 dependency exist.

Evidence: U3a-2 says its second blocked Stop "stages and launches" through `HandoffAndLaunch` (`metasystem/artifacts/reports/ctx-8d-design-r1.md:158`). That function is not introduced until U3c-1 (`metasystem/artifacts/reports/ctx-8d-design-r1.md:162`). U3c-1 activates launch before U3c-2 makes the successor a seat main, U3c-3 adds `context resume`, and U3c-5 transfers claim authority (`metasystem/artifacts/reports/ctx-8d-design-r1.md:162-166`). Its own preflight requires `metasystem context resume --help`, so U3c-1 cannot activate before U3c-3 (`metasystem/artifacts/reports/ctx-8d-design-r1.md:63`). The binding program orders U1 before all U3c work (`metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:130-138`), and the goal record says U3c is after U3b and U1 (`metasystem/plans/goals/coordinator-context-stays-under-budget.md:10`).

Design change: Replace the unit graph with an order in which U1, resume support, seat-main startup, claim succession, and mutation fencing are present before any hook can launch a successor. State exactly which earlier unit owns `HandoffAndLaunch` if U3a-2 still calls it.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: the dependency graph is acyclic and every activated call resolves to behavior already built and witnessed.

## CC8D-103

Severity: critical  
Material: yes

Claim: The design lowers the approved proof floor while the corresponding question is still open. It does not hold under both answers.

Evidence: The design changes the proof to p95 at the configured trigger and maximum below the configured ceiling, with defaults of 200K and 250K (`metasystem/artifacts/reports/ctx-8d-design-r1.md:27-29`). Its first open question says the build will use those defaults until Wido answers (`metasystem/artifacts/reports/ctx-8d-design-r1.md:172`). The goal's approved DONE remains p95 below 150K and no call above 200K (`metasystem/plans/goals/coordinator-context-stays-under-budget.md:8`). R-115 item 6 forbids lowering a proof floor or narrowing DONE to save tokens (`metasystem/memory/rulings.md:174`).

Design change: Keep the 150K p95 and 200K maximum as the unit's DONE and witness unless Wido changes the goal. The question may ask whether policy thresholds should change, but the design must not choose a weaker interim build.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: the unit DONE and acceptance witness preserve the recorded goal thresholds under either answer to the open question.

## CC8D-104

Severity: critical  
Material: yes

Claim: The proposed Claude transcript cross-check will classify a running background delegate as complete and cannot verify the declared delegate id.

Evidence: The design says a running delegate is an "Agent tool_use id whose result record is absent" and requires `--delegate id=<agent id>` (`metasystem/artifacts/reports/ctx-8d-design-r1.md:51`). It also admits that the real background Agent transcript shape has not been checked (`metasystem/artifacts/reports/ctx-8d-design-r1.md:179`). In a current Claude transcript, the Agent tool call has a tool-use id but no agent id, the next line is an immediate tool result containing the launched agent id, and only a later task notification records completion and the output file (`/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b-metasystem/542aa325-ce7a-4500-acc3-736972650bc3.jsonl:418-419`, `/Users/wido/.claude/projects/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b-metasystem/542aa325-ce7a-4500-acc3-736972650bc3.jsonl:484`). The immediate result means "result absent" is false while the delegate is still running. The tool-use id and agent id are distinct fields.

Design change: D4.3 and U3b-2 need a transcript state machine based on launch result, completion notification, agent id, task id, request text, and output path. `--no-delegates` must fail whenever that state machine finds any launch without a terminal notification.

Rigor: severe. Facts: local=true; recoverable=false; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=true; externalSideEffectBoundaryCrossed=false. Reopening trigger: checked Claude fixtures prove running, completed, failed, missing-notification, and mismatched-declaration cases against the actual record shape.

## CC8D-105

Severity: high  
Material: yes

Claim: A successor launch failure is not retried by the current steward path. The proposed retry statement is false.

Evidence: The design says that after a missing start signal the predecessor stays live, and "D2.4 takes over on the next tick and retries the same idempotent operation" (`metasystem/artifacts/reports/ctx-8d-design-r1.md:61`). The current revival code consumes the intent before launch and returns an escalation if launch fails (`metasystem/internal/steward/revive.go:189-204`). Retry selection considers only live intents (`metasystem/internal/steward/revive.go:343-351`, `metasystem/internal/steward/runner.go:171-185`). A consumed intent without a launch stamp is held and later reaped, not relaunched (`metasystem/internal/steward/reap.go:183-205`).

Design change: D5 must define a durable launch state and retry transition for consumed but unacknowledged continuations. It must distinguish launch failure from launch success with stamp-write failure and keep the predecessor authoritative until one successor is confirmed.

Rigor: severe. Facts: local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true. Reopening trigger: a failure-injection witness proves an unacknowledged launch is retried or safely reconciled without duplicate successors.

## CC8D-106

Severity: high  
Material: yes

Claim: Clearing the handoff marker after a successor dies does not return the lease or goal claim to the predecessor, and no event is guaranteed to wake the predecessor.

Evidence: The design says a reaped successor causes the next predecessor Stop to clear the marker and the predecessor then resumes under S1 (`metasystem/artifacts/reports/ctx-8d-design-r1.md:84`). Earlier, D9 moves the goal claim to the successor's lineage and main (`metasystem/artifacts/reports/ctx-8d-design-r1.md:89-91`). Current ownership is keyed by machine and lineage, and a second lineage is refused (`metasystem/internal/goal/verbs.go:502-506`, `metasystem/internal/goal/verbs.go:964-969`). The reaper queues a notification and closes a run; it does not transfer a lease or goal claim (`metasystem/internal/steward/reap.go:31-79`). If the predecessor is idle, there may also be no later Stop hook to perform D8.4.

Design change: D8.4 needs an event-driven recovery owner plus fenced lease and goal-claim return. Its witness must cover successor death both while the predecessor is active and while it is idle.

Rigor: severe. Facts: local=false; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true. Reopening trigger: a death witness proves authority returns automatically and exactly once, without depending on another predecessor Stop.

## CC8D-107

Severity: high  
Material: yes

Claim: The tool gate's allowlist names commands that do not exist and its over-ceiling rule denies the very landing and wait paths that R-114 requires it never to block.

Evidence: The design allowlists `metasystem land` and `metasystem goal land`, then says that above the ceiling every command outside handoff and resume is denied (`metasystem/artifacts/reports/ctx-8d-design-r1.md:43-44`). The CLI has a `landing` family, not `land`, and the goal family has `land-ready`, not `land` (`metasystem/cmd/metasystem/main.go:132-145`, `metasystem/cmd/metasystem/main.go:500-542`). The repository landing surface is `bash scripts/agents/land.sh` (`metasystem/scripts/agents/land.sh:1-15`). R-114 requires U3e never to block a landing or an in-flight wait (`metasystem/memory/rulings.md:173`).

Design change: D3 must classify the actual command grammar, including the landing wrapper, and preserve landing plus already registered wait and job-watch operations even above the ceiling. Add deny-path witnesses for each spelling.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: command-level witnesses show every real landing and in-flight wait form is allowed at both thresholds while unrelated work is denied.

## CC8D-108

Severity: high  
Material: yes

Claim: U3e edits the live Claude settings file but not the shipped hook configuration that setup and `hooks check` use. The hook would not survive adoption and would not be checked for drift.

Evidence: U3e names `.claude/settings.json` as its configuration file and claims `hooks check` will verify the added PreToolUse event (`metasystem/artifacts/reports/ctx-8d-design-r1.md:41`, `metasystem/artifacts/reports/ctx-8d-design-r1.md:159`). Claude hook adoption reads `scripts/enforcement/claude-code-hooks.json` (`metasystem/internal/runtimes/registration.go:69-72`, `metasystem/internal/runtimes/runtimes.go:254`). That shipped file currently defines only SessionStart, Stop, and SessionEnd (`metasystem/scripts/enforcement/claude-code-hooks.json:1-38`). Setup merges that shipped source, and hook checking compares its event arrays with the live settings (`metasystem/internal/hostsetup/setup.go:153-185`, `metasystem/internal/hooks/hooks.go:40-75`).

Design change: U3e must own the shipped Claude hook configuration and its setup and check witnesses, with `.claude/settings.json` treated as adopted output rather than the sole source.

Rigor: unproven. Facts: local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: setup and hook-check fixtures prove PreToolUse is installed, preserved, and detected when removed.

## CC8D-109

Severity: high  
Material: yes

Claim: The headless capture-failure path can end the current session without a successor.

Evidence: D2.4 says a capture failure is recorded but does not block the current Stop (`metasystem/artifacts/reports/ctx-8d-design-r1.md:36`). D6.2 says an allowed Stop in headless Claude ends the process (`metasystem/artifacts/reports/ctx-8d-design-r1.md:68`). The binding program states that no path may end a session with no successor started (`metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:62`).

Design change: D2.4 and D6.2 must define a fail-open result that is safe for interactive runtimes and a fail-closed continuation result for headless seat mains. A capture fault must not terminate the only live steward.

Rigor: severe. Facts: local=false; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true. Reopening trigger: a headless capture-failure witness proves the current main remains alive and authoritative until a successor is confirmed.

## CC8D-110

Severity: high  
Material: yes

Claim: U3c-5 cannot implement the promised session-specific mutation fence through the files and request shape it names.

Evidence: D9.3 says every claim, land, goal edit, and handoff verb from the handed-off session returns `SESSION_HANDED_OFF` while its successor runs (`metasystem/artifacts/reports/ctx-8d-design-r1.md:91`). U3c-5 names `internal/goal/verbs.go`, `internal/goal`, `cmd/metasystem/goal.go`, and `cmd/metasystem/main.go` (`metasystem/artifacts/reports/ctx-8d-design-r1.md:166`). The actual goal mutation and landing handlers are in `cmd/metasystem/goalsync_mutations.go` (`metasystem/cmd/metasystem/goalsync_mutations.go:2260-2289`). Context handoff is handled separately in `cmd/metasystem/context_verbs.go` (`metasystem/cmd/metasystem/context_verbs.go:139-245`). The internal goal request carries actor and caller class but no runtime session identity (`metasystem/internal/goal/verbs.go:197-220`).

Design change: D9 and U3c-5 must name the identity token, its propagation, and every mutation entry point. The refusal must be scoped to the predecessor session, not to all callers sharing its process, lineage, or machine.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: entry-point witnesses prove only the handed-off session is fenced across claim, land-ready, goal edits, and context handoff.

## CC8D-111

Severity: high  
Material: yes

Claim: The wait handoff record omits the lifecycle target needed to prove that the successor waits on the same job, run, or attempt.

Evidence: D4.1 records the complete `WaitSelector`, deadline, and local child pid plus birth tuple, then says the successor freshly registers each wait (`metasystem/artifacts/reports/ctx-8d-design-r1.md:49`). A stored waiter also has a `WaiterTarget` that pins job start time, round, and operation id, run generation and launch nonce, or attempt proof digest (`metasystem/internal/run/waiter.go:45-68`, `metasystem/internal/run/waiter.go:133-176`). A fresh wait resolves current open work before storing its row (`metasystem/cmd/metasystem/wait_verb.go:180-223`). Reusing the selector alone can attach the successor to a later lifecycle with the same visible id.

Design change: D4.1 and U3b-1 must persist and verify the full `WaiterTarget`, then define whether resume re-registers that exact target or rejects a changed lifecycle.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: a reused-id witness proves the successor cannot silently wait on a different lifecycle.

## CC8D-112

Severity: high  
Material: yes

Claim: The design does not bound the complete U3e hook to about 100 ms. It bounds only a proposed reader while the current read path can consume the whole transcript and the hook performs more work afterward.

Evidence: D3.2 proposes a 100 ms reader deadline, a 256 KB tail, and a 5 second Claude hook timeout, then D3.5 treats the observed cost as evidence for every call (`metasystem/artifacts/reports/ctx-8d-design-r1.md:41-45`). The current cursor reader has no byte or deadline option (`metasystem/internal/usage/calls.go:64-72`), permits a 32 MB line, and reads from the cursor to EOF before persisting samples and cursor state (`metasystem/internal/usage/cursor.go:19`, `metasystem/internal/usage/cursor.go:135-215`). The supervision hook also resolves runtime and engine state and delegates through additional commands on its common path (`metasystem/scripts/agents/supervision-hook.sh:1461-1535`, `metasystem/scripts/agents/supervision-hook.sh:1581-1589`). R-114 bounds the hook itself to about 100 ms (`metasystem/memory/rulings.md:173`).

Design change: D3 and U3e must specify the early hook path, the new bounded-read API, the total wall-clock deadline, and what is skipped on timeout. The witness must measure end-to-end hook latency, not an injected reader delay alone.

Rigor: unproven. Facts: local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: an end-to-end latency witness covers a 256 KB tail, a 32 MB partial line, a cold process, and a slow or unavailable engine while preserving the landing and wait allow paths.

## CC8D-113

Severity: medium  
Material: yes

Claim: The S7 check proves only that an arbitrary path was modified recently. It does not prove that the required lesson, reasoning, and delegate output paths are in the seat's note.

Evidence: D4.4 accepts any `--note <path>` whose modification time is after the main's `PidStartedAt`, records it, and has the verifier compare `modifiedAt` with handoff `writtenAt` (`metasystem/artifacts/reports/ctx-8d-design-r1.md:52`). S7 requires a memory note containing session lessons, reasoning in flight, and every delegate output path (`metasystem/docs/orchestration.md:370`). `PidStartedAt` is stored at whole-second precision (`metasystem/internal/lease/classify.go:21-35`). Neither an arbitrary path nor an mtime comparison proves the content or reliably separates writes in the same second.

Design change: D4.4 and U3b-2 must constrain the note location and validate its required content against the delegate records. Use an identity or digest tied to this session write, not only wall-clock mtime.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: negative witnesses reject a stale note, a same-second prior note, an out-of-scope path, and a note missing any delegate output path.

## CC8D-114

Severity: medium  
Material: yes

Claim: U3a-1 omits the registry and default configuration surfaces needed for valid, discoverable context keys.

Evidence: D1.1 adds `context.ceiling.tokens`, `context.trigger.margin.tokens`, and `config.ContextBudget(root)` (`metasystem/artifacts/reports/ctx-8d-design-r1.md:27`). U3a-1 names context and spend code plus docs, but not the config validator or `metasystem.conf` (`metasystem/artifacts/reports/ctx-8d-design-r1.md:157`). Operational numeric keys are explicitly registered in validation so typos surface there (`metasystem/internal/config/validate.go:496-512`). The checked-in configuration currently exposes only the spend settings in this area (`metasystem/metasystem.conf:27-35`).

Design change: U3a-1 must own validation, defaults, and configuration documentation for both keys. It must state range and relationship checks, including a positive ceiling and a margin below the ceiling.

Rigor: unproven. Facts: local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: validation witnesses reject unknown, malformed, non-positive, and inverted context-budget settings and show the checked-in defaults.

## CC8D-115

Severity: medium  
Material: yes

Claim: The handoff state and overflow design is internally inconsistent about `lessonsNote` and does not define the schema migration for the new fields.

Evidence: D4.4 defines one `lessonsNote` path, while D4.5 calls `openWork`, `delegates`, and `lessonsNote` state lists moved by `splitHandoffStateLists` (`metasystem/artifacts/reports/ctx-8d-design-r1.md:52-53`). The current handoff state is schema version 1 and rejects any other version (`metasystem/internal/steward/handoff_state.go:22`, `metasystem/internal/steward/handoff_state.go:139-151`, `metasystem/internal/steward/handoff_state.go:304-307`). The current overflow splitter handles only open jobs, scratch, and messages (`metasystem/internal/steward/handoff_capture.go:736-805`).

Design change: D4.5 and U3b-2 must say whether `lessonsNote` is scalar or a list, which new collections can overflow, and how schema versioning remains backward compatible or migrates existing records.

Rigor: unproven. Facts: local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: round-trip and old-schema witnesses cover large delegate and open-work lists plus one lesson-note reference.

## CC8D-116

Severity: medium  
Material: yes

Claim: The unit contracts do not meet the binding size and focused-witness rules.

Evidence: The design gives approximate limits such as "about 300 changed lines," often omits test files from the file list, and assigns hook fixture-bed witnesses to U3a-2 and U3c-2 (`metasystem/artifacts/reports/ctx-8d-design-r1.md:157-168`, `metasystem/artifacts/reports/ctx-8d-design-r1.md:119`, `metasystem/artifacts/reports/ctx-8d-design-r1.md:136`). The design rules require an exact `Changed-line allocation: <number>` that includes tests and docs (`metasystem/docs/design/design-principles.md:120-130`). The binding program requires each unit to fit within 300 changed lines including tests and to have a focused builder-run witness that does not require the fixture bed or a live session (`metasystem/plans/seats-spend-tokens-in-bounded-sessions-design.md:130`).

Design change: Recut each unit with an exact allocation, complete file list including tests and docs, and one isolated falsifiable witness. Fixture-bed checks may remain integration evidence but cannot be the only unit witness.

Rigor: severe. Facts: local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false. Reopening trigger: every unit has an exact allocation at or below 300 and a named focused witness that runs without a live runtime or fixture bed.

## CC8D-117

Severity: low  
Material: no

Claim: The design header points to the wrong brief filename, which weakens provenance but does not change what an implementer would build because the task and design body identify the amendment.

Evidence: The design says its source is `artifacts/reports/coordinator-context-8d-brief.md` (`metasystem/artifacts/reports/ctx-8d-design-r1.md:3`). The actual round-one brief is `ctx-8d-brief-r1.md` and identifies amendment 8d (`metasystem/artifacts/reports/ctx-8d-brief-r1.md:1`).

Material findings: 16.

Verdict: rework.
