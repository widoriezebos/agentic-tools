Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Build slice 2a, the risk basis: round four, the closing review's findings

The closing review (job str-p2-build-2a-cc1, tree 279d0cad) returned
five material findings and four notes; its return.json under that
chain's round 1 is the authority for their text. The design answers
them in revision 4.4 of metasystem/plans/severity-tiered-rigor-p2-design.md;
build that revision on the tree you inherit, in this order.

1. STR2P2A-01 (critical): the claim's accounting revision. Add
   `accountingRevision` to the claim record (render, parse, round-trip;
   absent reads as the claim revision); `goal claim` sets it to the claim
   revision, `set-budget` moves it to the new revision, the raise keeps
   it; `ProjectBudget` (metasystem/internal/dispatch/budget.go) counts
   records with `goalRevision` in `[accountingRevision, Claimed.Revision]`
   and leaves the above-claim BUDGET_UNKNOWN rule as it is. Extend
   TestSTR4R1RaiseTransaction: two roots dispatched before a raise still
   count in attempts, reserved minutes and active jobs after it; a
   set-budget by the human resets the tally.
2. STR2P2A-09: the raise lifts `reviewRoundLimit` to the new tier's box
   member when lower, no other member, no BudgetExceptions increment;
   one assertion in the same test.
3. STR2P2A-03: `goal edit` refuses a bare `--tier` with "answer the four
   questions", requires `--why` for an override above the derivation and
   writes the TierOverride history line; test through the command layer.
4. STR2P2A-08: a raise with an override writes both lines; a raise
   without `--tier` on a goal set above its derivation keeps the set
   tier when it is at or above the new derivation; tests for both.
5. STR2P2A-05: no history index at claim revision zero; the file with an
   Obligation line and a zero claim revision produces the existing
   problem line, never a panic; test with that file.
6. STR2P2A-02: the exception counter compares the elapsed limit as a
   parsed duration; test with a one-day limit against the eight-hour box.
7. STR2P2A-04: STR4-R1-MISCLASSIFICATION-KIND discharged by a test that
   drives `goal edit`'s raise through the command layer and reads the
   counselor register line of kind misclassification.
8. STR2P2A-06 and -07 (notes): run the confirm in
   TestSTR4R1SweepBackfill and assert the kept tier from written state;
   drop the mark/enforce words from the NIL-RISK-DIGEST fixture's name
   or make them true, whichever is smaller.

# Gate and constraints

gofmt, go vet, go build, staticcheck silent (go-gate.sh --fast); every
test you add or touch by name plus TestSTR4R1*, TestRisk*,
TestGoalReviewRoundLimitUsesTupleAndGoalFreeCeiling,
TestSTR3GapDischargeSelectVerb and TestSeverityTieredRigor*; the
dispatch, config, landing and steward packages in full. Stage nothing,
no commit wrapper, nothing under plans, records or memory. diffBoundary:
the 32 files of round three plus tests you add.

Wall-clock budget: 40 minutes; return by minute 35 whatever the state,
fast gate green, unbuilt items listed by number. Version-2 implementer
JSON with the finding-to-test table under `evidence`.

# Gap Rule

stop and report a gap; never fill it silently. The grain is the build
brief's: only a law-changing contract that neither revision 4.4 nor the
earlier briefs answer stops you; a mechanical choice is chosen as the
tree does it nearest the seam, recorded under `decisions`, and built.
