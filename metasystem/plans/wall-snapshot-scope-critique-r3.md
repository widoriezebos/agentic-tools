# Snapshot-scope critique, round 3 dispositions

Round 3 verdict: 10 material findings (trajectory 12, 13, 10), one of
them the invited shape finding. All ten adjudicated; the round's net
effect is a SIMPLER design — the semantic ref-tip lane is cut, not
patched.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| WSS-R3-01 | ACCEPTED (shape) | O15/O16 name no lawful arbitrary host ref motion; judging tips semantically reopened namespace, lifecycle, anchor-integrity, and sibling-scope gaps each round, and pseudorefs (ORIG_HEAD/FETCH_HEAD resolve here yet for-each-ref lists neither) escaped it entirely | Rule 5 becomes the exact record-bound transition fence; pseudorefs absent-or-accounted; anchor-ref deletion violates; no accounted-tip lane survives |
| WSS-R3-02 | ACCEPTED | The acceptance write follows drain and arbitrary-bash measurement (loop.go:1267, measure.go:271); a recheck before measurement gates nothing | The posture re-verifies immediately before the acceptance write; a gate that mutates carriers voids the pass |
| WSS-R3-03 | ACCEPTED | A stability token of HEAD+refs alone accepts cross-instant captures when a peer mutates index or worktree between samples | The capture and re-verification cover every carrier |
| WSS-R3-04 | ACCEPTED | Birth recorded topTree/topStaged/refMap but no headCommit; a detached switch before first open had no origin | Birth records headCommit |
| WSS-R3-05 | ACCEPTED | A lawful staged subset at conclusion differed from both headTree and preTree at the next open — a false park with zero motion | stagedTreePost joins the acceptance payload; opens judge motion from it |
| WSS-R3-06 | ACCEPTED | git write-tree on the live index rewrites the cache-tree extension and takes index.lock — the observer-only contract was false | StagedTree computes on a copied index file |
| WSS-R3-07 | ACCEPTED | write-tree requires a fully merged index; a preexisting sibling conflict would refuse a clean workspace at admission | The toplevel staged posture is the ls-files --stage serialization, motion-judged; conflicts representable, never blamed |
| WSS-R3-08 | ACCEPTED | A force-added committed ignored declared artifact vanishes from an expected-seeded projection and reads as drift next open | SnapshotSeeded takes declaredPaths as forced membership |
| WSS-R3-09 | ACCEPTED | GIT_REPLACE_REF_BASE redirects the replacement namespace; anchor.go and gate invocations run unpinned | Admission refuses the EFFECTIVE namespace; every runner git surface pins useReplaceRefs=false and scrubs the environment |
| WSS-R3-10 | ACCEPTED | Resolution names a tree while violations live in five carriers; adoption/restore had no coherent post-resolution origin | Resolution entries record the full carrier posture; RESTORE refuses while non-restorable carriers fail accounting; WSS-13 |
