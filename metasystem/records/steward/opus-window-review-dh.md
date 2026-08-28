Verdict: this slice is not sound enough to ship as the basis for automated reclamation. The headroom guard has three structural correctness gaps; the reclamation proof remains incomplete, and KI-35’s manual-cleanup claim is unsafe.

## Part A — Headroom guard

1. **STRUCTURAL — Filesystem discovery converts measurement failures into success.**

   [headroom.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/janitor/headroom.go:80) ascends to the lexical parent after *every* `os.Stat` error. `ENOENT` may justify that behavior; `EACCES`, `ELOOP`, `ENOTDIR`, and `ENAMETOOLONG` mean the requested path could not be established and must refuse.

   Ran: an overlong path component and a permission-denied descendant both returned exit 0 while reporting the ancestor filesystem as though the original path had been measured. `filepath.Clean` before symlink resolution also gives lexical rather than resolved-ancestor semantics for dangling links and `..` across symlinks.

2. **STRUCTURAL — The `Stat`/`Statfs` sequence is subject to identity-changing races.**

   [headroom.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/janitor/headroom.go:88) obtains `Dev` through `os.Stat`, then performs `Statfs` by pathname. A symlink, mount, or path component can change between those calls, producing a device identifier from one filesystem and free-space data from another. That corrupted pair is then used for deduplication.

   The measurement needs a pinned existing ancestor—normally an opened descriptor followed by descriptor-relative identity and capacity queries—or an equivalent revalidation loop.

3. **STRUCTURAL — `st_dev` is not a free-space-pool identity.**

   [headroom.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/janitor/headroom.go:55) assumes identical device IDs mean one free pool and different IDs mean independent pools. Bind mounts generally support the first half, although path-specific writability still differs. The second half fails for APFS: distinct volumes can have distinct device IDs while sharing container free space. Apple explicitly documents that APFS volumes share container space ([Apple File System overview](https://developer.apple.com/documentation/foundation/about-apple-file-system?changes=_7)).

   Ran: this host reports distinct device IDs for Data, VM, and Preboot while exposing the same container capacity. The guard consequently emits the same shared space three times. A future pressure or aggregate-capacity decision based on these entries would be unsound.

4. **STRUCTURAL — The suite does not reliably execute the guard before expensive work.**

   [validate-metasystem.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:66) invokes headroom only when `bin/metasystem` already exists and is executable. A clean checkout therefore starts the expensive validation/build path without any headroom check.

   That makes the protected operation depend on an old build artifact—and potentially a stale binary with a different command contract.

5. **MECHANICAL-GRAIN — Numeric validation and arithmetic are unchecked.**

   [janitor_verbs.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/janitor_verbs.go:30) converts arbitrary floating-point input directly to `int64`. Ran:

   - Negative and `-Inf` floors become permissive.
   - `NaN` becomes zero.
   - `+Inf` and very large finite values saturate at `MaxInt64`.
   - An empty `--path` silently measures the current directory while reporting an empty path.

   [headroom.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/janitor/headroom.go:98) also casts unsigned `Bavail` to signed `int64` before multiplying by `Bsize`; both that product and `FloorBytes-FreeBytes` can overflow. Reject non-finite or negative floors, convert with checked bounds, and use checked unsigned arithmetic or an explicit representable ceiling. The flag is also named “GB” while multiplying by 1024³, which is GiB.

6. **MECHANICAL-GRAIN — The validation script erases the exit-code contract.**

   The command correctly distinguishes:

   - 0: enough space
   - 1: measurement/runtime failure
   - 2: usage or invocation failure
   - 3: measured below floor

   But [validate-metasystem.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:67) maps every nonzero status to the same nonfatal “below floor” warning and continues. Exit 3 may intentionally remain advisory during migration; exits 1 and 2 mean the precondition was not established and should refuse or at least produce a distinct hard failure.

7. **MECHANICAL-GRAIN — Refusal behavior has effectively no test coverage.**

   [headroom_test.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/janitor/headroom_test.go:10) covers deduplication and low/high floors only. Its “unmeasurable” comment does not create an unmeasurable path. There are no tests for permission denial, non-directory ancestors, symlink races, overflow, invalid floats, CLI statuses 1/2/3, empty paths, stale/missing binaries, or the shell script’s status policy.

## Part B — Worktree-reclaim proof

8. **STRUCTURAL — `git worktree remove` is not a data-release proof.**

   The design first requires review, merge-or-discard approval, and no downstream consumer, but then calls no-force removal a “sound proxy” in [disk-hygiene-design.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:251). Git tests worktree cleanliness, not those lifecycle facts.

   Specifically:

   - Staged changes, tracked modifications, merge conflicts, and ordinary untracked files are normally refused.
   - A clean branch containing committed-but-unmerged work is removable. The branch usually preserves its commit, but Git has not proved review, release, or lack of consumers.
   - Ignored data is not protected. Nine currently clean delegate worktrees contain ignored artifacts, configuration, or cache files that removal could delete.
   - Populated submodules are refused without force, making them safe from this command but permanently unreclaimable unless the design specifies a supported outcome.
   - Detached `HEAD` and other per-worktree references require separate reachability treatment.

   Git documents the cleanliness and submodule limitations in its [worktree documentation](https://git-scm.com/docs/git-worktree.html). Its implementation also combines checking with recursive deletion and can proceed to administrative cleanup after a deletion failure, so it is not a side-effect-free predicate ([Git worktree removal source](https://raw.githubusercontent.com/git/git/master/builtin/worktree.c)). The release decision must be durable before removal begins, with recovery semantics for partial failure.

9. **STRUCTURAL — Garbage collection deletes records required by the proof.**

   Evidence garbage collection removes terminal job records after the default 5,400-second grace period in [gc.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/evidence/gc.go:368). Yet the proposed reclaim proof needs those records to establish lineage, aliases, ownership, terminality, and release.

   Ran: 118 delegate worktree directories exist; 12 have no same-named local job record. Four more have terminal setup-failure records with no `workspaceRoot`. The proof is therefore already unexecutable for current candidates.

   The design must provide a durable per-worktree ownership/release record retained until reclaim, or an integrity-checked archive contract, and serialize record pruning with reclamation. Missing or malformed records must fail closed.

10. **STRUCTURAL — The chain lock does not fence all aliases or readers.**

    The proposed lock excludes follow-up creation, but fresh dispatch accepts an arbitrary workspace under a different job lock, while conformance readers do not share the chain lock. A second job or reader can therefore hold the same canonical path while reclamation owns the original chain lock.

    The resource needs a canonical-workspace lease shared by fresh dispatch, follow-ups, conformance, and the reclaimer. Merely resolving every recorded alias while holding one lineage lock is insufficient.

11. **STRUCTURAL — Process-group death is still not custody or use-release.**

    “No-escape enrollment” in [disk-hygiene-design.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/disk-hygiene-design.md:262) has no state owner or enforcement mechanism. An empty original process group does not exclude:

    - a descendant that called `setsid` or changed groups;
    - another dispatched job;
    - conformance;
    - an editor or other same-user process with its current directory or an open descriptor inside the worktree.

    The proof needs an enforceable closed consumer set or a reclaim-time, same-user kernel census tied to the canonical path. “Procfs trust” must also be expressed by platform capability: supported macOS identity inspection uses `sysctl`, not procfs.

12. **STRUCTURAL — Descendant resources are outside the proposed containment proof.**

    Checking that the candidate itself is a registered worktree does not establish that it contains no registered descendant worktree, nested repository, ignored artifact root, or independently owned resource. A same-device descendant bypasses the mount-crossing check and can be recursively deleted.

    Reclamation needs a child-first resource inventory and explicit ownership/release proof for every managed descendant. Unsupported submodule or nested-resource cases should remain report-only; refusal must never fall back to forced removal or raw recursive deletion.

13. **STRUCTURAL — KI-35’s manual-cleanup instructions are unsafe as written.**

    [KI-35](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/known-issues.md:42) claims “no data loss” from `git worktree remove`, but supplies none of the lifecycle, consumer, alias, ignored-data, reachability, or lock preconditions above.

    Its subsequent unscoped `git worktree prune` is repository-global. Ran: `git worktree prune --dry-run --verbose` proposes pruning an unrelated detached scratchpad, `wt-flakefix`. Git documents that moved or temporarily unavailable worktrees may need locking or repair rather than pruning. Successful `git worktree remove` already removes its own registration, so the blanket prune is unnecessary and can erase administrative references belonging to other worktrees.

    KI-35 should be changed to report-only until a candidate-specific checklist and locking mechanism exist.

Evidence: I read the target code, tests, design, Git integration, evidence garbage collector, and repository guidance; ran the existing headroom command against boundary/error inputs; inventoried worktrees, records, ignored contents, and device identities; and ran Git prune only in dry-run mode. I did not run the Go tests because the read-only environment could not create Go’s temporary build directory. No files were modified.

Proposed review receipt: `Special review of disk-hygiene window output — revision required; structural headroom and reclamation-proof defects remain; read-only review.`

REVISE — structural findings remain
