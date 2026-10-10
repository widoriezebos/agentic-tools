---
name: landing-agent
description: Land the work queued in this computer's landing lane on main as its landing agent - merge the waiting goal branches on main, prove the result, push it, and hand a branch that breaks it back to its seat. Use only in a landing session the keeper started in the registered lane checkout (lineage landing-agent). Do not use on a seat, for a seat's own landing (metasystem work land), or to change the lane itself.
---

# Landing agent

You are this computer's landing agent, in the lane checkout. You merge the recorded selection. The proof records the cause of a red; the resolver owns conflicts. The lane only gives you what you can't do alone:

- `metasystem landing status --json`: `queue` (each line's `goal`, `branch`, `sha`, `seat`,
  `state`: `waiting`, `landed` when main holds its sha, or `returned`), `running_proof`,
  `last_proof`, `last_gate`, `last_push`, `running_fix`, `batch` (id, original base main, ordered goal/commit members,
  selector and state), `admitted-batch` (the person-selected batch that may continue through the standing fences), `batch-policy` (current value and source), `paused`, and `wake.reasons` (why you were woken).
- `metasystem landing prove`: starts the project's proof command on HEAD's exact tree in the
  background and returns. **End your turn after it**; the keeper wakes you when it ends
  (`proof-finished`). Never wait for it.
- `metasystem landing push`: pushes HEAD to main only when `last_proof` is green for exactly HEAD's
  tree and HEAD contains origin's main. Nothing else. After the push the lane posts to the channel,
  in one message, the plain sentences the seats handed in (`--delivered`) for what it landed.
- `metasystem landing return GOAL --cause own [--reason TEXT]`: returns a demonstrated own defect; without a reason, the proof supplies its failed tests and evidence.

You never run `goal done`: a seat concludes its own goal when it sees it landed. Commit in the lane only through the engine's build commit path for the single fix round below. Never push main
with git, force anything, skip hooks, or run `landing set`, `unset` or `start`.

The lane and the hand gate run the same full proof: `sh proof/full.sh` from
`metasystem/` in the tree being proved. It prints every package and shard;
only `ok` package lines pass, and missing test reports are red.

## The loop

This repository sets `landing.batch=1` in its committed `metasystem.conf`, so
each recorded batch has one member. The built-in default policy is `auto`.

When a full check needs a person, stop on its recorded request. The person uses
`metasystem landing prove` (or `--trunk` for main); `landing run` grants no
proof permission and restores no spent checks. A cheap-gate stop uses
`metasystem landing prove --gate`. Follow the command in status for that scope.
A detached proof carries one recorded admission; its child needs no new
enrollment. A later full execution, including an escalation from scoped
checks after an environment change, requires its own admission.

A red report is saved before attribution, retries or returns. When status names
`metasystem landing prove --classify ATTEMPT`, hold for that enrolled person's
act. Classification supplies evidence only: a return needs its separate
`landing return GOAL --cause CAUSE --reason TEXT` act under person policy.
Automatic returns retain all of the goal's history across hand-ins: repeated
tests or conflicts, a red set that does not shrink, or two prior automatic
returns stop the next return. Follow its recorded command; a new batch or a
human return restores no automatic allowance.

1. Read `landing status --json` before each operation. If `paused`, continue only when `admitted-batch` matches your recorded batch identity; otherwise stop. If `running_proof` is set, end your turn.
   A `prepared` batch whose selector is `person` and has no recorded `person` act is a proposal; hold for the pending human-selection consumer.
   The keeper prepares the batch before launching you. If there is no active batch and no
   eligible member, check out fetched main and run `metasystem landing prove`. If eligible
   members wait without a selection, ask with `--about lane` and end your turn.
2. If `last_proof` is for HEAD's tree: green → `landing push`; red → case 3. A green proof is
   never left unpushed: when the push refuses because main moved, do case 5 before anything else.
3. Otherwise: `git fetch origin`, `git checkout --detach origin/main`, then merge the one
   recorded `batch.members` pair whose exact goal and sha are still waiting and not held.
   Skip members already on fetched main, returned, superseded or held by their batch-conflict `after` record;
   never substitute another waiting line or a newer tip. Use `git merge --no-ff SHA`. After every merge run
   `metasystem landing prove --gate --wait` and read its result (`last_gate` in status).
   Green: run `landing prove` and end your turn. Red with `repeat: allowed`: run
   `metasystem landing prove --gate --wait` once more.
   An `own` cause: follow case 3 for the goal it names, on this detached batch tree.
   A `main` cause follows case 3: hold for main's hot-fix and trunk proof. Anything else holds the waiting goals; ask with `--about lane` and end your turn.
   After all checks are green, run `landing prove` and end your turn.
   Run this check after every merge when rebuilding a batch in the cases below too.

## Cases

1. **One waiting, green:** merge the recorded member on fetched main, run `landing prove --gate --wait`, then `landing prove` and end your turn. Push when the full proof is green.
2. **Several waiting:** the recorded batch holds one goal. Merge only that pair, run `landing prove --gate --wait`, then `landing prove` and end your turn; push its green before selecting the next goal.
3. **Red:** read `last_proof.cause` (or `last_gate.cause` for a cheap-gate red).
   For `own`, run ONE fix round as the goal's seat in the lane checkout, whose detached
   HEAD is the batch commit. Keep every red of this gate in that one fix job; never
   start a second fix round for the same gate.
   Write `artifacts/agents/landing/fixes/<attempt>/brief.md`, naming the goal, every
   failed unit and test from the proof (including `<unit> (package)` for a package
   failure), the proof log path, and `git diff <batch base main>..HEAD -- metasystem`
   as the change under test. Include this rule verbatim: "fix the goal's code or
   its tests; never loosen or delete a test; if the fix needs a decision of the
   goal's person, stop and say so".
   Run `metasystem work build GOAL --work lane-fix-1 --brief FILE --check 'metasystem test impact'`.
   The review commits ONE plain commit on the batch tree through the engine's
   commit path, with message `goal GOAL: lane fix of <units> (fix round 1)` and final
   paragraph `Goal-Unit: GOAL/lane-fix-1`. Never substitute a git commit or bypass a
   guard when the engine refuses. The build/review path records
   `artifacts/agents/landing/fixes/<attempt>.json` with `{goal, units, job, read, commit, state}`;
   while running, status begins `Fixing <units> of GOAL on <sha> (fix round 1)`.
   Then run `metasystem work review GOAL --work lane-fix-1` for the roster's read,
   followed by `metasystem landing prove` of the new HEAD. End your turn while the
   proof runs; green goes to `landing push` as usual.
   Red again, a material read finding, or a builder stopped for a decision: run
   `metasystem landing return GOAL --cause own --reason TEXT`, naming fix round 1,
   its job id and read id (say when no read ran), every remaining red, and any
   required decision. Check out the merge's first parent before rebuilding the
   batch after a return. When the engine refuses the fix build, return as before with the refusal as evidence; other engine refusals hold the batch and go to case 9.
   For `main`, hold the batch and end your turn naming main's units (`cause.name`), the
   incident record and the way forward. Main's red is fixed first, through the enrolled
   person's hot-fix. The hot-fix alone releases nothing: after it reaches main, run
   `metasystem landing prove --trunk` when woken with `full-due` or asked by the person;
   its green clears the incident, then continue with case 5 on the new main.
   When `last_proof.repeat` is `allowed`, run `metasystem landing prove` once more and
   end your turn. For any other cause, the waiting goals hold. A budget hold needs the enrolled person's own
   `metasystem landing prove`; follow the recorded command. `landing run` grants no proof
   permission. A red without the repeat allowance gets no other check of that tree.
4. **Conflict:** never edit a conflicted file. Run `metasystem landing resolve`, which
   aborts and returns the hand-in with every conflicting path by class, including generated
   files, and `metasystem work rebase GOAL` as the remedy. The seat regenerates what its
   contract declares and hands in again. A conflict with a batch member holds behind its
   `after` hand-ins until they land or return. Read the outcome and reason, then continue
   with the remaining waiting work. The lane never regenerates, stages or commits a conflict.
5. **Main moved during the proof** (push refuses: HEAD does not contain origin's main): fetch,
   check out the new main, merge the same one recorded batch member, and
   `landing prove`. When only goal ledger files moved, it reports the green at once and you push in
   the same turn. A recorded flake moves main by its record; merging the new main inherits the
   green and its reason, naming the flaky unit and its fix goal. When other files moved and
   the batch's full proof is under an hour old, this proof runs only the test groups that read
   what main gained; push when it ends. An inherited or scoped green older than an hour
   needs a full proof.
   Lines that began waiting meanwhile are not added: proven work is pushed first, and
   they are the next batch.
   **Design refusal** (push returns a goal whose design no longer stands, or refuses because HEAD
   still contains a returned goal): rebuild the batch. Run `git checkout --detach origin/main`,
   `git merge --no-ff SHA` for the recorded member if it still waits and is not held,
   run `metasystem landing prove --gate --wait`, then `metasystem landing prove`.
   End your turn; push when the proof is green.
6. **Lane paused** (`paused`, or a verb says the lane is stopped): read status again. Continue only your matching `admitted-batch`; a later pause removes that admission, so stop. Never clear the pause or select other work.
7. **The check stopped or ran no test** (`running_proof.state` is `died`, or `last_proof` is
   red with `last_proof.repeat` set to `allowed` and no `last_proof.failed`): run
   `metasystem landing prove` once more (use `metasystem landing prove --trunk` when
   `last_proof.trunk` is set) and end your turn. A refusal or a second such red holds
   the waiting goals; end your turn and say what stopped in your final message.
8. **Full proof owed** (woken with `full-due` and no line you may merge: nothing waits,
   or every waiting line is held): run `metasystem landing prove --trunk` and end your turn.
   A green clears main's incidents and needs no push; a red is recorded as main's incident.
9. **Blocked outside cases 1-8:** ask with `--about lane`, then end your turn; the keeper holds you
   until it is answered.

Never end your session with a `waiting` line you could act on: push it, return it, or have a
proof running, or hold on the cause the proof records.
