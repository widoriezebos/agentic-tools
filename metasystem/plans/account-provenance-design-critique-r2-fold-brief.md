Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal account-provenance)
Date: 2026-09-06

# Design critique, round 2 follow-up: revision 3 folds your two findings

FINDING IDS: chain-unique, continue the series: account-provenance-r2-<slug>; keep the two open ids exactly as you minted them.

Revision 3 of metasystem/plans/account-provenance-design.md is on main as
commit 2c4e2c13; its SHA-256 is
4db8f5d88b39d45d15e94e7c5e6aac85053fcdceb525d4bef962515c6df5da65. It
folds your two round-1 findings by id; the fold table at the end names
each fold, and the changed passages are the Q2 devin row and devin
mapping paragraph, the new Q3 paragraph "Records that never ran a
runtime are outside (b)", the retirement and devin fixtures in Q5, and
self-grade item (e). Your round-1 findings and their dispositions stand
in metasystem/records/misc/account-provenance-critique-r2.md.

# Mandate

1. account-provenance-r2-setup-husks-block-retirement: the evaluator now
   skips every job record whose phase is setup, BuildSetup stays a
   non-writer with the reason stated, and the retirement fixture gains
   the failed-husk and pending-setup cases. Resolved, or does the fold
   move the problem? Rule by id.
2. account-provenance-r2-devin-nonzero-cause: a nonzero devin exit is
   adapter-failed, exit 0 is surface-unmapped, not-logged-in needs a
   recognized logged-out output from the named observation record, and
   a devin floor fixture pins both causes. Resolved, or not? Rule by id.
3. Anything the two folds broke elsewhere in the design, judged by seam
   name as before. Material only if an implementer working from revision
   3 would build the wrong thing and you name the artifact it changes.

Your return's register must state each open id as resolved or still
open; a still-open id needs the sentence the design lacks. If both
resolve and nothing new is material, say so with zero material
findings: the build is dispatched from revision 3.

# Constraints

Wall-clock budget: 25 minutes. Return per the design-critic schema;
append your round-2 ruling under a "Round 2 follow-up" heading in the
record named by the declared outputs manifest.

# Gap Rule

stop and report a gap; never fill it silently.
