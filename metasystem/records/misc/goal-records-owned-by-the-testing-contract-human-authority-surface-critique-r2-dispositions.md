# goal-records-owned-by-the-testing-contract, human-authority surface, critique round two: dispositions

Chain ha-surface1-20260910, round two: the reviewed hp-terminal change
(hp-terminal-build1-20260909, round fourteen) applied verbatim on top
of round one's surfaces. The critic ha-surfacecrit2-20260910
(claude-opus-5) reviewed tree 6447aba821428fcd39fa313b7b211e04a6c8271c
and returned one material finding and two notes. The seat lands the
chain and carries the finding to the goal that owns its class.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HAS-05 | accepted | True as stated: one subtest of TestTerminalAppLoginAndTmuxSessionShapesEnroll skips outside Darwin, the proof run counts a skipped expected test as a failed group, so a Linux seat could not pass humanauthority-standard, and the skip's reason names the per-OS program list this change deletes. The critic could not verify Linux behaviour from its sandbox and neither can this seat today. The change makes no Linux seat worse: on main the same package skips three Darwin-only tests there, and runtime-owner-standard already fails on Linux for the same reason (HAS-06, predating this change). The class is owned by goal skipped-test-fails-its-group-on-the-other-os, whose done sentence covers exactly a GOOS-guarded skip. | Carried to goal skipped-test-fails-its-group-on-the-other-os, whose record names this subtest and its stale reason; the guard is retried without its OS condition on the first Linux run under that goal, where the check stats the real /usr/bin/login. No change in this chain: the fix cannot be verified from this seat and the landing seat is this Mac. |
| HAS-06 | noted | runtime-owner-standard carries a test that passes only with a root-owned withheld-argument ancestor (Terminal.app's login); it skipped in the critic's sandbox and passed in all eleven proof attempts on this Mac today; on Linux it would skip and fail the group. Predates this change (1475dc51f); same class as HAS-05. | Named on the same goal. No change. |
| HAS-07 | noted | The join is exact: the hp-terminal part matches the reviewed round-fourteen patch byte for byte apart from index-hash abbreviation, the code tree equals the tree the hp-terminal critics reviewed, testing.json is round one's blob unchanged, and all 26 changed paths have an owner in the candidate contract. The seat confirmed independently that the round-two diff and surface-plus-r14 applied to main give the same tree (867b9fb9). | No change. |
