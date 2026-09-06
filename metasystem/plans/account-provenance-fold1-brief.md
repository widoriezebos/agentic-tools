Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal account-provenance)
Date: 2026-09-06

# Round 2: close the boundary gap of round 1 (chain account-provenance-build1)

Round 1 built the account-provenance contract of
metasystem/plans/account-provenance-design.md revision 3 and reported one
boundary gap: the design's boundary named no room for
metasystem/internal/lease/verbs_test.go, which calls
`AnnounceWithProofAt` with the old arity, so round 1 kept that file
compiling by giving the function a variadic account slot and a runtime
check that at most one account is passed. The specification wants an
exact trailing account pointer; the round-1 brief's boundary was the
orchestrator's omission. The other two gaps stand as reported: the devin
surface stays unobserved at the floor, and the process-owning fixture
beds are the orchestrator's to run on the reviewed tree, not yours.

# Decisions (the orchestrator's; decided, not open)

D15. The boundary gains metasystem/internal/lease/verbs_test.go.
`AnnounceWithProofAt` takes an exact trailing parameter
`account *account.Account` (nil admitted, meaning no observation) in
place of the variadic slot, and the "at most one account" check goes.
Every caller passes the argument explicitly: the wrapper in
metasystem/internal/lease/verbs.go passes nil, metasystem/internal/up/up.go
passes the captured account, and the tests in
metasystem/internal/lease/verbs_test.go, metasystem/internal/lease/classify_test.go
and metasystem/internal/landing/observe_test.go pass what they test.

D16. Nothing else changes in this round. The diff boundary is the
round-1 boundary plus verbs_test.go.

# Verification

Required, run from the worktree and reported at evidence level ran:
`scripts/agents/go-gate.sh --fast`, then `go test ./internal/lease/
./internal/up/ ./internal/landing/ ./internal/account/ ./internal/adapter/
./internal/census/ ./internal/dispatch/ -count=1`, then `bash -n` on the
five shell files of the boundary.

# Constraints

Wall-clock budget: 30 minutes. Return per the implementer schema with
the cumulative diff boundary listed. Gap rule: stop and report a gap;
never fill it silently.
