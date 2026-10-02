# work-review-starts-its-critic

- Owner: the ui seat (headless seat session started by the steward, 2026-10-02 23:05), branch `goal/work-review-starts-its-critic` in the goal worktree `agentic-tools-ui-work-review-starts-its-critic`, pushed to origin.
- Goal and current status: the fix is built and was read twice by the code critic (Sol on Codex). It cannot land from this seat: the review of a commit cannot close (see "Waiting on the human"). Unit commit `91826da58a4b886b192ff83b57baefc501c7c0fe`, two files: `metasystem/internal/goal/branch/read.go` and `read_test.go`. Fast gate green, package tests green, package coverage 80.0 percent (floor 77.9).
- In flight right now: nothing runs. A question to Wido is recorded (`TGKJ5VPN0KZ63TD6EW1C1R2JQ8`) but was not sent, because this repository has no channel; the same facts went to the m1e seat (message `d-3a9014079096b4949424eed92b`).

## What was wrong (reproduced live on 2026-10-02, with this goal's own review)

1. `RunBranchRead` saves the review request as "dispatch pending" before it asks the delegate boundary to start the critic. Only three named refusals moved it on. Any other refusal left it pending with no critic job, and every repeat was refused with "whether the review's critic started isn't known yet", which also hid the first refusal's cause.
2. The refusal that strands it, on 1 October and tonight: `work review G` starts the critic from the goal's worktree, where no census exists (`dispatch refused: census verdict is absent; run metasystem system start --repo <goal worktree>`). `work review --commit SHA --goal G` from the primary checkout starts it.

## What the fix does

A pending request with no critic is settled from the critic job records, under the read lock. The delegate boundary writes a job record before it starts a process and names it after the request (goal, goal revision, frozen brief). A launchable code-critic root whose job id is the one derived from the request's frozen brief is adopted. None means this request started no critic: the request becomes retryable and the frozen brief is dispatched again, and the caller says so with the cause. Another caller's examination of the same commit is never adopted. A reservation that ended in setup is not a critic. Unreadable job records keep the request pending.

Seen live: the installed engine left this goal's own request pending and repeated "isn't known yet"; the engine built from the fix settled it, printed the census refusal, and a repeat from the primary checkout started the critic.

- Decisions made (and who made them):
  - No design record; the brief is the specification (the seat, because the goal asks for a narrow fix).
  - Critic 1 (`code-critic-8a01ed853ad14d5284e06166`, commit 65fc5411c) finding F-1, material: adoption by goal and commit alone can attach another caller's examination. Accepted and fixed (the seat): adoption is bound to the job id derived from the frozen brief.
  - Critic 2 (`code-critic-dcf62a8f8488c42ce3e1834d`, commit 91826da58) finding F-1, material, severe: the id check contradicts the brief. Refuted with evidence (the seat): the engine froze the unit run's first brief into that review, not the correction brief; the id check is the fix critic 1 required. The decisions file is `decisions-91826da.md` in the evidence folder.
  - Two tests that pinned the old rule (a pending request is never dispatched again) were rewritten to the new one. This is a contract change the goal asks for.
- Waiting on the human (open escalations, reviews, reserved decisions):
  - How this lands. The seat's recommendation: m1e lands commit 91826da58 by hand, as it did for reviewers-check-the-rulings on 2 October. The alternatives: widen this goal to the walls below, or park.
  - Critic 2's finding stands as severe in the engine's eyes until another critic round resolves it or a person accepts the risk (`metasystem goal accept-risk`).
  - Whether the walls below become goals. They are not this seat's to open.
- Walls on the tier 2 path that this goal does not fix (each seen tonight):
  1. A commit review cannot close: `cannot close a critic chain whose register is folded through round 0 while terminal round 1 exists`. Nothing in `cmd/metasystem` folds a single-round commit review into its findings register before the close; the lifecycle folds only before a follow-up round. Same refusal as on 2 October with a clean review.
  2. `work review G` starts the critic from the goal worktree, where the census is absent (above). With the fix it fails cleanly and repeatably; it still does not start.
  3. The unit commit `work review G` makes is refused by the commit hook (`HEAD names no branch`): the commit token is written under the installation, the hook looks for it under the scratch worktree the commit runs in. Made by hand with the same tree and the `Goal-Unit` trailer; the review then adopts it.
  4. The review of a corrected attempt is given the unit run's first brief, not the correction brief, so a correction that changes the specification is judged against the old one.
  5. `work build --check` runs from the worktree root, where there is no Go module; the check is frozen in the request, so work item `main` of this goal is stuck red and work item `settle` carries the goal. Use `go -C metasystem test ...`.
- Dead ends (do not retry without new evidence):
  - Repeating `work review ... --dispositions` to close: it stops at the unfolded register every time.
  - `work review G --work settle` from this seat: the census refusal, every time.
  - `metasystem question ask` from this repository: recorded, never delivered (no channel).
- Residual risk in the fix: a launch refused after its job record was reserved but before it became launchable is retried under the same job name while the goal revision is unchanged, and the delegate boundary answers that it already failed; the request then fails cleanly each time until the goal's revision changes. A review started from one installation root and settled from another is not seen (job records are per root).
- Next step: on Wido's or m1e's answer, either hand over for the hand landing and conclude through the person, or widen. Evidence: `~/metasystem-evidence/agentic-tools-ui/work-review-starts-its-critic-20261002/` (briefs, command outputs, decisions); critic returns under `metasystem/artifacts/agents/code-critic-*/rounds/1/return.json`.
