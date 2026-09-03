Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Last correction round on chain path-class-build1c (your reviewed tree
4e766443). The closing review found that the shipped inventory in
metasystem/internal/stateroot/owner.go still names fewer files than
adoption installs (PCM-CC6-001: twenty-eight docs files and the runtime
registration directories answer application-owned in the root layout).
The orchestrator rules, at the review budget of this chain: that
shortfall predates this feature, no fleet machine runs the root layout,
and making the inventory equal adoption's install set is its own
backlog item, goal adoption-inventory-from-install-set, which will read
the installed set instead of extending a hand list. The design's
resolution paragraph is narrowed to the exact list the inventory
carries. What remains for this chain is the prose that over-claims.

# The change

1. metasystem/internal/stateroot/owner.go: the two comments that
   claim the inventory is in lockstep with adoption's install set and
   is the source of truth the adoption tracer proves against (the
   paragraph above the root-layout branch, and the doc comment on
   shippedInventoryPath) say instead what is true: the inventory names
   the instruction-bearing files and the trees adoption creates;
   completeness against adoption's full install set is tracked by goal
   adoption-inventory-from-install-set.
2. metasystem/internal/validate/conformance_test.go: rename the
   docs/application.md leg of the root-layout waiver test to credit
   ownership, not the absent manifest row (PCM-CC6-002).
3. No other byte changes. Declare the boundary as the two files, with
   the metasystem/ prefix.

# Gate

`gofmt -l` empty and `go vet` clean on internal/stateroot and
internal/validate; `go test ./internal/stateroot/ -count=1` green;
the orchestrator replays internal/validate outside the sandbox (KI-15).

# Constraints

Wall-clock budget: 15 minutes. DESIGN-BEARING reach. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the two files.

# Gap Rule

stop and report a gap; never fill it silently.
