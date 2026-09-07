Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 2: the command-layer boundary the round-1 brief omitted (chain stopverb-build1)

Round 1 stopped on a gap the orchestrator owns: decision D2 of
metasystem/plans/metasystem-stop-verb-build-brief.md admitted one
command-layer file, `cmd/metasystem/main.go`, while section 13.1 of
metasystem/plans/metasystem-stop-verb-design.md puts the run creation
claim around a process start that `runRunLaunch` owns in `run.go`, and
section 10's fixture-only ignore-TERM seams need the flag parsing and
signal handling that `supervise_owner.go` and `supervise_component.go`
own. Nothing was changed in round 1; this round builds slice 1 in full
under the corrected boundary. Everything else in the round-1 brief
stands: D1, D3, D4, the verification and the gap rule.

# Decisions (the orchestrator's; decided, not open)

D5. D2 is amended. The command layer of the boundary is every file under
`cmd/metasystem/` that owns a seam section 12 or 13 of the design names:
metasystem/cmd/metasystem/main.go (dispatch and usage lines),
metasystem/cmd/metasystem/run.go (the run creation claim around the
wrapper start), metasystem/cmd/metasystem/supervise_owner.go and
metasystem/cmd/metasystem/supervise_component.go (the ignore-TERM and
slow-stop seams and the owner's published teardown ceiling),
metasystem/cmd/metasystem/supervise_arming.go and
metasystem/cmd/metasystem/up.go (the shutdown report printed by the
internal long form), metasystem/cmd/metasystem/steward_verbs.go (the
fence refusals of `steward arm` and `steward restart`, the `arm`
composition, the gate function shared by the four verbs),
metasystem/cmd/metasystem/missionrunner_verbs.go (the fence at launch
and loop start and the human check that opens it),
metasystem/cmd/metasystem/proof_run.go (the fence, the claim and the
record in the launcher's verb), metasystem/cmd/metasystem/dispatch_verbs.go
and metasystem/cmd/metasystem/delegate.go (the `REFUSED-STOPPED`
outcome's two lines), plus the new `cmd/metasystem/process_verbs.go`.
The internal packages, scripts and fixture files of D2 are unchanged.
Any other file under `cmd/metasystem/` is still a gap.

D6. Go caches in the sandbox: the previous chain on this machine ran the
gate with a private cache under the sandbox's temporary directory
(`GOCACHE=$TMPDIR/gocache`, and `XDG_CACHE_HOME=$TMPDIR/xdg` for
staticcheck); do the same rather than reporting the user cache as a gap.

# Verification

As in the round-1 brief: `scripts/agents/go-gate.sh --fast`,
`scripts/agents/dispatch-fixtures.sh`, `scripts/agents/goal-cli-fixtures.sh`,
and the supervision, mission and suite-progress fixture beds section 10
joins, all reported at evidence level ran, with the section 10 and 13
scenarios and package tests listed as passing and the pre-change
failures stated.

# Constraints

Wall-clock budget: 120 minutes. If the slice does not fit one round,
stop at a green gate with the families done so far listed and the rest
named as a gap; a follow-up round continues. Return per the implementer
schema with the cumulative diff boundary listed. Gap rule: stop and
report a gap; never fill it silently.
