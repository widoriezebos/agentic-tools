Working Mode: implement
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Code review of the path-class manifest's second part, landing rules by
class (chain path-class-build2; the round evidence holds diff.patch and
review.json with the reviewed tree). The certified design is
metasystem/plans/path-class-manifest-design.md revision 2: section 3
(the landing consumer's class-by-declaration table), section 5 (record
semantics), section 6 (the fail-closed code set promoted by this part
under R-64-m1 in metasystem/memory/rulings.md), section 7 (fixtures)
and section 8 (the slice boundary). The build brief is
metasystem/plans/path-class-manifest-build2-brief.md with its two
follow-ups (build2-fix and build2-fix2 in the same directory); the
first part is on main (4b1cd47e).

# Mandate

1. The class table: behavior takes a chain only; record lands under
   register carriage with section 5's semantics; ledger refuses in any
   wrapper landing (the goal verbs alone write it); runtime refuses;
   unclassified refuses naming the base manifest. Exact revert of a
   record refuses with its own code. Record ownership binds the base
   goal file's claimed machine and lineage to the wrapper actor with
   the codes and tie-breaks the design states.
2. The three obligations of the closing design review (PCM-R2-002, 003,
   005 in the build brief) are met by the named tests and fixtures.
3. The promotion: the nine codes of section 6 are in
   metasystem/scripts/agents/landing-promotion.json as strings, nothing
   else changed in that file's shape; direct-fix-floor-refused stays
   observed.
4. The wrapper inputs: land.sh carries --goal to commit.sh; Goal-Item
   validated against the ledger, single-valued, refused when already
   present; the unclassified-path detail printed from the Observation.
5. The residual comment edits in metasystem/internal/stateroot/owner.go
   state the constraint without naming a goal.
6. Regression against the first part and the base: the vendored layout
   the fleet runs lands exactly as before for chain landings and
   register carriage; every existing landing fixture stays green.

A finding is material only if it changes what gets built and names the
artifact. If nothing material remains, say so; that closes the chain.

# Constraints

Wall-clock budget: 25 minutes. Return per the code-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
