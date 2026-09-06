Working Mode: build
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal deadline-expired-register-row)
Date: 2026-09-06

# Goal

Goal deadline-expired-register-row (tier 1, approved by Wido). Its
record, metasystem/plans/goals/deadline-expired-register-row.md, is the
contract. The steward component outcome DEADLINE_EXPIRED landed in
19b14a9b without a row or exclusion in the refusal register, so
TestHCL03EveryCodeRowed in metasystem/internal/refusal/register_test.go
fails on main with "collected refusal token DEADLINE_EXPIRED has no row
or exclusion". The token is a component-evidence outcome that refuses
nothing, exactly like AUTO_HEAL_ELIGIBLE, which the register excludes
with the reason "a steward health observation that refuses nothing".

# The change

In metasystem/internal/refusal/register.go add one exclusion entry
immediately after the AUTO_HEAL_ELIGIBLE entry:
`{Pattern: "DEADLINE_EXPIRED", Reason: "a steward health observation that refuses nothing"}`.
Nothing else changes.

# Gate

`cd metasystem && GOTOOLCHAIN=go1.26.5 go build ./... && gofmt -l .` (empty);
`GOTOOLCHAIN=go1.26.5 go test ./internal/refusal/ -count=1` green (the
host runs Go 1.27.1 today and the pinned static checker cannot read it;
the toolchain variable is the workaround).

# Constraints

Wall-clock budget: 10 minutes. Declare the boundary as the one file.
Gap rule: stop and report a gap.

# Expected Return

Version-2 implementer JSON as the role schema requires.

# Gap Rule

stop and report a gap; never fill it silently.
