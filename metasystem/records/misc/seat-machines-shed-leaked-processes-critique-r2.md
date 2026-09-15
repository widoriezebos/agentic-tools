# Design critique: seat-machines-shed-leaked-processes, round 2

Reviewed commit: `6f7470331c7f36a0fcc75440e8695b274630c968`

Evidence mode: read. I read revision 2, round 1, the diagnosis, the current goal text from `origin/main`, the named repository code, and Codex plugin 1.0.6. I ran no tests. I started no test, fixture, plugin, or product process. I signalled and killed no process.

Materiality criterion: Would an implementer working from this design build something DIFFERENT, or WRONG, because of this finding?

## Material findings

### SMLP-201

Severity: critical  
Material: yes

Claim: "`metasystem delegate codex <launch|status|wait|cancel|release|adopt>` is the metasystem's owner of a Codex companion job's life and of the broker it leaves behind." The design says a separate `wait` command observes terminal state and releases the broker. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:69,71-73`)

Evidence:

- Read: the plugin background command spawns and detaches `task-worker`, records it, prints `jobId`, and returns. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/codex-companion.mjs:671-708,788-804`)
- Read: the detached worker alone records `completed` or `failed`. There is no callback to metasystem. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/lib/tracked-jobs.mjs:142-202`)
- Read: revision 2 gives launch no monitor and makes release depend on a later invocation of `wait`. The seat instruction in K8 is the only thing that requires that invocation. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:73,78,230-252`)

A caller can launch successfully and never invoke `wait`. The job reaches terminal state, the detached worker exits, and no metasystem owner observes the transition. The broker remains until the 30 minute janitor backstop. D1 requires shutdown at terminal state, not shutdown if a caller remembers a second command. Launch needs a durable terminal observer or another engine-owned completion route. SMLP-06 and SMLP-07 are reopened.

### SMLP-202

Severity: critical  
Material: yes

Claim: for a definitely gone worktree, K10 skips the plugin-store and companion checks and ends the broker through the remaining release steps. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:80`)

Evidence:

- Read: a plugin task worker is detached and keeps the requested directory as its process cwd. Removing the directory entry does not end that process. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/codex-companion.mjs:671-681`)
- Read: the broker can have an active request or stream after its worktree path has disappeared. The broker tracks that state, but accepts `broker/shutdown` before its busy refusal. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/app-server-broker.mjs:68-72,160-180,197-220`)
- Read: the proposed proof deliberately removes one worktree before `wait`. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:443`)

`os.Stat` returning ENOENT proves only that the directory entry is gone. It does not prove quiescence. K10 can shut a broker while the job whose worktree was removed is still running. This directly contradicts D1 and the binding janitor rule. The gone-worktree branch must still prove that no non-terminal job and no companion exists. SMLP-03 is reopened.

### SMLP-203

Severity: critical  
Material: yes

Claim: K6 calls the plugin-direct launch race "one process spawn wide", says the request fails once, and says metasystem cannot close the window. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:76`)

Evidence:

- Read: `broker/shutdown` is accepted before the broker checks active request and stream sockets. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/app-server-broker.mjs:160-180`)
- Read: the client calls `ensureBrokerSession` once, creates one client, and initializes it once. No retry of a request interrupted after that check is present. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/lib/app-server.mjs:335-352`)
- Read: the plugin shutdown helper has no connect, response, or total timeout. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/lib/broker-lifecycle.mjs:43-57`)

The race is fundamentally unclosable from outside the plugin because a direct caller does not take the metasystem lock. Revision 2 has not bounded it. The interval after the second scan includes broker discovery, identity work, an unbounded socket operation, and shutdown. The plugin does not retry the interrupted request, so the admitted residual can fail the direct job. K8 also does not prevent a direct rescue job and a verb-owned job from sharing the main checkout. The design needs an explicit accepted-risk ruling or a bounded refusal mechanism that does not send shutdown while plugin-direct use remains possible. SMLP-03 is reopened.

### SMLP-204

Severity: critical  
Material: yes

Claim: a waiter is acted on after sightings span 30 minutes, while S3 keeps "the last two `sighted` frames" for the waiter calculation. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:136,198`)

Evidence:

- Read: the steward tick defaults to 600 seconds, or 10 minutes. (`metasystem/internal/steward/runner.go:96-104`)
- Read: ownerless waiters appear in every seat pass, which can make consecutive retained sightings even closer together. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:135`)

The last two default sightings span about 10 minutes, not 30. The reducer discards the earlier CPU and child history needed by W4. Under normal cadence the deliberate waiter never becomes actionable. D2 is not implementable from this state shape. The reducer must retain a 30 minute baseline and the observations needed to prove the whole interval. SMLP-11 is reopened.

### SMLP-205

Severity: critical  
Material: yes

Claim: W4 treats a shell as childless when no child older than 10 seconds exists at each sample. It says a shell with a long-running `go test` or bed is never a waiter. W5 then signals the shell by identity. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:136-137`)

Evidence:

- Read: the observed bad loop continuously spawned short `pgrep`, `tr`, and `echo` children. (`metasystem/records/misc/leaked-processes-diagnosis-2026-09-15.md:73-80`)
- Read: the normal observation cadence is 10 minutes. (`metasystem/internal/steward/runner.go:96-104`)
- Read: W5 specifies identity proof for the shell, but no fresh child or CPU predicate immediately before TERM or KILL. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:137`)

Samples cannot prove that no child process existed for 30 minutes. A build, test, checksum, or `awk` child can exit and respawn between samples. A child can also appear after the census decision and before TERM. Revision 2 would then kill its legitimate parent shell and may strand the child. The action must recheck the full predicate beside each signal, and the design must state what evidence can prove continuous child absence. SMLP-11 is reopened.

### SMLP-206

Severity: high  
Material: yes

Claim: an ownerless waiter is notified on "the acting checkout" even though D2 requires reporting to the waiter's session. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:135-137`)

Evidence:

- Read: the captured shell argv names m1b's session id and transcript path even though the process was later treated as reparented. (`metasystem/records/misc/leaked-processes-evidence-2026-09-15.txt:2-3`)
- Read: `QueueNotification` stores the message under the checkout path passed as `repoRoot`, and delivery resolves that checkout's configured channel. (`metasystem/internal/steward/intervene.go:330-350`; `metasystem/internal/steward/notify.go:41-88`)

With three seats, the first pass to acquire the machine lock can be from a different checkout. The message then reaches that checkout, not the shell's session. Revision 2 discards session evidence that exists in the shaped command and defines no durable owner record for a reparented shell. It does not meet D2's report destination. SMLP-11 is reopened.

### SMLP-207

Severity: high  
Material: yes

Claim: K1 records a realpath cwd and plugin path. K2, K4, and K5 later use those values to find plugin state and the broker from any directory. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:71-75`)

Evidence:

- Read: the plugin state root depends on `CLAUDE_PLUGIN_DATA`. Without it, state moves to the operating-system temp directory. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/lib/state.mjs:29-43`)
- Read: the proposed codex-job record does not store the plugin data root or resolved state directory. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:71`)
- Read: the plugin canonicalizes a cwd inside a Git repository to `git rev-parse --show-toplevel`. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/lib/workspace.mjs:3-8`; `/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/lib/git.mjs:78-87`)
- Read: task execution and broker creation use that workspace root. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/codex-companion.mjs:461-485`)

A janitor or a later shell with different plugin environment reads a different store. Two launches from different subdirectories of one worktree also take different metasystem locks and record different cwd values while sharing one plugin store and broker. Status, cancel, companion matching, and release can all address the wrong namespace. K1 must persist the plugin data namespace and the plugin's canonical workspace root, and every later verb must restore both. SMLP-01 and SMLP-06 are reopened.

### SMLP-208

Severity: high  
Material: yes

Claim: the plugin job store is the authoritative check for all non-terminal jobs in a cwd, including two jobs and all sessions. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:75,84-87`)

Evidence:

- Read: plugin updates are an unlocked read, modify, and direct rewrite of `state.json`. Parse failure is silently treated as an empty state. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/lib/state.mjs:58-77,92-121`)
- Read: each detached worker updates that shared file when it starts and ends. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/lib/tracked-jobs.mjs:142-202`)
- Read: the store keeps only 50 jobs and deletes pruned job files. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/lib/state.mjs:13,80-83,105-114`)
- Read: status and cancel resolve only through the jobs array. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/lib/job-control.mjs:213-253,281-307`)

Two jobs in one cwd can lose an update or expose a transient empty store. An older active job can also be evicted after enough newer updates. The companion-process check reduces signal risk while its worker is visible, but it does not make status or cancel by id work. The design needs its own durable per-job state or a serialized and verified plugin-store contract. SMLP-07 is reopened.

### SMLP-209

Severity: high  
Material: yes

Claim: K4 runs plugin cancel, stamps the job cancelled, and immediately runs the release handshake, thereby shutting the broker on cancel. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:74-75,86`)

Evidence:

- Read: plugin cancel sends one SIGTERM to the worker tree. It does not wait for exit and has no KILL escalation. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/lib/process.mjs:57-118`)
- Read: cancel then writes `cancelled`, clears the pid, and returns immediately. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/codex-companion.mjs:976-1021`)
- Read: K5 must refuse while the companion is still live, and K4 names no retry or waiter after that refusal. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:74-75`)

The only release attempt can correctly return `COMPANION_LIVE`. Nothing retries after the worker actually exits. The broker is not shut on cancel. K4 needs to prove worker termination, escalate if required, and keep a durable pending-release obligation until release succeeds. SMLP-06 is reopened.

### SMLP-210

Severity: high  
Material: yes

Claim: K1 is titled "launch writes the record first" and says a failed plugin task writes no record. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:71`)

Evidence:

- Read: the plugin starts the detached worker before it writes its own queued record and before it returns the job id. (`/Users/wido/.claude/plugins/cache/openai-codex/codex/1.0.6/scripts/codex-companion.mjs:684-708`)
- Read: metasystem cannot write its record or machine index until after that return supplies the id. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:71`)
- Read: the two proposed witnesses cover plugin failure and success before return. Neither covers a metasystem record failure, index append failure, or crash after plugin launch. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:71,374`)

A live job can exist with no metasystem record, or with a record that is absent from the machine index. It is then not cancellable by id from another directory and has no terminal release owner. The launch transaction needs a predeclared metasystem identity, rollback by returned job id, or recovery that scans the exact plugin store before success or failure is reported. SMLP-06 and SMLP-07 are reopened.

### SMLP-211

Severity: high  
Material: yes

Claim: M4 places the run registry under `TmpPaths[0]` when `--tmp` is supplied, then relies on a launcher defer to read it after the suite exits. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:118`)

Evidence:

- Read: validation supplies its top-level temporary directory as `--tmp` and exports that same path to the child. (`metasystem/scripts/validate-metasystem.sh:147-190`)
- Read: the child uses that path as `stage_work` and removes it in its EXIT cleanup. (`metasystem/scripts/validate-metasystem.sh:261-268,384,1498-1499,1547-1588`)
- Read: M4 says a missing registry only logs `no beds registered`. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:118`)

On a normal validation exit, the child deletes the run registry before the parent defer reads it. The one deferred cleanup obligation then has no bed list and leaks exactly what it was added to reap. The registry must live in the durable control root, outside every child-owned cleanup root. SMLP-05 is reopened.

### SMLP-212

Severity: high  
Material: yes

Claim: "Before any action" the target identity is proved, and every process-ending path re-proves identity immediately before each signal. M4 specifically names all launcher early returns. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:40,118,183`)

Evidence:

- Read: the existing launcher has direct group SIGKILL calls on the early error paths. Most do not call `AliveRef` beside the signal. (`metasystem/internal/proofrun/launcher.go:245-276,299-337,365-375`)
- Read: the watchdog already has the required helper, which proves the recorded identity immediately beside each signal. (`metasystem/internal/proofrun/watchdog.go:282-295`)
- Read: Unit 8 includes `launcher.go` but names only the deferred reap. It has no rule or witness that replaces the existing raw signals. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:286-292,398-399`)

An implementer can add the defer exactly as written and leave the launcher signals unauthenticated. That violates the binding identity rule. Unit 8 must route every existing post-start signal through the exact-ref helper and witness every branch. SMLP-04 and SMLP-05 are reopened.

### SMLP-213

Severity: high  
Material: yes

Claim: "Each record stores a full `identity.Ref`" and X3 says the codex-job index and every janitor record carry `stopfence.Process` fields. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:34,95`)

Evidence:

- Read: K1's codex machine-index frame is only `{jobId, installation, cwd, at}`. It has no process ref. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:71`)
- Read: the reused lock owner schema stores pid, whole-second start, tag, and label only. (`metasystem/internal/lock/lock.go:30-36`)
- Read: `stopfence.Acquire` converts a full ref to that whole-second lock identity and probes it by seconds. (`metasystem/internal/stopfence/fence.go:253-276`)
- Read: Units 1, 2, 6, and 12 reuse `internal/lock`, but none includes it in its Boundary. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:230-244,270-276,318-324`)

The design cannot meet the binding full-ref rule while leaving the common lock schema unchanged. A pid reused within one second can make a live lock look like the recorded holder. The codex index also cannot prove who appended it. The exact record and lock schema change must be explicit, ordered before any signal owner uses it, and included in the relevant Boundary. SMLP-04 and SMLP-15 are reopened.

### SMLP-214

Severity: high  
Material: yes

Claim: Darwin CPU time uses `proc_pidinfo(PROC_PIDTASKINFO)` and `mach_timebase_info`; Linux uses stat fields 14 and 15 "times the clock tick." (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:134`)

Evidence:

- Read: every shipped engine build sets `CGO_ENABLED=0`. (`metasystem/scripts/agents/go-build.sh:90-101`)
- Read: the existing Darwin package reaches `proc_info` with a raw `unix.Syscall6`, but only for `PROC_PIDPATHINFO`. (`metasystem/internal/identity/identity_darwin.go:145-175`)
- Read: no repository or pinned `x/sys/unix` source names `mach_timebase_info`, `PROC_PIDTASKINFO`, `pti_total_user`, or `pti_total_system`. The only match in the worktree is this design.
- Read: the Linux identity package names the cgo-free clock rate as `userHZ = 100` and has a parser that deliberately handles closing parentheses in field 2. (`metasystem/internal/identity/identity_linux.go:44-48,115-138`)
- Read: Unit 10's Boundary contains only new Go files and existing census files. It names no assembly bridge, generated constants, ABI fixture, or dependency change. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:302-308`)

The Darwin call path is not specified in a form that can build with cgo disabled. The design also omits the task-info flavor value, the 96-byte struct layout and offsets, return-size validation, byte order, overflow rules, and the source of the timebase conversion. Linux does not say divide by `userHZ` or reuse the safe stat parser. An implementer must invent platform ABI details and tests. Unit 10 needs a precise cgo-free contract for both platforms. SMLP-16 is reopened.

### SMLP-215

Severity: high  
Material: yes

Claim: the bed and codex-job indexes use registry framing and are durable machine-wide discovery sources. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:34,71,115,120`)

Evidence:

- Read: registry tail inspection reads the whole file and explicitly relies on compaction to bound its size. (`metasystem/internal/registry/framing.go:111-115`)
- Read: a garbage line followed by a later valid frame is a fail-closed `CorruptionError`. (`metasystem/internal/registry/framing.go:159-170,173-215`)
- Read: revision 2 specifies compaction and corruption recovery only for `sightings.jsonl`. It gives neither machine index retirement, compaction, nor mid-file recovery. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:196-201`)
- Read: Units 1 and 6 build append-only indexes. No later unit owns their compaction. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:230-236,270-276,318-348`)

Both indexes grow for the life of the machine, and every append pays a whole-file read. A mid-file fault can also make every later bed or job undiscoverable. Three seats and manual operations increase both costs. The indexes need one lock across read, reduce, append or replace, a bounded live-record reduction, and a fail-safe recovery rule. SMLP-07 and SMLP-12 are reopened.

### SMLP-216

Severity: high  
Material: yes

Claim: pruning deletes no suite-failure copy until its mirror is verified, and a `.metasystem-pin` always prevents removal. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:205-210`)

Evidence:

- Read: the cited job mirror's copy wrapper discards the `durable` boolean returned by `atomicfile.CopyFile`. (`metasystem/internal/dispatch/mirror.go:327-333`)
- Read: `atomicfile.CopyFile` can return `(false, nil)` after publication when directory durability is unknown. (`metasystem/internal/atomicfile/atomicfile.go:9-22,108-113,170-184`)
- Read: `pin` is specified as a marker write. Revision 2 names no lock shared by pin and prune, while three seat passes serialize only the sightings registry transaction. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:185,197,205`)
- Read: the existing registry convention holds one lock across read, reduce, and atomic replace. (`metasystem/internal/registry/compact.go:137-164`)

The described mirror can acknowledge a copy whose directory entry is not proven durable and then delete the source. A concurrent pin can also land after prune's no-pin check and before deletion. Both violate the durable-copy acknowledgement and live-use pin. Prune needs a durable outcome it checks, a stable-source rule, and a lock covering pin check, pin creation, mirror acknowledgement, and deletion. SMLP-13 is reopened.

### SMLP-217

Severity: high  
Material: yes

Claim: P1 lists every suite-failure writer and reader, and Unit 16's Boundary includes the tests that hard-code the old path. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:212,228,350-356,478`)

Evidence:

- Read: `TestSchemaTwoGroupPreservesNestedSuiteFailureEvidenceBeforeCandidateCleanup` creates and walks `artifacts/agents/suite-failures`. (`metasystem/internal/proofrun/test_result_test.go:196-235`)
- Read: landing receipt tests create and walk the same old destination. (`metasystem/internal/landing/proof_receipt_test.go:90-135`)
- Read: the behavior-surface test and flight-recorder design also hard-code the old path. (`metasystem/internal/behaviorsurface/policy_test.go:408-413`; `metasystem/docs/design/flight-recorder.md:342-348`)
- Read: Unit 16 omits all four files. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:353`)

The proofrun test runs against a fresh temporary root. It has no `up` migration symlink. Changing the evidence destination to `.noindex` makes it walk the wrong path. The builder cannot fix that failure inside the declared Boundary. The move inventory and Boundary must include every product reader, writer, test fixture, and standing contract that names the path. SMLP-14 and SMLP-16 are reopened.

### SMLP-218

Severity: high  
Material: yes

Claim: the retained stream proves owner-to-exit latency and that the janitor world's resident memory stays under 6 GB. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:444-450`)

Evidence:

- Read: W2 adds only parent pid and CPU time. It does not add resident bytes. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:134`)
- Read: the current `census.Process` has no resident-memory field. (`metasystem/internal/census/run.go:29-48`)
- Read: the proposed JSON item schema also has no resident-memory field. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:157`)
- Read: sightings store observation time, owner, CPU, and children. They do not store job terminal time, cancel time, owner death time, or root-removal time. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:196`)
- Read: the proof says it derives all latency from `sighted`, `acted`, and `cleared` frames. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:446`)

The stream cannot calculate the 6 GB metric. It also cannot place the start of each latency interval. At best it knows when a later pass observed an owner dead. No unit adds the missing resident reader or terminal event join. The proof would report numbers the designed evidence cannot produce. SMLP-17 is reopened.

### SMLP-219

Severity: high  
Material: yes

Claim: every unit Boundary is complete and nothing a unit needs lands later. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:228`)

Evidence:

- Read: Unit 6 owns M6 and promises a witness in which "the census resolves" all indexed bed roots. The census core does not land until Unit 11. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:120,270-276,310-316,400`)
- Read: Unit 8 owns complete `ReapRoot` order and every shaped fake. The new census shapes do not land until Unit 11. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:163-175,286-292,310-316`)
- Read: the current `DefaultShapes` lacks the new broker, channel-fake, hook, steward, proof-launcher, and waiter shapes. (`metasystem/internal/janitor/killproof.go:40-63`)
- Read: Unit 8's Boundary omits `killproof.go`, so it cannot build the C5 behavior it claims. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:288-290`)

Unit 6 cannot run its named witness, and Unit 8 cannot implement its full rule at its landing point. A later Unit 11 silently completes earlier units. That changes the build order and the evidence each builder must return. SMLP-16 is reopened.

### SMLP-220

Severity: high  
Material: yes

Claim: Unit 15 can implement verified tree mirroring, four retention policies, pinning, pruning, configuration, and tests in 295 changed lines, including 170 production lines. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:342-348`)

Evidence:

- Read: the cited existing mirror implementation is 339 lines and is specialized to dispatch jobs. Its source gathering and landing helpers are unexported. (`metasystem/internal/dispatch/mirror.go:15-30,100-115,233-281,327-333`)
- Read: Unit 15 does not include `internal/dispatch/mirror.go` and cannot reuse those unexported helpers. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:345`)
- Read: the repository's allocation rule records that a 390-line allocation became 474 lines and requires margin below the hard ceiling. (`metasystem/plans/goals/design-allocations-leave-ceiling-margin.md:6-10`)

The unit either duplicates a non-trivial durable mirror inside 170 production lines or changes a Boundary and extracts shared code. It also has no margin at 295. Unit 11, Unit 13, and Unit 14 are similarly allocated at 295 or 300 while each owns several mechanisms. The unit list and budget assume these do not split. An implementer will have to build a different unit graph. SMLP-16 is reopened.

### SMLP-221

Severity: high  
Material: yes

Claim: "Everything else this revision needed from Wido is decided" because retention and bound values are configurable defaults. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:457`)

Evidence:

- Read: repository policy reserves choices that affect contracts, scope, data, or user-visible behavior, and requires escalation before spending or other reserved action. (`metasystem/AGENTS.md:17-18`)
- Read: deletion of user-visible data is explicitly human-reserved. (`metasystem/docs/project-rules.md:52-61`)
- Read: D2 approves the 30 minute waiter rule. D3 approves only the one-time legacy-root removal. The current human goal text does not approve 3-day suite-failure deletion, 7-day proof-payload deletion, 180-minute fixture lifetime, 30-minute broker idle, 6 GB memory, or load thresholds. (`metasystem/plans/goals/seat-machines-shed-leaked-processes.md:8`; current reworded goal at `origin/main`, same file and line)
- Read: revision 2 selects all of those defaults and makes them implementation rules. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:80,104,111,115,136,182,187,207-210,449-450`)

Making a policy configurable does not authorize its shipped default. These values decide when live work is ended, when evidence is deleted, and whether DONE passes. Wido must approve them with the budget or the design must leave them unset and refuse the corresponding action. SMLP-18 is reopened.

## Rigor rows for material findings

| Finding id | Rigor class | Facts | Reopening trigger |
| --- | --- | --- | --- |
| SMLP-201 | severe | `local=false; recoverable=false; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | A later fold still relies on a caller invoking `wait` after launch. |
| SMLP-202 | severe | `local=false; recoverable=false; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | A gone path can still bypass the job-store or companion quiescence proof. |
| SMLP-203 | severe | `local=false; recoverable=false; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | Shutdown can still race a plugin-direct request without a measured bound and explicit ruling. |
| SMLP-204 | severe | `local=false; recoverable=false; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | The reducer still discards the baseline needed for the full waiter interval. |
| SMLP-205 | severe | `local=false; recoverable=false; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | Child absence is still sampled or is not rechecked beside every signal. |
| SMLP-206 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | An ownerless waiter can still notify a checkout other than its session. |
| SMLP-207 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | A record still omits the plugin state namespace or plugin-canonical workspace root. |
| SMLP-208 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | Status or cancel still treats the unlocked, capped plugin jobs array as authoritative. |
| SMLP-209 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | Cancel still performs only one release attempt before worker death is proved. |
| SMLP-210 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | Plugin launch can still escape without a metasystem record and indexed recovery path. |
| SMLP-211 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | A run registry can still live inside a child-owned cleanup root. |
| SMLP-212 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | Any launcher signal remains outside an immediate exact-ref proof. |
| SMLP-213 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | Any action record or machine lock still falls back to whole-second identity. |
| SMLP-214 | severe | `local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false` | The platform binding still lacks a buildable ABI and deterministic parsing contract. |
| SMLP-215 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false` | Either machine index remains append-only or lacks mid-file recovery. |
| SMLP-216 | severe | `local=false; recoverable=false; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=true; externalSideEffectBoundaryCrossed=true` | Prune can still delete after a durability-unknown copy or race a pin. |
| SMLP-217 | severe | `local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false` | Any current path consumer or hard-coded test remains outside Unit 16. |
| SMLP-218 | severe | `local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false` | DONE still cites a metric or latency origin absent from the retained schema. |
| SMLP-219 | unproven | `local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false` | An earlier unit still needs a shape, implementation, or witness that lands later. |
| SMLP-220 | unproven | `local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false` | The unit still assumes unexported mirror machinery fits without a split or extraction. |
| SMLP-221 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=true; externalSideEffectBoundaryCrossed=true` | A shipped deletion, lifetime, or proof threshold still lacks Wido's explicit approval. |

## Round-1 closure audit

| Round-1 finding | Revision-2 disposition checked | Result |
| --- | --- | --- |
| SMLP-01 | Session ownership is replaced by records and a session-unfiltered plugin store. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:84,463`) | Reopened by SMLP-207. Ambient plugin-data state still selects ownership. |
| SMLP-02 | Paths are scope only. Shape plus designated owner proof authorizes action. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:44-47,179,183,464`) | Closed. The revision states the required causation rule and refusals. |
| SMLP-03 | Store and process checks run twice before broker shutdown. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:75-76,85,465`) | Reopened by SMLP-202 and SMLP-203. Gone worktrees skip the checks, and the direct-caller window is not bounded. |
| SMLP-04 | Every record is said to carry a full ref before steward actions. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:93-95,466`) | Reopened by SMLP-212 and SMLP-213. Launcher signals and common locks remain inexact. |
| SMLP-05 | A defer is registered after `suite.Start`, and fixture arm requires a marker. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:116-119,467`) | Reopened by SMLP-211 and SMLP-212. The child deletes the registry, and raw launcher signals remain. |
| SMLP-06 | Cancel and wait are engine verbs with tests. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:73-78,86,468`) | Reopened by SMLP-201, SMLP-207, SMLP-209, and SMLP-210. Terminal observation, namespace, cancel completion, and launch recovery are incomplete. |
| SMLP-07 | Job id, plugin status, sightings idle, and backstop are named. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:71-80,87,469`) | Reopened by SMLP-201, SMLP-208, SMLP-210, and SMLP-215. State can be absent, lost, evicted, or unindexed. |
| SMLP-08 | Fixture authority comes from verified enrollment or a bed owner. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:101,470`) | Closed. The config predicate remains only for exclusion. |
| SMLP-09 | Hold, channel, adapter, host, and hook fakes receive or require a deadline. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:124-127,471`) | Closed. The named uncovered shapes now have a rule and builder-runnable witnesses. |
| SMLP-10 | Unrecorded runners and handover are explicit cases. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:110-111,472`) | Closed. Both cases have owner proof, grace, ladder, and witnesses. |
| SMLP-11 | W1 to W5 make waiters actionable. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:129-137,473`) | Reopened by SMLP-204, SMLP-205, and SMLP-206. The history cannot reach the bound, child absence is not proved, and notification can target the wrong session. |
| SMLP-12 | A machine bed index replaces temp-base scanning. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:115,120,474`) | Reopened by SMLP-215. The index has no bound or recovery from mid-file corruption. |
| SMLP-13 | Expiry, verified mirror, and live-use pins precede deletion. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:205-210,475`) | Reopened by SMLP-216. The durability result is discarded and pin races are unlocked. |
| SMLP-14 | All three high-churn stores move or are ruled out. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:212-217,476`) | Reopened by SMLP-217. The suite-failure path inventory is incomplete. |
| SMLP-15 | Sightings has schema, transaction lock, compaction, recovery, and a race witness. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:196-201,477`) | Reopened by SMLP-213. The transaction is specified, but its reused lock does not carry the required exact identity. |
| SMLP-16 | Seventeen units claim complete Boundaries and runnable witnesses. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:228-364,478`) | Reopened by SMLP-214, SMLP-217, SMLP-219, and SMLP-220. Platform details, path consumers, dependencies, and allocation are incomplete. |
| SMLP-17 | A retained stream and per-class thresholds prove DONE. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:440-451,479`) | Reopened by SMLP-218. Required memory and latency inputs are absent. |
| SMLP-18 | D3 and D4 authorize deletion, fold, and a later budget approval. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:455-457,480`) | Reopened by SMLP-221. Several new operational and deletion defaults remain human decisions. |
| SMLP-19 | Broker units are first. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:228-252,481`) | Closed. Units 1 to 3 are the broker lane. |

## Non-material findings

### SMLP-222

Severity: low  
Material: no

Claim: 17 builds, 17 reads, and four rework rounds justify 40 attempts and 1,500 reserved job minutes. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:455`)

Evidence:

- Read: 17 plus 17 plus 4 is 38 attempts if each rework round is one attempt. If a rework also receives another read, it is 42. The page does not say which.
- Read: `17 * 45 + 17 * 20 + 4 * 65` is 1,365 minutes, not 1,500. A 135-minute reserve may be sensible, but it is not identified as margin.
- Read: the claim about the last two designs' rework rate has no record or receipt citation. The estimate also omits the extra fold/read work forced by the incomplete units above.

The arithmetic does not derive the requested tuple as written. This is non-material by the binding test because correcting the explanation alone does not decide implementation. Wido should receive the actual base, explicit margin, and revised split count before approving the budget.

### SMLP-223

Severity: low  
Material: no

Claim: revision 2 presents its new file and line anchors as checked facts about the worktree and plugin. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:12`)

Evidence:

Every new revision-2 file and line anchor was opened. Material consequences are reported above.

| Design line | Cited location | Status after read |
| --- | --- | --- |
| 30 | `internal/up/up.go:659` | Holds. It writes the session announcement. |
| 34 | `internal/stopfence/fence.go:33-47`; `internal/identity/identity.go:72-93` | Holds for the full process shape and exact-mode validation. |
| 34 | `internal/registry/append.go:20`; `internal/registry/selection.go:13-26` | Partial. Append and home override hold. The appended lock owner is whole-second, as SMLP-213 reports. |
| 36 | `internal/census/run.go:209-260`; `internal/census/production.go:45-76` | Holds. Current census is checkout-scoped and production enumeration has no CPU value. |
| 40 | `internal/identity/identity.go:187-217,8-13` | Holds. Alive, Dead, and Unknown have the claimed refusal behavior. |
| 44 | `internal/census/run.go:218-244` | Holds for current path-based scope. |
| 45 | `internal/janitor/killproof.go:41-63` | Holds for the current positioned shape list. It lacks the proposed shapes. |
| 46 | plugin `lib/process.mjs:100-117` | Holds. It sends TERM to the negative pid, with a single-pid fallback. |
| 54 | `internal/census/production.go:66` | Holds. PPID is currently zero. |
| 57 | `internal/steward/runner.go:419` | Holds. `EnsureRunner` calls the exclusion there. |
| 59 | `internal/supervise/arming.go:1232` | Holds. `ShutdownAt` exists and owns the current shutdown ladder. |
| 62 | `cmd/metasystem/main.go:724` | Holds. `delegate` routes there. |
| 71 | `internal/stopfence/fence.go:254-277` | Partial. Wait scaling holds. Its lock identity is seconds-only. |
| 71 | plugin `codex-companion.mjs:762-806` | Holds. The background task grammar accepts the proposed task flags. |
| 73 | plugin `codex-companion.mjs:892-897`; `lib/tracked-jobs.mjs:156-200` | Holds. Single-job status can wait, and workers write completed or failed. Cancel writes cancelled elsewhere. |
| 74 | plugin `codex-companion.mjs:963-972` | Partial. It resolves cancel through cwd and job state. It does not prove worker exit. |
| 75 | plugin `lib/job-control.mjs:15-25`; `app-server-broker.mjs:160-164` | Holds. Removing the session variable lists all stored sessions, and shutdown is accepted unconditionally. |
| 76 | plugin `lib/broker-lifecycle.mjs:113-160`; `app-server-broker.mjs:170-175` | Partial. A later ensure can replace a stale broker and busy sockets are tracked. No current-request retry or time bound supports K6. |
| 78 | plugin `session-lifecycle-hook.mjs:83-114` | Holds. SessionEnd tears down one cwd without a quiescence check. |
| 93 | `internal/steward/runner.go:41-49,141-148` | Holds for current fields and write site. The current record lacks Darwin microseconds. |
| 94 | `internal/steward/runner.go:917-925,942-1030` | Holds for current comparison and disarm sites. Darwin currently falls back to seconds. |
| 99 | `internal/steward/runner.go:167-218,775`; `cmd/metasystem/steward_verbs.go:538-574` | Holds for the current loop, detached runner, and command binding. |
| 101 | `internal/steward/runner.go:578-589,662-664` | Holds. The config predicate controls exclusion and currently rewrites enrollment. |
| 102 | `internal/steward/runner.go:212-217` | Holds for the 200 ms stop-file wait. |
| 104 | `cmd/metasystem/proof_run.go:908-945`; `internal/config/validate.go:501-511` | Holds. The first is a bounded key reader. The second validates positivity only. |
| 105 | `internal/steward/runner.go:149` | Holds. Runner record removal is deferred. |
| 116 | `internal/steward/runner.go:764` | Holds. `launchRunner` is the fixture-arm launch site. |
| 118 | `internal/proofrun/launcher.go:200-212,224,228-235,245-276,299-337,519-541` | Holds as insertion and return-path facts. The current child-environment filter does not yet contain the two proposed keys. |
| 119 | `internal/proofrun/watchdog.go:206-211`; `internal/proofrun/launcher.go:631-666` | Holds. The first has signal then sweep. The second builds watchdog flags but not the proposed registry flags. |
| 125 | `cmd/metasystem/channel_verbs.go:363-376`; `scripts/agents/adapters/fake.sh:206,263,283,318,324` | Holds. The fake server is signal-bounded only, and all five adapter sites start `util hold`. |
| 126 | `scripts/agents/hosts/fake.sh:42`; `scripts/agents/supervision-hook-fixtures.sh:1766-1774` | Holds. Both are current loops with caller-dependent bounds. |
| 134 | `internal/identity/enumerate_darwin.go:112`; `internal/identity/identity_darwin.go:145-175` | Partial. ParentPid holds. The raw syscall example is only `PROC_PIDPATHINFO`; it does not supply the CPU ABI or timebase call. |
| 181 | `cmd/metasystem/process_verbs.go:110` | Holds. The stop command uses the human-terminal gate. |
| 183 | `internal/proofrun/watchdog.go:284-297` | Holds. It re-proves exact identity beside a signal. |
| 185 | `internal/up/up.go:699` | Holds. `ensureStewardRunner` returns there before the proposed janitor line. |
| 186 | `internal/steward/health.go:66-86` | Holds. It is the closed current health-role order. |
| 208 | `internal/dispatch/budget.go:515` | Holds. Retained attempt records are budget input. |
| 212 | `internal/proofrun/watchdog.go:170`; `internal/proofrun/evidence.go:121,134`; the nine cited scripts; `internal/proofrun/watchdog_test.go:59` | Holds for each named writer or reader. The inventory is incomplete, as SMLP-217 reports. |
| 213 | `cmd/metasystem/test.go:879` | Holds. It creates the current `testing` payload root. |
| 214 | `internal/steward/identity.go:302,378`; `internal/up/up.go:636`; `scripts/agents/supervision-fixtures.sh:2563` | Holds for both pin writers and both named old-path readers. |
| 258 | `internal/steward/health.go:782`; `internal/stoptransition/families.go:671` | Holds. Both reconstruct or print the current runner identity and need the proposed field. |
| 482 | `scripts/agents/dispatch.sh:1684`; `internal/config/validate.go:501-511`; `internal/proofrun/launcher.go:380-385`; `scripts/validate-metasystem.sh:1614-1630` | Holds as the critique-record correction. The dispatch line is only workspace assignment. The validator is positivity-only. The launcher lines are the common wait. The validation lines are unrelated audits. |

Proposed receipt: `RECEIPT type=review outcome=rework goal=seat-machines-shed-leaked-processes artifact=artifacts/reports/leak-design-critique-r2.md note="Round 2 reopened broker lifecycle, waiter proof, exact identity, registry durability, placement, unit, authority, and DONE-proof obligations."`

Verdict: rework.
