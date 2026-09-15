# Build read: suite-progress-bed-runs-on-trunk (Codex fix task-mu2n6ael-jhk6ws)

Verdict: fix first (1 material finding). Not AGREE.

## Checklist

1. Partial pass. Namespace, record shape, unique job id and unchanged assertion text all pass. The copied dispatch.sh resolves its root to `$watch_root`, so the jobs, waiters (waiter.go:208) and mains (lease/verbs.go:102) directories all sit under `$tmp`. The record carries operationId, round 1, startedAt and completed, which is what ObserveJob accepts as green. The printer prints once at start (run.go startSuiteProgressPrinter) to stderr, and the bed captures stderr. The defect is material finding 1: the root and the workspace are now the same directory.
2. Pass. Everything the leg creates is under `$tmp`: the repository copy, the binary copy, the lease row, the job record, the waiter rows and pointers, and any hint FIFO. The lease pid is the bed shell itself, so no process is started. `job watch` runs in the foreground. Fake `wait-delivery` just prints `blocking`. The EXIT trap is unchanged and still covers every exit path.
3. Pass: a real cure, not a mask. The waiter row's identity is the `job watch` process itself (waiter.go:549). So exit 64 at registration needs a live pending row for the same job and owner in the same root. No earlier leg of the bed registers a waiter under the private root: the background watcher runs only scan-jobs and heartbeat, and the launch legs use their own bed roots. The live checkout holds no `suite-prefix` job or waiter rows. The private fake-runtime lease also fixes the owner and the delivery adapter. In the live checkout both depended on which seat ran the bed, and Store.Wait also returns 64 when delivery is missing or declined (waiter.go ~945-951).
4. Pass. The change is bed-only, one file, with no Go change. It adds no comments, so no round or finding references. It is bash 3.2 safe: no new arrays, lowercasing is done with `tr`, and `git init -b` needs git 2.28 or later (the host has 2.50.1). `cp` of the binary is required, because stateroot resolves symlinks back to the live installation (stateroot.go:266-282). It writes a new file, so there is no live-binary overwrite.

## Material findings

1. The dispatch watch leg no longer checks that dispatch.sh relays the job's workspace root.
   - Before the fix, dispatch.sh's own root was the checkout and `workspaceRoot` was the temp journal directory. A relay that ignored `workspaceRoot` would have printed the checkout's heartbeat (or nothing), and the grep would have failed.
   - Now `$watch_root` is the installation root, the journal root and `workspaceRoot` all at once (suite-progress-fixtures.sh:220, :240-245, :255-256). dispatch.sh:2076-2077 falls back to `$root` when the field is missing or unreadable. So the leg passes whether or not the relay reads `workspaceRoot`, and it passes even if `json_field` fails.
   - Nothing else covers this. dispatch-fixtures.sh:2237-2239 watches a completed job but checks only the exit code, and no fixture checks `--progress-root` routing.
   - This is the only shell logic in `watch_job`, so its only automated check is gone.
   - Fix: keep the journal in a separate workspace directory (for example `$tmp/watch-workspace`). Point `proof-run heartbeat --root`, the background watcher's `--scope` and both records' `workspaceRoot` at it, and leave the private installation at `$watch_root` with no journal. A relay that fell back to the installation root would then print nothing and the grep would fail. A decoy journal at the installation root with a different open section would make that failure explicit.

## Polish (never blocks)

- Add one comment saying why the bed builds a private installation. dispatch.sh derives its jobs and waiter directories from its own location, and the binary must be copied, not symlinked. Without that note, a later edit could "simplify" the leg back into the checkout.
- The lease is never retired. That is harmless because the root is deleted, but a `lease retire` would make the leg symmetric.
- `watch_session` is only a copy of `watch_run_id`.

## Unchecked

- How `runWaitCommand` resolves the owner from the caller pid through the private mains registry, and exactly how it handles a job that is already terminal before registration. The verdict does not depend on it.
- The bed was not run (the brief forbids it). Whether it passes on trunk rests on the fix author's run.

## Tool calls used

20 reader calls (budget 20), plus this write.
