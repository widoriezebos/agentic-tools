# seat-machines-shed-leaked-processes

- Owner: m1e (goal seat-machines-shed-leaked-processes, priority 1, sequence 11, tier 3). **Revision 2, 2026-09-15**, written by the Claude Fable design delegate from critique round 1 (19 material findings, verdict rework; the record is section 15), the seat's fold notes, and Wido's four decisions of 2026-09-15 (D1 to D4 below). Revision 1 was written the same day from the diagnosis record (records/misc/leaked-processes-diagnosis-2026-09-15.md, raw capture records/misc/leaked-processes-evidence-2026-09-15.txt) against origin/main 6f747033; this revision is checked against origin/main ad82057c, which carries c15d23be (every Go test package reaches `internal/testenv.Main` through a `testmain_test.go`). The seat integrates the page and edits only this header block.
- What changed in revision 2: (1) the Codex broker lane is a metasystem verb, `metasystem delegate codex`, that launches, waits, cancels by job id from any directory, and shuts the job's broker at its terminal state and on cancel behind a quiescence handshake; the janitor's broker role is a backstop for a gone worktree or an empty plugin job store (D1, SMLP-01, -03, -06, -07, section 3). (2) A path under a bed root is scope only; ending a process needs a shipped shape plus an owner proof (SMLP-02, section 2.3). (3) Every record carries a full `identity.Ref`; the steward's record and ladder become exact on Darwin before any unit ends a steward (SMLP-04, section 4.1). (4) Fixture authority comes from an authenticated enrollment or a bed owner, never the config predicate, and `arm` no longer rewrites a human enrollment to fixture on a fake-runtimes root (SMLP-08, section 4.2). (5) The unrecorded old-layout steward and the mid-exit handover case are rules (SMLP-10, fold notes, section 4.3). (6) Bed roots are indexed in a durable machine index under registry framing, the run registry is fenced to one launcher identity, the launcher's reap is one deferred obligation on every post-start return, and a fixture arm refuses without a marker (SMLP-05, -12, section 5). (7) The channel fakes of two more beds and the fake adapter's hold children are covered; the fixture-only fakes always require a deadline (SMLP-09, section 6). (8) A busy seat shell with no child for thirty minutes is ended by the engine, logged and reported to its session (D2, SMLP-11, section 7). (9) The sightings file is a registry with a schema, one lock across read, merge and replace, recovery and a concurrent-update witness (SMLP-15, section 8.5). (10) Retention deletes nothing without an expiry rule plus a live-use pin, suite-failures pruning stays off until the evidence root is configured, the legacy temp roots are removed once by the seat by hand (D3), and placement covers proof payloads and engine pins with the path consequences handled (SMLP-13, -14, section 9). (11) Seventeen units of at most 300 allocated lines with complete Boundaries and builder-runnable witnesses, brokers first (SMLP-16, -19, section 11). (12) The proof runs on a retained per-tick census stream after the last unit, with per-class thresholds and a stated load comparison (SMLP-17, section 13). (13) fixture-stewards-outlive-their-suite is folded now; the budget is estimated in section 14 and no unit is authorized until Wido approves it (D4, SMLP-18).
- Goal and current status: the diagnosis is landed; this page is the design; nothing is built.
- In flight right now: this revision awaits its critique read.
- Decisions made (and who made them): Wido, 2026-09-15: D1 the broker stop owner is a metasystem verb, the janitor is a backstop; D2 the waiter clause becomes engine-enforced (a seat shell burning CPU with no child for 30 minutes is ended, logged, reported); D3 the 586 unmarked legacy roots are removed once, now, by the seat under the machine lock after a no-live-process check; D4 fixture-stewards-outlive-their-suite is folded now, the budget is sized after this revision's unit list is critiqued. The seat, 2026-09-15: all 19 findings material; the ordering, causation, identity, launcher, fixture-authority, coverage, retention, placement, ledger, unit-size and proof rulings recorded in section 15. This page: the mechanisms of sections 2 to 9.
- Waiting on the human: the budget of section 14 (D4). Nothing else.
- Dead ends (do not retry without new evidence): a pattern kill by name (docs/orchestration.md, Shared Machines); killing supervision components before their owner (KI-32); a filter keyed on the temp directory or on `steward run` (missed the fake channel server and the old-layout steward); a reap written for bash and run under zsh (the no-op reap of 2026-09-15 07:01Z); age in days as liveness; broker ownership read from the broker's environment (revision 1, refused by SMLP-01: the environment names the session that first spawned a broker another session may be using); location as causation (revision 1, refused by SMLP-02); a launcher reap placed after the common wait (revision 1, refused by SMLP-05: several kill paths return earlier); a legacy-roots prune verb (revision 1, replaced by D3).
- Next step: unit 1 (the `delegate codex` records and launch) waits on Wido's approval of the budget in section 14; then it is briefed to Codex gpt-5.6-sol and read by Opus.

The goal's box, from its record: elapsed 1d, 10 attempts, 1200 reserved job minutes, 1 active job, 3 review rounds. `metasystem.budget.*` keys are not touched by this design. Source citations are relative to `metasystem/` unless they name the plugin under `/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/`. Uncited test names, verbs and config keys are proposed. Allocations are estimates with margin, never measured proof.

## 1. The problem, measured

On 2026-09-15 between 08:30 and 09:05 CEST, with three seats working, the shared machine ran at a load average of 37, then 18 to 27, and held about 24 GB resident in processes whose owners had ended. The Codex plugin family alone was 574 processes and about 21 GB: 68 `codex app-server` processes (7.7 GB, the oldest two days old), 66 `app-server-broker.mjs serve` processes (67 reparented to launchd), 185 codex-security MCP helpers (7.8 GB), 70 `codex-code-mode-host` processes and 185 `node_repl` helpers; of the 66 brokers, 14 served m1e worktrees of which three no longer existed on disk, and 27 served m1c scratchpad worktrees of which two had a live job. Fixture stewards (`<engine pin> steward run --repo <temp root>`, parent launchd) numbered 78 at 08:46 and 109 at 09:03, 2.1 to 3 GB, the oldest four days old; one 29-section engine run and one m1c run finished in between, so a full section run leaks about fifteen stewards. Six fixture fakes (`util hold`, `hosts/fake.sh start-turn`) and one supervision-hook pair eleven hours old survived beside them, and one generation-1 steward five days old was pinned to the m1b repository root of the previous layout. One seat shell (m1b, pid 53724) spun at 65 to 70 percent of a core for twelve hours in a zsh loop that never terminated. On disk the three seat checkouts held 225 preserved worktree copies under `artifacts/agents/suite-failures` (23.4 GB), 417 pinned engine binaries under `artifacts/agents/steward/engine-pins` (10.3 GB) and 252 proof-run entries (592 MB); the user temp directory held 24 GB in 6,451 entries, 586 of them `tmp.*` bed roots up to seven days old. Spotlight had indexed 63,410 `.go` files under the m1e metasystem directory against 1,115 tracked, 37,089 of them under suite-failures. The leaked processes were idle; their cost was memory, paging and churn, which turned engine policy timeouts, deadline scenarios and 15 to 36 second Stop hooks red. The hand reap of 97 fixture orphans needed only SIGTERM: none was wedged, none had been signalled.

## 2. The ownership model

### 2.1 Owners and records

Every process the metasystem starts or causes belongs to one owner of one of these kinds, and a janitor reads the owner from a record, never from the process's own claim:

| Owner kind | Owner id | Root | Record, and who writes it |
| --- | --- | --- | --- |
| `codex-job` | the plugin job id | the job's working directory (realpath) | `artifacts/agents/codex-jobs/<jobId>.json` under the installation, written by `metasystem delegate codex launch` or `adopt` (rules K1, K7); indexed machine-wide in `~/.metasystem/janitor/codex-jobs.jsonl` |
| `bed` (a fixture bed) | the bed's full process identity | the bed's temp root | `<root>/.metasystem-bed.json`, written by `metasystem janitor bed-open` (rule M1); indexed machine-wide in `~/.metasystem/janitor/beds.jsonl` |
| `run` (a proof run) | the launcher's full process identity | the run's registry file | `<registry>/beds.jsonl`, appended by `bed-open` with the launcher identity the launcher exported (rule M1) |
| `steward` (a runner's own record) | the runner's full process identity | `--repo` | `<repo>/artifacts/agents/steward/runner.json`, written by the runner (rule X1); it names the runner and, under a bed, the bed owner (rule M3) |
| `session` (a seat session) | the session id | the checkout that announced it | `artifacts/agents/mains/*.json`, written by `metasystem up --session` today (`internal/up/up.go:659`) |
| `job` (a delegate job) | the job id | the job worktree | `artifacts/agents/jobs/<job>.json`, written by dispatch today (pid, start, instance tag through the ownership patch) |
| `human-enrollment` | the enrolled checkout | the checkout | the steward identity record, minted by `steward arm` at an agent-free terminal today |

Each record stores a full `identity.Ref` (pid, `pidStartedAt`, `pidStartedAtMicro`, `pidStartTicks`, `bootId`, the spelling `stopfence.Process` already uses, `internal/stopfence/fence.go:33-47`), so the record is exact on Darwin (microseconds) and on Linux (start ticks plus boot id) and never falls back to whole seconds where an exact shape exists (`internal/identity/identity.go:72-93`). The two machine-wide indexes use the registry's framing (`internal/registry/framing.go`: one JSON object per line, trailing newline, torn markers, `ReadFrames`) and `LockedAppend` (`internal/registry/append.go:20`), and their home follows `METASYSTEM_SUPERVISION_REGISTRY_HOME` exactly as `armed-checkouts.jsonl` does (`internal/registry/selection.go:13-26`), so fixtures and `testenv.Main` never touch the seat's files. The janitor's third machine-wide file, the sightings registry of section 8.5, follows the same rules.

Why the existing census cannot carry this: `internal/census` answers "which agent-signature processes are in this checkout's scope" for one checkout (`internal/census/run.go:209-260`) and its verdict gates dispatch. The leak question is machine-wide across temp roots and worktrees, and it needs owner records the census never reads. The janitor census of section 8 is a second walk over the same process table (`census.EnumerateProcesses`, `internal/census/production.go:45-76`, extended with parent and CPU readers by rule W2) with the owner model above; the supervision census and its verdict are untouched.

### 2.2 Identity, three-way

Before any action the janitor probes the target with `identity.AliveRef(prober, ref)` (`internal/identity/identity.go:187-217`): a reused pid compares unequal on its exact start and reads Dead; an unreadable process reads Unknown; only Alive with a matching identity is signalled, and only a Dead owner, or a record that definitively names another process, makes an orphan. Unknown never authorizes anything (`identity.go:8-13`). The fixture runner's self-check (B2), the launcher's reap (M4), the broker release (K5) and the waiter action (W5) all reuse `identity.KernelProber{}` behind an injected `Prober`, and every process-ending path takes an injected `Signal` and `Clock` so its tests run on artificial time (R-104-m1e).

### 2.3 Causation: a shape plus an owner proof, never a path

- **O1 (a path is scope, never causation).** A process whose cwd or argv path lies under a bed root, a worktree or a checkout is in the scope of that root's owner for listing, exactly as the supervision census treats scope (`internal/census/run.go:218-244`). Scope never authorizes a signal. Witness: `TestCensusScopeNeverAuthorizesAction` (an unshaped process with cwd under a dead bed's root is listed as `outside-world` and `reap` refuses it with `not-caused`).
- **O2 (ending a process needs a shipped shape and an owner proof).** A process may be signalled only when its argv matches one shipped positioned shape (the existing `janitor.Shape` list, `internal/janitor/killproof.go:41-63`, extended by the table in section 8.2, each with a tag or path flag in a defined position, matched by `MatchShape`) and the shape's designated record proves the owner definitively: the bed marker for fixture shapes, the runner record for stewards, the codex-job record or the plugin job store for brokers and companions, the announced main for waiters. A shape match without the record's proof is `owner-unknown` and is refused. Witness: `TestReapRequiresShapeAndOwnerProof` (a shaped process whose designated record is unreadable is refused; the same process with a record proving a dead owner is reaped under the fake signal).
- **O3 (group members follow a proven leader).** A process whose process group leader is a shaped, owner-proven process, and whose own argv matches no shape, is that leader's member: it is listed with the leader's owner and signalled only through the leader's group signal (`-pgid`), never on its own. The broker's `codex app-server`, MCP helpers, `node_repl` and code-mode host are such members (the broker is spawned detached, so its pgid is its pid, and the plugin's own `terminateProcessTree` signals `-pid`, `lib/process.mjs:100-117`). Witness: `TestGroupMembersAreSignalledOnlyThroughTheLeader`.
- **O4 (what the census sees).** The census lists: every shaped process (owned, orphan or unknown), every member of a shaped leader's group, and, as `outside-world`, unshaped processes whose cwd or argv lies under a root the janitor knows (a bed root, a codex-job cwd, a checkout's worktrees). Everything else is invisible. Witness: `TestCensusWorldMembership` (a table over the four classes).

### 2.4 Existing machinery this builds on, and what stays untouched

| Owner today | Decision |
| --- | --- |
| `identity.KernelProber`, `AliveRef`, `Compare`, `AllPids`, `ProcessCwd`, `ParentPid` (`internal/identity`) | Reused; two readers added (CPU time, W2; environment is not read, SMLP-01). |
| `census.EnumerateProcesses`, `ResolveCwds` (`internal/census/production.go`) | Reused as the janitor's table; the production row gains its parent (`census.Process.PPID`, today 0, `production.go:66`). The supervision census verdict is untouched. |
| `janitor.Shape`, `MatchShape`, `DefaultShapes`, `GroupOwnership`, `Killable` (`internal/janitor/killproof.go`) | Extended with the new shapes; the positioned-tag proof is the causation rule. `SelectTargets` (`targets.go`) keeps its registry world and is untouched. |
| `registry.AppendFrame`, `LockedAppend`, `ReadFrames`, torn markers (`internal/registry`) and `lock.Acquire` (`internal/lock`) | Reused for the three janitor files (bed index, codex-job index, sightings). |
| `steward.RunLoop`, `RunnerRecord`, `liveRunner`, `sameRunner`, `Disarm` (`internal/steward/runner.go`) | Made exact (X1, X2); the loop gains guards (B0 to B5); the record gains an owner (M3); `arm` stops rewriting enrollment from the config predicate (B0). `EnsureRunner`'s exclusion (`runner.go:419`) is untouched. |
| `proofrun.LaunchSuite`, `RunWatchdog`, `SignalAuthenticated`, the `Prober` and `Signal` seams (`internal/proofrun`) | One deferred reap obligation in the launcher (M4), the same call in the watchdog's stall path (M5). |
| `supervise.ShutdownAt` (`internal/supervise/arming.go:1232`) through the root's `arm-supervision.sh --shutdown` | Reused per repository under a bed root, as the beds' cleanups do today. |
| `stoptransition.LocalFamilies` (`metasystem stop`) | Untouched. |
| `steward.RunTick`, health roles, `QueueNotification` (`internal/steward`) | One pass, one role and one notification producer added (C7, C8, W5). |
| `cmd/metasystem/delegate.go` `runDelegate` (routed at `cmd/metasystem/main.go:724`) | Routes a leading `codex` word to the new verb before its own flags. |
| The Codex plugin (`app-server-broker.mjs`, `lib/broker-lifecycle.mjs`, `session-lifecycle-hook.mjs`, `codex-companion.mjs`, `lib/state.mjs`) | Not modified. Used as it is: `task --background --json --cwd --prompt-file`, `status <id> --cwd --json --wait`, `status --cwd --json --all`, `cancel --cwd <id> --json`, the `broker/shutdown` request. |

## 3. The Codex broker lane (D1)

### 3.1 The verb

`metasystem delegate codex <launch|status|wait|cancel|release|adopt>` is the metasystem's owner of a Codex companion job's life and of the broker it leaves behind. `runDelegate` routes the leading `codex` word to it before parsing its own flags.

- **K1 (launch writes the record first).** `launch --cwd <dir> --prompt-file <f> [--model M] [--effort E] [--write] [--resume-last] [--json]` resolves `<dir>` to its realpath, takes the per-cwd lock (`<installation>/artifacts/agents/codex-jobs/locks/<sha256(cwd) first 16 hex>.lock.d` through `internal/lock`, wait 10 s scaled by `METASYSTEM_FIXTURE_CAP_SCALE_MILLI` as `stopfence.Acquire` scales, `internal/stopfence/fence.go:254-277`), runs `node <plugin>/scripts/codex-companion.mjs task --background --json --cwd <dir> --prompt-file <f> ...` (the plugin's own grammar, `codex-companion.mjs:762-806`), reads `jobId` from the JSON payload, writes `artifacts/agents/codex-jobs/<jobId>.json` (`{schemaVersion:1, jobId, cwd, installation, plugin, launchedAt, launcher: <identity.Ref of this verb>, session: <CODEX_COMPANION_SESSION_ID or empty>, state: "launched"}`), appends `{jobId, installation, cwd, at}` to `~/.metasystem/janitor/codex-jobs.jsonl` under `LockedAppend`, releases the lock and prints `job=<id> cwd=<dir>`. The plugin root is `--plugin-root`, else `CLAUDE_PLUGIN_ROOT`, else the metasystem.conf key `codex.plugin-root`; none of the three is a refusal `CODEX_PLUGIN_UNSET`. A `task` invocation that fails writes no record. Witness: `TestDelegateCodexLaunchRecordsBeforeReturn` (a fake `node` on PATH answers the JSON; the record and the index line exist with a full `identity.Ref`), `TestDelegateCodexLaunchFailureWritesNothing`.
- **K2 (status).** `status <id> [--json]` finds the record by id through the index from any directory and relays `codex-companion.mjs status <id> --cwd <cwd> --json`. Witness: `TestDelegateCodexStatusResolvesCwdFromTheIndex`.
- **K3 (wait ends at the terminal state and releases).** `wait <id> [--timeout <dur>]` relays `status <id> --cwd <cwd> --json --wait --timeout-ms <n>` (the plugin's own wait, `codex-companion.mjs:892-897`) until the job's status is `completed`, `failed` or `cancelled` (`lib/tracked-jobs.mjs:156-200`), stamps `state: "terminal", terminalStatus, terminalAt` on the record, then runs `release --cwd <cwd>` (K5) and exits 0 for completed, 1 for failed or cancelled, 3 on timeout with the job still running and nothing released. Witness: `TestDelegateCodexWaitReleasesAtTerminal` (a fake plugin returns running twice under the injected clock, then completed; the release handshake is recorded once), `TestDelegateCodexWaitTimeoutReleasesNothing`.
- **K4 (cancel by id from any directory).** `cancel <id>` finds the record through the index, runs `codex-companion.mjs cancel --cwd <cwd> <id> --json` (the plugin resolves the workspace from `--cwd`, `codex-companion.mjs:963-972`), stamps `state: "cancelled"`, then runs `release --cwd <cwd>`. A job without a record (launched by the plugin directly) is cancelled with `cancel <id> --cwd <dir>`, which first runs `adopt` (K7). Witness: `TestDelegateCodexCancelFromAnotherDirectory` (the verb runs with a cwd outside the checkout; the fake plugin receives `--cwd <recorded cwd>`), `TestDelegateCodexCancelWithoutRecordNeedsCwd`.
- **K5 (release is a quiescence handshake).** `release --cwd <dir>` ends the broker serving `<dir>` only in this order, under the per-cwd lock of K1, which holds every metasystem launch for the same cwd out of the window: (1) the plugin job store for `<dir>` shows no non-terminal job of any session: `codex-companion.mjs status --cwd <dir> --json --all` run with `CODEX_COMPANION_SESSION_ID` removed from the child environment (the plugin filters by session only when that variable is set, `lib/job-control.mjs:15-25`) must report an empty `running` array; (2) the live process table holds no companion process for `<dir>` (shape `codex-companion`, section 8.2, matched on its `--cwd` value or its process cwd); (3) the broker is found by shape (`app-server-broker.mjs serve --cwd <dir>`) and its full identity is recorded; (4) steps 1 and 2 are repeated, and the identity of step 3 is re-proved, immediately before the first action; (5) `{"id":1,"method":"broker/shutdown","params":{}}` is written to the broker's `--endpoint` socket (`app-server-broker.mjs:160-164`) and the leader is awaited Dead for `TermGrace`; (6) else SIGTERM to `-pid`, `TermGrace`, SIGKILL to `-pid`, `KillGrace`, each preceded by an identity re-proof of the leader; (7) the group is re-read and every survivor is printed by pid and state; (8) the record's `broker` object and the janitor log get the outcome. A store answer that cannot be read (the plugin exits non-zero, or prints no JSON) is Unknown and refuses the release with `STORE_UNREADABLE`; a running companion refuses with `COMPANION_LIVE`; no broker found is a no-op `NO_BROKER`. Witness: `TestReleaseHandshakeOrder` (a fake plugin, a fake table and a recording dialer and signaller: the store and table checks run twice, the socket request precedes any signal, a companion appearing between the two checks refuses, a leader whose identity changed after step 3 receives nothing), `TestReleaseRefusalsAreNamed`.
- **K6 (the residual window, stated).** A caller that bypasses the verb and starts a plugin job in the same cwd between step 4 and step 5 (one process spawn wide) can find its broker gone on its first request; the plugin's `ensureBrokerSession` then tears the stale record down and respawns (`lib/broker-lifecycle.mjs:113-160`), and that request fails once. The verb cannot close this window from outside the plugin; the seat rule of K8 keeps plugin-direct callers out of worktrees, and the upstream report of K9 asks the plugin to refuse `broker/shutdown` while a request or stream socket is active (it already tracks them, `app-server-broker.mjs:170-175`). Witness: none (a stated limit, not a rule).
- **K7 (adopt).** `adopt <id> --cwd <dir>` writes a record for a job the plugin launched directly (the `codex:codex-rescue` agent, which may only call `task`, `agents/codex-rescue.md`), after `status <id> --cwd <dir> --json` confirms the job exists; from then on K2 to K5 apply. Witness: `TestDelegateCodexAdoptRequiresAnExistingJob`.
- **K8 (how the rescue agent is covered).** The seat rule (docs/orchestration.md, Supervising Long Runs, added by unit 17): a Codex job in a worktree or scratchpad directory is launched with `metasystem delegate codex launch` and ended with `wait` or `cancel`; the `codex:codex-rescue` agent is used only in the seat's main checkout, whose broker the plugin's own SessionEnd hook tears down (`session-lifecycle-hook.mjs:83-114`); a rescue job that was started in a worktree anyway is adopted with `adopt` before its result is read. What the engine enforces without the rule: the janitor backstop (K10). Witness: `TestSeatInstructionsNameTheCodexVerb` (the three sentences are present; the remove-one run deletes one).
- **K9 (the upstream report).** The seat files one record, `records/misc/codex-plugin-upstream-report-2026-09.md`, naming what the metasystem cannot fix from outside: no idle timeout in the broker; `broker/shutdown` accepted while a request or stream is active; `cancel` resolving jobs by the caller's workspace; SessionEnd tearing down one cwd's broker only; SessionStart re-exporting its three variables on every start (five copies after three days, diagnosis section 2.C); and, from section 7, Claude Code's timeout-to-background behaviour with no turn-end reminder. Witness: `TestUpstreamReportNamesTheSixFacts`.
- **K10 (the janitor backstop).** The tick pass (C7) may run `release --cwd <dir>` for a broker only when (a) `<dir>` is definitively gone (`os.Stat` ENOENT on the realpath; steps 1 and 2 of K5 are then skipped because the store is keyed by that directory, and the broker is ended by identity through steps 3 to 8), or (b) the store check of step 1 and the table check of step 2 both pass and the sightings registry shows the broker first seen without a non-terminal job at least `janitor.broker-idle-min` (default 30, validated 1 through 1440) ago. Session identity plays no part (SMLP-01). Witness: `TestBackstopReleasesOnlyGoneCwdOrIdleEmptyStore` (four rows: gone cwd; empty store and idle 31 minutes; empty store and idle 5 minutes, refused; a queued job in the store, refused).

### 3.2 Answers to the four broker findings

- **SMLP-01 (the environment names the wrong session).** Session identity is no longer an owner input for brokers. A broker is owned by the codex-job record of the job that launched it (K1, K7) and, for the backstop, by the plugin's own job store for its cwd (K10). A broker that a second session reuses is protected by the store check, which lists every session's non-terminal jobs (K5 step 1).
- **SMLP-03 (an unauthenticated shutdown request can end a live job).** The request is sent only after the store and table checks pass twice, the second time immediately before the request, under a lock that holds metasystem launches out (K5). The endpoint is read from the argv of the identity-proved broker at step 3 and the identity is re-proved at step 4. The remaining window, one plugin-direct spawn wide, is stated (K6) and referred upstream (K9).
- **SMLP-06 (cancel by id from anywhere, and shutdown at terminal state, were prose).** `cancel <id>` resolves the cwd from the machine-wide index and calls the plugin with `--cwd` (K4); `wait` releases at the terminal state (K3); both are engine code with witnesses. The seat rule (K8) says which verb to use; it is no longer the mechanism.
- **SMLP-07 (the job id was discarded and idle had no owner check or hard bound).** The companion shape carries its `--job-id` and `--cwd` into the census (section 8.2); the store check reads the plugin's job status by id (K5 step 1); idle is measured from the sightings registry (S1) and acted on by K10; no unit stubs it: unit 2 owns the handshake, unit 14 the backstop.

## 4. Fixture stewards

### 4.1 Identity exactness first

- **X1 (the runner record is a full Ref).** `RunnerRecord` (`internal/steward/runner.go:41-49`) gains `pidStartedAtMicro` and writes all five identity fields from `self.Ref()` at `RunLoop` start (`runner.go:141-148`); readers accept the old shape (seconds plus the Linux pair) as the labelled legacy mode. Witness: `TestRunnerRecordCarriesFullRef`.
- **X2 (the ladder compares exactly).** `liveRunner` (`runner.go:1009-1030`), `sameRunner` (`:917-925`) and therefore `Disarm` (`:942-1005`) compare through `identity.Compare(exact, record.Ref())`, so a Darwin record with microseconds never matches a pid reused within the same second; a legacy record compares by seconds and the disarm output names the mode (`identity.ComparisonMode`). Witness: `TestDisarmRefusesAPidReusedWithinTheSecond` (a fake prober answers the same pid with a start one microsecond later; TERM is not sent).
- **X3 (every janitor record embeds a full Ref).** The bed marker, the two indexes, the codex-job record and every sightings frame carry `stopfence.Process` fields; a record whose ref has neither exact shape and no seconds is `CompareInvalid` and is refused as Unknown. Witness: `TestJanitorRecordsRefuseInvalidRefs`.

### 4.2 The runner bounds its own life

Today `RunLoop` stops on the stop file and the fence and on nothing else (`runner.go:167-218`); the runner is launched with `Setsid` (`runner.go:775`). `RunLoop` gains a `RunnerGuards` value (fixture authority, owner ref, lifetime, `Clock`, `Prober`, `RootExists`), built by `runStewardRun` (`cmd/metasystem/steward_verbs.go:538-574`) and injected by tests.

- **B0 (fixture authority is authenticated).** A runner is in fixture mode when, at `RunLoop` start, either the root's identity record verifies (`VerifyIdentity`) with `Enrollment == "fixture"`, or a bed marker above the root (M3) names an owner whose identity the runner can probe at start. The config predicate `fixtureauth.FixtureModeRoot` alone never makes fixture mode. `arm` stops rewriting an enrollment to fixture from that predicate (`runner.go:662-664`): the enrollment is `fixture` only when the caller's HUMAN classification came from the fixture table (`ArmFixture`, `RestartFixture`) or a bed marker sits above the root; a `steward arm` typed at an agent-free terminal on a root whose conf says `metasystem.runtimes=fake` mints `human-terminal` and its runner is unbounded. `runnerExclusion` (`runner.go:578-589`) keeps the config predicate for its own purpose, exclusion. Witness: `TestArmAtHumanTerminalOnFakeRuntimesRootMintsHumanEnrollment` and `TestRunLoopGuardsHumanEnrollmentOnFakeRuntimesRootIsUnbounded` (the human-enrolled runner on a fake-runtimes root with a dead owner, a removed root and an elapsed lifetime keeps ticking under the injected clock).
- **B1 (root gone).** A fixture-mode runner checks every guard interval of 5 seconds inside the 200 ms stop-file wait (`runner.go:212-217`) that `--repo` still exists; ENOENT ends the loop with reason `root-gone` on stderr and in the janitor log; any other error keeps it running. Witness: `TestRunLoopExitsWhenRootGone`.
- **B2 (owner dead).** A fixture-mode runner with a bed owner probes it every guard interval; Dead ends the loop with reason `owner-dead`; Unknown keeps it running. Witness: `TestRunLoopExitsWhenBedOwnerDies`, `TestRunLoopKeepsRunningOnUnknownOwner`.
- **B3 (hard lifetime).** A fixture-mode runner ends with reason `lifetime` when the clock passes its start plus `steward.fixture-runner-lifetime-min`, a metasystem.conf key with default 180, read as `resolveProofRunLimits` reads its keys (`cmd/metasystem/proof_run.go:908-945`) and validated as an integer from 1 through 1440 by a new bounded-knob table in `internal/config/validate.go` beside the positive-integer loop at lines 501 to 511 (which validates positivity only). 180 minutes is four times the committed 45 minute section cap and twice the shared Mac's 90. Witness: `TestRunLoopExitsAtFixtureLifetime`, `TestFixtureRunnerLifetimeKeyIsBounded` (0 and 1441 are refused).
- **B4 (no husk).** On any of the three exits the runner removes its record (`runner.go:149`) and exits 0. Witness: `TestRunLoopSelfExitRemovesRecord`.
- **B5 (ignore-TERM does not disable the guards).** Witness: `TestRunLoopGuardsApplyWhileTermIsIgnored`.

### 4.3 Unrecorded and handing-over stewards

- **B6 (an unrecorded runner is an orphan by record).** A `steward run --repo <root>` process whose designated record `<root>/artifacts/agents/steward/runner.json` is absent (ENOENT) or names another identity is `orphan reason=unrecorded`; the owner proof is the record's definitive absence or mismatch, never the process name. This covers the five-day generation-1 stewards pinned to a repository root of the previous layout on m1b and m1c (fold notes): the checkout exists, its installation is `<checkout>/metasystem`, and no record in the current layout names them. The scope is the checkout whose toplevel contains `<root>`. Their ladder is TERM, `TermGrace`, KILL by identity (not `Disarm`, which acts on the record's runner). Witness: `TestUnrecordedRunnerIsAnOrphanByRecord` (a fake table with a runner shape whose root's record names another pid; and one whose record is absent).
- **B7 (a handover is not a leak).** An unrecorded runner whose root's record was written within `janitor.handover-grace-sec` (default 120, validated 10 through 3600) reads `handover` and is never reaped inside that grace (a steward of the immediately previous generation still exiting after `up` re-armed, fold notes). Witness: `TestHandoverGraceKeepsThePreviousGeneration`.

## 5. Beds, markers and the launcher

- **M1 (the bed marker and the two registries).** Every bed writes, immediately after `mktemp -d` and before anything arms or spawns, `<root>/.metasystem-bed.json` through `metasystem janitor bed-open --root <root> --bed <name> --scenario <name> --pid $$ --source <harness root>`: the verb probes `$$` itself and writes `{schemaVersion:1, bed, scenario, owner: <stopfence.Process>, openedAt, deadlineEpoch, source, run: {registry, launcher: <stopfence.Process>} or null, keep:false}`, where `deadlineEpoch` is the probed start plus `fixture.bed-lifetime-min` (default 180, bounded 1 through 1440), `run.registry` is `METASYSTEM_PROOF_BED_REGISTRY` and `run.launcher` the parsed `METASYSTEM_PROOF_LAUNCHER_REF`, both exported by the launcher (M4). The verb then appends one frame to `~/.metasystem/janitor/beds.jsonl` (`{root, owner, openedAt, source, run}`) under `LockedAppend`, and, when the run registry is set, one frame `{root, owner, launcher, openedAt}` to that registry. The harness helper `harness_fixture_bed_open "$tmp" <bed>` wraps the verb and exports `METASYSTEM_FIXTURE_BED_ROOT` and `METASYSTEM_FIXTURE_DEADLINE_EPOCH`; `harness_fixture_bed_close "$tmp" <status> [keep]` wraps `bed-close`, which stamps `closedAt`, `status`, `keep`. Witness: `TestJanitorBedOpenWritesProbedIdentity` (owner fields equal the kernel's for the caller's pid), `TestJanitorBedOpenAppendsBothRegistries`, `TestJanitorBedOpenWithoutRunAppendsTheMachineIndexOnly`.
- **M2 (a fixture arm refuses without a marker).** `launchRunner` (`runner.go:764`), reached from `ArmFixture`, `RestartFixture` and any arm whose plan is a fixture enrollment (B0), refuses with `BED_UNREGISTERED: run metasystem janitor bed-open on the bed root first` when no marker sits above the root within eight levels. Registration before arm is therefore enforced by the engine, not by a source grep; a bed that dies before `bed-open` returns has armed nothing. Witness: `TestArmFixtureRefusesWithoutBedMarker`, `TestArmFixtureAcceptsAMarkerAbove`.
- **M3 (the runner records its bed owner).** A runner whose root sits beneath a marker copies the marker's `owner` into `runner.json` as `owner: {kind:"bed", root, ...stopfence.Process}` at start; it reads the marker itself, never an environment variable. Witness: `TestRunLoopRecordsBedOwnerFromMarker`, `TestRunLoopWithoutMarkerRecordsNoOwner`.
- **M4 (one deferred cleanup obligation in the launcher).** `LaunchSuite` exports `METASYSTEM_PROOF_LAUNCHER_REF=<pid>:<startedAtSec>:<micro>:<ticks>:<bootId>` and `METASYSTEM_PROOF_BED_REGISTRY=<path>` in the suite's environment (beside the existing proof variables, `internal/proofrun/launcher.go:200-212`; `proofChildEnvironment` at `:519-541` strips both from the inherited environment so a nested launcher gets its own), where the registry path is `<TmpPaths[0]>/beds.jsonl` when `--tmp` was given and otherwise `<controlRoot>/artifacts/agents/janitor/runs/<launcher pid>-<8 hex>/beds.jsonl`. Immediately after `suite.Start()` succeeds (`launcher.go:224`) it registers one `defer` that calls `janitor.ReapRun(registry, launcherRef, ReapOptions{Prober, Signal, Clock, TermGrace, KillGrace, Log})` exactly once, so every return path after the start, including the early kill-and-return paths at `launcher.go:228-235, 245-276, 299-337`, reaps. `ReapRun` reads the registry's frames, keeps only those whose `launcher` equals its own identity (a nested or stale launcher's beds are another launcher's), and for each root still on disk runs `ReapRoot` (C5). A missing registry logs `no beds registered`; a torn tail is tolerated by the framing rule and other frames are honoured. Witness: `TestLaunchSuiteReapsOnEveryPostStartReturn` (a table over the post-start returns driven through the existing seams: a failing watchdog pipe, a failed record write, a second-fence stop, a normal exit; each records exactly one reap call), `TestReapRunActsOnlyOnItsOwnLauncherFrames`, `TestReapRunToleratesAbsentOrTornRegistry`.
- **M5 (the watchdog reaps too).** `stopStalledSuite` calls `ReapRun` with the same options after `SignalSuiteGroup` and before `sweepExecutionGuard` (`internal/proofrun/watchdog.go:206-211`); the watchdog receives the registry path and the launcher ref as flags from `watchdogCommand` (`launcher.go:631-666`). Witness: `TestWatchdogStallPathReapsRegisteredBeds`.
- **M6 (every temp base is covered).** Bed roots are found through the machine bed index, never by scanning a temp directory, so roots under `$TMPDIR`, `/tmp` and `/private/tmp` (channel-fixtures.sh and goal-cli-fixtures.sh use `${TMPDIR:-/tmp}`, the hook bed's clean-environment runs used `/private/tmp`) are all reachable after their processes and their run registry are gone. Witness: `TestBedIndexFindsRootsUnderEveryTempBase` (three roots under three bases in a fixture registry home; the census resolves all three).

## 6. Fixture fakes carry their own deadline

- **F1 (one absolute deadline per bed).** `METASYSTEM_FIXTURE_DEADLINE_EPOCH` is the bed's probed start plus `fixture.bed-lifetime-min`, exported by `harness_fixture_bed_open` (M1). It is wall time, not `$SECONDS`, so it survives subshells, `exec` and rewritten exits. Witness: `TestJanitorBedOpenExportsDeadline`.
- **F2 (the fixture-only verbs always require a deadline).** `util hold` (`cmd/metasystem/hold.go`, a fixture stand-in by its own comment) and `channel fake serve` (`cmd/metasystem/channel_verbs.go:363-376`, "fixture-only" in the verb table) take `--deadline-epoch` with `METASYSTEM_FIXTURE_DEADLINE_EPOCH` as the default and refuse to start with exit 2 naming the variable when neither is set; they exit 70 with `deadline reached` when the injected clock passes it (`util hold`: a `select` over the signal channel and a clock timer; `channel fake serve`: the server context ends). No fixture-root detection is involved, so the two channel beds and the fake adapter's hold children (`scripts/agents/adapters/fake.sh:206,263,283,318,324`) are covered by the same refusal. Witness: `TestUtilHoldExitsAtDeadline`, `TestUtilHoldRefusesWithoutDeadline`, `TestUtilHoldReadsDeadlineFromEnvironment`, `TestChannelFakeServeEndsAtDeadline`, `TestChannelFakeServeRefusesWithoutDeadline`.
- **F3 (the shell fakes loop on the epoch).** `hosts/fake.sh`'s hold loop (`scripts/agents/hosts/fake.sh:42`), the hook bed's `deadline-engine` (`scripts/agents/supervision-hook-fixtures.sh:1766-1774`) and `supervision-hook.sh` under a fixture root loop `while (( $(date +%s) < deadline )) && kill -0 <parent>` and exit 70 past it; a shell fake started without the variable exits 2. The builder-runnable witness is a Go test that runs each fake script's loop function with the variable set to one second in the past through `bash -c` and asserts exit 70 within a bound ten times the expected second (`TestShellFakesExitAtDeadline`, a real-process test, three seconds of budget for a one-second event); the seat's bed runs are extra witnesses.
- **F4 (the deadline exceeds every ceiling).** 180 minutes exceeds the widest scaled fixture ceiling (the 12 second supervision-wait base at the maximum scale of 48 is 576 seconds) by a factor of eighteen. Witness: `TestBedLifetimeExceedsMaximumScaledCeiling` (a table over the base caps of `scripts/agents/fixture-budget.sh` at scale 48).

## 7. Seat waiters (D2)

The goal's waiter clause becomes: a seat shell that burns CPU with no child process for 30 minutes is ended by the engine, logged, and reported to its session. The seat edits the goal's DONE text to match.

- **W1 (the shape).** `claude-tool-shell`: argv[0]'s base is `zsh`, `bash` or `sh`, argv[1] is `-c`, and argv[2] begins with `source ` followed by a path whose parent directory is `<home>/.claude/shell-snapshots/` (the Bash tool's prefix, evidence line 4). It is a positioned shape in `janitor.DefaultShapes`. Witness: `TestClaudeToolShellShape` (the evidence line matches; a user's own `zsh -c` without the snapshot prefix does not).
- **W2 (the readers the census lacks).** The janitor's table fills `census.Process.PPID` through `identity.ParentPid` (`internal/identity/enumerate_darwin.go:112`) for every pid and adds `CPUTime`: on Darwin `proc_pidinfo(PROC_PIDTASKINFO)` through the same `proc_info` syscall the package already uses (`identity_darwin.go:145-175`), fields `pti_total_user` and `pti_total_system` converted with `mach_timebase_info`; on Linux fields 14 and 15 of `/proc/<pid>/stat` times the clock tick. A failed read is `known=false`. Witness: `TestParentPidOfSpawnedChildIsSelf` (one `os/exec` child, bounded), `TestCPUTimeOfOwnProcessIsMonotone` (two reads around a short spin; asserts the second is not smaller, never a wall bound).
- **W3 (the owner).** A tool shell whose parent is an announced main of a checkout is owned by that main's session and is in that checkout's scope. A tool shell whose parent is 1, or whose parent is not an announced main, is `ownerless` (the reparented case): it is in every pass's scope, because it belongs to no session that could still read it, and the pass that finds it acts under the janitor lock. Witness: `TestToolShellOwnerBySessionOrOwnerless`.
- **W4 (the predicate).** A tool shell is a `waiter` when the sightings registry holds sightings of it spanning at least `janitor.waiter-min` minutes (default 30, validated 5 through 1440) in which, at every sighting, no child of the shell older than 10 seconds exists (a child is any live pid whose parent is the shell; the busy loop's `pgrep`, `tr` and `echo` are transient) and the CPU time delta over the span is at least 25 percent of the wall span. A shell with a long-lived child (a `go test`, a `bash` bed) is never a waiter however busy. Witness: `TestWaiterPredicateOverSightings` (rows: busy and childless for 31 minutes, ended; busy with a 40 minute child, kept; childless and idle, kept; busy and childless for 20 minutes, kept; reparented busy and childless, ended).
- **W5 (the action).** The pass ends a waiter with TERM by identity, `KillGrace`, KILL; logs the argv's first 200 bytes, the CPU figure and the span; and queues `steward.QueueNotification(checkout, {Nonce: "waiter-<pid>-<micro>", Message: "janitor ended a busy seat shell pid N after M minutes with no child: <argv head>"})` on the session's checkout (or the acting checkout when ownerless), so the session's Stop hook line and `steward status` carry it. Witness: `TestWaiterActionSignalsLogsAndNotifies`.

## 8. The census and reap verbs

### 8.1 Names and outputs

Both live in the existing `janitor` family (`cmd/metasystem/main.go`, beside `headroom`).

- `metasystem janitor census --root <installation> [--all-owners] [--json]` prints one line per process in the janitor's world (O4):

  `ORPHAN  fixture-steward  pid=495 start=1789149385.512331 age=1d21h bound=3h owner=bed:/private/var/folders/.../tmp.4Hhb5RHimt (dead) reason=owner-dead first-seen=08:36Z action=reap`

  `OWNED   codex-broker     pid=289 start=... age=9h owner=codex-job:job-abc (running) cwd=/Users/wido/.../worktrees/bdrb-u1a`

  `ORPHAN  steward          pid=16685 start=... age=5d owner=none reason=unrecorded scope=/Users/wido/LocalStorage/GitHub/agentic-tools-m1b`

  `WAITER  claude-tool-shell pid=53724 start=... age=12h owner=session:e90b57fc... cpu=7h50m span=12h children=0`

  `UNKNOWN codex-broker     pid=... reason=store-unreadable`

  and a summary `census: owned=12 orphans=97 waiters=1 unknown=3 outside-world=66 roots=41 (dead beds: 31)`. `--json` prints an array of items (`class, pid, pgid, ref{...}, ageSec, boundSec, owner{kind,id,root,ref,liveness}, state, reason, firstSeen, cwd, argv, cpuSec, children`) plus the summary. Exit 0 when the scan completed; 1 when the table could not be enumerated (a partial scan never prints a summary).
- `metasystem janitor reap --root <installation> [--owner <kind>:<id>] [--all-owners] [--plan]` acts on the orphans and waiters of the scope and prints `REAPED <class> pid=... signal=term reason=...`, `REFUSED <class> pid=... because=<younger-than-minimum|owner-alive|owner-unknown|identity-unknown|outside-scope|not-caused|grace-not-elapsed|handover|store-unreadable|companion-live>`, `SURVIVED <class> pid=... after kill`; `--plan` prefixes `WOULD` and signals nothing. Exit 0 when every due orphan was reaped or nothing was due, 1 on a survivor, 2 on usage.
- `metasystem janitor bed-open`, `bed-close` (M1), `prune` and `pin` (section 9) complete the family.

### 8.2 Shapes

| Class | Shape (positioned) | Designated record and owner proof |
| --- | --- | --- |
| `fixture-steward`, `steward` | `<path> steward run --repo <root>` | `<root>/artifacts/agents/steward/runner.json`: names this identity (owned by its bed owner or human enrollment) or another identity or is absent (B6, B7) |
| `supervision-owner`, `supervision-component` | `metasystem supervise owner|component --tag <tag>` | the registry claim for the tag (`internal/registry`), listed; ended only through `ShutdownAt` inside `ReapRoot` |
| `codex-broker` | `node .../app-server-broker.mjs serve --cwd <dir> --endpoint <ep> --pid-file <file>` | the codex-job record for `<dir>`, else the plugin job store for `<dir>` (K5, K10) |
| `codex-companion` | `node .../codex-companion.mjs task|task-worker|review|adversarial-review [--cwd <dir>] [--job-id <id>]` | the same records by job id; listed, never signalled by the janitor |
| `fixture-hold` | `metasystem util hold --tag <tag>` | the bed marker above its engine path or cwd; the tag's job record when dispatch spawned it |
| `fixture-fake-host` | `bash .../hosts/fake.sh start-turn ... --instance-tag <tag>` (the existing `host-fake-start-turn` shape) | the bed marker above the script path |
| `fixture-channel-fake` | `metasystem channel fake serve --dir <dir>` | the bed marker above `--dir` |
| `fixture-hook` | `bash <bed root>/.../supervision-hook.sh <runtime> <event>` | the bed marker above the script path |
| `proof-launcher`, `proof-watchdog`, `delegate-adapter`, `mission-run-loop` | the existing shapes | their own records; listed, never ended by the janitor |
| `claude-tool-shell` | W1 | the announced main that is its parent (W3) |
| `group-member` | any argv, pgid equal to a proven leader's pid | the leader's owner (O3) |

### 8.3 Rules

- **C1 (never a process the metasystem did not cause).** O2 and O4; `reap` refuses `not-caused` for anything outside the world even when named by `--owner`. Witness: `TestReapRefusesUnshapedProcess`.
- **C2 (scope).** Without `--all-owners` the scope of `--root <installation>` is: sessions announced in its `artifacts/agents/mains/`; codex-job records under it; beds whose marker names it as `source` or whose run registry lies under it; unrecorded stewards whose root lies under its toplevel; worktrees under it; and ownerless waiters (W3). Everything else is `outside-scope`, counted, and printed only with `--all-owners`. Witness: `TestCensusScopeExcludesOtherSeatsOwners` (two installations in a fixture home, each with its own beds and jobs).
- **C3 (`--all-owners` on reap is a human act).** It passes the agent-free-terminal gate `metasystem stop` uses (`requireHumanTerminalAt`, `cmd/metasystem/process_verbs.go:110`); on `census` it needs no gate; a pass never runs with it (R-79-m2). Witness: `TestReapAllOwnersRequiresHumanTerminal`.
- **C4 (refusals).** Younger than 2 minutes (`younger-than-minimum`); owner Alive (`owner-alive`); owner or own identity Unknown (`owner-unknown`, `identity-unknown`); first sighting younger than `janitor.orphan-grace-min` (default 10, validated 1 through 1440; `grace-not-elapsed`) except `cwd-gone` and `root-gone`, which act once the minimum age has passed; `handover` (B7); a broker whose store is unreadable or whose companion is live (K5). Witness: `TestReapRefusalTable`.
- **C5 (the ladders and `ReapRoot`).** A recorded fixture steward through `steward.Disarm(root)` (X2); an unrecorded steward, a fixture fake and a waiter through TERM, `TermGrace`, KILL, `KillGrace` with `SignalAuthenticated` (`internal/proofrun/watchdog.go:284-297`) before each signal; a supervision owner through `ShutdownAt` by way of the root's `arm-supervision.sh --shutdown`; a broker through K5. `ReapRoot(root)` runs, in order: every runner record beneath the root, every supervision owner beneath it, then every shaped process whose designated record is the root's marker (fakes, hooks); it never signals an unshaped process under the root (O1). Witness: `TestReapRootOrdersStewardBeforeSupervisionBeforeFakes`, `TestReapRootLeavesUnshapedProcessesUntouched`.
- **C6 (everything is logged).** Every `REAPED`, `REFUSED`, `SURVIVED`, `WOULD` and every runner self-exit is one line in `~/.metasystem/janitor/janitor.log` (registry framing, home override honoured) with the caller's identity and root. Witness: `TestReapAppendsJanitorLog`.
- **C7 (who runs it, when).** The proof launcher and the watchdog after every run (M4, M5); the steward tick of every armed checkout, once per tick after `ReapContinuations` (`internal/steward/tick.go`), as `janitor.Pass(root)`: census, reap in scope, prune in scope, never `--all-owners`; `metasystem up` after `ensureStewardRunner` (`internal/up/up.go:699`) prints `janitor: orphans=N reaped=M` among its component lines; `metasystem test run` and `scripts/agents/go-gate.sh` print the census summary before a load-sensitive run and never refuse (R-35-m3). Concurrent passes of three seats serialize on the janitor lock (S2) for the registry transaction only. Witness: `TestTickRunsJanitorPassInScope`, `TestUpReportsJanitorLine`, `TestTestRunPrintsCensusSummaryWithoutRefusing`.
- **C8 (the health role).** `host-leaks` joins `healthRoleOrder` (`internal/steward/health.go:66-86`): alive when the last census of the scope has zero orphans past grace and zero waiters; dead when an orphan is older than twice its bound; unknown when the last census is older than two ticks or incomplete; reason `orphans=N waiters=M oldest=<age>`. Witness: `TestHostLeaksRoleVerdicts`.
- **C9 (the retained census stream).** Every pass writes its full JSON census to `<installation>/artifacts/agents/janitor/census/<UTC stamp>-<pid>.json` and the pass summary (orphans per class, reaps, refusals, 1 minute load, resident sum of the world) to `census.jsonl` beside it; files older than `janitor.census-retention-days` (default 7) are pruned by the pass. This stream is the proof's evidence (section 13). Witness: `TestPassWritesCensusStreamAndPrunesIt`.
- **C10 (interim).** Until unit 13 lands, the interim is the corrected bash procedure of the evidence file (`records/misc/leaked-processes-evidence-2026-09-15.txt`, the 07:03Z block): a fixture-only filter, TERM, verify, under bash; D3's one-time removal of the legacy roots is the seat's act under the machine lock, not a verb. From unit 11 on, `janitor census`; from unit 13 on, `janitor reap --plan` then `reap`.

### 8.4 The broker backstop inside the pass

The pass calls `delegate codex release --cwd <dir>` for a broker only under K10; it never signals a broker itself.

### 8.5 The sightings registry

- **S1 (schema).** `~/.metasystem/janitor/sightings.jsonl`, registry framing. Frames: `{schemaVersion:1, event:"sighted", key, ref: <stopfence.Process>, class, owner: {kind,id,root}|null, reason, at, cpuSec, children}`; `{event:"cleared", key, ref, at, why: "owner-alive"|"exited"}`; `{event:"acted", key, ref, at, action, outcome}`; `{event:"torn"}` as the framing marker. `key` is `<pid>:<mode>:<start fields>` from `identity.Ref.ModeName()`. Witness: `TestSightingsSchemaRoundTrip`.
- **S2 (one lock, one transaction).** A census pass acquires `~/.metasystem/janitor/lock.d` through `lock.Acquire` (identity-probed, wait 10 s scaled), then reads all frames, reduces, appends this pass's `sighted` and `cleared` frames, computes its decisions from the reduction plus its own frames, and releases; a reap acquires the same lock across its decision and its `acted` frames. Lock rank: the checkout's steward arbitration lock (held by the tick) is taken before the janitor lock; the per-cwd broker lock (K1) after it; no path takes them in another order. Witness: `TestSightingsTransactionHoldsTheLockAcrossReadAndAppend` (a second writer blocks until release and its frame follows).
- **S3 (reduce).** Per key: first `sighted` without a live owner is `firstSeenOrphan`; a `cleared` resets it; the last two `sighted` frames give the waiter's CPU delta and child observations across the span; `acted` closes the key. Witness: `TestSightingsReduce`.
- **S4 (compaction and retention).** Under the same lock, when the file exceeds 1 MB or on the first pass of a day, the reduction is rewritten atomically as the new file with closed keys older than 7 days dropped, exactly as `registry` compaction replaces under its lock (`internal/registry/compact.go`). Witness: `TestSightingsCompactionKeepsOpenKeys`.
- **S5 (recovery).** A torn tail is tolerated by the framing rule; a `CorruptionError` moves the file aside as `sightings.jsonl.corrupt-<stamp>`, starts an empty registry and logs it; the cost is a reset of first-seen ages, which delays reaps and never advances them. Witness: `TestSightingsCorruptionRestartsAndDelays`.
- **S6 (concurrent updates cannot lose each other).** Two passes and a manual reap racing append under the lock leave every frame present; no pass ever replaces the file from a reduction it did not compute under the lock it holds. Witness: `TestSightingsConcurrentAppendsUnderLock` (three goroutines on one fixture home).

## 9. Retention and placement

Numbers are metasystem.conf keys under `janitor.*`, validated by the bounded-knob table (B3), overridable in metasystem.conf.local. `metasystem janitor prune --root <installation> [--plan]` applies them; the pass calls it. `metasystem janitor pin <path> --reason <text>` writes `<path>/.metasystem-pin` (`{reason, by: <identity>, at}`); a pinned entry is never removed.

- **R1 (suite-failures).** Pruning is off until `evidence.root` in metasystem.conf is a real, writable path (the shipped value is the placeholder `<durable evidence root, outside the repository>`, `docs/project-rules.md`); `prune` refuses the store with `EVIDENCE_ROOT_UNSET` until then. When set: an entry older than `janitor.suite-failures-days` (default 3) that carries a `mirror.json` written by the prune's own mirror step (a copy to `<evidence.root>/suite-failures/<entry>` with a per-file sha256 manifest verified after the copy, the manifest shape `job mirror` already writes, `internal/dispatch/mirror.go`) and no `.metasystem-pin` is removed. No count rule (revision 1's twenty newest is withdrawn). Witness: `TestPruneSuiteFailuresRefusesWithoutEvidenceRoot`, `TestPruneSuiteFailuresRequiresVerifiedMirrorAndNoPin`.
- **R2 (proof payloads).** `<attempt>/testing.noindex/*` payloads and `processes/*.json` launch records of terminal attempts older than `janitor.proof-runs-days` (default 7) and unpinned are removed; `attempts/*.json` never (`internal/dispatch/budget.go:515` reads them); non-terminal attempts never. Witness: `TestPruneProofRunsKeepsAttemptRecordsLiveAttemptsAndPins`.
- **R3 (engine pins).** Keep the enrolled generation's pin and the `janitor.engine-pins-keep` newest earlier generations (default 2), across both pin directories (P3); never a pin whose path is the argv of a live process (the live-use pin is the process itself). Witness: `TestPruneEnginePinsKeepsEnrolledPreviousAndReferenced`.
- **R4 (bed roots).** A root in the bed index whose marker is closed with status 0 is removed `janitor.bed-retention-min` (default 60) after `closedAt` when no live shaped process names it; a root whose bed died unclosed (killed) is kept `janitor.suite-failures-days` and then removed unless pinned; `keep:true` exempts. Witness: `TestPruneBedRootsByMarkerState`.
- **R5 (the legacy roots).** The 586 unmarked `tmp.*` roots of 2026-09-15 are removed once, now, by the seat by hand under the machine lock after a no-live-process check (D3). No prune rule or verb covers unmarked roots; revision 1's `--legacy-roots` is withdrawn. Witness: none (no code).
- **P1 (suite-failures out of Spotlight).** `artifacts/agents/suite-failures` becomes `artifacts/agents/suite-failures.noindex`. Writers: `internal/proofrun/watchdog.go:170`, `internal/proofrun/evidence.go:134` (and the suffix match at `:121` accepts both names for one release), `scripts/validate-metasystem.sh:1581`, `scripts/adopt-fixtures.sh:116`, `scripts/agents/supervision-fixtures.sh:541`, `dispatch-fixtures.sh:306`, `health-fixtures.sh:119`, `goal-cli-fixtures.sh:174`, `brain-fixtures.sh:35`; readers: `scripts/agents/suite-progress-fixtures.sh:280` and the test `internal/proofrun/watchdog_test.go:59`. `metasystem up` moves an existing directory's entries into the new store once and leaves a symlink `suite-failures -> suite-failures.noindex`, so recorded paths in old notes still open. Witness: `TestSuiteFailureWritersUseNoindexStore`, `TestUpMigratesSuiteFailuresOnce`.
- **P2 (proof payloads out of Spotlight).** The per-attempt payload root `artifacts/agents/proof-runs/<attempt>/testing/<digest>` (`cmd/metasystem/test.go:879`) becomes `.../testing.noindex/<digest>`; the attempt records stay where they are. Witness: `TestTestRunPayloadRootIsNoindex`.
- **P3 (engine pins out of Spotlight, without breaking enrollments).** New pins are written under `artifacts/agents/steward/engine-pins.noindex` (`internal/steward/identity.go:302, 378`). The identity record stores the pin's absolute path (`InstallPath`), `OpenEnrolledBinary` opens that path, and the runner's argv is that path, so every existing enrollment keeps working and needs no migration; the census recognizes the steward shape under either directory; R3 counts generations across both; the old directory empties as its pins age out. Readers that name the old path: the remedy text at `internal/up/up.go:636` and the bed at `scripts/agents/supervision-fixtures.sh:2563` (which finds a generation-2 pin), both updated to the new directory. Witness: `TestNewPinsAreWrittenUnderNoindexAndOldEnrollmentsStillOpen`.
- **P4 (the placement threshold).** After the three renames land, `mdfind -count "kMDItemFSName == '*.go'" -onlyin <store>` is 0 for each of the three stores on this machine, and the count under each seat's metasystem directory is at most the tracked count plus the tracked count times the number of live delegate worktrees; the seat records the four numbers in the landing receipt of unit 16 and the proof re-records them (section 13). Witness: the recorded numbers (a seat measurement, since Spotlight is not in the test's control).
- **P5 (temp bases).** Bed roots stay under `$TMPDIR`, `/tmp` or `/private/tmp`; the diagnosis measured 0 items indexed under `$TMPDIR`; unit 16's receipt records the count under `/private/tmp` as well.
- **P6 (the delegate-worktree store).** `artifacts/agents/worktrees` stays with run-scoped-build-caches-have-a-janitor (seat ruling); this design does not rename it.

## 10. Related goals

- **fixture-stewards-outlive-their-suite: folded into this goal now (D4; the seat concludes it in Wido's name).** What survives as rules: self-exit on owner death (B2) and at a named bound from a validated key (B3), never a human-enrolled runner (B0); a fixture proves both exits and the human case; a health role counts what leaks (C8). What is superseded: the bed writing the suite's pid and start (the marker is engine-written from a probe, M1, and read by the runner, M3); counting fixture runners (the census counts orphans by record, B6). Its evidence shapes the ladders (every hand-reaped runner died on TERM). Its absorbed note (every suite-written identity carries the fixture kind) is met by B0's authenticated enrollment and by M2's refusal.
- **claude-delegate-scratch-cleanup (queued): untouched.** Its scratch directories hold no process.
- **run-scoped-build-caches-have-a-janitor (approved, m1c): touched.** Shared: the `janitor` family and home; its constraints are adopted (liveness is a process reference plus idle, never age in days; verbs take `--root` and `--owner`, never path lists). Taken here: bed roots (R4) and engine pins (R3). Left there: per-run GOCACHE and staticcheck directories, delegate worktrees and their store's placement (P6), landing-receipt worktrees, the shared Go cache's schedule.
- **winddown-census-handoff-leak (approved, m1b): untouched.** The janitor lists a survived group as `delegate-adapter` or `group-member` with its job owner and refuses it as the job reaper's.

## 11. Units

Land in this order: **1, 2, 3** (the broker lane), **4, 5** (identity, then the runner), **6, 7, 8** (markers, bed sites, the launcher), **9** (fakes), **10, 11, 12, 13, 14** (readers, census, registry, reap, wiring), **15, 16** (retention, placement), **17** (waiters and the seat rules). Every unit allocates at most 300 changed lines including tests (plans/goals/design-allocations-leave-ceiling-margin.md); the builder counts at a checkpoint and stops at the first overrun. Every Boundary is complete: config validation and metasystem.conf where a key is added, existing tests that hard-code a changed path, and `testmain_test.go` for a new package (c15d23be). Every rule has a witness the builder runs without a fixture bed; a seat-run bed is an extra witness only. Each brief carries: the goal id, the rule ids, `Boundary`, `Ceiling: 400` with the allocation below it, `Non-goals`, the proof rule verbatim ("for every rule you add, a test that fails when that rule alone is removed; run it before you return"), the tests by name, the verification order from `metasystem/` (`go build ./...`; `go vet <packages>`; `go test -race -count=1 -timeout 40m <packages>`; `bash -n <shell files>`; `scripts/agents/go-gate.sh --fast`), and the two standing lines (no fixture bed, never `METASYSTEM_BIN`; no commit, report commands and numstat). Each unit starts a fresh chain from the preceding landed unit and is read by Opus.

### Unit 1: `delegate codex` records, index, launch and status

- Rules: K1, K2; X3 for the codex-job record.
- Boundary: `["metasystem/internal/codexjob/codexjob.go", "metasystem/internal/codexjob/codexjob_test.go", "metasystem/internal/codexjob/testmain_test.go", "metasystem/cmd/metasystem/delegate.go", "metasystem/cmd/metasystem/delegate_codex.go", "metasystem/cmd/metasystem/delegate_codex_test.go", "metasystem/internal/config/validate.go", "metasystem/internal/config/validate_test.go", "metasystem/metasystem.conf"]`
- Non-goals: no wait, cancel, release or adopt; no broker signal; no janitor code; no plugin change; no fixture bed; no commit.
- Allocation: 130 production, 150 test, 10 config, total 290. The plugin invocation is a seam (`Runner func(args, env) (stdout, err)`), so tests use a fake.
- Packages: `./internal/codexjob ./cmd/metasystem ./internal/config`.

### Unit 2: cancel by id and the release handshake

- Rules: K4, K5, K6 (stated).
- Boundary: `["metasystem/internal/codexjob/release.go", "metasystem/internal/codexjob/release_test.go", "metasystem/internal/codexjob/codexjob.go", "metasystem/cmd/metasystem/delegate_codex.go", "metasystem/cmd/metasystem/delegate_codex_test.go", "metasystem/internal/janitor/killproof.go", "metasystem/internal/janitor/killproof_test.go"]`
- Non-goals: no wait or adopt; no tick wiring; no census verb; the broker and companion shapes are added to `DefaultShapes` here and nothing else in the janitor changes; no fixture bed; no commit.
- Allocation: 160 production, 130 test, total 290. Seams: the plugin runner, the process table (`[]census.Process`), a dialer, a signaller, a prober, a clock.
- Packages: `./internal/codexjob ./cmd/metasystem ./internal/janitor`.

### Unit 3: wait, adopt, the rescue rule and the upstream report

- Rules: K3, K7, K8, K9.
- Boundary: `["metasystem/internal/codexjob/wait.go", "metasystem/internal/codexjob/wait_test.go", "metasystem/cmd/metasystem/delegate_codex.go", "metasystem/cmd/metasystem/delegate_codex_test.go", "metasystem/docs/orchestration.md", "metasystem/internal/steward/instructions_codex_test.go", "metasystem/records/misc/codex-plugin-upstream-report-2026-09.md"]`
- Non-goals: no change to release's handshake; no janitor code; no waiter text; no fixture bed; no commit.
- Allocation: 90 production, 110 test, 30 prose, 40 record, total 270.
- Packages: `./internal/codexjob ./cmd/metasystem ./internal/steward`.

### Unit 4: the steward record and ladder are exact

- Rules: X1, X2.
- Boundary: `["metasystem/internal/steward/runner.go", "metasystem/internal/steward/runner_identity_test.go", "metasystem/internal/steward/health.go", "metasystem/internal/stoptransition/families.go"]`
- Non-goals: no guards; no owner field; no marker; no arm change; no fixture bed; no commit. `health.go:782` and `families.go:671` read the record's identity fields and follow the new shape.
- Allocation: 80 production, 140 test, total 220.
- Packages: `./internal/steward ./internal/stoptransition`.

### Unit 5: the fixture runner bounds its own life

- Rules: B0, B1, B2, B3, B4, B5, and the bounded-knob table that B3, K10, B7, W4, C4 and section 9 reuse.
- Boundary: `["metasystem/internal/steward/runner.go", "metasystem/internal/steward/runner_guards_test.go", "metasystem/cmd/metasystem/steward_verbs.go", "metasystem/cmd/metasystem/steward_verbs_test.go", "metasystem/internal/config/validate.go", "metasystem/internal/config/validate_test.go", "metasystem/metasystem.conf"]`
- Non-goals: no marker read (the guard's owner is supplied by the caller; unit 6 wires the marker); no launcher or census; no fixture bed; no commit.
- Allocation: 130 production, 150 test, 15 config, total 295.
- Packages: `./internal/steward ./cmd/metasystem ./internal/config`.

### Unit 6: the bed marker, the two registries and the arm refusal

- Rules: M1, M2, M3, M6, F1; X3 for the marker and index.
- Boundary: `["metasystem/internal/janitor/bed.go", "metasystem/internal/janitor/bed_test.go", "metasystem/cmd/metasystem/janitor_verbs.go", "metasystem/cmd/metasystem/janitor_bed_test.go", "metasystem/cmd/metasystem/main.go", "metasystem/internal/steward/runner.go", "metasystem/internal/steward/runner_owner_test.go", "metasystem/internal/config/validate.go", "metasystem/metasystem.conf"]`
- Non-goals: no harness helper (unit 7); no launcher export (unit 8); no reap; no fixture bed; no commit.
- Allocation: 150 production, 140 test, 10 config, total 300.
- Packages: `./internal/janitor ./cmd/metasystem ./internal/steward ./internal/config`.

### Unit 7: every bed opens and closes its marker

- Rules: the call sites of M1 and F1 in every bed that arms supervision, starts a fake or spawns a hold: `supervision-fixtures.sh`, `supervision-hook-fixtures.sh`, `dispatch-fixtures.sh`, `health-fixtures.sh`, `land-fixtures.sh`, `mission-fixtures.sh`, `fingerprint-harness.sh`, `delegate-caps-fixtures.sh`, `second-session-fixtures.sh`, `supervision-go-fixtures.sh`, `channel-fixtures.sh`, `goal-cli-fixtures.sh`, `fixture-bed-scenarios-fixtures.sh`, `brain-fixtures.sh`, `runtime-hook-fixtures.sh`, and the harness helpers in `fixture-budget.sh`.
- Boundary: the sixteen scripts above under `metasystem/scripts/agents/`, plus `metasystem/internal/audit/bed_markers.go` and `metasystem/internal/audit/bed_markers_test.go`.
- Non-goals: no Go change outside the audit; no fixture bed run; no commit. The builder-runnable witness is the audit: `TestEveryArmingBedOpensItsMarkerBeforeItsFirstArm`, a Go test that parses each bed script and requires the helper call to precede the first `arm-supervision.sh`, `steward arm`, `channel fake serve` or `util hold` line (an ordering audit, on top of M2's engine refusal, which is the structural guard). The seat's bed runs are the extra witnesses.
- Allocation: 90 shell, 60 production, 100 test, total 250.
- Packages: `./internal/audit`. Shell: the sixteen scripts.

### Unit 8: the launcher and the watchdog reap what a run armed

- Rules: M4, M5, C5 (`ReapRoot` and its order).
- Boundary: `["metasystem/internal/janitor/reap.go", "metasystem/internal/janitor/reap_test.go", "metasystem/internal/proofrun/launcher.go", "metasystem/internal/proofrun/launcher_reap_test.go", "metasystem/internal/proofrun/watchdog.go", "metasystem/internal/proofrun/watchdog_test.go", "metasystem/cmd/metasystem/proof_run.go"]`
- Non-goals: no census verb; no sightings; no broker; no fixture bed; no commit.
- Allocation: 150 production, 140 test, total 290.
- Packages: `./internal/janitor ./internal/proofrun ./cmd/metasystem`.

### Unit 9: fixture fakes carry their own deadline

- Rules: F2, F3, F4.
- Boundary: `["metasystem/cmd/metasystem/hold.go", "metasystem/cmd/metasystem/hold_test.go", "metasystem/cmd/metasystem/channel_verbs.go", "metasystem/cmd/metasystem/channel_verbs_test.go", "metasystem/internal/channel/fake/fake.go", "metasystem/internal/channel/fake/deadline_test.go", "metasystem/scripts/agents/hosts/fake.sh", "metasystem/scripts/agents/supervision-hook-fixtures.sh", "metasystem/scripts/agents/supervision-hook.sh", "metasystem/scripts/agents/fixture-budget.sh", "metasystem/cmd/metasystem/shell_fakes_test.go"]`
- Non-goals: no change to what a fake does before its deadline; no reap; no fixture bed; no commit.
- Allocation: 100 production, 130 test, 60 shell, total 290.
- Packages: `./cmd/metasystem ./internal/channel/fake`. Shell: the four scripts.

### Unit 10: the parent and CPU readers and the janitor's table

- Rules: W2; the table binding the census uses (a `[]census.Process` with `PPID` and `CPUSec` filled).
- Boundary: `["metasystem/internal/identity/cputime_darwin.go", "metasystem/internal/identity/cputime_linux.go", "metasystem/internal/identity/cputime_other.go", "metasystem/internal/identity/cputime_test.go", "metasystem/internal/census/production.go", "metasystem/internal/census/production_test.go", "metasystem/internal/census/run.go"]`
- Non-goals: no census classification; no shapes; no fixture bed; no commit.
- Allocation: 120 production, 110 test, total 230.
- Packages: `./internal/identity ./internal/census`.

### Unit 11: the census core and verb

- Rules: O1, O2 (the proof side; the reap side is unit 13), O3, O4, B6, B7, C2, W1, W3, the shapes of 8.2, the `census` output of 8.1.
- Boundary: `["metasystem/internal/janitor/census.go", "metasystem/internal/janitor/census_test.go", "metasystem/internal/janitor/killproof.go", "metasystem/internal/janitor/killproof_test.go", "metasystem/cmd/metasystem/janitor_verbs.go", "metasystem/cmd/metasystem/janitor_census_test.go", "metasystem/internal/config/validate.go", "metasystem/metasystem.conf"]`
- Non-goals: no reap; no sightings (first-seen is reported as unknown until unit 12); no waiter predicate; no fixture bed; no commit.
- Allocation: 170 production, 120 test, 10 config, total 300. The census takes an injected table and a `World` (marker, record, index, mains and store readers; a clock).
- Packages: `./internal/janitor ./cmd/metasystem ./internal/config`.

### Unit 12: the sightings registry

- Rules: S1 to S6.
- Boundary: `["metasystem/internal/janitor/sightings.go", "metasystem/internal/janitor/sightings_test.go", "metasystem/internal/janitor/census.go", "metasystem/internal/janitor/census_test.go"]`
- Non-goals: no reap; no waiter action; no compaction schedule beyond S4; no fixture bed; no commit.
- Allocation: 150 production, 140 test, total 290.
- Packages: `./internal/janitor`.

### Unit 13: the reap verb

- Rules: O2 (the reap side), C1, C3, C4, C5 (the ladders for unrecorded stewards and fakes; `ReapRoot` exists from unit 8), C6, W4 (the predicate, computed from S3), the `reap` output of 8.1.
- Boundary: `["metasystem/internal/janitor/reap.go", "metasystem/internal/janitor/reap_verb_test.go", "metasystem/internal/janitor/waiter.go", "metasystem/internal/janitor/waiter_test.go", "metasystem/cmd/metasystem/janitor_verbs.go", "metasystem/cmd/metasystem/janitor_reap_test.go", "metasystem/internal/config/validate.go", "metasystem/metasystem.conf"]`
- Non-goals: no tick wiring; no waiter action or notification (unit 17); no broker signal (the backstop calls unit 2's release in unit 14); no fixture bed; no commit.
- Allocation: 160 production, 130 test, 10 config, total 300.
- Packages: `./internal/janitor ./cmd/metasystem ./internal/config`.

### Unit 14: the tick pass, the backstop, the health role, the up line and the census stream

- Rules: K10, C7, C8, C9.
- Boundary: `["metasystem/internal/steward/tick.go", "metasystem/internal/steward/tick_janitor_test.go", "metasystem/internal/steward/health.go", "metasystem/internal/steward/health_host_leaks_test.go", "metasystem/internal/janitor/pass.go", "metasystem/internal/janitor/pass_test.go", "metasystem/internal/up/up.go", "metasystem/internal/up/up_janitor_test.go", "metasystem/cmd/metasystem/test.go", "metasystem/scripts/agents/go-gate.sh"]`
- Non-goals: no `--all-owners` in machinery; no retention rules (the pass calls `prune` only from unit 15 on); no fixture bed; no commit.
- Allocation: 150 production, 130 test, 15 shell, total 295.
- Packages: `./internal/steward ./internal/janitor ./internal/up ./cmd/metasystem`. Shell: `go-gate.sh`.

### Unit 15: retention

- Rules: R1, R2, R3, R4, the `prune` and `pin` verbs.
- Boundary: `["metasystem/internal/janitor/retention.go", "metasystem/internal/janitor/retention_test.go", "metasystem/internal/janitor/pass.go", "metasystem/cmd/metasystem/janitor_verbs.go", "metasystem/cmd/metasystem/janitor_prune_test.go", "metasystem/internal/config/validate.go", "metasystem/internal/config/validate_test.go", "metasystem/metasystem.conf"]`
- Non-goals: no store rename; no legacy roots; no fixture bed; no commit.
- Allocation: 170 production, 110 test, 15 config, total 295.
- Packages: `./internal/janitor ./cmd/metasystem ./internal/config`.

### Unit 16: placement

- Rules: P1, P2, P3, P4 and P5 (measured by the seat at landing).
- Boundary: `["metasystem/internal/proofrun/watchdog.go", "metasystem/internal/proofrun/watchdog_test.go", "metasystem/internal/proofrun/evidence.go", "metasystem/internal/proofrun/evidence_noindex_test.go", "metasystem/cmd/metasystem/test.go", "metasystem/cmd/metasystem/test_test.go", "metasystem/internal/steward/identity.go", "metasystem/internal/steward/identity_test.go", "metasystem/internal/up/up.go", "metasystem/internal/up/up_noindex_test.go", "metasystem/scripts/validate-metasystem.sh", "metasystem/scripts/adopt-fixtures.sh", "metasystem/scripts/agents/supervision-fixtures.sh", "metasystem/scripts/agents/dispatch-fixtures.sh", "metasystem/scripts/agents/health-fixtures.sh", "metasystem/scripts/agents/goal-cli-fixtures.sh", "metasystem/scripts/agents/brain-fixtures.sh", "metasystem/scripts/agents/suite-progress-fixtures.sh"]`
- Non-goals: no retention change; no worktree store change; no fixture bed; no commit.
- Allocation: 90 production, 130 test, 60 shell, total 280.
- Packages: `./internal/proofrun ./cmd/metasystem ./internal/steward ./internal/up`. Shell: the eight scripts.

### Unit 17: waiters are ended, and the seat rules

- Rules: W5, and the instruction text of section 7 (the loop-deadline and background-task sentences of revision 1's W1, kept as seat guidance beside the engine rule) with its test.
- Boundary: `["metasystem/internal/janitor/waiter.go", "metasystem/internal/janitor/waiter_test.go", "metasystem/internal/janitor/pass.go", "metasystem/internal/janitor/pass_test.go", "metasystem/internal/steward/notify.go", "metasystem/docs/orchestration.md", "metasystem/internal/steward/instructions_janitor_test.go"]`
- Non-goals: no change to the predicate of unit 13; no plugin change; no fixture bed; no commit.
- Allocation: 110 production, 120 test, 30 prose, total 260.
- Packages: `./internal/janitor ./internal/steward`.

## 12. Complete rule-to-witness table

| Rule | Unit | Witness |
| --- | --- | --- |
| O1 | 11, 13 | `TestCensusScopeNeverAuthorizesAction` |
| O2 | 11, 13 | `TestReapRequiresShapeAndOwnerProof` |
| O3 | 11, 13 | `TestGroupMembersAreSignalledOnlyThroughTheLeader` |
| O4 | 11 | `TestCensusWorldMembership` |
| K1 | 1 | `TestDelegateCodexLaunchRecordsBeforeReturn`, `TestDelegateCodexLaunchFailureWritesNothing` |
| K2 | 1 | `TestDelegateCodexStatusResolvesCwdFromTheIndex` |
| K3 | 3 | `TestDelegateCodexWaitReleasesAtTerminal`, `TestDelegateCodexWaitTimeoutReleasesNothing` |
| K4 | 2 | `TestDelegateCodexCancelFromAnotherDirectory`, `TestDelegateCodexCancelWithoutRecordNeedsCwd` |
| K5 | 2 | `TestReleaseHandshakeOrder`, `TestReleaseRefusalsAreNamed` |
| K6 | none | a stated limit |
| K7 | 3 | `TestDelegateCodexAdoptRequiresAnExistingJob` |
| K8 | 3 | `TestSeatInstructionsNameTheCodexVerb` |
| K9 | 3 | `TestUpstreamReportNamesTheSixFacts` |
| K10 | 14 | `TestBackstopReleasesOnlyGoneCwdOrIdleEmptyStore` |
| X1 | 4 | `TestRunnerRecordCarriesFullRef` |
| X2 | 4 | `TestDisarmRefusesAPidReusedWithinTheSecond` |
| X3 | 1, 6, 12 | `TestJanitorRecordsRefuseInvalidRefs` |
| B0 | 5 | `TestArmAtHumanTerminalOnFakeRuntimesRootMintsHumanEnrollment`, `TestRunLoopGuardsHumanEnrollmentOnFakeRuntimesRootIsUnbounded` |
| B1 | 5 | `TestRunLoopExitsWhenRootGone` |
| B2 | 5 | `TestRunLoopExitsWhenBedOwnerDies`, `TestRunLoopKeepsRunningOnUnknownOwner` |
| B3 | 5 | `TestRunLoopExitsAtFixtureLifetime`, `TestFixtureRunnerLifetimeKeyIsBounded` |
| B4 | 5 | `TestRunLoopSelfExitRemovesRecord` |
| B5 | 5 | `TestRunLoopGuardsApplyWhileTermIsIgnored` |
| B6 | 11, 13 | `TestUnrecordedRunnerIsAnOrphanByRecord` |
| B7 | 11, 13 | `TestHandoverGraceKeepsThePreviousGeneration` |
| M1 | 6 | `TestJanitorBedOpenWritesProbedIdentity`, `TestJanitorBedOpenAppendsBothRegistries`, `TestJanitorBedOpenWithoutRunAppendsTheMachineIndexOnly` |
| M2 | 6 | `TestArmFixtureRefusesWithoutBedMarker`, `TestArmFixtureAcceptsAMarkerAbove` |
| M3 | 6 | `TestRunLoopRecordsBedOwnerFromMarker`, `TestRunLoopWithoutMarkerRecordsNoOwner` |
| M4 | 8 | `TestLaunchSuiteReapsOnEveryPostStartReturn`, `TestReapRunActsOnlyOnItsOwnLauncherFrames`, `TestReapRunToleratesAbsentOrTornRegistry` |
| M5 | 8 | `TestWatchdogStallPathReapsRegisteredBeds` |
| M6 | 6 | `TestBedIndexFindsRootsUnderEveryTempBase` |
| bed sites | 7 | `TestEveryArmingBedOpensItsMarkerBeforeItsFirstArm` |
| F1 | 6 | `TestJanitorBedOpenExportsDeadline` |
| F2 | 9 | `TestUtilHoldExitsAtDeadline`, `TestUtilHoldRefusesWithoutDeadline`, `TestUtilHoldReadsDeadlineFromEnvironment`, `TestChannelFakeServeEndsAtDeadline`, `TestChannelFakeServeRefusesWithoutDeadline` |
| F3 | 9 | `TestShellFakesExitAtDeadline` (real process, bounded) |
| F4 | 9 | `TestBedLifetimeExceedsMaximumScaledCeiling` |
| W1 | 11 | `TestClaudeToolShellShape` |
| W2 | 10 | `TestParentPidOfSpawnedChildIsSelf`, `TestCPUTimeOfOwnProcessIsMonotone` |
| W3 | 11 | `TestToolShellOwnerBySessionOrOwnerless` |
| W4 | 13 | `TestWaiterPredicateOverSightings` |
| W5 | 17 | `TestWaiterActionSignalsLogsAndNotifies` |
| C1 | 13 | `TestReapRefusesUnshapedProcess` |
| C2 | 11 | `TestCensusScopeExcludesOtherSeatsOwners` |
| C3 | 13 | `TestReapAllOwnersRequiresHumanTerminal` |
| C4 | 13 | `TestReapRefusalTable` |
| C5 | 8, 13 | `TestReapRootOrdersStewardBeforeSupervisionBeforeFakes`, `TestReapRootLeavesUnshapedProcessesUntouched` |
| C6 | 13 | `TestReapAppendsJanitorLog` |
| C7 | 14 | `TestTickRunsJanitorPassInScope`, `TestUpReportsJanitorLine`, `TestTestRunPrintsCensusSummaryWithoutRefusing` |
| C8 | 14 | `TestHostLeaksRoleVerdicts` |
| C9 | 14 | `TestPassWritesCensusStreamAndPrunesIt` |
| C10 | none | interim procedure |
| S1 | 12 | `TestSightingsSchemaRoundTrip` |
| S2 | 12 | `TestSightingsTransactionHoldsTheLockAcrossReadAndAppend` |
| S3 | 12 | `TestSightingsReduce` |
| S4 | 12 | `TestSightingsCompactionKeepsOpenKeys` |
| S5 | 12 | `TestSightingsCorruptionRestartsAndDelays` |
| S6 | 12 | `TestSightingsConcurrentAppendsUnderLock` |
| R1 | 15 | `TestPruneSuiteFailuresRefusesWithoutEvidenceRoot`, `TestPruneSuiteFailuresRequiresVerifiedMirrorAndNoPin` |
| R2 | 15 | `TestPruneProofRunsKeepsAttemptRecordsLiveAttemptsAndPins` |
| R3 | 15 | `TestPruneEnginePinsKeepsEnrolledPreviousAndReferenced` |
| R4 | 15 | `TestPruneBedRootsByMarkerState` |
| R5 | none | the seat's one-time act (D3) |
| P1 | 16 | `TestSuiteFailureWritersUseNoindexStore`, `TestUpMigratesSuiteFailuresOnce` |
| P2 | 16 | `TestTestRunPayloadRootIsNoindex` |
| P3 | 16 | `TestNewPinsAreWrittenUnderNoindexAndOldEnrollmentsStillOpen` |
| P4, P5 | 16 | the recorded `mdfind` counts (seat measurement) |
| P6 | none | seat ruling |

## 13. Proof of DONE

The goal's DONE is "a day of normal three-seat work that ends with no leaked process and a lower load", read with D2's waiter clause. The proof is one calendar day that starts after unit 17 has landed on all three seats, with nothing landing on this goal during it, and it reads the retained census stream (C9) and the sightings registry (S1), not an end-of-day snapshot.

1. Before: the diagnosis numbers stand (109 orphan stewards, 66 brokers and 574 Codex processes, about 24 GB resident, load 25 to 37 with three seats working). At the day's start the seat runs `metasystem janitor census --all-owners --json` from the enrolled terminal and saves it under `artifacts/reports/leak-census/<date>/start.json` with `uptime`.
2. During: the three seats work normally, including at least three full section runs across seats, at least five `delegate codex` jobs in worktrees of which two are cancelled and one has its worktree removed before `wait`, and one deliberate busy tool shell started by a seat under a named test (the revision-1 loop, with a 40 minute budget) that the engine must end; nobody reaps by hand.
3. Thresholds, per class, read from the stream and the registry at the day's end:
   - orphans: every pass of the day reports zero orphans past grace in every class (`handover` and within-grace sightings allowed);
   - owner-to-exit latency, from `sighted`/`acted`/`cleared` frames: a fixture steward exits within 60 seconds of its owner's death or its root's removal (B1, B2: a 5 second guard interval, with margin); a broker of a `delegate codex` job is gone within 60 seconds of the job's terminal state or cancel (K3, K4); a backstop broker within the idle bound plus one tick (K10); the deliberate waiter within `janitor.waiter-min` plus one tick (W4, W5);
   - waiters: the deliberate one is ended, logged and its notification reaches the session's Stop line; no other waiter appears;
   - placement: `mdfind -count` of `.go` files is 0 under each of the three `.noindex` stores and under `/private/tmp`, and under each seat's metasystem directory at most the tracked count plus the tracked count times the number of live delegate worktrees (P4);
   - resident memory: the janitor world's resident sum at every pass is at most 6 GB across the machine (three seat stewards and their supervision, live beds, live brokers at five processes each), against 24 GB before;
   - load: the 1 minute load recorded by every pass, averaged over the passes in which at least one section run was live on the machine, is at most 70 percent of the diagnosis band's mean of 31, that is at most 22; a pass with no run live is reported separately and must average under 8 (the 6.0 measured after the hand reap, with margin).
4. The census outputs, the registry extract and the numbers go into `records/misc/leaked-processes-proof-<date>.md`, the artifact the goal's conclusion cites. A threshold miss is a defect to fix forward (R-103-m1e), then the day is run again.

## 14. Budget and the questions for Wido

The unit list is seventeen units. Each unit is one Codex build plus one Opus read, 34 attempts; the last two designs of this program needed rework on about one unit in four, so the estimate is 40 attempts and 1500 reserved job minutes (17 builds at 45 minutes, 17 reads at 20 minutes, four rework rounds at 65), over three working days at one active job, or two days if Wido allows two active jobs on m1b and m1c beside the seat. No unit is authorized until Wido approves that budget on this goal (D4, SMLP-18); the folded goal's box does not transfer.

Everything else this revision needed from Wido is decided (D1 to D4) and recorded above; the retention and bound numbers are config keys under `janitor.*`, `steward.*` and `fixture.*` with the defaults stated, so a different number is a conf line.

## 15. Critique record

Round 1 (2026-09-15, Codex gpt-5.6-sol, scratchpad leak-design-critique-r1.md, 19 material findings, verdict rework; all 19 accepted as material by the seat):

- SMLP-01 (broker owned by its environment's session): folded by D1. Session identity is not a broker input; ownership is the codex-job record and the plugin job store (section 3.1, K1, K5, K10; the answer at 3.2).
- SMLP-02 (location as causation): folded. A path is scope only; ending a process needs a shipped positioned shape plus the designated record's owner proof; `ReapRoot` never signals an unshaped process (section 2.3, O1, O2; C5).
- SMLP-03 (an unauthenticated shutdown request): folded by D1. The release handshake checks the plugin store and the process table twice under the per-cwd lock, re-proves identity, and only then requests shutdown; the one-spawn residual is stated and referred upstream (3.1, K5, K6, K9; 3.2).
- SMLP-04 (identity records not exact): folded. Every record carries a full `identity.Ref` in the `stopfence.Process` spelling; unit 4 makes `RunnerRecord`, `liveRunner`, `sameRunner` and `Disarm` exact before any unit ends a steward (section 4.1, X1 to X3).
- SMLP-05 (the launcher's reap missed early returns; registration proven by grep): folded. One `defer` registered immediately after `suite.Start()` reaps on every post-start return; the registry is fenced to the launcher's own identity; a fixture arm refuses without a marker, enforced by the engine; the ordering audit is an extra (section 5, M2, M4, M5; unit 7).
- SMLP-06 (cancel from anywhere and terminal shutdown were prose): folded by D1. `delegate codex cancel <id>` and `wait` are engine code with witnesses; the seat rule names the verb (3.1, K3, K4, K8; 3.2).
- SMLP-07 (job id discarded, idle unowned and unbounded): folded. The companion shape carries `--job-id` and `--cwd`; the store check reads job status by id; idle comes from the sightings registry and is acted on by the backstop in unit 14, not stubbed (3.1, K5, K10; 8.2; 3.2).
- SMLP-08 (fixture authority from the config predicate): folded. Fixture mode needs an authenticated fixture enrollment or a bed owner; `arm` stops rewriting a human enrollment on a fake-runtimes root; the human witness exercises exactly that root (section 4.2, B0).
- SMLP-09 (two channel fakes and the adapter's hold children uncovered): folded. The fixture-only verbs always require a deadline (no root detection); channel-fixtures.sh and goal-cli-fixtures.sh open markers in unit 7; the adapter's children are covered by the refusal (section 6, F2; unit 7).
- SMLP-10 (the old-layout steward had no rule): folded. An unrecorded runner is an orphan by record with its own ladder and scope; the handover grace covers the re-arm case from the fold notes (section 4.3, B6, B7).
- SMLP-11 (waiters only reported): folded by D2. A busy childless tool shell is ended by the engine after 30 minutes, logged and reported; the shape, the parent and CPU readers, the reparented case and the witnesses are named (section 7, W1 to W5; units 10, 13, 17).
- SMLP-12 (roots under /tmp and /private/tmp unreachable): folded. A durable machine bed index under registry framing replaces temp-directory scanning; every base is covered (section 5, M1, M6; R4).
- SMLP-13 (retention deletes the only copy): folded. No deletion without an expiry rule plus a live-use pin; suite-failures pruning is off until the evidence root is configured and then requires a verified mirror; the count rule is withdrawn (section 9, R1 to R4, the `pin` verb).
- SMLP-14 (placement covered one store): folded. Proof payloads and new engine pins move to `.noindex` stores, with the pin path consequences handled through the identity record's absolute `InstallPath`; the delegate worktree store stays with the caches goal by seat ruling; thresholds are stated (section 9, P2 to P6; section 13).
- SMLP-15 (the sightings file was an unlocked registry): folded. Schema, one lock across read, merge and replace, lock rank, reduce, compaction, recovery and a concurrent-update witness, built on `internal/registry` framing and `internal/lock` (section 8.5, S1 to S6).
- SMLP-16 (units at the ceiling, incomplete Boundaries, bed legs as the only witness): folded. Seventeen units at most 300 allocated lines; every Boundary names config validation, metasystem.conf, the tests that hard-code paths and `testmain_test.go` for the new package; every rule has a builder-runnable witness, with beds as extra witnesses only; the bed call sites are one unit with a Go ordering audit (section 11).
- SMLP-17 (the proof could not establish DONE): folded. A retained per-tick census stream and the sightings registry are the evidence; the day starts after the last unit; thresholds per class include orphan counts, owner-to-exit latency, indexed-file counts under each store and a stated load comparison (C9; section 13).
- SMLP-18 (the design authorized its own budget, fold, deletion and choices): folded by D3 and D4. The legacy roots are the seat's one-time act; the fold is Wido's; the budget is estimated and no unit is authorized until Wido approves it; the wrapper and worktree questions are decided by D1 and the seat ruling (sections 9, 10, 14).
- SMLP-19 (the biggest leak was not first): folded by the seat's ordering ruling. Units 1 to 3 build the broker lane first (section 11).
- SMLP-20 (non-material, citation table): the citations the table marked partial are corrected in this revision (`dispatch.sh:1684` is now described as the worktree path only; `identity_darwin.go` is no longer cited for environment reading; `validate.go:501-511` is described as positivity-only with a new bounded table; `launcher.go:380-385` is no longer an insertion point; `validate-metasystem.sh:1614-1630` is not cited).

Next step: unit 1 (the `delegate codex` records, index, launch and status) waits on Wido's approval of the budget in section 14; once approved it is briefed to Codex gpt-5.6-sol with its Boundary, Ceiling, Non-goals and the proof rule, and read by Opus.
