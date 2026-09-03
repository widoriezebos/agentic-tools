Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Build SLICE 1 of metasystem/plans/path-class-manifest-design.md (revision
2, landed; section 8 names the slice and its boundary; sections 1, 2, 3,
4, 6 and 7 are the contract for what the slice contains). The design was
reviewed by Sol twice (metasystem/records/misc/path-class-manifest-critique-r1.md
folded in full; the closing register is
metasystem/records/misc/path-class-manifest-critique-r2.md and any test
obligation it names is listed under Obligations below and binds you).
Zero judgment calls beyond what the design decided; where the design is
silent, stop and report the gap.

# Workspace

The delegate worktree the dispatcher created for this job.

# The change (slice 1: manifest, verb, resolver, conformance, deletions)

1. New scripts/agents/path-classes.txt: the manifest with the exact
   grammar and the complete initial table of design section 1 (three row
   kinds: install-relative rows, repository-relative rows read only in
   template mode, ownership rows).
2. New internal/pathclass/pathclass.go and pathclass_test.go: the parser,
   the longest-prefix resolver with the two namespaces, the "outside"
   answer for adopted repositories, the refusal detail with the nearest
   classified ancestor or the sentinel text from section 2, and the
   tests section 7 names for the resolver, the verb output, the sentinel,
   the compatibility rows and template-versus-adopted resolution.
3. cmd/metasystem/path_verbs.go and main.go: the path class verb beside
   path owner (one word for scripts, a sentence for humans, the refusal
   text); cmd/metasystem/path_verbs_test.go.
4. internal/landing/observe.go and observe_test.go: the floor reads
   behavior rows from the manifest instead of the hard-coded table;
   carriage eligibility is "record class and one of the three append-only
   files"; every other verdict keeps today's codes exactly (no verdict
   widens in this slice, per section 8).
5. internal/validate/conformance.go and its tests (conformance_test.go,
   nested_conformance_test.go, instructionowners_test.go): the waiver rule
   reads the manifest ("behavior or unclassified refuses") and conformance
   rejects any runtime-declared instruction file whose manifest class is
   not behavior.
6. Delete scripts/agents/register-carriage-paths.txt and
   scripts/agents/instruction-bearing-paths.txt and every reader of them
   (the nine sites the design lists), updating
   scripts/agents/landing-classes.json, static-reproof-fixtures.sh,
   conformance-fixtures.sh and scripts/validate-metasystem.sh as section
   4 states.
7. New scripts/agents/path-class-fixtures.sh with the shell fixtures of
   section 7 that belong to this slice, including the deleted-reader
   search derived from the manifest's behavior rows, registered in
   scripts/validate-metasystem.sh.

# Obligations from the closing review

Two of the closing review's five findings fall in this slice; each is a
test you must make pass, exactly as named:
- PCM-R2-001 adopted ownership precedes manifest matching: the resolver in
  internal/pathclass/pathclass.go consults application ownership before
  longest-prefix matching; TestAdoptedModeAnswersOutside in
  pathclass_test.go and an adopted leg of TestPathClassVerbAnswersFromManifest
  in scripts/agents/path-class-fixtures.sh place an application-owned
  docs/application.md beneath an install docs/ behavior row and still
  assert outside.
- PCM-R2-004 slice 1 preserves handoff carriage across the slice boundary:
  TestSliceOneRetainsHandoffCarriage in internal/landing/observe_test.go
  asserts that a new plans/handoff-fixture-1.md still passes register
  carriage after the two old list files are deleted and before slice 2's
  ownership logic exists; observe.go retains that current exception in
  this slice.
The other three (PCM-R2-002 existing-record append leg before promotion,
PCM-R2-003 Goal-Item verified on the final commit message, PCM-R2-005
floor-before-ownership precedence) bind slice 2 and are not yours.

# Gate

`go build ./...` clean; `go vet ./internal/pathclass/ ./internal/landing/ ./internal/validate/ ./cmd/metasystem/`; `gofmt -l` empty on the touched packages; `go test ./internal/pathclass/ ./internal/landing/ ./internal/validate/ ./cmd/metasystem/ -count=1` green; `bash scripts/agents/path-class-fixtures.sh` green; the deleted-reader search finds no reader. Run only the touched areas' fast tests; the full suite is the orchestrator's (KI-15).

# Constraints

Wall-clock budget: 110 minutes. DESIGN-BEARING reach. R-31: no benchmarks.
Do not touch slice 2's files beyond what slice 1 names (commit.sh, land.sh,
promotion.go, landing-promotion.json, landing_verbs.go stay as they are).

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the files named in The
change, with the two deletions listed.

# Gap Rule

stop and report a gap; never fill it silently.
