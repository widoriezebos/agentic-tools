Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal human-carried-landing)
Date: 2026-09-04

# Review brief: closing review of the refusal audit (hcl-audit-build-2, round 1)

Round budget: 1 focused round; this closes slice 1. R-60-m1's rule:
material only if it changes what gets built and names the artifact.

Threat model: an audit that looks complete and is not. Two trusted
seats, no adversaries. In scope: a token the walk should collect and
does not (a weaker grammar than the design's), a row whose
classification contradicts its site, an exclusion that hides a real
refusal, a test that passes by construction rather than by checking
the tree, a table shape that the slice-2 tests and the docs cannot
cite as the design names it. Out of scope: prose, naming taste, the
order of rows, and the design itself (revision 2.1 is the contract;
findings against point 03's rules go to the seat as notes, not as
material).

Scope: the computed diff of implementer job hcl-audit-build-2 round 1
(its diff.patch under that chain's round-1 directory; reviewed tree
3e5ba537f5399773075ae788d5eb0d43f5404bf8; two new files, metasystem/internal/refusal/register.go and
metasystem/internal/refusal/register_test.go, against main at
bd50c0ca). The design is
metasystem/plans/human-carried-landing-design.md revision 2.1, point
HCL-AUDIT-03 (lines 88-149) and point 01; the build brief is
metasystem/plans/human-carried-landing-slice1-audit-brief.md, which
fixed every classification. The round-1 return beside the diff lists
the implementer's `decisions`.

# Mandate

1. The walk is the design's walk: go/ast over the eight directories,
   test files skipped, tokens found INSIDE literals for the
   UPPER_SNAKE pattern, whole-literal match for the hyphen pattern,
   and in internal/landing the first argument of `wouldRefuse`, the
   `code` of `carriageError` literals, and the `case` literals of
   `knownRefusalCode`. Run the test and read the collected set: a
   token you can find by hand (grep the eight directories for the two
   patterns and for `wouldRefuse("`) that the walk does not collect is
   material. The seat's own count was 154 at main 4ad38918; the implementer collected 171 and excluded READY_FOR_RUNTIME as a design-obligation status; judge that exclusion and the eleven landing codes it rowed beyond rule 3 (promotion-record-malformed, promotion-base-unreadable, nine tier-1 codes).
2. Every row's classification matches its site: read at least the
   humanauthority rows (identity), GOAL_SPLIT_REFUSED (agent, no
   override, a Defects entry), the two landing question rows
   (malformed-candidate-tree, candidate-tree-unreadable), HAZARD_REFUSED
   and RISK_UNANSWERED (Commands 3, in Slow), and five landing rows at
   random (pending human-carried-landing, Override `land.sh --carried`).
   A row whose Shape or Override contradicts what the site does is
   material.
3. Every exclusion is what its reason says: for the census and custody
   hyphen codes, confirm none is returned to a caller as a refusal
   (the brief made the implementer verify; check three). An exclusion
   hiding a refusal is material.
4. The three tests check the tree, not the table against itself:
   EveryCodeRowed must fail if a row is deleted (delete one locally,
   run, restore); EveryRowReal must fail on an Override naming a verb
   main.go does not have (edit one locally, run, restore);
   PendingRowsNamed must fail on a pending row with the wrong goal
   name. Report the three observations; a test that cannot fail is
   material.
5. The exported shape is exactly the brief's table section (types
   Shape, Row, Exclusion, Shell, Defect; variables Rows, Exclusions,
   ShellRows, Defects, Slow). A missing or renamed export is material.
6. Nothing outside the two files changed.

Finding identifiers: this repository's register refuses an id another
chain already carries. Name findings HCL-A-01, HCL-A-02, ... and never
F-n.

# Constraints

Wall-clock budget: 20 minutes. Return per the code-critic schema with
the reviewedTree above. Run `go test ./internal/refusal/` and the
three local mutations; do not run the full suite.

# Gap Rule

stop and report a gap; never fill it silently.
