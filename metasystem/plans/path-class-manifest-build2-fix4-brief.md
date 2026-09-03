Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Correction round on chain path-class-build2 (your reviewed tree
3f53e22e). The code review found one material defect, PCM-CC8-001, and
three notes. Fold the defect; the notes are settled below.

# The defect

The evaluator classifies every changed path with Manifest.Class, which
resolves only the install namespace and answers unclassified for a key
without a row; it can never answer outside. So the outside branches
you added are dead, and in the root layout (the installation is the
repository root, empty git prefix) every application file an agent
commit touches is classified unclassified and, now that
path-unclassified is promoted, hard-refused. The design's section 1
says an adopted installation's application paths answer outside and
keep today's rules; section 3's outside row says pass when certified,
register-carriage-path-refused as an observed code under carriage,
pass when an exact inverse.

# The change

1. metasystem/internal/landing/observe.go: resolve changed paths with
   mode and application ownership the way the waiver rule does in
   metasystem/internal/validate/conformance.go (ResolveRepositoryPath
   with the ownership oracle of metasystem/internal/stateroot/owner.go),
   so adopted application paths answer outside and follow the outside
   row; the vendored layout's answers do not change.
2. metasystem/internal/landing/observe_test.go: add the adopted-mode
   outside legs: exact revert of an application file passes (this makes
   the exact-inverse comparison reachable, the leg PCM-CC2-005 deferred
   to this part); a chain landing that changes an application file
   passes when certified; a carriage landing that changes one refuses
   register-carriage-path-refused.
3. Nothing else changes. Declare the boundary as the files you touch,
   with the metasystem/ prefix.

# The notes, settled

- PCM-CC8-002: the fixture hygiene (unsetting the git object-store
  variables in land-fixtures.sh and static-reproof-fixtures.sh) is
  accepted knowingly and stays.
- PCM-CC8-003: the design's section 5 row is amended by the
  orchestrator to the PCM-R2-002 rule your implementation follows.
- PCM-CC8-004: attached short options in the wrapper's input scan stay
  as they are; the postcondition holds; recorded as a residual.

# Gate

`go test ./internal/landing/ ./internal/stateroot/ ./internal/validate/ -count=1`
green where the sandbox allows (the orchestrator replays the rest,
KI-15); `bash scripts/agents/land-fixtures.sh` and
`bash scripts/agents/path-class-fixtures.sh` green; `gofmt -l` empty;
`go vet` clean.

# Constraints

Wall-clock budget: 40 minutes. DESIGN-BEARING reach. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the files you changed.

# Gap Rule

stop and report a gap; never fill it silently.
