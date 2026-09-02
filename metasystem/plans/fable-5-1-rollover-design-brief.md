Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fable-5-1-model-rollover)
Date: 2026-09-02

# Goal

Author a one-page design for goal fable-5-1-model-rollover (read
metasystem/plans/goals/fable-5-1-model-rollover.md first). Wido's order,
verbatim: "Fable 5.1 is released, make sure we use that model for Fable
models going forwards". The model id is claude-fable-5-1; the dispatching
seat verified the CLI accepts it on 2026-09-02 (a one-word probe returned
canonicalModel claude-fable-5-1). You are yourself running on it.

# Workspace

The delegate worktree the dispatcher created for this job. Write exactly one
new file: metasystem/plans/fable-5-1-rollover-design.md (read anything else).

# What the design must settle

1. THE TRACKED LINE. metasystem/metasystem.conf line 6 reads
   `runtime.claude.maximal-models=claude-fable-5`. The maximal gate
   (metasystem/internal/dispatch/hazard.go, runtimeProvesMaximalExecution)
   refuses DESIGN-BEARING and destructive-reach Claude work whose model is
   not in that comma-separated list. Specify the new value. The dispatching
   seat proposes `claude-fable-5-1,claude-fable-5` — both admitted — until
   every fleet seat has switched its machine-local roster, because a seat
   still on claude-fable-5 would otherwise have every governed round refused
   the moment this lands. State whether the old id should be dropped later
   and what evidence would license dropping it (a follow-up ruling, or the
   ledger showing no round on claude-fable-5 for N days), or whether keeping
   it indefinitely is harmless. Check the duplicate and empty-token rules in
   metasystem/internal/config/validate.go (maximalModelsKey) so the chosen
   value validates.
2. WHERE THE LANE MODELS LIVE. The role and mode model keys
   (role.default.model.claude, role.code-critic.model.claude,
   mode.design.role.implementer.model.claude) are machine-local
   (metasystem.conf.local, uncommitted, one per seat). Confirm from
   metasystem/internal/config/resolve.go that no tracked default pins the old
   id and therefore the tracked change is the maximal-models line alone.
   Name exactly what each seat's operator must edit locally (the three keys,
   plus cap.min.<role>.claude.claude-fable-5-1 rows mirroring their existing
   claude-fable-5 rows, see the cap key shape in
   metasystem/internal/dispatch/cap.go).
3. TESTS AND FIXTURES. grep shows claude-fable-5 only as fixture strings in
   metasystem/internal/dispatch/composition_test.go,
   metasystem/internal/dispatch/decisions_test.go,
   metasystem/internal/dispatch/claim_test.go,
   metasystem/internal/config/validate_test.go and
   metasystem/cmd/metasystem/delegate_reroute_test.go — decide whether any of
   them asserts against the real committed conf value (then it must change)
   or merely uses the id as an arbitrary string (then leave it). State the
   answer per file with the line.
4. THE RULING TEXT. R-25-m1 in metasystem/memory/rulings.md names
   claude-fable-5 as the Fable lane model. Specify the ruling row that
   records Wido's rollover word (R-46-m0b, dated 2026-09-02, quoting the
   order) rather than editing R-25's history.
5. LIVE-ROUND SAFETY. The change must not refuse rounds already in flight on
   claude-fable-5 anywhere in the fleet. Say why the chosen value guarantees
   that, or what does.

Self-grade per the house rule: confidence, weakest claim, reject condition.

# Constraints

Wall-clock budget: 15 minutes. This is a small item: the exact conf line,
the per-file test verdicts, the ruling row text, the operator note. No
essay. Do not edit anything but the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file named
under Workspace.

# Gap Rule

stop and report a gap; never fill it silently.
