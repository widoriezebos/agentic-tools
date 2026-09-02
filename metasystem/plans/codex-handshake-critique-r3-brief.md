Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal codex-handshake-budget-load-fragile)
Date: 2026-09-02

# Goal

Round-3 critique of metasystem/plans/codex-handshake-design.md revision 4
(landed, in your worktree). Revisions 3 and 4 together fold your one
round-2 finding, CHS-R2-CRITIC-FOLD-01
(metasystem/records/misc/codex-handshake-critique-r2.md, landed), in three
parts: D2.5 restores the critic fold to `protocol_error` and carries the
exit status as a record field `handshakeExitStatus` written by the
custodian's fail-pending branch for every role; D2.7 admits a `failed`
plus `protocol_error` parent with no session at the follow-up gate and
sends it down the fresh-context road so the register fold runs; and the
orchestrator's claim decision (candidate a): the sessionless follow-up
claims with the child's own session key and an empty resumed session,
marked by a new `ParentSessionless` request field under `omitempty`, with
NO `LaunchFingerprintVersion` bump because
metasystem/internal/dispatch/hazard.go line 339 and
metasystem/internal/dispatch/claim.go line 113 would disown every existing
proven record on a bump.

Judge those three parts BY ID against the code:
metasystem/internal/adapter/adjudicate.go, metasystem/internal/adapter/patch.go,
metasystem/cmd/metasystem/adapter_verbs.go, metasystem/scripts/agents/adapters/runtime-common.sh,
metasystem/scripts/agents/dispatch.sh (the follow-up gate, the fresh-context
road, the three claim lines), metasystem/internal/dispatch/claim_fingerprint.go
(is the design's claim that the flag is "never hashed" true — read how the
digest is computed and whether an `omitempty` field can change any existing
v2 digest), metasystem/internal/dispatch/claim.go, metasystem/internal/dispatch/hazard.go
(what an empty `resumedSessionId` does at lines 280 and 386),
metasystem/internal/dispatch/record.go, metasystem/internal/dispatch/finding_register.go,
metasystem/scripts/agents/adapters/fake.sh and
metasystem/scripts/agents/dispatch-fixtures.sh (the `exit-before-session`
round-2 leg and the existing `null-session-follow-up` scenario). Confirm no
regression in what rounds 1 and 2 left standing (Part 1 and D2.1 to D2.4,
D2.6, unchanged since revision 2).

Findings material and grounded, quoting the disagreeing text or code, ids
CHS-R3-<TOPIC>-NN. Your sandbox is read-only: verify by reading, do not run
go. Zero material findings is an acceptable, closing answer if the reading
supports it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
