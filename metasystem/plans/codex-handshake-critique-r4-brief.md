Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Round-4 critique of metasystem/plans/codex-handshake-design.md revision 6
(landed, in your worktree). Revisions 5 and 6 fold your one round-3
finding, CHS-R3-EXIT-01 (metasystem/records/misc/codex-handshake-critique-r3.md,
landed): the critic fold is single-sourced through a new engine verb
`adapter critic-fold` that `fail_pending` and `finish_running` (failed
target) in metasystem/scripts/agents/adapters/runtime-common.sh call
before writing, so devin.sh's direct calls (lines 650-662 and 720) fold
from the one Go table in metasystem/internal/adapter/adjudicate.go,
refactored to an (error, phase) pair form; the pair
`handshake_missing_session_id handshake` joins the table for critic roles;
the proof rows live in metasystem/scripts/agents/adapter-deadline-fixtures.sh
plus Go rows. Judge that fold BY ID against the code
(metasystem/scripts/agents/adapters/devin.sh, runtime-common.sh,
adjudicate.go, metasystem/cmd/metasystem/adapter_verbs.go, the deadline
harness, metasystem/scripts/agents/dispatch.sh's follow-up gate) — in
particular whether the verb's idempotence claim holds so nothing
double-folds on `complete_from_cli`'s path, whether `finish_running` is
reached with a `failed` target anywhere a fold would be WRONG (a
non-transport failure that a critic must not have laundered into
`protocol_error`), and whether the harness's stub dispatch can observe the
folded patch as the design asserts. Confirm no regression in what rounds 1
to 3 left standing (Part 1, D2.1 to D2.4, D2.6, D2.7, the claim decision).

Findings material and grounded, quoting the disagreeing text or code, ids
CHS-R4-<TOPIC>-NN. Your sandbox is read-only: verify by reading, do not run
go. Zero material findings is an acceptable, closing answer if the reading
supports it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
