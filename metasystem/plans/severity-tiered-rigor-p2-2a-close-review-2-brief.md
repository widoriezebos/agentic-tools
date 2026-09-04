Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Review brief: second closing review of the tiering machinery, slice 2a, the risk basis (str-p2-build-2a, round 6)

Round budget: 1 focused round; this closes the slice. R-60-m1's rule:
material only if it changes what gets built and names the artifact.

Subject: the computed diff of implementer job str-p2-build-2a round 6
(its diff.patch under that chain's round-6 directory; reviewed tree
cee8c2ea22451ee7fcc9f88f52989ba3cf6ca099, 37 files against main at
c10fa035; the diff is the authority). The first closing review (job
str-p2-build-2a-cc1, reviewed tree 279d0cad) returned five material
findings STR2P2A-01 to -05 and four notes -06 to -09; its return.json
under that chain's round 1 is the authority for their text. The design
answered them in revision 4.4 of
metasystem/plans/severity-tiered-rigor-p2-design.md ("the raise keeps
the spend, and its edges"); round five built the eight items of
metasystem/plans/severity-tiered-rigor-p2-build-2a-r4-brief.md and
round six the three sweep tests of
metasystem/plans/severity-tiered-rigor-p2-build-2a-r6-brief.md. The
return.json of rounds five and six (beside their diffs) list what was
built and their `decisions`; read both.

# Mandate

1. Every finding of the first review is answered in the tree, by the
   design's answer and not another: STR2P2A-01 by `accountingRevision`
   on the claim record (claim sets it, human set-budget moves it, the
   raise keeps it; `ProjectBudget` counts job records, obligation
   states and governed runs with goalRevision in the inclusive interval
   `[accountingRevision, Claimed.Revision]`; a legacy claim line without
   the field reads it as the claim revision); STR2P2A-02 by the elapsed
   comparison as a parsed working duration (`1d` equals the eight-hour
   box, `1d2h` is over it); STR2P2A-03 by `goal edit` refusing a bare
   `--tier` ("answer the four questions"), requiring `--why` for an
   override above the derivation and writing the TierOverride history
   line; STR2P2A-04 by a misclassification test that goes through the
   raise and reads the counselor register line it wrote, not the writer
   called directly; STR2P2A-05 by the parser never indexing history at a
   zero claim revision (a problem, never a panic). A finding answered by
   a different mechanism than revision 4.4 names, or answered only in a
   test, is material; cite the finding id it re-opens.
2. The notes: STR2P2A-06 (sweep confirm proves the incumbent tier in
   written state), -07 (nil-risk fixture named for what it proves), -08
   (a raise without `--tier` keeps a set tier at or above the new
   derivation; raise plus override writes both Misclassified and
   TierOverride lines), -09 (a raise lifts `reviewRoundLimit` to the new
   tier's box member when lower, never lowers it, no exception counted
   for that lift). Report only if the tree contradicts the note's
   answer.
3. The first review's nine mandates still hold on this tree: no
   shape-derived tier anywhere; the raise rewrites only
   `Claimed.Revision` and the stop capability's `Generation` and
   `Revision` (now also keeping `accountingRevision`), never
   `ClaimEpoch`, `StopFence`, `Obligation`; `refusal:<code>` admits
   exactly `AdmissionRefusalCodes`; the risk gate's `mark` and `enforce`;
   `gateWidth: full` under accumulation 2 and the landing's refusal
   without the full battery receipt; `BudgetExceptions` counts every
   over-box member and two over-box operations end the appetite line
   with `repeated exception: defect signal`; the review-round seam reads
   the goal's tuple member capped at `config.ReviewRoundMax`; the sweep
   row `<goal-id> <s>,<n>,<e>,<a> <basis>` with the tool rendering the
   tier. A regression of any of these since tree 279d0cad is material.
4. The sweep tests of round six (TestSTR3MigrationBootstrap01...,
   TestClassifySweepInstallsTierLawForAnAlreadyTieredLedger,
   TestClassifySweepRecoverySkipsRowsAlreadyApplied) assert written Risk
   state, incumbent tier retention, TierLaw installation and recovery
   idempotence; a test that was made green by weakening its assertion
   rather than by the row grammar is material.
5. Nothing outside the build briefs' diffBoundary changed: no plans,
   records, memory, or files the build briefs do not name. The
   `decisions` in the two returns (ProjectBudget assertions living in
   the dispatch package to avoid an import cycle; two clone-sensitive
   tests moved to the local-sync fixture bed) are recorded choices, not
   findings, unless one hides a lost assertion.

Known and out of scope, do not report: the `dispatch` scenario of
dispatch-fixtures.sh, red on main since the alias landing 2c3776b8;
the host's temporary-repository failures ("could not parse HEAD",
bootstrap ledgers in the steward beds) that rounds three and five
reported as environment gaps; the goal package's full coverage number
(the seat's run, recorded at landing); the docs re-touch
(docs/orchestration.md and the two critique skills), which the seat
writes and lands with this chain.

Finding identifiers: this repository's register refuses an id another
chain already carries. Name any new finding STR2P2A-10, STR2P2A-11, ...
and never F-n; a re-opened first-review finding keeps its own id.

# Constraints

Wall-clock budget: 25 minutes. Return per the code-critic schema with
the reviewedTree above. The diff is the subject; do not run the full
goal or dispatch packages (17 minutes and the host failures); run tests
by name only if a finding needs it.

# Gap Rule

stop and report a gap; never fill it silently.
