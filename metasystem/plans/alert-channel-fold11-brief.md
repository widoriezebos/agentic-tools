Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal alert-escalation-channel)
Date: 2026-09-01

# Goal

Revision 11 of metasystem/plans/alert-channel-design.md: fold all four
round-4 findings (metasystem/records/misc/alert-channel-critique-r10.md) using the
EXECUTABLE SPIKE VERDICTS (metasystem/records/misc/alert-channel-spike-verdicts.md) as
your evidence base — both landed, in your worktree. Every disputed mechanism
now has a tested rule; your job is to write those rules into the design
coherently, not to re-derive them.

# The folds, by id, each with its spike-proven rule

- AC9-JOB-ID-ABA-001 (critical): the pin key and episode digest use the
  job record's machine-minted birth generation (timestamp plus nonce,
  minted by the create path under the record lock, immutable) — the
  contract change itself is goal job-record-birth-token (opened, budgeted);
  this design DEPENDS on it and says so, with the interleaving table's
  reuse row proven against the minted token per the spike.
- AC9-SCAN-BOUNDEDNESS-001: the producer scan's filename-index contract
  stands as revision 10 wrote it (spike: 8.4ms over 10,020 names, zero
  opens); the health path gains the spike's rule — the health load
  restricts to health-named files by the existing filename grammar
  (spike: 10,020 decodes at 110ms under the alert lock drops to 20 opens
  at 10.6ms).
- AC10-STOP-CLEAR-READSET-001: adopt the spike's reversible journal-time
  marker — alerts/stop-open/<goal>-r<revision> containing the digest,
  written before the episode, listed by the clear phase, bounded by open
  stop episodes, draining on clear; the spike reproduced the regression
  and demonstrated this rule restoring the clear with zero episode opens.
- AC9-ANSWER-FOLLOWUP-ACTION-001: the producer journals the failed
  record's own immutable reviews field and renders it into the advertised
  command (fixing the categorical refusal for code-critic and warden
  roles); row 1's follow-up validity is stated as journal-time-only with
  the pin's coverage boundary honest (completed chain roots are not
  pinned).

Then the consistency pass; self-grade; the reject condition stays a third
implementer gap-stop.

# Constraints

Wall-clock budget: 35 minutes. The spike's rules are evidence, not
suggestions to improve upon — deviations return through the loop. Wido's
standing words untouchable.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/alert-channel-design.md (that one file).

# Gap Rule

stop and report a gap; never fill it silently.
