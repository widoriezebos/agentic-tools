# Code critique brief: discard-launch (Sol)

You are the code critic (Codex, Sol). Review branch `discard-launch` in the worktree
`/Users/wido/LocalStorage/GitHub/agentic-tools-discard`. The diff is
`git diff origin/main...HEAD -- . ':!metasystem/plans' ':!metasystem/internal/ui/web/bundle*'`
(skip the built bundle). This is read-only: don't edit, commit, or run anything that
writes outside a scratch directory, and don't start servers on 127.0.0.1:7878. Never
read any `metasystem.conf.local`.

- **Spec:** `metasystem/plans/discard-launch-build-brief.md`. The build report, with its
  deviations, is `metasystem/plans/discard-launch-build-report.md`. Judge each deviation
  on safety and intent: the checkout hand rather than a session, refusing a running
  launch, and an older card surfacing after a discard.
- **Materiality (R-124):** material only if the slice doesn't WORK (the card comes back
  after a reload, the discard is lost, a refusal is shown wrongly) or isn't SAFE (a route
  reachable cross-origin, a write outside the record, a file deleted, a record corrupted
  by a race with the running launch verb). Everything else is deferred.
- **Budget:** 40 tool calls, one round.
- **Return:** findings `SOL-DL-NN`, each with severity, evidence (path:function) and the
  smallest fix; then Deferred. End with exactly one line: `VERDICT: <n> material`.
