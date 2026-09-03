Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Correction round on chain path-class-build1c (your reviewed tree
f0620d30). The closing review found that your round-three change made
the waiver rule and the path class verb disagree in the root layout
(PCM-CC5-001): the waiver now reads the manifest before ownership,
the shared resolver of the pathclass package (new in this chain) still
reads ownership first, and with the shipped manifest (docs/ is
behavior) an adopted application's own docs/application.md is
refused by one and answered outside by the other. The orchestrator
withdraws the round-three ruling and rules again, at the root cause.

# The ruling

Ownership decides first, in every consumer, exactly as the certified
design and obligation PCM-R2-001 say. The defect of PCM-CC4-001 was
never the order: it was that the shipped inventory in
metasystem/internal/stateroot/owner.go (shippedInventoryPath) does not
cover everything adoption installs. metasystem/scripts/adopt.sh creates
docs/project-rules.md (line 159) and the skeleton trees memory/,
plans/ and records/ with their READMEs, registers and the two goal
ledgers (lines 251 to 298). Those are the installation's in the root
layout. Once the inventory names them, ownership answers metasystem
for them, the manifest row decides their class (behavior, ledger,
record), the waiver refuses docs/project-rules.md and the ledgers in
every layout, and an application file with no inventory entry stays
outside and waivable in the verb and the waiver alike.

# The change

1. metasystem/internal/validate/conformance.go: restore the waiver
   classification of your round-two tree (ownership before the
   manifest; drop the empty-prefix manifest-first branch of round
   three).
2. metasystem/internal/stateroot/owner.go: extend shippedInventoryPath
   with the file docs/project-rules.md and the directories memory/,
   plans/ and records/. Keep the comment's promise: the inventory
   grows in lockstep with adopt.sh's install set.
3. metasystem/internal/stateroot/owner_test.go: extend
   TestOwnerUsesShippedInventoryInUnvendoredAdoptedShape with legs for
   docs/project-rules.md, memory/README.md, plans/goals/x.md,
   records/goals/x.md (metasystem) and docs/application.md, README.md
   (application).
4. metasystem/internal/validate/conformance_test.go: keep the five
   root-layout legs of round three, whose expectations hold under the
   corrected inventory, and rename the test to say what it asserts
   (PCM-CC5-002); its synthetic manifest may stay.
5. No other product byte changes; the doc comment on
   ResolveRepositoryPath is true again and stays. Declare the boundary
   as the four files, with the metasystem/ prefix.

# Gate

`go build ./...` clean; `go vet` and `gofmt -l` on internal/validate
and internal/stateroot; `go test ./internal/validate/ ./internal/stateroot/ ./internal/pathclass/ -count=1`
green where the sandbox allows (the orchestrator replays the
worktree-creating tests outside the sandbox, KI-15);
`bash scripts/agents/path-class-fixtures.sh` green.

# Constraints

Wall-clock budget: 25 minutes. DESIGN-BEARING reach. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the four files.

# Gap Rule

stop and report a gap; never fill it silently.
