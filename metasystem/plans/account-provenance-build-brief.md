Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal account-provenance)
Date: 2026-09-06

# Goal

Goal account-provenance (tier 2, approved by Wido). Its record,
metasystem/plans/goals/account-provenance.md, is the contract; the
design that closed its critique ladder is the specification:
metasystem/plans/account-provenance-design.md, revision 3, folded
through two critic chains (metasystem/records/misc/account-provenance-critique-r1.md
and metasystem/records/misc/account-provenance-critique-r2.md; the last
round closed with zero material findings). In short: the paying account
joins the run record. Every session announcement and every composed job
record carries a closed `account` object captured from the runtime CLI's
own identity surface, never from an operator-set key; the landing
evaluator reports whether the hand-written "Wido@M0" stamp may retire;
capture never gates arming or dispatch.

# Facts

- The design's tree references (file and line) were read on 2026-09-02
  and 2026-09-06; main has moved since. Build to the seam by name; a
  moved line is not a gap, a seam that changed shape or vanished is.
- Every decision is in the design: the record shape (section "The
  record shape: one closed contract"), the per-runtime grades and
  mappings (Q2), the `account` adapter verb and the engine capture
  through the bounded executor (section "Capture mechanics"), the
  retirement rule with the setup-phase exclusion (Q3), the registry
  boundary (Q4), and the fixtures with their gates (Q5). Do not
  re-decide them; where the tree contradicts the design, stop and
  report the gap with the seam named.
- The devin surface is unobserved. The design ships devin at the
  `unattested` floor with `adapter-failed` on nonzero exit and
  `surface-unmapped` on exit 0. The live observation named in Q2 is
  reported in your return as a gap, never performed from the sandbox.

# Decisions (the orchestrator's; decided, not open)

D1. Implement revision 3 as written: the `internal/account` leaf package
(Validate and Capture with the fixed twenty-second bound through
`internal/boundedexec`), the closed six-key object, the two
`adapter`-family seam verbs and the codex claims decoding in
`internal/adapter/codex.go`, the `account` verb in the four adapter
scripts, the announcement key and struct and writer, the two record
composers in `internal/dispatch/build.go`, the `accountStamp` field on
the landing observation with the one printed line in `commit.sh`, and
the suite's verb list.

D2. Boundary: exactly the paths the design names. Existing:
metasystem/internal/census/announcement.go, metasystem/internal/lease/classify.go,
metasystem/internal/lease/verbs.go, metasystem/internal/up/up.go,
metasystem/internal/dispatch/build.go, metasystem/internal/adapter/codex.go,
metasystem/internal/adapter/selftestrun.go, metasystem/internal/landing/observe.go,
metasystem/scripts/agents/commit.sh, metasystem/scripts/agents/adapters/claude.sh,
metasystem/scripts/agents/adapters/codex.sh, metasystem/scripts/agents/adapters/devin.sh,
metasystem/scripts/agents/adapters/fake.sh, metasystem/scripts/agents/adapters/runtime-common.sh,
metasystem/scripts/agents/dispatch.sh, metasystem/scripts/validate-metasystem.sh,
metasystem/scripts/agents/dispatch-fixtures.sh, metasystem/cmd/metasystem/main.go,
and the existing tests the design lists in Q5. New: the
`internal/account` package with its test file, and the two adapter test
files Q5 names. Anything beyond this list is a gap, not a change.

D3. Refusals from the design's own list are gaps: capture becoming a
dispatch or arming gate; a registered runtime needing a `runtimes.go`
edit; a fixture that has to store adapter stderr or token material to
pass.

D4. Source comments describe the application, never this round, the
critique, or a finding id.

# Verification

Required, run from the worktree and reported at evidence level ran:
`scripts/agents/go-gate.sh --fast`, then `scripts/agents/dispatch-fixtures.sh`,
then `scripts/agents/goal-cli-fixtures.sh`. Every fixture Q5 names
exists and passes: the nine in `internal/account`, the six codex claims
cases plus the token-secrecy assertion, the devin floor fixture, the
retirement fixture including the failed setup husk and pending-setup
cases, the lease announcement round-trip, and the shell case in the
dispatch fixture bed. State in the return which of them fail against
the tree before your change. The orchestrator proves the `metasystem up`
wiring live after landing; do not claim it from the sandbox.

# Constraints

Wall-clock budget: 90 minutes. Return per the implementer schema with
the diff boundary listed. Gap rule: stop and report a gap; never fill it
silently.
