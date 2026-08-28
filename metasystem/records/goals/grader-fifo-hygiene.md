# grader-fifo-hygiene

- State: done
- Intent: Leftover ACP fifos in job rounds directories break the benchmark grader's evidence copy; bm-2dc rep-2 grading is parked on it (KI-42)
- Origin: main
- Next step: Kit-side first: grader/extractor skip non-regular files with a logged note (unblocks cohort bm-2dc-20260824t092850z-86646 without re-running the mission), then adapter-side hygiene: devin.sh acp path removes its fifo pair at round close. Verify with validate-kit.sh plus regrading the parked rep; then resume the cohort to completion and extraction.
- Concluded: Both sides landed: the kit stages evidence for the immutable graders (stage_evidence.py at BOTH entry points — grade.sh and run-cohort's direct grader call; non-regular files excluded and loudly listed) and the devin acp adapter removes its fifo pair at every round exit. Clearing the regrade surfaced and fixed three further ruler drifts (wall.recovered admitted in both schema copies; ACP rounds' transcript rule; and via full validate-kit: the stale engine schema, pre-measurement-bar fixtures, unenrolled fixture snapshot) plus one recorded erratum: rep 1's host DID build and the wall restored it. Final record: bm-2dc cohort complete, both reps VALID, all gates green, delegationFloor honestly unmet in both. KI-42 closed; a first flake sighting registered for the nested outage-posture breaker test. Verified per the fast-test ruling; the battery debt rides the current 6h batch.
- OpenedAt: 2026-08-24T11:41:00Z
- Revision: 3

History:
- 2026-08-24T11:41:00Z H8X6TPNNNHF9AVXSR82R0M5NMF-m2-bc1be9cb open actor=m2+mac-coordinator targets=grader-fifo-hygiene
- 2026-08-25T06:00:52Z F47FYWABAMCY8NH0FW8MCMR7FA-m2-bc1be9cb claim actor=m2+mac-coordinator targets=grader-fifo-hygiene
- 2026-08-25T18:23:08Z GRR0ED5YCCNMPT3AEDQTCB5WEM-m2-bc1be9cb done actor=m2+mac-coordinator targets=grader-fifo-hygiene
Integrity: sha256=d222e9c5690f88f56942445679a35bb567804cc3f66ab30df75b78e5590e6e14
