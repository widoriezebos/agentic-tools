Working Mode: implement
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal failed-job-attention)
Date: 2026-09-01

# Goal

The machinery that knows a delegate job died says so loudly through
what already exists: the steward tick raises an ESCALATION EPISODE
(the existing episode machinery — the same class of surfacing as the
stalled-idle escalations) when (a) a job under a claimed goal sits in
terminal FAILED status, and (b) when a breach-stop has fired. Episodes
already reach the health output and the narrator digest; this is the
internal surfacing that should have existed on 2026-09-01 at 22:17
when a worker died and nobody learned for six hours
(records/misc/idle-loss-2026-09-01.md).

# Workspace

The dispatch-created job worktree, branched from main. Expected
touched area (declare every path in diffBoundary WITH the metasystem/
prefix): internal/steward (the tick's escalation path and its tests).
Read the stalled-idle escalation implementation FIRST and follow its
pattern exactly — episode id shape, dedup, digest emission, health
surfacing. If the pattern cannot carry these two classes without
structural change, STOP and report the gap.

# Mechanical rules

1. FAILED-JOB ESCALATION: each tick, for every job record under
   artifacts/agents/jobs whose status is failed AND whose goal field
   names a currently CLAIMED goal AND whose chain is not closed:
   raise/refresh one escalation episode keyed
   escalation-failed-job-<jobId>. The episode text names the goal, the
   job, and the failureReason (or "none recorded"). Dedup: one episode
   per job, refreshed not duplicated, exactly as stalled-idle
   episodes behave across ticks.
2. BREACH-STOP ESCALATION: when a breach-stop record exists for a goal
   revision and no resume has cleared it, one escalation episode keyed
   escalation-breach-stop-<stopId>, text naming the goal, the fence
   values, and the exact resume command that answers it (the
   plain-words law of docs/seat-communication.md applies to the text).
3. Both stop escalating once answered: a closed chain or acknowledged
   episode for (1); a resumed goal for (2). Acknowledgment rides the
   existing health acknowledge-alert seam untouched.
4. TESTS, following the existing steward escalation test idiom:
   (a) failed job + claimed goal → episode raised within one tick,
   with dedup proven across two ticks; (b) failed job with chain
   closed → no episode; (c) breach-stopped goal → episode with the
   resume command in its text; (d) resumed goal → episode stops.

# Constraints

- No new delivery machinery, no external sends — internal surfacing
  only; the external channel is another goal's scope.
- No changes to breach-stop or reaper behavior — read their records,
  never mutate. No test weakened. Wall-clock budget: 40 minutes.

# Expected Return

Version-2 implementer JSON; complete prefixed diffBoundary; evidence
includes the steward escalation-focused test run (name the exact -run
pattern) — these fixtures build their own temp repos, so they must run
in your sandbox; report any that cannot with the exact missing
visibility.

# Gap Rule

stop and report a gap; never fill it silently.
