# race-gate-red-on-main, critique round one: dispositions

Chain rgr-build1, critic rgr-critic1 (claude, claude-fable-5-1, xhigh),
reviewed tree d910ae5b41f8ea20f4c1b79e02a9f41f46618f10. The critic had
no shell; the orchestrator ran the six focused evidence commands on the
reviewed worktree itself and they all passed (refusal package ok, the
two race tests ok with no race report, vet clean, gofmt clean, gate
script syntax ok, five files changed).

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | accepted | The gate comment's 12-second per-test bound is false: the mission-runner package's slowest tests take 19 seconds standalone under the race detector (orchestrator measurement 2026-09-06, missionrunner package 414 s over 296 tests). The figure came from the build brief, written before that measurement finished. | Folded in round two (plans/race-gate-red-on-main-fold-r2-brief.md): the comment states the 19-second bound. |
| F-2 | noted | Not material: the sentence rests on the orchestrator's full-gate run, which measured the slowest packages at about ten minutes under contention on m1 and saw them pass thirty minutes on m2. The wording is replaced in the round-two fold anyway because the same comment is edited. | Folded in round two: the sentence states what is known without unseen measurements. |
| F-3 | noted | Not material and outside the round-one May-touch list; true as fact. The coverage-delta.sh comment mirrors a ceiling the gate no longer carries. Fixed as a direct consequence of this change rather than backlogged. | Folded in round two: coverage-delta.sh's comment states its own reason; its timeout value is unchanged. |
