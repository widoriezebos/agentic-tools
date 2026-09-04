Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Review brief: closing review of the tiering machinery, slice 2b, after its second correction and coverage round (chain str-p2-build-2d, round 1, carrying str-p2-build-2c round 3)

Round budget: 1 focused round; this is the closing pass. R-60-m1's
rule: material only if it changes what gets built and names the
artifact. Prior rounds: metasystem/records/misc/severity-tiered-rigor-p2-2b-critique-cc1.md
and metasystem/records/misc/severity-tiered-rigor-p2-2b-critique-cc2.md;
the second correction's orders: metasystem/plans/severity-tiered-rigor-p2-2b-fix2-brief.md.

Subject: the computed diff of implementer job str-p2-build-2d round 1
(its diff.patch under that chain's round-1 directory; reviewed tree
fdc9882590965651fce4cddbe8eefa1e8056db8a, 46 files, against main at eefeb16c; the diff is the authority).
That round carried the twice-reviewed round-3 diff of chain
str-p2-build-2c, rebased by the seat onto parts one and three (the
merge kept both sides: the budget struct's review-round and tier
members beside the finding/chain/why/test members, the legacy
review-round inference beside the bare-review-id helper, `--tier 3`
and `--goal-tier` on the fixture and build-record lines, and
`ResolveGoalRevision`'s third return ignored by the register rebind),
plus a tests-only coverage round in internal/dispatch and
internal/validate. Part three's law refused a follow-up on the old
chain, which is why the chain changed; nothing else did. Threat model and contract as in
metasystem/plans/severity-tiered-rigor-p2-2b-code-review-brief.md.

# Mandate

1. F-1 closed: the design-blob provenance check accepts a 40- or
   64-hex git object id through its own pattern, content digests keep
   the 64-hex rule, and TestBuildRecordDesignCriticCarriesDeclaredOutputs
   runs in a default (SHA-1) repository. This repository's own
   design-critic dispatch can now write a record.
2. F-2 closed: the cap-driver and cap-warden block of
   dispatch-fixtures.sh runs conformance on the reviewed implementer
   before the critic fold, its reviewed change is nested under
   `metasystem/` so the artifact grammar accepts it, every rigor row
   carries that artifact, its expected texts are the ones the engine
   now emits, and the removed successor retry is not silently
   replaced by an untested path. The dispatching seat runs the script
   on this tree while you review; judge the block by reading.
3. F-4, F-5, F-6 as ordered: the ` b/` header parse keeps a path with
   a space; the legacy path proves cancelled rounds consume nothing
   and exhaustion advances with both counters absent; the dead
   first-exhaustion branches and constant are gone with the malformed-
   record validation kept.
4. The carry and the coverage round: the merge resolutions above are
   faithful to both sides (no dropped member, flag, or test); the
   coverage tests exercise the package-level entry points they name
   and assert behaviour, not just lines; no production file changed
   in the coverage round beyond the carried diff.
5. Regression: nothing from rounds 1 and 2 reopened; `validArtifactPath`
   and `projectInstallPrefix` unchanged (F-3 stays backlog); the five
   recorded decisions are mechanical.

Known and out of scope, do not report: the `dispatch` scenario of
dispatch-fixtures.sh, red on main since the alias landing 2c3776b8
(recorded for the fixture-drift goal); the goal package's full run
and the fixture scripts (the seat's run, recorded at landing).

Finding identifiers: this repository's register refuses an id that
another chain already carries. Name your findings STR2P2B-01,
STR2P2B-02, ... and never F-n; refer to the prior rounds' F-n only as
history.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree above. Read the chain's delegate worktree for context;
the diff is the subject. Do not run scripts/agents/path-class-fixtures.sh;
run a test only if a finding needs it.

# Gap Rule

stop and report a gap; never fill it silently.
