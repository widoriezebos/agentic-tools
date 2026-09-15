# Amendment 8d, critique round 1: findings judged against origin/main

Reader for m1e, 2026-09-15. Code read at origin/main a24ecff03 (member A of registered-wait-matches-the-runtime-session is c4e7d0f31). Member A touched only lease (classify.go, verbs.go), up.go, cmd lease.go/up.go/main.go (+1 line at :602), census/announcement.go and goal/sessionstop.go; every other cited line is byte-identical to 098a48645. Proposals only; the seat rules.

Counts: accept 16, accept-moved 0, reject 1.

## G1. Authority while two sessions live (101, 106, 110)

Root cause: D6 to D9 move main-ness (lease, claim, mutation fence) to the successor without a fenced transfer or return.

- CC8D-101 (critical). The successor cannot become the holder while the predecessor lives; D6.1 announces it as a main, which classifies as advisor. Confirmed: claim.go:78-128 unchanged (a live different process never yields; one runner-edge exception at :93-116); handoff_capture.go:322-330 (main class needs MainId == HolderMainId). up.go: member A moved the advisor branch to :691-705 and :720-724 and inserted the session association at :682-688; semantics unchanged. Extra fact for r2: an active continuation already records a handoff as delegate class (context_verbs.go:223-233, handoff_capture.go:282-309), so section 1.6 "can record no handoff of its own" holds only for the Stop hook. Accept.
- CC8D-106. Clearing the marker when the successor dies returns neither lease nor claim, and an idle predecessor gets no Stop. Confirmed: reap.go:31-79 queues a notice and closes the chain only; ownership is machine plus lineage (goal/verbs.go:502-506, :964-969). Accept.
- CC8D-110. D9.3's session fence has no identity token and names the wrong files. Confirmed: handlers in goalsync_mutations.go:2260-2289; VerbRequest (goal/verbs.go:197-220) has no runtime session. Changed context: member A added `RuntimeSession`/`PreviousRuntimeSession` to lease.Announcement (classify.go:23-25) and `lease associate-session`; B1 (in build) makes the hook pass it. That is the natural token. Accept.

## G2. Unit graph and unit contracts (102, 116, 114, 108)

Root cause: units name missing or wrong surfaces and activate before their dependencies.

- CC8D-102 (critical). U3a-2 calls `HandoffAndLaunch`, which only U3c-1 adds; U3c-1's preflight needs `context resume` (U3c-3); U1 must precede all U3c. Confirmed in the design text (rows U3a-2, U3c-1 to U3c-5; D5.7) and P7 (program page :130). U1 on trunk: A landed, B1 in build, B2, B3, C open, so U3c-3 is still blocked. Accept.
- CC8D-116. Sizes are "about N", test files omitted, fixture-bed legs as sole witnesses (D2.1, D6.1). Confirmed: design-principles.md:120-130 needs `Changed-line allocation: <number>`; P7 :130 needs a focused witness without bed or live session. Note: r1's units total about 2,560 lines against P7's about 1,320 for U3a+U3e+U3b+U3c. Accept.
- CC8D-114. U3a-1 omits validate.go and metasystem.conf. Confirmed: validate.go:496-512 registers numeric knobs; metasystem.conf:27-35 has only spend keys. Accept.
- CC8D-108. U3e edits .claude/settings.json, but adoption and `hooks check` read scripts/enforcement/claude-code-hooks.json. Confirmed: registration.go:69-72, runtimes.go:254, the shipped file (:1-38, three events), setup.go:153-185, hooks.go:40-75. Accept.

## G3. Launch and failure lifecycle (105, 109)

- CC8D-105. "The tick retries" is false: the intent is consumed before launch, a failed launch escalates, retry reads live intents only, and a consumed unstamped intent is reaped. Confirmed: revive.go:189-206, :343-351; runner.go:171-185; reap.go:183-205. Accept.
- CC8D-109. D2.4's non-blocking capture fault plus D6.2 (allowed Stop ends a headless process) ends a session with no successor, against P2's "No path ends a session with no successor started" (program page :62). Confirmed. Accept.

## G4. Records against real shapes (104, 111, 113, 115)

- CC8D-104 (critical). "Agent tool_use with no result" never means running. Confirmed in the cited transcript: :418 assistant `tool_use` name Agent, id `toolu_…`; :419 an immediate `tool_result` with the same tool_use_id carrying `agentId` and `output_file`; :484 a user record `<task-notification>` with `<task-id>` = agentId, `<tool-use-id>`, `<output-file>`, `<status>completed`. Accept.
- CC8D-111. `openWork` drops `WaiterTarget` (job startedAt/round/operationId, run generation/launchNonce, attempt digest). Confirmed: waiter.go:45-53, :133-176 (`Target`); the fresh wait resolves current work (wait_verb.go:180-223). Accept.
- CC8D-113. The note check proves only an mtime. Confirmed: PidStartedAt whole seconds (classify.go:28 after member A, was :24). But S7's check column is the mtime order (orchestration.md:370), binding. Accept narrowed: keep S7's check; add location (seat memory directory), every declared delegate's output path present in the note, strict same-second handling; reject content validation of lessons and reasoning beyond what S7 checks.
- CC8D-115. `lessonsNote` scalar vs list; no schema migration. Confirmed: handoff_state.go:22 (version 1), :139-151 (DisallowUnknownFields), :304-307; splitter handles three lists only (handoff_capture.go:736-749). Accept.

## G5. Tool gate against R-114-m1e item 5 (107, 112)

- CC8D-107. Allowlist names `metasystem land` and `goal land` (absent), and D3.4 denies all but handoff and resume above the ceiling, blocking landings and waits. Confirmed: main.go:132-145 (`landing`), :534 (`goal land-ready`), land.sh:1-15; `wait` (:722) and `job watch` (:221) do exist. Accept.
- CC8D-112. The 100 ms bound covers only a proposed reader, not the hook. Confirmed: ReadOptions (calls.go:64-72) has no byte or deadline field (only NonBlocking); maxCallLineBytes 32 MB (cursor.go:19); the reader loops to EOF and writes the cursor; the hook calls `lease hook-delegate` before its work (supervision-hook.sh:1464-1476, :1581-1589). Accept.

## G6. Proof floor (103)

- CC8D-103 (critical). r1 builds p95 at the trigger and max at the ceiling "until Wido answers", weaker than the approved DONE (p95 under 150K, no call over 200K; goal record Intent, now revision 89, same numbers). Confirmed; rulings.md:174 item 6. Accept.

## Rejected

- CC8D-117 (non-material). Wrong brief filename in the header. Reject: the header says "coordinator-context-8d-brief.md (same directory)", which is true in the scratchpad; the `ctx-8d-*` names come from the worktree copies.

## Open questions posed wrong

- Q1 (DONE vs the 250K ceiling): posed as a forced choice with a weaker interim build (CC8D-103). R-114-m1e item 3 fixes the ceiling only ("a context ceiling of 250K in configuration, with the trigger at the ceiling minus the margin"); the margin is the design's. r2 keeps 150K/200K as DONE and witness and picks a margin and gate that aim at them. Ask Wido only if he wants the DONE restated.
- Q2 (allowlist): the recommended list names absent commands and a ceiling rule R-114-m1e item 5 already forbids ("never blocks a landing or an in-flight wait"). Re-pose after the grammar is fixed, or drop; `goal edit` is the design's call.
- Q3 and Q4: untouched by the critique.

## Findings that may need Wido

1. CC8D-101/106: if r2 gives the continuation the lease while the predecessor lives, that is a second exception to "a live holder never yields" (claim.go:117-122), an authority change no ruling covers. R-114-m1e item 2 picks only the launcher: "the automatic launcher: the steward continuation, a headless session the steward starts." If r2 keeps the continuation delegate class under the existing path, the seat can rule without Wido.
2. CC8D-103: only if the DONE numbers are to change. R-115-m1e item 6: "Program rule: no unit of the efficiency program lowers a proof floor, removes a witness or a gate, or narrows a DONE to save tokens, and each unit's read checks it."

## Not checked

The transcript shape on today's Claude build (the cited transcript is from 2026-09-05); `context_verbs.go:139-245` read only at :213-247; `cursor.go:135-215` by grep, not line by line.
