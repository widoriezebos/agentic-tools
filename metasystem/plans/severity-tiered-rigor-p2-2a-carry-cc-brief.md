Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2-2a-carry)
Date: 2026-09-04

# Review brief: closing review of the risk-basis carry (str-p2-2a-carry, round 1)

Round budget: 1 focused round; this closes the slice. R-60-m1's rule:
material only if it changes what gets built and names the artifact.

Subject: the computed diff of implementer job str-p2-2a-carry round 1
(its diff.patch under that chain's round-1 directory; reviewed tree
2397896399069ecc3bb69ccec8389a15c38371ba, 38 files against main at
1e2b1ce9; the diff is the authority). This chain carries the finished
risk basis of chain str-p2-build-2a (six rounds, reviewed twice: job
str-p2-build-2a-cc1 on tree 279d0cad, nine findings STR2P2A-01 to -09,
all answered by design revision 4.4; job str-p2-build-2a-cc2 on tree
cee8c2ea, findings STR2P2A-10 and -11 material, -12 to -14 notes) to
main: round 1 applied str-p2-build-2a's round-6 diff verbatim and built
the two restorations of
metasystem/plans/severity-tiered-rigor-p2-build-2a-r7-brief.md (design
revision 4.5, metasystem/plans/severity-tiered-rigor-p2-design.md). The
returns of str-p2-build-2a rounds 5 and 6 and of this round (beside
their diffs) list what was built and their `decisions`; the cc2 return
is the authority for STR2P2A-10 to -14.

# Mandate

0. The second review's two findings are answered by revision 4.5 and
   not otherwise: STR2P2A-10 by the listing and the confirm skipping a
   draft row whose goal already carries exactly that row's Risk record
   (four scores and basis), counted as applied, digest stable, a
   different record still SWEEP_UNKNOWN_GOAL, and the recovery test
   feeding the draft that still carries the applied row; STR2P2A-11 by
   the command layer appending the misclassification register line
   only when the goal package reports it performed a raise on a goal
   approved before the edit, with a queued goal's first four answers
   writing nothing. A weaker answer, or one only in a test, is material.
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
   `decisions` in the three returns (ProjectBudget assertions living in
   the dispatch package to avoid an import cycle; two clone-sensitive
   tests moved to the local-sync fixture bed; Risk equality as value
   equality over the four scores and basis; applied rows kept in the
   rendered listing so the digest stays stable;
   `PublishResult.RiskRaised` as the package-to-command signal) are
   recorded choices, not findings, unless one hides a lost assertion.

Known and out of scope, do not report: the `dispatch` scenario of
dispatch-fixtures.sh, red on main since the alias landing 2c3776b8;
the host's temporary-repository failures ("could not parse HEAD",
bootstrap ledgers in the steward beds) that rounds three and five
reported as environment gaps; the goal package's full coverage number
(the seat's run, recorded at landing); the docs re-touch
(docs/orchestration.md and the two critique skills), which the seat
writes and lands with this chain.

Finding identifiers: this repository's register refuses an id another
chain already carries. Name any new finding STR2P2A-15, STR2P2A-16, ...
and never F-n; a re-opened first-review finding keeps its own id.

# Constraints

Wall-clock budget: 25 minutes. Return per the code-critic schema with
the reviewedTree above. The diff is the subject; do not run the full
goal or dispatch packages (17 minutes and the host failures); run tests
by name only if a finding needs it.

# Gap Rule

stop and report a gap; never fill it silently.
