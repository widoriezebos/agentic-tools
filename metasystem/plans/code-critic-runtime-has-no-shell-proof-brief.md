Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal code-critic-runtime-has-no-shell, tier 3, hazard DESIGN-BEARING, live-envelope proof after the landing at 07d18614)
Date: 2026-09-06

# Review brief: the shell proof, chain ccs-build1 as landed

This is the goal's final proof, not a new review round: you are the
first claude critic dispatched by the landed engine, and the question
is whether your runtime now gives you a sandboxed shell. Scope: the
computed diff of implementer job ccs-build1-r2 against its base
(reviewed tree b7d5a0f81ae32cb3a2e39e3e370133e13a4222d4), the change
described in metasystem/plans/code-critic-runtime-has-no-shell-brief.md
and metasystem/plans/code-critic-runtime-has-no-shell-fold-r2-brief.md.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to do, in this order

1. Run `go test -count=1 ./internal/adapter` from the worktree root
   (the metasystem directory) and record the exact result as evidence
   level ran.
2. Run `gofmt -l ./internal/adapter` and `bash -n scripts/agents/adapters/claude.sh`,
   evidence level ran.
3. Try `touch internal/adapter/PROBE-CRITIC.txt` and record the exact
   refusal or success as evidence; a success is a material finding.
4. Say in your gaps whether you had a shell, and print the value of
   GOCACHE and TMPDIR your shell saw.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding.

# Gap Rule

stop and report a gap; never fill it silently.
