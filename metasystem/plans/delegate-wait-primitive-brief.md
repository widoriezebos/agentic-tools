Working Mode: implement
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal delegate-wait-primitive)
Date: 2026-09-01

# Goal

One honest wait: `scripts/agents/wait-job.sh <job-id>` blocks until the
job's record reaches a terminal status, always terminates, and always
reports what happened — replacing every hand-rolled wait. Two seats in
two days waited on artifacts a dead worker would never write
(records/misc/idle-loss-2026-08-31.md, idle-loss-2026-09-01.md).

# Workspace

The dispatch-created job worktree, branched from main. May touch
EXACTLY (inside the metasystem/ project):

- scripts/agents/wait-job.sh (new)
- scripts/agents/wait-job-fixtures.sh (new)

# Mechanical rules — no judgment calls

1. Input: one job id (validated against the existing valid_id shape in
   dispatch.sh). The job record is
   artifacts/agents/jobs/<id>.json.
2. Poll the record every 5 seconds. TERMINAL statuses are exactly the
   set the engine's terminalStatuses map recognizes
   (internal/dispatch/close.go territory — read the code, mirror the
   set, name it in a comment as mirroring that map).
3. Patience per the R-35 law — progress-based, never bare wall-clock:
   track the record file's mtime and size; if the record has not
   CHANGED for 15 minutes AND the status is non-terminal, exit 3
   (stalled) after printing the last-seen status. Overall hard bound:
   2 hours, exit 4 (bound), printing the same.
4. A missing record file: wait up to 60 seconds for it to appear
   (dispatch may still be writing), then exit 5 (no-record).
5. On terminal: print one line
   `job=<id> status=<status> failureReason=<reason-or-none>`
   and exit 0 for completed, 1 for failed, 2 for any other terminal.
6. No state written anywhere; read-only against the record.

FIXTURES (wait-job-fixtures.sh, self-contained temp dir, same idiom as
the sibling *-fixtures.sh scripts): (a) a record that goes terminal
completed while waiting → exit 0 and the line; (b) a record already
failed → immediate exit 1 with failureReason printed; (c) no record →
exit 5 after the grace; (d) a record that never changes → exit 3 at
the stall bound (shrink the bounds via env override variables the
script honors for testing — document them).

# Constraints

- Bash only, no engine changes. shellcheck-clean if available;
  `bash -n` clean regardless. Wall-clock budget: 30 minutes.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the two files WITH
the metasystem/ prefix; evidence includes running the fixture script
(it needs no process visibility — records are plain files — so it MUST
run green in your sandbox; a red here is a product red, not
environment).

# Gap Rule

stop and report a gap; never fill it silently.
