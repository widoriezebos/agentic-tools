# critique-closes-on-folded-proof

- Owner: m1e (goal 16 of plans/delivery-efficiency-plan.md), claimed 2026-09-13 01:05 local. **Revision 2, 2026-09-13**, written on the seat (Fable) under Wido's lanes; revision 1 was read by Codex gpt-5.6-sol (records/misc/critique-closes-on-folded-proof-design-critique-r1.md, eight material findings) and this revision folds them all (section 12).
- Goal and current status: designed, not built. Wido's word (R-97-m1e, 2026-09-11): yes to both rules; an unresolved invariant escalates to Wido, never a third automatic round.
- In flight right now: this revision, then its Codex read (round 2).
- Decisions made (and who made them): Wido, R-97-m1e (both rules; escalation over a third round). Wido, R-60-m1 (stop at the first round with no material finding; findings that fail the artifact test are demoted). Wido, R-42-m0 (three rounds is the ceiling). Wido, D81 (2026-08-16: two prose budgets spent without convergence exit to implementation behind fixtures, never a third prose budget). The seat: decisions 1 to 6 below.
- Waiting on the human: nothing for the build. The one question of revision 1 (test-only folds) is settled in decision 1 by the `proof-gap` kind.
- Dead ends (do not retry without new evidence): a path-keyed boundary (revision 1, refused: a fold that fixes the finding and changes unrelated policy in the same file passes); a "restoration" equivalence on reversed hunks (revision 1, refused: reversing a subset of approved hunks yields a tree no critic read).
- Next step: the round-2 Codex read of this revision; fold; then slice 0 on Codex with an Opus read.

## 1. What the audit found and what exists

The five-day audit: critique yield 68 percent by rounds; 26 percent of dispositions reported zero material findings; 137 fold-critique rounds cost 45.5 delegate hours; only 1 of 13 design chains ended on the skill's own stop rule. The pattern behind the cost: a critic reads a tree, finds bounded things, the implementer folds them, and the chain buys another whole read of the folded tree to learn what a test already proves; and a design loop that has stopped finding anything material keeps a third round on the calendar because nothing refuses it.

What exists (references to the tip of 2026-09-13):

- The finding register on the critic chain's root record (internal/dispatch/finding_register.go): `job critique-register-advance` folds each round (material findings pass the artifact test or are demoted; a `failed` round folds a synthetic unproven finding); `reviewRoundLimit` is frozen at dispatch from the goal's tier (build.go) under `metasystem.budget.review-round-max=3`; `job critique-register-close` is the four-row close table of plans/severity-tiered-rigor-design.md (a severe or unproven finding blocks until `goal accept-risk`; an empty unresolved set closes; bounded findings with budget left say "dispatch the next round"; bounded findings with the budget spent are deferred as review obligations on the goal). Exhaustion is checked before any successor record exists (dispatch.sh; `cap-exhausted-human-raise`, exit 10). Register entries carry `status` (open, disputed, resolved, deferred, accepted-risk) and `resolution` (withdrawn, out-of-scope, deferred, accepted-risk).
- The final-tree gate (internal/validate/conformance.go): `validate conformance --stage review` writes the immutable round artifacts `rounds/N/diff.patch` and `rounds/N/review.json` (`reviewedTree`, the project subtree); `--stage merge` refuses a critic whose `reviewedTree` is not the final project tree and issues the merge authorization on it. The schema-2 testing result's `candidateTree` is the whole-repository tree; landing validates its attempt owners (internal/landing/testing.go) and, after a base move, replays the certified patch and checks the post-image (`bindCertifiedChange`).
- The hazard close (internal/dispatch/hazard.go, `validateIndependentCritiqueReference`): the stamped critic must review the final work round and end at or after it; the stamp is written for a fresh critic dispatch only (review_reference.go), so a follow-up critic round that did review the terminal round cannot move it; `job review-reference-reconcile` is the repair.
- Landing (internal/landing/observe.go): a chain lands only closed, and the certified output is the closed critic's `reviewedTree`.
- The design-critique skill states the stop rule and the five-condition "fixtures as arbiter" exit in prose; nothing in code counts fold-read cycles, records a finding's grain, or refuses a redundant read. Metrics count critique rounds per chain; a landing's chain is on its commit trailer (`Landing-Provenance: chain=`), which the metrics loader does not read.

## 2. The rule, in one paragraph

A critic's read is bound to a read subject: the exact tree it read and the diff it was asked to judge. After the read, a chain closes in exactly one of three ways: the read was clean; the findings were folded and a new read of the folded subject is clean; or every remaining finding is bounded (low or medium), each is discharged by a correction whose every hunk is attributed to it and proven by a named test that passed on the final tree, and a certification record binds the read subject, the attributed hunks, the discharged findings and the test evidence to the final tree, which every later gate recomputes and verifies. A hunk no finding accounts for is a change no critic read, and it takes another read. A read is never repeated on a subject already read clean. A design loop closes at round two: a clean second round closes it, a mechanical residue becomes fixture obligations the implementation must discharge under its own critique, and a residue that is not mechanical goes to the human. No fourth round of anything.

## 3. Decision 1: the certification record

One immutable record, `artifacts/agents/<implementer root>/rounds/<fold round>/certification.json`, written by `job critique-certify --job <implementer root> --reviews <critic root> --manifest <file> --evidence <attempt ids>` and read by every gate. The record is a projection of the register: nothing in it is authority on its own; every gate recomputes from the register, the round artifacts and the trees.

```
schemaVersion        1
implementerJob       the implementation chain root
foldRound            the implementer round whose tree is certified (the final work round)
criticJob, criticRound
subject              the critic round's read subject (decision 3): {kind, implementerRoot, baseTree, reviewedTree, diffDigest}
finalWholeTree       the implementer workspace's HEAD tree (what a schema-2 result calls candidateTree)
finalProjectTree     its project-subtree projection, as conformance computes it
correctionDigest     sha256 of Diff(subject.reviewedTree, finalProjectTree) as conformance computes it
findings[]           the resolution manifest, one entry per certified finding:
                       {findingId, rigorClass, kind: correction|proof-gap, artifact,
                        hunks[]: sha256 of each hunk of the correction diff this finding accounts for,
                        test: {group, package|section, name, executionIdentity},
                        evidence: {attemptId, groupId}}
evidence             sorted attemptIds, each with its result digest and candidateTree
certifiedAt, by      RFC3339; the seat's machine and lineage
registerDigest       sha256 of the critic register as it stood when the record was validated
```

The boundary is derived, never stored: it is the union of the certified findings' artifacts and of the test files their manifests name. An artifact is one file path; a directory, a prefix, a glob refuses (`CERTIFY_ARTIFACT_SHAPE`); `NEW <path>` and `<old>=><new>` expand to their concrete endpoints before the check. The correction is verified hunk by hunk: the verb splits Diff(reviewedTree, finalProjectTree) into hunks (file, old range, new range, body), digests each, and requires every hunk to be claimed by exactly one manifest entry whose artifact is that hunk's file; a hunk nobody claims, a claim of a hunk that does not exist, or a hunk claimed twice refuses (`CERTIFY_BEYOND_BOUNDARY`, listing the unclaimed hunks: "a correction beyond the recorded boundary takes another read"). A hand-edited record cannot widen anything: the gates recompute the hunks from the trees and the manifest from the register entries (each certified entry carries its manifest fields and the record's digest).

Kinds. `correction`: the finding's production artifact changes (at least one claimed hunk in that file) and the named test passes on the final tree. `proof-gap`: the finding said a proof was missing and no production path was wrong; the claimed hunks are in test files only and are additive (no deletion or modification of an existing test line: every claimed hunk's old range is empty), and the named test is one the hunks add. A test-only fold for a `correction` finding refuses (`CERTIFY_NO_PRODUCTION_CHANGE`); a weakened test refuses (`CERTIFY_TEST_WEAKENED`).

Evidence. Each `attemptId` is a successful terminal proof attempt with a schema-2 test result whose `candidateTree` equals `finalWholeTree`, validated through the existing attempt-owner validator (internal/landing/testing.go, extracted so dispatch can call it); each manifest test is found in that result by group and execution identity with status passed (`CERTIFY_NO_TEST` names the finding and what is missing: no test named, the group absent, the test absent, the test not passed, or the evidence on another tree, `CERTIFY_EVIDENCE_TREE`).

Coverage and the transition. The manifest must name exactly the register's eligible unresolved set: every finding whose status is open or disputed and whose rigor class is bounded; a missing one or an extra one refuses (`CERTIFY_COVERAGE`); a severe or unproven open finding refuses outright (`CERTIFY_OPEN_SEVERE`: those close only through `goal accept-risk`); an unfolded latest round refuses (`CERTIFY_UNFOLDED`, the existing rule). Under the register's mutation lock the verb: (1) validates everything above and the read subject (`CERTIFY_STALE_READ`: the critic round's subject must equal the subject of the implementer round it reviewed); (2) writes the record atomically; (3) sets each certified entry to `status: resolved, resolution: certified, evidence: <record path>, evidenceDigest: <record digest>`; (4) stamps the implementer root's `independentCritiqueJobRef` at the critic through the reconcile path. A second run finding a record whose content equals what it would write completes steps 3 and 4 without rewriting (idempotent under a crash between 2 and 3); a record whose content differs refuses as review.json does ("immutable certification already exists").

## 4. Decision 2: what the gates verify

- Final-tree gate (`mergeCritique`): when the critic's `reviewedTree` is not the final project tree, read `rounds/<final round>/certification.json`; accept when its `subject.reviewedTree` equals the critic's, `finalProjectTree` equals the recomputed final project tree, `finalWholeTree` equals the workspace HEAD tree, the recomputed hunks of Diff(reviewedTree, finalProjectTree) are exactly the union of the manifest's claimed hunks, every certified register entry carries the record's digest, and `registerDigest` matches the register with those entries excluded; refuse otherwise with the existing "reviewed tree X is stale" plus the mismatch. The merge authorization is issued on the final tree.
- Hazard close: a critic whose latest folded round reviewed a subject other than the terminal work round's is accepted when a certification binds that subject to the terminal round (`criticRound`, `foldRound`) and `certifiedAt` is at or after the terminal round's end; the `reviews == final round` rule stands otherwise.
- Register close and `CloseCheck`: `resolved/certified` counts as resolved; `job critique-open-finding-ids` excludes it.
- Landing: the certified output selects the terminal review patch by the certification's `finalProjectTree`; `bindCertifiedChange` (patch replay and post-image after a base move) is kept unchanged. Bar (a) of plans/two-bars-for-changes-design.md is amended in the same landing: a chain lands only on a tree a critic read clean, or on a tree a verified certification binds to such a read.
- Every gate refuses a certification naming a finding the register does not carry as `resolved/certified` with that record's digest.

## 5. Decision 3: a read subject, and no read of a subject already read clean

A read subject is a typed identity recorded on every critic round's `review.json` and return: `{kind: live|commit, implementerRoot, baseTree, reviewedTree, diffDigest}`; a live subject's `baseTree` is the boundary base and `reviewedTree` the project tree; a commit subject's are the commit's parent tree and tree. Two subjects are equal only field by field; a live subject never equals a commit subject.

At critic dispatch (fresh or follow-up), before any record exists, the dispatcher computes the subject the critic would read and refuses `REDUNDANT_READ` when a folded round of any critic chain reviewing the same implementer chain read that exact subject and its register, folded through that round, had no open or disputed finding. The refusal names the round, the subject and the rule, and is persisted as an event on the implementer root (`artifacts/agents/<root>/reads-refused.jsonl`: at, criticRole, subject, priorCriticJob, priorRound, rule), which the measure reads. An empty follow-up diff is the same case (the subject is unchanged); there is no restoration rule: a diff that reverses part of a reviewed diff yields a tree no critic read, and it takes the read (the absorbed clause's two easy cases are kept, its hard case is refused on the critic's argument).

The chain then closes on the prior read: the register's last folded round is clean and the terminal subject equals its subject. The binding is persisted on the implementer root as `closedOnRead: {criticJob, round, subject}` by `job critique-register-close`, and the hazard close, the final-tree gate and landing accept a critic chain whose latest folded round's subject equals the terminal subject, whichever round the stamp names. This is the absorbed stamp clause: a follow-up critic round that provably reviewed the terminal subject counts, and no stamp-only dispatch is ever needed.

## 6. Decision 4: design critique fails safe at round two

- A design-critic chain's `reviewRoundLimit` is 2 at every tier: set in the common budget owner (build.go's resolution by role) and refused above 2 by `critique-budget-rebind` for the role. R-97-m1e supersedes R-60-m1's tier-derived budget for this role only; tier eligibility is unchanged (tier 1 has no design critique).
- Each material finding carries a grain, set at fold from the critic's return: `mechanical` when the finding names one artifact, one behaviour and the fixture that would prove it (three fields the return schema gains and the artifact test checks); `invariant` otherwise. The critic root keeps the trajectory: `materialByRound[]`.
- Round 2 folded with no material finding closes the loop.
- Round 2 folded with only mechanical findings: `job critique-register-close` defers each as a review obligation on the goal with `test: prove: <fixture>` (the existing deferral, now one obligation per finding with the finding's named fixture), the design chain closes with them open, and the implementation that follows discharges each against the chain, the artifact and the test; that implementation runs under its own code critique because its goal is tier 2 or 3 (critique-always), and the page header records the exit ("closed at round 2 on N fixture obligations").
- Round 2 folded with an `invariant` finding, or with a severe or unproven one: the close refuses with `cap-exhausted-human-raise` naming the finding and the human's two verbs (`goal accept-risk`, or a re-scope by `goal edit`), and round 3 is refused by the limit. Never a third automatic round.
- The skill's Round Budget section and docs/orchestration.md state the rule in these words; the stop hook's turn verdict prints, for an open critic chain on the seat's goal, its fold-read cycle ("design critique round 2 of 2 folded: 0 material" or "3 mechanical, deferred as obligations").

## 7. Decision 5: the measure

Three figures from `internal/metrics`, printed beside `critique_rounds`, computed over the landed chains of a window; the loader reads each landing commit's `Landing-Provenance: chain=` trailer, each critic root's `reviews`, each round's subject and fold outcome, and the refusal events:

- `rounds_per_landing`: for each landed implementation chain, its implementer rounds plus the rounds of every critic chain reviewing it; mean and maximum over the window. Target under 3.
- `redundant_reads`: the share of critic rounds whose subject equals a subject an earlier folded clean round on the same implementer chain read. Target under 10 percent. A certification is not a read.
- `reads_refused`: the count of `REDUNDANT_READ` events in the window.

The zero-yield share stays out: it penalizes clean first reads, which are the aim.

## 8. Decision 6: scope

Not in scope: the tier ladder, the code-critique round ceiling, the artifact test, the accept-risk path, the landing bars beyond the one clause of bar (a). The certification only ever closes bounded findings; a severe finding still needs a human word or a clean read.

## 9. The four absorbed clauses, restated in full

The ledger's park notes are cut at three hundred characters; the clauses as this design takes them, from the parked goals' records:

1. closing-read-follows-the-change-not-the-label: a final round that changes no file closes without a new read, and so does one whose diff is empty against the previously read subject; a round that only restores a value the chain already fixed was the hard case, and it is refused here: the restored tree is a tree no critic read (section 5). A behaviour change dressed as a no-op still demands the read; the refusal names which rule applied.
2. critique-stamp-follow-up: the closure gate accepts a critic chain whose latest folded round provably reviewed the terminal subject even when the stamp names an earlier round, so no stamp-only critic dispatch is needed (section 5).
3. design-loop-exit-in-the-skill: the skill's Round Budget section and docs/orchestration.md state the enforced stop rule and the D81 exit (implementation behind fixtures, never a third prose budget), and the stop-hook turn verdict prints the current fold-read cycle for an open critic chain (section 6).
4. severity-tiered-rigor-p2 slice 2b: the material stop and the close table read the unresolved set after certification and accepted risks leave it (sections 3 and 4).

## 10. Proof

| # | Obligation | Fixture | Home |
| --- | --- | --- | --- |
| 1 | A certification for two bounded findings, each with attributed hunks and a named passing test in a schema-2 result on the final whole tree, resolves them `certified`, closes the register, stamps the critic, and the final-tree gate issues on the final tree | Go test on a fixture chain with a real worktree and a canned schema-2 result through the extracted owner validator | internal/dispatch, internal/validate |
| 2 | An unclaimed hunk, a hunk claimed twice, a directory artifact, a hand-edited record (widened manifest, changed digest) and a register entry without the record's digest each refuse by name at the verb and at the gate | Go test | internal/dispatch, internal/validate |
| 3 | A severe open finding, a manifest that misses or adds a finding, a `correction` with test-only hunks, a `proof-gap` that modifies an existing test line, a test absent or failed, and evidence on another tree each refuse by name | Go test | internal/dispatch |
| 4 | A crash between the record write and the register update is completed by the next run without a second record; a differing record refuses | Go test | internal/dispatch |
| 5 | The hazard close accepts the pre-fold critic under a certification and refuses without one; landing selects the certified final tree and still replays the patch after a base move; bar (a)'s clause is in the page | Go test | internal/dispatch, internal/landing |
| 6 | A critic dispatch onto a subject already read clean is refused before any record exists, named, and persisted; a live subject never equals a commit subject; a follow-up whose diff is empty is the same refusal; a reversed hunk is dispatched | Shell leg in dispatch-fixtures.sh and a Go test on subject equality | scripts/agents, internal/dispatch |
| 7 | The chain closes on the prior clean read with `closedOnRead` persisted, and the hazard close, the gate and landing accept a critic whose latest folded round reviewed the terminal subject while the stamp names an earlier round | Go test | internal/dispatch, internal/validate, internal/landing |
| 8 | A design critic chain's limit is 2 at tier 3; a rebind above 2 refuses; round 2 clean closes; round 2 with mechanical findings closes with one obligation per finding naming its fixture; round 2 with an invariant refuses with the two verbs and refuses round 3; the return schema's grain fields are validated | Go test on the register and a shell leg | internal/dispatch, scripts/agents |
| 9 | `rounds_per_landing`, `redundant_reads` and `reads_refused` computed on a fixture window with known landings, chains, subjects and events | Go test | internal/metrics |
| 10 | The skill and docs carry the rule; the turn verdict prints the fold-read cycle for an open critic chain | Shell leg on the stop hook fixture; Go test on the verdict | scripts/agents, internal/goal |

## 11. Slices, each landing alone

0. Schemas and identity, no behaviour change: the read subject on `review.json` and critic returns; the manifest fields and the `certified` resolution admitted by the register reader; the grain fields on the return schema (optional until slice 3); the refusal-event file format; the attempt-owner validator extracted from internal/landing into a package dispatch can call; the metrics loader reading landing provenance trailers. Old records read as before.
1. The certification: the verb, the register transition, and the four gates with bar (a) (decisions 1 and 2), rows 1 to 5.
2. The read subject at dispatch, the redundant-read refusal with its event, and the close on a prior read consumed by every gate (decision 3), rows 6 and 7.
3. Design critique at round two (decision 4), rows 8 and 10.
4. The measure (decision 5), row 9.

## 12. Critique record

Round 1 (2026-09-13, Codex gpt-5.6-sol, read-only): eight material findings, all folded. 1: the boundary was path-keyed (a fold that fixes the finding and changes unrelated policy in the same file passed; a hand-edited record could widen it; directories and renames were undefined): now derived from the register, one file per artifact, renames expanded, and every hunk attributed to exactly one finding. 2: the evidence model could not represent the proof contract (project subtree versus whole tree, joined attempts, no finding-to-test mapping, the owner validation bypassed): now both trees are recorded, evidence is sorted attempt ids validated by the existing owner validator, and the manifest maps each finding to a test by execution identity. 3: the restoration rule was unsafe (reversing approved hunks yields an unread tree) and tree equality conflated live and commit subjects and collided with the hazard close: now a typed read subject, exact equality only, no restoration rule, and a persisted closure binding every gate consumes. 4: the transition was incomplete (partial certification, crash half-state, status versus resolution, landing's patch replay dropped): now exact coverage of the eligible set, an idempotent write order under the lock, `resolved/certified`, `bindCertifiedChange` kept, bar (a) amended. 5: a test-only fold could prove nothing: now the `proof-gap` kind with additive test hunks, and a `correction` must change its production artifact. 6: decision 4 lacked the machinery: now the grain field, the trajectory, the role's cap in the budget owner and the rebind, R-97 over R-60 for the role, and the code critique after the fixture exit named. 7: the metrics were not computable: now the refusal event is persisted and the loader reads landing provenance, subjects and fold outcomes. 8: the slices did not land alone: reordered with a schema slice first. Sound as built, per the critic: no certification of severe or unproven findings; a certification is not a read; every consumer recomputes and fails closed; the stop at zero material findings and no fourth round; the refusal before a successor record; the landing patch replay kept.
