Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal stop-decisions-record-deadline-evidence)
Date: 2026-09-13

# Goal

You are the design author for member 2 of the stop-hook umbrella. Decide
the eight material findings of the second rostered design read of
metasystem/plans/stop-decisions-record-deadline-evidence-design.md
(commit 5c6d8f83; your worktree is at or after that tip and the page is
a tracked file there), write your decisions into the page, and turn its
fixtures section into the build specification a Codex builder works from.

This ends the prose phase: two design reads are spent, and Wido's word
for the stop-hook members is to stop polishing pages and build per
member. There is no third design read. The Codex build, its tests and
an Opus read of the build check your decisions, so each must be one a
builder can implement and a test can falsify.

The seat has decided none of these findings. They are yours. Before you
decide, read the page, the umbrella page
metasystem/plans/stop-hook-never-forces-an-empty-turn-design.md, and the
code each finding cites. Where a finding is wrong about the code, say so
in one sentence in the page and keep what is right.

# Workspace

Your job worktree, branch agent/<job>. Modify exactly one file, the
design page above. Touch nothing else. Do not commit. Do not write code.

# The findings

The eight material findings of the second read, as the reader wrote them
with the evidence it cited, are in
metasystem/plans/stop-decisions-record-deadline-evidence-design-read2-findings.md:
SDE-01 (critical), SDE-11, SDE-02, SDE-03, SDE-05 and SDE-06 (high),
SDE-07 and SDE-08 (medium). Read that file first.

The reader resolved SDE-04 (the three-valued `recorded` field); keep it.

# What the page must carry when you are done

- A decision for each of SDE-01, SDE-11, SDE-02, SDE-03, SDE-05,
  SDE-06, SDE-07 and SDE-08, in the section it belongs to, in plain
  English. Where a decision removes something the page says today,
  remove it; never leave both versions.
- For SDE-01 and SDE-11 together, one timeline of the Stop's last
  seconds with absolute cutoffs: what the worker may still do, when the
  parent stops waiting, and the latest moment the parent emits. A
  refusal the seat never received must never be recorded as seen, on
  every interleaving, including a worker that holds the record lock when
  the parent's wait ends.
- Where the umbrella page says something different (the longer episode
  key of SDE-02, the incident drain of SDE-08), say which contract this
  member implements and why, in one sentence each. Do not edit the
  umbrella page.
- The landing section names every file the build changes, including
  any the findings show are missing.
- The fixtures section becomes the build specification: for each
  finding at least one named Go test or hook bed leg, the file it lives
  in (an existing file, or a new file named in prose without the
  metasystem/ prefix), and the interleaving or fault it forces. The race
  of SDE-01 and the crash windows of SDE-03 and SDE-07 each get a
  fixture that injects the interleaving deterministically.
- Plain English, short sentences. At most 320 lines.

# Constraints

- Every path you cite with the metasystem/ prefix must exist in the
  worktree. No globs. Do not run bin/metasystem test run, test plan or
  test verify.
- Wall clock: 60 minutes. Stop and report if the page is not finished by
  then.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json):
jobId, round, runtime, sessionId, model, evidence, gaps, mode, riskiestPart,
diffBoundary, whatWasDone. evidence carries exactly two `{command, observed,
level}` items, replayable verbatim from the worktree's repository root: (1)
`git -C metasystem status --short` observing the one modified file; (2)
`( cd metasystem/plans && wc -l stop-decisions-record-deadline-evidence-design.md )`
observing the line count. whatWasDone lists each of the eight findings, the
decision taken and the section it is in, and names the fixture that tests it.
riskiestPart names the decision you are least sure of and why.

Every path in your return (diffBoundary, files) is relative to the
repository root, so it starts with `metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
