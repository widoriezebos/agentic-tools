# Snapshot-scope critique, round 2 dispositions

Round 2 verdict: 13 material findings. All thirteen adjudicated;
every code citation the round leaned on was verified before folding
(state schemaVersion 3 and the exact-key validation at
`internal/mission/state.go:709/:236/:166` reproduced exactly as the
critic reported). Zero refutations this round — each finding survived
a refutation attempt on the facts.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| WSS-R2-01 | ACCEPTED | Symbolic "HEAD" resolves at exec time (gittree.go:192); judging and projecting at different instants is a TOCTOU | One resolution per inspection; end-of-inspection stability re-check; bounded re-runs then "repository would not hold still" |
| WSS-R2-02 | ACCEPTED | headCommitPost was recorded but unconsumed; between-turns motion escaped | Continuity section: turn open and resume run the full accounting from the previous acceptance's posture; refMapPost/topStagedPost join the payload; WSS-12 |
| WSS-R2-03 | ACCEPTED | Raw-view pinning protects the wall's own reads only; an active refs/replace mapping re-routes later unpinned git operations | refs/replace empty at admission; any creation or motion violates outright |
| WSS-R2-04 | ACCEPTED | heads+tags enumeration plus a namespace exemption left refs/keep, refs/remotes, and host-created refs/metasystem as retention lanes | refMap is all-namespace; exemptions are record-derived only; mid-turn fetches that move refs refuse (stated consequence) |
| WSS-R2-05 | ACCEPTED | "Live" is not a lifecycle boundary: terminal-but-unconsumed branches are legitimate candidates (loop.go:1168-1207) | Exemption keyed to this mission's dispatch job records until the chain's authorization is consumed; post-consumption motion judged |
| WSS-R2-06 | ACCEPTED | A default fast-forward puts implementer intermediate commits on the first-parent chain where whole-patch membership rightly fails | Integration is --no-ff by contract; the ff refusal names the remedy; WSS-8's positive lane is the no-ff merge |
| WSS-R2-07 | ACCEPTED | Declarations authorize exact PATHS, not sealed bytes (parent §host-artifact delta); whole-delta-or-none rejects a lawful draft commit | Declared paths are content-free at commit granularity; the conclusion equation still binds the final delta |
| WSS-R2-08 | ACCEPTED | equal-to-HEAD-or-post refuses an index holding a lawful reviewed subset — an invariant-free refusal by the repository's own strictness rule | Workspace staged membership is the same decomposition as commits |
| WSS-R2-09 | ACCEPTED | Judging toplevel staged state against toplevel HEAD blames preexisting sibling staged entries the design promised not to judge | topStagedTree anchored at admission and per turn; motion-only prefix judgment |
| WSS-R2-10 | ACCEPTED | Confirmed against state.go: fresh states are schema 3, exact key sets everywhere; "additive within v2" was a false premise | Mission state bumps to schemaVersion 4; pre-4 refuses resume via the existing barrier; fact sheet corrected |
| WSS-R2-11 | ACCEPTED | The admission origins had no durable authority; a crash before first open could silently adopt sibling state | The birth record carries initial topTree, topStagedTree, refMap beside E0 |
| WSS-R2-12 | ACCEPTED | The error split was not implementable from string-collapsing helpers (gittree.go:53-87) | Outcome-typed probes; classification by probe outcome, never strings |
| WSS-R2-13 | ACCEPTED | The conservative edge contradicted WSS-8's lawful-lane contract and the design's own strictness argument | SnapshotSeeded: every in-turn snapshot seeds from its comparison target, eliminating the edge rather than tolerating it |
