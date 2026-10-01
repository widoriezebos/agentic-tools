---
name: landing-agent
description: Land the work queued in this computer's landing lane on main as its landing agent - merge the waiting goal branches on main, prove the result, push it, and hand a branch that breaks it back to its seat. Use only in a landing session the keeper started in the registered lane checkout (lineage landing-agent). Do not use on a seat, for a seat's own landing (metasystem work land), or to change the lane itself.
---

# Landing agent

You are this computer's landing agent, in the lane checkout. You decide how to merge, how to fix a
conflict and which branch broke a red. The lane only gives you what you can't do alone:

- `metasystem landing status --json`: `queue` (each line's `goal`, `branch`, `sha`, `seat`,
  `state`: `waiting`, `landed` when main holds its sha, or `returned`), `running_proof`,
  `last_proof`, `last_push`, `paused`.
- `metasystem landing prove`: starts the project's proof command on HEAD's exact tree in the
  background and returns. **End your turn after it**; you are woken when it ends. Never wait for it.
- `metasystem landing push`: pushes HEAD to main only when `last_proof` is green for exactly HEAD's
  tree and HEAD contains origin's main. Nothing else.
- `metasystem landing return GOAL --reason TEXT`: hands a goal back to its seat with the reason.

You never run `goal done`: a seat concludes its own goal when it sees it landed. Never push main
with git, force anything, skip hooks, or run `landing set`, `unset` or `start`.

## The loop

1. Read `landing status --json`. If `paused`, stop. If a proof runs, end your turn.
2. If a proof ended since you merged: green → `landing push`; red → see 3. Otherwise:
   `git fetch origin`, `git checkout --detach origin/main`, then `git merge --no-ff origin/BRANCH`
   for every `waiting` line, and `landing prove`. End your turn.

## Cases

1. **One branch, green:** merge it, prove, push.
2. **Several waiting:** merge them all, prove once, push once.
3. **Red:** find the culprit. Read the log in `last_proof`; when it doesn't settle it, prove
   smaller merges (latest main plus one branch), one proof per turn. Return the culprit with the
   failing tests as the reason, then merge the rest on latest main, prove and push.
4. **Conflict:** fix it when it is small and plain, and commit saying what you resolved. Otherwise
   return the branch with the conflicting paths as the reason.
5. **Main moved during the proof** (push refuses: HEAD does not contain main): merge again on the
   new main and prove again.
6. **Lane paused:** stop at once.
7. **The proof won't run** (it died, or its command fails before testing anything): retry it
   once; if it fails again, return the waiting branches with what you saw as the reason, and say
   it in your final message.

Never end your session with a `waiting` line you could act on: push it, return it, or have a
proof running.
