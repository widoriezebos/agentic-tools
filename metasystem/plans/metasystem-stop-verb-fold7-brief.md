Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 8: a stop must be durable, and it must say who stopped it (chain stopverb-build1)

Wido added a binding requirement to this goal this morning, in his own
words: "If I stop it, the supervisor obviously should die too, instead
of starting it again. That kind of defeats the purpose. ... This is
totally broken at the moment and needs a fix." The evidence behind it:
his session stopped at 07:40 and supervision restarted the steward at
07:41:14, one minute later, on a checkout without this chain landed.

Most of what he asks is already the design's spine and this chain has
built it: the fence record is never deleted once written, and every
path that could resurrect anything reads it and refuses while it is
closed. What his words add is one gap and one proof, and that is this
round.

# What the orchestrator's bed run showed (round 7 tree, outside the sandbox)

status-is-live PASSED. stop-everything, seat-survives, stop-fence and
arm-again all failed for ONE mechanical reason in the fixture, not in the
product: each waits for a steward runner publication (or, for stop-fence,
a watcher completion) that arming will never produce in a fixture
repository. `up` reported
`component=steward-runner outcome=excluded detail="standing runner is
excluded by fixture or linked-worktree policy"`, which is deliberate:
`runnerExclusion` in metasystem/internal/steward/runner.go excludes the
standing runner both in a linked worktree and in a fake-runtimes root,
"fixtures arm deliberately". The design's own stop-everything text says
the scenario arms a scratch checkout through `up` AND THEN arms a runner
with `steward arm`. The waits are therefore waiting for the wrong actor.
Where the scenarios did produce output on the round-6 tree, the stop
report matched the section 6 grammar line for line, so no product defect
is implicated.

# Decisions (the orchestrator's; decided, not open)

D33. The shared acceptance setup arms the steward runner EXPLICITLY with
`steward arm` (fixture human authority, as the beds do elsewhere) after
`up`, and waits for the publication that command causes. Every wait in
these scenarios must be for an event the scenario itself causes: name
the command that causes it beside the wait. Apply the same rule to
stop-fence's watcher completion, which times out for the same reason.


D30. The stopped state names WHO stopped it and WHEN, everywhere it is
reported, from the fence record's `by` actor, which the record already
carries:
- `internal/stoptransition/transition.go` line 140, the status fence
  line, becomes `stopped since <changedAt> by <verb> pid <pid>; start
  again: metasystem arm --repo <toplevel>`.
- `cmd/metasystem/process_verbs.go` line 334, the refusal sentence,
  becomes `the metasystem is stopped for <checkout> since <changedAt>,
  by <verb> pid <pid>`.
- The stop verb's own final line at line 306 is unchanged: its caller is
  the actor, so naming it there says nothing new.
Every scenario that compares these lines is updated in the same round.

D31. The durability proof, in the stop-fence scenario: after the stop,
drive the two paths that actually resurrected the runner in the
observed incident, the watcher component's pass with its steward repair
(`cmd/metasystem/supervise_component.go`) and the arming path's
ensure-runner (`internal/up/up.go`), plus a stop-hook invocation the way
the seat-survives scenario fires one, and assert with a process-table
diff on the bed's tag prefix that nothing was created, that the runner
record names no live runner, and that the fence record still reads
closed with its original `by` and `changedAt`. This is the scenario
that answers his incident directly.

D32. Nothing else changes: no new scenario beyond the five of the cut
plus this proof, and no product change that a failing assertion did not
name.

# Verification

Reported at evidence level ran, with the private caches earlier rounds
used: `scripts/agents/go-gate.sh --fast`; `go test -count=1` over every
package in the boundary; `scripts/agents/dispatch-fixtures.sh` and
`scripts/agents/goal-cli-fixtures.sh` fully green; and the five
scenarios named with the exact supervision-bed command, which the
orchestrator runs outside your sandbox.

# Constraints

Wall-clock budget: 120 minutes. This is the final round of this chain
and its budget: what is not green at the end is named precisely in your
return, and the orchestrator parks rather than half-lands. Return per
the implementer schema with the cumulative diff boundary listed. Gap
rule: stop and report a gap; never fill it silently.
