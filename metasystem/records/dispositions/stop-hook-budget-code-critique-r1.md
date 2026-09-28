# Dispositions: shbo-cc1-20260906, round 1

Chain under review: shbo-build1-20260906 (reviewed tree
6871cd99e6eccc1e8a6f45be3296294967bfb476, the correction round
shbo-build1-20260906-r2). Critic: shbo-cc1-20260906, one material
finding. Orchestrator: m1d.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SHB-01 | accepted | Read the flag parser in metasystem/cmd/metasystem/steward_verbs.go: an engine built before this change refuses the unknown flag with exit 2, and the hook swallows the failure, so a seat that pulls the script before rebuilding its engine loses hook completion on every Stop until it rebuilds. The round-one brief promised that neither pairing breaks completion; the optional flag covered only the other one. | Follow-up round 3 per metasystem/plans/stop-hook-budget-fold2-brief.md: when a flagged completion call exits 2, the hook retries it once without the flag. |
| SHB-02 | out-of-scope | True as read: the deadline parent's running check trusts ps, and a ps that shows nothing for a live worker makes the parent wait without a deadline. The diff did not introduce it and the review brief's threat model does not name it. Recorded as goal stop-deadline-parent-trusts-ps. | none |
| SHB-03 | accepted | Read metasystem/internal/steward/component_evidence.go: a same-key retry reuses the loaded record and leaves the previous completion's elapsed seconds on an attempting record, so a mid-attempt reader sees a number from an earlier Stop. Cheap to fix and it changes what slice 2 reads. | Round 3: the same-key retry clears the field, as the interrupted closure does. |
| SHB-04 | noted | The paths that write no hooks-log line predate the diff and have nothing to measure or nowhere to write; the brief asked for elapsed on every line, not a line on every path. | none |
| SHB-05 | accepted | The digit-only assertions cannot tell a parent-start measurement from a worker-start one. The deadline scenario can: only the parent's start yields fifty-something seconds there. | Round 3: the deadline assertion binds the elapsed to the range 50 through 59. |
| SHB-06 | accepted | Two comments in metasystem/internal/goal/project.go still justify a four-second projection timeout from a five-second provider budget; a landed number that contradicts a comment is the drift this goal exists to end. Comment-only, two sites. | Round 3: the two comments say the budget is sixty and where it lives; no code change in that file. |
| SHB-07 | accepted | The round-two return claimed an interrupted-history test the diff does not contain, and no test drives the verb with a negative flag value. Certification is not damaged (the brief's named tests exist); the claim is corrected by adding what was claimed. | Round 3: the interrupted-history test and a command-level test that a negative value exits 2. |
