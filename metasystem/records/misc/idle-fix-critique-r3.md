# Idle bar-(a) fix — codex critique round 3, verdict: NOT impossible on every runtime

Critic: codex gpt-5.6-sol (idle-fix-a-critique-b) over reviewedTree
af5954fa. 7 findings, all material (6 critical). Terminal: NOT impossible
by honest agent on EVERY runtime. Trajectory across the fix's three
critiques: 6 -> 6 -> 7 - NOT converging. The deep cause: steward is
notify-only; impossible-on-non-hook-runtimes needs active re-engagement
(watch-verb acting side), a separate mechanism. The other six are
closeable fold holes.

## F-AGNOSTIC-NOTIFY-ONLY-001 [critical]

Terminal statement: No. Idle-with-backlog is not impossible for an honest agent on every runtime. With claimable backlog and no live work, the runtime-independent steward only queues an operator notification; it performs no action that resumes the Codex or Devin seat.

Evidence: The binding goal requires an enforcing owner with acting-side re-engagement at metasystem/plans/goals/idle-with-backlog-alarm.md:6. The patch returns the notification action at metasystem/internal/steward/verdict.go:106-109. That action only queues a message at metasystem/internal/steward/tick.go:215-226, while metasystem/internal/steward/runner.go:99-114 invokes continuation machinery only for the separate revival action. Thus a Codex or Devin turn can end, the tick can report idle-backlog-dead, and the seat remains idle indefinitely.

## F-AGNOSTIC-LIVE-IDLE-BYPASS-002 [critical]

A live process is incorrectly treated as proof of active work. An honest Codex or Devin seat can retain a live announced main process, end its turn with a claimed goal plus a separate ready goal, and make the steward return healthy without catching the idle backlog.

Evidence: metasystem/internal/goal/project.go:387-393 adds a claim to InFlight solely because a matching main process is alive. metasystem/internal/steward/openwork.go:46-51 gives that fact precedence over claimable backlog, and metasystem/internal/steward/verdict.go:98-104 returns healthy with no action before considering progress. metasystem/internal/goal/turnverdict.go:234-235 applies the same exemption. The supplied fixture at metasystem/internal/goal/turnverdict_idle_test.go:212-223 explicitly requires this quiet outcome with both a ready goal and a live claimed main. Process liveness cannot distinguish an agent doing work from the same agent waiting after its turn ended.

## F-TEMPLATE-STATE-ROOT-SPLIT-003 [critical]

The template checkout's Stop hook judges the wrong state root, so its end-of-turn decision does not see the accepted backlog and a real human's stop authorization is written or looked up in a different state tree.

Evidence: metasystem/scripts/agents/supervision-hook.sh:65-66 resolves the Git top level and passes it unchanged to the verdict at :274-276. In template mode, metasystem/internal/stateroot/stateroot.go:97-108 places state beneath the metasystem installation. The current accepted tree confirms its goal root is metasystem/plans/goals/backlog.md. Nevertheless metasystem/internal/goal/actor.go:57-63 probes plans/goals/backlog.md relative to the supplied Git root and falls into legacy mode when absent. The session-stop command likewise passes its raw --root value to holder lookup and storage at metasystem/cmd/metasystem/session_stop.go:41-42 and :68-95. Its test constructs only an unvendored root at metasystem/cmd/metasystem/session_stop_test.go:17-56. In this repository's supported self-hosted layout, the hook can therefore stop quietly without seeing backlog, while the attended-human path cannot reliably meet its consumer.

## F-WORLD-DETECTION-FAILOPEN-004 [critical]

Migrated-world detection still fails open: an unreadable or missing accepted goal-tree reference is interpreted as a legitimate legacy checkout, allowing the end-of-turn decision to skip the fresh canonical read and return quiet.

Evidence: metasystem/internal/goal/actor.go:57-63 converts every failure reading the accepted reference or its root record into the boolean value false. metasystem/internal/goal/project.go:203-206 then selects the legacy reader instead of returning uncertainty. When the legacy ledger is absent, metasystem/internal/goal/project.go:251-259 returns empty work without an error, and metasystem/internal/goal/turnverdict.go:234-235 returns without blocking. This directly defeats the required rule that inability to prove the canonical backlog must block rather than change worlds.

## F-STOP-DEADLINE-INCOMPLETE-005 [critical]

The five-second Stop-hook deadline is not an end-to-end deadline. Slow but lawful pre-verdict work or accepted-tree parsing can consume the provider budget, after which even the new bounded fetch cannot emit its blocking result before the provider ends the turn.

Evidence: metasystem/scripts/enforcement/claude-code-hooks.json:24-26 caps the entire supervision hook at five seconds. Before invoking the verdict, metasystem/scripts/agents/supervision-hook.sh:145-166 performs arming, health, and digest work, and :256 performs another watchdog read. The verdict starts only at :274. metasystem/internal/goal/project.go:37-43 allows four seconds for the fetch, while :49-64 performs accepted-reference and tree loading after that timed stage; the timer at :99-113 covers only fetchForProjection. The fixture at metasystem/internal/goal/turnverdict_idle_test.go:98-111 tests the fetch helper directly and does not exercise the complete shell lifecycle. The binding finding establishes that a provider hook timeout is non-blocking, so this remains a quiet-stop path.

## F-SANCTIONED-LIBRARY-AUTH-BYPASS-006 [critical]

An agent-classified caller can obtain a valid stop authorization through the sanctioned library interface without human classification. This is an API authorization defect, not raw-byte forgery.

Evidence: The command rejects non-human callers at metasystem/cmd/metasystem/session_stop.go:51-60, but the exported WriteSessionStop library method at metasystem/internal/goal/sessionstop.go:133-150 deliberately receives no classification proof and creates the authorization identifier itself. Validation at :98-121 checks only field shapes and a process identity, while consumption at :276-281 checks expiry and process liveness, not whether that process was ever classified as human. The production-style fixture calls this unclassified writer directly at metasystem/internal/goal/turnverdict_idle_test.go:115-128. An in-repository caller can therefore supply its own live process as Human and mint the same authorization the command would have produced.

## F-MARKER-NOHOOK-LIFECYCLE-007 [high]

Single-use and expiry do not bind authorization to a no-hook session lifecycle. A marker that never reaches the end-of-turn decision can survive parking or a logical session end and authorize a later stop while the same holder, lease generation, session identifier, and named processes remain alive.

Evidence: metasystem/internal/goal/turnverdict.go:206-223 is the only consumer and runs only when TurnVerdict is invoked. The SessionEnd path at metasystem/scripts/agents/supervision-hook.sh:341-345 retires the announcement but does not consume or delete session-stop authorization, and no steward code references session-stops. metasystem/internal/goal/sessionstop.go:276-281 checks only the eight-hour expiry and process liveness; it does not re-prove continued terminal attendance. The park fixture at metasystem/internal/goal/turnverdict_idle_test.go:168-180 succeeds only because it explicitly invokes TurnVerdict while work is absent, which is precisely the call Codex and Devin cannot be assumed to make.
