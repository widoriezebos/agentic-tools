Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 4: the regression premise was wrong; finish slice 1 (chain stopverb-build1)

Round 3 repaired the fixture review date (D8) and stopped, rightly, on
decision D7: it showed that the four supervision scenarios named as
regressions fail identically on the unchanged base inside the delegate
sandbox, at the execution of a freshly copied fixture binary (shell
status 126, no Go output). The orchestrator has now run
`scripts/agents/supervision-fixtures.sh` on the round-3 tree outside the
sandbox: every scenario passes except `rearm-launch-fails`, exactly as
on main. D7 is withdrawn; there is no regression to repair. The
specification stays metasystem/plans/metasystem-stop-verb-design.md
(section 13 wins); the boundary stays D2 as amended by D5 (the
command-layer seam files under cmd/metasystem) and D8
(scripts/agents/channel-fixtures.sh).

# Decisions (the orchestrator's; decided, not open)

D11. D7 is withdrawn. The supervision bed cannot execute a copied
binary in your sandbox; that is an environment fact, not evidence about
the diff. Do not spend time on it. Write the scenarios; the orchestrator
runs the supervision, mission and suite-progress beds on the returned
tree outside the sandbox and hands back any failure with its evidence
for the next round. Run in the sandbox what runs there (the fast Go
gate, the package tests, the dispatch and goal beds, and any bed that
does execute), with the private caches of D6.

D12. This round builds D9 in full: every section 10 scenario the
round-3 return lists as missing (stop-everything, seat-survives,
status-is-live, stop-fence, arm-again, arm-refuses-survivor,
mission-stop with its two variants and the dead-runner live-turn case,
proof-run-stop with the dead-watchdog case, slow-owner, crash-recovery,
remote-job, ignored-signal, wrong-terminal, seat-refused), each in the
bed section 10 assigns it and each comparing stdout line for line
against the section 6 grammar; the package-test matrices (creator-race
at every seam, orderly-owner-unproven-component, survives-kill,
fence-readers); the fixture-only `METASYSTEM_STOP_CRASH_AFTER` seam; and
one process enumeration shared with the supervision family for the
untracked sweep. Order the work so that a partial stop leaves whole
scenarios, never half of one.

D13. Source comments describe the application, never this round, the
critique, or a finding id. Fixture-only seams are refused outside a
fixture-mode root.

# Verification

Reported at evidence level ran, with the private caches:
`scripts/agents/go-gate.sh --fast`; `go test -count=1` over every
package in the boundary; `scripts/agents/dispatch-fixtures.sh` and
`scripts/agents/goal-cli-fixtures.sh` green; for the supervision,
mission and suite-progress beds, the exact command and the sandbox
outcome, and the list of scenarios written and expected green. State
which scenarios fail against the round-3 tree before your change (all
of the new ones, by absence).

# Constraints

Wall-clock budget: 120 minutes. If D12 does not fit, stop at a green
fast gate with the scenarios done listed and the rest named as a gap.
Return per the implementer schema with the cumulative diff boundary
listed. Gap rule: stop and report a gap; never fill it silently.
