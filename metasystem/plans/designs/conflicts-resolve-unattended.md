# Design for conflicts-resolve-unattended

- Kind: design
- Id: 01M41RKZ0WBCZA2NF3T3E6972Y
- Status: draft
- Goals: conflicts-resolve-unattended

Revision 1: first draft, 2026-10-03; facts read on main that day. Paths are relative to `metasystem/`. Nothing is built yet.

## Wido's words (2026-10-03, binding)

> "this is too simplistic. This needs a proper design and review round. The goal is unattended functionality."

> The mechanism is generic for any application adopted by the metasystem, in any language. Nothing about this repository is written into the mechanism; adopter specifics are declarations.

## Scope of step 1

Step 1 builds: a generated-files declaration in the testing contract; the docs' conflict sentences brought in line with requirement 4; one verb, `work rebase`, run before every hand-in, carrying reads whose change did not move; a lane verb that regenerates declared files and returns source conflicts with a written resolution; and a repeat hand-in that re-queues a returned line without a person. A person is asked only when both sides changed the same lines.

Step 1 does not merge source conflicts in the lane, does not change how a unit is built or read, adds no UI, and leaves the carried path alone.

## The problem in plain English

Every read names its unit by commit id, and a rebase renames every commit, so every read is orphaned even when the change did not move. The landing agent stops at a conflict; a returned branch waits on a person; a repeat hand-in at the same commit changes nothing. Generated files conflict on every parallel goal and are nobody's decision.

## Decision 1: generated files are a declaration (requirement 1)

**Where.** A new top-level key `generated` in `testing.json` (`internal/testpolicy/contract.go:40-51`):

```json
"generated": [{"paths": ["internal/ui/web/bundle/**"],
  "command": ["npm", "ci", "--ignore-scripts"], "then": ["npm", "run", "bundle"],
  "cwd": "internal/ui/web/_app"}]
```

One entry per set: gitignore-style `paths`, argv lists run in order from `cwd`, no shell. Without the key, nothing is generated.

**Owner and data.** The contract loader reads it; `work rebase` and `landing resolve` use the loaded list. The engine names no path, tool or language.

**A Java adopter** declares `{"paths": ["target/openapi/**"], "command": ["mvn", "-B", "generate-sources"]}`, or nothing, and then every conflict is a source conflict. No npm, Node or Go is assumed.

## Decision 2: the seat rebases before every hand-in (requirements 2 and 3)

**The verb.** `work rebase GOAL` fetches origin and rebases the goal branch onto origin's main in a private worktree with the merge-driver arguments (`internal/goal/branch/merge_driver.go:9-19`), then re-writes each Goal-Read commit for its rebased unit. `work land` runs it first, refuses a branch behind main, and its hand-in sentence says "rebased onto main <short sha>". A branch already on main's tip is reported as holding; nothing is written (R-129-ui).

**The digest a read survives on.** A new `ChangeDigest`: the sha256 of `git patch-id --stable` over `git diff --binary --full-index C^ C`. It covers the change's content alone, never its base blobs, and joins the attestation subject. A unit whose `ChangeDigest` is unchanged across the rebase has its read carried: `work rebase` is the first shipped caller of `commitRead` with `Carry` set (`internal/goal/branch/attest.go:745-760`); the carry check compares `ChangeDigest` where it compares `UnitDigest` today, and the record files are copied under the new commit id. A unit whose `ChangeDigest` changed has no read, and the verb names it for `work review`.

**At landing.** The transition check (`internal/goal/branch/attest.go:632-637`) stays: the merged tree's change must equal the attested `UnitDigest`, which the carried attestation recomputes for the new commit. A carried read lands on a fresh read's proof: content matched at the carry, tree at landing.

**Conflicts.** When the rebase stops on declared generated paths only, the verb takes main's side, runs the declared commands, and folds the result into the unit's rebased commit. On a source conflict it aborts, leaves the branch untouched, and writes `artifacts/agents/goals/<goal>/conflict.json`: base, main's tip, and for each path its three blob ids and class (decision 4).

**A Java adopter** needs only `git`.

## Decision 3: the lane regenerates and never merges sources (requirements 1 and 4)

The landing agent's conflict case (`skills/landing-agent/SKILL.md:41-43`) becomes: never edit a conflicted file; run `landing resolve`. The verb classifies every conflicting path (`git diff --name-only --diff-filter=U`) against the declared list.

| Conflicts | `landing resolve` |
|---|---|
| only generated paths | checks out main's side, runs the declared commands in the lane checkout, stages the result; the agent commits "regenerate <paths>" and `landing prove` proves the tree as any merge |
| any source path | aborts the merge and returns the goal with the resolution it can name |
| a command fails | aborts the merge and returns the goal with the exit and log path; nothing is committed |

The engine runs the command, so the agent cannot vary it. No time limit: `landing status` shows the running command and its log's last growth; `landing stop` ends it (R-35-m3, R-126-m1e). Every run ends with one line in `results.jsonl`, failures included.

**The return.** The reason gains structure: `{"conflict": {"main": sha, "paths": [{"path", "class", "resolution"}]}}`. `resolution` is plain text a builder applies: additive, "keep both: main's lines then the branch's"; judgement, "a person decides". `work land` shows it.

**A Java adopter** sees the same verb and return.

## Decision 4: who decides a conflict is a judgement

The classification is mechanical, in one package both verbs use:

| Class | Rule | Resolved by |
|---|---|---|
| generated | every conflicting path matches a declared pattern | the verb, by regenerating |
| additive | both sides only added lines and no line was changed by both | the seat's builder, applying the written resolution under `work revise`, read by the critic |
| judgement | both sides changed or removed one of the same lines | a person |

Additive is use case 3. A builder applying a written resolution under a critic's read resolves nothing by force, as the docs demand.

**The person's question** (R-143-m1e), written by `work rebase` to the goal's question: "Goal A and main both changed lines N to M of `path`. Keep main's, keep the goal's, or write a third. Impact: main's drops what A did there and unit U loses its read; the goal's undoes goal B's landed change; a third is read again. Nothing lands until you answer." The answer is recorded with the impact shown.

## Decision 5: a returned line re-queues itself (use case 5)

The gap is a hand-in at the same commit after a return that needed no change. `work land --again` makes `HandIn` (`internal/landing/plain/queue.go:220`) append a waiting line over a returned one; `work land` sets it itself when the return names only paths the rebase resolved and the branch is unchanged. `--again` on a line already waiting holds and writes nothing.

## Use cases

| Case | Step 1 |
|---|---|
| 1. component, test and bundle | handled: three reads carried, bundle regenerated, one unit reviewed |
| 2. bundle only | handled in the lane: regenerate, prove, push, no return |
| 3. same list in `intent.go` | handled: returned additive with the resolution; one unit revised and read, three carried |
| 4. 731 commits behind | handled unless a path is a judgement; then Wido is asked |
| 5. unchanged commit after a return | handled by decision 5 |

## Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| resolving a merge conflict in the lane | the landing agent's session | `landing resolve` | `metasystem/skills/landing-agent/SKILL.md` |
| regenerating the web bundle after a merge | a seat, by hand | `landing resolve` and `work rebase`, from the declaration | `metasystem/internal/ui/web/bundle/README.txt` |
| building the carry request | tests only | `work rebase` | `metasystem/internal/goal/branch/attest.go` |
| a repeat hand-in after a return | refused, reports the return | `HandIn` with `--again` | `metasystem/internal/landing/plain/queue.go` |

## Recurring findings, answered

- **Shared state** is keyed by goal (`conflict.json`) and by merge tree (the lane's results), never by checkout.
- **First run.** No `generated` key: nothing is generated; no `conflict.json`: nothing pending.
- **Repeats** hold and write nothing: a rebase onto the current base, `--again` on a waiting line, a resolve of a resolved tree.
- **Time.** No elapsed limit; stalls show in `landing status` and end by `landing stop`.
- **Overlap.** The lane merges the handed-in sha, which a seat's rebase never moves. `landing resolve` refuses when the lane checkout holds changes it did not make.
- **One line.** Each `work rebase` ends with one goal history line; each `landing resolve` with one results line.

## Deferred

- The lane merging additive conflicts itself (step 2, once the classification has proved right).
- The carried landing path (`internal/landing/landpath/carried.go:100-111`) keeps asking a person.
- UI for a pending question; ordering between generated sets.

## Decisions reserved for Wido

None. Who resolves an additive conflict between goals is settled by Wido's requirement 4 (2026-10-03 22:22): the lane returns source conflicts with the resolution it can name, "a seat with a builder and a critic resolves them", and only a conflict that needs a judgement is a person's call. Step 1 therefore amends the three sentences that still say every peer conflict is a human decision (`docs/working-with-agents.md:52`, `docs/orchestration.md:318`, `docs/orchestration.md:404`) to say exactly that.
