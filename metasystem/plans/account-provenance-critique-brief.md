Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal account-provenance)
Date: 2026-09-06

# Review brief: fresh independent critique of the final tree (chain account-provenance-build1, work round account-provenance-build1-r2)

FINDING IDS: chain-unique, APC-01, APC-02, ... never F-n.

Why this review exists: chain completion under DESIGN-BEARING reach
requires a fresh-context critic chain whose reviewed job is the final
work round. Round 1 built the contract; round 2 closed one boundary gap
(the exact announcement signature). This chain examines the FINAL tree
in a fresh session and closes the build chain.

Round budget: 1 focused round. A finding is material only if it changes
what gets built and names the artifact it would change.

Specification: metasystem/plans/account-provenance-design.md, revision 3
(three critique rounds closed it: metasystem/records/misc/account-provenance-critique-r1.md
and metasystem/records/misc/account-provenance-critique-r2.md). Build
contract: metasystem/plans/account-provenance-build-brief.md and
metasystem/plans/account-provenance-fold1-brief.md (D15, D16). Goal record:
metasystem/plans/goals/account-provenance.md. The orchestrator persisted
the diff and the reviewed tree hash at
metasystem/artifacts/agents/account-provenance-build1/rounds/2/review.json
(the diff sits beside it); take reviewedTree from that record. The
orchestrator's runs on the reviewed tree are listed at the end.

# Mandate

1. The record contract: the closed six-key `account` object, attested
   requires both identifiers, unattested carries none, cause-token
   errors, engine-stamped `capturedAt`, one validator serving the
   announcement and the two job record composers; no raw output, stderr
   or token material can reach a record.
2. Capture: every adapter runs through the bounded executor with the
   fixed twenty-second bound in its own process group; capture never
   gates `metasystem up` or a dispatch; a missing adapter is `unattested`.
3. The per-runtime grades: claude `cli-surface` from `claude auth status
   --json`; codex `credential-claims` exactly as defined (login reported
   plus locally decoded claims with issuer and expiry, no signature
   verification claimed) with token material confined; devin at the
   floor (`adapter-failed` on nonzero exit, `surface-unmapped` on exit 0,
   never `not-logged-in`); fake fixed with its knobs.
4. Retirement: the landing evaluator's `accountStamp` skips setup-phase
   job records, resolves the calling main by ancestry, judges every
   composed record with that `mainId` plus the declared chain, and
   `commit.sh` prints one line and never refuses on it.
5. Fixtures: every fixture Q5 names exists and would fail against the
   old code; no test weakened; nothing outside the declared boundary
   (the round-1 boundary plus `internal/lease/verbs_test.go`).

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 40 minutes. Return per the code-critic schema with
the reviewedTree from the persisted round-2 review record. Gap rule:
stop and report a gap; never fill it silently.

# Orchestrator runs

On the reviewed tree (ac5efedf66700fd646c082b37f490a021a92d04e, the
worktree rebased by the dispatcher onto main 88d766ab), from this Mac,
evidence level ran: `gofmt -l` on internal and cmd printed nothing; `go
vet` passed for the lease, up, landing, account, adapter, census,
dispatch and command packages; `bash -n` passed for the seven shell files
of the boundary; `go test -count=1` passed for lease, up, landing,
account, adapter, census and dispatch; `scripts/agents/go-gate.sh --fast`,
the whole `scripts/agents/dispatch-fixtures.sh` bed (dispatch, adapter
selftest, mission-runner and steward fixtures) and
`scripts/agents/goal-cli-fixtures.sh` passed on the same tree under a
loaded machine; `git apply --check --directory=metasystem` of the round-2
diff against main succeeded.
