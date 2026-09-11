Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, coordinator under goal human-carried-landing-carry, fifth and closing read of the carried landing, chain hcl-build3-20260911, final work round 5)
Date: 2026-09-11

# Review brief: the closing read of the carried-landing implementation

FINDING IDS: chain-unique, continue the carry's register: HCL-C-79, HCL-C-80, ... never F-n.

## What you are reading

Chain hcl-build3-20260911 for goal human-carried-landing-carry
(metasystem/plans/goals/human-carried-landing-carry.md), final work round
4 (job hcl-build3-20260911-r5), reviewed tree 70c7e1ba03828f34f3ae3ae8dcbaad29d0c806b3 (the
installation subtree the conformance review names as reviewedTree). The
specification is metasystem/plans/human-carried-landing-carry-design.md
revision 6 with the coordinator's three amendments (the stop list's
empty-staging rule excludes the rerun states; single-machine mode out of
scope with `carry-remote-required`; `goal carry` speaks after its
transaction). The fourth read (`metasystem/artifacts/agents/hcl-context/code-read-r4.md`,
Opus, hcl-cc4-20260911) found four material findings; the fold
(`metasystem/artifacts/agents/hcl-context/build3-fold-r4-brief.md`)
decided each, plus one host fact: HCL-C-74 the crash-local row patterns
count rendered rows; HCL-C-75 the crash-local leg moves origin before the
rerun and asserts the pushed commit's parent, author date, trailers and
the crashed sha gone; HCL-C-76 the counselor writer removes its lock on
release and `records/counselor/*.lock` is ignored; HCL-C-77 accept-risk
trims `--why` and refuses a blank one, with a round-trip test; the
prefixed fixture's contract paths carry the `metasystem/` prefix. Round 4
built those five and nothing else. The coordinator ran every bed on the
host.

Write your register as this new file: `metasystem/records/misc/human-carried-landing-carry-code-read-r5.md`
(a read-only runtime returns it in the job return; the coordinator
projects it).

This is the closing read of a DESTRUCTIVE-REACH chain. Read the diff of
round 5 against round 3 (the five items and the leg) and the decisions
side by side. A material finding is: a fold decision not carried out as decided;
a defect the five changes introduce; a way a carry lands what the human
did not name or refuses a verified human on the machine's own judgement,
in the changed code; a fixture among the five that does not test what it
says. Page fixtures the earlier folds recorded as follow-ups are not
findings. Do not re-read what the earlier reads settled.

## Mandate

1. The five decisions, one line each: folded as decided at file:line, or
   not, with what differs.
2. HCL-C-75's leg: with origin moved, the checks cannot pass on a recovery
   that resets and restamps; say which check catches it.
3. HCL-C-76: the lock's acquire and release paths for both registers; a
   crashed writer's stale lock does not block the next writer or the next
   landing.
4. What must not change: the identity gate, never carrying a record
   failure, the temporary word not a proof, the shared testing contract
   and the workspace receipt not weaker, the ordinary digest rule, trunk's
   changes intact.

## Return

Findings by id with file and line, the claim, the evidence, and what
changes if it stands; the five lines; then a verdict line: closable as
is, or one fold naming which findings. Wall-clock budget: 30 minutes.
