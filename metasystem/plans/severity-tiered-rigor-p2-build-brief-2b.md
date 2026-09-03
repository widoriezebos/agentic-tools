Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-03

# Goal

Build slice 2b of the tiering machinery's part two, the material stop
and the close, exactly as designed in
metasystem/plans/severity-tiered-rigor-design.md: revision 2's
mechanism points 4, 5 and 7 (lines 165-225) as amended by revision 3's
sections STR2-ROUND-ACCOUNTING-05, STR2-ARTIFACT-MEMBERSHIP-07,
STR2-DEMOTION-TRANSITION-08, STR2-CLOSE-STATE-MACHINE-09,
STR2-CLOSE-PERSISTENCE-10 and STR2-CRITIC-UNION-11 (lines 345-438),
plus one item from metasystem/plans/severity-tiered-rigor-p2-design.md
revision 4.1 (finding 008). Read the parent design's revision 2 and 3
first, then the p2 design. Where revision 3 amends revision 2, revision
3 wins. Wido's order, verbatim: "I want this done in 16 hours MAX";
this slice is one of three, so the wall clock below is the law.

Part one of the design (the Tier field, the five-member budget tuple
with ReviewRoundLimit, the classify sweep) is being built on ANOTHER
seat and is NOT in your base. Do not build any of it. Where this slice
reads part one, use the one seam named in item 2 below.

# What to build (the design is the spec; this is the index)

1. The artifact member (point 4, 07): the critic return schema gains
   `artifact` on the rigor row, three canonical forms `<path>`,
   `NEW <path>`, `<old>=><new>`, pattern-bound in
   metasystem/internal/returnschema/returnschema.go (follow the tree's
   generation mechanism and bump the version as the tree does), parsed
   by `ParseArtifactRef` in metasystem/internal/critique/model.go.
   Membership is decided at fold in
   metasystem/internal/dispatch/finding_register.go against the root's
   subject set exactly as 07 says (code chains: the changed-path set of
   the reviewed implementer round's diff.patch, refusing the fold with
   "run conformance --stage review first" when absent; design chains:
   `declaredOutputs` from a new required `--outputs <file>` on
   design-critic dispatch in metasystem/scripts/agents/dispatch.sh,
   written on the root by build.go). Critic rows of
   metasystem/scripts/agents/role-packets.json and
   scripts/agents/adapters/runtime-common.sh spell the member.
2. The counter and the frozen boundary (05): critic roots gain
   `reviewRoundLimit` and `criticRoundsConsumed`; `CritiqueRegisterAdvance`
   increments in its completed and failed branches only;
   `readCritiqueCapState` (metasystem/internal/dispatch/critique.go:133)
   replaces the three constants, `boundedCritiqueStart` and
   `recordFirstAllBoundedRound` with the one rule of 05; the rebind verb
   `job critique-budget-rebind --root-job` copies the limit onto the
   root with the goal revision and opid. SEAM: build.go reads the limit
   through one function `goalReviewRoundLimit(repoRoot, goalID string,
   revision uint64) (uint8, error)` in metasystem/internal/dispatch,
   which at your base returns 3 (R-42-m0's ceiling) with the comment
   `// part one's ReviewRoundLimit tuple member replaces this body`.
   Nothing else in this slice may reach for part one.
3. Demotion (08): `foldCritiqueFindings` gains the `demoted` outcome
   ahead of the `!material` branch, with the per-round `demotions` list
   on the root; only an explicit `material:false` resolves an entry.
4. The close table (09): statuses `deferred` and `accepted-risk`,
   `resolution` on every non-open entry, `CritiqueRegisterClose` in
   finding_register.go taking one branch with one write under the
   register lock, exactly the four rows of the table; `CloseCheck`
   (metasystem/internal/dispatch/close.go:15) counts open and disputed
   as unresolved; critiqueclosed.go writes `out-of-scope` and refuses it
   for a severe or unproven entry, as do CloseCheck and conformance.
5. The obligation model and the commit order (10): `ReviewObligations`
   on `GoalFile` (metasystem/internal/goal/file.go, one line each,
   untouched by `clearClaimBinding`); `DeferFindings`,
   `DischargeReviewObligation --finding --by`, `Done` refusing while any
   is open, `AcceptedRiskDecision` under `humanauthority` proof as
   `SetBudgetApproved` does (metasystem/internal/goal/verbs.go); NEW
   metasystem/internal/counselor/register.go with
   `AppendAcceptedRisk` writing one strict-schema line (sources.go:48-77)
   and, from the p2 design's finding 008, `AppendMisclassification`
   (kind `misclassification`, id `mc-<goal>-<opid>`, facts = the two
   derivations and the evidence) writing to
   records/counselor/misclassification-register.jsonl; the strict reader
   in sources.go (line 168) admits the new kind and `loadAcceptedRiskRegister`
   opens both files. Nothing calls `AppendMisclassification` yet; slice
   2a wires it. Commit order (1) ledger, (2) register append skipped if
   the id exists, (3) root write, each keyed and skipped on rerun.
6. The union and the close verb (11): `mergeCritique`
   (metasystem/internal/validate/conformance.go:919) collects every
   code-critic root whose reviewed tree equals the final tree and
   requires all to pass, the union of registers holding no open or
   disputed entry; the close verb is `job critique-register-close
   --root-job <root>` in metasystem/cmd/metasystem/dispatch_verbs.go
   (registered in main.go), no `--goal`.
7. Recurrence (point 5, last paragraph): a later finding with the same
   artifact and facts digest as a deferred one is classed unproven by
   the normalizer's existing recurrence rule.

# Fixture obligations (each is a named test that exists and passes)

Every fixture named in 05, 07, 08, 09, 10 and 11 and in revision 2's
part-two list (line 251: material without artifact demoted; artifact
outside the tree demoted; NEW path accepted; chain closes at zero
material; boundary at three rounds; raise by rebind; close with bounded
findings writes obligations; close with a severe finding refuses;
accepted risk recorded; conclude refuses with obligations; recurrence
classes unproven), plus:

- STR2B-SEAM-CONSTANT: `goalReviewRoundLimit` returns 3 at this base and
  the root carries `reviewRoundLimit: 3`.
- STR4-R1-MISCLASSIFICATION-KIND: the strict reader opens both register
  files and admits kind `misclassification`; a malformed line of the
  new kind is excluded with the existing limitation text.
- STR2B-RENAME-EITHER-SIDE: `<old>=><new>` accepted when either side is
  in the subject set.
- STR2B-CLOSE-ONE-WRITE: the exhausted-all-bounded row writes deferred
  entries once; a rerun after a simulated crash between steps 1 and 2
  of the commit order completes without a duplicate.

# Gate

gofmt, go vet, go build; `GOFLAGS=-buildvcs=false go run
honnef.co/go/tools/cmd/staticcheck@2025.1 ./...` silent
(metasystem/scripts/agents/go-gate.sh `--fast` is the same check and
the commit gate refuses a chain on any line); go test -count=1 ./...
green; metasystem/scripts/agents/return-schema-fixtures.sh,
dispatch-fixtures.sh and conformance-fixtures.sh each run once in your
sandbox (report the exact refusal if the sandbox cannot run one). No
benchmarks (R-31), no sleeps (R-35). Leave the work in your working
tree, stage nothing, do not run the commit wrapper. diffBoundary:
metasystem/internal/returnschema, metasystem/internal/critique,
metasystem/internal/dispatch, metasystem/internal/goal,
metasystem/internal/counselor, metasystem/internal/validate,
metasystem/cmd/metasystem, metasystem/scripts/agents/dispatch.sh,
metasystem/scripts/agents/adapters/runtime-common.sh,
metasystem/scripts/agents/role-packets.json, and the fixture scripts
named above. Nothing under metasystem/plans, metasystem/records or
metasystem/memory; nothing that part one owns (Tier, the tuple, the
sweep, norm.go, config/budget.go). Paste the final gate lines and the
list of new test names in your return.

# Constraints

Wall-clock budget: 45 minutes. Build items 1, 2, 3 first, then 4, 5,
6, 7. If the clock runs out, leave the tree consistent with the gate
green and list in the return exactly which items and fixtures are
unbuilt; a follow-up finishes them. Version-2 implementer JSON. If a
design cite has drifted against the tree at your base, follow the tree
and name the drift in the return; do not redesign.

# Gap Rule

stop and report a gap; never fill it silently.
