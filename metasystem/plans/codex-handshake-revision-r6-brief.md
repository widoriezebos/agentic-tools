Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Revision 6 of metasystem/plans/codex-handshake-design.md (revision 5
landed, in your worktree). Revision 5 reported three points to the
orchestrator; all three are decided below. Fold them, run the consistency
pass over D2.5, sections 4, 5, 6 and 7, and stop. Edit in place;
diffBoundary is that one file. Keep it under eight minutes; no re-reading
beyond the lines named.

# The decisions (orchestrator m0b, 2026-09-02 17:25Z)

1. Fixture home (section 7's open point): the Devin critic-shaped row
   lives in metasystem/scripts/agents/adapter-deadline-fixtures.sh, the
   one harness that sources the real runtime-common.sh and drives the real
   `fail_pending` under the stub dispatch that logs the CAS argv. The row
   calls `fail_pending handshake_missing_session_id handshake <usage> 0`
   under a `{"role":"design-critic"}` record and asserts the logged patch
   carries `error == protocol_error`, `phase == handshake`,
   `handshakeExitStatus == 0`; a sibling row under `{"role":"implementer"}`
   asserts the class is unchanged and the field present. No Devin adapter
   fixture is created. Section 7 then reads "None" plus the two implementer
   facts.
2. The table pair: CONFIRMED. `handshake_missing_session_id handshake`
   folds to `protocol_error handshake` for the three critic roles; it is a
   transport shape (a runtime that spoke without naming a session), not a
   critique, exactly the rationale the existing table states. Keep the
   reasoning D2.5 already gives.
3. The third bypass: fold `finish_running` too. `finish_running`
   (metasystem/scripts/agents/adapters/runtime-common.sh lines 186-194)
   calls the same `adapter critic-fold` verb before `write_patch` when its
   target is `failed`, so devin.sh line 720 (`finish_running failed
   empty_reply delivery`, the spent-repair case after a session) and every
   other direct `finish_running failed` in any adapter folds for critics
   from the one table; `completed` targets never fold. State that
   `complete_from_cli`'s own `finish` verdicts arrive already folded and
   the verb is idempotent on `protocol_error`, so nothing double-folds.
   Add the Go row for the `empty_reply delivery` pair if the table test
   lacks it, and one harness row: `finish_running failed empty_reply
   delivery <usage>` under a critic record logs `protocol_error`. Section
   4: the runtime-common.sh row names both functions; section 6: the
   sentence on what devin.sh inherits names the running-side fold as well.

Bump the header to revision 6 naming the round. R-31: no benchmarks.

# Constraints

Wall-clock budget: 8 minutes. Design only; edit nothing but the design
file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the design file.

# Gap Rule

stop and report a gap; never fill it silently.
