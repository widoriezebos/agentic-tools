Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Second and last correction round on slice 1 (chain path-class-build1,
your reviewed tree 71d5f0ea): fold the five material findings of the
code review metasystem/records/misc/path-class-manifest-code-critique-r1.md.
Two of them would turn the landing gate red on its first full run, so
the chain cannot land without this round. Same design (revision 2), same
slice; the boundary grows by exactly the two coverage baselines named
below.

# The five findings and what each requires

- PCM-CC1-001: TestRepositoryManifestClassifiesEveryTrackedPath runs git
  ls-files against the installation's parent and fails on any git error;
  the Go gate runs package tests inside a frozen export without .git.
  Make the test skip cleanly (with a named reason) when the parent is not
  a git work tree, or enumerate from a recorded listing the gate can
  read; the gate must be green in the export.
- PCM-CC1-002: the new package internal/pathclass has no coverage floor;
  the ratchet refuses a measured package without a floor or exemption.
  Add a floor for internal/pathclass to
  scripts/agents/coverage-ratchet.json and coverage-ratchet-linux.json at
  the measured coverage (state the number in your return); both files
  join the diff boundary.
- PCM-CC1-003: the waiver rule in internal/validate/conformance.go
  classifies only in the installation namespace, so a changed path
  outside the installation (a template record such as
  development/evidence-index.md, or any adopted application file) reads
  as unclassified and the prose waiver refuses it where it was waivable
  before. Resolve by location as the design's section 3 says: repository
  rows in template mode, outside otherwise, and the waiver rule refuses
  only behavior or unclassified within the installation; add the test.
- PCM-CC1-004: ResolvePath accepts the ownership oracle's "outside" before
  reading the error that comes with it, so a misplaced engine binary or an
  unreadable repository top prints "outside" with exit 1 and no
  diagnostic. Check the error first: a discovery failure returns the
  error (and the verb prints it); "outside" only when discovery
  succeeded. Test both.
- PCM-CC1-005: exactRevertFloorError lets an unclassified path through
  to the exact-inverse checks, where the old floor caught nested
  instruction paths by suffix and directory name (product/AGENTS.md,
  product/scripts/x). Slice 1 promised no verdict widens: an
  unclassified path under exact revert refuses exactly as a behavior
  path does; add the nested cases to the revert test.

The four low, non-material notes (symlinked working directory; the dead
skip idiom around os.IsNotExist on a wrapped error; the empty rg search
set outside the template layout; the replaced NEWRT test case) are yours
to fix only if each is a one-line change inside the boundary; otherwise
leave them and say so.

# Gate

`go test ./internal/pathclass/ ./internal/landing/ ./internal/validate/ ./cmd/metasystem/ -count=1` green; `bash scripts/agents/path-class-fixtures.sh` green; `gofmt -l` empty; `go vet` clean on the touched packages; and, because the first two findings are about the gate itself, run `bash scripts/agents/go-gate.sh` if the sandbox allows it and report its result honestly (the orchestrator replays it outside the sandbox either way, KI-15).

# Constraints

Wall-clock budget: 50 minutes. DESIGN-BEARING reach (correction at high effort). R-31: no benchmarks. Slice 2's files stay untouched.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the files you changed,
including the two coverage baselines.

# Gap Rule

stop and report a gap; never fill it silently.
