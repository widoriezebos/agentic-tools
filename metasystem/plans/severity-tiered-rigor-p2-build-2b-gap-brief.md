Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-03

# Goal

Answer the three gaps your round 1 stopped on (STR3-DESIGN-OUTPUTS-
CONTRACT-03, STR3-DEFERRED-OBLIGATION-WIRE-04, STR3-ACCEPTED-RISK-
TRANSITION-05, recorded in metasystem/plans/severity-tiered-rigor-build1-brief.md
lines 76-78) and build slice 2b per
metasystem/plans/severity-tiered-rigor-p2-build-brief-2b.md with the
contracts below. The three answers are binding amendments of revision 3
sections 07, 10 and 09/10 of metasystem/plans/severity-tiered-rigor-design.md;
they are recorded as revision 4.2 of the p2 design. Nothing else
changes; the wall clock of the 2b brief restarts with this round.

# Gap 03: the declared-outputs file

Byte grammar: one repository-relative path per line, `metasystem/`
prefixed, forward slashes, no `./` or `..` segments, trimmed, LF
terminated, no blank lines, no comments, unique, sorted ascending by
bytes. Parser owner: `ParseDeclaredOutputs(path string) ([]string,
error)` in metasystem/internal/dispatch/build.go, refusing any
violation with the line number. The root records `declaredOutputs`
(the list), `declaredOutputsDigest` (sha256 of the canonical bytes) and
`declaredOutputsSource` = `<design path>@<git blob sha of that file at
the reviewed commit>`, taken from a second required flag `--design
<file>` on design-critic dispatch. Provenance: the list is the
dispatching seat's assertion, bound to the design blob it was written
from; a mechanical proof that the list equals the design's build list
is NOT buildable, because the build list has no grammar, and this
slice does not invent one. The design critic's brief carries the
digest, and the critic's review of the design is the check. Fixture:
STR3-GAP03-OUTPUTS-GRAMMAR, an unsorted, a duplicated and a
`..`-bearing file each refused with the line number; a canonical file
accepted with its digest on the root.

# Gap 04: the obligation wire

`- ReviewObligation: finding=<id> chain=<root> artifact=<q> test=<q>
state=open|discharged` where `<q>` is a Go-quoted string
(`strconv.Quote` on write, `strconv.Unquote` on read, so spaces, `=>`
and quotes survive); `finding` and `chain` are bare identifiers (the
register's id grammar, no whitespace). Inputs at close: for each
deferred entry, `artifact` is the entry's artifact reference verbatim
and `test` is `prove: ` followed by the entry's title on one line; the
discharge verb replaces `test` with the citation it is given
(`--test "<citation>"`, required, Go-quoted the same way). Fixture:
STR3-GAP04-OBLIGATION-ROUNDTRIP, a NEW artifact with spaces and a test
containing `=` and `"` written and parsed back identical; a line whose
quoted value does not unquote is a parse refusal naming the line.

# Gap 05: the accepted-risk transition

Command surface: `goal accept-risk --id <goal> --finding <id> --chain
<root> --by <human> --why "<text>"` in metasystem/internal/goal/verbs.go
(`AcceptedRiskDecision`) and metasystem/cmd/metasystem/goalsync_mutations.go,
under `humanauthority` proof exactly as `SetBudgetApproved`. Selection
contract: the finding must exist in the root's register with status
open or disputed and rigor class severe or unproven; bounded findings
are refused with "bounded findings defer at close, not by acceptance";
the chain must be bound to the goal. Transition, in the commit order of
10: (1) the goal line `AcceptedRisk: finding=<id> chain=<root>
by=<human> opid=<opid>`; (2) `counselor.AppendAcceptedRisk`, skipped if
the id exists; (3) the register entry becomes status `accepted-risk`,
resolution `accepted-risk`, `decisionOpid=<opid>`. The close table of
09 reads with one rule above its first row: entries with status
`accepted-risk` are resolved and are not members of U. So the sequence
is: close refuses and prints the severe entry; the coordinator seat
(the holder of the goal claim) obtains the human's word; the human's
word runs the verb; close reruns and takes the ordinary branch.
Fixture: STR3-GAP05-ACCEPT-THEN-CLOSE, a severe entry refuses close,
the verb as the pair is refused, the verb as the human writes the three
steps, close then succeeds; a bounded finding offered to the verb is
refused.

# Everything else

The 2b brief's items, fixtures, gate, diffBoundary and constraints are
unchanged; add the three fixtures above to its list. The
`goalReviewRoundLimit` seam stands. Stop and report only a gap that is
not answered here or in the two designs.
