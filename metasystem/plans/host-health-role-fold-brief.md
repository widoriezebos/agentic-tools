Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal host-health-role)
Date: 2026-09-02

# Goal

Revision 2 of metasystem/plans/host-health-role-design.md (revision 1,
landed; edit it in place, bump the revision line): fold the seven material
findings of metasystem/records/misc/host-health-critique-r1.md by id.
Every closure is a design change verified against the tree, never a
softened claim. Keep the design small: this is one new steward role, not
a monitoring product.

# Workspace

The delegate worktree the dispatcher created for this job. Edit exactly one
existing file, the design; nothing else.

# Direction per finding

- HH-R1-LINUX-CPU-LIFETIME: procps reports CPU over the process lifetime,
  so a long-lived process that turns runaway may never cross the
  threshold on Debian. Specify a recent-CPU signal on Linux: two reads of
  /proc/<pid>/stat utime+stime across the tick interval (or across two
  consecutive ticks, persisted in the role's state), divided by the
  interval and the core count; on macOS keep the ps figure or use the
  same two-read method for symmetry. State which and why; the fixture
  must replay an idle-then-hot process.
- HH-R1-CENSUS-PID-ONLY: the ownership join must use the census row's
  full identity (pid plus start identity), never the pid alone; read how
  metasystem/internal/census identifies a row and join on that; a stale
  or reused pid is "not ours, unproven", never "ours".
- HH-R1-INVENTED-OWNERSHIP-SHAPES: drop the invented fallback rules; "ours"
  means exactly what the census and janitor shape rules already say
  (metasystem/internal/census, the janitor shape table the custody design
  names in metasystem/plans/proof-harness-custody-design.md); anything
  else is foreign, and the remedy says so.
- HH-R1-MEMORY-NOT-SCALED: read physical memory (sysctl hw.memsize on
  macOS, MemTotal in /proc/meminfo on Linux) and express the per-process
  resident threshold as a percentage of it with a sane default; keep the
  swap signal where swap exists and say what the role does on a swapless
  VM (available memory percentage from the same sources).
- HH-R1-HOST-ONLY-ATTRIBUTION: a load-only or swap-only verdict names the
  top process as "the largest consumer" without claiming cause, and says
  whether that process is ours by the census join; never borrow a
  remedy from a process that crossed no threshold of its own.
- HH-R1-CONFIG-REMEDY-BLIND: add the six threshold keys to the validator
  (metasystem/internal/config/validate.go) with their grammar and ranges,
  so the advertised "metasystem config validate" actually diagnoses them.
- HH-R1-SWEEP-REMEDY-NOOP: the remedy for an owned offender names the
  janitor orphans invocation WITH --apply as the repair and the
  report-only form as the preview, matching the custody design's
  contract; the fixture checks the apply text.

Also answer the critic's two gaps: state the budget for this design loop
(this fold, then one closing review; if material findings remain, the
seat decides) and re-derive the size honestly against the roughly 715
lines the critic counted, splitting into two slices if the four-hour box
with a correction round does not fit.

Fold record: add a section mapping each finding id to its fold. Self-grade
per the house rule.

# Constraints

Wall-clock budget: 30 minutes. Design only; edit nothing but the design
file. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
