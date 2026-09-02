Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Round-1 critique of the design file codex-handshake-design.md in the
metasystem plans directory (revision 1, landed, in your worktree), written
by the Fable lane for goal codex-handshake-budget-load-fragile. The brief it
answered is metasystem/plans/codex-handshake-design-brief.md; the goal
record carries the evidence (`metasystem goal show --id
codex-handshake-budget-load-fragile`): codex-cli 0.148.0 on m1 reaches
`thread.started` in 16 to 18 seconds with the operator's ten plugins loaded
and in 1 second with `-c 'plugins={}'`, against a 10-second handshake cap.

Judge the design against the code it must change, by reading:
metasystem/internal/adapter/codex.go (`BuildCodexCommand`),
metasystem/internal/adapter/codexcommand_test.go,
metasystem/scripts/agents/adapters/codex.sh (the snapshot literal and
`codex_config_identity`), metasystem/internal/capability/select.go,
metasystem/internal/dispatch/build.go, metasystem/internal/dispatch/ownership.go,
metasystem/internal/dispatch/reapfacts.go, metasystem/scripts/agents/dispatch.sh
(the launch loop, `await_handshake`, `internal_handshake_timeout`, the
reaper's handshake backstop), metasystem/cmd/metasystem/census.go
(`proc alive`), metasystem/internal/adapter/fake.go and
metasystem/scripts/agents/dispatch-fixtures.sh.

Questions the design must survive:

1. Part 1: is `-c plugins={}` the correct Codex CLI override form for
   0.148.0, and does it hold on `codex exec resume` (a resumed thread
   inherits config)? Does the override belong in the snapshot's
   configuration identity or not, and does the design say why?
2. Part 2: is "progress" defined so the waiter cannot wait forever on a
   child that is alive but stuck (the hang bound is judged from last
   progress, so a child that never progresses is refused at exactly that
   bound), and is the exit verdict (`handshake_failed` with exit status)
   distinct from the hang verdict?
3. Do the dispatcher's waiter and the reaper's backstop
   (`HandshakeWaiting`, the "provably gone" supervisor rule) still work
   from ONE number? Show the case where the dispatcher is gone and the
   child is alive but pre-session: what does the reaper decide, and when?
4. Old snapshots are immutable evidence: does a reinterpreted or renamed
   `sessionEstablishedTimeoutSec` stay readable, and does `await_handshake`'s
   `-le 60` guard survive or go, with a reason?
5. R-35-m3 and R-31: every wait carries a named ceiling, no wait converts
   slowness into failure, no benchmarks. Are the named fixture scenarios
   sufficient to prove slow-but-alive, early-exit, hung and
   dispatcher-gone, and do they use scaled caps?
6. Does the change stay inside the files section 4 names, or does it need a
   file the design forgot?

Findings material and grounded, quoting the disagreeing text or code, each
with an identifier of the form CHS-R1-<TOPIC>-NN. Your sandbox is
read-only: verify by reading, do not run go. Zero material findings is an
acceptable, closing answer if the reading supports it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
