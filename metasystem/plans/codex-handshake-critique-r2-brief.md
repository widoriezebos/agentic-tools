Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Round-2 critique of metasystem/plans/codex-handshake-design.md revision 2
(landed, in your worktree), which folded your six round-1 findings
(metasystem/records/misc/codex-handshake-critique-r1.md, landed):
CHS-R1-SNAPSHOT-01 (a new capability `handshakeProgressBoundSec` beside the
old field, `handshakeBound` launch|progress on the job record, one gate in
`RefreshHandshakeDeadline`), CHS-R1-EXIT-02 (`handshake_failed:exit=N`
unfolded for every role), CHS-R1-DEADLINE-03 (one `>=` boundary for waiter,
custodian and reaper), CHS-R1-PROGRESS-04 (three writers, launch stamps
both fields), CHS-R1-FIXTURE-05 (no-signal relates the verdict to the
refreshed deadline; every wait names its scaled cap), CHS-R1-FILES-06 (test
files enumerated). Judge each fold BY ID against the code it cites —
metasystem/internal/capability/select.go, metasystem/internal/dispatch/build.go,
metasystem/internal/dispatch/record.go, metasystem/internal/dispatch/ownership.go,
metasystem/internal/dispatch/custody.go, metasystem/internal/dispatch/reapfacts.go,
metasystem/internal/adapter/adjudicate.go, metasystem/internal/adapter/fake.go,
metasystem/scripts/agents/dispatch.sh, metasystem/scripts/agents/adapters/runtime-common.sh,
metasystem/scripts/agents/adapters/codex.sh, metasystem/scripts/agents/adapters/fake.sh,
metasystem/scripts/agents/dispatch-fixtures.sh, metasystem/scripts/agents/fixture-budget.sh
— and confirm no regression in what round 1 left standing (Part 1, section
2, unchanged). Two things to check hardest: does adding `handshakeBound` to
`immutableFields` break any existing pending→pending CAS that rewrites the
record wholesale; and does `criticFailureFold`'s consumer (whatever needed
`protocol_error`) still work when a critic job ends `handshake_failed:exit=N`.

Findings material and grounded, quoting the disagreeing text or code, ids
CHS-R2-<TOPIC>-NN. Your sandbox is read-only: verify by reading, do not run
go. Zero material findings is an acceptable, closing answer if the reading
supports it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
