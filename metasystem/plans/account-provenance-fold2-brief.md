Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal account-provenance)
Date: 2026-09-06

# Round 3: fold the closing critique (chain account-provenance-build1)

The fresh critic on round 2 (chain account-provenance-crit3) found one
material item, APC-01, and one cosmetic one, APC-02. APC-01 is a defect
in the design's premise, now amended in
metasystem/plans/account-provenance-design.md (Q2 codex row, Q5 codex
fixtures, fold table): the codex identity token is issued with a
one-hour lifetime and is not reissued when the CLI refreshes its access
token, so checking the identity token's `exp` against now turns nearly
every codex capture into `unattested credential-expired` while the CLI
reports a valid login. The critic proved it live on this Mac: an
identity token 7.3 days past its `exp` beside an access token still in
date, and the reviewed engine's codex `account` verb printing
`credential-expired`.

# Decisions (the orchestrator's; decided, not open)

D17. In metasystem/internal/adapter/codex.go, expiry is read from the
ACCESS token's `exp` claim, decoded the same way as the identity token's
claims, never from the identity token's `exp`. Identity (`iss`, the
account id and the email) still comes from the identity token. An
access token that is not decodable or carries no `exp` is
`credential-invalid`; an access token whose `exp` is before the engine's
now is `credential-expired`. Access token material stays confined
exactly as the identity token's does. In
metasystem/internal/adapter/codexaccount_test.go the expired case
becomes an access token past its `exp`; two cases are added: an identity
token days past its own `exp` beside an access token in date gives
`credential-claims`, and an access token without a decodable `exp` gives
`credential-invalid`; the token-secrecy assertion keeps its access-token
marker.

D18. In metasystem/scripts/agents/commit.sh, the stamp line carries no
trailing space when the evaluator names no record: `account stamp:
required (<code>)` then, and `account stamp: required (<code> <record>)`
when it does.

D19. The diff boundary is the round-2 boundary; nothing else changes.

# Verification

Required, run from the worktree and reported at evidence level ran:
`scripts/agents/go-gate.sh --fast`, `go test ./internal/adapter/
./internal/account/ -count=1`, `bash -n scripts/agents/commit.sh`, and
the codex `account` verb run live against this Mac's credential file
through the rebuilt engine, reporting only the attestation and error
fields (never an identifier).

# Constraints

Wall-clock budget: 30 minutes. Return per the implementer schema with
the cumulative diff boundary listed. Gap rule: stop and report a gap;
never fill it silently.
