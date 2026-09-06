# metasystem stop design critique — round 1 (revision 1)

Chain: revision 1 (landed 426012505, sha256 19abc2fdbc5ee11049c04df0f80b0a4debc30a88d96d4784e63ea3ed373c3979) -> critic stopverb-crit1-20260906 (Sol, design-critic, read-only runtime; the coordinator carried its return here verbatim because the job was failed at validation: the return named itself by a descriptive id instead of the job id, so the engine recorded protocol_error and consumed no review round). Reviewed commit 426012505df49c851a4381984601eeaea41c294b. 11 material findings.

## STOP-R1-001 — critical, material=True

CLAIM: STOP-R1-001 — The proposed stop is not omnipotent. A successful invocation can leave a running mission's detached run-loop and active host process group, a monitored-run wrapper and workload, and a proof-run suite with its sibling watchdog alive. The design defines neither record terminalization nor process-identity-safe orderly and forceful termination for these managed process families.

EVIDENCE: The inventory and stop order in metasystem/plans/metasystem-stop-design.md:20-54 and 142-187 omit these families. metasystem/internal/missionrunner/launch.go:613-646, metasystem/internal/missionrunner/loop.go:99-180, metasystem/internal/missionrunner/host.go:340-418, metasystem/cmd/metasystem/run.go:90-216, and metasystem/internal/proofrun/launcher.go:43-149 show that they create detached or grouped processes with distinct lifecycles. The omission would cause an implementer to build a stop command that reports success while metasystem work continues.

## STOP-R1-002 — critical, material=True

CLAIM: STOP-R1-002 — The stopped record is not a closed process-creation fence. Direct steward arm, restart, and run operations, the watcher's steward-repair pass, monitored-run launch, and proof-run launch remain able to create processes while the checkout is stopped. The stated stop order even stops the steward before the watcher, allowing the still-running watcher to recreate it during the same command.

EVIDENCE: metasystem/plans/metasystem-stop-design.md:138 says nothing else reads the record and leaves steward long forms unchanged at lines 421-423. metasystem/cmd/metasystem/steward_verbs.go:507-633 exposes direct creation paths; metasystem/cmd/metasystem/supervise_component.go:181-217 calls RepairEnrolledRunner; metasystem/internal/steward/runner.go:331-477 can start a replacement. The run and proof-run launch paths contain no stopped-record check.

## STOP-R1-003 — critical, material=True

CLAIM: STOP-R1-003 — Stop and arm lack a checkout-wide transition contract. Stop writes the fence before a multi-step cancellation and shutdown, while arm clears it before recovery, but neither shares a named lock, generation, rollback rule, or failure-state mapping. Concurrent or partially failed operations can therefore clear the fence and start a new generation while stop is killing processes, or leave the checkout partially stopped but recorded as armed.

EVIDENCE: metasystem/plans/metasystem-stop-design.md:108-116 makes writing stopped.json the first stop action; lines 89-95 and 133 make clearing it the first arm action. The only named locks are narrower steward or mission-launch locks. No checkout-wide serialization or recovery rule is specified for the aggregate transition, so implementers must invent materially different race and failure behavior.

## STOP-R1-004 — critical, material=True

CLAIM: STOP-R1-004 — The promised honest orderly-versus-forceful output cannot be implemented while ShutdownAt and up.Shutdown remain unchanged. Those APIs discard which signal stopped each process, return only an error, and do not append the promised reaped reason after caller escalation. Consequently the command cannot print exactly one truthful TERM or KILL line per owner, watcher, and reaper, and the required ignored-signal behavior has no observable result to consume.

EVIDENCE: The output contract is in metasystem/plans/metasystem-stop-design.md:189-234, while lines 330-331 declare ShutdownAt and up.Shutdown unchanged. metasystem/internal/supervise/arming.go:270-316 and 400-541 hide signal outcomes behind errors; lines 810-840 return only an error. metasystem/internal/up/up.go:765-777 returns a generic owner-stopped line. No shutdown-escalated implementation was found in these packages.

## STOP-R1-005 — high, material=True

CLAIM: STOP-R1-005 — The headless handover is specified against the wrong process identity. The design says human mission start clears the stopped record and arms as the mission-runner identity, but today's flow arms with the transient mission-start command before the detached runner exists. It also says human mission resume may proceed without specifying whether resume clears and rearms. An implementer cannot produce the required self-owned headless handover from the described changes.

EVIDENCE: metasystem/plans/metasystem-stop-design.md:130-135 and 253-261 promise mission-runner ownership. metasystem/internal/missionrunner/launch.go:613-646 calls armAndPreflight before spawning the detached run-loop, and metasystem/internal/missionrunner/launch.go:347-371 resolves the current launcher process identifier. The required control-flow transfer into the detached runner is absent from the design.

## STOP-R1-006 — high, material=True

CLAIM: STOP-R1-006 — Human-enrollment gating for status is an unsupported authority expansion that makes the read-only companion unusable by seats, scripts, and fleet coordination. Reading the same list that stop would act on grants no stopping authority, and the design names no invariant protected by denying that observation.

EVIDENCE: metasystem/plans/metasystem-stop-design.md:236-249 gates status solely because it exposes the human page. Existing diagnostic status and health operations at metasystem/cmd/metasystem/supervise.go:105-159 and metasystem/cmd/metasystem/steward_verbs.go:41-76 are open read surfaces. This changes the public interface and forces automation to understand or impersonate terminal enrollment.

## STOP-R1-007 — critical, material=True

CLAIM: STOP-R1-007 — Fleet discovery is not a live-checkout inventory for an omnipotent stop. It selects checkouts only through open supervision publication or claim state and explicitly skips a checkout with a provably dead owner, although that checkout may still contain a steward runner, mission, monitored run, proof suite, watchdog, watcher, or reaper. The page correctly reduces append-only rows per checkout, but it reduces an incomplete source of truth.

EVIDENCE: metasystem/plans/metasystem-stop-design.md:299-316 defines the fleet set and the dead-owner skip. metasystem/internal/registry/reduce.go:107-170 shows that publication and owner checkout selection model supervision ownership, not every process family. Therefore a per-checkout reduction can still omit a checkout containing live metasystem processes.

## STOP-R1-008 — high, material=True

CLAIM: STOP-R1-008 — The contract that status equals stop's list is internally unimplementable. After the first stop, the second stop must print only that nothing is running, while status in the same state must still list every previously seen identity as not running. The proposed stopped record stores no inventory snapshot, and job cancellation mutates running status to cancelled, so the design does not decide whether the shared list is current, historical, or captured at stop time.

EVIDENCE: metasystem/plans/metasystem-stop-design.md:101-103 requires the same list, lines 211-214 collapse a no-op stop, and fixture stop-status-matches at lines 357-359 requires status to retain each prior identity. The stopped-record schema at lines 108-116 has only version, time, terminal, and checkout fields. Different implementers would necessarily choose different list semantics.

## STOP-R1-009 — critical, material=True

CLAIM: STOP-R1-009 — The proof plan cannot arbitrate the design as written. It has no fixture for a running mission and host turn, monitored run, or proof suite and watchdog; it does not test the stopped fence against direct steward repair or uncovered launch paths; and it never positively proves the human's enclosing session identity survives. Its two escalation fixtures also rely on seams that do not produce the claimed behavior.

EVIDENCE: The fixture inventory at metasystem/plans/metasystem-stop-design.md:335-406 omits the managed process families and fence readers above. The ignored-signal fixture assumes an existing owner-ignore seam, but metasystem/cmd/metasystem/supervise_owner.go:161-175 always installs orderly-signal handling and the crash seam exits. The survives-kill fixture uses METASYSTEM_CENSUS_PROCESS_FILE, while shutdown liveness and signaling in metasystem/internal/supervise/arming.go:270-316 and 523-530 use kernel process identity; a census row cannot keep a process alive after KILL.

## STOP-R1-010 — high, material=True

CLAIM: STOP-R1-010 — The design assigns the stopped record to a package that cannot serve all named readers without an import cycle and leaves aggregate process-control ownership undefined. internal/up already depends on internal/steward, while the design requires steward health to read state owned by internal/up. Placing stop, status, arm transitions, inventory, refusals, and output assembly in the command package also leaves core state-machine decisions outside a named testable Go owner.

EVIDENCE: metasystem/plans/metasystem-stop-design.md:318-331 assigns state ownership to internal/up, a health reader to internal/steward, and aggregate orchestration to cmd/metasystem/process_verbs.go. metasystem/internal/up/up.go:16-23 already imports internal/steward. A direct steward-to-up reader would create a Go cycle, forcing the implementer to invent a new package or duplicate the contract.

## STOP-R1-011 — medium, material=True

CLAIM: STOP-R1-011 — One required refusal ends with a command known not to work. When fleet stop finds a live checkout without an installation, it prints metasystem stop --repo followed by that checkout, but the same design says repository resolution rejects a checkout without its installation. Repeating the command therefore repeats the refusal instead of telling the human how to stop it.

EVIDENCE: metasystem/plans/metasystem-stop-design.md:79-87 defines installation-dependent repository resolution; lines 287-291 prescribe the same per-checkout command for the missing-installation fleet case. The remedy does not change the failed precondition and violates the design's own refusal contract.

## Gaps the critic named

- No process was launched or signaled; live proof was not required, and the review was intentionally read-only.
- The repository-local copy of the required design-critique skill file was absent. The complete binding skill text embedded in the review brief was read and applied instead.
- The declared outputs-manifest digest could not be recomputed because the brief supplied no manifest location. The separate design-file digest and reviewed commit were verified.
- The launcher classified this runtime as advisory and did not prove tool-catalog isolation or independent context isolation.

## Coordinator disposition (m1, 2026-09-06)

All eleven findings are accepted and fold into revision 2 of the design;
none is deferred to the build, because each changes what gets built.
Grouped:

- Inventory and omnipotence (001, 007): the page's inventory of what the
  metasystem runs gains the mission runner (the detached run loop and its
  host turn's process group), monitored runs, and proof-run suites with
  their sibling watchdogs, each with its record's terminal state and its
  identity-safe orderly-then-forceful stop. Fleet discovery becomes a
  live-process inventory: the registry reduction joined with a census of
  the configured signatures by checkout path and with the mission, run and
  proof records, so a checkout whose owner is dead but whose runner or
  mission is alive is stopped, not left to the janitor.
- The fence and its ownership (002, 003, 010): the stopped record becomes
  a closed process-creation fence read by every creation path, including
  the steward's direct arm, restart and run, the watcher's steward-repair
  pass, monitored-run launch and proof-run launch; the order of stopping
  is restated so nothing can recreate what was just stopped; stop and arm
  share one checkout-wide lock, a generation, a rollback rule and a
  failure-state mapping; and the record lives in one small package of its
  own that up, steward, dispatch, mission and goal import without a cycle,
  with the transition owned by a named, testable Go owner rather than the
  command package.
- Honesty (004, 008, 011): the shutdown transaction is extended, not
  copied, to return per-component outcomes (which signal ended each
  process, and the escalated reason appended), so every printed line is
  true; status is the current live inventory and stop prints what it
  acted on, with the "same list" contract restated on that basis and the
  historical-identities fixture dropped; the missing-installation fleet
  refusal ends with an act that can succeed.
- The headless handover (005): specified against the real launch flow,
  where the transient launcher arms before the detached runner exists,
  including how the fence clears and how ownership transfers, for start
  and for resume.
- Status is open (006): read-only, no human gate; seats, scripts and the
  fleet brain read it.
- Proof (009): fixtures for every process family and every fence reader,
  a positive proof that the human's enclosing session survives, and
  escalation fixtures built on seams that exist or on seams the page adds
  honestly.

Wido's words stand as the acceptance test: one intuitive word, omnipotent,
honest about what it could not stop.
