Working Mode: build
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal turn-verdict-hardening)
Date: 2026-09-02

# Goal

Build SLICE 1a of metasystem/plans/turn-verdict-hardening-design.md (the
certified design; read sections 0, 1, 2 and the slice 1a row of section 10 in
full before writing a line). Slice 1a is the READY predicate, RELEVANT flight,
and their proofs — pure Go, no hook line, no verdict behaviour change. The
design decided everything; where it cites a line range, read that range on
the base tree and follow it. Zero judgment calls: a gap is reported, not
filled.

# Precondition (verify first)

Goal supervision-hook-wrong-root has landed on main (the dispatching seat
asserts it; confirm by reading metasystem/scripts/agents/supervision-hook.sh
for the root-resolution shape the root design
metasystem/plans/supervision-hook-root-design.md prescribes). If it has not,
stop and report the gap.

# Workspace

The delegate worktree the dispatcher created for this job.

# The change (slice 1a row, section 10)

1. metasystem/internal/goal/verbs.go and metasystem/internal/goal/validate.go:
   `ClaimAdmission` (side-effect-free extraction of the claim verb's rules in
   the verb's own order, carrying the authenticated claim epoch and the
   machine quota with the arc exception), `ClaimRefusal`,
   `MachineQuotaAllows`, the Mutate reorder (existence, replay
   already-applied, competitor outcomes, then ClaimAdmission) and the
   `OwnPair` export — exactly as section 1.2.0 specifies.
2. A new package for the frontier (the design names it `readywork`): `Frontier`
   with R1, R2, R3 in the SET form of section 1.2.1 (`X(g)`, `t′(g)`, the
   full re-admission over `t′(g)`), the `Move` rendered in the exact syntax
   metasystem/cmd/metasystem/goalsync_mutations.go accepts, WaitingOnHuman
   and Reasons; `Relevant` over job records AND run records with
   platform-exact identity (section 2.1, 2.2), the `dispatch.IdentityRefOf`
   export from metasystem/internal/dispatch/ownership.go, the `RunFact`
   fields from metasystem/internal/run/run.go, and the full-ref fix in
   metasystem/internal/report/scan.go.
3. Every test the slice 1a row lists, by name, including the two-arc fixture
   `TestReadyHeldReleasableNamesEveryQuotaBlockingClaim` whose decisive
   assertion executes the rendered Move line by line through the REAL verbs
   against a copy of the fixture ledger, and
   `TestReadyMoveLinesParseUnderParseSyncFlags` fed to the real
   `parseSyncFlags`.
4. No verdict behaviour changes; no existing test's expectation changes in
   this slice (that is slice 1b).

# Gate

`go build ./...`; `go vet ./...`; `gofmt -l .` empty; `go test ./internal/goal/
./internal/report/ ./internal/dispatch/ ./internal/run/` and the new package,
`-count=1`, green; then the FULL `go test ./... -count=1` green. Paste the
final lines of each run in the return. Every fixture wait carries a named
ceiling; no benchmarks (R-31).

# Constraints

Wall-clock cap as dispatched. Touch nothing outside the files the slice names.
Version-2 implementer JSON; diffBoundary lists every file changed or created.

# Gap Rule

stop and report a gap; never fill it silently.
