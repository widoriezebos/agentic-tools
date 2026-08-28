# F4 — Closing the orphan window (REVISION 5, after critique round 4)

Status: r5. Round-1 critique (codex gpt-5.6-sol, xhigh, CLI fallback —
this checkout's supervision dispatch is blocked on subdirectory conf
resolution): VERDICT REVISE, ten material findings, all folded. The
biggest (finding 9) surfaced a structurally simpler option the draft
missed; it is now the chosen design.

## The window, precisely (unchanged from r1)

Kill authority over a job's process group lives in the DISPATCHER
("waiter") and, per turn, the mission runner. The standing reaper may
not kill (it acts only on a provably dead custodian; since D31 it
emits REAP-DECLINED for the running∧CapExpired∧custodian-alive state).
When waiter and runner both die, a live group has no enforcer until
mission end.

## CHOSEN: Option C — the custodian enforces its own deadlines

Round 1's finding 9, verified against the launch path: the adapter
supervisor is launched via `supervise launch-detached` into ITS OWN
session and process group (dispatch.sh launch_adapter). It already
survives waiter death by construction, already spawns and custodies
the runtime CLI child, and already owns the kill-capable relationship
to it (register_cli_custody / terminate_cli_child). The record's
custodian IS this process. So the enforcer is placed where the
liveness already is:

- The adapter supervisor's wait loop enforces the record's OWN stamped
  deadlines: `handshakeDeadline` (finding 5 — closes the handshake
  orphan: a pending or session-less running job whose deadline passed
  fails as handshake_timeout, session absence re-checked immediately
  before the terminal CAS, exactly the waiter's stand-down rule) and
  `capDeadline` (budget expiry: `running`, cap expired, regardless of
  session — finding 1's separate state machines, honored).
- Enforcement sequence inside the supervisor: TERM the CLI child's
  subtree → grace → re-check → KILL (its existing terminate path), then
  CAS the terminal verdict through the same fail_pending/finish_running
  authority path every adapter failure already rides. The supervisor
  never signals outside its own child subtree, so group_owned proof
  questions do not arise for it (finding 6's table still governs every
  OUTSIDE signaler, unchanged).
- Overshoot bound (finding 8): the wait loop's poll interval (already
  sub-second) bounds cap overshoot; no sweep cadence is involved.

Why this beats Options A and B on the critique's own findings:
- No adoption, so no terminating-claim protocol (finding 2), no
  per-job lifecycle-lock/CAS duality for a new actor (finding 3 — the
  existing waiter-vs-supervisor race already serializes on the record
  CAS: first terminal verdict wins, the loser's signal hits a dead
  group behind ownership proof), no new authority class or component
  identity scheme (finding 4), no write-once waiter identity fields
  (finding 7), and no new standing component.
- The no-kill-authority rule is untouched IN FACT, not just in name:
  the only killers remain processes that own what they kill.

## Residual window, accepted and stated

If the CUSTODIAN itself dies leaving tagged grandchildren, the reaper
already acts on the record (provably dead custodian — existing rule),
and group survivors are census-visible UNTRACKED strays until mission
end. That residual exists today, is strictly narrower than F4's
window (it requires the detached leader to die while its children
survive), and closing it would require exactly the adoption machinery
findings 2-4 price out. Recorded as a known residual, reopened if the
census ever shows it recurring.

## Verification (finding 10)

- Injection seams, not just the fake identity file: deadline clocks
  and CLI-child liveness injected in adapter fixture legs.
- Suite legs: cap expiry kills a live CLI and lands timeout/budget-cap;
  a handshake published before the timeout branch fires produces ZERO
  termination signals and NO terminal CAS attempt (round 3.1 — the
  single-writer stand-down is before-any-signal, not a CAS race);
  waiter and supervisor both enforcing race to one terminal verdict;
  supervisor crash mid-enforcement leaves a dead custodian the
  standing reaper finalizes; a CLI survivor after KILL leaves the
  record nonterminal.
- Events: the supervisor's enforcement emits through the existing
  adapter event path (registry-listed), AFTER the record CAS wins
  (record is the authority, recorder the witness).

## Round-2 findings, folded

1. PRE-ARMING WINDOW: deadline enforcement runs from the supervisor's
   FIRST instruction, including while blocked on the start gate — the
   gate wait itself gets a bounded local startup deadline (the existing
   wait_for_start_gate cap) independent of the record, so a dispatcher
   that dies on either side of the launch CAS leaves a custodian that
   still times out and terminates itself. Suite legs crash the
   dispatcher before and after the launch CAS.
2. HANDSHAKE STAND-DOWN IS SINGLE-WRITER, STATED AS FACT: sessionId is
   published by record_handshake INSIDE the same supervisor process
   that runs the wait loop, so handshake-vs-timeout needs no
   cross-process serialization — the supervisor's own control flow
   checks its in-process handshake state immediately before signaling,
   which IS the stand-down, deterministically. The terminal transition
   accepts both pending and running-without-session (fail_pending
   covers both today; verified at implementation or extended). The
   WAITER's outer record-read stand-down stays as-is.
3. DEATH IS PROVEN, NOT INFERRED: after KILL the supervisor reaps its
   CLI child (wait — it is the parent) and proves the child's process
   subtree absent before the terminal CAS; if the proof fails the
   record stays nonterminal (running), the failure is emitted, and the
   census/reaper backstop keeps the case visible. A leg proves
   survival-after-KILL leaves the record nonterminal.
4. THE BOUND, COMPLETE, AND THE INERT-CUSTODIAN RESIDUAL: maximum
   live-work overshoot = poll interval + TERM grace (2s) + kill-verify
   window; stated in the enforcement comment and asserted loosely in
   the fixture leg. A live-but-wedged custodian has NO hard bound —
   accepted explicitly as a residual in the same class as today's
   wedged waiter, priced against a fenced progress-heartbeat lease
   that would be its own design; reopened if the census shows it.

## Round-3 findings, folded

1. The stand-down is BEFORE ANY SIGNAL: the timeout branch first reads
   its own in-process handshake state; if a session was recorded, the
   branch exits without signaling and without a terminal CAS. The
   fixture leg asserts zero signals (folded into Verification above).
2. THE KILL DOMAIN IS THE SUPERVISOR'S OWN GROUP, MINUS ITSELF
   (revised in round 4): a separate CLI group would escape the
   waiter's single-group wind-down and recreate the window when the
   waiter kills the supervisor first — so NO second group is created
   and the waiter's outer contract is untouched in fact. The durable
   containment boundary is the supervisor's own process group:
   process-group membership survives reparenting, so orphaned
   grandchildren remain enumerable (AllPids + Getpgid, the census's
   own primitives) without ancestry evidence. The insider's
   enforcement signals each enumerated member EXCEPT itself
   (TERM sweep → grace → re-enumerate → KILL sweep), and the death
   proof is "no member of my group except me remains", re-checked
   until proven or bounded out; an unproven domain leaves the record
   nonterminal.
3. DEADLINES ARE CACHED, FAIL-CLOSED: handshakeDeadline and
   capDeadline are immutable once stamped; the supervisor reads them
   ONCE after the launch CAS (retry briefly, bounded), caches them
   in-process, and if they cannot be obtained within the bound it
   terminates its child and fails the turn rather than waiting
   unbounded on an unreadable record.

## What changes where (implementation sketch)

1. runtime-common.sh wait loop (all four adapters inherit): deadline
   checks against the record's handshakeDeadline/capDeadline; on
   expiry, terminate CLI subtree, then fail_pending handshake_timeout
   or finish_running timeout budget-cap (existing verbs; codes align
   with the reaper's and waiter's spellings so one record reads one
   way — finding 1).
2. Waiter and reaper: UNCHANGED (the waiter remains a second, outer
   enforcer when alive; D31's decline line becomes rare and is the
   regression signal that the inner enforcer is missing).
3. Fixture support: fake adapter grows deadline-expiry behaviors so
   the suite drives both legs without real time.
