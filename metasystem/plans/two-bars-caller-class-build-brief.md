Working Mode: implement
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal two-bars-for-changes)
Date: 2026-09-02

# Goal

The caller-class slice of two-bars-for-changes, implemented exactly per
the ACCEPTED design metasystem/plans/two-bars-caller-class-design.md
(revision 3; critic chain two-bars-cc-crit-3, {{CLOSURE}}; dispositions
in metasystem/plans/two-bars-caller-class-dispositions.md and
metasystem/plans/two-bars-caller-class-dispositions-r2.md{{R3DISP}}).
The commit wrapper branches on a typed engine verdict from one new
lease verb, `lease commit-authority`, never on whether a claim epoch
happened to be present: a person keeps the sovereign path, an announced
holding main takes the gated path with its epoch, a delegate committing
inside its own RUNNING job's worktree takes the worker path, every other
caller is refused with the one fixed sentence. The Machine trailer's
suffix comes from the verdict and the wrapper owns its three trailers.
A second verb, `lease job-worktree`, lets the landing driver refuse
inside a job worktree.

# Workspace

The dispatch-created job worktree, branched from main. The design's
sections bind exactly as written — ruling R-25b-m1: the design is
carried whole, and any deviation, simplification, or scope cut you find
necessary is a GAP to report, never a silent choice.

Expected touched paths (declare every touched path in diffBoundary WITH
the metasystem/ prefix): metasystem/internal/lease/verbs.go (both
verbs, the shared MAIN-branch helper, the worker rule, the pure
`jobWorktreeGeometry` function and its thin probe wrapper, design
section 3), a new metasystem/internal/lease/commit_authority_test.go
(section 7.4), metasystem/internal/lease/verbs_test.go (wire-shape
cases), metasystem/cmd/metasystem/lease.go and
metasystem/cmd/metasystem/main.go (CLI wiring for both verbs),
metasystem/scripts/agents/commit.sh (both halves, the ordered inner-half
checks including the trailer monopoly and the post-commit count,
sections 2 and 6), metasystem/scripts/agents/land.sh (the geometry
refusal at its start, section 3, nothing else),
metasystem/scripts/agents/static-reproof-fixtures.sh (the stub contract
7.1 and the legs 7.2), metasystem/scripts/agents/land-fixtures.sh (the
`job-worktree` scenario, 7.5), metasystem/internal/behaviorsurface/
consumer_wiring_test.go (7.3). Nothing in the non-goals of section 8
moves: landing-promotion.json, the never-direct-fix floor, the carriage
allowlist, gateHolder, ClassifyAt, HolderView's wire shape, the
pre-commit guard, evidence-gc.sh, dispatch.sh, the adapters.

# What binds (by design section)

- §2: the branch table row by row including the worker row; the
  refusal sentence byte-for-byte; exit code 2 for every wrapper-side
  refusal; the inner half's five ordered checks (path match, lineage,
  push rule, trailer monopoly with its readable-source rule and
  `git interpret-trailers --parse`, then token mint), all before the
  token and before any proof; the post-commit count and soft rollback.
- §3: both verbs' names, flags, structs in alphabetical wire order and
  per-class semantics; the worker rule's five ordered steps with
  `J.status == "running"` exactly and fail-closed on any unreadable or
  ambiguous record; the shared helper boundary; the three error
  strings unchanged; every proof still running on the worker path;
  land.sh's exact refusal block and placement.
- §4: require-holder and run-held byte-identical; no other caller's
  contract moves.
- §6: the Machine trailer suffix from the verdict's lineage; the
  wrapper no longer reads METASYSTEM_OWNER_LINEAGE; the monopoly's
  fleet consequence is stated in the landing receipt, not in code.
- §7: the stub contract exactly as 7.1 states; every leg in 7.2 by name
  with its exit code, stderr bytes and HEAD assertion (including the
  prose-mention positive control, the three hand-typed-trailer runs,
  the unreadable-source run, and the hook-injection rollback); the
  consumer-wiring stub of 7.3; the Go tests of 7.4 by name (the
  geometry table test, the verb test on a real temp worktree, the
  worker-in-own-worktree and running-follow-up tests, the
  outside-worktree refusal table with pending, pending-setup, empty and
  unknown statuses); the landing bed's `job-worktree` scenario of 7.5
  with its label change; internal/lease at or above its coverage floors
  (scripts/agents/coverage-ratchet.json and coverage-ratchet-linux.json);
  every existing leg's assertions unchanged.
- §9: the seams are assumptions with stop conditions — if the helper
  extraction changes an error string, if the fixture authority refuses
  a temp worktree as a root, if a real parentJob shape breaks the chain
  walk, or if git's `-m` joining differs from the wrapper's, STOP and
  report the gap.

# Constraints

- KNOWN SANDBOX LIMIT: the full validation suite needs real process
  visibility your sandbox denies; run the focused proofs named below
  and report anything environment-limited as such, never faked.
- No test weakened; gofmt and go vet clean; no new environment variable
  read by commit.sh (the bed's escape scan refuses one).
- Wall-clock budget: 90 minutes.

# Expected Return

Version-2 implementer JSON; complete diffBoundary; evidence commands
replayable from the worktree root including:
- `go build ./...`
- `go test ./internal/lease/ -run 'CommitAuthority|JobWorktree|VerbResultWireShapes' -count=1`
- `go test ./internal/behaviorsurface/ -count=1`
- `bash scripts/agents/static-reproof-fixtures.sh` and
  `bash scripts/agents/land-fixtures.sh` (name the legs that ran and
  their verdicts; if a bed cannot run in the sandbox, say which legs
  and why)
- `gofmt -l` over touched Go files, `go vet ./internal/lease/ ./cmd/metasystem/`

# Gap Rule

stop and report a gap; never fill it silently — especially anywhere
the design underdetermines an implementation choice.
