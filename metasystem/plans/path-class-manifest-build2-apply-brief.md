Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Re-issue, on this machine, the reviewed second part of the path-class
manifest (landing rules by class) exactly as it was built and corrected
on m1, so that a code-critic chain here can close it and the landing can
bind to that chain. The m1 chain's job records do not travel between
machines; the work does, as the branch origin/preserve/path-class-build2-r5
(its metasystem/ subtree is the reviewed tree
852f7250a87ee190465a0a367bea7f9d2a7fd8aa).

# Workspace

The delegate worktree the dispatcher created for this job. The branch
origin/preserve/path-class-build2-r5 is already fetched in this repository.

# The change

Take the eleven files below from that branch, byte for byte, with one
command from the repository root:

    git checkout origin/preserve/path-class-build2-r5 -- \
      metasystem/cmd/metasystem/landing_verbs.go \
      metasystem/internal/landing/observe.go \
      metasystem/internal/landing/observe_test.go \
      metasystem/internal/landing/promotion.go \
      metasystem/internal/stateroot/owner.go \
      metasystem/scripts/agents/commit.sh \
      metasystem/scripts/agents/land-fixtures.sh \
      metasystem/scripts/agents/land.sh \
      metasystem/scripts/agents/landing-promotion.json \
      metasystem/scripts/agents/path-class-fixtures.sh \
      metasystem/scripts/agents/static-reproof-fixtures.sh

Then prove the files equal the branch: `git diff --stat
origin/preserve/path-class-build2-r5 -- <the eleven paths>` prints
nothing. Write nothing else. Do not edit, reformat or "improve" any of
them; the review that follows judges this exact tree. Declare the
boundary as exactly these eleven paths, with the metasystem/ prefix.

# Inputs

metasystem/plans/path-class-manifest-design.md revision 2 (sections 3,
5, 6, 7 are the contract); metasystem/plans/path-class-manifest-build2-brief.md
(the original build brief); metasystem/plans/path-class-manifest-build2-fix4-brief.md
(the one folded correction, PCM-CC8-001). Read them only to report; they
authorize no change here.

# Constraints

Wall-clock budget: 15 minutes. MECHANICAL reach for this round: a
byte-exact re-issue of a reviewed tree. Non-goals: any change to the
eleven files, any other file, any test run beyond the gate below.

# Gate

`cd metasystem && go build ./... && go test ./internal/landing/ -count=1`
green. Do not run scripts/agents/path-class-fixtures.sh: it calls
ripgrep, which this host lacks (backlog item path-class-fixture-ripgrep).

# Expected Return

Per the implementer schema: the eleven-path boundary, the empty
diff-stat against the branch as evidence, the gate commands and their
observed results.

# Gap Rule

stop and report a gap; never fill it silently.
