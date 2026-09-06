Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal race-gate-red-on-main, tier 3, hazard DESIGN-BEARING, code critique of chain rgr-build1)
Date: 2026-09-06

# Review brief: race-gate-red-on-main, round one

Round budget: three focused rounds for the goal's tier-3 box; this is
round one. The orchestrator adjudicates every finding; you edit nothing.

Threat model: one seat and its delegates on one machine, no adversaries.
In scope: wrong behavior on the stop hook's verdict path (the goal
Store's turn verdict and the session-stop verbs), a concurrency defect
that survives the change, a weakened or bypassed test or gate, a
register entry that hides a real refusal, an unrelated change, and a
gate ceiling that no longer bounds a hang. Out of scope: hostile inputs,
the speed of the goal and mission-runner suites themselves, and the
steward test the build brief names as out of scope.

Scope: the computed diff of implementer job rgr-build1 (round one)
against its base, commit 414c9dc4. The brief it implements is
metasystem/plans/race-gate-red-on-main-brief.md, landed at that commit;
it binds. The computed diff is
metasystem/artifacts/agents/rgr-build1/rounds/1/diff.patch and its
reviewed tree is d910ae5b41f8ea20f4c1b79e02a9f41f46618f10; carry that
hash into your return exactly.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

1. The Store copy in TurnVerdict, WriteSessionStop and EndSessionStop in
   metasystem/internal/goal/turnverdict.go and
   metasystem/internal/goal/sessionstop.go. The brief demands that no
   verb writes its receiver's Root after construction. Check every
   caller of the three verbs (the command surface in
   metasystem/cmd/metasystem/goal.go and any package-internal caller)
   for one that reads the receiver's Root after the call and expected
   the resolved value. Check that the copy carries every field the verb
   body uses (Root, Prober, Now) and that nothing in the body still
   reaches the original receiver.
2. The hook capture in fetchProjectionWithinDeadline in
   metasystem/internal/goal/project.go: the goroutine must read no
   package-level variable. Check the timeout read and the error message
   as well.
3. The refusal register exclusion in
   metasystem/internal/refusal/register.go: exact pattern, exact reason,
   placed after AUTHENTICATED_CHANNEL_WORD; the test in
   metasystem/internal/refusal/register_test.go must pass with the test
   file unchanged. Confirm the token is an admission, not a refusal, by
   reading its use in metasystem/internal/humanauthority/authority.go.
4. The gate ceiling in metasystem/scripts/agents/go-gate.sh: only the
   timeout value on the race run line changed, the comment above it is
   in the file's own voice with no goal names, dates, machine names or
   round references, and no other line moved. One fact in that comment
   is already known to be wrong and is yours to register: it says no
   individual test exceeds 12 seconds, a figure the build brief took
   from the goal package alone; the mission-runner package's slowest
   tests take 19 seconds standalone under the race detector. Register
   it as a finding with that evidence so it folds with the rest.
5. Conformance: every changed path is inside the brief's May-touch list;
   no test file changed; nothing under plans changed.

# Evidence you may run

From the reviewed worktree root (the metasystem directory):

- `go test -count=1 ./internal/refusal`
- `go test -race -count=1 -run 'TestFreshLedgerFailureAndFetchTimeoutBlockTheStop|TestWatchdogProtocol' ./internal/goal`
- `go vet ./internal/goal ./internal/refusal`
- `gofmt -l ./internal/goal ./internal/refusal`
- `bash -n ./scripts/agents/go-gate.sh`
- `git diff --stat` between the base and the reviewed tree

The full race gate is not yours to run; the orchestrator runs it.

# Expected Return

The code-critic role's version-3 JSON with every required property.
Findings only, each with a stable id, severity, a boolean material
value, a claim a stranger can understand, and evidence marked ran, read
or inferred. Every material finding carries exactly one rigor row. A
round with zero material findings is a closure candidate; say so
plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
