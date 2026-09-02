Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Write the file named codex-handshake-design.md in the metasystem plans
directory, revision 1: the design for goal
codex-handshake-budget-load-fragile. Read the goal record's intent
and next step first (`metasystem goal show --id
codex-handshake-budget-load-fragile`); the cause is FOUND and the design
has two parts Wido already chose. Your job is to specify them exactly
against the code, not to reopen them.

The evidence: on m1, codex-cli 0.148.0 takes 16 to 18 seconds from launch
to its `thread.started` event with the operator's ten interactive Codex
plugins loaded, and 1 second with `-c 'plugins={}'`; MCP servers change
nothing. The adapter declares a 10-second session-establishment budget
(metasystem/scripts/agents/adapters/codex.sh, `sessionEstablishedTimeoutSec`
in the capability snapshot), so every Codex dispatch on m1 failed with
`handshake_timeout` while the child was alive and healthy. R-35-m3 (in
metasystem/memory/rulings.md) names that a defect: anything converting
slowness into failure is fixed with progress-based patience.

# Part 1 — delegates run without the operator's plugins

metasystem/internal/adapter/codex.go `BuildCodexCommand` assembles both the
dispatch argv (`codex exec --json …`) and the follow-up argv (`codex exec
resume --json …`) from `-c` overrides. Specify the `-c plugins={}` override
on BOTH verbs (a resumed thread inherits config; state whether the override
must be repeated on resume and why), its exact position among the existing
`-c` settings, and the test rows in
metasystem/internal/adapter/codexcommand_test.go that pin it. State the
rationale for the record: a delegate never needs the operator's interactive
plugins, and the run becomes deterministic across machines. Say whether the
override belongs in the capability snapshot's configuration identity
(`codex_config_identity` in the adapter script) or is invisible to it, and
why.

# Part 2 — liveness-based patience replaces the fixed cap

Today's chain, read it all before writing:

- metasystem/scripts/agents/adapters/codex.sh writes the snapshot with the
  literal 10 (claude.sh writes 20, devin.sh 30).
- metasystem/internal/capability/select.go reads it;
  metasystem/internal/dispatch/build.go copies it onto the job record as
  `sessionEstablishedTimeoutSec`.
- metasystem/scripts/agents/dispatch.sh stamps `handshakeDeadline` at
  launch (the ownership patch, metasystem/internal/dispatch/ownership.go),
  then `await_handshake` polls the record until that deadline and calls
  `__handshake-timeout` (`internal_handshake_timeout`) on expiry, which
  writes `{"error":"handshake_timeout","phase":"handshake"}`. Note its
  guard `"$timeout" -le 60`.
- metasystem/internal/dispatch/reapfacts.go derives `HandshakeWaiting`
  from the same deadline for the reaper backstop in dispatch.sh (the
  "provably gone" supervisor rule).
- The launch loop in dispatch.sh already proves the child's start identity
  and `metasystem proc alive --identity-file` (metasystem/cmd/metasystem/census.go)
  answers whether that exact process still lives.

Decide and specify: the waiter's patience is progress-based — while the
launched process (its proven identity, not a pid number) is alive and has
not exited, the waiter keeps waiting; the fixed number becomes a HANG bound
only, judged from the last observed progress, not from launch. Define
"progress" precisely for Codex (which events count — thread.started is the
session signal; what earlier stdout/JSONL activity the adapter can surface
as liveness before it, if any — read metasystem/internal/adapter/codex.go's
event reader), define the hang bound's value and where it comes from (the
snapshot field keeps its name or is renamed; say which and why; the
snapshot is immutable evidence, so a renamed or reinterpreted field must
be versioned or defaulted for old snapshots), and define the exit verdict:
a child that EXITS before the session signal is `handshake_failed` with its
exit status quoted, distinct from a child that hangs. Keep the
`handshakeDeadline` stamp coherent with the reaper's backstop
(`HandshakeWaiting`) so the two do not disagree; state exactly what the
reaper reads and what it decides when the dispatcher is gone. The
handshake-timeout verb's stand-down rules (session landed late) stay.

Every wait carries a named ceiling (R-31: no benchmarks; fixtures use
`dispatch_fixture_wait_cap`-style scaled caps). Name the fixture scenarios
in metasystem/scripts/agents/dispatch-fixtures.sh that must change or be
added: a slow-but-alive child that signals after the old 10 seconds and is
accepted; a child that exits early and is refused with its exit status; a
child that hangs with no progress and is refused at the hang bound; the
reaper backstop with a gone dispatcher. Use the fake adapter
(metasystem/internal/adapter/fake.go) where it already models the
handshake.

# Shape of the design

Sections: 1 problem and evidence (short, cite the goal record); 2 Part 1
decisions; 3 Part 2 decisions (definitions, the waiter algorithm, the
verdict table: signal seen / exited / hung / dispatcher gone); 4 files and
functions touched, each with the change in one or two sentences; 5 tests
and fixtures by name; 6 out of scope; 7 open questions (only if a real
gap). Under 250 lines. Decisions, not options.

# Constraints

Wall-clock budget: 20 minutes. Design only; create nothing but the design
file; diffBoundary is that one file. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the new design file.

# Gap Rule

stop and report a gap; never fill it silently.
