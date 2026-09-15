# Leaked processes on the seat machine: diagnosis

Goal: `seat-machines-shed-leaked-processes` (priority 1, sequence 11). Written by seat m1e on 2026-09-15 from live evidence gathered between 08:30 and 09:05 CEST while three seats worked; the raw capture is `records/misc/leaked-processes-evidence-2026-09-15.txt`. This page is the diagnosis the goal's first step asks for. It names each leak class, its owner, its creation path, why teardown misses it, and the code locations; it ends with what the fix must cover, so that the design page can be written against it. It is not the design.

## 1. What was measured

Load and CPU. At 08:30 the load average was 37 on a machine with three seats running; at 08:46 it was 18 to 27; at 09:00, after the two engine section runs ended and m1b's busy shell was killed, 7.5; after the fixture-orphan reap, 6.0. The leaked processes themselves were idle (0.0% CPU on every sampled steward and broker). The CPU came from four places: one seat shell spinning at 65 to 70% of a core for twelve hours; the concurrent engine runs and race suites (Go compiles bursting past 200%); macOS churn services (fseventsd 37%, mds_stores 9.5% averaged over ten days with a 937 MB store, XprotectService 6% over four and a half hours); and WindowServer plus Terminal at 27 to 33%, rendering five seat terminals. The leaks' cost is memory and churn, not cycles: they held about 24 GB resident and kept the machine paging, which is what made engine policy timeouts, deadline scenarios and 15-to-36-second stop hooks red.

Processes whose owners had ended, counted at 08:46:

| Class | Count | Resident | Oldest |
| --- | ---: | ---: | --- |
| `codex app-server` | 68 | 7.7 GB | 2 days |
| `app-server-broker.mjs serve` (67 reparented to launchd) | 66 | 2.4 GB | 2 days |
| codex-security MCP helpers (`node .../codex-security/0.1.24/mcp/server.mjs`) | 185 | 7.8 GB | 2 days |
| `codex-code-mode-host` | 70 | 1.3 GB | 2 days |
| ChatGPT `node_repl` helpers | 185 | 2.1 GB | 2 days |
| fixture stewards (`... steward run --repo <tmp root>`), parent launchd | 78 at 08:46, 109 at 09:03 | 2.1 to 3 GB | 4 days |
| fixture fakes under those roots (`util hold`, `hosts/fake.sh start-turn`) | 6 | small | 3 days |
| one supervision-hook fixture pair (`supervision-hook.sh claude stop` waiting on `deadline-engine steward hook-attempt`) | 2 | small | 11 hours |
| one seat shell (m1b, pid 53724) at 65 to 70% CPU | 1 | small | 12 hours |

The Codex family alone is 574 processes and about 21 GB. Of the 66 brokers, 14 served m1e worktrees, of which three no longer existed on disk; 27 served m1c scratchpad worktrees (two with a live job); none served m1b, which has run no delegate job since 09-12.

Disk, per seat checkout, at 09:00:

| Store | m1e | m1b | m1c |
| --- | --- | --- | --- |
| `artifacts/agents/suite-failures` (preserved evidence, whole worktree copies) | 102 entries, 7.5 GB | 75, 9.3 GB | 48, 6.6 GB |
| `artifacts/agents/steward/engine-pins` (one engine binary per pin) | 118, 3.0 GB | 188, 4.6 GB | 111, 2.7 GB |
| `artifacts/agents/proof-runs` | 116, 220 MB | 89, 269 MB | 47, 103 MB |

The user temp directory (`$TMPDIR`, under `/var/folders/.../T`) held 24 GB in 6,451 top-level entries, 4,773 of them fixture-shaped; 586 were `tmp.*` roots: 99 from today, 139 one day old, 186 two, 142 three, 15 four, 4 five, 1 seven days old. Suite-failure entries in m1e were 31 from today, 8, 25, 34 and 4 from the previous four days.

Indexing. Spotlight does not index the temp directory (0 items under it), but it does index the checkouts: 63,410 `.go` files were indexed under `agentic-tools-m1e/metasystem` against 1,115 tracked, and 37,089 of them sit under `suite-failures`. Every preserved failure and every pinned engine is new content for mds, fseventsd, XProtect and syspolicyd.

Growth rate. Between the 08:46 census and the 09:03 census, C1b's 29-section engine run and one m1c run finished, and the orphaned-steward count went from 78 to 109: about fifteen orphans per full section run. Five known trunk reds recur on every run (the land-fixtures rc-141 scenarios, adoption filled-delivery, dispatcher mission-runner, goal-cli power-of-attorney, suite-progress exit 64), so every run also preserves about five worktree copies.

## 2. Root cause per class

### A. Codex brokers, app-servers and their helpers

Owner: the delegate lane (codex-rescue and the seats' `codex-companion.mjs task ... --background` jobs). Plugin: openai-codex 1.0.6 under `~/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts`.

Creation path. `lib/broker-lifecycle.mjs:113 ensureBrokerSession(cwd)` keeps one broker per working directory: it returns the existing session record for that cwd or spawns `app-server-broker.mjs serve --cwd <cwd> --endpoint unix:<session dir>/broker.sock` with `detached: true` and `child.unref()` (lines 59 to 67), so the broker reparents to launchd at once. Each broker starts one `codex app-server`, which starts a code-mode host, a codex-security MCP helper and node_repl helpers; that is the five-process tree per worktree seen in the census. `codex-companion.mjs:673` spawns the job's `task-worker` the same way, detached and unref'd.

Why teardown misses it. `app-server-broker.mjs` exits on exactly three events: a `broker/shutdown` request over its socket (line 158), SIGTERM and SIGINT (lines 236 to 243). It has no idle timeout, no client count, and never checks that its cwd still exists. The only code that sends `broker/shutdown` or calls `teardownBrokerSession` is `session-lifecycle-hook.mjs handleSessionEnd` (lines 83 to 106), and it tears down the broker of `input.cwd`, the Claude session's own working directory, at session end. A delegate job runs in a worktree (`--cwd <worktree>`), so its broker is keyed to a cwd the session-end hook never looks at; and seats here run for days without a session end. Every worktree ever used for a delegate job therefore keeps its five-process tree until someone kills it, including after the worktree is deleted (three of m1e's fourteen). A cancel is cwd-keyed as well: m1c reports that `codex-companion cancel` run from the main checkout answers "No job found" and only works from the worktree, so a cancel from the wrong place leaves the job and its runtime alive.

### B. Fixture stewards and fakes

Owner: every fixture bed under `scripts/agents/*-fixtures.sh` that arms supervision (supervision, supervision-hook, land, health, goal-cli, and the harness beds), and the proof launcher that runs them.

Creation path. A bed brings a fixture repository up through the engine; `internal/steward/runner.go:775` launches the runner (`<engine pin> steward run --repo <root>`) with `SysProcAttr{Setsid: true}`: its own session and process group, and once the arming verb returns, launchd is its parent. `cmd/metasystem/steward_verbs.go:538 runStewardRun` calls `steward.RunLoop`, which stops on the runner stop file written by disarm and on the process-creation fence (`runner.go` lines 110 to 160), and on nothing else: no tie to a host or parent pid, no check that `--repo` still exists, no maximum lifetime. In fixture mode `steward run` may even ignore SIGTERM (`METASYSTEM_STEWARD_RUNNER_IGNORE_TERM`, fixture-only). The pinned binary lives inside the fixture root, so a leaked steward also pins its root.

Why teardown misses it. Bed cleanup is trap-based: `supervision-fixtures.sh:558-560` traps EXIT, INT and TERM, and `cleanup()` (line 493) disarms every harness root through `arm-supervision.sh --shutdown` and kills its `owned_pids`. That runs on a normal exit and on a trapped signal. It does not run when:

1. The proof launcher expires a cap. `internal/proofrun/launcher.go:213` puts the suite in its own process group and lines 231 to 330 kill that group with SIGKILL. SIGKILL runs no trap, and the stewards are outside the group (Setsid), so the bed dies and its stewards survive. Every capped or cancelled engine run leaks the stewards of the sections it was running.
2. A Go test times out. The test binary panics and reaps nothing it armed; m1b lists three such timeouts on 09-12 and 09-14 (`TestStopParentAbsoluteCutoffs` starts workers).
3. A seat stops a task by hand (TaskStop and `kill -9` are SIGKILL), or a Codex sandbox job dies under its parent.
4. A harvest run keeps failing legs alive by design: m1b's harvest procedure rewrites `exit 1` into a failure counter, so a failing leg runs past its own teardown and keeps `rm -rf "$tmp"` from running (harvest27, 28, 29, 29b on 09-14 evening; roots `.fEHVv5`, `.AMTMlK`, `.nfYAQB`, `.cA5Bed`).
5. A scenario dies of SIGPIPE (rc 141, the five land-fixtures scenarios on every run): the scenario shell dies without its trap; the bed's own trap still runs, so this leaks roots more than stewards.

Two more shapes of the same class, found by m1b after the reap: a `metasystem channel fake serve --dir .../brain-fake` from a supervision-hook bed run in a clean environment under `/private/tmp` (8 hours old; the hook bed's fake channel server outlives the bed, and a filter keyed on the temp directory or on `steward run` does not see it), and a generation-1 steward five days old pinned to the m1b repository root rather than the metasystem directory, from the previous layout, which no current disarm path addresses. The janitor must key on ownership, not on a process name or a directory.

The eleven-hour hook pair comes from harvest29b's root `.cA5Bed`: `supervision-hook.sh claude stop` waits on the fixture's fake `deadline-engine steward hook-attempt`, whose own 61-second ceiling (`supervision-hook-fixtures.sh:1758-1778`, a `while kill -0 $hook_parent && (( SECONDS - delay_started < 61 ))` loop) did not end it under the harvest's rewritten exits, and the fixture's `stop_hook_process` (lines 135 to 150) only runs on the bed's own path. The class is the same: a fake with a wait loop whose bound depends on the caller's shell state.

Measured consequence: fifteen orphaned stewards per full section run, roots that stay on disk with the pinned engine inside them, 24 GB in the temp directory.

### C. Seat waiters

Owner: the seat's own shell commands (Claude Code Bash tool) and the harness that runs them.

m1b's shell, pid 53724, ran this from 20:49 on 09-14 until killed at 09:00 today:

```
root=$(pgrep -f 'scripts/agents/zz-harvest.sh' | head -1); pids=$root; all=$root
while [ -n "$pids" ]; do next=""; for p in $pids; do c=$(pgrep -P $p | tr '\n' ' '); next="$next $c"; all="$all $c"; done; pids=$(echo $next); done
```

The Bash tool runs commands under zsh. `next="$next $c"` always adds a space; in bash `pids=$(echo $next)` word-splits that to empty and the loop ends, in zsh an unquoted `$next` is not split, `pids` stays a single space, `[ -n "$pids" ]` stays true, and every pass forks pgrep, tr and echo: 65 to 70% of a core, no matter what the process tree looks like. Two things then let it run for twelve hours: the tool's 120-second timeout did not kill the call but moved it into a background task, and nobody read that task again. Seat m1c has no such shell; the same check on m1e found none.

A smaller symptom of the same family: the openai-codex plugin's SessionStart hook re-exports its environment block on every session start, and after three days and several compactions every Bash call in m1b's session carried the same three `export` lines five times over.

### D. Indexing and scanning churn

Owner: the engine's evidence and pin stores, and the seats' checkout placement.

Spotlight indexes `~/LocalStorage/GitHub/*` and the metasystem writes its highest-churn content there: `artifacts/agents/suite-failures` preserves a whole worktree copy for every red scenario (37,089 indexed `.go` files in m1e alone, 23 GB across the three seats, five new copies per run from the known trunk reds), `artifacts/agents/proof-runs` extracts a candidate tree per run, and `artifacts/agents/steward/engine-pins` writes a new engine binary per pin (417 pins, 10 GB across seats), each a fresh binary for XProtect and syspolicyd to scan. None of the three stores has a retention rule. The temp directory is not indexed, but fseventsd sees every create and delete there regardless, and the 24 GB of leaked roots is churn on every backup, index and scan pass. WindowServer and Terminal at 27 to 33% are the five seat terminals rendering high-volume output; that is not a metasystem process, only a reminder that seat scripts should not stream thousands of lines to the terminal.

## 3. What the fix must cover

This is the brief for the design page, not the design.

1. Named ownership. Every process the metasystem starts or causes has an owner a janitor can read: brokers and app-servers per worktree and job; stewards, fakes and hooks per fixture root; waiters per seat command. Where the plugin owns the process (class A), the metasystem's delegate lane owns the shutdown: at a job's terminal state, and on cancel, the worktree's broker is shut down (`sendBrokerShutdown` on the cwd's session record, or SIGTERM to the pid in its pid file), and a worktree removal shuts its broker first.
2. Bounded life. A process without a live owner ends on its own: a fixture-mode `steward run` exits when its `--repo` root is gone or its host pid is dead, and carries a hard lifetime (`fixtureauth.FixtureModeRoot` already distinguishes fixture roots); fixture fakes keep their own deadline independent of the caller's shell state; the proof launcher reaps what a section armed, not only the section's process group, by reading a run-scoped registry of armed pids that beds append to (or by killing the fixture root's processes by cwd) after its group kill.
3. Census and janitor. One verb lists every metasystem-caused process by owner and age and flags the orphans (owner gone, root gone, or past its bound); a reap verb ends them and retires their roots. The engine runs it after every proof run, a seat runs it at session start and before a load-sensitive run, and it refuses nothing that is younger than its bound or has a live owner. The goal's proof is a day of three-seat work ending with a zero-orphan census and a lower load.
4. Retention and placement. `suite-failures`, `proof-runs`, `engine-pins` and fixture temp roots get a count-and-age retention rule, and preserved evidence and candidate trees live where Spotlight does not look (a directory whose name ends in `.noindex` is skipped by Spotlight; the temp directory already is), so that a red scenario costs one preserved copy for a bounded time, not a permanent indexed tree.
5. Seat waiters. No seat polling loop without a deadline, and no backgrounded timed-out call left unread; the tool's timeout-to-background behaviour is Claude Code's and goes upstream as a report, the seat rule goes into the seat instructions.
6. Cancel from anywhere. A delegate cancel finds the job by id regardless of the caller's cwd.

Existing goals this subsumes or touches: `fixture-stewards-outlive-their-suite` (class B, to be folded into this goal), `claude-delegate-scratch-cleanup` (worktrees and their brokers), `run-scoped-build-caches-have-a-janitor` (retention of pins and caches overlaps item 4).

## 4. Interim measures taken on 2026-09-15

m1b killed its busy shell at 09:00. m1e reaped 97 fixture orphans older than three hours (stewards, fakes, the hook pair; SIGTERM sufficed, no survivor): orphaned stewards went from 109 to 15 (the rest younger than the bound) and the one-minute load from 7.5 to 6.0 at that moment; it climbs back past 13 with every engine rebuild, which is the ordinary cost of three seats, not the leak. m1b's two late finds (the fake channel server and the old-layout steward) were reaped afterwards. Codex runtimes were left to their seats: m1c reaps the 25 brokers of its finished worktree jobs, m1e its own; nothing on disk was removed, so the roots and preserved evidence remain as evidence for the build. A first reap attempt by m1e killed nothing (the pid list was not word-split under zsh, the same class-C mistake), which is recorded in the evidence file.
