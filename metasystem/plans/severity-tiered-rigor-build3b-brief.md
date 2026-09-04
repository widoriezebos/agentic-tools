Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-04

# Goal

Finish PART THREE of the tiering machinery on a fresh chain. The
previous chain (str-build3) built it (reviewed tree 71f3ac42, review
in metasystem/records/misc/severity-tiered-rigor-build3-critique-cc1.md)
but its correction round cannot be dispatched: the finding register
collided on a generic finding id and the chain is stuck (backlog item
finding-register-id-collision-across-chains). Its tree is preserved on
the branch `preserve/str-build3-r2` (already in this repository; 14
files against main). Take that work, apply the correction, gate, return.

# Workspace

The delegate worktree the dispatcher created for this job. First
command, from the repository root:

    git cherry-pick --no-commit preserve/str-build3-r2

(its base is main's tip as it was at faa137e0; no conflict is
expected; a conflict is a gap). Never check the branch out over main.

# The correction to apply on that tree

Exactly metasystem/plans/severity-tiered-rigor-build3-fix-brief.md:
the two defects (the goal file's Tier compared at the landing base;
a dedicated `landing.receipt-bound-min` bound for the receipt command,
measured once) and the note F-6 (the two narrower fixtures). The
contract of the part is metasystem/plans/severity-tiered-rigor-build3-brief.md
with its gap answer metasystem/plans/severity-tiered-rigor-build3-gap-brief.md.

# Gate

As in the build3 brief, plus the fixtures the correction adds.
Declare the boundary as every file that differs from main.

# Constraints

Wall-clock budget: 60 minutes; return before it ends even if something
is red, naming it. DESIGN-BEARING reach. Gap rule: stop and report a
gap with your proposed contract written out.
