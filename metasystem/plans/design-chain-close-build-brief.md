Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal design-chain-has-no-lawful-close)
Date: 2026-09-06

# Goal

Goal design-chain-has-no-lawful-close (tier 2, approved by Wido on
2026-09-06). Its record,
metasystem/plans/goals/design-chain-has-no-lawful-close.md, is the
contract. In short: a design round is an implementer job at
DESIGN-BEARING reach whose only changed path is one design file under
plans, its critique is a design-critic chain, and the close verb then
refuses the design chain because chain completion wants an
independent-critique reference and the reconcile door only accepts
code-critic, warden and verifier evidence. DONE: a design chain whose
design-critic register is closed can be closed by the same close verb
with the design-critic root as its independent-critique reference, and
the fixture proves it.

# Facts (read and ran on 2026-09-06)

- metasystem/internal/dispatch/review_reference.go: reviewReferenceBinding
  binds code-critic and warden to the independent-critique field and
  verifier to the live-proof field, refuses every other role that carries
  a reviews value, and requires reviews to be a valid job id.
  ReconcileReviewReference requires completed evidence, the binding,
  requireFoldedCritique for critic evidence, a terminal reviewed chain,
  and reviews equal to the chain's final work round, then
  stampReviewReference writes the pointer on the reviewed root.
- metasystem/internal/dispatch/hazard.go, validateIndependentCritiqueReference,
  already accepts a design-critic as the reference role. It also requires:
  a fresh-chain critic (no parent, not resumed), a completed record,
  reviews equal to the final work round, a distinct session, the maximal
  effort tier and reasoning effort, a maximal model (codex always counts),
  and a critic that ended at or after the final work round.
- metasystem/scripts/agents/dispatch.sh: `close --job <root>
  --reconcile-evidence <job>` runs the reconcile before close-check;
  design-critic dispatch validates `--design` against the reviewed
  workspace and records `--outputs` as declaredOutputs.
- A real design-critic record (hook-root-critique4b-20260905 on machine
  m1's checkout, read) carries role design-critic, dispatchMode fresh,
  declaredOutputs (one record path under records), declaredOutputsDigest,
  findingRegisterRound 2, reviews null, and NO design path. The design
  rounds it examined (implementer-ea7bd720076c2ba8ab502ecc,
  hook-root-fold5-20260905) carry reviews null, no critique reference,
  and their return's diffBoundary is exactly one plans path (the record
  itself has no diffBoundary). close-check on all three refuses with
  REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE.
- Timing on that ladder: implementer-ea7bd7... ended 21:32, the critic
  ended 21:51, fold5 ended 22:03 (all 2026-09-05 UTC).

# Decisions (the orchestrator's; decided, not open)

D1. reviewReferenceBinding accepts role design-critic for the
independent-critique field. A design-critic carries no reviews value at
dispatch, so StampClaimedReviewReference stamps nothing for it (return
nil, today's behavior), and ReconcileReviewReference derives the
reviewed job for a design-critic as the named chain's final work round
and uses that as the binding for the rest of the reconcile.

D2. Preconditions for a design-critic reference, checked in
ReconcileReviewReference before any write: the critic's register is
folded through its terminal round (requireFoldedCritique, kept) AND
closed, meaning no register finding has status open or disputed (the
same rule close-check applies to a critic chain); a violation refuses
naming the finding id.

D3. Pairing. The dispatcher records the design path on a design-critic
record at dispatch under the key `design` (the repo-root-relative path
it already validates for `--design`); find where declaredOutputs is set
on the record and set `design` beside it. At reconcile, when the critic
record carries `design`, the final work round's return (rounds/<n>/return.json
of that job, field diffBoundary) must be exactly that one path,
otherwise refuse naming both; a critic record without `design` (dispatched
before this change) is accepted on the caller's explicit pairing.

D4. After the checks pass, the reconcile stamps the pointer on the
reviewed root (stampReviewReference, unchanged) and writes
`reviews: <final work round job id>` onto the design-critic ROOT record
under its record lock, so the closure law's existing coverage check
(critic reviews equals the final work round) holds with no design-critic
special case in hazard.go.

D5. The closure law is not relaxed: a critic that ended before the
final work round still refuses at close. A fold made after the last
critique is unexamined work; it stays open until a critique round
examines it. State this in the reconcile's refusal when the pairing is
otherwise valid but the critic predates the final round (the message
names both times).

D6. Non-goals: no change to hazard.go; no change to the code-critic,
warden or verifier bindings; no change to the design-critic dispatch
flags; no change to critique-register verbs; nothing under plans.

D7. The pin, in metasystem/internal/dispatch/review_reference_test.go
using the package's existing record helpers: a design chain (implementer
root, round 1, DESIGN-BEARING, completed, with a rounds/1/return.json
whose diffBoundary is one plans path) and a design-critic root (fresh,
completed, ended later, declaredOutputs, `design` equal to that path,
findingRegisterRound 1, a register with every finding resolved, a
distinct session, xhigh with maximal configurationObligations, runtime
codex): ReconcileReviewReference succeeds, the pointer and the reviews
value are stamped, and CloseCheck on the implementer root passes.
Negative cases, each asserting the refusal text: an open register
finding; `design` not equal to the diffBoundary; a critic that ended
before the work round (reconcile refuses per D5); a critic record without
`design` is accepted. If metasystem/scripts/agents/dispatch-fixtures.sh
has a design-critic dispatch scenario, extend it to assert the recorded
`design`; if it does not, say so.

# Gate

From the metasystem directory: `gofmt -l .` prints nothing; `go vet
./...`; `go build ./...`; `go test ./internal/dispatch/ -count=1`;
`bash -n scripts/agents/dispatch.sh`. Report each with its evidence
level.

# Constraints

Wall-clock budget: 45 minutes. DESIGN-BEARING reach (a closure law);
a code critic reviews the tree next. Declare the boundary as every file
that differs from main. Gap rule: stop and report a gap with your
proposed contract written out; never fill it silently.
