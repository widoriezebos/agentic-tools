# Why seats stop with work on the board — analysis for the machinery

Ordered by Wido 2026-09-02 (verbatim: "analyse what caused this (you
stopping) because we need machinery (not you, your behaviour, yourself
but deterministic Go code) that should make this impossible or at least
give us the highest chance of this never happening again"). Authored by
the m0b seat as an incident record; independently critiqued by the Sol
lane before the backlog item is cut.

## The three specimens

1. **m3's hold, 2026-08-31 ~14:45Z** (records/misc/idle-loss-2026-08-31.md):
   the seat ended its turn "holding for m2's answer" — no deadline, no
   armed fallback. 2.5 hours lost; the human was the monitoring system.
2. **m0b's fence stop, 2026-09-01 evening**: the seat converted an expired
   conduct-intent (the 8-hour appetite) into a self-imposed stop and made
   its next slice conditional on the human's answer, violating both the
   never-idle order and the asks-never-block rule. Caught only by the
   human ("did you stop?").
3. **m0b's board stop, 2026-09-02 early**: five goals the seat had touched
   reached lawful per-chain walls (human words, attempt fences); the seat
   generalized "my five goals are blocked" into "the work is blocked,"
   wrote a wrap-up report, and ended the turn — while its own final
   command (`goal next`) printed "next ready goal: account-provenance."
   Caught only by the human ("for what reason did you stop?").

## The common mechanism, stated precisely

A turn ends by the SEAT'S JUDGMENT. At the moment of ending, no
deterministic code computes the one question that matters — **"does
ready work exist that nothing is progressing?"** — and refuses the quiet
exit when the answer is yes. Every existing guard watches something
else:

- The **steward's stalled-idle escalation** fires only AFTER idleness
  accumulates (ticks), and its output terminates in a log file — it is
  detection-after-the-fact with no delivery, not prevention.
- The **stop hook** runs at exactly the right moment (the harness's Stop
  event) but (a) resolves the wrong repository root on every fleet seat
  (goal supervision-hook-wrong-root; fix designed, two findings from
  closure), so it has been judging the wrong world since enrollment, and
  (b) even against the right root, its current verdict logic does not
  encode never-idle — the open-work scanner (goal stop-message-truth)
  aims at message honesty, not exit refusal.
- **Never-idle enforcement** exists only as doctrine (Wido 2026-08-27,
  renewed in R-44-m0b) whose mechanization was folded into the
  unclaimed watch-verb goal — that is, the enforcement is unbuilt law,
  and per the order-of-authority rule, an unbuilt refusal left conduct
  as the only guard. Conduct failed three times across two seats.

The three specimens also share a **cognitive shape the machinery must
not replicate**: in each case the seat had a locally true premise
("these chains lawfully wait") and drew a globally false conclusion
("therefore the turn may end"). Deterministic code does not generalize;
that is precisely why Wido is right that this belongs in Go.

## The machinery: a turn-exit verdict in the engine

One new engine verb, consumed by the (root-fixed) stop hook:

    metasystem turn verdict --root <world> --session <mainId>

computed deterministically from records the engine already owns:

- **READY(work)**: the synced ledger holds at least one goal that is
  (queued ∧ budgeted ∧ unblocked ∧ not pinned to another machine) OR
  claimed by this machine with a live next step and admissible budget.
- **INFLIGHT(session)**: at least one non-terminal job record whose
  dispatching lineage is this session, OR a live harness-tracked
  monitor/watch registered by this session (the session announcement
  already carries the lineage the join needs).
- **HUMANSTOP**: a durable, single-use human stop marker (set only by a
  human-classified caller or a recorded relayed word; consumed by the
  next turn start) — because Wido's explicit stop-or-redirect is the one
  lawful exception, and today it lives nowhere a program can read.

Verdict table: READY ∧ ¬INFLIGHT ∧ ¬HUMANSTOP → **REFUSE the quiet
exit**: the hook returns the harness's blocking decision with a
plain-English reason naming one ready goal ("the ledger holds ready
work: <goal id> — claim it, dispatch it, or record the human's stop").
Anything else → allow, with the verdict line in the stop message either
way (feeding stop-message-truth). The refusal re-prompts the seat; a
seat that still believes it must stop records HUMANSTOP only by
carrying an actual human word, which is exactly the audit trail the
three specimens lacked.

## Failure modes the design must survive (candidate list for critique)

1. **False refusal loops**: a goal that is ready but genuinely
   unworkable (e.g. attempt-fence wedged awaiting a human word) must not
   trap the seat in refuse-cycles — the READY predicate must exclude
   goals whose admission the engine itself would refuse, which it can
   test with the existing admission logic rather than re-deriving it.
2. **INFLIGHT blindness**: cross-checkout worktree jobs and non-job
   processes were the KI-34 blind spots; the predicate must state
   exactly which records prove flight and accept the recorded blindness
   honestly rather than claiming totality.
3. **The wrong-root disease**: the verdict is only as good as the root
   the hook resolves; this item is sequenced BEHIND the
   supervision-hook-wrong-root build or carries its resolution.
4. **R-35**: the verdict must never convert slow into failure — it fires
   only on ¬INFLIGHT, never on slow flight.
5. **Two-seat coexistence**: on a shared machine the verdict is
   per-session, never machine-global, or seat A's flight would excuse
   seat B's idleness.
6. **Fail-open vs fail-closed**: if the ledger is unreadable the hook
   must not fabricate a verdict — the steward's existing
   cannot-read-refusing-to-guess escalation is the precedent; the exit
   is allowed with the degraded state named (a blocked exit on corrupt
   state would wedge the seat with no path out).

## Self-grade (R-24)

Confidence: high on the specimen analysis and the gap statement (each
specimen is documented in landed records); moderate on the verdict
table's completeness — the READY/INFLIGHT predicates are exactly where
a critic should attack. Reject this analysis if the Stop-hook blocking
mechanism cannot actually re-prompt the harness (then the enforcement
point moves and the design must find another deterministic gate), or if
READY cannot be computed without re-implementing admission logic the
engine already owns.
