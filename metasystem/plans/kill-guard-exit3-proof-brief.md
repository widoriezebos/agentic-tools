Working Mode: implement
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal kill-guard-fold-consumers)
Date: 2026-08-31

# Goal

The kill-guard's fail-closed exit code becomes provable. The dab1dbd
contract change made an empty member scan INDETERMINATE (exit 3,
fail-closed) where callers previously saw NOT-OWNED (exit 1). The
orchestrator's completed caller enumeration found every consumer
already handles exit 3 correctly — but NO test anywhere asserts exit 3
of `metasystem proc group-owned`, and the one shell fixture stubs the
guard as a boolean, structurally unable to produce 3. This change adds
the missing proof; it changes NO production behavior.

# Workspace

The dispatch-created job worktree, branched from main. May touch EXACTLY
two files (inside the metasystem/ project):

- cmd/metasystem/identity_probes_test.go (create if absent; if exit-code
  coverage for this verb clearly belongs in an existing cmd test file
  instead, report that as a gap rather than choosing)
- scripts/agents/dispatch-fixtures.sh

# Inputs

- cmd/metasystem/identity_probes.go:58 runIdentityGroupOwned — the
  exit-code surface: 0 = owned or recorded-proof matched; 1 = not owned
  (killproof.go:148 also returns it for pgid<2 or empty tag) or, on the
  fallback path only, group-not-signalable; 2 = usage; 3 = INDETERMINATE
  (no --root/--record fallback offered, fixture-auth error, record read
  error, or recorded proof mismatch).
- internal/janitor/killproof.go:175-200 groupOwnershipFromVerifications —
  the fold; empty scan returns GroupIndeterminate (line 199).
- internal/janitor/killproof_test.go — the fold's own tests ("empty scan
  proves nothing"), which prove the fold but not the verb's exit code.
- scripts/agents/dispatch-fixtures.sh:1340-1381 — the cross-group
  wind-down fixture whose group_owned stub at :1367 is boolean.
- scripts/agents/dispatch.sh:331-368 — group_owned() (lossless
  pass-through) and wind_down_one_group (refuses on any nonzero, both
  pre-TERM and post-TERM).

# Mechanical rules — no judgment calls

GO TEST. Add tests that execute the group-owned verb's decision path
and assert exit codes: (a) an empty/unscannable group with no fallback
flags exits 3, never 1; (b) a not-owned live scan exits 1; (c) a
recorded-proof mismatch with --root/--record offered exits 3. Follow
the existing cmd/metasystem test idiom (see claim_launch_test.go for
how verbs are exercised). If the verb cannot be exercised hermetically
in a unit test (real process groups needed), report the gap naming
exactly what visibility is missing — do not weaken the test to pass.

SHELL FIXTURE. Extend dispatch-fixtures.sh: the group_owned stub gains
an exit-3 mode, and a new fixture asserts wind_down_one_group refuses
to signal on exit 3 with the same refusal it gives exit 1 (the
"refusing to signal unowned process group" path) — proving fail-closed
handling of INDETERMINATE end to end. Follow the existing fixture
idiom at :1340-1381 exactly.

# Constraints

- No production code changes anywhere.
- KNOWN SANDBOX LIMIT: the shell fixture suite needs process
  visibility your sandbox denies. Do NOT run dispatch-fixtures.sh;
  write it, prove the Go tests compile and pass if the sandbox allows,
  and report anything unrunnable as environment-limited evidence. The
  orchestrator replays all proofs outside the sandbox — and holds them
  until a shared-machine window clears, which is the orchestrator's
  problem, not yours.
- Wall-clock budget: 25 minutes.

# Expected Return

Version-2 implementer JSON; diffBoundary lists exactly the two files
WITH the metasystem/ repository prefix:

- metasystem/cmd/metasystem/identity_probes_test.go
- metasystem/scripts/agents/dispatch-fixtures.sh

Evidence commands replayable from the worktree root, including
`go test ./cmd/metasystem/ -run GroupOwned -count=1` and
`bash -n scripts/agents/dispatch-fixtures.sh`.

# Acceptance Criteria

1. Diff touches exactly the two named files.
2. Go tests assert exits 3, 1, and 3 for the three cases above (or a
   reported gap names the exact missing visibility).
3. The new shell fixture makes the stub return 3 and asserts the
   wind-down refusal fires.
4. `bash -n` passes; `gofmt -l` is clean on touched Go files.
5. No existing test or fixture weakened.

# Gap Rule

stop and report a gap; never fill it silently.
