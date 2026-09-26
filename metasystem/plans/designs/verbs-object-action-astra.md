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
