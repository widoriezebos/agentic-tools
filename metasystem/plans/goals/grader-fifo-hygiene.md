# grader-fifo-hygiene

- State: claimed
- Intent: Leftover ACP fifos in job rounds directories break the benchmark grader's evidence copy; bm-2dc rep-2 grading is parked on it (KI-42)
- Origin: main
- Next step: Kit-side first: grader/extractor skip non-regular files with a logged note (unblocks cohort bm-2dc-20260824t092850z-86646 without re-running the mission), then adapter-side hygiene: devin.sh acp path removes its fifo pair at round close. Verify with validate-kit.sh plus regrading the parked rep; then resume the cohort to completion and extraction.
- OpenedAt: 2026-08-24T11:41:00Z
- Revision: 2
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-25T06:00:52Z

History:
- 2026-08-24T11:41:00Z H8X6TPNNNHF9AVXSR82R0M5NMF-m2-bc1be9cb open actor=m2+mac-coordinator targets=grader-fifo-hygiene
- 2026-08-25T06:00:52Z F47FYWABAMCY8NH0FW8MCMR7FA-m2-bc1be9cb claim actor=m2+mac-coordinator targets=grader-fifo-hygiene
Integrity: sha256=4b3278352ca7135d29331565e0fce6128a6c12f7c73782383867d8596727b693
