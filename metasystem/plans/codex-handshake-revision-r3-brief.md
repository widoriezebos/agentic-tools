Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Revision 3 of metasystem/plans/codex-handshake-design.md (revision 2
landed, in your worktree). Sol's round-2 register is
metasystem/records/misc/codex-handshake-critique-r2.md: ONE material
finding, CHS-R2-CRITIC-FOLD-01 (high). Everything else held. Edit in
place; diffBoundary is that one file; mark the closure with the id. One
fold, then the consistency pass. Keep it under twelve minutes.

# The fold

CHS-R2-CRITIC-FOLD-01. A critic (design-critic, code-critic, warden) whose
child exits before its session must still be folded into its canonical
register and its chain must be able to continue. Today the only road to
`CritiqueRegisterAdvance` is the follow-up path in
metasystem/scripts/agents/dispatch.sh: its gate (lines 1727-1753) accepts
`completed` or `failed && error == protocol_error`, and line 1757 then
refuses any parent without a `sessionId`. Revision 2's unfolded
`handshake_failed:exit=N` fails the gate; and the session refusal is
pre-existing, so even `protocol_error handshake` critics wedge today — the
goal's own specimens were exactly such critics. Close BOTH halves:

1. The class. Restore the critic fold for the pre-session exit:
   `criticFailureFold` (metasystem/internal/adapter/adjudicate.go lines
   155-179) maps `fail-pending handshake_failed:exit=N handshake` to
   `fail-pending protocol_error handshake` for the three critic roles, so
   the gate and `syntheticProtocolFindingID`
   (metasystem/internal/dispatch/finding_register.go) are untouched. Keep
   CHS-R1-EXIT-02 satisfied by carrying the status OUTSIDE the error
   class: the custodian's fail-pending branch in
   metasystem/scripts/agents/adapters/runtime-common.sh already holds the
   CLI status it passed as `--cli-status`; specify a record field
   `handshakeExitStatus` (integer) written in that same patch whenever the
   phase is `handshake` and the child exited, for every role, and the
   dispatcher's outcome line reads it ("exited with status N") from the
   field, not from the error. Non-critic roles keep the unfolded
   `handshake_failed:exit=N` error AND the field. Fix D2.5, the verdict
   table row, the outcome-line paragraph and section 4's adjudicate rows.
2. The session. Specify that the follow-up path accepts a parent that is
   `failed && error == protocol_error` with NO `sessionId` by taking the
   existing fresh-context road (dispatch.sh lines 1888-1900: `resume_mode=
   fresh-context`, `adapter_verb=dispatch`, the `prior-brief` continuation;
   the `prior-return` continuation is omitted when the parent round has no
   return.json) so the register fold at lines 1855-1859 runs and round N+1
   starts fresh. State the exact condition added at line 1757 and that a
   `completed` parent without a session is still refused as today.
3. The proof. `exit-before-session` (section 5) stays design-critic shaped
   and, after asserting `error == protocol_error`, `phase == handshake`,
   `handshakeExitStatus == 137` and the dispatcher text "exited with status
   137", dispatches a follow-up on the same chain and asserts that round 2
   started (`dispatchMode` follow-up, `resumeMode` or the equivalent field
   naming fresh-context — read the record field names in
   metasystem/internal/dispatch/record.go and dispatch.sh) and that the
   canonical register carries one synthetic finding for round 1. Add a
   `TestCriticFailureFoldKeepsHandshakeExit` row in
   metasystem/internal/adapter/adjudicate_test.go and a non-critic row that
   stays unfolded. Name the caps every new wait uses.

Consistency pass over sections 3, 4 and 5 where the fold touches them; bump
the header to revision 3 naming the round. R-31: no benchmarks.

# Constraints

Wall-clock budget: 12 minutes. Design only; edit nothing but the design
file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the design file.

# Gap Rule

stop and report a gap; never fill it silently.
