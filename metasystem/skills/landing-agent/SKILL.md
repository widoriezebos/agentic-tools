---
name: landing-agent
description: Land the work queued in this computer's landing lane on main as its landing agent - merge the waiting goal branches on main, prove the result, push it, and hand a branch that breaks it back to its seat. Use only in a landing session the keeper started in the registered lane checkout (lineage landing-agent). Do not use on a seat, for a seat's own landing (metasystem work land), or to change the lane itself.
---

# Landing agent

You are this computer's landing agent, in the lane checkout. You merge the recorded selection. The proof records the cause of a red; the resolver owns conflicts. The lane only gives you what you can't do alone:

- `metasystem landing status --json`: `queue` (each line's `goal`, `branch`, `sha`, `seat`,
  `state`: `waiting`, `landed` when main holds its sha, or `returned`), `running_proof`,
  `last_proof`, `last_gate`, `last_push`, `running_fix`, `batch` (id, original base main, ordered goal/commit members,
  selector and state), `main_move` (kind `none`, `records-only`, or `code`, with the commit count), `admitted-batch` (the person-selected batch that may continue through the standing fences), `batch-policy` (current value and source), `paused`, and `wake.reasons` (why you were woken).
- `metasystem landing prove`: starts the project's proof command on HEAD's exact tree in the
  background and returns. **End your turn after it**; the keeper wakes you when it ends
  (`proof-finished`). Never wait for it.
- `metasystem landing push`: pushes HEAD to main only when the proof at the batch's depth is green for exactly HEAD's
  tree and HEAD contains origin's main. After a `records-only` move it merges the record commits
  into the proven batch, checks that only coordination paths changed, and records the folded commits.
  A conflict or a change to a unit path refuses the push and requires a rebuild. After the push the lane posts to the channel,
  in one message, the plain sentences the seats handed in (`--delivered`) for what it landed.
- `metasystem landing return GOAL --cause own [--reason TEXT]`: returns a demonstrated own defect; without a reason, the proof supplies its failed tests and evidence.

You never run `goal done`: a seat concludes its own goal when it sees it landed. Commit in the lane only through the engine's build commit path for the single fix round below. Never push main
with git, force anything, skip hooks, or run `landing set`, `unset` or `start`.

The lane and the hand gate run the same full proof: `sh proof/full.sh` from
`metasystem/` in the tree being proved. It prints every package and shard;
only `ok` package lines pass, and missing test reports are red.

## The batch

When `landing status --json` shows no batch member left waiting or running (every member landed, returned or superseded), end your turn at once — unless you were woken with `full-due` (case 8) or main owes a proof with no active batch (step 1): run that proof first. The keeper wakes the agent when there is work; a person's return of the last member ends your turn.

## The loop

Read `metasystem landing status --json` before each step. When `landing status --json` shows no batch member left waiting or running (every member landed, returned or superseded), end your turn at once — unless you were woken with `full-due` (case 8) or main owes a proof with no active batch (step 1): run that proof first. The keeper wakes the agent when there is work; a person's return of the last member ends your turn.

The built-in `auto` policy admits every eligible waiting goal until the full proof starts.
Numeric caps and person selections keep their recorded membership.

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
3. Otherwise: `git fetch origin`, then read `landing status --json` again.
   A freshly prepared batch with nothing merged yet merges its recorded members (cases 1 and 2),
   whatever `main_move` says: a freshly prepared batch with nothing merged yet always starts with
   `git fetch origin` and `git checkout --detach origin/main`, then merges its recorded members
   with this step's merge loop. "Continue the current step" applies to a step already in progress.
   Rebuild only when status reports `main_move.kind: code` or there is no batch.
   On `main_move.kind: records-only`, continue the current step: keep the pending merge,
   resolution fix record, read and running proof. On `none`, continue the current step too.
   When rebuilding, use `git checkout --detach origin/main`, then merge each
   recorded `batch.members` pair whose exact goal and sha are still waiting and not held.
   Skip members already on fetched main, returned, superseded or held by their batch-conflict `after` record;
   never substitute another waiting line or a newer tip. Use `git merge --no-ff SHA`. After every merge run
   `metasystem landing prove --gate --wait` and read its result (`last_gate` in status).
   Green or skipped: merge the next recorded member, or run `landing prove` after the last member and end your turn.
   None: the merge gate has not run for this merge; run `landing prove --gate --wait`. Red with `repeat: allowed`: run
   `metasystem landing prove --gate --wait` once more.
   An `own` cause: follow case 3 for the goal it names, on this detached batch tree.
   A `main` cause follows case 3: hold for main's hot-fix and trunk proof. Anything else holds the waiting goals; ask with `--about lane` and end your turn.
   Before the landing merge gate and before the full proof, re-read `landing status --json` and merge any member that joined the open batch since the last read, then run its landing merge gate.
   After all merge gates are green or skipped, run `landing prove` and end your turn.
   Run this check after every merge when rebuilding a batch in the cases below too.

## Cases

1. **One waiting, green:** merge the recorded member on fetched main, run `landing prove --gate --wait`, read `last_gate`: green or skipped continues to `landing prove` and ends your turn; none means the merge gate has not run for this merge. Push when the proof at the batch's depth is green.
2. **Several waiting:** merge each recorded member in order, run `landing prove --gate --wait`, read `last_gate`: green or skipped continues to the next recorded merge or `landing prove` after the last member; none means the merge gate has not run for this merge. End your turn while the batch proof runs; push when the proof at the batch's depth is green before selecting the next batch.
3. **Red:** read `last_proof.cause` (or `last_gate.cause` for a cheap-gate red).
   For `own`, run ONE fix round as the goal's seat in the lane checkout, whose detached
   HEAD is the batch commit. Keep every red of the batch proof's gate in that one fix job; never
   start a second fix round for the same batch proof gate.
   The lane owns this allowance (`landing.fix-rounds`, default 1 per batch proof;
   `landing.fix-minutes`, default 120 minutes per job); the goal's budget is untouched.
   `<stamp>` is the red proof attempt's timestamp through seconds, for example
   `20261011T040737` from `20261011T040737.033326000Z`. Keep that stamp through
   the repair's follow-up proofs; a changed hand-in's new proof gets its own stamp.
   Write `artifacts/agents/landing/fixes/<attempt>/brief.md` (FILE below), naming the goal, every
   failed unit and test from the proof (including `<unit> (package)` for a package
   failure), the proof log path, and `git diff <batch base main>..HEAD -- metasystem`
   as the change under test. Include this rule verbatim: "fix the goal's code or
   its tests; never loosen or delete a test; if the fix needs a decision of the
   goal's person, stop and say so".
   Run `metasystem work build GOAL --work lane-fix-<stamp>-1 --brief FILE --check 'metasystem test impact'`.
   The review commits ONE plain commit on the batch tree through the engine's
   commit path, with message `goal GOAL: lane fix of <units> (fix round 1)` and final
   paragraph `Goal-Unit: GOAL/lane-fix-<stamp>-1`. Never substitute a git commit or bypass a
   guard when the engine refuses. The build/review path records
   `artifacts/agents/landing/fixes/<attempt>.json` with `{goal, units, job, read, commit, state}`;
   while running, status begins `Fixing <units> of GOAL on <sha> (fix round 1)`.
   Then run `metasystem work review GOAL --work lane-fix-<stamp>-1` for the roster's read,
   followed by `metasystem landing prove` of the new HEAD. End your turn while the
   proof runs; green goes to `landing push` as usual.
   Red again, a material read finding, or a builder stopped for a decision: run
   `metasystem landing return GOAL --cause own --reason TEXT`, naming fix round 1,
   the fix job id and read id (say when no read ran), every remaining red, and any
   required decision. Check out the merge's first parent before rebuilding the
   batch after a return. When the engine refuses the fix build, return as before with the refusal as evidence; other engine refusals hold the batch and go to case 9.
   For `main`, hold the batch; fetch main and compare it with the main commit recorded when the incident
   was registered. When fetched main moved since the incident was registered, run
   `metasystem landing prove --trunk` yourself (detached) before any question and end
   your turn while it runs. Ask only if that proof is red. Its green clears the incident,
   then continue with case 5 on the new main. When main has not moved, hold the batch
   naming main's units (`cause.name`), the incident record and the way forward through
   the enrolled person's hot-fix. The hot-fix alone releases nothing: it needs the trunk proof.
   When `last_proof.repeat` is `allowed`, run `metasystem landing prove` once more and
   end your turn. For any other cause, the waiting goals hold. A budget hold needs the enrolled person's own
   `metasystem landing prove`; follow the recorded command. `landing run` grants no proof
   permission. A red without the repeat allowance gets no other check of that tree.
4. **Conflict:** resolve as the goal's seat on this batch tree first.
   Run `metasystem landing resolve --as-seat`, which records `resolving` and the
   conflicted paths by class without aborting the merge. Source conflicts with main
   or another batch member take the same route; keep `MERGE_HEAD` present.
   Write `artifacts/agents/landing/fixes/<attempt>/merge-brief.md`, naming each path
   and class, both sides' intent, and the merge base. Find `<attempt>` in
   `metasystem landing status --json` under `running_fix.attempt`. For main's intent read
   `git log --oneline <base>..origin/main -- PATH`; for the goal's read
   `git log --oneline <base>..<goal tip> -- PATH`. If the conflict is with a batch
   member, read that member's tip in place of origin/main too.
   Include this rule verbatim: "keep both sides whole: main's behaviour plus this
   goal's additions; never drop a test; a generated file is regenerated by its
   declared command, never hand-edited". Include a units table estimating the
   resolution's changed lines, with unit `lane-merge-1`.
   Launch ONE resolution job:
   `metasystem work build GOAL --work lane-merge-1 --brief FILE --check 'metasystem test impact'`.
   The builder resolves and stages the source conflicts in this checkout, leaving
   generated conflicts for the engine's declared regeneration commands. It leaves
   the pending merge uncommitted. While running, status begins
   `Resolving COUNT conflicts of GOAL (PATHS)`.
   Wait for the resolution job through `metasystem work wait j2:JOB`, then repeat the same build
   command to collect it. The engine reuses the regeneration path's commands, stages
   their outputs, runs the check and completes this merge using the lane's merge
   message plus `Goal-Unit: GOAL/lane-merge-1`. Never substitute a git commit or
   bypass a guard. Run `metasystem work review GOAL --work lane-merge-1`, wait for
   its read and repeat that command to collect it, then `metasystem landing prove`.
   The read covers the resolution diff from the automatic merge tree — every file
   the resolution changed, generated paths excluded; when that diff is empty, the
   conflicted paths' diff against the parent whose side was not kept. The context
   records their classes and the check result; the goal's
   units were read on their branch and the rest of the goal is out of scope.
   The read checks out the merge commit itself and applies neither input to it.
   If the read errors, status says the merge is resolved with its read pending and
   names the error; repeat the same review command.
   A resolution job that stops because it cannot resolve, incompatible intents, a required
   decision of the goal's person goes back through
   `metasystem landing resolve`. Its return names every path and the resolution job id in its
   reason; a conflict with another batch member retains today's `after` hold.
   A material read finding after the merge commit returns the goal through
   `metasystem landing return GOAL --cause own --reason TEXT`, naming the resolution job,
   read and findings; rebuild from the merge's first parent, as in case 3.
   Read the outcome and reason, then continue with the remaining waiting work.
5. **Main moved during the proof:** fetch, then read `landing status --json` again.
   On `main_move.kind: records-only`, continue the current step and run `landing push`
   when its proof is green; the engine folds the records with one merge and checks the tree.
   Never check out new main or rebuild for a records-only move.
   A recorded flake moves main by its record too; the inherited green keeps its reason,
   naming the flaky unit and its fix goal.
   On `main_move.kind: code`, check out the new main, merge the same recorded batch members,
   and run `landing prove --gate --wait` and read `last_gate`: green or skipped continues to
   `landing prove`; none means the merge gate has not run for this merge. When other files moved and
   the batch's full proof is under an hour old, this proof runs only the test groups that read
   what main gained; push when the proof at the batch's depth is green. An inherited or scoped green older than an hour
   needs a full proof. A refused record merge follows this same rebuild path.
   Lines that began waiting after sealing are not added: proven work is pushed first, and
   they are the next batch.
   **Design refusal** (push returns a goal whose design no longer stands, or refuses because HEAD
   still contains a returned goal): rebuild the batch. Run `git checkout --detach origin/main`,
   `git merge --no-ff SHA` for each recorded member that still waits and is not held,
   run `metasystem landing prove --gate --wait` and read `last_gate`: green or skipped continues to
   `metasystem landing prove`; none means the merge gate has not run for this merge.
   End your turn; push when the proof at the batch's depth is green.
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

Never end your turn with a `waiting` member of your recorded batch you could act on: push it, return it, or have a
proof running, or hold on the cause the proof records.
