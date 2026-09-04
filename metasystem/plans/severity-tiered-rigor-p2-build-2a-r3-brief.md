Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Build slice 2a, the risk basis: round three, the fixtures

Round two (job str-p2-build-2a-r2) built all six items of
metasystem/plans/severity-tiered-rigor-p2-build-brief-2a.md with the
answers of metasystem/plans/severity-tiered-rigor-p2-build-2a-gap-brief.md
in force; the fast gate is green on the tree you inherit (28 files
against main). It returned with named fixtures unbuilt and the full
battery unfinished. This round finishes exactly that: tests, and the
production fixes those tests force. Every fixture below is a named test
that exists and passes; the return lists each name beside the fixture
it discharges.

# Fixtures to build, in this order

1. STR4-R1-RAISE-TRANSACTION (the riskiest, first): a raise after claim
   in one transaction; a root dispatched before it keeps its goalTier
   and gateWidth; the next dispatch reads the new tier; the approval
   validation admits the re-bound digest with the Misclassified line
   present and refuses it with the line removed; and, per answer 3 of
   the gap brief, a raise on a breach-stopped goal with a governed
   obligation keeps the fence and the obligation byte-for-byte while
   only the claim's and the stop capability's revision coordinates move.
2. STR4-R1-FOUR-DOWNGRADES-REFUSED: as the pair, lower one score, the
   derived tier, the set tier (an override edited back) and the width:
   four refusals; the same four as the human (`--by`, proof as
   `SetBudgetApproved`) succeed.
3. STR4-R1-FIVE-MEMBER-EXCEPTIONS: an over-box elapsed limit and an
   over-box active-job limit each increment `BudgetExceptions`; two
   over-box operations end the appetite line with `repeated exception:
   defect signal`.
4. The revision-4 fixtures still unbuilt by name (the design's
   "Fixtures of this revision" paragraph): `--tier` alone refused with
   "answer the four questions"; the pair's override above the
   derivation recorded with `--why`; the pair's override below refused
   and the human's override below recorded; lowering after claim
   refused for the pair; mark-mode admission prints the
   `RISK_UNANSWERED` line and proceeds, enforce-mode refuses with the
   same code; accumulation 2 writes `gateWidth: full` on a dispatched
   root.
5. metasystem/scripts/agents/goal-cli-fixtures.sh: one scenario for
   `goal open --risk ... --basis` through the built binary, asserting
   the rendered `- Risk:` line above `- Tier:`; and the
   `dispatch` scenario of dispatch-fixtures.sh is red on main since
   2c3776b8 and stays out of scope.

# The battery

Run, and leave green: `go test -count=1 -timeout 30m $(go list ./... |
grep -v /internal/goal$)`. Round two saw "could not parse HEAD" fatals
in temporary-repository tests on this host; if they recur, run that
package alone once more (`-count=1`) before calling them a regression;
if they hold, they are yours: the tree you inherit is the cause until
proven otherwise. For internal/goal run every test you add or touch by
name, plus `TestSTR3GapDischargeSelectVerb` and the slice-2b
`TestSeverityTieredRigor*` tests, which share the goal file's renderer
with your Risk and BudgetExceptions lines.

# Gate and constraints

gofmt, go vet, go build, staticcheck silent
(metasystem/scripts/agents/go-gate.sh `--fast`); the battery above;
goal-cli-fixtures.sh once. Stage nothing, no commit wrapper, nothing
under plans, records or memory. diffBoundary: round two's 28 files plus
the tests you add and goal-cli-fixtures.sh; nothing outside the build
brief's boundary. Paste the final gate lines and the fixture-to-test
table in your return.

Wall-clock budget: 40 minutes; return by minute 35 whatever the state,
fast gate green, unbuilt fixtures listed by name. A round that ends at
the cap without a return is charged and proves nothing. Version-2
implementer JSON.

# Gap Rule

stop and report a gap; never fill it silently. The grain is the build
brief's: only a law-changing contract that none of the three briefs nor
the design answers stops you; a mechanical choice is chosen as the tree
does it nearest the seam, recorded under `decisions`, and built. A
production fix that a fixture forces (the test proves the built
behaviour wrong against the design) is not a gap: fix it, name it under
`decisions` with the test that forced it.
