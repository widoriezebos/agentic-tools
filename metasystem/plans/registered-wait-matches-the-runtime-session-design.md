# Registered wait matches the runtime session

- Kind: design
- Id: 01M3A2YHDMV0SVQ4QJMWQWTHF1
- Status: superseded
- Goals: registered-wait-matches-the-runtime-session

Design for `metasystem/plans/goals/registered-wait-matches-the-runtime-session.md`. Revision 4.
Drafted 2026-09-15 on the m1c seat by a Claude Fable design delegate, from files re-read at worktree HEAD 0b8d9d62.
Revision 4 is the final prose revision under R-97-m1e with D81: after three critique rounds, all five members are marked ready to build, and every standing finding from rounds 1 to 3 is either closed in the text or carried as a named test in the unit that owns its rule. No further critique round runs; a code read of each unit carries its checks.
The revision record at the end holds every disposition, quoting the closed ones and naming the carried tests.
The parent is `metasystem/plans/coordinator-wakes-on-events-not-polls-design.md`; its member three landed in bf59fd0d.
This page is the design deliverable only. It changes no code, fixture or ledger.

## 1. The fault, stated once

The wait verb records the session it reads off the main's announcement, not a session the shell knows.
`metasystem job watch --job J --caller-pid $$` (`metasystem/cmd/metasystem/run.go:460-486`) calls the wait verb, which classifies the caller pid to an announced main (`metasystem/cmd/metasystem/wait_verb.go:144`, `metasystem/internal/lease/classify.go:383-415`) and copies `view.Announcement.SessionId` into `Owner.SessionId` and `RuntimeSession` (`wait_verb.go:157-158`); the row stores both (`metasystem/internal/run/waiter.go:881-884`).
The announcement's `sessionId` is the first announcer's string, never changed afterward (`metasystem/internal/lease/verbs.go:126-175`). `up` sets it from `--session`, then `METASYSTEM_SESSION_ID`, then `session-<pid>` (`metasystem/internal/up/up.go:256-265`). Two routes mint the `session-<digits>` placeholder: a shell `up` without `--session`, and a hook payload without `session_id`.
At Stop the hook presents the runtime's real session from the payload (`supervision-hook.sh:539,554-555`) to `report turn-verdict --session <runtime session>` (lines 1355-1358). The gate at `turnverdict.go:671` requires `row.Session == sessionID && row.RuntimeSession == sessionID`. A placeholder row carries the placeholder; the Stop presents the real session; they differ; the wait is refused with block source `unwatched-work`, naming the job it is watching.
The gate's comparison is not wrong. The row's contents are: the announcement handed registration a placeholder instead of the runtime session.

So the fix is upstream of the gate. Make the announcement carry the real runtime session; make registration copy that into the row; and the existing `row.Session == live Stop session` comparison then passes. The gate's continued comparison to the *live* Stop session is also what defeats a forged association (finding 1): a forged association changes a hint, not the session the runtime's own hook presents at a real Stop.
Round-2 also found two paths that bypass the gate — a token-free `LiveWaiter` and a raw `CurrentWaitingLines` prepend — which member B2 closes.

The historical cause is a reproduced hypothesis: the goal records the row's placeholder session and the refusal, but not the Stop's session or the announcement, so the exact placeholder route on m1b is not recoverable from this checkout. The mechanism is proven; the incident tuple is not.

---

## Member A — announcement carries the associated session

**Ready to build at revision 4.** Owns the record and its one writer. Changes no Stop decision. Depends on nothing in B1, B2, B3 or C.

Files: `metasystem/internal/lease/verbs.go`, `metasystem/internal/lease/classify.go`, `metasystem/internal/census/announcement.go`, `metasystem/internal/up/up.go`, `metasystem/cmd/metasystem/up.go`, `metasystem/internal/goal/sessionstop.go`, `metasystem/cmd/metasystem/lease.go`.
Fixtures: lease and up package tests only (no hook, no gate).

### A.1 The record

The announcement gains three optional fields: `runtimeSession` (the session currently bound to the process), `previousRuntimeSession` (the value a SessionStart replaced), `sessionAssociatedAt` (a stamp).
`sessionId` and the file name `<session>-<pid>.json` (`verbs.go:198`) keep their meaning as the first announced session, so every reader of the file identity is unaffected.
The fields are optional in the strict key contract `AnnouncementOptionalKeys` (`metasystem/internal/census/announcement.go:31-41`, validated at line 45, read by `classify.go:168` and `metasystem/internal/census/run.go:630`), in `lease.Announcement` (`classify.go:21-39`), and in `sessionStopAnnouncement` with its strict decoders (`sessionstop.go:70-84,201`; also `turnverdict.go:646`).

The record is a registration-time hint, not an authority. It supplies the runtime session to a shell that cannot know it. Whether a wait is honored is decided at the gate by comparing the row to the *live* Stop session (member B2); a wrong or forged `runtimeSession` therefore cannot make a wait pass. A alone grants no Stop allowance, so A can land before B2 exists.

### A.2 The one writer and its transitions

One internal operation writes the fields: `lease associate-session --main-id M --runtime-session S --event start|stop|end [--start-source SRC]`, called only by the hook path inside `up` (member B1 does the calling). The general announce path never touches them: `AnnounceWithProofAt`, `AnnounceWithPair`, `AnnounceWithProof` and the `lease announce` verb (`metasystem/cmd/metasystem/lease.go:29-50`) leave `runtimeSession` empty on a fresh mint and never change it on re-announce (`verbs.go:126-175`).
`up` reads the runtime session only from a new explicit flag, never from ambient state: `--runtime-session S`, `--no-runtime-session` and `--start-source SRC`; the association step ignores `METASYSTEM_SESSION_ID` (closing the fallback at `up.go:257`). `--session` keeps its present meaning for the first `sessionId` and the verdict-state key.

Transitions, under the registry lock, monotonic and order-safe:

- `runtimeSession` empty and S is not the placeholder grammar `^session-[0-9]+$`: fill it, set `previousRuntimeSession=""`, stamp. Any event may fill.
- `runtimeSession == S`: refresh the stamp only.
- `runtimeSession == B`, S differs and is not a placeholder: replace only when `--event start` with `--start-source` in `resume|clear|compact`. Move B to `previousRuntimeSession`, set S, stamp.
- `--event stop` or `--event end` with S differing from `runtimeSession`: no change. A late or out-of-order Stop or End for an earlier session never rolls the association back.
- S is a placeholder, or `--no-runtime-session`: no change ever.

`sessionAssociatedAt` is a stamp, not an ordering key; the rule, not the stamp, forbids rollback.

### A.3 Retirement and legacy

`Retire` (`verbs.go:215-233`, line 227) removes an announcement only when the requested session equals its `runtimeSession`, or, for a legacy announcement with no `runtimeSession` field, its `sessionId`. A late SessionEnd for an earlier session whose association has moved does not delete a live announcement.
An announcement with no `runtimeSession` field is unassociated. Its `sessionId` counts as the effective session only when it does not match `^session-[0-9]+$`; the sole producers of that shape are the `session-<pid>` default (`up.go:263`) and the hook's `session-$PPID`, so the grammar, not any particular pid, marks a placeholder. The m1c hook-minted uuid announcement needs no re-announce.

### A.4 Why A changes no Stop decision

The lifecycle token the human-stop marker binds is a hash of the decoded announcement (`sessionstop.go:173`). Optional fields are omitted from JSON while empty, so an announcement that is never associated marshals byte-for-byte as today, and its token is unchanged. The token changes only when the hook writes an association; a `session stop` marker is session-scoped, and a same-session Stop refreshes (no value change, no token change), so a live human-stop marker is never invalidated by A. A replacement of the session (SessionStart to a new session) does change the token, which correctly retires a marker minted for the earlier session. No Stop decision is altered by A; only the record grows.

### A.5 Tests member A owns

- `TestAssociateSessionTransitions` in `metasystem/internal/lease/verbs_test.go`: placeholder mint leaves `runtimeSession` empty; associate fills it; a same-session associate refreshes only the stamp; a `--event stop`/`end` with a differing session does not replace (late-Stop, late-End); a `--event start --start-source resume` replaces and moves the old value to `previousRuntimeSession`; a placeholder S never associates. (round-2 finding 3.)
- `TestAssociateSessionRetireMatchesCurrent` in `metasystem/internal/lease/verbs_test.go`: `Retire` deletes only when the session equals the current `runtimeSession` (or a legacy `sessionId`); a stale earlier session does not delete a live announcement. (round-2 finding 3.)
- `TestAnnounceNeverAssociates` in `metasystem/internal/lease/verbs_test.go`: `AnnounceWithPair`/`AnnounceWithProof` and the `lease announce` verb leave `runtimeSession` empty on mint and re-announce. (round-3 finding 1, defense in depth.)
- `TestLegacyPlaceholderGrammarUnassociated` in `metasystem/internal/lease/classify_test.go`: an announcement whose `sessionId` matches `^session-[0-9]+$` is classified unassociated regardless of the pid; a uuid `sessionId` counts as effective. (round-2 finding 4, legacy migration.)
- `TestUpAssociationIgnoresEnv` in `metasystem/internal/up/up_test.go`: `--runtime-session` drives the associate call; `METASYSTEM_SESSION_ID` never does; `--no-runtime-session` associates nothing. (round-2 finding 4, absence bit.)
- `TestAnnouncementLifecycleTokenStableWhenUnassociated` in `metasystem/internal/goal/turnverdict_test.go` (beside the SessionStop cases): the lifecycle token is byte-identical for an unassociated announcement carrying the new empty fields, and changes only when a real association is written — proving A alters no human-stop decision. Every strict reader (`census.ValidateAnnouncementKeys`, `lease.Announcement`, `sessionStopAnnouncement`) decodes an announcement carrying the fields.

---

## Member B1 — the hook writes the session, and a sessionless Stop reads no wait

**Ready to build at revision 4.** Owns the hook's session wiring and the absent-session short-circuit. Changes no observable Stop decision (a sessionless Stop has no wait allowance today either). Depends on A's flags; depends on nothing in B2, B3 or C.

Files: `metasystem/scripts/agents/supervision-hook.sh`, `metasystem/cmd/metasystem/goal.go`, `metasystem/internal/goal/turnverdict.go` (option and short-circuit only).
Fixtures: hook fixtures and one `report turn-verdict` unit test.

### B1.1 The hook wiring

The hook reads `session_id` from the payload (`supervision-hook.sh:539`). When it is present, the hook passes `--runtime-session <session>` (and, at SessionStart, `--start-source <source>`) to the `up` calls at lines 935-941 (Stop), 1506 (SessionEnd) and 1520 (SessionStart), so `up` associates through A's operation. SessionStart fires for `startup|resume|clear|compact`, so the association is written at session start, before any agent turn.
When `session_id` is absent, the hook passes `--no-runtime-session` to those `up` calls (A associates nothing) and `--session-absent` to `report turn-verdict` (verb flags at `metasystem/cmd/metasystem/goal.go:652-718`, call at line 718).

Wiring `--runtime-session` changes no Stop decision on its own: the gate at HEAD reads the announcement's `sessionId`, not `runtimeSession`, so B1 only fills a field the gate does not yet consult. B2 flips the gate. This is why B1 precedes B2 and does not depend on it.

### B1.2 The absent-session short-circuit

`TurnVerdictOptions` gains `SessionAbsent`. `TurnVerdict` normalizes the session before any gate reads it (`turnverdict.go:343`), so an empty string is otherwise indistinguishable from a hash; the bit preserves the fact. When set, `registeredWaits` (`turnverdict.go:565`) returns nothing before reading a row, and the verdict carries one diagnostic line saying registered waits were not read because the Stop supplied no session.
This is behaviorally identical to today for a sessionless Stop — the placeholder gate already yields no allowance — so B1 changes no observable decision. It closes the round-2 hazard that a future normalized-empty session could inherit a main's wait once B2 reads by main.

### B1.3 Tests member B1 owns

- `TestSessionAbsentReadsNoWait` in `metasystem/internal/goal/turnverdict_test.go`: a `TurnVerdict` with `SessionAbsent` set returns no registered-wait allowance and the one diagnostic line, and yields the same `ShouldBlock`/`BlockSource` as the identical scan carrying a session; no `WAITING` line appears. (round-1 finding 2; round-2 finding 4.)
- `hook-session-args` fixture leg in `metasystem/scripts/agents/runtime-hook-fixtures.sh`: for `start`, `stop` and `end`, a payload carrying `session_id` makes the hook pass `--runtime-session <session>` (and `--start-source` on start) to `up`; a payload without `session_id` makes the hook pass `--no-runtime-session` to `up` and `--session-absent` to `report turn-verdict`. (round-2 finding 4, Start/Stop/End coverage.)

---

## Member B2 — the gate is the only authority

**Ready to build at revision 4.** Owns the gate change and the two round-2 bypass closures. Depends on A (the field) and B1 (the hook writes it, and `SessionAbsent`). Its first unit is the fixture set in B2.4; its DONE carries the human-run seam question named there.

Files: `metasystem/internal/goal/turnverdict.go`, `metasystem/internal/goal/turnfacts.go`, `metasystem/internal/report/scan.go`, `metasystem/internal/run/waiter.go` (remove `LiveWaiter` from the decision path), `metasystem/cmd/metasystem/goal.go` (drop the prepend), `metasystem/docs/design/turn-verdict-delivery-contract.md:64`.

### B2.1 The gate change

`registeredWaitOwner(sessionID, mainID, options)` stops keying the owner on the announcement's `sessionId`. It proves `mainID` is the live checkout holder at the current claim epoch — the lease names `mainID`, the holder is alive at the lease's recorded birth (`turnverdict.go:623-627`), and exactly one announcement carries `mainId == mainID` with `pid`, `pidStartedAt`, `pidStartTicks`, `bootId` equal to the lease's (a by-main loop beside `currentSessionLifecycle`; the by-main lookup has precedent in `lease.CurrentHolder`, `verbs.go:283`). It returns the lineage. It does not derive the session from the announcement.
The authenticated session is the live Stop `sessionID` (real, set by the runtime's own hook, unforgeable by the agent, and non-empty because `SessionAbsent` is false). `registeredWaitEligible` keeps `row.Session == sessionID && row.RuntimeSession == sessionID` (already at line 671), now satisfied because A and B1 make the row carry the real session; every other check at lines 669-707 stays, including the waiter-liveness check whose `identity.Ref` already carries the Darwin microsecond token (`waiter.go:685-690,883`). The lineage check accepts the announcement's lineage or `mainID` (the recorded default for a row written before a lineage fill: `wait_verb.go:153-156`; filled later at `verbs.go:143-150`; lease following at `metasystem/internal/lease/claim.go:164`).
`currentSessionLifecycle` itself is untouched, so the human-stop marker path is unchanged.

Forgery is closed here, not by an unforgeable hook token: a forged association sets `runtimeSession` to S', so registration writes `row.Session = S'`; at a real Stop the hook presents the real session S; `S' == S` fails; the forged wait is refused. A forged association only sabotages the agent's own genuine wait.

### B2.2 The two bypass closures

- The unwatched-work rule (`turnverdict.go:1184-1216`) stops reading `!job.WaiterLive`/`!runFact.WaiterLive` for main-owned work and decides watched purely from the authenticated `waits` set (`watchesJob`, `watchesRun`). Human-owned work keeps a watched signal through B2.3.
- The Stop verdict's `WAITING` lines come only from `waits.lines()` (`turnverdict.go:294,1509`); `runReportTurnVerdict` no longer prepends `CurrentWaitingLines` (`goal.go:723-740`). `CurrentWaitingLines` stays for `session start` (`wait_verb.go:378`) and `goal next` (`goal.go:587`) as recovery orientation, which decides no Stop.
- The recovery `watch-job`/`watch-run` actions (`metasystem/internal/goal/turnfacts.go:266-283`) take the authenticated `waits` set (and the human authority of B2.3) from `freezeTurnVerdictFacts`, emitting an action only for a source not watched.
- `LiveWaiter` (`waiter.go:345`) is removed from the goal package's decision path. The `WaiterLive` fact may remain for display, but no decision reads it; the existing test `turnverdict_test.go:1237`, which fabricates `WaiterLive=true` on a `main-1` job, is rewritten as an authenticated-wait fixture.

### B2.3 Human-launched runs keep an authenticated watched signal (finding 2)

Human run records carry empty main coordinates by design (`metasystem/internal/run/verbs.go:31`), and `decideRuns` owns them when the caller `mainId` is empty (`turnverdict.go:1194`); their waiters key on `human:<uid>` (`metasystem/internal/run/run.go:602-604`, tested at `metasystem/internal/run/run_test.go:563`). Because `registeredWaitOwner` requires a non-empty main, a human wait has no `registeredWaitOwner`. B2 therefore keeps a separate authenticated human-wait check for empty-main work: read the row at the `human:<uid>` owner-digest path, require the run incarnation to match, and require the waiter alive at its *exact* recorded birth (the microsecond token, not the token-free `LiveWaiter`). This retains the watched signal for a human run and closes the same-second-reuse hole for it too.

### B2.4 Tests member B2 owns — its first unit

- `TestPendingWaitTurnVerdict` (extended) in `metasystem/internal/goal/turnverdict_test.go`: the m1b case — a placeholder-`sessionId` announcement whose `runtimeSession` A/B1 set to S, a row carrying S, a Stop presenting S — for job, run, attempt, landing has `ShouldBlock`, `BlockSource`, `IdleRefusal`, `CountSpent` and the display equal to the live-delegate control. Subcases assert: another logical session (`runtimeSession=B`, row session `A`, Stop `B`) refused with today's block source and no `WAITING` line (round-1 finding 1); a lineage filled after registration accepted, `another-lineage` refused (round-1 finding 3); the hostile table at `turnverdict_test.go:378-452` extended with a second announcement carrying the same main id and an announcement whose birth is not the lease's.
- `TestPendingWaitIdleBacklog` (extended) in `metasystem/internal/goal/turnverdict_idle_test.go`: equals the live-delegate control on `ShouldBlock`, `BlockSource`, `IdleRefusal`, `CountSpent`, `IdleBlocks`, `IdleBlockDigest`; gains the `CountSpent` assertion the current test omits (`turnverdict_idle_test.go:93-131`).
- `TestUnwatchedRunIgnoresRawLiveWaiter` in `metasystem/internal/goal/turnverdict_test.go`: a live session-A run row, Stop session B, run at the pinned generation and nonce, blocks `unwatched-work` with no `WAITING` line and no counter reset; it fails if `WaiterLive` still short-circuits the block. The same test reuses the waiter pid in one Darwin second and asserts the block still fires, proving the eligibility path carries the microsecond token. (round-2 finding 1; round-1 finding 4.)
- `TestForgedAssociationRefused` in `metasystem/internal/goal/turnverdict_test.go`: an announcement whose `runtimeSession` is S' while the Stop presents S, and a row carrying S', is refused; only a row whose session equals the live Stop session passes. (round-3 finding 1.)
- `TestHumanRunWatchedSignal` in `metasystem/internal/goal/turnverdict_test.go`: a watched human run (row at the `human:<uid>` owner digest, alive at its exact recorded birth) is not reported unwatched and draws no `watch-run` action; an unwatched human run blocks and draws the action; a human run whose waiter pid is reused in the same Darwin second is reported unwatched. (round-3 finding 2; round-1 finding 4.)
- `TestWatchedJobUsesAuthenticatedWait` in `metasystem/internal/goal/turnverdict_test.go`: the rewrite of the fabricated-`WaiterLive` case at `turnverdict_test.go:1237`; a main-owned job counts as watched only through the authenticated `waits` set, not a raw `WaiterLive=true`.
- `TestStopVerdictWaitingLinesFromGateOnly` in `metasystem/cmd/metasystem/goal_test.go`: `runReportTurnVerdict` prints WAITING lines only from `waits.lines()`; a refused row prints none, an accepted row prints exactly one, and no `CurrentWaitingLines` prepend appears; `session start` and `goal next` still print recovery orientation. (round-2 finding 2.)

DONE carries the one seam question the human-run fixture settles: whether the `human:<uid>` authenticated check lives in `internal/run` (beside a token-carrying `LiveWaiter`) or in the goal package. `TestHumanRunWatchedSignal` decides it; either placement passes the same assertions.

---

## Member B3 — no allowance without an authenticated-at-registration session

**Ready to build at revision 4.** Owns registration behavior when the association is absent, supersession of a wrong-session row, successor adoption, and the rule that the gate never writes a waiter row. Depends on A, B1, B2. Its first unit is the race test in B3.4; its DONE carries the two predicates named there.

Files: `metasystem/cmd/metasystem/wait_verb.go` (refuse an unassociated registration; register-fresh on succession), `metasystem/internal/run/waiter.go` (the supersession operation under the waiter lock; the successor-adoption rule in `ResumeWait`).

### B3.1 The invariant, and why there are no pending rows

A Stop allowance is granted only from a row whose session was authenticated at registration. Revision 3's association-pending row, bound to whatever session is current at the first Stop, is withdrawn: exact waiter birth proves the process, not its logical session, so a legacy-placeholder registration followed by `/clear` to B before the first Stop would bind to B, and a same-second Darwin holder-pid reuse would bind the old main's live child to a new session (finding 3). The gate is read-only over waiter rows (finding 6): it never binds, never writes, so it cannot race terminalization.

Registration reads A's association. In the supported flow the hook associates at SessionStart before any agent turn (B1.1), so registration always reads a real session and writes a real `row.Session`; there is no pending row and no binding. Registration refuses (exit 64) only when the main has no effective session — a shell `up` before its SessionStart (no agent turn happens there) or an unsupported runtime. A refusal leaves no row, so nothing blocks a later replacement, and the seat falls back to today's behavior: an unwatched job nags once and the seat may then stop, exactly as before this goal. That is not a regression and introduces no 24-hour trap, because the trap in revision 3 was the live pending row that refused replacement (`waiter.go:561`) — which no longer exists.

### B3.2 Supersession (finding 4)

A live row whose session differs from the current authenticated session is stale: after `/clear` the seat is a new session, and the old row's foreground waiter is orphaned. An authenticated registration for the current session may replace such a row, so the diligent seat recovers by re-watching. Supersession is an operation in `internal/run` under the waiter lock: it re-reads the row, and replaces it only when the caller proves the same owner main and a strictly newer authenticated session (the current association's session differs from the row's and is the live one), recording the new session and a fresh nonce. It never replaces a row of a different owner or a row whose session still matches.
Two consecutive Stops with a stale row and no re-watch both nag as unwatched (block-once fallback), never a false allowance; a re-watch supersedes and the next Stop counts it. Both are fixtures.

### B3.3 Successor adoption (finding 5)

`ResumeWait` rewrites main, session and runtime session for a proven successor (`wait_verb.go:185`, `waiter.go:1461-1466`). B3's rule: a successor never adopts a row that lacks an authenticated session for the successor's own current session. Resume of such a row is refused; the successor registers fresh under its own authenticated session, which supersedes the predecessor's row through B3.2. There is no half-written row with a session and a stale flag, because there is no flag. Holder-death takeover is a fixture.

### B3.4 Tests member B3 owns — its first unit

- `TestSupersedeVersusTerminalizeRace` in `metasystem/internal/run/waiter_test.go`: a supersession and a terminalization contend for the same row under the waiter lock; the row ends in exactly one consistent state, and a superseded-away wait is never observable as pending after it ended. This is the atomic-store question (round-3 finding 6), settled by the test rather than by prose; it is B3's first unit.
- `TestSupersedeStaleSessionRow` in `metasystem/internal/run/waiter_test.go`: an authenticated registration for the current session replaces a live row whose session differs and is older; it refuses to replace a row of a different owner or a row whose session still matches the current one. (round-3 finding 4.)
- `TestUnassociatedRegistrationRefusesNoRow` in `metasystem/cmd/metasystem/wait_verb_test.go`: registration under a main with no effective session refuses with exit 64 and leaves no row; a subsequent registration after association succeeds without a stale row blocking it. (round-3 findings 3 and 4.)
- `TestTwoConsecutiveStopsStaleRow` in `metasystem/internal/goal/turnverdict_test.go`: with a stale wrong-session row and no re-watch, two consecutive Stops both report the job unwatched and neither grants an allowance; after a superseding re-watch, the next Stop counts it. (round-3 finding 4, two-Stops.)
- `TestSuccessorRefusesWrongSessionRow` in `metasystem/internal/run/waiter_pair_test.go`: a successor main resuming a predecessor's row that lacks an authenticated session for the successor's own session is refused; the successor registers fresh, which supersedes the predecessor's row. Holder-death takeover. (round-3 finding 5.)

DONE carries the two predicates these tests fix: the exact supersession predicate (same owner, plus a strictly-newer authenticated session that is the current one) and whether `ResumeWait` forbids the transfer or fresh-registers — both decided by the takeover and race tests rather than in prose.

---

## Member C — child-shell watch counts at the next Stop

**Ready to build at revision 4.** Owns the end-to-end fixture and its bed wiring, including the un-stubbed hook-to-association leg. Depends on A, B1, B2, B3. One review.

Files and the one test it owns: `TestPendingWaitFromChildShell` in `metasystem/cmd/metasystem/wait_verb_test.go`; the `-run` pattern at `metasystem/scripts/agents/supervision-fixtures.sh:694`; the `tests` list at `metasystem/testing.json:42`; the bed paragraph at `metasystem/docs/design/turn-verdict-delivery-contract.md:89-96`. (round-1 finding 5: the reproduced-hypothesis fixture with a pending job on the correct main and both loop directions.)

Steps (installed-binary route, run by the `wait-stop-fake` bed leg when `METASYSTEM_WAIT_BINARY` is set, `supervision-fixtures.sh:691-698`):

1. Build the root as `pendingWaitVerdictCommandFixture` does (`wait_verb_test.go:539-623`) with the fake runtime; write no row; give the job record `status` `pending`, `mainId` the main, `operationId`, `round` 1 and `startedAt` (the scan counts only `pending`/`running`: `metasystem/internal/report/openwork.go:58`; a `pending` job needs its round and start for the wait to pin an incarnation, `metasystem/internal/dispatch/watch.go:102-104`, and for the gate to join it, `turnverdict.go:763-800`).
2. Associate the runtime session S through the real hook-to-`up` path, un-stubbed: run the installed `supervision-hook.sh fake start` (then `stop`) with payload `{"session_id":"S",...}` and `METASYSTEM_FAKE_AGENT_ANCESTOR_PID` set to the test pid (`metasystem/internal/census/ancestor_production.go:73-90`), letting the real engine's `up` run rather than the `already-healthy` stub (`wait_verb_test.go:689-690` wrapper replaced by a passthrough engine for this leg). Assert the announcement's `runtimeSession` is S. This leg is C's proof that the load-bearing path works from hook to association, not the lease API alone.
3. Register from a child shell of the test process: `/bin/bash -c 'exec "$0" job watch --root "$1" --job "$2" --caller-pid $$' <binary> <root> wait-stop-job`, started with `exec.Command`, output to files. `exec` keeps the shell's pid, so `--caller-pid $$` names the waiter, as m1b's command did. One goroutine owns `Cmd.Wait` and reports on a channel. Register cleanup first: write the job `completed`, `wait notify --job wait-stop-job`, wait on the channel with a Go deadline, kill on expiry, then wait on the channel again so the child is reaped. Poll the row until `state` `pending`; assert `Session` and `RuntimeSession` equal S, `mainId` the main, `pid` the child.
4. Control Stop, plain route, session S2 that was never associated: `shouldBlock` true, `blockSource` `unwatched-work`, no `WAITING` line (the m1b symptom, reproduced with a pending job on the correct main). Hook route with payload `session_id` S2: `hooks.log` gains `stop response decision=block`.
5. Stop, plain route, session S: `shouldBlock` false, `blockSource` null, `idleRefusal` false, `countSpent` false, exactly one `WAITING: registered wait <waitId> covers job wait-stop-job until <deadline>` line, no `unwatched`. Hook route, payload `session_id` S: no `"decision":"block"`; `hooks.log` gains `stop response decision=allow`; `stop-verdicts/S.txt` carries the one line.
6. Hostile Stops with the row live: session S2 (never associated) blocks `unwatched-work` with no `WAITING` line; a payload without `session_id` makes the hook pass `--session-absent` and blocks the same way; rows beside the live one under another main id and digest, and one whose waiter is a killed and reaped `sleep` child, are ignored; every allowed Stop lists exactly one `WAITING` line.
7. End the wait; assert the child exits 0 within the ceiling and the row is no longer pending.

Under bash 3.2 the child shell uses only `$$`, `exec` and positional parameters: no `$BASHPID`, no arrays, no `mapfile`, no `|&`. macOS has no `timeout`; every bound is Go's: the test deadline, the poll ceilings, the `Cmd.Wait` channel deadline and `Process.Kill`. Child output goes to files, never through a pipe into a reader that may stop early. The bed leg stays one `go test` call with the new name added to `-run` at `supervision-fixtures.sh:694`; `testing.json:42` adds the name to the `tests` list of `wait-stop-standard`.

Mutation proof: restoring the announcement-`sessionId` owner key, restoring `!WaiterLive` in `decideRuns` or `scanActions`, restoring the `CurrentWaitingLines` prepend, dropping `SessionAbsent`, dropping A's no-rollback rule, dropping supersession, or letting the gate write a waiter row must each fail its named fixture.

---

## Ordering, cutover and out of scope

Order: A, B1, B2, B3, C. All five are ready to build at revision 4. A and B1 are independent; A changes no Stop decision, B1 changes no observable decision. B2 needs A's field and B1's wiring and `SessionAbsent`; B3 needs B2's gate; C needs all. Each is one review.
B2 and B3 land together, or B3 immediately after B2, because B2's gate refuses a stale wrong-session row and B3's supersession is what lets the seat recover; B2's DONE names that pairing. This is a landing-order constraint, not an unsettled contract: both members' rules are fixed here and carried by their named tests.
Cutover is atomic, no rollback, already enforced: one checkout runs one enrolled engine, and a rebuilt engine that differs is refused with `ENROLLMENT_DRIFT` until re-armed (`metasystem/docs/orchestration.md:310`), so an old engine and a new record never coexist. A adds the fields to the strict contract; no mixed-version support is added.
Every seat rebuilds its enrolled engine and runs `metasystem up`; the hook ships with B1, reinstalled through runtime setup. Rows written before B2/B3 land all carry the pre-change session equal to the pre-change announcement, so nothing needs migration.

Out of scope, a follow-on goal, unchanged here:

- `session start` refuses with 64 when `holder.SessionId` is not the hook's session (`wait_verb.go:357-387,374`); `CurrentHolder` returns the first announced session (`verbs.go:283-285`). Moving it to the effective session would move the failure to the stop marker's lifecycle, so this page leaves both alone. `TestWaitSessionStartPrintsPendingRows:531-536` asserts the 64 for another session.
- `session stop` writes and reads its marker by session (`metasystem/cmd/metasystem/session_stop.go:83-90`, `sessionstop.go:437-446`), matching the first announced session.
- The lease's own holder identity is at second precision on Darwin (`metasystem/internal/lease/claim.go:348-355`; the announcement/lease chain carries no microsecond token: `metasystem/internal/census/verbs.go:63-64`, `metasystem/internal/lease/identity.go:49-50`, `metasystem/internal/lease/lease.go:29-43`, `sessionstop.go:61-84`). Carrying the microsecond token through the announcement and lease closes the lease protocol's own same-second holder-pid reuse. This goal does not depend on it: the gate authenticates by the live Stop session, so a reused-pid process presents its own session and a stale row fails the comparison.

## Runtime mapping

- Claude Code: SessionStart, Stop and SessionEnd payloads carry `session_id`. Evidence: the SessionStart handler requires it (`metasystem/internal/adapter/claude.go:278-280`); the shipped hook and fixtures read it (`supervision-hook.sh:539`, `wait_verb_test.go:707`). SessionStart also carries `source` (`startup|resume|clear|compact`) for `--start-source`. Supported.
- Codex: the event stream carries `thread_id`/`session_id`/`id` (`metasystem/internal/adapter/codex.go:23`); whether the coordinator hook payload carries `session_id` is unobserved (`metasystem/docs/design/turn-verdict-delivery-contract.md:104`). Probe: capture one live Codex Stop and SessionStart payload and confirm a non-empty `session_id`. Until observed, a Codex Stop without `session_id` takes `--session-absent` and falls back to block-once — the safe default.
- Devin: the Stop envelope is unknown (`turn-verdict-delivery-contract.md:104`); the adapter reading `session_id` from records (`metasystem/internal/adapter/devin.go:103,167`) is not proof of the hook payload. Probe: the same capture. Until observed, Devin is out of the association contract and takes `--session-absent`.

A runtime is associated only through its own hook payload's `session_id`; a runtime that supplies none is never associated and never gains a registered-wait allowance — the safe default, not a regression.

## Revision record

This is the final prose revision under R-97-m1e with D81: after three critique rounds, every standing finding from rounds 1 to 3 is either closed in the text (quoted below) or carried as a named test in the unit that owns its rule, and no further critique round runs. From here a code read of each unit carries the check.

Round 3 (the rounds are cumulative; earlier findings that were already closed are marked so):

| Finding | Disposition |
| --- | --- |
| R3-1, the hook-only association is forgeable | Closed in text. B2.1: "a forged association sets `runtimeSession` to S', so registration writes `row.Session = S'`; at a real Stop the hook presents the real session S; `S' == S` fails; the forged wait is refused." A.1: "The record is a registration-time hint, not an authority ... a wrong or forged `runtimeSession` therefore cannot make a wait pass." Also carried as test `TestForgedAssociationRefused` in unit B2 and `TestAnnounceNeverAssociates` in unit A. |
| R3-2, B2 removes the only watched signal for human-launched runs | Closed in text. B2.3: "B2 keeps a separate authenticated human-wait check for empty-main work: read the row at the `human:<uid>` owner-digest path, require the run incarnation to match, and require the waiter alive at its exact recorded birth." Carried as test `TestHumanRunWatchedSignal` in unit B2. |
| R3-3, a pending row has no evidence tying it to its birth session | Closed in text by withdrawing pending rows. B3.1: "Revision 3's association-pending row ... is withdrawn ... A Stop allowance is granted only from a row whose session was authenticated at registration." Carried as test `TestUnassociatedRegistrationRefusesNoRow` in unit B3. |
| R3-4, the one-turn refusal is still unbounded | Carried as tests. B3.1 removes the trap ("leaves no row, so nothing blocks a later replacement ... introduces no 24-hour trap"); the bound is proven by `TestTwoConsecutiveStopsStaleRow` in unit B3 (two Stops, neither allows) and `TestSupersedeStaleSessionRow` in unit B3 (recovery by supersession). |
| R3-5, successor-main resume has no B3 transition rule | Carried as test. B3.3 states the rule ("a successor never adopts a row that lacks an authenticated session for the successor's own current session ... registers fresh"); proven by `TestSuccessorRefusesWrongSessionRow` in unit B3. |
| R3-6, gate-time binding lacks an atomic waiter-store operation | Closed in text by removing gate writes (B3.1: "The gate is read-only over waiter rows ... it never binds, never writes, so it cannot race terminalization"), and the one write introduced (supersession) is carried as test `TestSupersedeVersusTerminalizeRace` in unit B3, its first unit. |

Round 2:

| Finding | Disposition |
| --- | --- |
| R2-1, the raw `LiveWaiter` decision path | Closed in text (B2.2: `LiveWaiter` removed from the decision path). Carried as test `TestUnwatchedRunIgnoresRawLiveWaiter` in unit B2 (the run leak and the same-second reuse). |
| R2-2, Stop output bypasses the gate via `CurrentWaitingLines` | Closed in text (B2.2: WAITING lines "come only from `waits.lines()`"). Carried as test `TestStopVerdictWaitingLinesFromGateOnly` in unit B2. |
| R2-3, association is not authenticated or monotonic | Closed in text (A.2: one hook-event writer; fill/refresh/replace-only-on-start/no-rollback; A.3 Retire matches current). Carried as tests `TestAssociateSessionTransitions` and `TestAssociateSessionRetireMatchesCurrent` in unit A. |
| R2-4, missing-session handling and runtime mapping | Closed in text (A.2 ignores env; B1.1 wires Start/Stop/End; A.3 grammar migration; Runtime mapping section). Carried as tests `TestUpAssociationIgnoresEnv`, `TestLegacyPlaceholderGrammarUnassociated` in unit A, `TestSessionAbsentReadsNoWait` and the `hook-session-args` leg in unit B1. |
| R2-5, the one-turn refusal | Superseded by R3-4; carried by the same B3 tests. |

Round 1 (all closed or carried; kept for the cumulative record):

| Finding | Disposition |
| --- | --- |
| R1-1, another logical session on the same main | Closed in text (session authentication at the gate). Carried as a subcase of `TestPendingWaitTurnVerdict` in unit B2. |
| R1-2, the empty-session guard never fires | Closed in text (B1.2 `SessionAbsent`). Carried as test `TestSessionAbsentReadsNoWait` in unit B1. |
| R1-3, a real wait refused after a lineage fill | Closed in text (B2.1 lineage alias). Carried as a subcase of `TestPendingWaitTurnVerdict` in unit B2. |
| R1-4, PID birth is not exact on macOS | Carried as tests: the microsecond token on the eligibility check (`TestUnwatchedRunIgnoresRawLiveWaiter`, unit B2) and the human path (`TestHumanRunWatchedSignal`, unit B2); the lease's own same-second holder reuse is Out of scope with the reason it does not gate here. |
| R1-5, the fixture does not prove the cause or both loop directions | Closed in text (section 1 calls the cause a reproduced hypothesis). Carried as test `TestPendingWaitFromChildShell` in unit C, with a `pending` job on the correct main and the un-stubbed hook-to-association leg. |

## Open for Wido

One owner decision, and only a scope choice: whether to fold the lease protocol's own same-second Darwin holder-pid reuse (carrying the microsecond token through the announcement and lease, Out of scope above) into this goal, or leave it a separate goal. Consequence of folding it: one more cross-cutting member touching `census`, `lease` and the goal structs, landing before B2. Consequence of leaving it: this goal is unaffected, because the gate authenticates by the live Stop session, so a reused-pid process presents its own session and a stale row fails the comparison; the lease protocol's own identity hardening waits for its own goal.
No other decision blocks the build. All five members are ready to build at revision 4, in order A, B1, B2, B3, C.
