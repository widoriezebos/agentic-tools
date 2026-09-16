# Codex design critique of clause 5 amendment r1

Job m1b-cw5-crit-1789526763, against metasystem/plans/coordinator-wakes-clause5-amendment-r1.md (landed a64d4408a).

## Finding 1 [critical]

Section 7.3, lines 35–44, does not establish the identity of the consuming inference request. The recorded `blocking` value is only an adapter's static answer; it does not observe the outer tool invocation, foreground execution, result delivery, or the next provider request. Therefore the first same-session usage sample after `returnedAt` can be an automatic or otherwise unrelated request, making the amendment's central definition an estimate and permitting clause 5 to pass against the wrong request. This must be fixed before a builder starts.

Evidence: metasystem/scripts/agents/adapters/runtime-common.sh:15–29 merely validates arguments and prints `blocking`. metasystem/internal/adapter/runtime_test.go:14–59 proves only that output shape. The existing delivery contract at metasystem/docs/design/turn-verdict-delivery-contract.md:43–48 separately says real-host blocking evidence is required, while lines 118–126 still classify the runtimes as unobserved or synthetic. No event or usage record joins a wait identifier to an inference request.

## Finding 2 [critical]

Section 7.4, lines 60–77, can produce a passing sample whose true latency exceeds 60 seconds. Drift detection covers only `wait-returned` to `seat-turn-ended`; it neither compares the previous observation's wall and boot clocks with the return nor gives a usage sample a boot timestamp. A backward wall step after a previous observation but before return can make `publishedLower` later than `returnedAt`, after which a stable post-return clock yields a small or negative wall-time latency despite more than 60 monotonic seconds having elapsed. This must be fixed before a builder starts.

Evidence: The waiter row stores LastObservedBootNanos at metasystem/internal/run/waiter.go:162–163, but the proposed `wait-returned` fields at amendment line 27 omit it. The waiter check at waiter.go:695–704 rejects a wall time before registration, not a wall time before the last successful observation. Amendment line 77 compares only return with turn end, so a completed pre-return clock step is invisible; the edge rules contain no ordering validation that would reject returnedAt before publishedLower.

## Finding 3 [high]

The runtime-without-usage-stream path in sections 7.3 and 7.4 does not prove that its selected `seat-turn-ended` belongs to the request consuming the wait result. The event carries only the runtime session, and the design selects the first later event even though plain reports, goal-next calls, duplicate gate evaluations, or concurrent callers can generate an event unrelated to that resumed request. The current no-hook contract also does not require a plain report at every turn end. The turn-end timestamp would be a valid conservative upper edge if correlated, but this design supplies no such correlation. This must be fixed before a builder starts.

Evidence: Amendment lines 28–29 give `seat-turn-ended` no turn or invocation identity, while line 72 selects solely by session and time. The fallback contract at metasystem/docs/design/turn-verdict-delivery-contract.md:50–58 requires a report only after a governed Stop, not at every turn end. The proposed location at metasystem/internal/goal/turnverdict.go:447 is conditional and follows earlier exits at lines 410–430. TestSeatTurnEndedAtGate covers one normal call but no unrelated or concurrent same-session verdict.

## Finding 4 [high]

The session mismatch rule in section 7.1 line 17 and section 7.4 line 83 rejects a lawful session transition. `SessionID != EffectiveRuntimeSession()` is expected after a resume, clear, or compaction replaces the associated runtime session; treating that condition as `session-key-mismatch` makes every such successor session unprovable. Builder item 6 also omits how existing cursor, sample, and registration records are retained, migrated, or aliased during the key change. This must be fixed before a builder starts.

Evidence: metasystem/internal/lease/verbs.go:21–25 and 251–268 deliberately replace RuntimeSession while retaining the announcement's SessionId and PreviousRuntimeSession. TestAssociateSessionTransitions at metasystem/internal/lease/verbs_test.go:287–326 proves this is intended behavior. Cursor and sample paths embed the supplied session key at metasystem/internal/usage/calls.go:108–116, and cursor validation requires that exact key at cursor.go:454–461; the amendment's one-line change defines no cutover for already recorded data.

## Finding 5 [high]

The measurement join omits the runtime half of the usage-store key. Wait rows and the proposed lifecycle events carry `runtimeSession` but not the runtime name, while `Calls` requires `(runtime, session)`. Because session strings are not globally unique across runtimes, the optional `--runtime` flag cannot safely default and a builder must guess whether to search the append-only registry, current announcements, or every runtime. A wrong choice can join another runtime's request. This must be fixed before a builder starts.

Evidence: The required fields at amendment lines 25 and 27 contain runtimeSession but no runtime; metasystem/internal/run/waiter.go:133–175 likewise stores no runtime. metasystem/internal/usage/cursor.go:222–246 and calls.go:108–116 key storage by both runtime and session. Amendment line 93 makes `--runtime` optional and promises per-runtime output without specifying an authoritative retained mapping.

## Finding 6 [high]

The five event contracts in section 7.2 are not durable or exactly once. `wait-registered`, `wait-returned`, and `seat-turn-ended` can be skipped by a crash after their owning state change; `wait-published` can be dropped or repeated independently of the publication; and concurrent or repeated `wait measure` runs can duplicate `wait-consumed`. Because terminal rows may later be replaced and the design says rows are not retained input, some lost evidence is unrecoverable. This must be fixed before a builder starts if these events remain the measurement source.

Evidence: Amendment line 21 explicitly chooses best-effort emission, and line 56 rejects rows as retained input. metasystem/internal/events/emit.go:41–50 and 124–133 silently drops failures, appends without synchronization, and provides no key-based deduplication. finishV2 persists at waiter.go:647–659 and only then reaches the proposed emit point, leaving a crash gap. The proposed once-per-pair `wait-consumed` rule names no lock, durable key, or compare-and-write owner.

## Finding 7 [high]

Section 7.4 cannot derive its `no-wait-returned` outcome from the declared inputs. It says samples are created from `wait-returned` events, then claims a dropped `wait-returned` is listed as unavailable, but a registration without a return is indistinguishable from a still-pending wait, a measurement cut before return, a crashed emitter, or a session that has not ended. Rows are expressly excluded as retained input. The builder therefore has to invent the expected-return universe and stream-completion rule. This must be fixed before a builder starts.

Evidence: Amendment line 58 starts sample creation from each ready, non-entry `wait-returned`; lines 79–85 nevertheless list `no-wait-returned` and several overlapping stream-end reasons. Line 56 says rows are not retained input. TestWaitMeasurementAccounting at line 100 demands `no-wait-returned` without defining which other durable fact proves that a return should exist.

## Finding 8 [high]

Builder item 7 can miss an existing usage witness because it instructs `wait measure` to call `Calls`, which only reads normalized samples already ingested by another steward operation. The design does not require a final `LatestCall` ingestion, define who performs it after the resumed request, or distinguish stale normalization from a runtime with no sample. Real legs can therefore fall back to turn-end or report unavailable even when the raw usage stream contains the consuming request. This must be fixed before a builder starts.

Evidence: metasystem/internal/usage/cursor.go:222–295 only reads the cursor-bounded samples file and returns empty when no cursor or normalized bytes exist. Raw Claude and Codex ingestion is performed by LatestCall in metasystem/internal/usage/calls.go:65–145. Amendment lines 52 and 113 call the usage stream an accelerator but specify only Calls, with no freshness owner or ingestion step.

## Finding 9 [high]

The amendment moves a measurement effect without the mandatory `Moved effects` inventory. Builder item 4 tells the goal observer to leave `terminalStamp` empty and transfers the goal publication lower-edge responsibility to `prevObservedAt`, but the shipped goal observer currently writes the accepted history row timestamp. The same item inaccurately presents job and attempt stamping as new work although both already do it. This must be fixed before a builder starts.

Evidence: The moved-effects validator reported that the section is absent. Current writes are metasystem/internal/dispatch/watch.go:126, metasystem/internal/proofrun/attempt.go:334, and metasystem/internal/goal/attention.go:879. Amendment line 110 directs a different goal behavior, while lines 62–70 assign different clock treatment to that effect.

## Finding 10 [medium]

Two section 7.2 emit-point instructions are internally unimplementable without an unmade decision. `wait-published` must be emitted before the FIFO loop while carrying `matched` and `hinted`, but those counts are learned only during that loop. The successful at-entry publication is said to be a no-row return at waiter.go:873, while the actual successful at-entry path writes and delivers a row before calling the same `finishV2` used by later observations; `finishV2` receives no at-entry flag. A builder must choose new state or a different emit point. This must be fixed before a builder starts.

Evidence: NotifyWaiters initializes counts and increments them inside metasystem/internal/run/wait_hint.go:148–169. The actual at-entry success reaches finishV2 at metasystem/internal/run/waiter.go:982–983 after persistence at lines 925–980; line 873 is instead an invalid-incarnation failure before row creation. Amendment lines 26–27 and builder items 2–3 do not resolve either contradiction.

## Finding 11 [high]

The seven fixtures in section 7.5 can pass while the governing rules remain false. No fixture places an unrelated same-session inference between return and actual consumption; the adapter fixtures only check the word `blocking`; TestWaitPublishedAtOwners omits the landing-script owner; TestSeatTurnEndedAtGate omits conditional and early-return gate paths; and the clock fixture does not require a pre-return wall step. These omissions allow the central association, all-owner coverage, every-turn emission, and full-window clock rule to be violated without a failing test. The fixture obligations must be corrected before a builder starts.

Evidence: Amendment lines 97–103 list only synthetic time-ordered samples for the usage join. The publication fixture names dispatch, proofrun, and goal Go tests but not metasystem/scripts/agents/land.sh, although amendment line 11 names that as an owner. The turn-end fixture has only normal, absent, and foreign sessions. Existing adapter tests at metasystem/internal/adapter/runtime_test.go:14–59 exercise adapter output, not host tool-result consumption.

## What the critique did not check

- No future fixture named in section 7.5 exists to run, so fixture sensitivity was judged from its specified setup and assertions rather than execution.
- No live Claude, Codex, or Devin host trace was available in the repository evidence; the core carrier finding therefore relies on the shipped adapter seam and the contract's own unobserved classifications.
- The worktree contained unrelated modified and untracked files before review. None was edited or used as implementation evidence beyond the brief-named amendment and design materials.
