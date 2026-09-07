Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 3: finish slice 1 and repair what round 2 broke (chain stopverb-build1)

Round 2 built the fence, the lock, the creation claims, the ordered
inventory and teardown, the three verbs, the fence readers and the
typed shutdown report across 59 files, with the fast Go gate and the
focused package tests green, and stopped at the partial-slice boundary
the brief allowed. It carried seven gaps. This round closes them. The
specification stays metasystem/plans/metasystem-stop-verb-design.md
(section 13 wins over earlier sections); the boundary stays D2 as
amended by D5 in the round-2 brief (plans/metasystem-stop-verb-fold1-brief.md,
landed after this worktree was cut: the command-layer files main.go,
run.go, supervise_owner.go, supervise_component.go, supervise_arming.go,
up.go, steward_verbs.go, missionrunner_verbs.go, proof_run.go,
dispatch_verbs.go, delegate.go and the new process_verbs.go under
cmd/metasystem), plus the one file D8 adds.

# Facts (the orchestrator's runs, 2026-09-07 00:55Z to 01:05Z)

- On main at 4ff5ac75, `scripts/agents/supervision-fixtures.sh` passes
  every scenario except `rearm-launch-fails`, which is red on main and
  owned by another goal. On the round-2 tree the bed also failed
  `census-lifecycle`, `stop-hook-monitor`, `rearm-rebuild` and
  `rearm-provenance`: those four are regressions of round 2.
- `scripts/agents/dispatch-fixtures.sh` line 1795 and
  `scripts/agents/channel-fixtures.sh` lines 105 and 170 pass
  `--review-by 2026-09-06`, a literal now in the past, so the
  temporary-human-word scenarios refuse and the dispatch bed goes red
  before its assertions on every tree, main included.
- The round-2 review (`rounds/2/review.json`) records the reviewed tree
  82df1393; its diff applies to main.

# Decisions (the orchestrator's; decided, not open)

D7. The four regressions are yours to fix first: find what the round-2
change did to `up`, the owner, the census or the hook path that turns
`census-lifecycle`, `stop-hook-monitor`, `rearm-rebuild` and
`rearm-provenance` red, and repair it without weakening a scenario;
state the cause in your return. `rearm-launch-fails` is not yours.

D8. The boundary gains metasystem/scripts/agents/channel-fixtures.sh.
In it and in metasystem/scripts/agents/dispatch-fixtures.sh the
`--review-by` literal becomes a date computed at run time, today plus
one day in UTC, through one small helper in the shared fixture library
if the beds share one (`scripts/agents/fixture-bed-scenarios.sh` or
`fixture-budget.sh`), otherwise in each bed; portable across the macOS
and GNU `date` commands.

D9. Then the rest of slice 1: every section 10 scenario the round-2
return lists as missing (stop-everything, seat-survives, status-is-live,
stop-fence, arm-again, arm-refuses-survivor, mission-stop with its two
variants and the dead-runner live-turn case, proof-run-stop with the
dead-watchdog case, slow-owner, crash-recovery, remote-job,
ignored-signal, wrong-terminal, seat-refused); the package-test matrices
(creator-race at every seam, orderly-owner-unproven-component,
survives-kill, fence-readers); the fixture-only
`METASYSTEM_STOP_CRASH_AFTER` seam; and one process enumeration shared
with the supervision family for the untracked sweep.

D10. Source comments describe the application, never this round, the
critique, or a finding id. Fixture-only seams are refused outside a
fixture-mode root.

# Verification

Required, reported at evidence level ran, with the private caches of
D6: `scripts/agents/go-gate.sh --fast`; `go test` over every package in
the boundary; then `scripts/agents/supervision-fixtures.sh` (green
except `rearm-launch-fails`), `scripts/agents/dispatch-fixtures.sh`,
`scripts/agents/goal-cli-fixtures.sh`, `scripts/agents/mission-fixtures.sh`
and `scripts/agents/suite-progress-fixtures.sh`, all green, each new
scenario named as passing. State which scenarios fail against the
round-2 tree before your change.

# Constraints

Wall-clock budget: 120 minutes. D7 and D8 come first and are not
optional; if D9 does not fit, stop at a green gate with the scenarios
done so far listed and the rest named as a gap. Return per the
implementer schema with the cumulative diff boundary listed. Gap rule:
stop and report a gap; never fill it silently.
