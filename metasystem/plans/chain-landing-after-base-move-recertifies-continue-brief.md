Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal chain-landing-after-base-move-recertifies)
Date: 2026-09-08

# Continue a build that was killed at its cap

## FIRST ACTION, before anything else

Your worktree is seeded by the orchestrator immediately after your
dispatch, so it may be empty for the first seconds of your life. Before
you read anything else or touch any file, wait for the seed:

```sh
for i in $(seq 1 60); do [ -f .orchestrator-seeded ] && break; sleep 2; done
cat .orchestrator-seeded
git status --porcelain | wc -l
```

The marker file `.orchestrator-seeded` sits at the root of your
workspace, one level above `metasystem/`. When it appears, the working
tree carries the previous round's work. The status count must be 21:
twenty changed paths plus the marker.

If the marker has not appeared after two minutes, STOP and report that
as a gap. Do not build anything from scratch. Do not delete the marker.

## What happened, and what you are not doing

Round 1 of this work ran for 120 minutes, hit its budget cap and was
killed before it could write its return. Its code survives and is what
the orchestrator seeded into your worktree. The machinery could not hand
it to you itself: a follow-up refuses after a timeout, and a fresh
dispatch refuses to reuse an existing worktree, so the orchestrator
carried the diff across by hand. Goal
capped-round-continues-instead-of-restarting exists to remove that
manual step.

You are NOT starting this build. Do not restart, re-plan or rewrite what
is there.

The specification is
metasystem/plans/chain-landing-after-base-move-recertifies-design.md,
revision 2, final. The build contract is
metasystem/plans/chain-landing-after-base-move-recertifies-build-brief.md
and it still applies in full, including every prohibition in it.

## The baseline the orchestrator verified on exactly this code

Twenty paths: sixteen modified, four new. The four new files are
`internal/gittree/disjoint_merge.go`,
`internal/gittree/disjoint_merge_test.go`,
`internal/landing/park.go` and
`internal/validate/recertification.go`.

`go build ./...` succeeds, and all three of the design's canaries pass,
each run individually outside the delegate sandbox:

- `go test ./internal/gittree -run '^TestDisjointMergeProof$'`, 4.1s.
- `go test ./cmd/metasystem -run '^TestChainLandingRecertifiesAfterBaseMove$'`, 14.9s.
- `go test ./cmd/metasystem -run '^TestRecertifiedLandingParksOnOriginMove$'`, 8.7s.

So the design's acceptance is already met. Protect that: if a change of
yours turns one of those three red, revert your change rather than
adjusting the test.

## What to do

1. **Check one gap.** The design's implementation boundary names
   `metasystem/internal/validate/authorization.go` as a primary target
   and round 1 never touched it. Decide from the page whether a change
   there is required. If it is, make the smallest one. If it is not, say
   so in the return and cite the page.
2. **Find the marks of interruption.** Round 1 was cut off mid-flight,
   so look for a declared flag with no handler, a refusal code defined
   and never returned, a missing help entry for a new command, a helper
   referenced but not written. Fix what is genuinely incomplete. Do not
   refactor what works.
3. **Return the account round 1 never wrote.** The return must carry the
   full cumulative `diffBoundary` naming every path this chain changed,
   including the four new files, or conformance cannot validate the
   round. State in the return that round 1 was capped and that this
   round continued its code rather than rebuilding it.
4. **Delete the marker** as your last file action, so it never reaches
   the review: `rm -f .orchestrator-seeded`, from your workspace root.

## Verification

Canary first, each reported individually:

- The three canaries above, by their exact commands, so your evidence
  carries them and not only mine.
- `go build ./...`
- `go test` once for the packages this chain touches:
  `./internal/gittree ./internal/landing ./internal/validate ./internal/dispatch ./cmd/metasystem`
- `bash -n` for `scripts/agents/land.sh` and `scripts/agents/commit.sh`.
- `scripts/agents/go-gate.sh --fast`, once, at the end. If staticcheck
  cannot write its cache in your sandbox, redirect the cache and say so.

Do not run the fixture beds and do not run a battery; the orchestrator
runs those and the receipt-bound battery outside your sandbox.

Gap rule: stop and report a gap; never fill it silently. If round 1 left
something structurally wrong rather than merely unfinished, say that in
the return instead of redesigning it.
