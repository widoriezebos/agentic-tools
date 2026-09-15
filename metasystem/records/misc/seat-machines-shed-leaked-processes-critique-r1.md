# Design critique: seat-machines-shed-leaked-processes, round 1

Evidence level: read. I read the design, diagnosis, raw capture, cited repository code, cited plugin code, goal record, and current goal output. I did not run a test, fixture bed, or process-changing command.

## Material findings

### SMLP-01

Severity: critical  
Material: yes

Claim: "A broker's record is its argv ... and its environment (`CODEX_COMPANION_SESSION_ID`)" and "every broker carrying that session id becomes `session-ended`." (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:37,110,114`)

Evidence:

- Read: `/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/lib/broker-lifecycle.mjs:113-117` returns any ready broker record for the cwd. It does not compare a session id.
- Read: `/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/lib/broker-lifecycle.mjs:162-169` persists endpoint, pid file, log file, session directory, and pid. It does not persist a session id.
- Read: `/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/session-lifecycle-hook.mjs:77-80` exports the current session id, but `/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/session-lifecycle-hook.mjs:83-113` tears down by cwd.

The environment belongs to the session that first spawned the broker. A later session in the same cwd reuses that broker without changing its environment. Under A1 and A5, the janitor can call the old session dead while a new session is actively using the broker. It can then end a live Codex job. The broker owner premise must be replaced or proved by live plugin state. It cannot be inferred from the broker environment.

### SMLP-02

Severity: critical  
Material: yes

Claim: "A process is in the janitor's world ... [when] its cwd, or an argv path, lies under a bed root" and `ReapRoot` sends TERM and KILL to "every remaining process the census attributes to that root." (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:54,94`)

Evidence:

- Read: the current census treats cwd or argv under a root as scope only, then separately classifies ownership (`metasystem/internal/census/run.go:218-244`).
- Read: the current signal proof requires a positioned shipped shape and claim tag, not merely a path (`metasystem/internal/janitor/killproof.go:202-220`).
- Read: the first path-based hand filter matched Codex brokers merely because their socket argv contained `/T/` (`metasystem/records/misc/leaked-processes-evidence-2026-09-15.txt:262-277`).

O6 turns location into causation. An editor, shell, debugger, evidence copier, or other user process can lawfully have a cwd or argv path below a fixture root. L1 would signal it even though the metasystem did not start it. The witness only covers an unshaped editor outside bed roots. It does not cover the dangerous case inside one. This violates C1 and can end a process the metasystem did not cause.

### SMLP-03

Severity: critical  
Material: yes

Claim: broker reaping first sends an unauthenticated `broker/shutdown` request, then "Each signal re-proves the leader's identity first." (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:111`)

Evidence:

- Read: the plugin accepts `broker/shutdown` and exits without checking whether a request or stream is active (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/app-server-broker.mjs:160-164`).
- Read: the broker does track active request and stream sockets for ordinary request exclusion (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/app-server-broker.mjs:170-175`), but the shutdown branch runs before that check.
- Read: a new caller can reuse a ready endpoint at any time (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/lib/broker-lifecycle.mjs:113-117`).

The design re-proves pid identity only before signals. It does not re-prove the orphan reason or absence of an active companion immediately before the socket shutdown. An old broker can become active between census and action. The socket endpoint can also cease to belong to the observed broker. The first action can therefore end a live job even when every later signal is identity-safe. The design needs a quiescence or ownership handshake, or a plugin-owned shutdown at job terminal state.

### SMLP-04

Severity: critical  
Material: yes

Claim: "Every owner record and every sighting carries pid plus exact start identity." O1 and O3 then specify only `startedAtMicro`. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:32,45,49,51`)

Evidence:

- Read: on Linux, `identity.Exact.Ref()` stores start ticks plus boot id and returns without setting microseconds (`metasystem/internal/identity/identity.go:49-58`).
- Read: a record with neither microseconds nor the complete ticks and boot-id pair is invalid, or falls back to whole seconds only when `StartedAtSec` exists (`metasystem/internal/identity/identity.go:72-93`).
- Read: the current runner record stores Linux ticks and boot id, but only whole seconds on Darwin (`metasystem/internal/steward/runner.go:41-49`).
- Read: `steward.Disarm` calls `sameRunner`, whose Darwin fallback compares only pid and whole-second start time (`metasystem/internal/steward/runner.go:917-924,966-994`).

The proposed marker and owner schemas cannot represent Linux exact identity. They would make owner liveness Unknown or tempt an implementer to weaken it. The reused steward ladder is not exact on Darwin and can accept a pid reused within the same second. The design must name the full `identity.Ref` schema everywhere and upgrade `RunnerRecord` and `Disarm` before claiming safe process termination.

### SMLP-05

Severity: high  
Material: yes

Claim: "After `suite.Wait()` returns (`launcher.go:380-385`, on every path including the kill paths) the launcher calls `janitor.ReapRun`." (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:94`)

Evidence:

- Read: the launcher kills the suite group, waits, and returns before the common wait at lines 380 to 385 on several paths (`metasystem/internal/proofrun/launcher.go:228-235,245-276,299-337`).
- Read: the current common wait is reached only after those returns (`metasystem/internal/proofrun/launcher.go:380-385`).
- Read: the launcher passes proof control variables to the suite, but does not itself bind `METASYSTEM_SUITE_PROGRESS_TMP` to a launcher identity (`metasystem/internal/proofrun/launcher.go:198-212`). The current outer validator supplies that variable from shell (`metasystem/scripts/validate-metasystem.sh:176-185`).

An implementer following the cited insertion point will miss kill paths on which a suite had enough time to arm a fixture. L3 then declares a missing or torn registry harmless and performs no fallback. A bed that dies before registration cannot be recovered from `beds.jsonl`. Correct placement of `bed-open` before every arm would make death before its return harmless, but the proposed source grep checks only that the helper text exists. It does not prove ordering. The run record also claims a launcher identity at line 26, while O1 and O2 store only a path and root. Nested or stale use is not fenced to one launcher. Reaping must be a single launcher cleanup obligation on every post-start return, and registration-before-arm must be structurally enforced and tested.

### SMLP-06

Severity: high  
Material: yes

Claim: after a result or cancel the seat manually runs reap, while "The engine enforces what it can" through the idle bound. The design recommends no wrapper. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:112,117,349`)

Evidence:

- Read: the binding diagnosis requires delegate-lane shutdown at terminal state and cancel, and cancel by job id regardless of caller cwd (`metasystem/records/misc/leaked-processes-diagnosis-2026-09-15.md:94,99`).
- Read: plugin cancel resolves the workspace from an explicit or current cwd before resolving the job (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/codex-companion.mjs:963-972`).
- Read: a task worker records completion and returns without shutting its broker down (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/codex-companion.mjs:838-880`).

A separate census followed by a cwd-qualified plugin command is not cancel by id from anywhere. Prose asking a seat to reap is not delegate-lane shutdown at terminal state. This leaves the main 21 GB leak on a human convention and a 30-minute fallback. The wrapper question changes whether the core requirement is implemented and must be resolved before a broker unit is briefed.

### SMLP-07

Severity: high  
Material: yes

Claim: companions are owned by a session and cwd, and a broker becomes idle only when "no companion process of that session names its cwd" for 30 minutes. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:52,110`)

Evidence:

- Read: detached task workers carry both `--cwd` and `--job-id` (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/codex-companion.mjs:671-680`).
- Read: the worker then runs the stored job with no hard process lifetime in the worker path (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/codex-companion.mjs:838-880`).
- Read: the current metasystem has only `janitor headroom`; no code currently calls the companion or implements the idle pass (`metasystem/cmd/metasystem/main.go:415-420`).

The design discards the job id that is present in argv. A stuck or leaked companion makes the broker look active forever, including during a seat session that runs for days. It has no job-terminal owner check and no hard bound. Also, Unit 4b only reports a stubbed `idle-since`, while Unit 5's rule list omits A1 (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:241-254`). No unit clearly owns the actual 30-minute state transition and reap. The intended janitor is each checkout's steward tick, but the design does not deliver a complete idle enforcer.

### SMLP-08

Severity: high  
Material: yes

Claim: fixture mode is enrollment `fixture` or `FixtureModeRoot(repo)`, yet "a human-terminal or temporary-word enrollment is never fixture mode" and a human runner is unbounded. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:79,81`)

Evidence:

- Read: `FixtureModeRoot` means only that `metasystem.runtimes=fake` is present in that root's config (`metasystem/internal/fixtureauth/fixtureauth.go:286-292`).
- Read: current arming rewrites any non-temporary enrollment to `fixture` when that config predicate is true (`metasystem/internal/steward/runner.go:662-664`).
- Read: the fixture exclusion itself is driven by the same config predicate (`metasystem/internal/steward/runner.go:578-587`).

The design's OR predicate and its human-safety witness contradict each other. A human-enrolled checkout with fake runtimes is fixture mode and can self-end on root, owner, or lifetime. A config edit after startup also changes what the next runner will be. This can end a real seat steward. Fixture authority must be tied to an authenticated enrollment or bed owner, with explicit treatment of config changes. The proposed human witness must exercise the fake-runtime case.

### SMLP-09

Severity: high  
Material: yes

Claim: "every bed calls" the marker helpers and "every fake honours" the deadline. Unit 3 covers the fake implementations. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:69,103,225-231`)

Evidence:

- Read: `channel-fixtures.sh` creates a bed root and starts `channel fake serve` from a sibling fake directory (`metasystem/scripts/agents/channel-fixtures.sh:16-38`). It is in neither the Unit 2 bed call-site boundary nor the Unit 3 boundary.
- Read: `goal-cli-fixtures.sh` creates its own temp root and later starts another channel fake (`metasystem/scripts/agents/goal-cli-fixtures.sh:43-49,1274-1284`). It is absent from the Unit 2 boundary and appears in Unit 8 only for a path rename.
- Read: the fake adapter starts several `util hold` children (`metasystem/scripts/agents/adapters/fake.sh:199-207,255-286`). Unit 3 does not include that caller in its deadline witness.

The two channel fakes have neither a bed marker export nor a fixture repository above their `--dir`. A new "refuse without deadline in a fixture root" check cannot recognize them as fixture processes. They can still start without the deadline. The fake adapter children should inherit the dispatch bed's export once every dispatch bed is marked, but no witness exercises that propagation. The supervision-hook pair and its `deadline-engine` are covered by F2 and Unit 3, and `hosts/fake.sh` is named. The two channel sites remain unowned and unbounded.

### SMLP-10

Severity: high  
Material: yes

Claim: a steward with a human enrollment is "listed, never reaped by a pass," and the fixture steward design is said to subsume the old steward goal. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:151-152,192`)

Evidence:

- Read: the raw capture contains the five-day-old generation-1 steward pinned to the repository root, not the metasystem root (`metasystem/records/misc/leaked-processes-evidence-2026-09-15.txt:280-284`).
- Read: current `RunLoop` writes its record under the supplied `--repo` root and has no owner field (`metasystem/internal/steward/runner.go:113-149`).
- Read: O3 would add a bed owner only when a marker is found above that root (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:51`).

The old-layout root has no proposed marker or migration. It will have no bed owner. Depending on stale identity data, it is human or unknown, and the pass will not reap it. R5 covers temp directories, not a repository-root runner. This exact diagnosed shape has no rule and no unit.

### SMLP-11

Severity: high  
Material: yes

Claim: the engine lists a waiter after 30 minutes and 300 CPU seconds, but "A waiter is reaped only by its own session's verb with `--waiters`, never by a pass." (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:124`)

Evidence:

- Read: the binding DONE says "a seat waiter never busy-loops" (`metasystem/plans/goals/seat-machines-shed-leaked-processes.md:8`).
- Read: the observed shell already had a live parent and consumed 68.6 percent CPU for almost twelve hours (`metasystem/records/misc/leaked-processes-evidence-2026-09-15.txt:2-3`).
- Read: the reused production process table currently records `PPID: 0` and has no cumulative CPU field (`metasystem/internal/census/production.go:45-70`; `metasystem/internal/census/run.go:29-47`).

W1 is prose. W2 detects only after substantial harm and never ends the process automatically. If the main exits and the shell reparents, the direct-child predicate also loses it. The design could add separate parent and CPU readers inside Unit 4b, but it does not name that binding or a reparented-waiter witness. The design can report some waiters but cannot prove or enforce that a waiter never busy-loops.

### SMLP-12

Severity: high  
Material: yes

Claim: R4 finds roots "by reading the user temp directory's top level for markers" while P2 leaves temp roots where they are, including `/private/tmp`. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:184,187`)

Evidence:

- Read: the diagnosed no-root channel fake used `--dir /private/tmp/...` (`metasystem/records/misc/leaked-processes-evidence-2026-09-15.txt:280-284`).
- Read: existing fixture scripts commonly choose `${TMPDIR:-/tmp}`, so Linux roots are under `/tmp`, not a macOS user temp directory (`metasystem/scripts/agents/channel-fixtures.sh:16`; `metasystem/scripts/agents/goal-cli-fixtures.sh:46`).

A marker under `/private/tmp` or `/tmp` is not found by scanning one user's macOS temp top level. Once its processes are gone and its run directory is removed, no process or durable machine index points to it. It remains forever. O2 is therefore a bed registry in all but name, but it is run-local and disposable. The root discovery contract must cover every configured temp base or maintain a durable, concurrent machine index.

### SMLP-13

Severity: high  
Material: yes

Claim: R1 deletes suite-failure entries by count and age, and "A seat that needs an entry beyond that copies it to the evidence root." (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:181`)

Evidence:

- Read: repository policy says evidence that must survive is mirrored with verified content hashes before the original counts as disposable (`metasystem/plans/README.md:3-5`).
- Read: the configured durable evidence root is still an unfilled placeholder (`metasystem/docs/project-rules.md:5-13`).
- Read: current proof evidence writers place the copy under the checkout's suite-failures tree (`metasystem/internal/proofrun/watchdog.go:169-175`; `metasystem/internal/proofrun/evidence.go:113-140`).

The proposed pruner has no proof that an entry was mirrored, no content-hash receipt, and no pin or lease for an open diagnosis. It can delete the only copy after three days or because 20 newer failures arrived. The same problem applies to proof payloads still needed to explain an open failure. Retention must consume a durable-copy acknowledgement and an explicit live-use pin before deletion. The root must be configured before pruning can be enabled.

### SMLP-14

Severity: high  
Material: yes

Claim: suite failures move to `.noindex`, but candidate trees and engine pins stay in their existing locations. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:186-188`)

Evidence:

- Read: the binding requirement says both preserved evidence and candidate trees live where Spotlight does not look (`metasystem/records/misc/leaked-processes-diagnosis-2026-09-15.md:97`).
- Read: test candidate payloads are created below the indexed checkout at `artifacts/agents/proof-runs/<attempt>/testing/<digest>` (`metasystem/cmd/metasystem/test.go:879-891`).
- Read: every executable pin is created below the indexed checkout at `artifacts/agents/steward/engine-pins` (`metasystem/internal/steward/identity.go:294-313`).
- Read: the goal's DONE requires fixture roots and build output to stay out of Spotlight (`metasystem/plans/goals/seat-machines-shed-leaked-processes.md:8`).

P1 handles only one of the measured stores. Seven-day retention does not move candidate trees out of Spotlight. R3 bounds pin count but deliberately leaves fresh executable build output in the indexed tree and under XProtect scanning. P2 has only before and after counts and no pass threshold. The proposed placement cannot prove the goal's placement clause.

### SMLP-15

Severity: high  
Material: yes

Claim: the global sightings file is "an observation, not an owner registry" and is rewritten after each census; three seat passes run concurrently under a file lock. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:39,172-173`)

Evidence:

- Read: first sighting is control state. It authorizes idle reap under A1 and grace expiry under C4 (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:110,170`).
- Read: the existing registry mutation owner locks each write in one implementation (`metasystem/internal/registry/append.go:12-32`).
- Read: existing compaction requires the same lock across read, reduce, and replace so a concurrent writer cannot be discarded (`metasystem/internal/registry/compact.go:137-157`).

This file is a process-action registry regardless of its label. The design says "file lock" but does not say that census holds it across read, merge, stale-entry reduction, and atomic replace. The only witness covers a torn temporary file. It does not cover two seat ticks, `--all-owners`, and a manual reap racing. A lost update resets first-seen age and changes whether a process is reapable. A stale merge can retain misleading state. The design needs a schema, durability owner, lock rank, full transaction, recovery rule, and a concurrent update witness.

### SMLP-16

Severity: high  
Material: yes

Claim: there are "eleven builder units of at most 400 changed lines each," and each unit carries a witness for every rule. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:4,197-199,289-335`)

Evidence:

- Read: five units allocate 390 or 395 lines (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:209-223,233-255,265-271`). The repository already records that 390-line allocations became 474 lines and sets a 300-line design allocation for this reason (`metasystem/plans/goals/design-allocations-leave-ceiling-margin.md:6-10`).
- Read: Unit 1b owns F1 and its new config key, but its Boundary omits `internal/config/validate.go`, its test, and `metasystem.conf` (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:209-215`).
- Read: the C1 reap witness is assigned to Unit 4a even though that unit forbids reap and omits `reap.go`; Unit 5's rule list omits C1 (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:233-255,319`).
- Read: A1 and W2 are assigned to Units 4b and 5 in the witness table, but Unit 5's brief omits both rule ids (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:241-254,311,318`).
- Read: Unit 8 changes the watchdog path but omits the existing test that hard-codes the old path (`metasystem/internal/proofrun/watchdog_test.go:45-65`; boundary at `metasystem/plans/seat-machines-shed-leaked-processes-design.md:273-279`).
- Read: P2 is witnessed only by receipt measurements, while C9 and P3 have no code witness (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:327,334-335`).

The units are not proven below the ceiling and several cannot implement their own rule inside their Boundary. Units 2, 3, 6, and 8 also name fixture legs as required witnesses while every builder brief forbids running a fixture bed. Their builders cannot return the promised rule-removal proof. These are slicing defects, not later implementation details.

### SMLP-17

Severity: high  
Material: yes

Claim: a calendar day after Unit 6, while Units 7 to 9 may land during it, proves DONE when every tick in the last six hours is clean "read from the ledger," memory is below 4 GB, and load is lower. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:339-343`)

Evidence:

- Read: the ledger schema stores one first-seen time and reason per current process. It has no per-tick history (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:39,110,172`).
- Read: Unit 8 is the placement change, yet the proof may start and mostly run before Unit 8 lands (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:257-279,339`).
- Read: the goal requires a day that proves no process outlives its owner beyond a bound, waiters never busy-loop, and fixture roots and build output stay out of Spotlight (`metasystem/plans/goals/seat-machines-shed-leaked-processes.md:8`).
- Read: the current janitor command has only `headroom`; there is no existing retained census stream that could silently supply the missing history (`metasystem/cmd/metasystem/main.go:415-420`).

The ledger cannot prove six hours of tick results. A day whose implementation changes midway is not a day on the final system. An end-only waiter count misses earlier loops. The proof has no maximum indexed-file threshold for candidate trees or pins, no retained census stream, no owner-to-exit latency distribution, and no controlled definition of comparable load. The 4 GB threshold is not tied to a per-class expected workload. This proof cannot establish the DONE clause.

### SMLP-18

Severity: high  
Material: yes

Claim: the design is waiting on four human questions, but its next step is Unit 1a; it also says the seat may choose destructive retention and lifetime numbers without a question. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:7,9,345-352,358`)

Evidence:

- Read: the current goal record allows 10 attempts and 1200 reserved job minutes (`metasystem/plans/goals/seat-machines-shed-leaked-processes.md:13`). The design itself estimates about 22 attempts and asks for 24 and 1800 (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:347`).
- Read: spending past a stated budget is human-reserved (`metasystem/AGENTS.md:17-18`; `metasystem/docs/project-rules.md:52-61`).
- Read: concluding or folding a human-opened goal is human-reserved (`metasystem/AGENTS.md:29-35`).
- Read: the goal engine refuses agent conclusion of a human-origin goal (`metasystem/internal/goal/verbs.go:1933-1944`) and requires a human actor for `set-budget` (`metasystem/internal/goal/verbs.go:1119-1140`).
- Read: R5 can delete unmarked user temp trees by a broad name and content heuristic (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:185,348`).

The design cannot authorize its own budget extension, fold the related human goal, permit the one-time legacy deletion, or settle the broker wrapper and worktree placement. The wrapper and placement questions change whether requirements 1, 4, and 6 are met. The retention and kill bounds also determine when user evidence is deleted and live work is ended. Unit 1a is not an authorized next step until Wido answers the choices that affect the design.

### SMLP-19

Severity: medium  
Material: yes

Claim: the ordered build handles fixture stewards first and brokers in Units 4b and 5. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:201-255`)

Evidence:

- Read: the measured Codex family was about 21 GB and 574 processes, while fixture stewards and helpers were about 2.1 to 3 GB (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:15`).
- Read: the plugin has no idle timer and shuts down only on request or signal (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/app-server-broker.mjs:160-164,236-244`).

The order does not stop the biggest measured leak first. Units 1a through 4a can land while the 21 GB broker family continues accumulating. Once SMLP-01, SMLP-03, SMLP-06, and SMLP-07 are resolved, the safe automatic broker terminal path should precede broad retention and probably precede census generalization. The fixture runner self-bound remains valuable, but it is not the largest load reduction.

## Non-material findings

### SMLP-20

Severity: low  
Material: no

Claim: the design presents its file and line citations as facts about current repository and plugin code. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:11`)

Evidence:

Every cited current-code and plugin anchor was opened. Status follows. The material consequences of false or partial anchors are already reported above.

| Design line | Cited location | Status after read |
| --- | --- | --- |
| 26 | `metasystem/internal/proofrun/launcher.go:144` | Holds. It appends the progress header. It does not write the proposed run owner. |
| 27 | `metasystem/internal/up/up.go:659` | Holds. It announces the session. |
| 28 | `metasystem/scripts/agents/dispatch.sh:1684` | Partial. It assigns the worktree path. That line does not write the claimed pid, start, or instance tag. |
| 29 | plugin `lib/broker-lifecycle.mjs:61` | Holds. Broker argv includes `--cwd`. |
| 32 | `metasystem/internal/identity/identity.go:50-59` | Holds. It also disproves the microseconds-only Linux schema in SMLP-04. |
| 34 | `metasystem/internal/steward/runner.go:41-49` | Holds for the record location and current fields. It has no owner and Darwin is seconds-only. |
| 37 | plugin `session-lifecycle-hook.mjs:77-81`; `lib/broker-lifecycle.mjs:59-67` | Holds for export and inheritance at initial spawn. It does not hold for ownership after cwd-based broker reuse. |
| 39 | `metasystem/internal/registry/selection.go:13-26` | Holds for absolute registry-home redirection. The cited code does not by itself prove every fixture sets the override. |
| 41 | `metasystem/internal/census/run.go:209-260`; `production.go:45-76` | Holds for path scope and live enumeration. The current production row has no parent or CPU value. |
| 45 | `metasystem/internal/identity/identity.go:187-217,8-13`; `proofrun/launcher.go:56-57` | Holds for three-way liveness and injected launcher seams. Proposed records still must carry a valid native ref. |
| 49 | `metasystem/scripts/validate-metasystem.sh:1614-1630` | Does not hold as the claimed analogous header grep. Those lines run the audit and check brief-template headers. They do not verify marker-helper ordering. |
| 51 | `metasystem/internal/steward/runner.go:113` | Holds as the current `RunLoop` entry. No marker read exists today. |
| 52 | `metasystem/internal/identity/identity_darwin.go:105-143` | Partial. The raw buffer contains environment after argv, but the function parses argv and discards the remaining bytes. |
| 53 | plugin `lib/process.mjs:100-117` | Holds. It attempts TERM on the negative pid, then the pid on a non-ESRCH group error. |
| 54 | `metasystem/internal/janitor/killproof.go:41-63` | Holds for current shipped shapes. Current shapes require positioned tags. |
| 63 | `metasystem/internal/steward/runner.go:419` | Holds only as the call site. The actual config-based exclusion is at lines 578 to 587. |
| 65 | `metasystem/internal/supervise/arming.go:1232` | Holds. `ShutdownAt` exists. |
| 71 | plugin `codex-companion.mjs:963-975`; `app-server-broker.mjs:160-164` | Holds. Cancel accepts `--cwd`; shutdown accepts the socket request. |
| 79 | `metasystem/internal/steward/runner.go:167-218,775`; `identity.go:40`; `notify.go:49`; `fixtureauth.go:292`; `steward_verbs.go:538-574` | Holds for current loop, Setsid, enrollment constant, notification read, config predicate, and command binding. The claim that human enrollment is never fixture mode does not hold, as SMLP-08 shows. |
| 82 | `metasystem/internal/steward/runner.go:212-217` | Holds for the current 200 ms wait. |
| 84 | `metasystem/internal/config/validate.go:501-511`; `cmd/metasystem/proof_run.go:908-945` | Partial. The config loop validates positivity only, not a maximum of 1440. The proof-run reader does implement bounded reads for its own keys. |
| 85 | `metasystem/internal/steward/runner.go:149` | Holds. The runner record is removed on return. |
| 86 | `metasystem/cmd/metasystem/steward_verbs.go:548-554` | Holds. Ignore-TERM is gated by the config fixture predicate. |
| 92 | `metasystem/internal/proofrun/launcher.go:213,231-335`; `watchdog.go:199-211` | Holds for group creation, kill branches, and watchdog order. |
| 94 | `metasystem/internal/proofrun/launcher.go:380-385` | Does not hold for "every path." Several kill-and-return paths wait earlier and never reach these lines. |
| 95 | `metasystem/internal/proofrun/watchdog.go:206-211` | Holds for the current signal then execution-guard sweep adjacency. No reap exists there today. |
| 100 | `metasystem/cmd/metasystem/hold.go:30-33`; `scripts/agents/hosts/fake.sh:42`; `cmd/metasystem/channel_verbs.go:363-376`; `scripts/agents/supervision-hook-fixtures.sh:1766-1774` | Holds for the four current unbounded or caller-state-bound waits. |
| 108 | plugin `lib/broker-lifecycle.mjs:59-67`; `app-server-broker.mjs:160-164,236-244`; `session-lifecycle-hook.mjs:83-114` | Holds for detached spawn, explicit shutdown and signal handlers, and cwd-only SessionEnd cleanup. Fatal main errors are an additional exit path at `app-server-broker.mjs:249-250`. |
| 114 | `metasystem/scripts/agents/supervision-hook.sh:2486-2488` | Holds. SessionEnd calls `up --retire`. |
| 169 | `metasystem/cmd/metasystem/process_verbs.go:110` | Holds. Stop uses the human-terminal gate. |
| 171 | `metasystem/internal/steward/runner.go:942-1005`; `proofrun/watchdog.go:284-297` | Partial. Both recheck before signals, but steward identity is whole-second on Darwin. |
| 173 | `metasystem/scripts/agents/supervision-hook.sh:1054-1059`; `internal/up/up.go:699` | Holds for the hook calling up and the current runner call. The proposed janitor pass does not exist. |
| 175 | `metasystem/records/misc/leaked-processes-evidence-2026-09-15.txt:262-278` | Holds. It records the corrected bash reap and the unsafe earlier filter. |
| 179 | `metasystem/internal/config/validate.go:501-511` | Partial. It provides positive-integer validation only. Per-key maxima need new code and tests. |
| 182 | `metasystem/internal/dispatch/budget.go:515` | Holds. The budget reads retained proof attempts. |
| 184 | `metasystem/scripts/agents/supervision-fixtures.sh:538-539` | Holds. The keep variable preserves the root. |
| 186 | `metasystem/internal/proofrun/watchdog.go:170`; `evidence.go:121,134`; `scripts/validate-metasystem.sh:1581`; `scripts/adopt-fixtures.sh:116`; `scripts/agents/supervision-fixtures.sh:541`; `dispatch-fixtures.sh:306`; `health-fixtures.sh:119`; `goal-cli-fixtures.sh:174`; `brain-fixtures.sh:35`; `suite-progress-fixtures.sh:280` | Holds for every named current writer and reader. It is not a complete reader inventory because `metasystem/internal/proofrun/watchdog_test.go:59` also requires the old path and is absent from Unit 8. |
| 194 | `metasystem/scripts/agents/dispatch.sh:1363-1367`; `internal/gittree/detached.go:40` | Holds. Delegate worktrees remain, and detached receipt worktrees use a temp parent. |

Proposed receipt: `RECEIPT type=review outcome=rework goal=seat-machines-shed-leaked-processes artifact=artifacts/reports/leak-design-critique-r1.md note="Round 1 found unsafe broker ownership, path-as-causation, incomplete identity records, uncovered leak shapes, and a proof that cannot establish DONE."`

Verdict: rework.
