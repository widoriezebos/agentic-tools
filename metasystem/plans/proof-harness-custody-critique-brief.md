Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal proof-harness-process-custody)
Date: 2026-09-02

# Goal

Round-1 critique of metasystem/plans/proof-harness-custody-design.md
(revision 1, landed, in your worktree), the design for goal
proof-harness-process-custody (read metasystem/plans/goals/proof-harness-process-custody.md
first: two specimens, the done criterion with the seat-runnable sweep).
The design builds on metasystem/plans/recovery-analysis.md section 4 and
states a scope boundary against the recovery umbrella's slice S4. It
decides two things: harness custody is an engine verb (load-generate,
heading its own process group, winding the group down on parent death,
ceiling, or signal, with workers watching the leader), not a shell trap;
and the sweep is a janitor orphans verb, report-only by default, run by
hand, never on a cadence. Five declared gaps ride its return; the
self-grade names the bed-age rule and the tracked-run launcher assumption
as its two risks.

# Your mandate

1. SETTLE THE TWO DECLARED RISKS against the tree: (a) the preferred
   harness contract routes the load verb through the tracked-run launcher
   with an inherit-group flag and assumes the run wrapper keeps its
   workload inside the wrapper's process group; trace that into the
   wrapper's launch code (metasystem/cmd/metasystem, metasystem/internal/run,
   the run launch verb) and say whether the assumption holds or the
   named fallback must be the design; (b) the bed-age rule uses the bed
   root directory's modification time; confirm it errs toward keeping
   and can never remove a live bed.
2. ATTACK DECISION 1 (section 2): is every exit path covered as claimed
   (normal, error, interrupt, terminate, hangup, hard kill of the
   custodian, tmux pane death, the specimen's wrapper detachment)? Does
   the parent watch use the exact identity (pid, start, ticks, boot id)
   the delegate adapters use (metasystem/internal/census), and does a
   worker really exit on leader death without a race that leaves it
   running? Is the janitor shape-table row enough for the existing
   group-ownership proof (the group-owned verb) to cover both verbs?
3. ATTACK DECISION 2 (section 3): the candidate rule (engine argv shape,
   bed under a temp or preserved-failure root, older than the bound) and
   the victim rule (a production identity re-proof) against the
   shared-machine rule in metasystem/docs/orchestration.md (kill only what
   you can prove is yours, by exact pid, never by pattern); name any
   process the sweep could kill that is not ours, and any leaked class
   from the 2026-09-02 specimens (steward runners, supervise components,
   the revocation-race loop, the fake-adapter loop, fingerprint-harness
   runners) it would miss. Judge the by-hand-only choice against Wido's
   idempotent start-and-stop word on goal recovery-to-good-state.
4. ATTACK THE SCOPE BOUNDARY (section 0) against recovery-analysis.md
   section 6.2 slice S4: is anything designed here that S4 must also
   design, or left to S4 that this goal's done criterion needs?
5. ATTACK THE FIXTURES (section 4) and SIZE (section 5): deterministic,
   bounded waits only, each exit path actually exercised, the lookalike
   refusal proven; two slices at most 240 reserved minutes each with the
   estimates honestly marked.
6. NEW FINDINGS only if material and grounded.

Findings quote the disagreeing text or code. Your sandbox is read-only:
verify by reading, do not run go. Declared gaps are residuals, not
findings, unless one hides a false claim. Zero material findings is an
acceptable, closing answer if the reading supports it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
