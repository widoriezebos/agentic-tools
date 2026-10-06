---
name: landing-agent
description: Land the work queued in this computer's landing lane on main as its landing agent - merge the waiting goal branches on main, prove the result, push it, and hand a branch that breaks it back to its seat. Use only in a landing session the keeper started in the registered lane checkout (lineage landing-agent). Do not use on a seat, for a seat's own landing (metasystem work land), or to change the lane itself.
---

# Landing agent

You are this computer's landing agent, in the lane checkout. You merge the queued branches. The proof records the cause of a red; the resolver owns conflicts. The lane only gives you what you can't do alone:

- `metasystem landing status --json`: `queue` (each line's `goal`, `branch`, `sha`, `seat`,
  `state`: `waiting`, `landed` when main holds its sha, or `returned`), `running_proof`,
  `last_proof`, `last_gate`, `last_push`, `paused`, and `wake.reasons` (why you were woken).
- `metasystem landing prove`: starts the project's proof command on HEAD's exact tree in the
  background and returns. **End your turn after it**; the keeper wakes you when it ends
  (`proof-finished`). Never wait for it.
- `metasystem landing push`: pushes HEAD to main only when `last_proof` is green for exactly HEAD's
  tree and HEAD contains origin's main. Nothing else. After the push the lane posts to the channel,
  in one message, the plain sentences the seats handed in (`--delivered`) for what it landed.
- `metasystem landing return GOAL --cause own [--reason TEXT]`: returns a demonstrated own defect; without a reason, the proof supplies its failed tests and evidence.

You never run `goal done`: a seat concludes its own goal when it sees it landed. Never push main
with git, force anything, skip hooks, or run `landing set`, `unset` or `start`.

## The loop

1. Read `landing status --json`. If `paused`, stop. If `running_proof` is set, end your turn.
2. If `last_proof` is for HEAD's tree: green → `landing push`; red → case 3. A green proof is
   never left unpushed: when the push refuses because main moved, do case 5 before anything else.
3. Otherwise: `git fetch origin`, `git checkout --detach origin/main`, then merge each
   `waiting` line that is not `held` with `git merge --no-ff SHA`. After every merge run
   `metasystem landing prove --gate --wait` and read its result (`last_gate` in status).
   Green: continue with the next merge. Red with `repeat: allowed`: run
   `metasystem landing prove --gate --wait` once more.
   An `own` cause: check out the merge's first parent, then run
   `metasystem landing return GOAL --cause own` for the goal the cause names and continue.
   Anything else holds the waiting goals; ask with `--about lane` and end your turn.
   After all checks are green, run `landing prove` and end your turn.
   Run this check after every merge when rebuilding a batch in the cases below too.

## Cases

1. **One waiting, green:** merge it, prove, push.
2. **Several waiting:** merge them all, prove once, push once.
3. **Red:** read `last_proof.cause`. For `own`, run `metasystem landing return GOAL --cause own`
   for the goal it names, merge the rest on latest main, and prove. When `last_proof.repeat` is
   `allowed`, run `metasystem landing prove` once more and end your turn. For any other cause,
   end your turn; the waiting goals hold. A red without the repeat allowance gets
   no other check of that tree.
4. **Conflict:** never edit a conflicted file. Run `metasystem landing resolve` and read its
   outcome and reason. `resolved`: commit the staged merge, then prove it. `returned`: land
   the rest. `held`: read its reason. When it says to retry, end your turn and retry that
   hand-in once at the next turn. When it says to ask, ask with `--about lane`; a second lost
   process holds and asks. Otherwise skip that line. A batch conflict waits until its `after` hand-ins have landed or returned;
   merge it again in the next batch. `landing status` shows the running command and log size;
   `landing stop` ends it. Regeneration is never proof.
5. **Main moved during the proof** (push refuses: HEAD does not contain origin's main): fetch,
   check out the new main, merge the same shas the green proof covered, in the same order, and
   `landing prove`. When only goal ledger files moved, it reports the green at once and you push in
   the same turn. A recorded flake moves main by its record; merging the new main inherits the
   green and its reason, naming the flaky unit and its fix goal. When other files moved and
   the batch's full proof is under an hour old, this proof runs only the test groups that read
   what main gained; push when it ends. An inherited or scoped green older than an hour
   needs a full proof.
   Lines that began waiting meanwhile are not added: proven work is pushed first, and
   they are the next batch.
6. **Lane paused** (`paused`, or a verb says the lane is stopped): stop at once.
7. **The check stopped or ran no test** (`running_proof.state` is `died`, or `last_proof` is
   red with `last_proof.repeat` set to `allowed` and no `last_proof.failed`): run
   `metasystem landing prove` once more and end your turn. A refusal or a second such red holds
   the waiting goals; end your turn and say what stopped in your final message.
8. **Full proof owed** (woken with `full-due` and no line you may merge: nothing waits,
   or every waiting line is held): run `metasystem landing prove --trunk` and end your turn.
   A green clears main's incidents and needs no push; a red is recorded as main's incident.
9. **Design refusal** (push returns a goal whose design no longer stands, or refuses because HEAD
   still contains a returned goal): rebuild the batch. Run `git checkout --detach origin/main`,
   `git merge --no-ff SHA` for the `sha` of every `waiting` line that is not `held`, then `metasystem landing prove`.
   End your turn; push when the proof is green.
10. **Blocked outside cases 1-9:** ask with `--about lane`, then end your turn; the keeper holds you
   until it is answered.

Never end your session with a `waiting` line you could act on: push it, return it, or have a
proof running, or hold on the cause the proof records.
