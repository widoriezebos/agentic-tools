# Confirmation read: suite-progress-bed-runs-on-trunk (fold of material finding 1)

Verdict: land (0 material findings)

The prepared copy's `git diff` is byte-identical to suite-progress-code.diff. The only modified file is metasystem/scripts/agents/suite-progress-fixtures.sh.

## Folded finding

1. finding 1: confirmed.
   - Journal location. The journal is written only to `$watch_workspace` = `$tmp/watch-workspace` (suite-progress-fixtures.sh:221, :226, :244-248). The private installation is `$watch_root` = `$tmp/watch-repository/metasystem` (:219-220). Its mkdir creates only artifacts/agents/jobs, scripts/agents/adapters and bin (:226-228), so there is no supervision directory and no journal in it. The only non-test code that names suite-progress.jsonl is the proof-run reader (proof_run.go:1304) and gate or adopt scripts (validate-metasystem.sh:137, go-gate.sh:81, adopt-fixture-helpers.sh:269). None of these runs in this leg, and lease announce does not name the journal.
   - Where the leg points. `proof-run heartbeat --root` (:249), the background watcher's `--scope` (:254) and both records' `workspaceRoot` (:251 prefix-job.json, :259-260 the dispatch record) all point at `$watch_workspace`.
   - Relay and root. The copied dispatch.sh resolves `root` from its own location (dispatch.sh:21), which gives `$watch_root`. `jobs` is `$root/artifacts/agents/jobs` (:70-71), so the record at fixtures:258 is found and watch does not refuse with exit 5. watch_job passes `--progress-root "$watched_root"` (dispatch.sh:2076-2082).
   - The mutation must fail. Suppose the relay ignores `workspaceRoot` and uses `$root`. The printer then runs `deepestSuiteHeartbeat($watch_root)`, which is a plain `filepath.Join(root, "artifacts/agents/supervision/suite-progress.jsonl")` with no stateroot remap or env override (proof_run.go:1303-1309). That file does not exist, so the printer prints nothing (proof_run.go:1320-1327 prints only when ok). The strings `inner` and `child` appear only in the workspace journal, and the dispatch record (:259) does not contain `inner:child since 0min`. A fallback to the repository root would fail the same way, because `$tmp/watch-repository/artifacts` does not exist. The mutation does not change what `job watch` waits on: the record is still terminal and the printer returns no value. So the command still finishes, the grep at :263 fails, and the leg exits 1 with "dispatch watch did not print the deepest heartbeat" (:264). The fold author's mutation result must follow from the code.
   - Comment. The single added comment (:230-231) says dispatch.sh derives its jobs and waiter directories from its own location, so the private installation needs a copied binary rather than a symlink. That covers the requested explanation, and it carries no round or finding reference.
   - No other change of meaning. The assertion texts at :250, :255-256 and :263-264 are unchanged. The background watcher leg moved `--scope` and `workspaceRoot` together, so they still match each other as they did in the base. The header comment at :216-218 is unchanged. The record removal dropped from cleanup is safe, because the record now lives under `$tmp`, which `rm -rf "$tmp"` removes.

## New material defects introduced by the fold

None found.

Non-blocking: the optional decoy journal at the installation root was not added. It is not needed, because a missing journal already makes the grep fail.

## Unchecked

- The body of `proofrun.ReadLatestProgressRun` and `DeepestLiveHeartbeat` (progress.go:230+) for a missing file. The conclusion does not depend on it: no reader can produce `inner:child` from a directory where those strings do not exist.
- Whether the fixture runs under `set -e` (lines 1-20 not read). This does not change the result, because the mutation does not change the exit code of `job watch`.
- The bed and the mutation were not run (the brief forbids it).

Tool calls used: 8 of 8.
