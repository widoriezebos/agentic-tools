Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal claude-critic-sandbox-allows-loopback, tier 2, hazard MECHANICAL, live proof after the landing)
Date: 2026-09-06

# Review brief: the loopback proof, chain lpb-build1 as landed

This is the goal's live proof, not a review round: you are a claude
critic dispatched by the landed engine, whose settings set
sandbox.network.allowLocalBinding. Scope: the computed diff of
implementer job lpb-build1 against its base, the change described in
metasystem/plans/claude-critic-sandbox-allows-loopback-brief.md.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to do, in this order

1. Run `go test -count=1 ./internal/adapter` from the worktree root
   (the metasystem directory) and record the exact result as evidence
   level ran; all tests must pass now, including the thirteen that
   open a loopback listener.
2. Run `python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname())'`
   and record the result.
3. Run `curl -sS -m 10 -o /dev/null -w '%{http_code}' https://example.com/`
   and record whether egress is still allowed (it should be: this
   change touches loopback only).
4. Say in your gaps whether you had a shell.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding.

# Gap Rule

stop and report a gap; never fill it silently.
