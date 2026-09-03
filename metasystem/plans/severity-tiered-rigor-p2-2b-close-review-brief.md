Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Review brief: closing review of the tiering machinery, slice 2b, after its second correction (chain str-p2-build-2c, round 3)

Round budget: 1 focused round; this is the closing pass. R-60-m1's
rule: material only if it changes what gets built and names the
artifact. Prior rounds: metasystem/records/misc/severity-tiered-rigor-p2-2b-critique-cc1.md
and metasystem/records/misc/severity-tiered-rigor-p2-2b-critique-cc2.md;
the second correction's orders: metasystem/plans/severity-tiered-rigor-p2-2b-fix2-brief.md.

Subject: the computed diff of implementer job str-p2-build-2c round 3
(its diff.patch under the chain's round-3 directory; reviewed tree
3cc4ff9ade2a3510d19aaa54cbb9bd39b31d09d4, 45 files against main at
d59cc100; the diff is the authority). Threat model and contract as in
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
4. Regression: nothing from rounds 1 and 2 reopened; `validArtifactPath`
   and `projectInstallPrefix` unchanged (F-3 stays backlog); the five
   recorded decisions are mechanical.

Known and out of scope, do not report: the four reds on main from
landing c285d5a0; the goal package's full run and the fixture scripts
(the seat's run, recorded at landing).

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree above. Read the chain's delegate worktree for context;
the diff is the subject. Do not run scripts/agents/path-class-fixtures.sh;
run a test only if a finding needs it.

# Gap Rule

stop and report a gap; never fill it silently.
