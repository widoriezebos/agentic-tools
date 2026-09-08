Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal chain-landing-after-base-move-recertifies)
Date: 2026-09-08

# Continuation: round 1 was cut off at its cap, finish and report it

READ THIS FIRST. You are not starting this build. Round 1 of this chain
ran for 120 minutes, hit its budget cap and was killed before it could
write its return. Its work survives: chain rounds accumulate in this
worktree rather than starting clean, and the worktree you have opened
already contains it. Do NOT restart, re-plan or rewrite what is there.
Your job is to confirm it, finish anything genuinely missing, and return
a proper account of it.

The specification is unchanged:
metasystem/plans/chain-landing-after-base-move-recertifies-design.md,
revision 2, final. The build contract is
metasystem/plans/chain-landing-after-base-move-recertifies-build-brief.md,
which still applies in full, including its prohibitions.

## What the orchestrator verified on the tree you are holding

Twenty paths are dirty in this worktree: sixteen modified and four new,
the four new ones being `internal/gittree/disjoint_merge.go`,
`internal/gittree/disjoint_merge_test.go`, `internal/landing/park.go`
and `internal/validate/recertification.go`.

`go build ./...` succeeds.

All three of the design's canaries exist and PASS, run individually by
the orchestrator outside your sandbox on exactly this tree:

- `go test ./internal/gittree -run '^TestDisjointMergeProof$'` passed in
  4.1 seconds.
- `go test ./cmd/metasystem -run '^TestChainLandingRecertifiesAfterBaseMove$'`
  passed in 14.9 seconds.
- `go test ./cmd/metasystem -run '^TestRecertifiedLandingParksOnOriginMove$'`
  passed in 8.7 seconds.

So the acceptance the design asks for is already met on this tree. Treat
that as the baseline and protect it: if any change you make turns one of
those three red, revert your change rather than adjusting the test.

## What to do

1. **Check one gap.** The design's implementation boundary names
   `metasystem/internal/validate/authorization.go` as a primary target
   and round 1 did not touch it. The page discusses mission
   authorization binding around its authorization section. Decide
   whether a change there is required by the specification. If it is,
   make the smallest one. If it is not, say so in the return and name
   why, citing the page.
2. **Look for its own loose ends.** Round 1 was interrupted, so check
   for the marks of unfinished work in the dirty paths: a declared flag
   with no handler, a refusal code defined and never returned, a help
   entry missing for a new command, a test helper referenced but not
   written. Fix what is genuinely incomplete. Do not refactor what
   works.
3. **Return a complete account**, which is the thing round 1 never got
   to write. The return must carry the full cumulative `diffBoundary`
   naming every one of the paths this chain has changed, including the
   four new files, or conformance cannot validate the round. Also state
   plainly in the return that round 1 was capped and that this round
   continued its worktree rather than rebuilding it.

## Verification

Canary first, and report each individually:

- The three canaries above, by their exact commands, so the return
  carries them in your own evidence rather than only mine.
- `go build ./...`
- `go test` once for the packages this chain touches:
  `./internal/gittree ./internal/landing ./internal/validate ./internal/dispatch ./cmd/metasystem`
- `bash -n` for `scripts/agents/land.sh` and `scripts/agents/commit.sh`.
- `scripts/agents/go-gate.sh --fast`, once, at the end. If staticcheck
  cannot write its cache in your sandbox, redirect the cache and say so.

Do not run the fixture beds and do not run a battery; the orchestrator
runs those and the receipt-bound battery outside your sandbox.

Gap rule: stop and report a gap; never fill it silently. If you believe
round 1 left something structurally wrong rather than merely unfinished,
say that in the return instead of redesigning it.
