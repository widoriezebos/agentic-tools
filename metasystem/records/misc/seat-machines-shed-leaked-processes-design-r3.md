# seat-machines-shed-leaked-processes

- Owner: m1e (goal seat-machines-shed-leaked-processes, priority 1, sequence 11, tier 3). **Revision 3, 2026-09-15**, written by the Claude Fable design delegate on the narrowed scope of R-113-m1e, against critique round 2 (21 material findings, verdict rework; dispositions in section 16) and the round-1 dispositions that still apply. Checked against origin/main 0938219c. Revision 2's full text is archived beside the seat's critique files (scratchpad `leak-design-r2.md`) for the three goals that inherit its moved sections. The seat integrates the page and edits only this header block.
- What changed in revision 3: (1) the scope is what the metasystem fully owns (R-113-m1e decision 1): fixture-mode steward runners bound their own life from an authenticated fixture owner, the proof launcher reaps what a run armed on every return path, fixture fakes carry their own deadlines, and a census verb and a reap verb cover metasystem-caused processes, with the exact identity, owner records and registry those need. (2) The Codex broker lane (revision 2 section 3, rules K1 to K10) moves to codex-jobs-run-through-a-metasystem-verb; seat waiters (W1 to W5, S3's waiter reduction) move to busy-seat-shells-are-ended; retention and placement (R1 to R5, P1 to P6) move to evidence-and-build-output-have-retention-and-stay-unindexed; each hand-over is a seed section (8, 9, 10) with the rules as they stood, the findings the next design must answer, and the code facts verified here. (3) The run registry lives in the durable control root, never under the child-owned `--tmp` directory (SMLP-211). (4) Every post-start signal in the launcher goes through the exact-identity helper, with a witness per branch (SMLP-212). (5) The lock owner identity carries the full `identity.Ref`; the change is its own unit, ordered before any unit that ends a process (SMLP-213). (6) The bed index and the sightings become one machine-wide janitor registry with one lock across read, reduce and replace, bounded compaction, mid-file recovery and re-indexing from markers (SMLP-215). (7) The proof reads the janitor log's own timestamps for owner-to-exit latency and drops the memory and load metrics the narrowed DONE no longer names (SMLP-218). (8) Fifteen units in dependency order, none needing a later unit, at most 290 allocated lines each, complete Boundaries (SMLP-219, -220). (9) Every shipped default is listed in section 15 for Wido's approval with the budget; nothing ships before (SMLP-221). (10) The census lists brokers, companions and seat tool shells so it never misattributes them, and never ends them; ending them belongs to the two new goals.
- Goal and current status: the diagnosis is landed; this page is the design on the narrowed scope; nothing is built. The goal's DONE now reads: no fixture process outlives its owner beyond a bounded time (fixture-mode steward runners bound their own life from an authenticated fixture owner, the proof launcher reaps what a run armed on every return path, fixture fakes carry their own deadlines); a census verb names every metasystem-caused process by owner, age and bound, and a reap verb ends only proven orphans after re-proving exact identity beside each signal; proven by a day of normal three-seat work that ends with no orphaned fixture process.
- In flight right now: this revision awaits the goal's last critique round (round 3 of 3); no fourth round exists.
- Decisions made (and who made them): Wido, R-112-m1e and R-113-m1e (2026-09-15): the narrowing, the three new goals, every Codex job on a seat machine through the verb once it exists, the one-time removals of the legacy roots and of old build output by the seat. The seat: the round-1 rulings of revision 2 (causation, identity, launcher, fixture authority, coverage, ledger, unit size, proof) stand on the kept scope. This page: the mechanisms of sections 2 to 7.
- Waiting on the human: the budget and the defaults of section 15, after round 3.
- Dead ends (do not retry without new evidence): a pattern kill by name (docs/orchestration.md, Shared Machines); killing supervision components before their owner (KI-32); a filter keyed on the temp directory or on `steward run`; a reap written for bash and run under zsh; age in days as liveness; location as causation (SMLP-02); a launcher reap after the common wait (SMLP-05); a run registry under the `--tmp` directory the validation child deletes at exit (SMLP-211); broker ownership from a broker's environment (SMLP-01); an unlocked append-only machine index (SMLP-215); ending brokers or shells from this goal (R-113-m1e).
- Next step: critique round 3, the last; then Wido's approval of the budget and defaults in section 15; then unit 1 (the steward runner's record and ladder are exact).

The goal's box, from its record: elapsed 1d, 10 attempts, 1200 reserved job minutes, 1 active job, 3 review rounds. `metasystem.budget.*` keys are not touched. Citations are relative to `metasystem/` unless they name the plugin under `/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/`. Uncited test names, verbs and config keys are proposed. Allocations are estimates with margin, never measured proof.

## 1. The problem, measured

On 2026-09-15 between 08:30 and 09:05 CEST, with three seats working, the shared machine ran at a load average of 37, then 18 to 27, and held about 24 GB resident in processes whose owners had ended. Of that, the fixture classes this goal keeps: 78 fixture stewards (`<engine pin> steward run --repo <temp root>`, parent launchd) at 08:46 and 109 at 09:03, 2.1 to 3 GB, the oldest four days old; one 29-section engine run and one m1c run finished in between, so a full section run leaks about fifteen stewards. Six fixture fakes (`util hold`, `hosts/fake.sh start-turn`) and one supervision-hook pair eleven hours old survived beside them, a `channel fake serve` from a hook bed run under `/private/tmp` was eight hours old, and one generation-1 steward five days old was pinned to the m1b repository root of the previous layout (a second one, pid 16333, on m1c). The hand reap of 97 fixture orphans needed only SIGTERM: none was wedged, none had been signalled. The rest of the day's measurement (574 Codex processes and 21 GB, one busy seat shell for twelve hours, 23 GB of preserved evidence and 10 GB of pins indexed by Spotlight) belongs to the three goals of sections 8 to 10.

## 2. The ownership model

### 2.1 Owners and records

Every fixture process the metasystem starts belongs to one owner, and a janitor reads the owner from a record, never from the process's own claim:

| Owner kind | Owner id | Root | Record, and who writes it |
| --- | --- | --- | --- |
| `bed` (a fixture bed) | the bed's full process identity | the bed's temp root | `<root>/.metasystem-bed.json`, written by `metasystem janitor bed-open` (M1); a `bed-opened` frame in the janitor registry (S1) |
| `run` (a proof run) | the launcher's full process identity | the run's registry file under the control root | `<controlRoot>/artifacts/agents/janitor/runs/<launcher pid>-<micro>/beds.jsonl`, appended by `bed-open` (M1, M4) |
| `steward` (a runner's own record) | the runner's full process identity | `--repo` | `<repo>/artifacts/agents/steward/runner.json`, written by the runner (X1), carrying its bed owner under a bed (M3) |
| `human-enrollment` | the enrolled checkout | the checkout | the steward identity record, minted by `steward arm` at an agent-free terminal today |

Two more owners exist only so the census never misattributes their processes: `session` (a seat session, from `artifacts/agents/mains/*.json`, written by `metasystem up --session`, `internal/up/up.go:659`) owns the seat's tool shells for listing, and `external-lane` names a Codex broker or companion by its argv `--cwd`; neither is ended by this goal (section 7.2).

Each record stores a full `identity.Ref` in the `stopfence.Process` spelling (`pid`, `pidStartedAt`, `pidStartedAtMicro`, `pidStartTicks`, `bootId`, `internal/stopfence/fence.go:33-47`), exact on Darwin and on Linux, never a whole-second fallback where an exact shape exists (`internal/identity/identity.go:72-93`). The janitor registry and its lock follow `METASYSTEM_SUPERVISION_REGISTRY_HOME` exactly as `armed-checkouts.jsonl` does (`internal/registry/selection.go:13-26`), so fixtures and `testenv.Main` never touch the seat's files.

Why the existing census cannot carry this: `internal/census` answers "which agent-signature processes are in this checkout's scope" for one checkout (`internal/census/run.go:209-260`) and its verdict gates dispatch. The leak question is machine-wide across temp roots and needs owner records the census never reads. The janitor census of section 7 is a second walk over the same process table (`census.EnumerateProcesses`, `internal/census/production.go:45-76`) with the owner model above; the supervision census and its verdict are untouched.

### 2.2 Identity, three-way

Before any action the janitor probes the target with `identity.AliveRef(prober, ref)` (`internal/identity/identity.go:187-217`): a reused pid compares unequal on its exact start and reads Dead; an unreadable process reads Unknown; only Alive with a matching identity is signalled, and only a Dead owner, or a record that definitively names another process, makes an orphan. Unknown never authorizes anything (`identity.go:8-13`). Every process-ending path takes an injected `Prober`, `Signal` and `Clock` so its tests run on artificial time (R-104-m1e).

### 2.3 Causation: a shape plus an owner proof, never a path

- **O1 (a path is scope, never causation).** A process whose cwd or argv path lies under a bed root or a checkout is in that root's scope for listing, as the supervision census treats scope (`internal/census/run.go:218-244`). Scope never authorizes a signal. Witness: `TestCensusScopeNeverAuthorizesAction` (an unshaped process with cwd under a dead bed's root is listed `outside-world`; `reap` refuses it `not-caused`).
- **O2 (ending a process needs a shipped shape and an owner proof).** A process may be signalled only when its argv matches one shipped positioned shape (`janitor.Shape` with a tag or path flag in a defined position, `internal/janitor/killproof.go:41-63`, extended by section 7.3, matched by `MatchShape`) and the shape's designated record proves the owner definitively: the bed marker for fixture fakes and hooks, the runner record for stewards. A shape match without the record's proof is `owner-unknown` and is refused. Witness: `TestReapRequiresShapeAndOwnerProof`.
- **O3 (group members follow a proven leader).** A process whose process group leader is a shaped, owner-proven process, and whose own argv matches no shape (a fake host's `sleep`), is that leader's member: listed with the leader's owner and signalled only through the leader's group signal, never on its own. Witness: `TestGroupMembersAreSignalledOnlyThroughTheLeader`.
- **O4 (what the census sees).** The census lists every shaped process (owned, orphan, unknown, or listed-only for the external lanes and tool shells), every member of a shaped leader's group, and, as `outside-world`, unshaped processes whose cwd or argv lies under a root the janitor knows. Everything else is invisible. Witness: `TestCensusWorldMembership`.

### 2.4 Existing machinery this builds on, and what stays untouched

| Owner today | Decision |
| --- | --- |
| `identity.KernelProber`, `AliveRef`, `Compare`, `AllPids`, `ProcessCwd`, `ParentPid` (`internal/identity`) | Reused; nothing added (no environment reader, no CPU reader). |
| `census.EnumerateProcesses`, `ResolveCwds` (`internal/census/production.go`) | Reused as the janitor's table; the production row gains its parent (`census.Process.PPID`, today 0 at `production.go:66`) for the tool-shell listing. The supervision census verdict is untouched. |
| `janitor.Shape`, `MatchShape`, `DefaultShapes`, `GroupOwnership` (`internal/janitor/killproof.go`) | Extended with the shapes of 7.3; the positioned-tag proof is the causation rule. `SelectTargets` (`targets.go`) is untouched. |
| `lock.Identity`, `lock.Acquire` (`internal/lock/lock.go:30-36, 141`) and its callers `stopfence.Acquire` (`internal/stopfence/fence.go:254-277`), `registry.LockedAppend` (`internal/registry/append.go:20`), `gaterun/guard.go:91,274`, `dispatch/ownerlock.go:89`, `brain/brain.go:343` | The identity gains the exact fields (X3); `stopfence` and `registry` populate and probe them; the other three callers keep working unchanged and stay whole-second until their own goals move them. |
| `registry.AppendFrame`, `ReadFrames`, torn markers, compaction under one lock (`internal/registry/framing.go`, `compact.go`) | Reused by the janitor registry (S1 to S5). |
| `steward.RunLoop`, `RunnerRecord`, `liveRunner`, `sameRunner`, `Disarm` (`internal/steward/runner.go`) | Made exact (X1, X2); the loop gains guards (B0 to B5); the record gains an owner (M3); `arm` stops rewriting enrollment from the config predicate (B0). `EnsureRunner`'s exclusion (`runner.go:419`) is untouched. |
| `proofrun.LaunchSuite`, `RunWatchdog`, `SignalAuthenticated`, `StopSuite`, the `Prober` and `Signal` seams (`internal/proofrun`) | Every post-start signal authenticated (X4); one deferred reap (M4); the same in the watchdog (M5). |
| `supervise.ShutdownAt` (`internal/supervise/arming.go:1232`) through the root's `arm-supervision.sh --shutdown` | Reused per repository under a bed root inside `ReapRoot` (C5). |
| `stoptransition.LocalFamilies` (`metasystem stop`) | Untouched. |
| `steward.RunTick`, health roles (`internal/steward`) | One pass and one role added (C7, C8). |
| `cmd/metasystem/hold.go`, `channel_verbs.go`, `scripts/agents/hosts/fake.sh`, the hook bed's `deadline-engine` | Each gains the deadline (F2, F3). |
| The Codex plugin | Not modified and not called by this goal. |

## 3. Exact identity

- **X1 (the runner record is a full Ref).** `RunnerRecord` (`internal/steward/runner.go:41-49`) gains `pidStartedAtMicro` and writes all five identity fields from `self.Ref()` at `RunLoop` start (`runner.go:141-148`); readers accept the old shape as the labelled legacy mode (`identity.CompareLegacySeconds`). Its two other readers, `health.go:782` and `stoptransition/families.go:671`, build the ref from the full record. Witness: `TestRunnerRecordCarriesFullRef`.
- **X2 (the ladder compares exactly).** `liveRunner` (`runner.go:1009-1030`), `sameRunner` (`:917-925`) and so `Disarm` (`:942-1005`) compare through `identity.Compare(exact, record.Ref())`; a Darwin record with microseconds never matches a pid reused within the same second; a legacy record compares by seconds and the disarm output names the mode. Witness: `TestDisarmRefusesAPidReusedWithinTheSecond` (a fake prober answers the same pid with a start one microsecond later; TERM is not sent).
- **X3 (the lock owner is a full Ref).** `lock.Identity` gains `pidStartedAtMicro`, `pidStartTicks` and `bootId` (`omitempty`, so every existing owner file still decodes); `lock.Identity.Ref()` and `lock.IdentityFromRef(ref, tag, label)` convert. `stopfence.Acquire` (`fence.go:262-276`) builds its identity from the full ref and its probe compares with `identity.AliveRef` on that ref; `registry.LockedAppend` (`append.go:20`) does the same for its caller-supplied identity; the janitor's lock (S2) uses the same probe. A holder record without exact fields is probed by seconds and labelled legacy in `HolderError`. The other three `lock.Acquire` callers are unchanged and out of this goal. This unit lands before any unit that ends a process. Witness: `TestLockIdentityRoundTripsExactFields`, `TestStopfenceProbeRefusesAPidReusedWithinTheSecond`, `TestLockedAppendProbesExactHolder`.
- **X4 (every launcher signal is authenticated).** The nine raw `syscall.Kill(-suite.Process.Pid, SIGKILL)` calls (`internal/proofrun/launcher.go:231,248,256,263,273,303,318,330,368`) and the five `watchdog.Process.Kill()` calls (`:271,305,320,332,370`) go through one launcher helper that proves the recorded exact identity (`suiteExact`, `watchdogExact`, captured at `:228` and `:268`) with `identity.AliveRef` immediately beside the signal, exactly as `SignalAuthenticated` does (`internal/proofrun/watchdog.go:284-297`), through the `Prober` and `Signal` seams; a Dead or Unknown identity sends nothing and logs the refusal. `StopSuite` at `:348` already authenticates. Witness: `TestLauncherPostStartSignalsAreAuthenticated` (a table over every branch, each driven through its seam: a failing watchdog pipe, a failing watchdog start, an unobservable watchdog identity, a failed launch id, a failed record write, a failed attempt update, a failed claim close, a second-fence stop; each row asserts the signal sequence and that a changed identity receives nothing).

## 4. Fixture stewards

### 4.1 The runner bounds its own life

Today `RunLoop` stops on the stop file and the fence and on nothing else (`runner.go:167-218`); the runner is launched with `Setsid` (`runner.go:775`). `RunLoop` gains a `RunnerGuards` value (fixture authority, owner ref, lifetime, `Clock`, `Prober`, `RootExists`), built by `runStewardRun` (`cmd/metasystem/steward_verbs.go:538-574`) and injected by tests.

- **B0 (fixture authority is authenticated).** A runner is in fixture mode when, at start, the root's identity record verifies (`VerifyIdentity`) with `Enrollment == "fixture"`, or a bed marker above the root (M3) names an owner the runner can probe. `fixtureauth.FixtureModeRoot` alone never makes fixture mode. `arm` stops rewriting an enrollment to fixture from that predicate (`runner.go:662-664`): the enrollment is `fixture` only when the caller's HUMAN classification came from the fixture table (`ArmFixture`, `RestartFixture`) or a bed marker sits above the root; a `steward arm` typed at an agent-free terminal on a root whose conf says `metasystem.runtimes=fake` mints `human-terminal` and its runner is unbounded. `runnerExclusion` (`runner.go:578-589`) keeps the predicate for exclusion only. Witness: `TestArmAtHumanTerminalOnFakeRuntimesRootMintsHumanEnrollment`, `TestRunLoopGuardsHumanEnrollmentOnFakeRuntimesRootIsUnbounded`.
- **B1 (root gone).** A fixture-mode runner checks every guard interval of 5 seconds inside the 200 ms stop-file wait (`runner.go:212-217`) that `--repo` exists; ENOENT ends the loop with reason `root-gone` on stderr and in the janitor log; any other error keeps it running. Witness: `TestRunLoopExitsWhenRootGone`.
- **B2 (owner dead).** A fixture-mode runner with a bed owner probes it every guard interval; Dead ends the loop with reason `owner-dead`, logging the detection time; Unknown keeps it running. Witness: `TestRunLoopExitsWhenBedOwnerDies`, `TestRunLoopKeepsRunningOnUnknownOwner`.
- **B3 (hard lifetime).** A fixture-mode runner ends with reason `lifetime` when the clock passes its start plus `steward.fixture-runner-lifetime-min` (default 180, subject to section 15), read as `resolveProofRunLimits` reads its keys (`cmd/metasystem/proof_run.go:908-945`) and validated as an integer from 1 through 1440 by a new bounded-knob table in `internal/config/validate.go` beside the positive-integer loop at lines 501 to 511. 180 minutes is four times the committed 45 minute section cap and twice the shared Mac's 90. Witness: `TestRunLoopExitsAtFixtureLifetime`, `TestFixtureRunnerLifetimeKeyIsBounded`.
- **B4 (no husk).** On any of the three exits the runner removes its record (`runner.go:149`) and exits 0. Witness: `TestRunLoopSelfExitRemovesRecord`.
- **B5 (ignore-TERM does not disable the guards).** Witness: `TestRunLoopGuardsApplyWhileTermIsIgnored`.

### 4.2 Unrecorded and handing-over stewards

- **B6 (an unrecorded runner is an orphan by record).** A `steward run --repo <root>` process whose designated record `<root>/artifacts/agents/steward/runner.json` is absent (ENOENT) or names another identity is `orphan reason=unrecorded`; the owner proof is the record's definitive absence or mismatch, never the process name. This covers the five-day generation-1 stewards pinned to a repository root of the previous layout on m1b and m1c. The scope is the checkout whose toplevel contains `<root>`. Its ladder is TERM, `TermGrace`, KILL by identity. Witness: `TestUnrecordedRunnerIsAnOrphanByRecord`.
- **B7 (a handover is not a leak).** An unrecorded runner whose root's record was written within `janitor.handover-grace-sec` (default 120, bounded 10 through 3600) reads `handover` and is never reaped inside that grace (a steward of the previous generation still exiting after `up` re-armed). Witness: `TestHandoverGraceKeepsThePreviousGeneration`.

## 5. Beds, the janitor registry and the launcher

### 5.1 Markers and registries

- **M1 (the bed marker and the two registries).** Every bed writes, immediately after `mktemp -d` and before anything arms or spawns, `<root>/.metasystem-bed.json` through `metasystem janitor bed-open --root <root> --bed <name> --scenario <name> --pid $$ --source <harness root>`: the verb probes `$$` itself and writes `{schemaVersion:1, bed, scenario, owner: <stopfence.Process>, openedAt, deadlineEpoch, source, run: {registry, launcher: <stopfence.Process>} or null, keep:false}`, where `deadlineEpoch` is the probed start plus `fixture.bed-lifetime-min` (default 180, bounded 1 through 1440, section 15), and `run` is read from `METASYSTEM_PROOF_BED_REGISTRY` and `METASYSTEM_PROOF_LAUNCHER_REF` (M4). It then appends a `bed-opened` frame to the janitor registry (S1) under its lock and, when a run is set, one line `{root, owner, launcher, openedAt}` to the run registry file. `harness_fixture_bed_open "$tmp" <bed>` in `scripts/agents/fixture-budget.sh` wraps the verb and exports `METASYSTEM_FIXTURE_BED_ROOT` and `METASYSTEM_FIXTURE_DEADLINE_EPOCH`; `harness_fixture_bed_close "$tmp" <status> [keep]` wraps `bed-close`, which stamps `closedAt`, `status`, `keep` on the marker and appends a `bed-closed` frame. Witness: `TestJanitorBedOpenWritesProbedIdentity`, `TestJanitorBedOpenAppendsRegistryAndRunLine`, `TestJanitorBedOpenWithoutRunAppendsTheRegistryOnly`, `TestJanitorBedCloseStampsAndAppends`.
- **M2 (a fixture arm refuses without a marker).** `launchRunner` (`runner.go:764`), reached from `ArmFixture`, `RestartFixture` and any arm whose plan is a fixture enrollment (B0), refuses with `BED_UNREGISTERED: run metasystem janitor bed-open on the bed root first` when no marker sits above the root within eight levels. Registration before arm is enforced by the engine; a bed that dies before `bed-open` returns has armed nothing. Witness: `TestArmFixtureRefusesWithoutBedMarker`, `TestArmFixtureAcceptsAMarkerAbove`.
- **M3 (the runner records its bed owner).** A runner whose root sits beneath a marker copies the marker's `owner` into `runner.json` as `owner: {kind:"bed", root, ...stopfence.Process}` at start, reading the marker itself, never an environment variable. Witness: `TestRunLoopRecordsBedOwnerFromMarker`, `TestRunLoopWithoutMarkerRecordsNoOwner`.
- **M4 (one deferred cleanup obligation in the launcher, with a durable registry).** `LaunchSuite` creates `<controlRoot>/artifacts/agents/janitor/runs/<launcher pid>-<startedAtMicro>/` and exports `METASYSTEM_PROOF_BED_REGISTRY=<that dir>/beds.jsonl` and `METASYSTEM_PROOF_LAUNCHER_REF=<pid>:<startedAtSec>:<micro>:<ticks>:<bootId>` in the suite's environment (beside the existing proof variables, `launcher.go:200-212`; `proofChildEnvironment` at `:519-541` strips both from the inherited environment so a nested launcher gets its own). The registry never lives under `--tmp`: the validation child uses that directory as its stage work and removes it in its EXIT cleanup (`scripts/validate-metasystem.sh:261-268,384`), so a registry there is gone before the launcher reads it. Immediately after `suite.Start()` succeeds (`launcher.go:224`) the launcher registers one `defer` that calls `janitor.ReapRun(registry, launcherRef, ReapOptions{Prober, Signal, Clock, TermGrace, KillGrace, Log})` exactly once on every return path, including the early kill-and-return paths (`:228-235, 245-276, 299-337`). `ReapRun` reads the run registry, keeps only lines whose `launcher` equals its own identity, and for each root still on disk runs `ReapRoot` (C5); it removes the run directory after a reap with no survivor and keeps it, with the reap's log lines, when a survivor remains. A missing registry logs `no beds registered`; a torn last line is skipped. Witness: `TestLaunchSuiteReapsOnEveryPostStartReturn` (a table over the post-start returns through the existing seams; each records exactly one reap call), `TestRunRegistryLivesUnderTheControlRoot` (with `--tmp` given, the exported registry path is under the control root), `TestReapRunActsOnlyOnItsOwnLauncherLines`, `TestReapRunToleratesAbsentOrTornRegistry`.
- **M5 (the watchdog reaps too).** `stopStalledSuite` calls `ReapRun` with the same options after `SignalSuiteGroup` and before `sweepExecutionGuard` (`internal/proofrun/watchdog.go:206-211`); the watchdog receives the registry path and the launcher ref as flags from `watchdogCommand` (`launcher.go:631-666`). Witness: `TestWatchdogStallPathReapsRegisteredBeds`.
- **M6 (every temp base is covered, and the registry heals from markers).** Bed roots are found through the janitor registry's `bed-opened` frames, never by scanning a temp directory, so roots under `$TMPDIR`, `/tmp` and `/private/tmp` are reachable after their processes and run directory are gone. A live shaped process whose root carries a marker the registry does not know (after a recovery under S5, or a bed opened while the registry was moved aside) is re-indexed: the census appends a `bed-opened` frame from the marker under the lock. Witness: `TestRegistryFindsRootsUnderEveryTempBase`, `TestCensusReindexesAMarkerTheRegistryLost`.

### 5.2 The janitor registry

- **S1 (schema).** One machine-wide file, `~/.metasystem/janitor/registry.jsonl`, in registry framing (`internal/registry/framing.go`). Frames: `bed-opened {root, owner: <stopfence.Process>, openedAt, deadlineEpoch, source, run}`; `bed-closed {root, closedAt, status, keep}`; `sighted {key, ref: <stopfence.Process>, class, owner: {kind,id,root}|null, reason, at}`; `cleared {key, ref, at, why}`; `acted {key, ref, at, action, outcome, by: <stopfence.Process>}`; `torn` as the framing marker. `key` is `<pid>:<mode>:<start fields>` from `identity.Ref.ModeName()`. Every frame carries the appender's full identity in `by`. Witness: `TestJanitorRegistrySchemaRoundTrip`.
- **S2 (one lock, one transaction).** Every reader or writer acquires `~/.metasystem/janitor/lock.d` through `lock.Acquire` with the exact identity of X3 (wait 10 s scaled as `stopfence.Acquire` scales), then reads all frames, reduces, appends or replaces, and releases. A census pass appends its `sighted` and `cleared` frames and computes its decisions from the reduction plus its own frames inside the transaction; a reap appends `acted` inside its own transaction; `bed-open` and `bed-close` append inside theirs. Lock rank: the checkout's steward arbitration lock (held by the tick) is taken before the janitor lock; no path takes them in another order. Witness: `TestJanitorRegistryTransactionHoldsTheLockAcrossReadAndAppend`.
- **S3 (reduce).** Per bed root: the newest `bed-opened` and its `bed-closed`; per sighting key: the first `sighted` without a live owner is `firstSeenOrphan`, a `cleared` resets it, an `acted` closes the key. Witness: `TestJanitorRegistryReduce`.
- **S4 (bounded compaction).** Under the same lock, when the file exceeds 1 MB or on the first transaction of a day, the reduction is rewritten atomically as the new file: closed beds whose root is gone or whose `closedAt` is older than `janitor.registry-retention-days` (default 7) are dropped, as are closed sighting keys older than the same; open beds and open keys are kept whatever their age. Every append's whole-file read (`framing.go:111-115`) is therefore bounded. Witness: `TestJanitorRegistryCompactionKeepsOpenRecords`.
- **S5 (recovery).** A torn tail is tolerated by the framing rule; a `CorruptionError` (`framing.go:159-170`) moves the file aside as `registry.jsonl.corrupt-<stamp>`, starts an empty registry, logs it, and the next census re-indexes every marker it can see (M6); the cost is a reset of first-seen ages, which delays reaps and never advances them. Witness: `TestJanitorRegistryCorruptionRestartsAndReindexes`.
- **S6 (concurrent updates cannot lose each other).** Two passes, a bed-open and a manual reap racing leave every frame present; no transaction replaces the file from a reduction computed outside the lock it holds. Witness: `TestJanitorRegistryConcurrentAppendsUnderLock` (four goroutines on one fixture home).

## 6. Fixture fakes carry their own deadline

- **F1 (one absolute deadline per bed).** `METASYSTEM_FIXTURE_DEADLINE_EPOCH` is the bed's probed start plus `fixture.bed-lifetime-min`, exported by `harness_fixture_bed_open` (M1). It is wall time, not `$SECONDS`. Witness: `TestJanitorBedOpenExportsDeadline`.
- **F2 (the fixture-only verbs always require a deadline).** `util hold` (`cmd/metasystem/hold.go`) and `channel fake serve` (`cmd/metasystem/channel_verbs.go:363-376`) take `--deadline-epoch` with the variable as the default and refuse with exit 2 naming it when neither is set; they exit 70 with `deadline reached` when the injected clock passes it (`util hold`: a `select` over the signal channel and a clock timer; `channel fake serve`: the server context ends). No fixture-root detection is involved, so the two channel beds (`scripts/agents/channel-fixtures.sh:16-25`, `goal-cli-fixtures.sh:1274-1284`) and the fake adapter's hold children (`scripts/agents/adapters/fake.sh:206,263,283,318,324`) are covered by the same refusal once their beds export the variable (unit 6). Witness: `TestUtilHoldExitsAtDeadline`, `TestUtilHoldRefusesWithoutDeadline`, `TestUtilHoldReadsDeadlineFromEnvironment`, `TestChannelFakeServeEndsAtDeadline`, `TestChannelFakeServeRefusesWithoutDeadline`.
- **F3 (the shell fakes loop on the epoch).** `hosts/fake.sh`'s hold loop (`scripts/agents/hosts/fake.sh:42`), the hook bed's `deadline-engine` (`scripts/agents/supervision-hook-fixtures.sh:1766-1774`) and `supervision-hook.sh` under a fixture root loop `while (( $(date +%s) < deadline )) && kill -0 <parent>` and exit 70 past it; a shell fake started without the variable exits 2. Witness: `TestShellFakesExitAtDeadline` (a real-process test that runs each script's loop with the variable one second in the past and asserts exit 70 within ten times the expected second; bounded, not wall-fragile) and the seat's bed runs as extra witnesses.
- **F4 (the deadline exceeds every ceiling).** 180 minutes exceeds the widest scaled fixture ceiling (12 seconds at scale 48 is 576 seconds) by a factor of eighteen. Witness: `TestBedLifetimeExceedsMaximumScaledCeiling`.

## 7. The census and reap verbs

### 7.1 Names and outputs

Both live in the existing `janitor` family (`cmd/metasystem/main.go`, beside `headroom`).

- `metasystem janitor census --root <installation> [--all-owners] [--json]` prints one line per process in the janitor's world (O4):

  `ORPHAN  fixture-steward  pid=495 start=1789149385.512331 age=1d21h bound=3h owner=bed:/private/var/folders/.../tmp.4Hhb5RHimt (dead) reason=owner-dead first-seen=08:36Z action=reap`

  `ORPHAN  steward          pid=16685 start=... age=5d owner=none reason=unrecorded scope=/Users/wido/LocalStorage/GitHub/agentic-tools-m1b`

  `OWNED   fixture-hold     pid=17039 start=... age=9m bound=3h owner=bed:/private/var/.../tmp.XLvKzDZRdM (alive)`

  `LISTED  codex-broker     pid=289 start=... age=9h owner=external-lane:/Users/wido/.../worktrees/bdrb-u1a (not this goal's)`

  `LISTED  claude-tool-shell pid=53724 start=... age=12h owner=session:e90b57fc... (not this goal's)`

  `UNKNOWN fixture-steward  pid=... reason=record-unreadable`

  and a summary `census: owned=12 orphans=97 listed=68 unknown=3 outside-world=66 beds=41 (closed: 31)`. `--json` prints an array of items (`class, pid, pgid, ref{...}, ageSec, boundSec, owner{kind,id,root,ref,liveness}, state, reason, firstSeen, cwd, argv`) plus the summary. Exit 0 when the scan completed; 1 when the table could not be enumerated (a partial scan never prints a summary).
- `metasystem janitor reap --root <installation> [--owner <kind>:<id>] [--all-owners] [--plan]` acts on the orphans of the scope and prints `REAPED <class> pid=... signal=term reason=...`, `REFUSED <class> pid=... because=<younger-than-minimum|owner-alive|owner-unknown|identity-unknown|outside-scope|not-caused|grace-not-elapsed|handover|listed-only>`, `SURVIVED <class> pid=... after kill`; `--plan` prefixes `WOULD` and signals nothing. Exit 0 when every due orphan was reaped or nothing was due, 1 on a survivor, 2 on usage.
- `metasystem janitor bed-open` and `bed-close` (M1) complete the family in this goal.

### 7.2 What the census lists but never ends

So that a broker, a companion or a seat tool shell is never mistaken for a fixture orphan, the census lists them with their own classes and the state `LISTED`, and the reap refuses them with `listed-only` whatever their age: `codex-broker` and `codex-companion` by their argv shapes with owner `external-lane:<--cwd>`, and `claude-tool-shell` (argv[0] base `zsh`, `bash` or `sh`, argv[1] `-c`, argv[2] beginning `source ` followed by a path whose parent is `<home>/.claude/shell-snapshots/`) with owner `session:<id>` when its parent is an announced main of the scope, else `ownerless`. Ending them is the work of codex-jobs-run-through-a-metasystem-verb and busy-seat-shells-are-ended (sections 8 and 9). Witness: `TestCensusListsExternalLanesAndToolShellsWithoutEndingThem`.

### 7.3 Shapes

| Class | Shape (positioned) | Designated record and owner proof |
| --- | --- | --- |
| `fixture-steward`, `steward` | `<path> steward run --repo <root>` | `<root>/artifacts/agents/steward/runner.json`: names this identity (owned by its bed owner or human enrollment), another identity, or is absent (B6, B7) |
| `supervision-owner`, `supervision-component` | `metasystem supervise owner|component --tag <tag>` | the registry claim for the tag; listed; ended only through `ShutdownAt` inside `ReapRoot` |
| `fixture-hold` | `metasystem util hold --tag <tag>` (the existing `tagged-hold` shape) | the bed marker above its engine path or cwd |
| `fixture-fake-host` | the existing `host-fake-start-turn` shape | the bed marker above the script path |
| `fixture-channel-fake` | `metasystem channel fake serve --dir <dir>` | the bed marker above `--dir` |
| `fixture-hook` | `bash <bed root>/.../supervision-hook.sh <runtime> <event>` | the bed marker above the script path |
| `proof-launcher`, `proof-watchdog`, `delegate-adapter`, `mission-run-loop` | the existing shapes | their own records; listed, never ended by the janitor |
| `codex-broker`, `codex-companion`, `claude-tool-shell` | section 7.2 | listed only |
| `group-member` | any argv, pgid equal to a proven leader's pid | the leader's owner (O3) |

### 7.4 Rules

- **C1 (never a process the metasystem did not cause).** O2 and O4; `reap` refuses `not-caused` for anything outside the world even when named by `--owner`. Witness: `TestReapRefusesUnshapedProcess`.
- **C2 (scope).** Without `--all-owners` the scope of `--root <installation>` is: beds whose marker names it as `source` or whose run registry lies under it; unrecorded stewards whose root lies under its toplevel; sessions announced in its `artifacts/agents/mains/` (for listing). Everything else is `outside-scope`, counted, and printed only with `--all-owners`. Witness: `TestCensusScopeExcludesOtherSeatsOwners`.
- **C3 (`--all-owners` on reap is a human act).** It passes the agent-free-terminal gate `metasystem stop` uses (`requireHumanTerminalAt`, `cmd/metasystem/process_verbs.go:110`); on `census` it needs no gate; a pass never runs with it (R-79-m2). Witness: `TestReapAllOwnersRequiresHumanTerminal`.
- **C4 (refusals).** Younger than `janitor.minimum-age-sec` (default 120; `younger-than-minimum`); owner Alive; owner or own identity Unknown; first sighting younger than `janitor.orphan-grace-min` (default 10, bounded 1 through 1440) except `root-gone`, which acts once the minimum age has passed; `handover` (B7); `listed-only` (7.2); `outside-scope`; `not-caused`. Witness: `TestReapRefusalTable`.
- **C5 (the ladders and `ReapRoot`).** A recorded fixture steward through `steward.Disarm(root)` (X2); an unrecorded steward and a fixture fake through TERM, `TermGrace`, KILL, `KillGrace` with the exact-identity proof before each signal (`watchdog.go:284-297`); a supervision owner through `ShutdownAt` by way of the root's `arm-supervision.sh --shutdown`. `ReapRoot(root)` runs, in order: every runner record beneath the root, every supervision owner beneath it, then every shaped process whose designated record is the root's marker; it never signals an unshaped process under the root (O1), and `TermGrace` and `KillGrace` default to the proof-run's 5000 and 1000 milliseconds. Witness: `TestReapRootOrdersStewardBeforeSupervisionBeforeFakes`, `TestReapRootLeavesUnshapedProcessesUntouched`.
- **C6 (everything is logged, with times the proof can read).** Every `REAPED`, `REFUSED`, `SURVIVED`, `WOULD` line, every runner self-exit (reason, detection time, exit time), every launcher group kill and reap (kill time, reap time per root) and every pass summary is one frame in `~/.metasystem/janitor/janitor.log` (registry framing, home override honoured) carrying the writer's full identity; the log is compacted like the registry (S4) to `janitor.registry-retention-days`. Witness: `TestJanitorLogFramesCarryIdentityAndTimes`.
- **C7 (who runs it, when).** The proof launcher and the watchdog after every run (M4, M5); the steward tick of every armed checkout, once per tick after `ReapContinuations` (`internal/steward/tick.go`), as `janitor.Pass(root)`: census, then reap in scope, never `--all-owners`; `metasystem up` after `ensureStewardRunner` (`internal/up/up.go:699`) prints `janitor: orphans=N reaped=M` among its component lines; `metasystem test run` and `scripts/agents/go-gate.sh` print the census summary before a load-sensitive run and never refuse (R-35-m3). Witness: `TestTickRunsJanitorPassInScope`, `TestUpReportsJanitorLine`, `TestTestRunPrintsCensusSummaryWithoutRefusing`.
- **C8 (the health role).** `host-leaks` joins `healthRoleOrder` (`internal/steward/health.go:66-86`): alive when the last pass of the scope found zero orphans past grace; dead when an orphan is older than twice its bound; unknown when the last pass is older than two ticks or incomplete; reason `orphans=N oldest=<age>`. Witness: `TestHostLeaksRoleVerdicts`.
- **C9 (the table's parent field).** The janitor's table fills `census.Process.PPID` through `identity.ParentPid` (`internal/identity/enumerate_darwin.go:112` and the Linux reader) for every pid, so 7.2's tool-shell owner and O3's group members are read from the kernel, not guessed. Witness: `TestJanitorTableFillsParentPids` (one `os/exec` child, bounded).
- **C10 (interim).** Until unit 14 lands, the interim is the corrected bash procedure of the evidence file (`records/misc/leaked-processes-evidence-2026-09-15.txt`, the 07:03Z block): a fixture-only filter, TERM, verify, under bash. From unit 13 on, `janitor census`; from unit 14 on, `janitor reap --plan` then `reap`.

## 8. Moved to codex-jobs-run-through-a-metasystem-verb

Seed notes, not a design. The next design starts from revision 2's section 3 (archived as `leak-design-r2.md` beside the seat's critique files) and answers the findings below one by one.

- The rules as they stood: K1 launch writes a codex-job record and a machine index before returning; K2 status by id from any directory; K3 wait ends at the terminal state and releases; K4 cancel by id through the plugin's `cancel --cwd`; K5 release as a quiescence handshake (plugin store empty of non-terminal jobs, no live companion, twice, under a per-cwd lock, then `broker/shutdown`, then group TERM and KILL by identity); K6 the one-spawn residual window; K7 adopt a plugin-launched job; K8 the codex-rescue agent's coverage; K9 the upstream report (no idle timeout, shutdown accepted while a request is active, cancel by caller workspace, SessionEnd for one cwd, SessionStart re-exports); K10 the janitor backstop for a gone worktree or an idle empty store.
- Findings the next design must answer: SMLP-201, launch needs a durable terminal observer, never a later caller action; SMLP-202, a gone worktree still needs the quiescence proof (a detached worker keeps the removed directory as its cwd); SMLP-203, the plugin-direct race is unclosable from outside, so either R-113-m1e decision 2 (no plugin-direct use on seat machines) is recorded as the accepted risk with a refusal while a direct caller is possible, or a bounded mechanism is designed; SMLP-207, persist the plugin data namespace (`CLAUDE_PLUGIN_DATA`, `lib/state.mjs:29-43`) and the plugin's canonical workspace root (`git rev-parse --show-toplevel`, `lib/workspace.mjs:3-8`) in every record; SMLP-208, the plugin store is unlocked and capped at 50 jobs, so the verb keeps its own durable per-job state; SMLP-209, cancel must prove worker death, escalate, and keep a pending-release obligation until release succeeds; SMLP-210, the launch transaction needs a predeclared identity, rollback by returned job id or a store scan before reporting; SMLP-213's codex half, the machine index frames carry a full `identity.Ref` (the lock exactness lands in this goal's unit 3); round 1's SMLP-01, -03, -06, -07.
- Code facts verified: the plugin spawns the worker detached and returns the id (`codex-companion.mjs:671-708, 788-804`); the worker alone records completion (`lib/tracked-jobs.mjs:142-202`); `broker/shutdown` is accepted before the busy check (`app-server-broker.mjs:160-180`); the shutdown helper has no timeout (`lib/broker-lifecycle.mjs:43-57`); `cancel` sends one TERM to the tree and returns (`lib/process.mjs:57-118`, `codex-companion.mjs:976-1021`); `status <id> --cwd --json --wait` exists (`codex-companion.mjs:892-897`); the session filter drops when `CODEX_COMPANION_SESSION_ID` is absent (`lib/job-control.mjs:15-25`); `runDelegate` is routed at `cmd/metasystem/main.go:724`.

## 9. Moved to busy-seat-shells-are-ended

- The rules as they stood: W1 the `claude-tool-shell` positioned shape (kept here for listing, 7.2); W2 parent and CPU readers (the parent reader lands here as C9; the CPU reader moves); W3 owner by parent main, else ownerless; W4 the predicate (30 minutes, no child older than 10 seconds at every sighting, CPU delta at least 25 percent of wall); W5 TERM then KILL, log, `QueueNotification` to the session's checkout; S3's waiter reduction over the last two sightings.
- Findings the next design must answer: SMLP-204, the reduction must retain a 30 minute baseline, not two ten-minute sightings; SMLP-205, sampled child absence proves nothing (the observed loop forked continuously), so the predicate must be provable from evidence the engine reads (the shell's own accumulated CPU against its children's) and rechecked beside each signal; SMLP-206, the report goes to the shell's own session (the captured argv carries the session id and transcript path, evidence line 4), and a reparented shell needs a durable owner record; SMLP-214, a cgo-free CPU reader (Darwin: `proc_pidinfo` flavor `PROC_PIDTASKINFO` = 4, a 96 byte `proc_taskinfo` with `pti_total_user` at offset 16 and `pti_total_system` at offset 24 in mach absolute time, converted through `mach_timebase_info`, return-size checked; Linux: fields 14 and 15 of `/proc/<pid>/stat` divided by `userHZ = 100` with the existing parenthesis-safe parser, `internal/identity/identity_linux.go:44-48,115-138`); round 1's SMLP-11.
- Code facts verified: `census.Process.PPID` is 0 today (`internal/census/production.go:66`); `identity.ParentPid` exists; no native CPU reader exists (`internal/missionrunner/launch.go:850` reads `ps` cputime); `QueueNotification` stores under the checkout passed as `repoRoot` (`internal/steward/intervene.go:330-350`).

## 10. Moved to evidence-and-build-output-have-retention-and-stay-unindexed

- The rules as they stood: R1 suite-failures pruning off until `evidence.root` is real, then expiry plus a verified mirror plus no pin; R2 proof payloads by expiry and pin, never attempt records; R3 engine pins by generation and live argv reference; R4 bed roots by marker state (this goal keeps the marker and the registry the rule reads; the deletion moves); R5 the legacy roots removed by hand (done under R-112-m1e decision 3; old build output under R-113-m1e decision 3); P1 `suite-failures.noindex` with a migration symlink; P2 `testing.noindex` payload roots; P3 `engine-pins.noindex` for new pins with `InstallPath` unchanged for old enrollments; P4 `mdfind` thresholds; P5 temp bases; P6 the delegate worktree store stays with run-scoped-build-caches-have-a-janitor; the `prune` and `pin` verbs.
- Findings the next design must answer: SMLP-13 and SMLP-216, a durable-copy acknowledgement that checks the `durable` boolean `atomicfile.CopyFile` returns (the job mirror discards it, `internal/dispatch/mirror.go:327-333`) and one lock across pin check, pin creation, mirror acknowledgement and deletion; SMLP-14 and SMLP-217, the complete path inventory, which also includes `internal/proofrun/test_result_test.go:196-235`, `internal/landing/proof_receipt_test.go:90-135`, `internal/behaviorsurface/policy_test.go:408-413` and `docs/design/flight-recorder.md:342-348`; SMLP-220, the mirror's helpers are unexported and 339 lines (`mirror.go:15-30,100-115,233-281`), so an extraction unit precedes reuse.
- Code facts verified: the writers of the suite-failures path (`internal/proofrun/watchdog.go:170`, `evidence.go:121,134`, `scripts/validate-metasystem.sh:1581`, `scripts/adopt-fixtures.sh:116`, `scripts/agents/supervision-fixtures.sh:541`, `dispatch-fixtures.sh:306`, `health-fixtures.sh:119`, `goal-cli-fixtures.sh:174`, `brain-fixtures.sh:35`), its readers (`scripts/agents/suite-progress-fixtures.sh:280`, `internal/proofrun/watchdog_test.go:59`), the payload root (`cmd/metasystem/test.go:879`), the pin writers (`internal/steward/identity.go:302,378`) and their readers (`internal/up/up.go:636`, `scripts/agents/supervision-fixtures.sh:2563`), and the budget's dependence on attempt records (`internal/dispatch/budget.go:515`).

## 11. Related goals

- **fixture-stewards-outlive-their-suite: folded into this goal (R-112-m1e decision 4).** Its DONE clauses are B0 to B3 and C8; its evidence (every hand-reaped runner died on TERM) shapes the ladders.
- **codex-jobs-run-through-a-metasystem-verb, busy-seat-shells-are-ended, evidence-and-build-output-have-retention-and-stay-unindexed:** sections 8 to 10; this goal's census lists their processes and never ends them.
- **run-scoped-build-caches-have-a-janitor (approved, m1c): touched.** Shared: the `janitor` family and home; its constraints are adopted (liveness is a process reference, never age in days; verbs take `--root` and `--owner`, never path lists). This goal ends processes and keeps the bed marker and registry; every deletion of a directory belongs to that goal or to the retention goal.
- **claude-delegate-scratch-cleanup, winddown-census-handoff-leak: untouched.**

## 12. Units

Land in this order: **1, 2** (the steward runner), **3** (the lock), **4, 5, 6, 7** (registry, marker verbs, bed sites, the arm refusal), **8, 9** (shapes and owner proofs, the ladders), **10, 11** (launcher signals, launcher reap), **12** (fakes), **13, 14** (census, reap), **15** (wiring). No unit needs a later unit. Every unit allocates at most 290 changed lines including tests (plans/goals/design-allocations-leave-ceiling-margin.md sets 300); the builder counts at a checkpoint and stops at the first overrun. Every Boundary is complete; the only new Go package is none (every file lands in a package that already carries `testmain_test.go`, c15d23be). Every rule has a witness the builder runs without a fixture bed; a seat-run bed is an extra witness only. Each brief carries: the goal id, the rule ids, `Boundary`, `Ceiling: 400` with the allocation below it, `Non-goals`, the proof rule verbatim ("for every rule you add, a test that fails when that rule alone is removed; run it before you return"), the tests by name, the verification order from `metasystem/` (`go build ./...`; `go vet <packages>`; `go test -race -count=1 -timeout 40m <packages>`; `bash -n <shell files>`; `scripts/agents/go-gate.sh --fast`), and the two standing lines (no fixture bed, never `METASYSTEM_BIN`; no commit, report commands and numstat). Each unit starts a fresh chain from the preceding landed unit and is read by Opus.

### Unit 1: the steward runner's record and ladder are exact

- Rules: X1, X2.
- Boundary: `["metasystem/internal/steward/runner.go", "metasystem/internal/steward/runner_identity_test.go", "metasystem/internal/steward/health.go", "metasystem/internal/stoptransition/families.go"]`
- Non-goals: no guards, no owner field, no marker, no arm change, no lock change, no fixture bed, no commit.
- Allocation: 80 production, 140 test, total 220. Packages: `./internal/steward ./internal/stoptransition`.

### Unit 2: the fixture runner bounds its own life

- Rules: B0, B1, B2, B3, B4, B5, and the bounded-knob table that B3, B7, C4 and section 15 reuse.
- Boundary: `["metasystem/internal/steward/runner.go", "metasystem/internal/steward/runner_guards_test.go", "metasystem/cmd/metasystem/steward_verbs.go", "metasystem/cmd/metasystem/steward_verbs_test.go", "metasystem/internal/config/validate.go", "metasystem/internal/config/validate_test.go", "metasystem/metasystem.conf"]`
- Non-goals: no marker read (the guard's owner is supplied by the caller; unit 7 wires the marker), no launcher, no census, no fixture bed, no commit.
- Allocation: 120 production, 140 test, 15 config, total 275. Packages: `./internal/steward ./cmd/metasystem ./internal/config`.

### Unit 3: the lock owner is a full Ref

- Rules: X3.
- Boundary: `["metasystem/internal/lock/lock.go", "metasystem/internal/lock/lock_test.go", "metasystem/internal/stopfence/fence.go", "metasystem/internal/stopfence/fence_test.go", "metasystem/internal/registry/append.go", "metasystem/internal/registry/append_test.go"]`
- Non-goals: no change to the three other `lock.Acquire` callers, no janitor code, no fixture bed, no commit.
- Allocation: 70 production, 130 test, total 200. Packages: `./internal/lock ./internal/stopfence ./internal/registry`.

### Unit 4: the janitor registry

- Rules: S1, S2, S3, S4, S5, S6 (the `bed-opened`, `bed-closed`, `sighted`, `cleared`, `acted` frames and their reduction; the re-index of M6 is called from unit 13).
- Boundary: `["metasystem/internal/janitor/registry.go", "metasystem/internal/janitor/registry_test.go", "metasystem/internal/config/validate.go", "metasystem/metasystem.conf"]`
- Non-goals: no verbs, no marker, no census, no fixture bed, no commit.
- Allocation: 150 production, 130 test, 5 config, total 285. Packages: `./internal/janitor ./internal/config`.

### Unit 5: the bed marker verbs and the harness helpers

- Rules: M1, F1 (the run registry line is written when the environment names one; the launcher exports it in unit 11).
- Boundary: `["metasystem/internal/janitor/bed.go", "metasystem/internal/janitor/bed_test.go", "metasystem/cmd/metasystem/janitor_verbs.go", "metasystem/cmd/metasystem/janitor_bed_test.go", "metasystem/cmd/metasystem/main.go", "metasystem/scripts/agents/fixture-budget.sh", "metasystem/internal/config/validate.go", "metasystem/metasystem.conf"]`
- Non-goals: no arm refusal, no runner change, no bed call sites, no fixture bed, no commit.
- Allocation: 120 production, 110 test, 30 shell, 10 config, total 270. Packages: `./internal/janitor ./cmd/metasystem ./internal/config`. Shell: `fixture-budget.sh`.

### Unit 6: every bed opens and closes its marker

- Rules: the call sites of M1 and F1 in every bed that arms supervision, starts a fake or spawns a hold: `supervision-fixtures.sh`, `supervision-hook-fixtures.sh`, `dispatch-fixtures.sh`, `health-fixtures.sh`, `land-fixtures.sh`, `mission-fixtures.sh`, `fingerprint-harness.sh`, `delegate-caps-fixtures.sh`, `second-session-fixtures.sh`, `supervision-go-fixtures.sh`, `channel-fixtures.sh`, `goal-cli-fixtures.sh`, `fixture-bed-scenarios-fixtures.sh`, `brain-fixtures.sh`, `runtime-hook-fixtures.sh`, `telemetry-census-fixtures.sh` (it runs the fake adapter, whose children are `util hold`).
- Boundary: the sixteen scripts above under `metasystem/scripts/agents/`, plus `metasystem/internal/audit/bed_markers.go` and `metasystem/internal/audit/bed_markers_test.go`.
- Non-goals: no Go change outside the audit, no fixture bed run, no commit. The builder-runnable witness is the audit `TestEveryArmingBedOpensItsMarkerBeforeItsFirstArm`, a Go test that parses each bed and requires the helper call to precede the first `arm-supervision.sh`, `steward arm`, `channel fake serve` or `util hold` line (on top of M2's engine refusal, which lands next).
- Allocation: 90 shell, 60 production, 100 test, total 250. Packages: `./internal/audit`.

### Unit 7: the arm refusal and the runner's owner

- Rules: M2, M3, and B0's bed-owner source.
- Boundary: `["metasystem/internal/steward/runner.go", "metasystem/internal/steward/runner_owner_test.go", "metasystem/cmd/metasystem/steward_verbs.go", "metasystem/cmd/metasystem/steward_verbs_test.go"]`
- Non-goals: no launcher, no census, no fixture bed, no commit.
- Allocation: 90 production, 140 test, total 230. Packages: `./internal/steward ./cmd/metasystem`.

### Unit 8: shapes and owner proofs

- Rules: O2 (the proof side), O3, B6, B7 (classification), the shapes of 7.3 including the listed-only shapes of 7.2.
- Boundary: `["metasystem/internal/janitor/killproof.go", "metasystem/internal/janitor/killproof_test.go", "metasystem/internal/janitor/owners.go", "metasystem/internal/janitor/owners_test.go"]`
- Non-goals: no signal, no census verb, no fixture bed, no commit.
- Allocation: 140 production, 130 test, total 270. Packages: `./internal/janitor`.

### Unit 9: `ReapRoot`, the ladders and the janitor log

- Rules: C5, C6, and O2's action side.
- Boundary: `["metasystem/internal/janitor/reap.go", "metasystem/internal/janitor/reap_test.go", "metasystem/internal/janitor/log.go", "metasystem/internal/janitor/log_test.go"]`
- Non-goals: no verb, no launcher, no census, no fixture bed, no commit.
- Allocation: 140 production, 130 test, total 270. Packages: `./internal/janitor`.

### Unit 10: every launcher signal is authenticated

- Rules: X4.
- Boundary: `["metasystem/internal/proofrun/launcher.go", "metasystem/internal/proofrun/launcher_signal_test.go"]`
- Non-goals: no reap, no registry, no watchdog change, no fixture bed, no commit.
- Allocation: 100 production, 150 test, total 250. Packages: `./internal/proofrun`.

### Unit 11: the launcher and the watchdog reap what a run armed

- Rules: M4, M5.
- Boundary: `["metasystem/internal/proofrun/launcher.go", "metasystem/internal/proofrun/launcher_reap_test.go", "metasystem/internal/proofrun/watchdog.go", "metasystem/internal/proofrun/watchdog_test.go", "metasystem/cmd/metasystem/proof_run.go"]`
- Non-goals: no change to the signal helper of unit 10, no census, no fixture bed, no commit.
- Allocation: 120 production, 140 test, total 260. Packages: `./internal/proofrun ./cmd/metasystem`.

### Unit 12: fixture fakes carry their own deadline

- Rules: F2, F3, F4.
- Boundary: `["metasystem/cmd/metasystem/hold.go", "metasystem/cmd/metasystem/hold_test.go", "metasystem/cmd/metasystem/channel_verbs.go", "metasystem/cmd/metasystem/channel_verbs_test.go", "metasystem/internal/channel/fake/fake.go", "metasystem/internal/channel/fake/deadline_test.go", "metasystem/scripts/agents/hosts/fake.sh", "metasystem/scripts/agents/supervision-hook-fixtures.sh", "metasystem/scripts/agents/supervision-hook.sh", "metasystem/cmd/metasystem/shell_fakes_test.go"]`
- Non-goals: no change to what a fake does before its deadline, no reap, no fixture bed, no commit.
- Allocation: 100 production, 130 test, 50 shell, total 280. Packages: `./cmd/metasystem ./internal/channel/fake`.

### Unit 13: the census verb

- Rules: O1, O4, C2, C9, 7.2, M6's re-index, the `census` output of 7.1.
- Boundary: `["metasystem/internal/janitor/census.go", "metasystem/internal/janitor/census_test.go", "metasystem/internal/census/production.go", "metasystem/internal/census/production_test.go", "metasystem/cmd/metasystem/janitor_verbs.go", "metasystem/cmd/metasystem/janitor_census_test.go"]`
- Non-goals: no reap, no tick, no fixture bed, no commit.
- Allocation: 160 production, 120 test, total 280. Packages: `./internal/janitor ./internal/census ./cmd/metasystem`.

### Unit 14: the reap verb

- Rules: C1, C3, C4, the `reap` output of 7.1.
- Boundary: `["metasystem/internal/janitor/reap.go", "metasystem/internal/janitor/reap_verb_test.go", "metasystem/cmd/metasystem/janitor_verbs.go", "metasystem/cmd/metasystem/janitor_reap_test.go", "metasystem/internal/config/validate.go", "metasystem/metasystem.conf"]`
- Non-goals: no tick wiring, no fixture bed, no commit.
- Allocation: 120 production, 130 test, 10 config, total 260. Packages: `./internal/janitor ./cmd/metasystem ./internal/config`.

### Unit 15: the tick pass, the health role, the up line and the test-run summary

- Rules: C7, C8.
- Boundary: `["metasystem/internal/steward/tick.go", "metasystem/internal/steward/tick_janitor_test.go", "metasystem/internal/steward/health.go", "metasystem/internal/steward/health_host_leaks_test.go", "metasystem/internal/janitor/pass.go", "metasystem/internal/janitor/pass_test.go", "metasystem/internal/up/up.go", "metasystem/internal/up/up_janitor_test.go", "metasystem/cmd/metasystem/test.go", "metasystem/scripts/agents/go-gate.sh"]`
- Non-goals: no `--all-owners` in machinery, no fixture bed, no commit.
- Allocation: 130 production, 130 test, 15 shell, total 275. Packages: `./internal/steward ./internal/janitor ./internal/up ./cmd/metasystem`.

## 13. Complete rule-to-witness table

| Rule | Unit | Witness |
| --- | --- | --- |
| O1 | 13, 14 | `TestCensusScopeNeverAuthorizesAction` |
| O2 | 8, 9 | `TestReapRequiresShapeAndOwnerProof` |
| O3 | 8, 9 | `TestGroupMembersAreSignalledOnlyThroughTheLeader` |
| O4 | 13 | `TestCensusWorldMembership` |
| X1 | 1 | `TestRunnerRecordCarriesFullRef` |
| X2 | 1 | `TestDisarmRefusesAPidReusedWithinTheSecond` |
| X3 | 3 | `TestLockIdentityRoundTripsExactFields`, `TestStopfenceProbeRefusesAPidReusedWithinTheSecond`, `TestLockedAppendProbesExactHolder` |
| X4 | 10 | `TestLauncherPostStartSignalsAreAuthenticated` |
| B0 | 2, 7 | `TestArmAtHumanTerminalOnFakeRuntimesRootMintsHumanEnrollment`, `TestRunLoopGuardsHumanEnrollmentOnFakeRuntimesRootIsUnbounded` |
| B1 | 2 | `TestRunLoopExitsWhenRootGone` |
| B2 | 2 | `TestRunLoopExitsWhenBedOwnerDies`, `TestRunLoopKeepsRunningOnUnknownOwner` |
| B3 | 2 | `TestRunLoopExitsAtFixtureLifetime`, `TestFixtureRunnerLifetimeKeyIsBounded` |
| B4 | 2 | `TestRunLoopSelfExitRemovesRecord` |
| B5 | 2 | `TestRunLoopGuardsApplyWhileTermIsIgnored` |
| B6 | 8 | `TestUnrecordedRunnerIsAnOrphanByRecord` |
| B7 | 8 | `TestHandoverGraceKeepsThePreviousGeneration` |
| M1 | 5 | `TestJanitorBedOpenWritesProbedIdentity`, `TestJanitorBedOpenAppendsRegistryAndRunLine`, `TestJanitorBedOpenWithoutRunAppendsTheRegistryOnly`, `TestJanitorBedCloseStampsAndAppends` |
| M2 | 7 | `TestArmFixtureRefusesWithoutBedMarker`, `TestArmFixtureAcceptsAMarkerAbove` |
| M3 | 7 | `TestRunLoopRecordsBedOwnerFromMarker`, `TestRunLoopWithoutMarkerRecordsNoOwner` |
| M4 | 11 | `TestLaunchSuiteReapsOnEveryPostStartReturn`, `TestRunRegistryLivesUnderTheControlRoot`, `TestReapRunActsOnlyOnItsOwnLauncherLines`, `TestReapRunToleratesAbsentOrTornRegistry` |
| M5 | 11 | `TestWatchdogStallPathReapsRegisteredBeds` |
| M6 | 13 | `TestRegistryFindsRootsUnderEveryTempBase`, `TestCensusReindexesAMarkerTheRegistryLost` |
| bed sites | 6 | `TestEveryArmingBedOpensItsMarkerBeforeItsFirstArm` |
| S1 | 4 | `TestJanitorRegistrySchemaRoundTrip` |
| S2 | 4 | `TestJanitorRegistryTransactionHoldsTheLockAcrossReadAndAppend` |
| S3 | 4 | `TestJanitorRegistryReduce` |
| S4 | 4 | `TestJanitorRegistryCompactionKeepsOpenRecords` |
| S5 | 4 | `TestJanitorRegistryCorruptionRestartsAndReindexes` |
| S6 | 4 | `TestJanitorRegistryConcurrentAppendsUnderLock` |
| F1 | 5 | `TestJanitorBedOpenExportsDeadline` |
| F2 | 12 | `TestUtilHoldExitsAtDeadline`, `TestUtilHoldRefusesWithoutDeadline`, `TestUtilHoldReadsDeadlineFromEnvironment`, `TestChannelFakeServeEndsAtDeadline`, `TestChannelFakeServeRefusesWithoutDeadline` |
| F3 | 12 | `TestShellFakesExitAtDeadline` |
| F4 | 12 | `TestBedLifetimeExceedsMaximumScaledCeiling` |
| 7.2 | 13, 14 | `TestCensusListsExternalLanesAndToolShellsWithoutEndingThem` |
| C1 | 14 | `TestReapRefusesUnshapedProcess` |
| C2 | 13 | `TestCensusScopeExcludesOtherSeatsOwners` |
| C3 | 14 | `TestReapAllOwnersRequiresHumanTerminal` |
| C4 | 14 | `TestReapRefusalTable` |
| C5 | 9 | `TestReapRootOrdersStewardBeforeSupervisionBeforeFakes`, `TestReapRootLeavesUnshapedProcessesUntouched` |
| C6 | 9 | `TestJanitorLogFramesCarryIdentityAndTimes` |
| C7 | 15 | `TestTickRunsJanitorPassInScope`, `TestUpReportsJanitorLine`, `TestTestRunPrintsCensusSummaryWithoutRefusing` |
| C8 | 15 | `TestHostLeaksRoleVerdicts` |
| C9 | 13 | `TestJanitorTableFillsParentPids` |
| C10 | none | interim procedure |

## 14. Proof of DONE

The DONE is "a day of normal three-seat work that ends with no orphaned fixture process", and the bounded-time clause behind it. The proof is one calendar day that starts after unit 15 has landed on all three seats, with nothing landing on this goal during it. No retained census stream is needed: the janitor log (C6) already retains every self-exit, launcher kill and reap with its times, and the end-of-day census is the DONE artifact.

1. Before: the diagnosis numbers stand (78 to 109 orphan fixture stewards, six fakes, one hook pair, one old-layout steward). At the day's start the seat runs `metasystem janitor census --all-owners --json` from the enrolled terminal and saves it under `artifacts/reports/leak-census/<date>/start.json`.
2. During: the three seats work normally, including at least three full section runs across seats, one of which a seat ends by `TaskStop` or a deliberate section cap so the launcher's kill path is exercised, and at least one hand-run bed that is killed with SIGKILL before its trap; nobody reaps by hand.
3. Thresholds, read from the end-of-day census and the janitor log:
   - the end-of-day census, `--all-owners`, reports zero `ORPHAN` and zero `UNKNOWN` in the classes `fixture-steward`, `steward` (unrecorded), `fixture-hold`, `fixture-fake-host`, `fixture-channel-fake` and `fixture-hook`, and every `bed-opened` frame of the day has a `bed-closed` frame or a `ReapRoot` line;
   - owner-to-exit latency: for every runner self-exit of the day, exit time minus the bed's `closedAt` (closed beds) or minus the launcher's group-kill time (killed runs) is at most 60 seconds (a 5 second guard interval, with margin); for the killed run, every registered bed's steward and fakes are gone within 60 seconds of the launcher's kill time, both times from the log;
   - fakes: no fake of the day is older than its bed's `deadlineEpoch` at any pass, and every `util hold` and `channel fake serve` of the day started with a deadline (no exit 2 refusal in any bed log);
   - the human-enrolled seat stewards of the three checkouts are `OWNED` at every pass of the day.
4. The census outputs, the log extract and the numbers go into `records/misc/leaked-processes-proof-<date>.md`, the artifact the goal's conclusion cites. A threshold miss is a defect to fix forward (R-103-m1e), then the day is run again.

## 15. Budget, defaults and the approval Wido gives

The unit list is fifteen units. Base: 15 Codex builds at 45 minutes and 15 Opus reads at 20 minutes, 30 attempts and 975 reserved minutes. Rework: four rework rounds (one unit in four, the rate of this program's last two designs as the seat recorded it), each one build and one read, 8 attempts and 260 minutes. Margin: 2 attempts and 165 minutes. Requested: **40 attempts and 1400 reserved job minutes**, over three working days at one active job, or two days if Wido allows two active jobs on m1b and m1c beside the seat. No unit is authorized until Wido approves this on the goal (R-112-m1e decision 4); the folded goal's box does not transfer.

The defaults this design ships are policy and are approved with the budget, not by being configurable (SMLP-221); each is a metasystem.conf key or a named constant, so a different number is a conf line:

| Setting | Default | Where |
| --- | --- | --- |
| `steward.fixture-runner-lifetime-min` | 180 | B3 |
| `fixture.bed-lifetime-min` | 180 | M1, F1 |
| `janitor.orphan-grace-min` | 10 | C4 |
| `janitor.handover-grace-sec` | 120 | B7 |
| `janitor.minimum-age-sec` | 120 | C4 |
| `janitor.registry-retention-days` | 7 | S4, C6 |
| runner guard interval | 5 seconds (constant) | B1, B2 |
| reap `TermGrace` and `KillGrace` | 5000 and 1000 milliseconds, the proof-run's defaults | C5 |

## 16. Critique record

Round 1 (2026-09-15, Codex gpt-5.6-sol, 19 material findings, rework): dispositions as they stand after the split. SMLP-02 closed (O1, O2). SMLP-04 folded (X1 to X4). SMLP-05 folded (M2, M4 with the registry under the control root, X4). SMLP-08 closed (B0). SMLP-09 closed (F2, F3; unit 6 marks the two channel beds). SMLP-10 closed (B6, B7). SMLP-12 folded (M6, S1 to S5). SMLP-15 folded (S1 to S6, X3). SMLP-16 folded (section 12). SMLP-17 folded (section 14). SMLP-18 folded (section 15). SMLP-19 not applicable after the split (the broker lane is its own goal; the runner is first by the seat's instruction). SMLP-01, -03, -06, -07 moved to codex-jobs-run-through-a-metasystem-verb (section 8). SMLP-11 moved to busy-seat-shells-are-ended (section 9). SMLP-13, -14 moved to evidence-and-build-output-have-retention-and-stay-unindexed (section 10).

Round 2 (2026-09-15, Codex gpt-5.6-sol, 21 material findings, rework):

- SMLP-201, -202, -203, -207, -208, -209, -210: moved to codex-jobs-run-through-a-metasystem-verb, each with the line the next design must answer (section 8).
- SMLP-204, -205, -206: moved to busy-seat-shells-are-ended (section 9).
- SMLP-211 (the run registry under the child-deleted `--tmp`): folded. The registry lives under `<controlRoot>/artifacts/agents/janitor/runs/<launcher>/`, never under `--tmp`, with a witness that the exported path is under the control root even when `--tmp` is given (M4).
- SMLP-212 (raw launcher signals): folded. All fourteen post-start signal sites go through the exact-identity helper, witnessed per branch through the existing seams (X4, unit 10, before the reap unit).
- SMLP-213 (records and locks not exact): folded for the kept scope. `lock.Identity` gains the exact fields; `stopfence.Acquire` and `registry.LockedAppend` populate and probe them; the janitor's lock and every registry frame carry a full `identity.Ref`; the change is unit 3, before any unit that ends a process (X3, S1). The codex index half moved (section 8).
- SMLP-214 (the CPU reader's ABI): not applicable after the split; the kept census needs the parent reader only (C9). The ABI facts are handed over (section 9).
- SMLP-215 (append-only unbounded indexes): folded. The bed index and the sightings are one registry with one lock across read, reduce and replace, bounded compaction, mid-file recovery and re-indexing from markers (S2, S4, S5, M6). The codex index moved.
- SMLP-216, -217, -220: moved to evidence-and-build-output-have-retention-and-stay-unindexed with the facts they name (section 10).
- SMLP-218 (metrics the evidence cannot produce): folded. The narrowed DONE names no memory or load metric; the latency origins are the bed's `closedAt` and the launcher's logged kill time, both written by the engine (C6, section 14).
- SMLP-219 (units needing later units): folded. Fifteen units in dependency order; shapes (unit 8) precede the ladders (9), the launcher reap (11), the census (13) and the reap (14); M6's witness sits in the census unit (section 12).
- SMLP-220 (allocations without margin): folded on the kept scope. No unit allocates above 290; the retention mirror question moved (section 10).
- SMLP-221 (shipped defaults are policy): folded. Every default is listed for Wido's approval with the budget; nothing ships before (section 15).
- SMLP-222 (non-material, the arithmetic): folded. Section 15 states the base, the rework and the margin separately.
- SMLP-223 (non-material, citations): the partial anchors of revision 2 are corrected or no longer cited; every anchor in this revision was opened in the worktree at 0938219c.

Next step: critique round 3, the goal's last review round; then Wido's approval of the budget and the defaults of section 15; then unit 1 (the steward runner's record and ladder are exact), briefed to Codex gpt-5.6-sol with its Boundary, Ceiling, Non-goals and the proof rule, and read by Opus.
