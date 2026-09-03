# first-headless-run

- State: queued
- Intent: Run the metasystem headless for real: on m0, with no Claude session on top, the mission runner (family metasystem mission, cmd/metasystem/mission.go, so far exercised only through its fake host in scripts/agents/dispatch-fixtures.sh) picks one approved tier-2 goal, composes its brief, dispatches the build and the review, folds or defers findings, lands the result and talks to Wido through the fleet conversation channel where a severe finding needs his word; every defect the run surfaces is fixed forward as its own tier-1 or tier-2 item the same day; the run is recorded, and its landing on its own is the proof that the machines can be headless.
- Origin: main
- Next step: APPROVED FOR EXECUTION by Wido, R-66-m1 (verbatim: 'open the first headless run as a backlog item, approved for execution, on m0 tonight after its approval feature lands. Yes!'). TIER 3 per R-54-m1 (a new seam: the runner on a real host). Runs on m0 the moment human-approval-for-execution lands; m0 claims it then. Steps: (1) find the runner's real entry point and what it needs to start on a host (lease, mission id, the host adapter that replaces the fake host), and write the one-page run plan into plans/first-headless-run-plan.md; (2) pick the goal to run: an approved tier-2 goal with a landed design and a build brief, for example adoption-inventory-from-install-set or merge-stage-critic-close once Wido approves either; (3) stop the Claude session on m0, start the runner from the shell, watch only from outside (census, the channel, the ledger); (4) each stop is a finding: fix forward, restart, until the goal lands with a chain landing signed by the runner; (5) record the run (what it needed a person for, what it did alone) in records/misc/first-headless-run.md and open the follow-ups that make up starting the runner and the fleet join bootstrap.
- OpenedAt: 2026-09-03T12:10:50Z
- Revision: 1

History:
- 2026-09-03T12:10:50Z Y3RZW9JRD4KE9WTRF7TFX4SX73-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=first-headless-run
Integrity: sha256=1351de5bb13ed0cb8b6de0d58224cde47b07c073feda57326d8bd4dc1a0c022a
