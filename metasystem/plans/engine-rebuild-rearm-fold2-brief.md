Working Mode: design
Orchestrator Identity: m1 (lineage main-1788594343-3833-fb64b9, dispatch delegate under goal engine-rebuild-rearms-itself, tier 3 DESIGN-BEARING)
Date: 2026-09-06

# Goal

Revision 2, eight-finding fold of
metasystem/plans/engine-rebuild-rearm-design.md (revision 1 landed at
a4c5947d5). Register: metasystem/records/misc/engine-rebuild-rearm-critique-r1.md,
landed; it carries the critic's exact claims and evidence, and they bind.
Fold each finding BY ID. Where a finding contradicts a ruling, the ruling
wins and the design says so.

# The folds, by id

- ERAR-R1-01-STRAY-CRON: the sole eligibility test (invoking engine is the
  enrolled path) admits the stray cron caller the threat model exists to
  reject, because up derives Binary from os.Executable and re-arms before
  session identity is resolved. Revision 2 must bind automatic re-arm to a
  resolved, announced session identity (the same proof up already needs
  before announcing) or to another fact a stray cron cannot supply, and
  say which and why.
- ERAR-R1-02-LOCKED-ORDER: the locked eligibility-and-skip decision must
  precede every stop and no-op, not merely the mint. A stranger that
  should refuse must never stop the enrolled runner; a second concurrent
  re-arm must return already-current without stopping the newly installed
  runner. Restate the arm ordering as steps under the lock.
- ERAR-R1-03-PROVENANCE: R-37-m3 requires engines built from landed
  commits and requires every re-arm to record the consumed commit. A dev
  build stamp is therefore not eligible for automatic re-arm; say what an
  unstamped or dev-stamped rebuild does (refuse with the human remedy), and
  bind EngineBuild to the bytes digested under the lock, not to the
  invoking process's stamp.
- ERAR-R1-04-LEGACY-MIGRATION: the current generation on m1 was
  machine-minted and carries no stamp; ordinary Arm returns already-armed
  before minting when a runner is live, so "the next steward arm clears
  it" is false. Specify the migration: what marks legacy generations, and
  what human act clears machine state, as a build prerequisite.
- ERAR-R1-05-VISIBILITY: a re-arm must never be silent end to end. Carry
  the re-armed fact through every Result the helpers construct after it,
  through the Stop path including emit_failed_stop and the four-second
  parent's deadline response, and through the session-start path. Say
  where it is persisted so a later turn can still show it.
- ERAR-R1-06-PARTIAL-MINT-API: arm returns only string and error, so the
  partial-mint contract cannot be implemented. Specify a typed stage or
  outcome return from arm covering: before mint, mint landed, reopen
  failed, snapshot failed, launch failed - and what the automatic path does
  at each.
- ERAR-R1-07-FIXTURE-HARNESS: F6's steward run never reads BuildStamp, an
  in-process fixture cannot prove engine B's commit, and the gate installs
  after tests. Name a proof that can run: which fixtures become
  shell-driven with two real engine builds under the orchestrator, which
  become Go tests with a fake, and which claims have no proof and are
  recorded as residual risk instead of claimed.
- ERAR-R1-08-C16-OUTCOME: drift returned by EnrolledBinary.Command during
  supervision-owner or steward-runner launch propagates as a component
  failure, not through enrollmentDrift. Route it to the declared refusal
  outcome or state and test the distinct mapping.

Also correct the non-material label: C13 and C18 are both labelled RE-ARM;
say "one automatic cause, C13, with C18 conditional on it".

Consistency pass over every decision; self-grade; reject condition
restated. Bump the revision header to 2 with today's date.

# Constraints

Wall-clock budget: 25 minutes. The eight folds and the label only; no
other decision moves. Read metasystem/internal/steward/identity.go,
metasystem/internal/steward/runner.go, metasystem/internal/up/up.go and
metasystem/scripts/agents/supervision-hook.sh before writing.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/engine-rebuild-rearm-design.md (that one file).

# Gap Rule

Stop and report a gap; never fill it silently.
