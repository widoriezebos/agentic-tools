Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Review brief: re-review of the tiering machinery, slice 2b, after its one correction (chain str-p2-build-2c, round 2)

Round budget: this is the re-review after the one correction the
ceremony allows; a material finding here goes to Wido with evidence,
so name only what changes what gets built (R-60-m1) and name its
artifact. Nothing material closes the chain and slice 2b lands.

Subject: the computed diff of implementer job str-p2-build-2c round 2
(its diff.patch under the chain's round-2 directory; reviewed tree
aea3bbefb86cf1112bf0de87c783e8cf90d84736, 44 files against main at
de92aa8b; the diff is the authority). The round-1 review is
metasystem/records/misc/severity-tiered-rigor-p2-2b-critique-cc1.md
(F-1 to F-8 accepted, F-9 and F-10 noted); the correction's orders are
metasystem/plans/severity-tiered-rigor-p2-2b-fix-brief.md. The
implementer's round-2 return lists what it did per finding, the new
test names, and seven recorded decisions (one: conformance's install-
prefix derivation is duplicated in dispatch because validate imports
dispatch; judge whether the two derivations are the same rule).

# Mandate

1. Each of F-1 to F-9 is fixed as the fix brief orders, and the test
   the brief demands exists, proves the property, and is named in the
   return: F-1 TestCritiqueSubjectPrefixesProjectRelativeDiffPaths
   (project-relative headers and the rename form both yield a non-empty
   subject set); F-2 TestLegacyCritiqueRootBackfillsRoundAccountingOnAdvance
   and TestCritiqueBudgetRebindBackfillsLegacyAccounting (a root with
   a register and neither counter folds, advances and closes; the
   consumed count excludes cancelled rounds and does not double-count
   the round being folded); F-7 TestCritiqueRegisterCloseKeepsRegisterlessCompatibility
   (one helper shared by the three readers); F-3 the fixture bed's
   design file at the nested path; F-4 TestSTR3Gap05AcceptRiskWritesGoalCounselorAndRegisterThenCloses
   drives the verb through the injected prover: pair refused, three
   writes in order, close succeeds, bounded finding refused; F-5 the
   two union tests with two critic roots; F-6 TestBuildRecordDesignCriticCarriesDeclaredOutputs
   and TestDelegateDesignCriticWithoutOutputsRefusesAtFrontDoor; F-8
   TestCritiqueClosePrintsEveryBlockingFinding names two entries; F-9
   the two next-step texts verbatim.
2. Regression: the round-1 properties still hold on this tree (the
   close order and idempotence, the out-of-scope write refusing severe
   and unproven before any write, accept-risk human-only, a malformed
   present register still a refusal, the seam constant untouched, and
   nothing part one owns touched).
3. The decisions: each of the seven is a mechanical choice inside the
   contract, or name the one that is not.

Known and out of scope, do not report: the four reds on main from
landing c285d5a0 (named in the round-1 brief); the goal package's full
run and the fixture scripts, which the dispatching seat runs on this
tree while you review (the implementer's sandbox could run neither).

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree above. Read the chain's delegate worktree for context;
the diff is the subject. Do not run scripts/agents/path-class-fixtures.sh;
run a test only if a finding needs it.

# Gap Rule

stop and report a gap; never fill it silently.
