Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-08

# Fold brief: round 29 of chain stopverb-build1, one declaration

Round 28 is correct and its conformance review refuses on one point:
`scripts/agents/brain-fixtures.sh` changed and no round has declared it.
The change itself is right and stays. The actor-seam-coverage scenario
in that bed pins the exact list of verb-classifier call sites, D119
changed those call sites, so the pinned list had to change with them.
A boundary that omits it would hide a real edit.

This round changes no code. Do exactly this:

1. Confirm the working tree still holds every round-28 change and that
   nothing else is dirty.
2. Return a cumulative `diffBoundary` that names every path the chain
   has changed, including `scripts/agents/brain-fixtures.sh`.
3. If any other changed path is missing from the boundary you would have
   declared, name it too, and say in the return which round introduced
   it.

Do not edit any file. Do not reformat. Do not add or remove a test. If
you believe a change in the tree should not be there, say so in the
return and leave it alone.

# Verification

- `git -C .. diff --check HEAD`, and confirm no plan file is dirty.
- `go build ./...` to prove the tree still builds.

Nothing else needs running: the round-28 tree was already proven by the
orchestrator with all eleven boundary packages, the fast gate, and the
supervision, supervision-hook, dispatch, goal-cli, mission and
suite-progress beds, and this round changes nothing.

Gap rule: stop and report a gap; never fill it silently.
