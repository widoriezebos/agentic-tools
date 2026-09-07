Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 5: the last round of slice 1 (chain stopverb-build1)

Round 4 stopped on three boundary gaps and one specification gap, all
correctly raised. This round answers them and finishes slice 1. The
specification stays metasystem/plans/metasystem-stop-verb-design.md,
whose section 13 wins over any earlier section it contradicts. The
boundary stays what the round-2 and round-3 briefs set (the internal
packages and scripts of the first build brief, plus these files under
`cmd/metasystem`: main.go, run.go, supervise_owner.go,
supervise_component.go, supervise_arming.go, up.go, steward_verbs.go,
missionrunner_verbs.go, proof_run.go, dispatch_verbs.go, delegate.go and
the new process_verbs.go, plus
metasystem/scripts/agents/channel-fixtures.sh) with the one file D14
adds.

# Facts

- The orchestrator ran `scripts/agents/supervision-fixtures.sh` on the
  round-3 tree outside your sandbox: every scenario passed except
  `rearm-launch-fails`, which is red on main and owned elsewhere. The
  four "regressions" of round 2 were a sandbox artifact, as round 3
  proved. Nothing there is yours.
- metasystem/scripts/agents/suite-progress-fixtures.sh is mode 100644 in
  the tree; the validation script invokes it through `bash`. Do the same
  rather than reporting it as non-executable, and do not change its mode.

# Decisions (the orchestrator's; decided, not open)

D14. The boundary gains metasystem/scripts/agents/hosts/fake.sh. The
fake host gains, fixture-mode only, a held host that cooperates with the
orderly signal and, under `METASYSTEM_FAKE_HOST_IGNORE_TERM`, one that
ignores it, so the mission-stop scenario exercises the production seam.

D15. The single shared process enumeration moves to slice 2. Section 3
step 7's untracked sweep keeps its own enumeration in slice 1:
`internal/census` exports no entry point that accepts a pre-enumerated
table, and that package stays outside this boundary. Say so in your
return; do not add an export.

D16. The dispatch fixture bed's temporary-human-word scenario cannot
pass today and is NOT yours to fix. Its blocker is the constant
`TemporaryGoalAuthorityHorizon` in metasystem/internal/governance/types.go,
a human policy date (ruling R-32-m1) that passed on 2026-09-06, so
`--review-by` refuses every date now. Do not touch that package or that
constant. Keep the run-time date computation round 3 added, run the
dispatch bed, and report that one scenario as a known pre-existing block
with the constant named; every other scenario in that bed must pass.

D17. The remote-job survivor must read as sections 2 and 9 require: the
stop line and the `arm` refusal both name the owning machine and the
exact cancellation command for it, `metasystem delegate --cancel <job>`.

D18. Then the rest of slice 1, ordered so a stop leaves whole scenarios:
stop-everything, seat-survives, status-is-live, stop-fence, arm-again,
arm-refuses-survivor, mission-stop with its ignore-TERM and
host-ignores-TERM variants and its dead-runner live-turn case,
proof-run-stop with its dead-watchdog case, slow-owner, crash-recovery,
remote-job, ignored-signal, wrong-terminal, seat-refused; the
orderly-owner-unproven-component and supervision survives-kill package
cases; and the fixture-only `METASYSTEM_STOP_CRASH_AFTER` seam. Each
scenario compares stdout line for line against the section 6 grammar.
Fixture-only seams are refused outside a fixture-mode root.

D19. Source comments describe the application, never this round, a
critique, or a finding id.

# Verification

Reported at evidence level ran, with the private caches round 3 used:
`scripts/agents/go-gate.sh --fast`; `go test -count=1` over every
package in the boundary; `scripts/agents/dispatch-fixtures.sh` and
`scripts/agents/goal-cli-fixtures.sh` green but for the D16 scenario;
for the supervision, mission and suite-progress beds, the exact command
and the sandbox outcome, plus the list of scenarios you wrote and expect
green, which the orchestrator runs outside the sandbox. State which
scenarios fail against the round-4 tree before your change.

# Constraints

Wall-clock budget: 120 minutes. This is the last round of slice 1: if
something still does not fit, stop at a green fast gate and name it
precisely. Return per the implementer schema with the cumulative diff
boundary listed. Gap rule: stop and report a gap; never fill it
silently.
