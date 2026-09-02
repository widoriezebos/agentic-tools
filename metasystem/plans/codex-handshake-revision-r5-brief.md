Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Revision 5 of metasystem/plans/codex-handshake-design.md (revision 4
landed, in your worktree). Sol's round-3 register is
metasystem/records/misc/codex-handshake-critique-r3.md: ONE material
finding, CHS-R3-EXIT-01 (high). Everything else held. Edit in place;
diffBoundary is that one file; mark the closure with the id. One fold,
then the consistency pass over D2.5, section 4, section 5 and section 6.
Keep it under ten minutes.

# The fold

CHS-R3-EXIT-01. metasystem/scripts/agents/adapters/devin.sh lines 650-662:
on a status-zero exit with no session and empty stdout, the adapter calls
`fail_pending` directly (`handshake_missing_session_id handshake` when the
presence scan finds candidates, else `empty_reply delivery`), bypassing
`complete_from_cli`, adjudication, `criticFailureFold` and the
`handshakeExitStatus` forwarding. A Devin critic in that shape is not
`protocol_error`, so the follow-up gate refuses it and the chain wedges —
today as well as under revision 4. The D64 rule in that file stands (the
presence decision is the engine's; the shell only routes verdicts), so do
NOT re-route Devin through `complete_from_cli`. Instead single-source the
fold where every adapter already passes:

1. `fail_pending` in metasystem/scripts/agents/adapters/runtime-common.sh
   (lines 175-184) asks the engine to fold before it writes: a new verb
   `adapter critic-fold --record R --error E --phase P` in
   metasystem/cmd/metasystem/adapter_verbs.go prints the error class to use,
   calling the same table as `criticFailureFold`
   (metasystem/internal/adapter/adjudicate.go lines 155-179), refactored so
   the fold operates on an (error, phase) pair and the verdict-string case
   is one wrapper over it; for non-critic roles and unlisted pairs it
   prints the input unchanged. `fail_pending` then writes the folded class
   and, when its optional exit-status argument is present, `handshakeExitStatus`.
   State that `complete_from_cli`'s own path folds once (the adjudicator
   already folded; the verb is idempotent on `protocol_error`), so nothing
   double-folds.
2. devin.sh lines 650-662: both direct calls pass `"$cli_status"` (zero) as
   the exit-status argument in the handshake-phase case; the
   `empty_reply delivery` case passes nothing (it is a delivery verdict, not
   a handshake one) but is now folded for critics like every other. Name
   the fixture in metasystem/scripts/agents/dispatch-fixtures.sh or the
   Devin fixture file (find it) that pins Devin's two pinned outcomes today
   and state that its assertions for non-critic roles are unchanged and one
   critic-shaped row is added asserting `protocol_error` and
   `handshakeExitStatus == 0`.
3. Section 6: strike "inherit the exit verdict automatically"; say
   precisely what each of claude.sh and devin.sh inherits (the fold via
   `fail_pending`, the field when the call passes a status) and what they
   do not (P3 progress, in-loop custodian enforcement). Section 4: add the
   verb, the refactor and the devin.sh rows; section 5: the Go rows for the
   pair-form fold and the verb.

Bump the header to revision 5 naming the round. R-31: no benchmarks.

# Constraints

Wall-clock budget: 10 minutes. Design only; edit nothing but the design
file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the design file.

# Gap Rule

stop and report a gap; never fill it silently.
