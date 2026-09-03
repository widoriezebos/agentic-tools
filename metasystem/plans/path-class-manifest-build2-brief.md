Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Build SLICE 2 of metasystem/plans/path-class-manifest-design.md (revision
2, landed; section 8 names the slice and its boundary; sections 3, 5, 6
and 7 are the contract). Slice 1 is landed on main (the manifest, the
path class verb, the resolver, conformance, the deletions); build on it.
The design was reviewed by Sol twice; the closing register
metasystem/records/misc/path-class-manifest-critique-r2.md names three
test obligations for this slice, listed below, which bind you. Zero
judgment calls beyond what the design decided; where the design is
silent, stop and report the gap.

# Workspace

The delegate worktree the dispatcher created for this job.

# The change (slice 2: class table, exact revert, ownership, wrapper inputs, promotion, end to end)

1. internal/landing/observe.go and observe_test.go: the class-by-
   declaration table of section 3 (behavior takes a chain only; record
   under register carriage with the section 5 semantics; ledger refuses
   in any wrapper landing; runtime refuses; unclassified refuses with the
   base-manifest detail in the Observation); exact revert of a record
   refuses with its own code and the revert test covers all five classes;
   record ownership binds the base goal file's claimed machine and
   lineage to the wrapper actor, with the goal-item-not-held code, the
   longest-identifier tie-break, frozen legacy files, and the
   append-only check for the three registers.
2. internal/landing/promotion.go, scripts/agents/landing-promotion.json
   and cmd/metasystem/landing_verbs.go: the complete fail-closed code set
   of section 6, promoted exactly as section 6 states for this slice.
3. scripts/agents/commit.sh and scripts/agents/land.sh: the --goal input
   carried from land.sh through commit.sh; machine resolution before the
   evaluator call; Goal-Item validated against the ledger and
   single-valued, refused when already present in -m, -F or --trailer
   input; the unclassified-path detail printed from the Observation.
4. scripts/agents/land-fixtures.sh, scripts/agents/path-class-fixtures.sh
   and scripts/agents/static-reproof-fixtures.sh: the end-to-end
   land-to-evaluator fixtures of section 7 for this slice.
5. memory/rulings.md: the one promotion row section 6 names, appended as
   the last row, byte for byte as the design writes it.

# Obligations from the closing review

- PCM-R2-002: the existing-record-append leg of TestObserveRecordSemantics
  in internal/landing/observe_test.go creates records/misc/fx-analysis.md
  at the base, appends bytes under a goal held by the actor and asserts
  pass, while replacement or deletion asserts
  register-carriage-not-append-only and a missing owner asserts
  record-not-owned; landing-promotion.json promotes record-not-owned only
  with that leg green.
- PCM-R2-003: TestCommitWrapperStampsGoalItemTrailer in
  scripts/agents/path-class-fixtures.sh proves a lowercase goal-item input
  is refused, a commit-msg hook that injects or changes Goal-Item causes a
  soft rollback with HEAD unchanged, and a successful commit contains
  exactly one byte-exact Goal-Item value; commit.sh implements that
  final-message postcondition.
- PCM-R2-005: TestObserveFloorPrecedesGoalOwnershipValidation in
  observe_test.go combines a behavior path with a foreign Goal and asserts
  direct-fix-floor-refused, and a record-only change with the same foreign
  Goal asserts goal-item-not-held; observe.go follows that ordering.

# Gate

`go build ./...` clean; `go vet ./internal/landing/ ./cmd/metasystem/`; `gofmt -l` empty on the touched packages; `go test ./internal/landing/ ./cmd/metasystem/ -count=1` green; `bash scripts/agents/path-class-fixtures.sh`, `bash scripts/agents/land-fixtures.sh` and `bash scripts/agents/static-reproof-fixtures.sh` green. Run only the touched areas' fast tests; the full suite is the orchestrator's (KI-15).

# Constraints

Wall-clock budget: 110 minutes. DESIGN-BEARING reach. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the files named in The
change.

# Gap Rule

stop and report a gap; never fill it silently.

# Residuals from the first part's code reviews (bind this slice where named)

- PCM-CC7-002: the two comments in internal/stateroot/owner.go that name
  goal adoption-inventory-from-install-set violate the source-comment
  rule (no provenance in code); reword them to state the constraint in
  the system's own terms. One-line changes; do them in this slice.
- PCM-CC7-001: the design's resolution paragraph says "the six named
  docs files" without naming them; the orchestrator fixes the design
  text, not you.
- PCM-CC6-003 (directory queries answer application-owned in the root
  layout) and PCM-CC6-001 (inventory short of adoption's install set)
  belong to goal adoption-inventory-from-install-set; do not touch them.
- PCM-CC2 low notes (tracked-path walk ignoring mode; unreachable
  exact-inverse legs; misinstalled diagnostic when the manifest is
  absent): fold each only if it is a one-line change in a file this
  slice already touches; otherwise leave it recorded.
