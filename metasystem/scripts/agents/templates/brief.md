Working Mode: <working mode>
Mission Stream: <active mission stream; omit this line when no mission is active>
Orchestrator Identity: <identity>
Date: <YYYY-MM-DD>

# Goal

<State the observable outcome.>

# Workspace

<State the path, branch, what may be touched, and what must not be touched.>

The build cache is provided: the adapter sets GOCACHE, GOTMPDIR and
STATICCHECK_CACHE to the chain's cache in the worktree's git dir, shared by every
round; never set, unset or strip them (no env -u, no env -i before a proving
run): a self-set cache is cold every round.

# Inputs

<Name the design, accepted critique round, and files the delegate may rely on. An implementation design leaves no judgment calls.>

# Constraints

<State non-goals, wall-clock and enforceable token budgets, and the critic round budget when applicable.>

# Expected Return

<Name every required property from the role's return schema. Each evidence `command` is one command replayable verbatim from the declared workspace. The orchestrator may rerun commands individually and compare world-state observations; returned commands are never executed as a batch. Keep the settled `{command, observed, level}` evidence schema unchanged.>

Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

# Acceptance Criteria

<List observable, machine-checkable acceptance criteria.>

# Gap Rule

stop and report a gap; never fill it silently.
