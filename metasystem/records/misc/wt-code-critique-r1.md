Verdict: 8 material findings—5 structural and 3 mechanical-grain. The report-only command deletes nothing today, but its `reclaimable` verdict is not safe for a future reclaimer to trust.

### Findings

1. **CRITICAL — STRUCTURAL — Terminal and quiescent do not prove the worktree’s data is disposable.**

   [ClassifyWorktree](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/janitor/worktrees.go:58) treats terminality as the complete lifecycle proof. In the actual workflow, conformance review and merge happen after the implementer terminates; they read the worktree to compute the authoritative diff.

   Verified by running the new verb: it classified three dirty implementer worktrees with `chainClosed:false` as reclaimable, including `caps-census-gate-order`, which still has a modified `scripts/agents/dispatch.sh`, and `implementer-20260807t103103z-fc4f`, which has a staged new file. A future deletion would discard those bytes without proving they were reviewed, merged, or released.

   Even `chainClosed` alone is insufficient: [CloseCheck](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/close.go:55) explicitly allows an unreviewed completed implementer with no computed diff. Reclamation needs an explicit data-release/merge proof, not lifecycle terminality. The design row itself therefore needs revision.

2. **CRITICAL — STRUCTURAL — Custody-list death is not process-group quiescence.**

   Input: terminal job, registered CLI process dead, but an unregistered grandchild or detached process remains in or uses the worktree. Output: `reclaimable`.

   [readJobRecord](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/janitor/worktrees.go:165) ignores the record’s supervisor `pid`, `pgid`, ownership proof, and `groupDeathProvenAt`. An empty custody list is explicitly considered quiescent and is blessed by [the table test](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/janitor/worktrees_test.go:28).

   This conflicts with the real topology:

   - Normal completion waits only for the direct CLI child before terminalizing ([runtime-common.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:277)).
   - Group enumeration exists separately because grandchildren survive reparenting ([runtime-common.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:209)).
   - Handshake timeout writes `failed` before attempting best-effort wind-down, which may fail ([dispatch.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:1460)).

   `dispatch.TerminalStatus` is correct as the immutable record-state vocabulary; it is not a reclamation-safety predicate. `completed` and `failed` lack universal group-death proof, while even group sweeps cannot account for a process that escaped the group unless enrollment/no-escape is enforced.

3. **CRITICAL — STRUCTURAL — Follow-up aliases produce a direct false reclaimable verdict, with a time-of-check race.**

   Input:

   - `jobs/root.json`: terminal, custody dead.
   - `jobs/root-r2.json`: running, live process, `workspaceRoot` points to `worktrees/root`.

   Output: `worktrees/root` is `reclaimable`.

   This is a normal topology, not corruption. Follow-ups deliberately inherit the prior record’s workspace ([BuildFollowRecord](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/build.go:273)); dispatch reads that shared path and starts `root-rN` there ([dispatch.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:1091)). The observer only reads `jobs/root.json`.

   It also takes no chain/coordinator lock. A follow-up can start after the root was classified but before the report is consumed. A destructive slice must resolve every record referencing the canonical workspace and revalidate while holding the same lock that excludes follow-up creation.

4. **HIGH — STRUCTURAL — Dispatch ownership and filesystem containment are assumed, not proven.**

   Input: directory `worktrees/x` plus `jobs/x.json` containing terminal status and dead/empty custody, but missing or mismatching `jobId` and `workspaceRoot`. Output: `reclaimable`.

   The parser does not read either identity field. It also does not verify the directory is registered as a Git worktree.

   Direct symlink entries and nested symlinks are safely not traversed. However, a symlinked `artifacts/agents/worktrees` ancestor is followed by `os.ReadDir` and `WalkDir`; the reported path remains lexically inside the checkout while resolving outside it. A future `RemoveAll`-style reclaim could escape containment. Canonical path, no-symlink ancestry, Git registration, `jobId`, and `workspaceRoot` equality need to be part of the ownership proof.

5. **HIGH — STRUCTURAL — The same-user/procfs premise required for trusting `Dead` is not enforced here.**

   On Linux, [KernelProber](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/identity/identity_linux.go:34) interprets `ENOENT` as `Dead`, relying on supervision’s separate refusal under restrictive `hidepid` procfs. The janitor verb neither performs that refusal nor proves that its caller is the user who created the delegate jobs.

   Input: a shared checkout surveyed by another user under restrictive procfs, where a live custody PID is hidden as `ENOENT`. Output: `Dead`, then `reclaimable`. Ordinary `EACCES`/`EPERM` correctly becomes `Unknown`; the missing invariant is what makes `ENOENT` trustworthy.

6. **HIGH — MECHANICAL-GRAIN — Invalid identity values can become proof of death.**

   The `*int64` fields correctly distinguish absent from zero, but zero and negative values are not rejected.

   Input: terminal record with custody `{pid: 4002, pidStartedAt: 0}`, while PID 4002 is alive with start time 20. `AliveRef` treats the mismatch as `Dead`, so the observer returns `reclaimable`. Invalid identity data should produce `Unknown`.

   Separately, [ClassifyWorktree](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/janitor/worktrees.go:68) treats every `identity.Liveness` value other than `Alive` and `Unknown` as dead. `identity.Liveness(99)` therefore authorizes reclamation. Only the explicit `Dead` value should count as proof.

7. **MEDIUM — MECHANICAL-GRAIN — Invalid root resolution returns a successful empty report.**

   Verified by running:

   - `--root /definitely/not/a/metasystem-checkout --json` → exit 0, empty survey.
   - Running with the default root from `internal/janitor` → exit 0, empty survey.

   This makes a typo or wrong current directory indistinguishable from a valid checkout containing no worktrees. Also, `WorktreeState.Path` claims to be absolute, but the live JSON emitted relative paths such as `artifacts/agents/worktrees/...`.

   Invalid flags correctly return 2 and actual `ReadDir` errors return 1. The verb should validate and canonicalize the checkout root before treating a missing worktrees directory as empty.

8. **MEDIUM — MECHANICAL-GRAIN — Size failures silently corrupt byte totals.**

   [dirSize](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/janitor/worktrees.go:201) discards walk errors and returns a partial value, which is then added to `TotalBytes` and `ReclaimableBytes` without any “size unknown” indication. An unreadable subtree can therefore report zero or a partial size while the worktree remains classified reclaimable.

   It also sums logical file lengths rather than allocated disk blocks, so sparse files and hard links can materially misstate the disk space recoverable. This does not corrupt the verdict, but it does corrupt the disk-hygiene report.

### Checks that behaved safely

By reading the code:

- Missing/unreadable job record → `Unknown`.
- Malformed JSON → nonterminal/`Retained`.
- Missing PID or start field inside a custody entry → `Unknown`.
- Kernel probe permission errors → `Unknown`.
- Non-directory entries and direct symlink entries are skipped.
- State sorting is deterministic; directory names are inherently unique, so no deduplication issue exists.
- `humanBytes` handles exact binary-unit boundaries and the `int64` range safely.

The existing tests do not exercise lifecycle release, aliases, group membership, containment, restricted procfs, zero identity values, concurrency, or CLI behavior. Several tests instead pin the unsafe assumptions.

Verification: I ran the existing new binary against the live checkout; it reported 118 worktrees, 106 reclaimable, 12 unknown, and 527,323,503 reclaimable bytes. Focused `go test` could not start because the read-only sandbox denied creation of Go’s build directory. No files were modified.

REVISE — structural findings remain
