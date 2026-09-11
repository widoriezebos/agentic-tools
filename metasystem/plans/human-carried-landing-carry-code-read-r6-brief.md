Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, coordinator under goal human-carried-landing-carry, sixth and closing read of the carried landing, chain hcl-build3-20260911, final work round 6)
Date: 2026-09-11

# Review brief: the closing read of the carried-landing implementation

FINDING IDS: chain-unique, continue the carry's register: HCL-C-83, HCL-C-84, ... never F-n.

## What you are reading

Chain hcl-build3-20260911 for goal human-carried-landing-carry
(metasystem/plans/goals/human-carried-landing-carry.md), final work round
4 (job hcl-build3-20260911-r6), reviewed tree bcaa8063840666045a054b365e38b77df550d8e0 (the
installation subtree the conformance review names as reviewedTree). The
specification is metasystem/plans/human-carried-landing-carry-design.md
revision 6 with the coordinator's three amendments (the stop list's
empty-staging rule excludes the rerun states; single-machine mode out of
scope with `carry-remote-required`; `goal carry` speaks after its
transaction). The fifth read (`metasystem/artifacts/agents/hcl-context/code-read-r5.md`,
Opus, hcl-cc5-20260911) found two material findings, both low; the fold
(`metasystem/artifacts/agents/hcl-context/build3-fold-r6-brief.md`)
decided each: HCL-C-79 the counselor lock is released without a race
(inode check after locking, the path deleted while still held) with a
sixteen-writer test `TestHCL79RegisterLockSurvivesRelease`; HCL-C-80 a
trailing-space `--why` round-trips through `goal accept-risk` on chain
`human-carried` into the carried admission, `TestHCL80TrailingWhitespaceWhyIsAdmitted`.
Round 6 built those two and nothing else. Rounds 1 to 5 were read five
times; every earlier finding is folded or recorded. The coordinator ran
every bed on the host.

Write your register as this new file: `metasystem/records/misc/human-carried-landing-carry-code-read-r6.md`
(a read-only runtime returns it in the job return; the coordinator
projects it).

This is the closing read of a DESTRUCTIVE-REACH chain. Read the diff of
round 6 against round 5 (the two items) and the two decisions side by
side. A material finding is: a fold decision not carried out as decided;
a defect the two changes introduce; a way a carry lands what the human
did not name or refuses a verified human on the machine's own judgement,
in the changed code; a fixture among the two that does not test what it
says. Page fixtures the earlier folds recorded as follow-ups are not
findings. Do not re-read what the earlier reads settled.

## Mandate

1. The two decisions, one line each: folded as decided at file:line, or
   not, with what differs.
2. HCL-C-79's lock: trace two writers and a third arriving during
   release; the inode check and the delete-while-held close the race the
   fifth read ran; the test would fail on round 5's code.
3. What must not change: the identity gate, never carrying a record
   failure, the temporary word not a proof, the shared testing contract
   and the workspace receipt not weaker, the ordinary digest rule, trunk's
   changes intact.

## Return

Findings by id with file and line, the claim, the evidence, and what
changes if it stands; the two lines; then a verdict line: closable as
is, or one fold naming which findings. Wall-clock budget: 25 minutes.
