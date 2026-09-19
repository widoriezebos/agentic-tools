Working Mode: <working mode>
Mission Stream: <active mission stream; omit this line when no mission is active>
Orchestrator Identity: <identity>
Date: <YYYY-MM-DD>
Changed-line allocation: <the design's estimate for this unit, additions plus deletions including tests; an estimate, not a cap>

# Goal

<State the observable outcome.>

# Workspace

<State the path, branch, what may be touched, and what must not be touched.>

The build cache is provided: the adapter sets GOCACHE, GOTMPDIR and
STATICCHECK_CACHE to the chain's cache in the worktree's git dir, shared by every
round; never set, unset or strip them (no env -u, no env -i before the go gate): a
self-set cache is cold every round. The proof of your round is made by the
orchestrator's engine on the worktree as you leave it (metasystem job
prove-round); run the focused tests and the fast gate for what you change,
and leave everything you return in the worktree, uncommitted.

Leave `metasystem/memory/receipts.log` unchanged; the seat writes the receipt at landing.

# Inputs

<Name the design page and its revision, the section or sections this unit builds, and the files the delegate may rely on. The design page is the specification: this brief adds workspace, inputs, return shape and proof group, and restates no rule of the page. If the page needs rulings to be buildable, fold the page first.>

# Constraints

<State non-goals, wall-clock and enforceable token budgets, and the critic round budget when applicable.>

Set the changed-line allocation under `metasystem/docs/design/design-principles.md` before briefing an implementation unit.

Estimate the complete candidate against the allocation before drafting,
including its tests, and report the count at the end. The allocation is an
estimate, not a cap: overrunning it is not a reason to stop. If the unit does
not hold together as one coherent change, return a split proposal (a gap)
instead of trimming required tests.

Request the smallest section you need, read a large file through a bounded view, and open a reference only through its path.

No single command may wait longer than 240 seconds; run a longer one in the background with its output to a file and poll the file. A wait that outlives the turn is registered with `metasystem wait register`.

# Expected Return

<Name every required property from the role's return schema. Each evidence `command` is one command replayable verbatim from the declared workspace. The orchestrator may rerun commands individually and compare world-state observations; returned commands are never executed as a batch. Keep the settled `{command, observed, level}` evidence schema unchanged.>

Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

# Acceptance Criteria

<List observable, machine-checkable acceptance criteria.>

# Gap Rule

stop and report a gap; never fill it silently.
