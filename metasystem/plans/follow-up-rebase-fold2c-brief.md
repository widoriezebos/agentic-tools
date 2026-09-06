Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal delegate-follow-up-cannot-merge-main)
Date: 2026-09-06

# Round 4: the round-2 fold brief with its boundary corrected (chain followup-rebase-build1)

Your round-3 gap is accepted. The boundary sentence of the previous brief
omitted the file that holds the record validation; this brief amends
it and nothing else. Every decision D7 to D11 of the previous brief
stands as written, with the D10 record shapes restated here.

# Boundary, corrected

The files this round may touch: metasystem/scripts/agents/dispatch.sh,
metasystem/scripts/agents/dispatch-fixtures.sh,
metasystem/internal/dispatch/build.go,
metasystem/internal/dispatch/record.go,
metasystem/cmd/metasystem/dispatch_verbs.go, and the two files round 1
created, internal/dispatch/followup_rebase.go and
internal/dispatch/followup_rebase_test.go. Source comments state
application facts; never a finding id, a round or a brief.

# D10, the record shapes

The validation in metasystem/internal/dispatch/build.go admits exactly
three shapes of the rebase fields on a follow-up record: no provenance
and no conflicts (rebasedFrom null, rebasedTo null, conflictedPaths
empty); both full Git object identifiers with zero or more conflicted
paths; or rebasedFrom null with a full rebasedTo and at least one
conflicted path, which means an earlier wrapper fast-forwarded this
worktree and this one found its markers. Every other combination is
rejected with a message naming the fields.

# Verification

As in the previous brief: `gofmt -l .`, `go vet ./internal/dispatch/
./cmd/metasystem/`, `go test ./internal/dispatch/ -count=1`, `bash -n
scripts/agents/dispatch.sh`, and the dispatch fixture bed including the
new scenarios. Report every run and its outcome; a scenario you cannot
make pass is a gap to report, never to skip.

# Constraints

Wall-clock budget: 60 minutes. Gap rule: stop and report a gap; never
fill it silently. Return per the implementer schema with the full
boundary.
