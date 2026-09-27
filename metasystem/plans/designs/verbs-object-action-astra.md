# Astra's critique of verbs-object-action

Round 1: Codex gpt-6-astra, read-only, against revision 1 (2026-09-26). Verbatim; dispositions are in the design's section 9.

---

**VOA-01 — High — The fixture ports cannot replace the proof providers that must approve their landing**

Evidence: The design deletes the gate and suite scripts in [verbs-object-action.md:317](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:317) and [verbs-object-action.md:332](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:332). However, [testpolicy/protection.go:55](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/testpolicy/protection.go:55) preserves base-controlled groups, and [protection.go:187](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/testpolicy/protection.go:187) merges replacements starting from the base definition, retaining its adapter and command. The protected groups include `fast-static-build` invoking Bash at [testing.json:56](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/testing.json:56). Section discovery requires the executable selector at [test_go.go:698](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/proofrun/test_go.go:698).

Scenario: U7a deletes `go-gate.sh` and changes its group to a Go implementation. Delivery evaluation still selects the base group’s Bash command against the candidate tree, where that script is absent. U7b similarly deletes the selector that protected section groups require. A complete scenario map does not authorize either provider replacement.

Change: Specify the proof-policy transition before these deletions, including how the trusted destination accepts replacement providers without letting the candidate weaken its own judge. Extend scenario maps to identify the replacement group, selector, assertions and actual discovery/execution evidence. Exercise the first deletion against the preceding landed policy.

Test 1: yes — changes policy-transition implementation, fixture integration and unit order.  
Test 2: WORK — the affected units cannot obtain the delivery proof required to land. SAFE — bypassing the protected provider would discard independent proof.

**VOA-02 — High — R7 preserves too little of the subprocess authority context**

Evidence: U1 binds existing handlers directly and removes subprocesses at [verbs-object-action.md:167](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:167) and [verbs-object-action.md:283](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:283); R7’s specified witness compares the recorded actor. Classification still starts from `os.Getppid()` at [goalsync_mutations.go:738](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/cmd/metasystem/goalsync_mutations.go:738), human proof independently uses it at [goalsync_mutations.go:631](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/cmd/metasystem/goalsync_mutations.go:631), and classification determines epoch authority separately from the actor at [goalsync_mutations.go:706](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/cmd/metasystem/goalsync_mutations.go:706). The landing owner announces its own PID at [landing_batch_owner.go:179](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/cmd/metasystem/landing_batch_owner.go:179) and gives children explicit lineage at [landing_batch_owner.go:365](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/cmd/metasystem/landing_batch_owner.go:365).

Scenario: A landing child currently classifies its parent—the announced landing owner. Calling the same handler inside that owner instead classifies the owner’s parent. The actor can still contain the expected machine and lineage while holder classification and claim epoch are lost. Conversely, replacing proof with a previously observed actor would omit the human-ancestry check.

Change: Define an invocation-scoped owner API carrying the effective caller, authenticated classification and epoch, human-proof context, selected roots and lineage. Remove hidden parent-PID/environment reads from these paths. Test authorization and refusal parity, holder epoch, human-proof rejection, and hand-back/release—not merely actor equality. Avoid process-global environment changes to emulate child environments.

Test 1: yes — changes owner APIs, authentication flow and tests.  
Test 2: WORK and SAFE — direct handler reuse can strand landing claims or change the authority accepted for mutations.

**VOA-03 — High — U1 removes routing still used by Go launchers**

Evidence: U1 explicitly prefixes shell calls while removing family fallthrough at [verbs-object-action.md:275](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:275). Non-public Go launchers still construct unprefixed argv: `proc setsid … launch supervise` at [launch/process.go:143](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/launch/process.go:143), `run wrap` at [run.go:152](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/cmd/metasystem/run.go:152), and destination `goal fetch` at [seat/launch/sequence.go:625](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/seat/launch/sequence.go:625).

Scenario: After U1, a public build reaches its launch owner successfully, but the supervisor child immediately receives an unknown-command refusal. A governed run can likewise create its pending record before its wrapper fails to route. Porting the public handler does not change these nested launches.

Change: Make U1 migrate every argv producer and consumer needed by the removed fallthrough, including nested executable arguments, generated settings and Go launchers. Assign the entrypoint/seat-protocol changes to that same unit. Distinguish calls to the current executable from calls to destination or retained binaries; do not blindly prefix foreign-engine requests.

Test 1: yes — expands U1’s required code changes and launch tests.  
Test 2: WORK and SAFE — ordinary launches fail after admission and can leave pending work without its controller.

**VOA-04 — High — The entrypoint list omits controllers that must outlive their caller**

Evidence: Section 3.2’s closed list and shrink-only rule are at [verbs-object-action.md:184](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:184) and [verbs-object-action.md:202](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:202). The browser detaches `seat launch` specifically to outlive both request and server at [ui_launch.go:165](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/cmd/metasystem/ui_launch.go:165). Dispatch detaches an adapter supervisor at [dispatch.sh:1001](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/scripts/agents/dispatch.sh:1001); that supervisor continues through runtime completion and result publication at [adapters/claude.sh:165](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/scripts/agents/adapters/claude.sh:165). U6a calls its replacement entry merely “interim.”

Scenario: Removing `seat launch` breaks browser machine creation. Removing the adapter entry after U6b without another persistent owner either keeps the public dispatch call alive for the entire job or terminates supervision when that call exits. `seat-step` supplies destination operations, not the detached controller.

Change: Add permanent controller entries, or explicitly assign these lifecycles to an existing persistent controller with equivalent custody, cancellation and recovery. Specify who continues after the requesting process exits. Correct section 7’s claim that the browser changes only displayed strings.

Test 1: yes — changes the entrypoint inventory and lifecycle ownership.  
Test 2: WORK and SAFE — machine launches and delegate completion cannot retain their existing lifetime guarantees.

**VOA-05 — High — `proof-run preserve` is not safely replaceable by an ordinary function call**

Evidence: The design explicitly folds it in-process at [verbs-object-action.md:202](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:202). Its owner documents that a separate process supplies the hard time bound at [proofrun/evidence.go:23](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/proofrun/evidence.go:23). [watchdog.go:228](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/proofrun/watchdog.go:228) starts that child and kills it on timeout. Evidence preservation precedes suite shutdown and signalling at [watchdog.go:191](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/proofrun/watchdog.go:191).

Scenario: Evidence copying blocks in filesystem I/O. With a synchronous in-process replacement, the watchdog never reaches suite cleanup. A context or abandoned goroutine does not reproduce the existing ability to terminate the copier.

Change: Retain a bounded copier process, potentially as a mode of an existing genuine process entrypoint. Require a test where copying does not return and the watchdog still records partial evidence and proceeds to cleanup.

Test 1: yes — changes process architecture and timeout tests.  
Test 2: SAFE — the proposed deletion removes the watchdog’s hard bound and can strand the stalled workload.

**VOA-06 — High — The census misses command spellings that establish authority and permission to signal**

Evidence: U3 proposes preserving the watcher’s writer string at [verbs-object-action.md:297](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:297). But [janitor/killproof.go:40](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/janitor/killproof.go:40) recognizes process argv containing `supervise`, `mission run-loop`, and adapter script names; matching requires exact words or path basenames at [killproof.go:77](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/janitor/killproof.go:77). Steward authority requires argv word 1 to be `steward` at [lease/classify.go:504](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/lease/classify.go:504). Runtime signatures are discovered by enumerating adapter `.sh` files at [classify.go:257](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/lease/classify.go:257).

Scenario: `internal steward` no longer satisfies steward classification. `internal supervisor` and `internal mission-loop` no longer match the janitor’s ownership shapes. Deleting adapter scripts also removes the signature discovery source. None of these strings is an invocation that section 6.1’s caller search necessarily finds; retaining a JSON writer label repairs none of them.

Change: Include argv recognizers, signature discovery and custody predicates in each command/script cutover. Define their new typed owners and test classification plus authorized/refused signalling against the actual new process argv.

Test 1: yes — changes authority discovery and process-ownership predicates.  
Test 2: WORK and SAFE — lawful recovery can be refused, runtime ancestry can be misclassified, and surviving processes can become unkillable through the governed path.

**VOA-07 — High — The prescribed dispatch package placement creates import cycles**

Evidence: U6b places the lifecycle driver in `internal/dispatch` at [verbs-object-action.md:324](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:324), while section 6.2 requires reuse of owner functions. Dispatch currently needs lease ownership operations at [dispatch.sh:371](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/scripts/agents/dispatch.sh:371) and steward authorization at [dispatch.sh:1474](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/scripts/agents/dispatch.sh:1474). Both [lease/sweep.go:14](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/lease/sweep.go:14) and [steward/tick.go:12](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/steward/tick.go:12) already import `internal/dispatch`.

Scenario: A literal port imports lease and steward into dispatch, producing `dispatch → lease → dispatch` and `dispatch → steward → dispatch`. Copying those decisions into dispatch to make it compile would duplicate authority owners.

Change: Specify the orchestration composition point and dependency direction before U6b. Either place orchestration above these packages or define narrow injected operations wired from above. Assign that boundary work before the parallel ports that depend on it.

Test 1: yes — changes package ownership, interfaces and implementation order.  
Test 2: WORK — the prescribed direct composition cannot compile.

**VOA-08 — High — The surviving build script has no equivalent bootstrap fence or stamp path**

Evidence: Section 3.3 replaces `gate fence` with a plain file check at [verbs-object-action.md:228](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:228). The current build calls that fence at [go-build.sh:47](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/scripts/agents/go-build.sh:47). Its owner excludes the requesting process’s ancestry at [gaterun/fence.go:16](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/gaterun/fence.go:16), and distinguishes live, dead and unreadable markers at [gaterun/gaterun.go:100](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/gaterun/gaterun.go:100). The stamp also invokes source code through the soon-deleted `behavior-surface select` CLI at [go-build.sh:65](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/scripts/agents/go-build.sh:65).

Scenario: Checking marker existence blocks the gate’s own rebuild and stale markers indefinitely; ignoring them permits replacement under another live gate. Keeping the current stamp computation leaves a forbidden CLI dependency in the supposedly binary-independent bootstrap.

Change: Define a bootstrap path that preserves live-owner exclusion, self-exemption and dirty-engine stamping without the installed engine. Name its executable/helper boundary and migrate it before either caller disappears. Prove clean, dirty, absent-engine, stale-marker, own-gate and foreign-gate cases.

Test 1: yes — changes bootstrap implementation and prerequisite order.  
Test 2: WORK and SAFE — rebuilding can deadlock or replace an engine during another owner’s proof.

**VOA-09 — High — Replacing the static group’s command with `test run` introduces recursive scheduling**

Evidence: U7a tells `fast-static-build` itself to invoke a public `test run` profile at [verbs-object-action.md:317](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:317). That group currently executes the leaf fast gate at [testing.json:56](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/testing.json:56) and belongs to every standard selection at [testing.json:139](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/testing.json:139). Selection incorporates those always-required groups at [testpolicy/select.go:199](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/testpolicy/select.go:199). The existing command has modes and group selection, not the proposed leaf profile, at [test.go:322](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/cmd/metasystem/test.go:322).

Scenario: A standard run schedules `fast-static-build`, which launches another standard run, which schedules `fast-static-build` again. Selecting that group explicitly still selects the group whose command starts another run.

Change: Make the group execute the Go static/build operation directly through a worker adapter or bounded worker mode. Public `test run` should schedule that leaf. If a special profile is intended, define its non-recursive execution semantics and resource ownership explicitly.

Test 1: yes — changes the group execution boundary and tests.  
Test 2: WORK — the proposed replacement has no terminating scheduling path as specified.

**VOA-10 — High — Proof once per wave cannot authorize the earlier per-unit landings**

Evidence: Every unit lands independently at [verbs-object-action.md:266](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:266), but shared testing runs only before the wave’s last landing at [verbs-object-action.md:388](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:388). The existing commit boundary requires successful delivery verification for the candidate tree at [commit.sh:402](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/scripts/agents/commit.sh:402). Batch admission also launches retained testing for each admitted candidate at [landing_batch_admission.go:112](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/cmd/metasystem/landing_batch_admission.go:112).

Scenario: U2 finishes first in wave 1. Its package tests pass, but its changed tree has no matching delivery result. The wrapper refuses its landing. Waiting for the final unit’s suite does not produce proof for that earlier tree, and removing the verification would violate the stated preservation of proof semantics.

Change: Require a delivery selection and verification for every landing tree, reusing eligible unchanged group results through the existing machinery. Reserve “once per wave” for additional aggregate validation, not the proof consumed by each landing. Name the path used while U5 and U7 replace that machinery.

Test 1: yes — changes execution order and the per-unit proof rule.  
Test 2: WORK and SAFE — earlier units cannot land lawfully under the proposed schedule.

**VOA-11 — High — Stable hook paths do not make the live cutover atomic**

Evidence: The design promises unchanged settings will work through the retained stub at [verbs-object-action.md:220](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:220) and [verbs-object-action.md:399](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:399). Delegate settings instead embed a direct engine command at [adapter/claude.go:122](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/adapter/claude.go:122), whose session signal drives handshake completion at [adapters/claude.sh:165](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/scripts/agents/adapters/claude.sh:165). The build replaces the executable separately at [go-build.sh:108](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/scripts/agents/go-build.sh:108). Landed recovery deliberately invokes the rebuilt binary, rather than its current process’s implementation, at [rearm_on_landed.go:410](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/cmd/metasystem/rearm_on_landed.go:410).

Scenario: A peer receives the new hook stub while its existing executable still lacks `internal hook`. Conversely, replacing the executable leaves an already-generated delegate settings file invoking the deleted `adapter claude-session-signal`. A running old recovery process can also invoke its old `up` argv against the newly replaced binary. Seat-step negotiation covers none of these boundaries.

Change: Specify a quiesce, rebuild, enroll and restart sequence for each affected checkout, including active delegates and generated settings. Make the new hooks usable only with the matching engine generation, and preserve recoverability of interrupted cutovers. Include retained proof-engine protocols in this generation audit; the design’s claim that seat-step is the only cross-version boundary is false.

Test 1: yes — changes cutover ordering, generated-state handling and integration tests.  
Test 2: WORK and SAFE — unchanged paths alone can leave sessions without functioning hooks or delegates without a handshake.

**VOA-12 — Medium — The work-ID collapse has no complete disambiguation contract**

Evidence: G3 promises all former ID kinds through one generalized resolver at [verbs-object-action.md:97](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/designs/verbs-object-action.md:97). The current resolver distinguishes only launch and dispatch records, with `j1:`/`j2:` escape references for collisions, at [intent_process.go:576](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/cmd/metasystem/intent_process.go:576). Unit runs occupy a separate store at [launch/unit_run.go:803](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/launch/unit_run.go:803), while launch IDs can be supplied explicitly at [launch/launch.go:110](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/launch/launch.go:110). Proof waits and durable wait resumptions invoke different owners at [intent_work.go:1227](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/cmd/metasystem/intent_work.go:1227) and [intent_work.go:1265](/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/cmd/metasystem/intent_work.go:1265).

Scenario: A launch and unit run legitimately share an exact raw ID. Previously `wait job j1:ID` and `wait run ID` distinguish them. After removing subject words, lookup order would select the wrong owner; unconditional ambiguity refusal would leave the unit unreachable unless a new qualified reference exists. Goal-targeted work commands add another namespace that G3 does not resolve.

Change: Define canonical opaque references for every supported owner, their storage scope, raw-ID ambiguity behavior and action applicability. Preserve an exact address for every existing record without rewriting its identity. Test collisions across stores, goal/work collisions, and resuming a durable wait versus starting another wait on its target.

Test 1: yes — changes the public reference contract, resolver and tests.  
Test 2: WORK — previously addressable work can become ambiguous or unreachable; wrong-owner selection can also violate SAFE.

Material findings: 12

Codex session ID: 01a0df9c-6db2-7b81-9a4d-6571b5333988
Resume in Codex: codex resume 01a0df9c-6db2-7b81-9a4d-6571b5333988

---

Round 2: Codex gpt-6-astra, read-only, against revision 2 (6614a1dd2).

Read-only review of `6614a1dd2`. All twelve folds were checked. No builds, tests, or writes were performed; the failure scenarios below are inferred from the cited code.

**VOA-02-R2 — fold does not resolve**

Evidence: [Design:436](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:436) constructs the public command’s context from “its own caller.” Currently, [intent_operations.go:341](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/intent_operations.go:341) launches a coordinator-mutation child, whose human gate classifies its parent—the public command process—at [brain.go:28](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/brain.go:28). The classifier starts runtime-signature checks at the supplied process’s **parent**, at [classify.go:426](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/lease/classify.go:426). This exact-node omission is documented at [directinvoker.go:11](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/lease/directinvoker.go:11). An otherwise unrecognized caller with a controlling terminal becomes HUMAN at [classify.go:463](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/lease/classify.go:463).

Scenario: An unannounced, terminal-bearing agent runtime directly invokes `settings coordinator --declare --by NAME`. Today, the owner child’s ancestry walk encounters that runtime and rejects the human-only action. Starting the replacement context from the public command’s caller skips the runtime’s own signature and can classify it HUMAN. The gate at `brain.go:32` then permits the declaration.

Change: Specify the source identity per replaced call edge. Replacing a child requires the current process’s identity—the parent that child previously observed. Already-direct owners retain their existing caller identity. Add a direct-runtime invocation fixture for the coordinator’s human gate.

Test 1: yes — changes context construction and authorization tests.

Test 2: SAFE — the prescribed starting identity can turn an agent invocation into an accepted human act.

**VOA-03-R2 — fold does not resolve**

Evidence: [U1:340](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:340) retains only **family** fallthrough; [6.5:477](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:477) migrates callers of colliding family pairs. However, `wait` and `delegate` are separate top-level dispatcher branches at [main.go:848](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/main.go:848), not family pairs or retained §3.2 entries. Surviving callers include dispatch’s `wait --root … --job …` at [dispatch.sh:1131](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/scripts/agents/dispatch.sh:1131), cancellation’s `delegate --cancel` at [stoptransition/families.go:241](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/stoptransition/families.go:241), and critic continuation’s `delegate --follow-up` at [goal_branch.go:216](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/goal_branch.go:216). Dispatch’s port is deferred to [U6b:390](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:390).

Scenario: U1 removes the non-entry top-level routes while preserving only family fallthrough. A subsequent checkout stop reaches job cancellation, launches `delegate --cancel`, and receives an unknown-command refusal. The job remains nonterminal; cancellation reports incomplete at [families.go:249](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/stoptransition/families.go:249).

Change: Include top-level machinery branches in U1’s caller migration. Convert their callers to owner calls or explicitly prefixed temporary machinery routes until their ports land. Preserve the public hard cutover; do not retain old public spellings.

Test 1: yes — changes U1’s routing scope, caller migrations, and cancellation/wait tests.

Test 2: WORK and SAFE — intermediate slices break waiting, continuation, and governed cancellation of running jobs.

**VOA-08-R2 — fold does not resolve**

Evidence: [Design:288](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:288) prescribes the one-line stub `exec go run ./cmd/devgate build "$@"`. Today’s build script resolves its installation directory and changes into it at [go-build.sh:10](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/scripts/agents/go-build.sh:10). Adoption resolves its source root in a subshell at [adopt.sh:94](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/scripts/adopt.sh:94), then invokes the build script by absolute path at [adopt.sh:216](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/scripts/adopt.sh:216). The stub lands in [U7a:362](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:362); adoption remains a script until [U8:401](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:401).

Scenario: Between U7a and U8, invoke the adoption script from this repository’s top level. Its mandatory build reaches the stub, which resolves `./cmd/devgate` against the repository top instead of the `metasystem` module. Adoption fails before the new build owner runs.

Change: Preserve location-independent invocation: the stub must resolve its own installation and change directory, or pass that directory through `go -C`. Add an adoption/bootstrap witness invoked from outside the module directory.

Test 1: yes — changes the specified stub and its witnesses.

Test 2: WORK — a surviving production caller cannot complete adoption during the planned intermediate state.

**VOA-11-R2 — fold does not resolve**

Evidence: [Section 7:525](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:525) requires the agent seat to execute `system stop`, bootstrap build, and `system start` after each landing. The existing public checkout stop calls the process owner at [intent_process.go:508](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/intent_process.go:508). That owner requires a human terminal at [process_verbs.go:172](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/process_verbs.go:172) and explicitly rejects non-HUMAN callers at [process_verbs.go:479](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/process_verbs.go:479). Checkout start likewise invokes the arm owner at [intent_process.go:471](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/intent_process.go:471). Additionally, the prescribed bootstrap first arrives in [U7a:362](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:362), after U0 and U1.

Scenario: Even with the new grammar installed, the m1e agent seat cannot perform the required quiesce: its stop is refused by the preserved authority checks. Earlier landings also cannot use the not-yet-delivered `cmd/devgate`. Thus the stated recovery sequence is unavailable when first required.

Change: Define the cutover executor and available commands for each generation. Use an existing authorized rearm path, or assign the human-terminal operations to a human. Establish bootstrap availability before any dependent cutover, and specify quiescence before replacing the affected live machinery.

Test 1: yes — changes cutover ownership, prerequisites, and execution order.

Test 2: WORK — the mandatory cutover cannot execute under the authority and delivery order the design preserves.

**VOA-13 — High — `test plan` is simultaneously public and exclusively hidden**

Evidence: [Public table:166](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:166) exposes `test plan`; [entrypoint table:243](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:243) retains that exact pair as an entry. Yet [design:225](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:225) forbids sharing a pair, and [6.5:483](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:483) says entries do not become public. This is an active machine protocol: [test.go:909](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/test.go:909) invokes retained-engine `test plan`, and [test.go:932](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/test.go:932) strictly decodes its raw plan. Candidate probes also invoke it at [test_protection.go:233](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/test_protection.go:233). The ordinary public JSON envelope is different, as defined at [intent.go:596](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/intent.go:596).

Scenario: Applying entry precedence omits the promised public planning action from generated help and the catalogue. Applying ordinary public routing and rendering changes the response consumed by retained engines and proof probes. No single registration satisfies the specified visibility, disjointness, and protocol rules.

Change: Explicitly resolve this pair. One workable contract is a public `test plan` registration preserving its existing machine argv and raw JSON, with machinery-only options hidden. Amend the disjoint-pair rule accordingly and test calls from the preceding engine generation.

Test 1: yes — changes registration rules, visibility, response handling, and protocol tests.

Test 2: WORK — the current instructions cannot simultaneously deliver the public action and preserve the active proof protocol.

**VOA-14 — Medium — In-process proofs inherit another invocation’s restart state**

Evidence: [U1:346](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:346) replaces landing batch children with owner calls. Today each batch proof gets a separate process and environment at [landing_batch_prove.go:322](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/landing_batch_prove.go:322). Testing preparation reads `METASYSTEM_PREPARATION_RESTARTED` at [test.go:485](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/test.go:485), permanently sets it after one base movement at [test.go:498](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/test.go:498), and refuses another movement when it is set at [test.go:492](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/test.go:492). The landing owner repeatedly processes work in the same process at [landing_batch_owner.go:664](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/landing_batch_owner.go:664).

Scenario: One in-process proof encounters a legitimate base movement and successfully retries. Its environment flag remains in the resident owner. A later proof’s first base movement is incorrectly refused as the second movement “during one test invocation.” The removed child boundary previously discarded that state.

Change: Extend §6.2 beyond authority context to preserve invocation-local execution state. Make the restart allowance request-local. Test two consecutive preparations in one owner, each with one base movement, while retaining refusal for two movements within one invocation.

Test 1: yes — changes preparation state ownership and sequential-invocation tests.

Test 2: WORK — a later valid proof, and therefore its landing, is incorrectly refused because an earlier invocation consumed its own allowance.

Folds verified: 8 of 12
Material findings: 6

Codex session ID: 01a0dfab-41d4-7c62-91b9-956bdfa4f955
Resume in Codex: codex resume 01a0dfab-41d4-7c62-91b9-956bdfa4f955

---

Round 3: Codex gpt-6-astra, read-only, against revision 3 (c1e6f31f7).

Read-only review of `c1e6f31f7`. No files or Git state were changed; no builds or tests were run. Verification below means the proposed fold matches the traced code path. Failure scenarios are source-derived.

| Round-2 fold | Verification |
|---|---|
| VOA-02-R2 | Verified. The [per-edge identity rule](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:455) preserves the current child’s [parent-based human gate](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/brain.go:28), including the classifier’s [parent-first signature walk](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/lease/classify.go:426). |
| VOA-03-R2 | Verified. [U1 now migrates the separate top-level machinery routes](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:359), covering the actual [dispatch wait](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/scripts/agents/dispatch.sh:1131), [cancellation](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/stoptransition/families.go:241), and [critic continuation](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/goal_branch.go:216) calls. The existing [explicit internal dispatcher](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/main.go:793) supports the proposed temporary routing. |
| VOA-08-R2 | Verified. The [self-resolving stub](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:296) preserves the existing [installation-directory behavior](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/scripts/agents/go-build.sh:10) required by adoption’s [absolute-path invocation](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/scripts/adopt.sh:216). |
| VOA-11-R2 | Not verified; see VOA-11-R3 below. |
| VOA-13 | Verified. The [explicit public/protocol exception](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:230) preserves the existing [raw JSON output](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/test.go:223), [retained-engine argv and strict decoder](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/test.go:908), and [candidate-engine probe](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/test_protection.go:233). |
| VOA-14 | Verified for the original state leak. [Request-local execution state](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:475) removes the lifetime mismatch between the [process-global restart flag](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/test.go:484), today’s [separate proof child](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/landing_batch_prove.go:322), and the [resident owner loop](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/landing_batch_owner.go:664). |

**VOA-11-R3 — High — fold does not resolve: automatic cutover has no steward trigger**

Evidence: [Section 7](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:569) assigns rebuilding to an already-existing steward path. The actual `landedRearm` entry is [test preparation](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/test.go:560); its implementation explicitly describes that caller at [rearm_on_landed.go:548](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/rearm_on_landed.go:548). The steward’s [resident loop](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/steward/runner.go:205) ticks, revives and delivers notifications; its [revival callback](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/steward_verbs.go:639) invokes `steward revive`. The `up` recovery path [re-enrolls an already-rebuilt executable](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/up/up.go:566), rather than building it.

Scenario: An armed peer pulls U1 without starting another test invocation or landing a batch. Its source and instructions contain the new grammar, but its installed engine remains old. The steward does not perform the promised rebuild. The newly instructed `work build` therefore cannot run. Moving the bootstrap earlier fixes availability, but does not supply the missing trigger.

Change: Assign an explicit cutover invocation and executor before new commands become usable on each affected checkout. Wire that invocation to the authorized rebuild/rearm sequence, and test a peer that pulls U1 without subsequently running tests.

Test 1: Adds required cutover control flow and an integration fixture.

Test 2: WORK — the prescribed automatic transition does not execute, leaving peers unable to use the delivered grammar.

**VOA-15 — High — In-process proof cancellation can terminate the shared landing owner**

Evidence: [U1 replaces landing batch children with owner calls](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:370). Today the proof has a [separate process](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/landing_batch_prove.go:322). Admission records the [current process as launcher](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/proof_run.go:1130), and suite publication likewise records [its own PID](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/proofrun/launcher.go:125). Goal-stop cancellation [stops that launcher directly or through its process records](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/dispatch/stop.go:707); the latter path explicitly includes [signalling the launcher](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/proofrun/stop.go:64).

Scenario: The resident landing owner runs a proof in-process. A goal stop cancels that proof. Its recorded launcher is now the landing owner itself, so the existing cancellation ladder sends TERM—and potentially KILL—to the process serving other batches. Invocation-local environment and caller classification do not isolate this cancellation.

Change: Retain a dedicated, declared per-proof launcher process for resident callers, with its own cancellation identity. Add a fixture cancelling one proof while the landing owner remains alive and continues other work.

Test 1: Changes the process-entry inventory, U1’s subprocess-removal scope, and cancellation tests.

Test 2: SAFE — stopping one proof terminates a shared running owner outside that proof’s intended scope.

**VOA-16 — Medium — Retained executable plans can replay deleted commands after cutover**

Evidence: The design promises [resuming an existing unit run through `work build ID`](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:154), but its cutover checks only [running delegates](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:578) and says [records retain old spellings](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:587). Build stores the supplied proof command [verbatim in the unit plan](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/intent_work.go:624). Resume [reloads that retained plan](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/launch/unit_run.go:155), then [launches its stored argv](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/launch/unit_run.go:369). Editing the plan instead triggers the [reserved-input digest refusal](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/launch/unit_named.go:295).

Scenario: A suspended unit has a proof command invoking an installation’s engine with the old public `test --goal G` spelling. No delegate is currently running, so cutover proceeds. Resuming the unit replays that command against the replaced engine and fails. Changing its retained argv manually also prevents continuation because the reservation digest no longer matches.

Change: Add an explicit cutover treatment for retained executable plans: inventory affected resumable runs and migrate their active command bindings through an owner-controlled transition preserving original inputs, reservation integrity, and run/round lineage. Test continuation across U1 with an old public proof command.

Test 1: Adds migration behavior for durable execution inputs, beyond updating source callers and historical text.

Test 2: WORK and SAFE — a promised continuation becomes unusable; bypassing the digest to repair it would lose the existing input-integrity check.

**VOA-17 — Medium — Filesystem waits have no replacement in the public grammar**

Evidence: The existing public table advertises [`wait file PATH --until present|absent`](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/intent_work.go:193). Its handler [resolves the caller-relative path and invokes the wait owner](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/intent_work.go:1219). The proposed [replacement row](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:158) omits file waits, and the [reference contract](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:103) supplies no filesystem target. U1 nevertheless [deletes every old public spelling](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:353).

Scenario: An agent needs to wait for an external process to create a nonempty result file. After U1, the old public command is removed, while no new object/action form can start that path wait. A `wait:` reference can resume an existing wait but cannot create this missing one.

Change: Add `work wait --path PATH --until present|absent`, retaining caller-relative resolution, existing observation semantics, and durable resumption. Include it in U1’s public table and routing witnesses.

Test 1: Adds a required public form and its owner binding.

Test 2: WORK — the grammar cutover removes an existing executable waiting operation without a replacement.

Round-2 folds verified: 5 of 6
Material findings: 4

Codex session ID: 01a0dfb9-f67d-7f73-80dd-1a6f229705ba
Resume in Codex: codex resume 01a0dfb9-f67d-7f73-80dd-1a6f229705ba

---

Round 4: Codex gpt-6-astra, read-only, against revision 4 (b1ef33db3).

Revision 4 has **two material failed folds**. No additional finding met both materiality tests. This was a read-only source review at `b1ef33db3`; no files or Git state were changed, and no builds or tests were run. Scenarios below are inferred from the cited code.

| Round-3 fold | Verification |
|---|---|
| VOA-11-R3 | Not verified; see VOA-11-R4. |
| VOA-15 | Verified. Retaining dedicated proof children in [§6.2](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:475) preserves the separate launcher used by [landing_batch_prove.go:322](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/landing_batch_prove.go:322). Admission records that child’s identity at [proof_run.go:1130](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/proof_run.go:1130), so cancellation through [dispatch/stop.go:707](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/dispatch/stop.go:707) targets the proof launcher without terminating the shared owner. |
| VOA-16 | Not verified; see VOA-16-R4. |
| VOA-17 | Verified. The added [`work wait --path` form](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:158) preserves the existing [caller-relative resolution and owner invocation](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/intent_work.go:1219). Its `wait:` continuation maps to the separate [durable-resume owner](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/intent_work.go:1265). |

**VOA-11-R4 — fold does not resolve**

**Evidence:** [Section 7:584](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:584) adds the rebuild trigger to `runSessionStart` in U1. The preceding engine’s [existing handler](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/wait_verb.go:471) only validates the holder session and prints waiting lines. The hook selects the installed executable at [supervision-hook.sh:936](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/scripts/agents/supervision-hook.sh:936) and invokes that executable’s `session start` at [line 1101](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/scripts/agents/supervision-hook.sh:1101). Its earlier `up` operation can re-enroll an **already rebuilt** engine, as [up.go:566](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/up/up.go:566) shows. The rebuilding hook stub arrives only in [U4](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:383).

**Scenario:** A peer has the pre-U1 executable, pulls U1, runs no test, and starts a session—the fold’s own witness. Its installed executable does not contain the newly added trigger. Even if the hook reaches `session start`, it runs the old handler and performs no rebuild. The updated instructions then request `work build`, which that executable cannot route.

**Change:** Put the first-cutover invocation somewhere executable before the replacement engine exists: for example, a hook-side bootstrap invocation delivered with U1, using the authorized decision/rebuild/rearm path. Test with an actual preceding-generation executable installed and only the source checkout advanced to U1.

**Test 1:** Yes—changes the bootstrap entry path, delivery ordering and integration fixture.

**Test 2:** **WORK**—the prescribed first automatic cutover cannot execute; the peer remains unable to use the delivered grammar.

**VOA-16-R4 — fold does not resolve**

**Evidence:** [Section 7:601](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:601) requires affected runs to be finished or abandoned through existing owners and makes abandonment the remedy for post-cutover refusal. No unit-run abandonment owner exists: searching `func.*Abandon|Abandon` throughout `internal/launch/*.go` returned no matches. The available [cancellation owner](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/launch/launch.go:532) updates an individual launch record. The [close owner](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/intent_delivery.go:1165) closes a dispatch chain. Unit completion instead writes `awaiting-judgement` at [unit_run.go:563](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/launch/unit_run.go:563), while changing retained inputs remains forbidden by [unit_named.go:295](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/launch/unit_named.go:295).

**Scenario:** An incompatible suspended named run from another checkout reaches the new resume guard after U1. The guard correctly refuses its stored command, but the advertised abandonment remedy cannot retire that run. Cancelling a child does not retire the unit or its reservation; closing a dispatch chain addresses another store. Editing the retained command still fails the digest check. The fold detects the incompatibility without providing its promised recovery.

**Change:** Specify an executable recovery path. If abandonment is selected, define its unit-run owner, reachable action, persisted retirement state and reservation treatment, preserving run/round history and resolving any live children. Otherwise specify an owner-controlled command-binding migration. Exercise recovery after the incompatibility refusal, not merely the refusal itself.

**Test 1:** Yes—adds lifecycle behavior and a public owner binding absent from the proposed implementation.

**Test 2:** **WORK**—an affected run cannot complete the prescribed recovery through the named existing owners.

Proposed receipt, not written: `Read-only revision-4 design critique; source inspection; two failed folds; no additional material findings.`

Round-3 folds verified: 2 of 4  
Material findings: 2

Codex session ID: 01a0dfc2-5ca5-7281-a66d-885ffb7a64e2
Resume in Codex: codex resume 01a0dfc2-5ca5-7281-a66d-885ffb7a64e2

---

Round 5: Codex gpt-6-astra, read-only confirmation of revision 5 (58bd20bc7).

One fold verifies; the retained-command migration still misses an executable copy of the old argv. This was a read-only source review at `58bd20bc7`; no builds, tests, file edits or Git writes were performed.

**VOA-11-R4 — verified.** The [revised bootstrap](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:585) executes from the pulled hook before `session start`, removing the dependency on a newly compiled session handler. Its build-then-`up` sequence matches the [existing rearm command](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/cmd/metasystem/rearm_on_landed.go:73), and [`up` re-enrolls rebuilt engine bytes](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/up/up.go:566).

**VOA-16-R5 — fold does not resolve**

**Evidence:** The [migration contract](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:614) updates the retained argv and reservation digest, preserving rounds. However, the runner also materializes argv in `round-N/proof-NAME.json`. It [writes that file only when absent, then passes its path to the launcher](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/launch/unit_run.go:369). [`PlainExec.Command`](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/launch/plain.go:19) reads and executes that file’s argv. The reservation check [validates the plan](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/launch/unit_named.go:295); updating it does not replace an existing proof brief.

**Scenario:** A run is interrupted after writing its proof brief but before launching the proof. Its saved command is the fold’s own `metasystem test --goal G` example. U1 permits cutover because the spelling has a successor. Resume migrates the plan and digest to `metasystem test run --goal G`, but the existing round brief survives and supplies the deleted spelling to `PlainExec`. This failure is inferred from the cited replay path.

**Change:** Extend the owner-controlled migration to the materialized argv of pending, unlaunched proof steps, or regenerate those briefs from the migrated plan before launch. Preserve evidence for steps already launched. Add a witness interrupted after proof-brief creation that checks the command actually executed after U1.

**Test 1:** Yes—changes migration’s persisted-artifact scope and the proof-resume path.

**Test 2:** **WORK**—a migratable suspended run still executes the deleted command and fails despite a successfully migrated reservation.

No additional independent material finding is reported.

Proposed receipt, not written: `Read-only revision-5 confirmation; source inspection; one fold verified, one failed fold.`

Round-4 folds verified: 1 of 2
Material findings: 1

Codex session ID: 01a0dfc7-afa8-7b22-9279-4268dcd347a3
Resume in Codex: codex resume 01a0dfc7-afa8-7b22-9279-4268dcd347a3

---

Round 6: Codex gpt-6-astra, read-only confirmation of revision 6 (c51afa0cf).

**VOA-16-R5 is verified.** Revision 6 resolves the missed proof-file migration; no new material defect was found.

The original failure is confirmed by the code: [unit_run.go:371](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/launch/unit_run.go:371) preserves an existing `proof-NAME.json`, and [plain.go:20](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/launch/plain.go:20) reads that file to construct the executed command. [Section 7:620](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:620) now explicitly migrates this executable copy before an unlaunched step proceeds. Its [witness at line 634](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/plans/designs/verbs-object-action.md:634) checks the command actually executed after interruption, directly covering the original WORK failure.

The extension introduces no identified WORK or SAFE defect. It preserves launched steps’ files and evidence and records rewritten argv in the migration history. This fits the existing launch boundary: [read_sequence.go:38](/Users/wido/LocalStorage/GitHub/agentic-tools-verbs/metasystem/internal/launch/read_sequence.go:38) records `starting` before launch, then checks for the launch record before starting anything. Thus an interrupted `starting` step with no launch record is still unlaunched and covered by the new requirement; existing launches remain untouched.

No other new material defect was found in revision 6’s changes. Verification was by source inspection at `c51afa0cf`; no builds, tests, file edits, or Git writes were performed.

Proposed receipt, not written: `Read-only revision-6 confirmation; materialized proof-argv fold verified; no new material findings.`

Round-5 folds verified: 1 of 1
Material findings: 0

Codex session ID: 01a0dfcc-9101-7780-9d76-f2bb4a552bb5
Resume in Codex: codex resume 01a0dfcc-9101-7780-9d76-f2bb4a552bb5

---

Revision 7 read: Codex gpt-6-astra, read-only, against b5354fcc5.

Reviewed revision 7 at `b5354fcc5` in `/Users/wido/LocalStorage/GitHub/verbs-b3`. Findings below are source-traced; no files, commits, or Git state were written. Later design commits arriving during the review are excluded.

**VOA-18 — Deleting the hook stub breaks the Go Stop worker**

**Evidence:** [Design:292](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:292) replaces runtime settings with the direct hook entry and deletes the stub after checking entry acceptance. However, [hook_entry.go:104](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/cmd/metasystem/hook_entry.go:104) still launches the Stop worker with `exec.Command("bash", script, runtime, "stop")`. Ordinary Stop calls enter that supervisor at [runtime_hook_stop.go:76](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/hooks/runtime_hook_stop.go:76). A failed worker produces a degraded allowance at [runtime_hook_deadline.go:203](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/hooks/runtime_hook_deadline.go:203).

**Scenario:** U9 verifies that `internal hook --accepts` succeeds, updates settings, and removes the stub. The next Stop enters Go successfully, but its child tries to execute the deleted script. The actual Stop decision never runs.

**Change:** Include the worker launch in U9: launch the engine’s hook entry directly, preserving the deadline-parent identity, payload, installation context, and cleanup behavior. Require an end-to-end Stop witness with the stub absent, covering both an allowed and a blocked decision.

**Test 1:** Yes — changes the Go worker launcher and U9’s acceptance test.

**Test 2:** **WORK and SAFE** — normal Stop evaluation becomes a degraded allowance despite an available engine.

**VOA-19 — The generated launcher loses engine discovery and bootstrap**

**Evidence:** [Design:293](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:293) specifies `bin/metasystem internal hook …`. The stub currently selects the primary checkout’s engine for linked worktrees at [supervision-hook.sh:19](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/supervision-hook.sh:19), and bootstraps a missing or older engine at [supervision-hook.sh:79](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/supervision-hook.sh:79). Runtime settings are committed—for example [.claude/settings.json:29](/Users/wido/LocalStorage/GitHub/verbs-b3/.claude/settings.json:29)—while [metasystem/.gitignore:2](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/.gitignore:2) excludes the binary.

**Scenario:** A linked worktree receives the committed direct settings but has no local binary; its valid engine exists only in the primary checkout. The launcher fails before Go can resolve that relationship. Likewise, advancing source and settings in a checkout with an older engine removes the only pre-engine bootstrap; printing fallback JSON does not repair it.

**Change:** Specify engine selection in the generated launcher and the ordering of the managed cutover. Build and validate the selected engine before activating direct settings and deleting their fallback dependency in each affected checkout. Exercise a linked worktree without a local binary and a source-advanced checkout with a preceding engine. This needs no committed script.

**Test 1:** Yes — changes generated commands and the U9 activation protocol.

**Test 2:** **WORK** — an ordinary supported checkout cannot execute lifecycle hooks after cutover.

**VOA-20 — A direct pre-commit hook discards existing hook composition**

**Evidence:** [Design:300](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:300) prescribes `.git/hooks/pre-commit` executing the engine. Existing enrollment resolves Git’s effective hooks directory, including `core.hooksPath`, at [fence.go:66](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/ledgerfence/fence.go:66). It preserves foreign hooks at [fence.go:162](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/ledgerfence/fence.go:162), then runs `pre-commit.local` after the guard at [fence.go:210](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/ledgerfence/fence.go:210). Enrollment also proves execution and rejection propagation, rather than merely recognizing text, at [fence.go:118](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/ledgerfence/fence.go:118).

**Scenario:** A checkout already has a project check preserved as `pre-commit.local`. Replacing its composed hook with the specified direct `exec` silently stops running that check. With a configured hooks directory, writing literally under `.git/hooks` can instead install a fence Git never invokes.

**Change:** Retain the existing enrollment and composition contract, replacing only the guard invocation with the Go entry. Preserve effective-hook-path resolution, foreign-hook ownership, local-hook execution, and the nonce/exit-status probe. Test a rejecting local hook and a repository-local `core.hooksPath`.

**Test 1:** Yes — changes the generated hook contract and its installation checks.

**Test 2:** **SAFE** — commits can bypass existing project checks or the ledger fence.

**VOA-21 — Scoped publication cannot be enforced by a routing-only wrapper**

**Evidence:** [Design:350](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:350) promises refusal for edits outside the named goal set, while [design:337](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:337) limits U1d to routing and text with unchanged owners. The current preview independently captures files at [intent_operations.go:375](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/cmd/metasystem/intent_operations.go:375). `Reconcile` subsequently acquires its own lock, resolves its base, captures again, and maps every delta at [reconcilepub.go:38](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/goal/reconcilepub.go:38). Its [request:211](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/goal/verbs.go:211) carries no allowed-goal set or validated snapshot.

**Scenario:** `goal sync --publish --goal A --by NAME` checks that only A differs. Another writer changes B before `Reconcile` captures the tree. The unchanged owner publishes both edits under the named human. The advertised scope refusal has checked different bytes from those published.

**Change:** Extend the existing reconciliation owner’s request with the allowed goal identities. Enforce set membership against the same captured deltas used for publication, before creating the pending publication record. Do not implement this as a separate preview followed by unscoped reconciliation. Add a fixture introducing B between the wrapper’s inspection and owner capture.

**Test 1:** Yes — changes the owner request and publication admission, beyond public routing.

**Test 2:** **SAFE** — edits outside the caller’s explicitly named scope can be published.

**VOA-22 — Preview-by-default removes journal recovery**

**Evidence:** [Design:350](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:350) gives `goal sync` preview, publish, and interrupted-refresh behavior. The old default instead calls `recoverGoalJournal` at [intent_operations.go:177](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/cmd/metasystem/intent_operations.go:177). This is distinct from `RefreshOnly`, which refuses when no reconciliation refresh is pending at [reconcile.go:484](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/goal/reconcile.go:484). An unresolved pushed journal entry blocks every subsequent publication at [txn.go:609](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/goal/txn.go:609).

**Scenario:** A normal goal mutation dies after recording its push but before confirming the outcome. There is no hand-edit reconciliation refresh. The replacement preview changes nothing; `--refresh` refuses; publication remains blocked. The former recovery action has no replacement.

**Change:** Give journal recovery an explicit mode under the new intent, separate from preview and refresh, and bind it to `recoverGoalJournal`. Update its remedies. Test recovery of a pushed-but-unconfirmed ordinary mutation with no refresh record.

**Test 1:** Yes — adds a missing public mode and owner binding.

**Test 2:** **WORK** — the checkout cannot recover through the replacement surface and resume goal mutations.

**VOA-23 — Removing `work close` loses completion of non-review jobs**

**Evidence:** [Design:346](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:346) assigns closure to finished reviews and landing. Existing `closeChain` handles non-critic terminal chains at [intent_delivery.go:1268](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/cmd/metasystem/intent_delivery.go:1268). `reviewJob` rejects an investigator because it accepts implementers only at [intent_delivery.go:886](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/cmd/metasystem/intent_delivery.go:886). Landing requires a goal and joins a delivery batch at [intent_delivery.go:1390](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/cmd/metasystem/intent_delivery.go:1390). Closure itself mirrors every terminal member, validates durability, marks the chain closed, and removes its build cache at [dispatch.sh:3019](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/dispatch.sh:3019).

**Scenario:** An investigator finishes with useful findings but no code to land. Its chain still needs durable completion. Neither replacement performs that operation: review rejects its role, and landing is an unrelated action.

**Change:** Specify a completion intent or automatic completion point for terminal non-review chains, retaining the existing close owner and its authority and durability checks. Verify that an investigator can complete without launching a critic, joining a landing batch, or concluding its goal.

**Test 1:** Yes — changes the replacement routing and lifecycle coverage.

**Test 2:** **WORK** — an existing job lifecycle loses its supported completion path.

**VOA-24 — R11’s literal scan rejects the declared end state**

**Evidence:** [Design:282](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:282) requires scanning tracked files under `metasystem/` for every non-allowlisted shell file and every `environment/vms` reference. Yet [design:316](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:316) explicitly retains historical mission scripts. `git ls-files` confirms both [gate.sh:1](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/first-headless-run/gate.sh:1) and [guard.sh:1](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/first-headless-run/guard.sh:1). The design itself contains the forbidden VM reference at line 285. “A project’s own testing-contract commands” supplies no concrete directory predicate.

**Scenario:** U9 completes the mandated deletions and preserves the mandated history. A literal R11 implementation still fails on those retained scripts and the document defining the rule. Broad directory exemptions would instead admit unspecified scripts without establishing that they are extension points.

**Change:** Define exact repository-relative predicates for executable extension points and retained historical instruments. Scope VM dependency detection to executable/configuration dependencies, explicitly handling narrative evidence and the witness itself. Pin positive and negative examples so the approved end state passes while a new core script or core VM dependency fails.

**Test 1:** Yes — changes the required static test’s scope and assertions.

**Test 2:** **WORK** — the prescribed end state cannot pass its mandatory closing witness as specified.

Material findings: 7

Codex session ID: 01a0e177-9a64-76b2-a19c-b70ba584bc5e
Resume in Codex: codex resume 01a0e177-9a64-76b2-a19c-b70ba584bc5e

---

Revision 7 confirmation: Codex gpt-6-astra, read-only, against 464fd20d0.

All seven folds resolve their reported failures at the design level. No new material WORK or SAFE defect was found in those folds, C1, or the section 7 withdrawal.

Reviewed revision 7 on `design-r7`, HEAD `464fd20d0d7ecbe2a7387ca49381dfae2587868c`, exclusively in `/Users/wido/LocalStorage/GitHub/verbs-b3`. Verification was by source inspection; no builds, tests, file edits, commits, or Git writes were performed.

**VOA-18 — verified.** The [fold](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:308) explicitly replaces the Stop worker’s script launch with the engine hook entry and preserves its deadline-parent identity, payload, installation context, and cleanup. That addresses the actual dependency in [startStopWorker](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/cmd/metasystem/hook_entry.go:104). The [parent launcher](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/hooks/runtime_hook_deadline.go:326) already supplies the payload descriptors and parent environment, so changing the executable does not require weakening the supervision boundary. The required allowed-and-blocked Stop witnesses with the stub absent cover the original degraded-allowance failure.

**VOA-19 — verified.** The [fold](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:300) restores engine discovery before Go starts and orders activation as build, validate, regenerate settings, then remove the stub. Its primary-checkout selection matches the existing [worktree discovery](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/supervision-hook.sh:19). Source advancement after cutover has an existing Go bootstrap path in [bootstrapEngine](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/hooks/runtime_hook_start.go:755), backed by the [detached rebuild owner](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/cmd/metasystem/hook_entry.go:648). The specified worktree and preceding-engine witnesses exercise both original failures. Missing-engine recovery is now explicitly defined through fallback output and the Go bootstrap command.

**VOA-20 — verified.** The [fold](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:320) retains enrollment and composition, changing only the guard invocation. This preserves [effective hooks-directory resolution](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/ledgerfence/fence.go:66), the [nonce and rejection-propagation probe](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/ledgerfence/fence.go:118), and [local-hook execution after a successful guard](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/ledgerfence/fence.go:210). Consequently, neither a configured `core.hooksPath` nor a rejecting `pre-commit.local` loses its effect. Both have explicit witnesses.

**VOA-21 — verified.** The [fold](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:375) carries allowed goal identities into the reconciliation owner and checks the captured publication deltas before recording a pending publication. This places admission within the existing [locked capture-and-map sequence](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/goal/reconcilepub.go:38), before [WriteBase records publication intent](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/goal/reconcilepub.go:85). Publication then applies those [same mapped rows](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/goal/reconcilepub.go:103). A change to goal B between preview and owner capture is therefore refused when only A was authorized; the wrapper’s earlier observation no longer authorizes different bytes.

**VOA-22 — verified.** The [explicit `goal sync --recover` mode](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:375) restores the lost journal-recovery path independently of preview and refresh. Its named binding, [recoverGoalJournal](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/cmd/metasystem/goalsync_verbs.go:223), retains guard enrollment and the existing recovery policy. [RecoverWithPolicy](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/goal/recover.go:51) classifies stranded entries against canonical history and leaves live owners alone. It does not require the refresh record that [RefreshOnly requires](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/goal/reconcile.go:484). The original pushed-but-unconfirmed ordinary mutation can therefore recover through the replacement surface.

**VOA-23 — verified.** The [new `work finish REF` intent](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:371) provides terminal non-review jobs with their missing completion path while retaining the close owner’s checks. The existing [closeChain](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/cmd/metasystem/intent_delivery.go:1225) checks writer authority and distinguishes critic from non-critic chains. The underlying [closure sequence](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/dispatch.sh:3019) mirrors terminal members, validates durability, records closure, and removes the build cache. [CloseCheck](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/dispatch/close.go:81) permits non-implementer records without requiring an implementer patch. An investigator can thus finish without launching a critic, joining a landing batch, or concluding its goal.

**VOA-24 — verified.** The [revised predicates](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:286) replace the ambiguous blanket scan with an exact extension-point pattern and two explicitly retained historical files. The tracked [gate](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/first-headless-run/gate.sh:1) and [guard](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/first-headless-run/guard.sh:1) match those exceptions. VM-reference detection now names executable/configuration file types and excludes narrative Markdown and the witness’s test literals. The declared end state can pass while an additional core shell script or production/configuration VM dependency remains detectable.

**C1 — no new material finding.** [C1](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:389) changes suggestions without authorizing execution of an unknown command. The current [unknown-command boundary](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/cmd/metasystem/main.go:756) returns a refusal, and [suggestion generation](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/cmd/metasystem/intent.go:1170) already obtains candidates from current public commands. Giving exact current-action matches priority and preserving the remaining arguments needs no removed-spelling registry. The explicit rule governs superseded witness wording elsewhere in the page.

**Section 7 withdrawal — no new material finding.** The [withdrawal](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:699) replaces migration machinery with a measured absence of affected runs and a pre-landing check. The code confirms why that precondition matters: resume reads the [retained plan](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/launch/unit_run.go:154), preserves existing [materialized proof briefs](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/launch/unit_run.go:369), and [executes their stored argv](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/launch/plain.go:19). Given the supplied measurement and required landing precondition, there is no demonstrated affected command requiring migration. I did not independently remeasure the 151 installation records outside the permitted checkout.

Proposed receipt, not written: `Read-only revision-7 confirmation at 464fd20d0; seven folds verified by source inspection; C1 and retained-plan withdrawal checked; no material findings.`

Folds verified: 7 of 7  
Material findings: 0

Codex session ID: 01a0e17f-960e-7d21-abfe-defd86e14046
Resume in Codex: codex resume 01a0e17f-960e-7d21-abfe-defd86e14046

---

Revision 8 read: Codex gpt-6-astra, read-only, against 26708ddce.

Static review of `design-r8` at `26708ddce`. No files changed or tests executed; the tests below are proposed fixtures. All cited paths are under the requested checkout.

**VOA-25 — WORK: The external contract cannot supply mission-host turns**

- **Evidence:** [verbs-object-action.md:426](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:426) defines `command` for “one turn of a job” and names only `delegate-supervisor` as its consumer at line 442. [missionrunner/host.go:337](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/missionrunner/host.go:337) independently resolves `hosts/<runtime>.sh`, refuses its absence, and passes `start-turn`, mission, turn, result and resume arguments. [hosts/claude.sh:31](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/hosts/claude.sh:31) constructs a record-less host command; lines 50–55 produce the host result envelope.
- **Scenario:** Install an external adapter implementing every listed operation and select it as a mission host. Discovery and delegate execution can succeed, but the mission launch still needs a host implementation that the external contract cannot provide. Porting only the four built-in hosts leaves each additional host requiring Go changes.
- **Change:** Include mission-host turns in the same external interface, with explicit host/delegate context, resume identity, permissions and result semantics. Route `missionrunner` through that interface while retaining its existing lineage and supervision ownership.
- **Test 1:** Install an executable under a previously unknown runtime name and complete a mission’s first host turn without adding Go registration or a host script.
- **Test 2:** Resume that mission through the external adapter; verify session continuity, host result and usage records, and cancellation of its owned processes.

**VOA-26 — WORK: The supposedly universal shared layer embeds a runtime permission assumption**

- **Evidence:** [verbs-object-action.md:413](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:413) moves everything in `runtime-common.sh` into an identical shared layer. [runtime-common.sh:144](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/adapters/runtime-common.sh:144) calls `effective-workspace`, then checks for widening. [adapter/permissions.go:20](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/adapter/permissions.go:20) explicitly assumes the baseline CLIs use their working directory as the write boundary; lines 34–35 replace every nonempty write-root set with the workspace root. The enforcement map contains only `mapped`/`notEnforced` labels, enforced by [adapter/snapshot.go:100](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/adapter/snapshot.go:100).
- **Scenario:** A conforming external runtime supports an exact write root `/repo/package` while working in `/repo`. Its launch can enforce the request, but the shared transformation records `/repo` instead. The subsequent comparison refuses the valid launch. Changing that runtime’s executable cannot correct a mandatory shared transformation.
- **Change:** Have runtime preparation report the effective permission envelope produced by its settings and command. Keep comparison and refusal shared; keep runtime-specific permission mapping outside the shared layer. The existing workspace rewrite becomes a built-in runtime choice.
- **Test 1:** An external runtime enforcing a requested subdirectory passes admission without its effective roots being widened to its working directory.
- **Test 2:** The same adapter reports an actually wider grant; the shared owner refuses before starting the CLI.

**VOA-27 — WORK: Command construction and stdout decoding do not cover the existing runtime lifecycle**

- **Evidence:** [verbs-object-action.md:426](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:426) supplies argv/environment/stdin and an output decoder; line 437 gives that decoder CLI output on stdin, without defining its invocation context or other observation channels. Existing behavior includes settings and a callback command at [adapter/claude.go:122](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/adapter/claude.go:122), a prelaunch session baseline at [adapters/devin.sh:535](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/adapters/devin.sh:535), live session polling at line 597, and transcript-based usage and identity settlement at lines 641–646. [runtime-common.sh:391](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/adapters/runtime-common.sh:391) detects runtime repair/settlement callbacks; lines 427–443 invoke repair, recompute usage and certify the repaired session.
- **Scenario:** Implement the interface literally for the existing legacy Devin behavior: stdout remains empty while the turn runs, identity comes from session-list comparison, and delivery and usage arrive in files. A stdout-only decoder cannot reproduce those observations from its specified input. Separately, the shared adjudicator has no defined external equivalent of the repair callbacks it currently invokes. Runtime-specific Go branches or an undocumented second lifecycle become necessary.
- **Change:** Define preparation, observation and finalization inputs explicitly: turn context, generated settings and callback channels, artifact paths, previous usage state, CLI exit status, and repair eligibility. Invoke repair through the same runtime interface, preserving shared adjudication and custody. Merely naming `usage` and handshake events does not define how these facts reach the adapter.
- **Test 1:** An external fixture emits no stdout before completion, establishes its session through a separate channel, and delivers through a named file; handshake and completion must succeed.
- **Test 2:** A malformed return triggers one permitted same-session repair; final usage includes repair spend and conflicting transcript identity refuses completion. A transport declaring repair unavailable must make no repair call.

**VOA-28 — WORK: Name non-shadowing does not prevent signature interference; execution trust is unspecified**

- **Evidence:** [verbs-object-action.md:435](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:435) discovers executable files and prohibits only built-in **name** shadowing. It specifies no installer or directory-writer authority, so who may populate that future directory cannot be verified from this proposal. Today [census/fingerprint.go:48](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/census/fingerprint.go:48) directly executes signature providers without setting a reduced execution identity or environment. [census/signature.go:46](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/census/signature.go:46) applies exclusions only within each signature, then chooses the first matching runtime at line 68. [adapters/devin.sh:798](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/adapters/devin.sh:798) deliberately excludes `devin acp`; [lease/classify.go:430](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/lease/classify.go:430) returns `DELEGATE` immediately upon a signature match.
- **Scenario:** An executable named `devin-compatible` declares the ordinary Devin match but omits its ACP exclusion. It shadows no built-in name. Nevertheless, it claims the host’s `devin acp` intermediary, stopping ancestry classification before the announced main. Built-in-first ordering does not fix this: the built-in deliberately excludes that process.
- **Change:** Define external installation as a trusted-code authority boundary, including authorized writers and execution authority. Protect runtime recognizer ownership, including reserved helper exclusions; reject conflicting declarations instead of treating unique filenames as sufficient isolation. This finding establishes classification interference, **not** a demonstrated privilege escalation.
- **Test 1:** Register the conflicting adapter above; admission must reject it, and the existing host’s descendants must still classify through their announced main.
- **Test 2:** A file supplied outside the authorized installation path must not execute during discovery or lease classification; an authorized, nonconflicting extension must remain discoverable.

**VOA-29 — WORK/SAFE: Runtime signatures are insufficient janitor ownership proofs**

- **Evidence:** [verbs-object-action.md:440](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:440) says janitor recognizers take signatures from the registry. [census/signature.go:17](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/census/signature.go:17) contains only runtime match/exclude patterns. In contrast, [janitor/killproof.go:19](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/janitor/killproof.go:19) requires invocation words and a positioned claim tag, including prefix and path-basename variants. Lines 54–56 declare different tag carriers for Codex, Claude and Devin. [killproof.go:206](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/janitor/killproof.go:206) requires claim-consistent argv in addition to recorded process identity.
- **Scenario:** An external CLI survives its supervisor. Census recognizes its runtime, but the registry’s classification regex does not say where its claim tag appears. Retaining the current proof refuses cleanup; substituting runtime recognition for that proof would authorize signalling without establishing ownership.
- **Change:** Distinguish classification signatures from claim-bound invocation shapes in the contract. Supply the latter declaratively—or through a universally recognizable owned wrapper—while keeping kernel identity and signalling decisions in Go. An adapter’s assertion that a process belongs to it must not itself authorize a kill.
- **Test 1:** Kill the external fixture’s supervisor and verify that janitor identifies and terminates the correctly tagged surviving CLI without runtime-specific Go additions.
- **Test 2:** Processes with a different claim tag, a tag merely mentioned in another argument, or a reused PID must remain unsignalled.

**VOA-30 — WORK: R13 can pass without proving that a new runtime is usable**

- **Evidence:** [verbs-object-action.md:470](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:470) requires fake equivalence, external listing and recognition. Yet [config/validate.go:170](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/config/validate.go:170) rejects runtimes absent from `runtimes.Supported`; [runtimes/runtimes.go:298](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/runtimes/runtimes.go:298) resolves only compiled declarations. Fake already has a declaration at line 267 and does not source the shared lifecycle, as [adapters/fake.sh:158](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/adapters/fake.sh:158) records. The operation list also omits the existing `selftest` entry at [adapters/claude.sh:246](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/adapters/claude.sh:246); custom probes currently require Go registration and otherwise refuse at [adapter/probe.go:47](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/adapter/probe.go:47).
- **Scenario:** Fake produces identical records through both interface implementations, and a separate executable appears in discovery. Every specified assertion passes while production configuration still rejects a fifth runtime name. Runtime selftest integration can likewise remain tied to compiled probes.
- **Change:** Retain differential fake testing, but require a previously unknown runtime identity through production configuration, dispatch and mission paths using an unchanged engine binary. Define the external selftest entry or shared selftest parameters and probe callbacks. Make all relevant consumers use the extensible registry.
- **Test 1:** Install a uniquely named external helper after building the engine; validate its configuration, dispatch, resume and cancel it, and run a mission-host turn without test-only registry overrides.
- **Test 2:** Run its selftest with an external custom probe and runtime-specific denial behavior; verify the evidence and permission-denial checks without adding a Go probe registration.

Material findings: 6

Codex session ID: 01a0e1c8-b52d-7172-914c-8506199ca403
Resume in Codex: codex resume 01a0e1c8-b52d-7172-914c-8506199ca403

---

Revision 8 confirmation: Codex gpt-6-astra, read-only, against 233caaa2b.

The six folds resolve their original failure scenarios. One new **WORK** defect remains in the built-in override rule.

Reviewed `design-r8` at `233caaa2bc260eec97cd0ed24e7bf4eb2967a50f`, exclusively under `/Users/wido/LocalStorage/GitHub/verbs-b3`. Evidence is from source inspection; no builds, tests, file edits, commits, or Git writes were performed.

**VOA-31 — A signature override can discard exclusions required by its built-in fallback**

**Evidence:** The [override rule](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:447) permits replacing `describe` while falling back to built-in operations. The [cross-check](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:451) protects **another runtime’s** positive vectors and reserved exclusions. It does not preserve the displaced implementation’s reservations when both implementations share one runtime name.

This matters for an existing first-use path. Devin’s [reserved lookalike](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/runtimes/runtimes.go:206) is `devin acp`. Its [adapter exclusion](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/adapters/devin.sh:798) keeps that host intermediary transparent to ancestry classification. Otherwise, [the classifier](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/lease/classify.go:433) returns `DELEGATE` before reaching the announced main. The existing [ancestry test](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/lease/classify_test.go:220) requires the opposite result.

**Scenario:** An authorized `adapters.devin.use=external` override implements `describe`, retains the broad Devin match, supplies `metasystem-devin-lookalike` as its negative vector, and omits the ACP exclusion. It returns 64 for launch operations, retaining the built-in host behavior. Its signature matches none of the other runtimes’ positive or lookalike vectors, so the specified cross-check admits it. The first host tool invocation through `devin acp` is then classified as a delegate and loses its main/holder identity.

Keeping the displaced built-in as a separate competing declaration is insufficient: checking its positive `devin` vector as “another runtime” would reject a legitimate same-name override.

**Required change:** Specify one effective declaration per runtime name, while preserving the built-in helper exclusions required by any retained fallback paths. Check overridden signatures against those reservations without treating legitimate matches of that runtime’s own positive vectors as cross-runtime conflicts. Add an R13 fixture that overrides Devin’s `describe`, falls back for host preparation, and verifies both admission rejection when the ACP exclusion is lost and successful host ancestry classification when it is preserved.

**Materiality:** Changes registry admission and R13 assertions. **WORK fails** without it: a permitted partial override can break the existing mission host’s first tool invocation. This is a classification defect; no privilege escalation is claimed.

**VOA-25 — verified.** The [preparation contract](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:423) now carries host/delegate role, mission and turn context, resume identity, workspace and permissions. The [mission-runner requirement](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:433) explicitly replaces the separate [host-script dependency](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/missionrunner/host.go:337). R13 requires an unknown external runtime to serve a mission host turn. The original missing-consumer failure is resolved.

**VOA-26 — verified.** The [fold](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:429) assigns effective-envelope construction to the runtime and retains comparison and prelaunch refusal in the shared layer. This removes the universal application of [RewriteWriteScope](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/adapter/permissions.go:20), which would widen an exact subdirectory grant to the workspace. The existing [comparison owner](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/adapter/permissions.go:40) remains applicable to the returned envelope. A partial override of preparation does not, under this contract, bypass that comparison.

**VOA-27 — verified.** The [operation table](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:423) now covers settings, callback channels, artifact observation, finalization, prior usage, observed identity and same-session repair. These cover the previously missing [Devin baseline and live correlation](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/adapters/devin.sh:535), [transcript settlement](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/adapters/devin.sh:640), and [repair accounting and settlement](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/scripts/agents/adapters/runtime-common.sh:427). The interface no longer requires runtime observation to come solely from stdout, and shared adjudication remains explicitly owned by Go.

**VOA-28 — verified for the original new-name scenario.** The [trust boundary](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:443) requires explicit configuration, installation-user ownership and safe file modes before execution. The cross-check now rejects the original `devin-compatible` declaration because it matches Devin’s reserved `devin acp` vector. That addresses the original combination of executable discovery and [per-signature-only exclusions](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/census/signature.go:46). VOA-31 concerns the subsequently added same-name override path.

**VOA-29 — verified.** The [recognizer contract](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:457) separates classification signatures from claim-bound invocation shapes and retains the kill decision in Go. The declared tag position, prefix and basename variants cover the existing [Shape contract](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/janitor/killproof.go:19). Kernel identity and claim-consistent argv remain required by [Killable](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/janitor/killproof.go:206). R13 now exercises orphan cleanup and rejection of incorrect tags and reused PIDs.

**VOA-30 — verified.** The [consumer rule](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:463) explicitly includes configuration validation, runtime resolution, probes, selftest, dispatch and missions. This reaches the existing [compiled-runtime admission check](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/config/validate.go:170) and [compiled custom-probe lookup](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/internal/adapter/probe.go:47). [R13](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:487) now requires a previously unknown helper installed after the engine was built, including dispatch, resume, cancellation, host execution and custom-probe selftest. Fake equivalence alone can no longer satisfy it.

**Exit 64 — no separate material finding.** The stated rule reserves 64 for operations the override does not implement. Falling back at that interface boundary does not authorize skipping the [shared permission, custody or adjudication owners](/Users/wido/LocalStorage/GitHub/verbs-b3/metasystem/plans/designs/verbs-object-action.md:411). A trusted executable performing an operation and then falsely reporting it unimplemented would violate that contract; it is not an additional demonstrated design failure within the declared trust boundary.

Proposed receipt, not written: `Read-only revision-8 confirmation at 233caaa2b; six original folds verified by source inspection; one new WORK finding, VOA-31, concerning signature reservations under partial built-in overrides.`

Folds verified: 6 of 6  
Material findings: 1

Codex session ID: 01a0e1d0-7f8b-7751-93f3-b5b738469c46
Resume in Codex: codex resume 01a0e1d0-7f8b-7751-93f3-b5b738469c46
