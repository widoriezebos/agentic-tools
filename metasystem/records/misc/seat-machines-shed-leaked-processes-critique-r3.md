# Design critique: seat-machines-shed-leaked-processes, round 3

Reviewed commit: `6f7470331c7f36a0fcc75440e8695b274630c968`

Evidence level: read. I read revision 3, both earlier critiques, the diagnosis, R-113-m1e, the narrowed goal on `origin/main`, and the cited code. The worktree base has no `bin/metasystem`, so the current goal was read from Git instead of through `goal show`. I ran no tests. I started, signalled, or killed no process.

Materiality criterion: Would an implementer working from this design build something DIFFERENT, or WRONG, because of this finding?

## Material findings

### SMLP-301

Severity: critical  
Material: yes

Claim: A fixture runner "checks every guard interval of 5 seconds inside the 200 ms stop-file wait" and exits when its root is gone, its owner is dead, or its hard lifetime expires. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:79-83`)

Evidence:

- Read: `RunLoop` calls `RunTick`, revival, notification delivery, and the channel phase synchronously before it enters the only 200 ms wait loop. (`metasystem/internal/steward/runner.go:167-217`)
- Read: `RunTick` calls a custodian that can run `dispatch.sh` through `CombinedOutput` with no context or timeout. (`metasystem/internal/steward/tick.go:69-85,150-154`)
- Read: `RunTick` also holds its arbitration lock across its synchronous work. (`metasystem/internal/steward/tick.go:104-114`)

A blocked tick never reaches the proposed guard checks. The owner can die and the lifetime can pass while the runner remains alive without a bound. Unit 2 therefore cannot implement B1 through B3 by adding checks only to the wait loop. The design needs a guard execution boundary that remains live while tick work blocks.

### SMLP-302

Severity: critical  
Material: yes

Claim: X2 makes "the ladder" exact by changing `liveRunner`, `sameRunner`, and `Disarm`, and Unit 1 owns the runner ladder. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:68,203-208`)

Evidence:

- Read: automatic repair and explicit arm replacement both call `stopRunnerForReplacement`. (`metasystem/internal/steward/runner.go:547-562,681-699`)
- Read: `stopRunnerForReplacement` sends TERM and CONT to the recorded pid without a fresh identity proof beside either signal. Its later KILL check compares only the pid after reading the record. (`metasystem/internal/steward/runner.go:797-816`)
- Read: `Disarm` has the stronger pattern the design claims. It rereads and compares the runner immediately before TERM and again before KILL. (`metasystem/internal/steward/runner.go:966-994`)

The replacement path is a kept process-ending path, but no rule or witness names it. An implementer can complete X2 exactly as written and leave TERM able to hit a reused pid. SMLP-213 is reopened.

### SMLP-303

Severity: critical  
Material: yes

Claim: "Each record stores a full `identity.Ref`" and `ReapRoot` reuses `supervise.ShutdownAt` for supervision. Unit 9 owns C5 inside only four janitor files. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:33,59,165,259-264`)

Evidence:

- Read: `ArmingOwner` has seconds, Linux ticks, and boot id, but no Darwin microsecond field. Its comparison and liveness conversion fall back to whole seconds on Darwin. (`metasystem/internal/supervise/arming.go:25-34,256-271`)
- Read: recorded supervision components are also reconstructed without a Darwin microsecond field. (`metasystem/internal/supervise/arming.go:531-560`)
- Read: the orderly owner's component ladder probes before TERM, but sends KILL after the wait without a new identity proof beside KILL. (`metasystem/internal/supervise/proc.go:184-200`)
- Read: no `internal/supervise` file is in Unit 9's Boundary. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:259-264`)

`ShutdownAt` is not safe to reuse unchanged under the design's exact-identity rule. On Darwin it can authorize from a whole-second record. Its orderly component path also fails the beside-each-signal rule. The required schema, ladder, tests, Boundary, and allocation are absent. SMLP-213 and SMLP-219 are reopened.

### SMLP-304

Severity: critical  
Material: yes

Claim: all fourteen launcher signals use `suiteExact` or `watchdogExact`, and Dead or Unknown sends nothing. The witness includes an unobservable watchdog identity. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:70`)

Evidence:

- Read: the suite-identity failure branch exists because the probe at line 228 did not produce an Alive exact identity. The current code then kills by pid, waits, and returns. (`metasystem/internal/proofrun/launcher.go:224-234`)
- Read: the watchdog-identity failure branch likewise has no proven Alive `watchdogExact`. It currently kills and waits. (`metasystem/internal/proofrun/launcher.go:261-276`)
- Read: the proposed helper pattern refuses a signal unless a fresh `AliveRef` check returns Alive. (`metasystem/internal/proofrun/watchdog.go:282-295`)

Replacing those two raw kills with the named helper produces no signal and then waits without a bound. The test list does not name the unobservable suite-identity branch, and the unobservable watchdog row asserts signal behavior but no bounded return. The design needs a safe start handshake or a bounded failure contract for a child whose exact identity cannot be captured. SMLP-212 is reopened.

### SMLP-305

Severity: critical  
Material: yes

Claim: the launcher reaps what its run armed on every post-start return, but a missing run registry logs `no beds registered` and does nothing. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:4,97`)

Evidence:

- Read: `LaunchSuite` starts the suite with the launcher's credentials and passes control paths in its environment. The design adds the run-registry path to this same environment. (`metasystem/internal/proofrun/launcher.go:198-213`)
- Read: `proofChildEnvironment` removes inherited variable names. It creates no filesystem authority boundary between the suite and the launcher. (`metasystem/internal/proofrun/launcher.go:515-540`)
- Read: the existing suite already removes child-owned working state from its EXIT trap. (`metasystem/scripts/validate-metasystem.sh:259-268,382-389`)

The suite can unlink the exported registry because it runs as the same user. The required case in which the suite deletes its own registry therefore takes the design's no-op branch. The machine registry already has launcher identity in each bed-opened record, but M4 does not use it as a recovery source. The launcher will not reap the beds it armed, and the 60 second proof can fail.

### SMLP-306

Severity: critical  
Material: yes

Claim: `proofChildEnvironment` strips the enclosing run registry so "a nested launcher gets its own", and each launcher and watchdog reaps only that registry. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:97-98`)

Evidence:

- Read: each suite is placed in a new process group, and each watchdog is placed in another new process group. (`metasystem/internal/proofrun/launcher.go:198-213,243-244`)
- Inferred: a nested launcher itself inherits the outer suite's process group. Its inner suite and watchdog do not. This follows from the two `Setpgid` sites above and the absence of a group change for the launcher process.
- Read: the watchdog records no launcher identity and does not watch launcher death. A passed deadline and section cap only produce notes. The watchdog stops a suite only for a progress verdict or a cancellation intent. (`metasystem/internal/proofrun/watchdog.go:20-46,48-90`)

When an outer launcher kills its suite group, it can kill the nested launcher while leaving the inner suite and inner watchdog in their separate groups. The nested defer cannot run. The outer registry deliberately cannot see the inner beds. A progressing inner suite can keep running because the watchdog does not act on launcher death or elapsed time. No witness covers this kill ordering. Nested cleanup needs an ownership link that survives the nested launcher.

### SMLP-307

Severity: critical  
Material: yes

Claim: O2 requires a shape and owner proof, while the direct `ReapRoot` ladder re-proves only exact target identity before each signal. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:44,165`)

Evidence:

- Read: the cited `SignalAuthenticated` helper checks only the target `identity.Ref`. It does not reread a designated owner record or the action predicate. (`metasystem/internal/proofrun/watchdog.go:282-295`)
- Read: the existing runner ladder rereads `runner.json` immediately before TERM and KILL and refuses when its identity changed. (`metasystem/internal/steward/runner.go:966-994`)
- Read: the new unrecorded-runner decision can be based on an absent or mismatched `runner.json`. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:87-88`)

An unrecorded runner can publish its matching record after census classification and before TERM. The target identity is still Alive, so the proposed helper authorizes the signal even though the owner predicate has changed. B7 does not close this race because it is evaluated before the action. The exact target and the designated owner proof must both be reread beside each signal.

### SMLP-308

Severity: high  
Material: yes

Claim: `bed-open` appends "one line" to the per-run `beds.jsonl`; `ReapRun` tolerates an absent file or torn last line. S2 defines one transaction for the machine-wide janitor registry. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:94,97,101-108`)

Evidence:

- Read: the existing framing owner states that callers must hold a registry lock. Framing itself does not lock. (`metasystem/internal/registry/framing.go:30-40`)
- Read: a bad middle line followed by a valid record is a fail-closed corruption, not a torn-tail case. (`metasystem/internal/registry/framing.go:159-180`)
- Read: the existing locked append serializes one append only. A read, reduce, and replace transaction needs an explicitly shared lock. (`metasystem/internal/registry/append.go:12-32`; `metasystem/internal/registry/compact.go:137-157`)

The design does not say whether the run file uses registry framing, whether its writer and both reapers take the machine lock, what happens on middle corruption, or how a reap excludes a concurrent late append. The named concurrency and recovery witnesses cover only the machine file. Different implementers can build incompatible run registries, and one permitted implementation can miss a bed.

### SMLP-309

Severity: high  
Material: yes

Claim: S4 says compaction makes every whole-file append bounded while "open beds and open keys are kept whatever their age." (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:106`)

Evidence:

- Read: registry tail inspection reads the whole file and explicitly relies on compaction to bound its size. (`metasystem/internal/registry/framing.go:111-119`)
- Read: the proof deliberately kills a hand-run bed with SIGKILL before its trap. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:361-366`)
- Read: only `bed-close` stamps `closedAt` and appends `bed-closed`. `ReapRoot` and `ReapRun` log their actions but do not close the bed record. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:94,97,165-166`)

Every bed that loses its trap remains open forever even after all of its processes are reaped. S4 must keep it forever. The required hand-run proof creates at least one such record. The registry therefore has no growth bound, and every append eventually reads an unbounded file. SMLP-215 is reopened.

### SMLP-310

Severity: high  
Material: yes

Claim: the narrowed DONE requires the census to name every metasystem-caused process by owner, age, and bound. Revision 3 lists brokers and tool shells as `external-lane:<cwd>` or `session:<id>` and leaves their lifecycle to satellite goals. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:5,31,123-143`)

Evidence:

- Read: the current process and inventory schemas have no bound field. (`metasystem/internal/census/run.go:29-64`)
- Read: the current session announcement records one main process and its session lineage, not a broker owner, tool-shell owner record, or lifetime bound. (`metasystem/internal/lease/classify.go:21-35`)
- Read: `metasystem up` writes only that session announcement. (`metasystem/internal/up/up.go:655-667`)

The kept Unit 13 has no source for a broker's owner or bound. A cwd is a location, not an owner. A reparented tool shell is explicitly `ownerless`, and neither listed class has a bound in the human output. The implementer must either invent values, report missing values contrary to DONE, or wait for the moved goals. This is a real dependency across the split. The hand-off sections otherwise remain seed notes and do not authorize broker, shell, or retention implementation in this goal.

### SMLP-311

Severity: high  
Material: yes

Claim: no retained census stream is needed because the janitor log can prove every owner-to-exit latency and the state of every fake and human steward at every pass. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:357-368`)

Evidence:

- Read: the process prober returns a current exact identity and three-way liveness. It has no owner-death timestamp. (`metasystem/internal/identity/identity.go:168-176`)
- Read: the current census verdict is one snapshot with one completion time and one inventory. (`metasystem/internal/census/run.go:66-90,268-289`)
- Read: C6 records runner detection and exit times, launcher kill times, reap times, and pass summaries. It does not record the death time of a bed owner or the per-process state of every pass. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:166`)
- Read: S4 may compact during the day whenever the registry exceeds 1 MB. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:106`)

The required hand-run bed is killed before its trap, so it has neither `closedAt` nor a launcher group-kill time. Its owner-to-exit interval cannot be computed from the named evidence. A pass summary also cannot prove that each human steward was OWNED and no fake exceeded its deadline at every pass. Compaction can remove the detailed sightings before the day ends. SMLP-218 and the round-1 proof finding are reopened.

### SMLP-312

Severity: high  
Material: yes

Claim: all fifteen Boundaries are complete, every rule has a builder-runnable no-bed witness, and the budget estimate is based on those fifteen units. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:201,259-306,308-355,370-385`)

Evidence:

- Read: the required supervision identity and ladder work is owned by `internal/supervise/arming.go` and `internal/supervise/proc.go`, but neither file nor its tests occur in any Boundary. (`metasystem/internal/supervise/arming.go:25-34,531-560`; `metasystem/internal/supervise/proc.go:180-210`; `metasystem/plans/seat-machines-shed-leaked-processes-design.md:259-306`)
- Read: C10 is a rule, but the complete witness table assigns it `none`. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:170,355`)
- Read: the current `janitor` family contains only `headroom`, so there is no current verb that can serve as C10's automated witness before Unit 13. (`metasystem/cmd/metasystem/main.go:415-420`)

The unit graph omits required production files and a binding witness. Adding the missing work changes at least one Boundary and may add a unit. The arithmetic in section 15 is internally correct, and every stated allocation is at most 290 lines, but the requested 40 attempts and 1,400 minutes are not an estimate of the complete build described by the findings above. SMLP-219 and SMLP-220 are reopened.

## Rigor rows for material findings

| Finding id | Rigor class | Facts | Reopening trigger |
| --- | --- | --- | --- |
| SMLP-301 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | A fixture runner can still be prevented from checking owner, root, or lifetime by blocked tick work. |
| SMLP-302 | severe | `local=false; recoverable=false; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | Any runner replacement signal still lacks an immediate full-ref proof. |
| SMLP-303 | severe | `local=false; recoverable=false; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | A supervision signal still uses a whole-second record or skips a proof beside escalation. |
| SMLP-304 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | A post-start identity-probe failure can still signal without proof or wait without a bound. |
| SMLP-305 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | Deleting the suite-visible run registry can still turn launcher cleanup into a no-op. |
| SMLP-306 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | Killing an outer suite can still kill the nested launcher while leaving the inner suite and watchdog outside its cleanup reach. |
| SMLP-307 | severe | `local=false; recoverable=false; proofBoundaryCrossed=true; authorityBoundaryCrossed=true; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=true` | A direct reap can still signal after its designated owner proof changed. |
| SMLP-308 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false` | The per-run registry still lacks one explicit locking, corruption, and late-append contract. |
| SMLP-309 | severe | `local=false; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false` | A reaped bed that missed `bed-close` can still remain an immortal open registry record. |
| SMLP-310 | severe | `local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false` | The kept census still claims an owner and bound for a moved class without a source for either value. |
| SMLP-311 | severe | `local=true; recoverable=true; proofBoundaryCrossed=true; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false` | DONE still depends on an owner-death time or per-pass state that no retained record carries. |
| SMLP-312 | unproven | `local=true; recoverable=true; proofBoundaryCrossed=false; authorityBoundaryCrossed=false; secretsBoundaryCrossed=false; irreversibleDataBoundaryCrossed=false; externalSideEffectBoundaryCrossed=false` | A required file or witness still sits outside the unit graph and its estimate. |

## Non-material findings

### SMLP-399

Severity: low  
Material: no

Claim: the three unchanged `lock.Acquire` callers "stay whole-second." (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:55`)

Evidence:

- Read: the gate-run and brain callers do store whole seconds. (`metasystem/internal/gaterun/guard.go:90-91`; `metasystem/internal/brain/brain.go:338-345`)
- Read: the dispatch owner lock stores only pid and tag. Its custom probe authenticates the tag from argv and stores no start second. (`metasystem/internal/dispatch/ownerlock.go:45-83,86-92`)

The phrase is false for one caller, but that caller is explicitly unchanged and outside this goal's signal paths. Correcting the description would not change this implementation.

## Closure audit

| Prior finding marked folded | Revision 3 text checked | Result |
| --- | --- | --- |
| Round 1 SMLP-04 | X1 through X4 and the full-ref statement. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:33,67-70`) | Reopened by SMLP-302, SMLP-303, and SMLP-304. |
| Round 1 SMLP-05 | The post-start defer and durable control-root registry. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:97-98`) | Reopened by SMLP-304, SMLP-305, and SMLP-306. |
| Round 1 SMLP-12 | The machine registry and live-marker re-index. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:99,103-108`) | Closed for discovery across temp bases. Growth is separately reopened under SMLP-215 by SMLP-309. |
| Round 1 SMLP-15 | The exact lock and full machine transaction. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:69,103-108`) | Closed for the machine registry. The separate run registry is incomplete in SMLP-308. |
| Round 1 SMLP-16 | Fifteen ordered units, Boundaries, allocations, and witness table. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:199-355`) | Reopened by SMLP-303 and SMLP-312. |
| Round 1 SMLP-17 | The day proof and retained janitor log. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:357-368`) | Reopened by SMLP-311. |
| Round 1 SMLP-18 | Human approval before any unit and the explicit defaults table. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:370-385`) | Closed. The page leaves these choices with Wido. |
| Round 2 SMLP-211 | The run registry is under the control root, never child `--tmp`. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:97`) | Closed. SMLP-305 is a different child-deletion path because the registry path is exported to the child. |
| Round 2 SMLP-212 | The launcher signal helper and branch table. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:70`) | Reopened by SMLP-304. |
| Round 2 SMLP-213 | Full runner, lock, and janitor refs. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:33,67-70`) | Reopened by SMLP-302 and SMLP-303. |
| Round 2 SMLP-215 | One machine registry, locking, compaction, recovery, and concurrency. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:99,103-108`) | Reopened by SMLP-308 and SMLP-309. |
| Round 2 SMLP-218 | Narrowed metrics and log-based latency. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:357-368`) | Reopened by SMLP-311. |
| Round 2 SMLP-219 | Dependency order and complete Boundaries. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:199-306`) | Reopened by SMLP-303 and SMLP-312. |
| Round 2 SMLP-220 | Every stated allocation is at most 290 lines. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:201-306`) | Reopened by SMLP-312 because required work is absent from the allocations. |
| Round 2 SMLP-221 | Defaults await Wido with the budget. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:370-385`) | Closed. |
| Round 2 SMLP-222 | Base, rework, margin, and request are separate. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:372`) | Closed. The arithmetic is correct. |
| Round 2 SMLP-223 | Revision 3 says its anchors were opened. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:406`) | Closed except for the non-material description in SMLP-399. The current-worktree anchors used by the kept design resolve to the named code. |

The moved broker, waiter, retention, and placement findings remain outside the kept units. Sections 8 through 10 label themselves as seed notes and do not add those implementations here. The one split dependency that remains is SMLP-310: the kept census promises owner and bound values that only the future broker and shell designs can establish.

## Revision 3 file and line audit

Every current-worktree anchor newly used by the kept revision was opened. The material consequences are in the findings above.

| Design line | Cited current code | Result |
| --- | --- | --- |
| 31 | `metasystem/internal/up/up.go:659` | Holds. It writes the session announcement. |
| 33 | `metasystem/internal/stopfence/fence.go:33-47`; `metasystem/internal/identity/identity.go:72-93`; `metasystem/internal/registry/selection.go:13-26` | Holds for the record spelling, identity modes, and home override. SMLP-303 reports the supervision records left outside that claim. |
| 35, 43 | `metasystem/internal/census/run.go:209-260,218-244`; `metasystem/internal/census/production.go:45-76` | Holds. The existing census is checkout-scoped, and path is only its scope test. |
| 39, 44 | `metasystem/internal/identity/identity.go:8-13,187-217`; `metasystem/internal/janitor/killproof.go:41-63` | Holds for target liveness, Unknown refusal, and current positioned shapes. SMLP-307 reports the missing second owner check. |
| 53 | `metasystem/internal/census/production.go:66` | Holds. Production PPID is zero today. |
| 55 | `metasystem/internal/lock/lock.go:30-36,141`; `metasystem/internal/stopfence/fence.go:254-277`; `metasystem/internal/registry/append.go:20`; `metasystem/internal/gaterun/guard.go:91,274`; `metasystem/internal/dispatch/ownerlock.go:89`; `metasystem/internal/brain/brain.go:343` | The call sites hold. The description of dispatch as whole-second is false, as SMLP-399 records. |
| 57, 67-70, 76-82 | `metasystem/internal/steward/runner.go:41-49,141-149,167-218,419,578-589,662-664,764,775,917-925,942-1030`; `metasystem/internal/steward/health.go:782`; `metasystem/internal/stoptransition/families.go:671`; `metasystem/cmd/metasystem/steward_verbs.go:538-574` | The cited sites hold. SMLP-301 and SMLP-302 report the control-flow and signal omissions. |
| 58, 70, 97-98, 165 | `metasystem/internal/proofrun/launcher.go:200-212,224,228-276,299-337,519-541,631-666`; `metasystem/internal/proofrun/watchdog.go:206-211,284-297` | The cited sites hold. SMLP-304 through SMLP-307 report the uncovered failure and nesting cases. |
| 59 | `metasystem/internal/supervise/arming.go:1232` | Holds. `ShutdownAt` exists. SMLP-303 reports why reuse without changes is unsafe. |
| 81 | `metasystem/cmd/metasystem/proof_run.go:908-945`; `metasystem/internal/config/validate.go:501-511` | Holds. The first reads bounded keys. The second currently checks only positivity. |
| 97 | `metasystem/scripts/validate-metasystem.sh:261-268,384` | Holds. The suite owns and removes its stage work. |
| 103, 106-107 | `metasystem/internal/registry/framing.go:111-115,159-170` | Holds. Whole-file reads and middle-corruption behavior are as stated. SMLP-308 and SMLP-309 report the incomplete contracts built on them. |
| 113-114 | `metasystem/cmd/metasystem/channel_verbs.go:363-376`; `metasystem/scripts/agents/channel-fixtures.sh:16-25`; `metasystem/scripts/agents/goal-cli-fixtures.sh:1274-1284`; `metasystem/scripts/agents/adapters/fake.sh:206,263,283,318,324`; `metasystem/scripts/agents/hosts/fake.sh:42`; `metasystem/scripts/agents/supervision-hook-fixtures.sh:1766-1774` | Holds for the current fake loops and launch sites. |
| 121, 163, 167-169 | `metasystem/cmd/metasystem/main.go:415-420`; `metasystem/cmd/metasystem/process_verbs.go:110`; `metasystem/internal/up/up.go:699`; `metasystem/internal/steward/health.go:66-86`; `metasystem/internal/identity/enumerate_darwin.go:112`; `metasystem/internal/identity/enumerate_linux.go:47-60` | Holds for the current janitor family, human gate, insertion point, health order, and both parent readers. |

The plugin, waiter CPU, and retention anchors in sections 8 through 10 were carried from round 2. They were already opened there. Revision 3 labels those sections as hand-off data, not kept implementation.

## Process-path audit

| Kept path | Result |
| --- | --- |
| Fixture runner self-exit | Not closed. Guard checks can be starved by synchronous tick work. SMLP-301. |
| `steward.Disarm` | Closed by X1 and X2 as a design contract. The current reread sites are `metasystem/internal/steward/runner.go:966-994`. |
| Runner replacement | Not closed. SMLP-302. |
| Launcher early returns | Not closed for failed exact capture. SMLP-304. Other raw launcher sites have a captured ref and fit X4. |
| Normal nested launcher return | Closed only while the nested launcher survives to run its defer. Outer-group termination is not closed. SMLP-306. |
| Missing or deleted run registry | Not closed. SMLP-305. |
| Direct janitor TERM and KILL | Target identity is rechecked, but owner authority is not. SMLP-307. |
| Supervision shutdown | Not closed. Its records and orderly component escalation remain inexact. SMLP-303. |
| Fixture fake self-exit | Closed as a design contract by F2 through F4. It sends no signal. (`metasystem/plans/seat-machines-shed-leaked-processes-design.md:112-115`) |

This is the third and last review round. Severe and unproven findings remain, so the exhausted review rule leaves the design waiting on the human. No fourth critique round is available.

Verdict: rework.
