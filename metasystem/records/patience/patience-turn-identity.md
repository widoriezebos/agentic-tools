# Patience satellite 1: turn identity

Owner: main session (claude). Status: ACCEPTED FOR IMPLEMENTATION —
rounds 1 and 2 adjudicated (7/7 and 4/4 accepted; dispositions r1/r2
committed). Round 2 challenged no structure — all four findings were
seams of round 1's own amendments — the documented diminishing-returns
stop signal: the loop stops by judgment, rounds retained verbatim,
implementation is the next source of truth.
Program: `plans/stop-loss-satellites.md` satellite 1; concepts in
`docs/patience.md`; ground truth in `docs/design/mission-cycle-sequence.md`
(built for this purpose — cite it, not assumptions). Inherited findings:
parent r1[6][7][8], r3[9] (`plans/stop-loss-dispositions-r1.md`, the r3
return); map surprises 5 (post-hoc, double-checked session identity with a
stale-announcement crash path) and 2 (failed turns never measure; the
capped outcome is overwritten).

# Intent

An honest host must never be booked as a valueless cycle. Today the
session identity is announced before launch (from the PREVIOUS concluded
turn's result envelope), discovered after the CLI exits, and checked twice
— the adapter hard-fails the turn on rotation, and adjudication separately
requires the return to echo the announcement. Both checks punish truth
whenever the harness's announcement is wrong or stale, and a rejected turn
is booked `no-progress, unmeasurable` without ever draining or measuring
the tree it committed. Every one of those bookings is FALSE STALL: it
debits patience for work that happened.

# Non-goals

- No change to what counts as progress or to patience floors (satellite 4).
- No change to the consecutive-host-failure breaker's semantics — a real
  protocol violation still feeds it.
- No new session sources: only artifacts the harness already produces
  (the session-established signal, the adapter's terminal result
  envelope) — never the return's own claim — establish observed identity.

# Design

## T1. Two recorded identities; announced is a hint, never an authority

`turn.json` records both:
- `announcedSession`: what the prompt's `Host-Session` header said (null
  when it said `none`). Written at assembly (map S-step where turn.json
  is created), never changed. It derives from the previous concluded
  turn and CAN be stale after the map's ledger/state crash window — the
  design treats it accordingly.
- `observedSession`: stamped harness-side from the earliest trusted
  source THE RUNTIME ACTUALLY PRODUCES, per its capability snapshot: the
  session-established signal where `sessionEstablishedSignal` is declared
  (today: the claude delegate adapter), else the adapter's terminal
  result envelope (`sessionId`) — the universal source for host turns,
  which emit no launch signal. Absent only when neither names a session.
  Both sources are the harness's own artifacts.
The legacy `hostSession` field keeps its value (equal to
`announcedSession`) until the fixtures migrate.

## T2. One adjudication rule, honesty-proof

A return's `identity.sessionId` is accepted when it equals
`announcedSession` OR `observedSession`. Echoing a stale announcement and
reporting the true session are both correct — the host cannot lose by
telling the truth, and cannot lose by trusting the prompt. When it
matches neither AND an observed identity exists, that is a host protocol
violation: the turn fails normally and feeds the breaker. When it matches
neither and NO observed identity exists (no signal, no envelope session):
fail closed on APPLICATION, fail open on BLAME — the ONE application
rule below governs (no witness ⇒ the return is not accepted ⇒ never
applied), the turn takes the T4 path, the mismatch-with-no-witness is
recorded as its own annotation, and the turn does NOT feed the breaker
(no witness convicts either side). The return schema and the turn prompt
document the echo rule.

THE ONE APPLICATION RULE (referenced by T2 and T4, stated once): a
return's state mutations (stream transitions, asks, waiting list) are
applied ONLY when the return is accepted. Measurement effects (the
cycle's classification, gatePassed, and completion) always conclude,
from the measured tree, whatever the return's fate. There is no third
case.

## T3. The adapter stops sentencing; the runner judges

Exit code 6 today conflates two duties. They separate:
- ROTATION (the envelope names a session different from the resumed
  one): the hard-fail is retired — the adapter reports what it observed
  in its result envelope; it is a witness, not a judge, and the runner's
  adjudication (T2) is the single place a session verdict is reached.
- MISSING SESSION (the envelope carries no session at all): unchanged —
  that remains the adapter's genuine fault signal exactly as today, and
  on the runner side it simply yields an absent `observedSession`
  feeding T2's no-witness branch. The branch is defined, not dropped.

## T4. Rejected turns still drain and still measure

A turn rejected for identity (or any return-validity fault) no longer
bypasses the cycle's remaining duties. The failed-turn path keeps the
map's binding order: drain jobs FIRST (parent r3[9] — measurement must
never race live delegates), then run measurement over the committed tree,
then conclude with BOTH facts: the classification the measurement earned
and the fault that rejected the return. Grammar (the critical round-1
finding): the classification line is NOT touched — the fault and the cap
are separate annotation lines inside the cycle block (`- Return:
rejected:<reason>`, `- Outcome: capped`), which every parser tolerates
today (the strict one-classification-per-block rule counts only
Classification lines; proven by the reset-line regression test). The
turn's state-log entry carries the same measurement outcome and fault,
so ledger and turn log tell one story. Precedence when the measured gate
PASSES on a rejected-return cycle: the measurement is runner-run truth —
the cycle classifies as its movement, gatePassed stands, and the mission
completes on the measured product with the envelope fault recorded; a
broken envelope does not un-build the product. A turn whose work moved
the gate classifies as the movement it earned. When draining stalls, the
drain-deadline rules (satellite 2) own the outcome; until then the
existing drain behavior applies unchanged on this path exactly as on the
success path.

Conclusion inputs on a rejected return (round-2 PTI-R2-004): conclude
receives an EMPTY verdict — no accepted entries, no asks, no stream
transitions — plus the measurement. Streams keep the states they had at
turn start; open asks stay open. Completion on measured gatePassed is
the runner's own transition and is legal from any stream configuration:
the streams' recorded states are untouched by it, and the completion
ledger entry names the envelope fault beside the result.

Breaker transition (round-2 PTI-R2-003), decoupled from classification:
a WITNESSED protocol violation is a failed turn — consecutiveFailures
increments and the host-failure park fires on the second consecutive
one, exactly as today — while the CYCLE still books the classification
its measurement earned. The two ledgers tell different stories because
they answer different questions (host health vs mission progress). The
no-witness case increments nothing. A turn whose return was accepted
resets consecutiveFailures as today.

## T4b. Observed identity propagates forward

The next turn's `Host-Session` announcement derives from the last
concluded turn's `observedSession` when one exists (whatever that turn's
return's fate), else its announced value as today. A rejected turn with
a trusted witness still corrects the future — the stale-announcement
window closes instead of compounding.

## T5. The capped outcome survives

`turn.json` keeps `outcome=capped` when the cap fired (today it is
overwritten to `failed`); the ledger entry names it. Capped turns take
the same T4 path — drain, measure, conclude — so a cap that landed real
work registers as the progress it made. The turn-cap enforcement itself
is untouched.

# Invariants

1. A host that reports the session it actually has is never failed for
   identity, whatever the announcement said.
2. A session verdict exists in exactly one place (adjudication); the
   adapter never fails a turn over rotation.
3. Every concluded cycle with a measurable tree was drained, then
   measured — regardless of the return's fate.
4. `outcome=capped` in turn.json means the cap fired; it is never
   rewritten to `failed`.
5. No identity source outside the harness's own artifacts is ever used
   to establish `observedSession`.

# Failure behavior

- Handshake signal and envelope both absent → `observedSession` absent;
  T2's no-witness acceptance applies and the ledger records the gap.
- Measurement fails on the T4 path → the cycle books `no-progress,
  unmeasurable:<detail>` exactly as today — T4 adds duties, never
  invents measurements.
- Crash inside T4 between drain and measure or measure and conclude →
  the same windows as the success path; the #17 heal covers the
  reserve/append gap; nothing new to recover.

# Tests

- Adjudication matrix: echo-announced accepted; report-observed accepted;
  stale announcement + truthful return accepted via observed; neither
  match with observed present → failed turn feeding the breaker; neither
  match with no witness → not applied, not blamed: T4 path runs, the
  annotation lands, the breaker is not fed.
- Adapter: rotation reports instead of failing (fixture: rotated session
  in the envelope; runner adjudicates); handshake timeout still fails.
- T4: rejected-identity turn with committed work drains, measures, and
  classifies the movement; ledger line carries both facts; a genuinely
  unmeasurable tree still books unmeasurable.
- T5: capped turn keeps its outcome end to end and measures its work.
- Stale-announcement crash path: reproduce the map's window (turn log
  dropped), next announcement stale, truthful host passes.
- Legacy: turn.json vintages without the new fields adjudicate as today.

# Migration

turn.json gains `announcedSession`/`observedSession` (additive;
`hostSession` retained). The ledger gains two ANNOTATION LINE kinds
inside cycle blocks — `- Return: rejected:<reason>` and
`- Outcome: capped` — never touching the classification line. Every
cycle-block consumer is named and handled: `ParseLedger` and
`ParseLedgerEvents` tolerate and expose them as annotations;
`promptLedgerRecords` counts only Classification lines (pinned by the
reset-line regression test — extend it to both new kinds); the stop-loss
replay in internal/missionrunner/stoploss.go reads ONLY classification,
best, and reset lines — annotations are audit trail, never fuse input,
stated as an invariant of the replay; `assert-stop-loss.sh` serves
non-mission ledgers that never contain these lines. Adapter result
envelopes already carry `sessionId`; no envelope change. The adapter
rotation change is behavioral in the host scripts, and the schema and
prompt documentation gain the echo rule.
