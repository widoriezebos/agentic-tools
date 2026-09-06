# metasystem stop design critique — round 2 (revision 2)

Chain: revision 2 (landed 5d1688133, sha256 747bb358a8a6a5d3e9beafa55800197ff232e15c7b95535648dfabf17754a359) -> critic stopverb-crit2-20260906 (design-critic, read-only runtime; the return named the job id and validated). Reviewed commit 5d16881336902b166558ee079cc1705eac37271a. 8 material findings. The coordinator carried the return here verbatim because the critic wrote no register file.

## STOP-R2-001 — critical, material=True

CLAIM: STOP-R2-001 — The accepted round-one creation-fence finding remains open: a creator can pass the stopped-fence check, pause, and publish or start a process after stop has closed the fence and completed its inventory. Stop can consequently report success while a new run, proof suite, mission runner, steward runner, or supervision process starts behind it.

EVIDENCE: Read: metasystem/plans/metasystem-stop-design.md specifies only a pre-creation fence read for every creator and gives stop the transition lock, but it does not make creators hold that lock through process publication or bind their work to a fence generation. The same page is internally inconsistent: its lock rule says mission start and resume hold the lock while arming, while the headless handover releases the lock before armAndPreflight. Concrete delayed-start windows exist in metasystem/cmd/metasystem/run.go, metasystem/internal/proofrun/launcher.go, metasystem/internal/missionrunner/loop.go, metasystem/internal/up/up.go, and metasystem/internal/steward/runner.go. The proposed stop-fence fixture is sequential and cannot expose this race.

## STOP-R2-002 — critical, material=True

CLAIM: STOP-R2-002 — The supervision owner can be force-killed while it is still following the orderly shutdown path, so the proposed output “killed (TERM ignored)” is false and the design does not satisfy its rule that force-kill is reachable only after orderly shutdown failed.

EVIDENCE: Read: metasystem/internal/supervise/arming.go sends the owner the orderly termination signal and waits only five seconds before force-killing it. metasystem/internal/supervise/owner.go handles that signal by stopping watcher and reaper sequentially before appending its terminal event. metasystem/internal/supervise/proc.go permits each component stop to consume one five-second StopCeiling, configured in metasystem/cmd/metasystem/supervise_owner.go. One slow component can exhaust the caller's entire allowance; two can require ten seconds plus registry-write time. The design retains the five-second fact and tests only an owner that itself ignores the signal, not an orderly owner with a slow child.

## STOP-R2-003 — high, material=True

CLAIM: STOP-R2-003 — The designed stopping order destroys the proof-run completion evidence it says it will use. When a proof-run launcher is the workload of a monitored run, stopping the run process group first also kills the proof launcher, preventing both the watchdog done marker and the run sidecar from being written.

EVIDENCE: Read: metasystem/cmd/metasystem/run.go starts the wrapper in a new process group and the workload inherits that group; the wrapper writes its sidecar only after the workload returns. metasystem/internal/proofrun/launcher.go writes the watchdog done marker only after the suite wait completes. The design orders monitored runs before proof-run suites and asserts that the wrapper then writes its sidecar, but killing the group terminates the wrapper and launcher together. The proof-run fixture nevertheless expects a red result read from that sidecar, which the specified order cannot produce.

## STOP-R2-004 — high, material=True

CLAIM: STOP-R2-004 — The mission-host result cannot be printed honestly with the proposed data model. A host that ignores the orderly signal can be force-killed by the mission runner, but the stop process receives only the runner's aggregate success and cannot distinguish “stopped by the runner” from host escalation.

EVIDENCE: Read: metasystem/internal/missionrunner/host.go has terminateGroup send the orderly signal, wait, optionally force-kill, and return only an error. The design adds a terminal status to the mission-runner record but no host termination-result field or other observation channel. Its output grammar nevertheless requires separate host results for runner handling, direct orderly termination, and force-kill. The mission fixture covers only a runner that ignores the signal while its host accepts it; it does not exercise a host that requires escalation.

## STOP-R2-005 — high, material=True

CLAIM: STOP-R2-005 — Recovery from a crashed stop has no defined arm outcome. The design says a dead stopping owner causes re-inventory with an empty survivor list, but Arm subsequently checks only the old stored survivor list; a literal implementation can reopen the fence while a process left by the crashed stop is still alive.

EVIDENCE: Read: metasystem/plans/metasystem-stop-design.md first maps a dead stopping owner to stop-incomplete with an empty notStopped list and requires re-inventory, then defines Arm solely as probing identities already stored in notStopped. It never maps live identities discovered by that re-inventory to refusal, resumed stopping, or a new survivor list. Different implementers must choose different state transitions. The crashed-stop fixture is confined to the fence package and does not test aggregate Arm with a live mission, job, proof suite, or supervisor remaining.

## STOP-R2-006 — high, material=True

CLAIM: STOP-R2-006 — A delegate job owned by another machine can remain running while the command exits successfully and prints “stopped.” Calling it “not ours, not touched” excludes it from the only survivor category that produces stop-incomplete, despite the stated checkout-wide omnipotence requirement.

EVIDENCE: Read: the design inventories every nonterminal delegate job but maps a job whose machine identifier differs from the current machine to “not ours, not touched.” Only lines classified as NOT STOPPED populate the survivor list and force exit status 1; the remote-job line is not assigned that outcome. No fixture covers this case. An implementer must therefore either report false success or invent an unstated host-local limitation or remote cancellation mechanism.

## STOP-R2-007 — critical, material=True

CLAIM: STOP-R2-007 — The fleet promise for a checkout whose directory has disappeared has no identity-safe implementation. The design requires the normal locked, fenced checkout transaction while simultaneously removing the state root that contains its lock and authoritative job, mission, run, proof-run, and seat records.

EVIDENCE: Read: metasystem/plans/metasystem-stop-design.md says every live checkout uses the full per-checkout transaction under its transition lock, then says a directory-gone checkout is stopped using registry identities and command-line shapes alone. The supervision registry carries supervision claims, not all process-family records, and its reducer explicitly treats an identity as evidence rather than signal authority. Without the deleted state records, ordinary runtime processes cannot always be distinguished safely from a human Claude session; the design's own untracked-process rule forbids signaling on command-line shape alone. The fleet fixture asserts success without specifying lock behavior, state-root non-resurrection, unavailable-family outcomes, or human-session safety for this branch.

## STOP-R2-008 — high, material=True

CLAIM: STOP-R2-008 — The proposed supervision registry event is invalid for an orderly owner whose component death cannot be proven. Appending “reaped, shutdown-escalated” in that case contradicts the registry contract and may be ignored because an earlier orderly terminal event is absorbing.

EVIDENCE: Read: the design directs ShutdownReport to append a reaped shutdown-escalated event both when the caller force-kills the owner and when component death merely cannot be proven. metasystem/docs/design/supervision-registry.md reserves shutdown-escalated for the caller-force-kill path. metasystem/internal/registry/reduce.go makes terminal events absorbing, so a later reaped event cannot supersede an orderly exited event. The fixtures cover the legal owner-force-kill case, but not an orderly owner with an unproven component.

## Reopening triggers the critic named

- STOP-R2-001: Keep the design open until every process creator is serialized with stop through publication and startup, or an equivalent generation handshake is specified, and a deterministic creator-versus-stop race fixture proves no process can appear after successful stop.
- STOP-R2-002: Keep the design open until the caller's orderly deadline covers the owner's complete teardown contract and a fixture proves that a slow but cooperating owner is not labeled as ignoring the signal or force-killed prematurely.
- STOP-R2-003: Keep the design open until the run and proof-run ownership order preserves a trustworthy completion result, with a fixture nesting proof-run launch under run launch and checking identities, done marker, sidecar, and printed result.
- STOP-R2-004: Keep the design open until host termination outcome is carried to the stop reporter or the output contract is changed, and a host-ignore fixture proves force-kill appears on its own truthful line.
- STOP-R2-005: Keep the design open until live identities discovered after a crashed stop have one explicit Arm outcome and an aggregate crash-recovery fixture proves the fence cannot reopen over a survivor.
- STOP-R2-006: Keep the design open until remote delegate jobs have an explicit checkout-wide cancellation path or are classified as survivors that make stop incomplete, with a multi-machine-identifier fixture asserting the exit status and output.
- STOP-R2-007: Keep the design open until the directory-gone fleet branch specifies its lock and fence authority, complete identity sources, state-root non-resurrection, and safe unavailable-family outcomes, with a fixture proving the human session survives.
- STOP-R2-008: Keep the design open until every ShutdownReport outcome maps to a registry event allowed by the existing reducer contract, with a fixture for an orderly owner whose component death remains unproven.

## Gaps the critic named

- No process was launched or signaled; this was a read-only design critique, and the brief did not require live proof.
- The declared outputs-manifest digest 896cd51f902a230c6ac95576079fa24a94105859640cab3c0af5955edcf5bd0c could not be independently recomputed because the brief supplied no manifest path.
- The runtime notice classifies context isolation and the provider tool catalog as advisory, so neither was independently proven.

## Coordinator disposition (m1, 2026-09-06)

All eight are accepted; every one changes what gets built, so the design
loop's stop criterion is not met by revision 2. Reads per finding:

- STOP-R2-001 (creation race): the fence needs a generation handshake.
  A creator reads the fence generation, publishes its record carrying that
  generation, and stop's inventory after closing the fence treats any
  record carrying the pre-close generation as a survivor to stop. Creators
  never hold the transition lock through startup. The page's own
  inconsistency (mission start holds the lock while arming versus the
  handover releasing it before armAndPreflight) is resolved the same way.
- STOP-R2-002 (owner teardown deadline): the caller's orderly deadline is
  derived from the owner's contract (components times StopCeiling plus
  registry write) instead of the fixed five seconds; the fixture is a slow
  but cooperating owner that must not be force-killed.
- STOP-R2-003 (proof-run under run): stop proof-run suites before the
  monitored runs that host them, and read the done marker and sidecar in
  that order; fixture nests proof-run launch under run launch.
- STOP-R2-004 (host result): terminateGroup returns an outcome, the
  runner record carries the host termination result, and the stop
  reporter prints it; add the host-ignores-TERM fixture.
- STOP-R2-005 (crashed stop): re-inventory after a dead stopping owner
  produces a new survivor list; Arm refuses over any live survivor and
  prints the resumed-stop command; aggregate crash fixture.
- STOP-R2-006 (remote jobs): a delegate job owned by another machine is a
  survivor that makes stop incomplete (NOT STOPPED, exit 1) with the
  owning machine named; no remote cancellation is invented.
- STOP-R2-007 (directory gone): the promise is withdrawn. A registry
  checkout whose directory is gone is reported as checkout-gone and never
  signalled from command-line shapes; this keeps the untracked-process
  rule and the human session safe. Wido may overturn this if he wants the
  stronger fleet promise, at the cost of a records-free identity design.
- STOP-R2-008 (registry event): the unproven-component case appends no
  reaped event; it is reported on the stop line only, and the escalated
  row is reserved for the caller-force-kill path the registry contract
  allows.

Next: revision 3 folds these eight; it is the last design round for this
page. The build then proceeds behind the section 10 fixtures whatever a
third critique says (D81), with any remaining findings carried as build
findings.
