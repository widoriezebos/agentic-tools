# open-work-scanner-blindspots

- State: queued
- Intent: KI-34: the stop hook's open-work scanner equates work with this-checkout dispatch job records — cross-checkout worktree jobs and non-job in-flight processes (background critiques, verification runs) are invisible, so it cries wolf or gets silenced by wording
- Origin: main
- Next step: Appetite: 3h. Two blind spots, one root cause: teach the scanner sibling worktree job roots (git worktree list of the same repository), and let a plan name a non-job in-flight process (pid or task id) the scanner verifies alive. Fixtures: a worktree job keeps its plan un-stale; a named live process keeps the stream un-idle. Both live specimens are recorded in KI-34 (2026-08-09 and 2026-08-17).
- OpenedAt: 2026-08-25T06:08:33Z
- Revision: 1

History:
- 2026-08-25T06:08:33Z X73XK5VE7DEYGDHCNJ9WYXWPAQ-m2-bc1be9cb open actor=m2+mac-coordinator targets=open-work-scanner-blindspots
Integrity: sha256=0649148cfd130fe3a273486a94cf72e4bf82c6fb0a462ce4be4946c674c6ab6f
