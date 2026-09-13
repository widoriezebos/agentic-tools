Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal defect-analysis-gate)
Date: 2026-09-13

# Goal

Author the design document defect-analysis-gate-design.md, a NEW file you
create in the metasystem plans directory (the directory that holds
delivery-efficiency-plan.md), for goal defect-analysis-gate (the goal
record metasystem/plans/goals/defect-analysis-gate.md carries the full
intent and the next-step text; read both). Wido's word (the paper's
section metasystem/docs/paper/12-learning-systems.md, "How a bug becomes
a fix"): the analysis of a defect must EARN the fix item. DONE: a
defect-fix goal cannot open, or a defect-fix dispatch cannot admit, or
both, unless a CHALLENGED analysis is attached, fail-closed at a real
boundary; the analysis record keeps observed apart from suspected and
names a cause; the challenge scales with the four risk questions
(severity, novelty, exposure, accumulation: low risk means the
reproduction plus one causal test, vary or remove the suspected cause and
watch the failure follow; severe, unfamiliar or wide-reaching means a
fresh cross-family critic attacks the diagnosis, reruns the reproduction,
hunts other causes and tests over- and under-coverage); the surviving
analysis attaches to the goal as evidence with its ruled-out causes; the
reproduction bar IS two-bars' Defect-Proof (red on the old tree, green on
the new), joined, never duplicated; a refusal names what is missing.

# Workspace

Your job worktree (the dispatcher names it), branch agent/<job>. Create
exactly one file, defect-analysis-gate-design.md, in the metasystem plans
directory. Touch nothing else. Do not commit.

# Inputs

- The goal record metasystem/plans/goals/defect-analysis-gate.md: intent,
  constraints, freedoms, roster, and the expected arc of three members
  (the analysis record and its reproduction join; the risk-scaled
  challenge gate; the intake or admission enforcement).
- The reproduction bar: metasystem/plans/two-bars-for-changes-design.md
  and metasystem/records/two-bars/two-bars-design.md (Defect-Proof); the
  goal intent forbids reinventing it.
- The boundaries: goal intake in metasystem/cmd/metasystem/goal.go
  (runGoalOpen) and metasystem/internal/goal/verbs.go (Open); dispatch
  admission in metasystem/internal/dispatch/admission.go
  (EvaluateGoalAdmission, EvaluateGoalRevisionAdmissionForDispatch).
- The challenge join candidate: metasystem/cmd/metasystem/validate_verbs.go
  (validate critique-closed) and the finding register in
  metasystem/internal/dispatch/finding_register.go (how a critic round's
  findings are folded and closed).
- The four risk questions: the Risk tuple in
  metasystem/internal/goal/file.go and how the tier derives from it.
- The one-mechanism rule of 2026-09-10: an umbrella record for context
  plus members with their own DONE; the page names the members.

# Constraints

- One design page in plain English (short sentences, no bullet padding),
  at most 240 lines, with these sections: 1. What a defect-fix item is
  today and where one can enter (every path a defect-fix goal or dispatch
  takes, with file:line evidence, and what evidence each carries now);
  2. The mechanism (the analysis record: where it lives, its schema with
  observed, suspected, cause, ruled-out causes, the reproduction join to
  Defect-Proof by identity; the challenge: how the low-risk causal test is
  recorded and proven, how the severe path's cross-family critic is
  dispatched and its findings joined through the register; the gate:
  which boundary refuses, what the refusal names, how an item without a
  challenged analysis is turned away without prose); 3. What does not
  change (critique-always, design-gate-at-dispatch, commit-goal-binding
  stay separate); 4. Risks (a gate that blocks legitimate non-defect
  goals; an analysis faked by prose; a reproduction that passes on both
  trees; the cross-family critic unavailable); 5. Fixtures (Go tests and
  bed legs by name, with setup and assertion, for each DONE clause); 6.
  Landing (the three members with their own DONE and fixtures, in
  landing order, no member's DONE needing a later one).
- Every path you cite must exist in the worktree under the metasystem/
  prefix. No globs. Do not write code. Do not run bin/metasystem test run,
  test plan or test verify.
- Wall clock: 40 minutes. Stop and report if the page is not finished by
  then.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json):
jobId, round, runtime, sessionId, model, evidence, gaps, mode, riskiestPart,
diffBoundary, whatWasDone. evidence carries exactly two `{command, observed,
level}` items, replayable verbatim from the worktree's repository root: (1)
`git -C metasystem status --short` observing the one new file; (2)
`( cd metasystem/plans && wc -l defect-analysis-gate-design.md )` observing
the line count. whatWasDone names the boundary the page chooses and the
record the analysis lives in.

Every path in your return (diffBoundary, files) is relative to the
repository root, so it starts with `metasystem/`.

# Acceptance Criteria

- The one new file exists with the six sections and every cited path
  exists; section 2 decides the record, the challenge and the gate so an
  implementer builds without guessing; section 6 names three members
  with their own DONE.

# Gap Rule

stop and report a gap; never fill it silently.
