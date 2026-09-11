# run-scoped-build-caches-have-a-janitor

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="severity 2: a janitor that deletes a live cache breaks an in-flight build, and a missing one fills the disk and cascades into a read-only guest; novelty 2: a new owner spanning several writers, the liveness rule is proven by hand but not in product; exposure 2: every delegated run and landing on every seat; accumulation 2: crosses run launch, landing, fixture beds and a scheduled trim"
- Tier: 2
- Intent: Every run-scoped build cache and worktree the metasystem creates is deleted by a janitor once its run is over or idle. On 2026-09-11 the disk reached 2.4GB free: 145GB in the shared Go build cache written within 24 hours, 163GB in per-run GOCACHE and staticcheck dirs under /private/tmp (one implementer cache alone 30GB), 36GB of leaked landing-receipt worktrees and run homes in TMPDIR, 11GB of per-run scratch; all regenerable, none pruned by anything, and the Go trimmer five-day rule never fires at that rate. DONE means every writer of a run-scoped cache or worktree (implementer and critic runs, landing receipts, fixture beds, adoption comparison) hands its directory to one janitor; the janitor deletes it when the run ends, or when no process references it and nothing in it changed for a bounded idle window; the shared Go cache is trimmed by idle mtime on a schedule; and a fixture proves a leaked directory of a dead run is gone after one janitor pass while a live run directory survives.
- Origin: human
- Next step: INTENT: no run-scoped cache or worktree outlives its run by more than the idle window, on any seat. CONSTRAINTS: liveness is process reference (argv or open file) plus idle mtime, never age in days (a one-day cut freed nothing on 2026-09-11; a two-hour idle cut was safe mid-suite because Go bumps a used entry mtime at most hourly); the janitor own argv must not carry run paths (a reaper killed the 2026-09-11 sweep with SIGTERM when its argv listed 485 run directories, a path-free Python sweep survived); never touch another seat session scratch; after deleting landing worktrees run git worktree prune. FREEDOMS: janitor as a steward tick, a scheduled job, or a run-exit hook; the idle window value; whether writers register directories or the janitor discovers them by name. This is the first concrete member of the parked umbrella disk-hygiene (backlog-notes item 19); the umbrella stays parked. ROSTER: Sol implements, Fable critiques (R-25). Tier 2, full gate width.
- OpenedAt: 2026-09-11T10:28:10Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-11T10:28:10Z R3S86XJGMCDM5N5R7AQWWBDSQT-m1-c6925449 open actor=m1+main-1788680071-18713-e76d5d targets=run-scoped-build-caches-have-a-janitor
Integrity: sha256=8bdc5d0b6cd9da443149ec1e7802a22eb51d9d034ddeec5ca3361969576bd57b
