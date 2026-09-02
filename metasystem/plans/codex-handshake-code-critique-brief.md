Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Two-layer implementation critique of the Codex handshake build (job
ch-build-1c, Sol; diff.patch in its round evidence): first conformance
against the certified design metasystem/plans/codex-handshake-design.md
(revision 7; sections 2 and 3 are the contract, section 4 the file
boundary, section 5 the proof; Sol's four registers are
metasystem/records/misc/codex-handshake-critique-r1.md to -r4.md), then
adversarial defect review of the diff. The design's revision 7 fold
(CHS-R4-FOLD-SCOPE-01: the Claude adapter's two pre-fork failure writes
carry `launch_failed handshake`) had no Sol round of its own; check it
with particular care.

# Attack surface

- Part 1: `plugins={}` as a `-c` pair after `approval_policy="never"` on
  BOTH the dispatch and the resume verb in metasystem/internal/adapter/codex.go;
  the pins named in D1.6.
- Part 2: the three deadline writers each set `handshakeDeadline` AND
  `handshakeProgressAt`; the one gate `RefreshHandshakeDeadline`; the
  capability `handshakeProgressBoundSec` beside the old field and the
  record field `handshakeBound` immutable; custodian `now >= deadline+1`,
  waiter `now >= deadline`, reaper `now < deadline+2`.
- D2.5: `fail_pending` and `finish_running` (failed target only) in
  metasystem/scripts/agents/adapters/runtime-common.sh call
  `adapter critic-fold` on ONE line, `set -e` safe, keeping the caller's
  class when the verb cannot run; `handshakeExitStatus` written only where
  the design says (0..255, absent otherwise); the fold table is the single
  source and `criticFailureFold` is one wrapper over the pair form; no
  double fold on `complete_from_cli`'s path; claude.sh:134 and :138 write
  `launch_failed handshake`; devin.sh:650-662 and :720 as designed.
- D2.7: the follow-up gate admits `failed && protocol_error` with no
  session, the fresh-context road, `--parent-sessionless`, NO
  `LaunchFingerprintVersion` bump (metasystem/internal/dispatch/claim_fingerprint.go
  still says 2), `ParentSessionless` under `omitempty`.
- Fixtures: `no-signal`, `slow-session`, `exit-before-session` (with the
  round-2 follow-up leg), `hang-gone-dispatcher` in
  metasystem/scripts/agents/dispatch-fixtures.sh; ADPT-DL-006 to -009 in
  metasystem/scripts/agents/adapter-deadline-fixtures.sh; every wait
  carries a named scaled ceiling; no benchmark (R-31).
- Any hunk outside section 4's file boundary is a finding. Any test the
  return claims green that the diff does not actually contain is a finding.
  A weakened refusal or narrowed guarantee to make the gate pass is a
  finding.

# Constraints

Wall-clock budget: 20 minutes. Your sandbox is read-only; verify by reading.
Return per the code-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
