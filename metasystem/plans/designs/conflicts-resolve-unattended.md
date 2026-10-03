# Design for conflicts-resolve-unattended

- Kind: design
- Id: 01M41RKZ0WBCZA2NF3T3E6972Y
- Status: accepted
- Goals: conflicts-resolve-unattended

Revision 3: folds the three accepted findings of critique round 2 (Codex on Astra, 2026-10-03); the rest is revision 2's. Paths relative to `metasystem/`; nothing is built yet.

Accepted 2026-10-04 00:08 CEST by m1e for Wido under his power of attorney (grant EJSNF4E0Q4GRGW0R3YHWM7RX5X-m1e-718ba0eb), after Codex on Astra rounds 7/3/1/0; Wido may overturn.

## Wido's words (2026-10-03, binding)

> "this is too simplistic. This needs a proper design and review round. The goal is unattended functionality."

Requirement (22:22): generic for any adopted application in any language; adopter specifics are declarations.

## Scope of step 1

Step 1 builds decisions 1 to 5 and amends the docs to requirement 4; a person is asked only for a judgement. Not in step 1: how a unit is built or read, and the Deferred list.

## Decision 1: generated files are a declaration (requirement 1)

A new top-level key `generated` in `testing.json` (`internal/testpolicy/contract.go:40-51`):

```json
"generated": [{"paths": ["internal/ui/web/bundle/**"],
  "command": ["npm", "ci", "--ignore-scripts"], "then": ["npm", "run", "bundle"],
  "cwd": "internal/ui/web/_app"}]
```

One entry per set: gitignore-style `paths`, argv lists run in order from `cwd`, no shell. Without the key nothing is generated; the engine names no path, tool or language.

**A Java adopter** declares `{"paths": ["target/openapi/**"], "command": ["mvn", "-B", "generate-sources"]}`, or nothing, and every conflict is then a source conflict.

## Decision 2: the seat rebases before every hand-in (requirements 2 and 3)

**The verb.** `work rebase GOAL` fetches origin and rebases the goal branch onto origin's main in a private worktree with the merge-driver arguments (`internal/goal/branch/merge_driver.go:9-19`), then re-writes each Goal-Read commit for its rebased unit. `work land` runs it first, refuses a branch behind main, and its hand-in sentence says "rebased onto main <short sha>". A branch on main's tip holds and writes nothing (R-129-ui). The pre-rebase tip is kept at `refs/metasystem/goals/<goal>/before/<tip>` and pushed with the branch. The goal-branch fetch (`internal/goal/branch/push.go:63,167`) and the lane's fetch take the branch and its `before/*` refs together, so the old unit commits a carried read's validation needs (`internal/goal/branch/attest.go:414,429`) are present wherever it is validated, and go with the branch.

**The digest a read survives on.** `ChangeDigest` is the sha256 of `git diff --binary --full-index --no-renames C^ C` reduced to the change's own bytes: per file the `diff --git`, mode, `---`, `+++` and `\ No newline` lines and every ` `, `+`, `-` and binary-patch line, byte for byte, no whitespace folding; `index` lines dropped; each hunk header replaced by `@@`; files under a pattern declared at C dropped whole. Moving the base elsewhere keeps it; a change on main inside a hunk's context lines breaks it and the unit is read again.

**The carry.** `work rebase` is the first shipped caller of `commitRead` with `Carry` set (`internal/goal/branch/attest.go:745-760`). The subject is unchanged: at carry time and at validation (`internal/goal/branch/attest.go:428-437`) `ChangeDigest` is computed from both commits and must be equal, replacing today's `UnitDigest` comparison; existing reads carry without migration. Folds are compared with the Read kind removed (`internal/goal/branch/attest.go:434`, `:750`), so a rewritten earlier Goal-Read no longer breaks a later unchanged unit; each Goal-Read below is still validated itself, and the carried attestation's `Folds` are the new branch's. The carried read records a fresh quick-gate run on the new tree (`internal/goal/branch/attest.go:422`); its record files go under the new commit id, the old ones kept. A unit whose `ChangeDigest` changed has no read; the verb names it for `work review`. The landing transition check (`internal/goal/branch/attest.go:632-637`) stays: the merged change must equal the attested `UnitDigest`, recomputed for the new commit.

**Conflicts.** At a conflicting unit the rebase stops. Generated paths never go to the builder or a person: the verb takes main's side for each; when no source path conflicts it runs the declared commands, stages, continues. Decision 4 classifies the source paths. A judgement aborts the rebase, leaving the branch untouched, and writes the question. Anything else runs a **resolve round**: a revise round (`internal/launch/unit_revise.go:97-218`) in the rebase worktree whose plan base is the stopped rebase's HEAD, origin's main plus the units already replayed. The builder gets the unit's brief, each conflicting source hunk as diff3 text (base, main's, the goal's), any written resolution from the return or a person's answer, and the rule to resolve those hunks and touch no other path. In a mixed conflict the verb then runs the declared commands, rebuilding the generated paths from the merged sources. It stages and continues: the rebased commit is the resolution on the new base, so the same lines never conflict again. That unit has no read; the critic reads it before `work land` hands in. `artifacts/agents/goals/<goal>/conflict.json` holds base, main's tip and per path its blob ids, class and resolution, and goes when the rebase completes.

**A Java adopter** needs only `git`.

## Decision 3: the lane regenerates and never merges sources (requirements 1 and 4)

The landing agent's conflict case (`skills/landing-agent/SKILL.md:41-43`) becomes: never edit a conflicted file; run `landing resolve`, which classifies every conflicting path.

- **Only generated:** takes main's side, runs the declared commands, stages; the agent commits and `landing prove` proves the tree.
- **Any source:** aborts the merge; returns the goal with each path's class and the resolution it can name.
- **Command fails:** aborts the merge; returns the goal with the exit and log path.

**Its record.** Every run appends one line to the lane's `artifacts/agents/landing/regenerate.jsonl` (goal, sha, command, exit, log, outcome), failures included, never to `results.jsonl`: only `landing prove` writes a proof result, and `landing push` still requires green for HEAD's tree (`internal/landing/plain/push.go:94-104`), so an unproved tree cannot be pushed. No time limit: `landing status` shows the running command and its log's growth; `landing stop` ends it (R-35-m3, R-126-m1e).

**The return.** `{"conflict": {"main": sha, "paths": [{"path", "class", "resolution"}]}}`; `resolution` is text the builder applies when the verb can name it (both inserted at one place: "keep both, main's lines then the goal's"), else empty. `work land` shows it; `work rebase` feeds it to the resolve round.

**A Java adopter** sees the same verb and return.

## Decision 4: who decides a conflict is a judgement

One package, used by both verbs, classifies each conflicting path by the set of original lines each side changed or removed:

- **generated:** the path matches a declared pattern; resolved by the verb, by regenerating.
- **judgement:** a source path whose sets share a line, or without a common original (add against add, delete against change); a person's call.
- **builder:** every other source path, insertions at one place or edits to different lines; the seat's builder in a resolve round, read by the critic.

Pure insertions change no original line, so use case 3 is builder class. A builder resolving named hunks under a critic's read resolves nothing by force.

**The person's question** (R-143-m1e), written by `work rebase`: "Goal A and main both changed lines N to M of `path`. Keep main's, keep the goal's, or write a third. Impact: main's drops what A did there and unit U loses its read; the goal's undoes goal B's landed change; a third is read again. Nothing lands until you answer." The answer is recorded in `conflict.json` with its impact, bound to the path's three blob ids (base, main's, the goal's). The next `work rebase` reuses it as the resolve round's written resolution only when the path conflicts with those same ids, and otherwise asks anew with the new impact.

## Decision 5: a returned line re-queues itself (use case 5)

After a return that needed no change, `work land --again` makes `HandIn` (`internal/landing/plain/queue.go:220`) append a waiting line over a returned one; `work land` sets it itself when the return names only paths the rebase resolved and the branch is unchanged. `--again` on a waiting line holds and writes nothing.

## Use cases

1. Component, test, bundle: three reads carried; one unit's source resolved, its bundle regenerated, read.
2. Bundle only: the lane regenerates, proves, pushes; no return.
3. Same list in `cmd/metasystem/intent.go`: returned builder class, "keep both"; one resolve round and read, three carried.
4. 731 commits behind: handled unless a judgement; then Wido is asked.
5. Unchanged commit after a return: decision 5.

## Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| resolving a lane merge conflict | the landing agent | `landing resolve` | `metasystem/skills/landing-agent/SKILL.md` |
| a non-judgement peer conflict | the person | the seat's builder and critic | `metasystem/docs/working-with-agents.md:52`, `metasystem/docs/orchestration.md:318,404` |
| the base a conflict is resolved on | the attempt's original base | the stopped rebase's HEAD | `metasystem/internal/launch/unit_revise.go` |
| regenerating the bundle after a merge | a seat, by hand | both verbs | `metasystem/internal/ui/web/bundle/README.txt` |
| building the carry request | tests only | `work rebase` | `metasystem/internal/goal/branch/attest.go` |
| a repeat hand-in after a return | refused | `HandIn` with `--again` | `metasystem/internal/landing/plain/queue.go` |

Fetching the kept `before/*` refs is a new responsibility, not a moved one; decision 2 gives it to the goal-branch and lane fetches.

## Recurring findings, answered

- **Shared state** is keyed by goal (`conflict.json` with its answer, kept refs) or by lane (`regenerate.jsonl`), never by checkout.
- **First run.** No `generated` key, `conflict.json` or `regenerate.jsonl`: nothing generated, pending or run.
- **Repeats** hold and write nothing: a rebase onto the current base, `--again` on a waiting line, a resolved tree.
- **Time.** No elapsed limit; stalls show in `landing status` and `work status` and end by their stop verbs.
- **Overlap.** The lane merges the handed-in sha, which a rebase never moves; `landing resolve` refuses a checkout with changes it did not make; `work rebase` refuses while a resolve round runs.
- **One line.** Each `work rebase` and resolve round ends with one goal history line, each `landing resolve` with one `regenerate.jsonl` line.

## Deferred

- The lane merging builder-class conflicts itself (step 2, once the classification has proved right).
- The carried landing path (`internal/landing/landpath/carried.go:100-111`) keeps asking a person.
- UI for a pending question; ordering between generated sets.

## Decisions reserved for Wido

None. Requirement 4 (22:22) settles it: "a seat with a builder and a critic resolves them"; only a judgement is a person's call. Step 1 amends the three docs sentences to say exactly that.

## Critique rounds 1 and 2, adjudicated

All accepted. Round 1: CRU-CHANGE-DIGEST (decision 2, digest); CRU-REGEN-PROOF-RESULT (decision 3, record); RULING-conflicts-resolve-unattended (decision 2, conflicts); CRU-CARRY-FOLDS and CRU-EXISTING-READS (decision 2, carry); CRU-CONFLICT-CLASSIFICATION (decision 4); MOVED-EFFECTS-PEER-CONFLICT-OWNER (moved effects, line 2).

Round 2: RULING-R-143-m1e (decision 4, question); CRU-MIXED-GENERATED-SOURCE (decision 2, conflicts; decision 4, classes); CRU-CARRY-TRANSPORT (decision 2, verb; moved effects, line 6).
