# critique-closes-on-folded-proof

- Owner: m1e (goal 16 of plans/delivery-efficiency-plan.md), claimed for the design. **Revision 3, 2026-09-13**, written on the seat (Fable) under Wido's lanes after his word on section 11 (option (a)): this page is narrowed to the rules he has ruled on, no read of a subject already read clean and the close on a prior clean read, design critique failing safe at round two, and the measure; the certification record (rule 1 of the DONE line) moves to goal certification-closes-a-folded-bounded-finding, opened in his name, whose page starts from the two read records of this one.
- Goal and current status: revision 3 (narrowed to the buildable rules, section 11) with build spec 9b. Slice 0 landed 4fbd271c; part 1a-i of slice 1 lands with this revision of the page; parts 1a-ii and 1b are not built.
- In flight right now: slice 1 part 1a-i (subjects computed before claim and published after it wins, SUBJECT_MISMATCH, the fold-time binding, the closure on the critic root) carried onto the tip, green in full on four packages and the fast gate, under its Opus read; worktree .claude/worktrees/ccf-s1ai-tip.
- Decisions made (and who made them): Wido, R-97-m1e (both rules; escalation over a third round). Wido, R-60-m1 (stop at the first round with no material finding; findings that fail the artifact test are demoted). Wido, R-42-m0 (three rounds is the ceiling). Wido, D81 (2026-08-16: two prose budgets spent without convergence exit to implementation behind fixtures, never a third prose budget). Wido, 2026-09-13 ("I agree with all your proposals"): option (a) of section 11. The seat: the mechanisms of sections 3 to 5.
- Waiting on the human: nothing.
- Dead ends (do not retry without new evidence): a path-keyed boundary and a "restoration" equivalence on reversed hunks (revision 1); textual hunks as a certification unit (revision 2); both belong to the certification goal now. A read subject shared by design, live and commit critics (revision 2, refused: the three read different things).
- Next step: in flight (2026-09-13 14:30Z): part 1a-i (read subjects computed at dispatch, SUBJECT_MISMATCH, the clean closure written by CritiqueChainClose in the one record write that sets chainClosed) lands in the same commit as this line after three Codex rounds and three Opus reads (the last with no material finding) and the dispatch bed through the engine. Next: a Fable delegate writes the build briefs for part 1a-ii (the hazard close, merge gate and landing reading the closure, requiring the closure round to equal the last critic round; types moved to a stdlib-only internal/readsubject) and part 1b (REDUNDANT_READ and CONCURRENT_READ admission, durable events, the three follow-up legs), carrying the read's non-material findings N3-1 to N3-7. Slice 0 landed 4fbd271c.

## 1. What the audit found and what exists

The five-day audit: critique yield 68 percent by rounds; 26 percent of dispositions reported zero material findings; 137 fold-critique rounds cost 45.5 delegate hours; only 1 of 13 design chains ended on the skill's own stop rule. The pattern behind the cost: a critic reads a tree, finds bounded things, the implementer folds them, and the chain buys another whole read of the folded tree to learn what a test already proves; and a design loop that has stopped finding anything material keeps a third round on the calendar because nothing refuses it.

What exists (references to the tip of 2026-09-13):

- The finding register on the critic chain's root record (internal/dispatch/finding_register.go): `job critique-register-advance` folds each round (material findings pass the artifact test or are demoted; a `failed` round folds a synthetic unproven finding); `reviewRoundLimit` is frozen at dispatch from the goal's tier (build.go) under `metasystem.budget.review-round-max=3`; `job critique-register-close` is the four-row close table of plans/severity-tiered-rigor-design.md (a severe or unproven finding blocks until `goal accept-risk`; an empty unresolved set closes; bounded findings with budget left say "dispatch the next round"; bounded findings with the budget spent are deferred as review obligations on the goal). Exhaustion is checked before any successor record exists (dispatch.sh; `cap-exhausted-human-raise`, exit 10). Register entries carry `status` (open, disputed, resolved, deferred, accepted-risk) and `resolution` (withdrawn, out-of-scope, deferred, accepted-risk).
- The final-tree gate (internal/validate/conformance.go): `validate conformance --stage review` writes the immutable round artifacts `rounds/N/diff.patch` and `rounds/N/review.json` (`reviewedTree`, the project subtree); `--stage merge` refuses a critic whose `reviewedTree` is not the final project tree and issues the merge authorization on it. The schema-2 testing result's `candidateTree` is the whole-repository tree; landing validates its attempt owners (internal/landing/testing.go) and, after a base move, replays the certified patch and checks the post-image (`bindCertifiedChange`).
- The hazard close (internal/dispatch/hazard.go, `validateIndependentCritiqueReference`): the stamped critic must review the final work round and end at or after it; the stamp is written for a fresh critic dispatch only (review_reference.go), so a follow-up critic round that did review the terminal round cannot move it; `job review-reference-reconcile` is the repair.
- Landing (internal/landing/observe.go): a chain lands only closed, and the certified output is the closed critic's `reviewedTree`.
- The design-critique skill states the stop rule and the five-condition "fixtures as arbiter" exit in prose; nothing in code counts fold-read cycles, records a finding's grain, or refuses a redundant read. Metrics count critique rounds per chain; a landing's chain is on its commit trailer (`Landing-Provenance: chain=`), which the metrics loader does not read.

## 2. The rule, in one paragraph

A critic's read is bound to a read subject: the exact thing it read, typed by the kind of critic. After the read, a chain closes in one of two ways here: the read was clean, or the findings were folded and a new read of the folded subject is clean; a read is never repeated on a subject already read clean, and a chain whose terminal subject equals a subject a critic already read clean closes on that read through one closure object every gate consumes. A design loop closes at round two: a clean second round closes it, a mechanical residue on a falling trajectory becomes fixture obligations the implementation must discharge under its own critique, and a residue that is not mechanical or not falling goes to the human. No fourth round of anything. Closing a folded bounded finding without a new read, through a certification record, is the certification goal's rule, not this page's.

## 3. Decision 1: a read subject, and no read of a subject already read clean

A read subject is a typed identity the ENGINE computes and persists before the critic launches (never reconstructed by the critic), written to the critic round's `review.json` and copied into its return by reference:

- design critic: `{kind: design, designPath, contentDigest, reviewedCommit, declaredOutputsDigest}`;
- live code critic: `{kind: live, implementerRoot, reviewedMember, boundaryBaseProjectTree, reviewedProjectTree, immutableDiffDigest}`, computed at dispatch after any rebase, from the reviewed member's immutable `diff.patch` and `review.json` (validate conformance --stage review), so a follow-up whose base moved gets a new subject;
- commit critic: `{kind: commit, commit, selectedParent, tree, diffDigest}`.

Two subjects are equal only when their kind and every field agree; a live subject never equals a commit subject. At critic dispatch (fresh or follow-up), before any record exists, the dispatcher computes the subject the critic would read and refuses `REDUNDANT_READ` when a folded round of any critic chain reviewing the same implementer chain (or the same design path) read that exact subject and its register, folded through that round, held no open or disputed finding. The refusal names the round, the subject and the rule. An empty follow-up diff is the same case, the subject being unchanged. There is no restoration rule.

The refusal is durable: `artifacts/agents/<implementer root>/reads-refused.jsonl`, one line per event with a stable id (`<root>-<round>-<nonce>`), written under the finding-register lock by the dispatcher and mirrored under the chain's manifest with the round evidence (internal/dispatch/mirror.go learns the file), deduplicated by id when loaded.

The chain then closes on the prior read. `job critique-register-close` writes ONE canonical closure object on the implementation root, `closure: {criticRoot, round, subject, mechanism: "clean"}` (a certification, when its goal lands, adds `mechanism: "certified"` to the same object, never a second authority), and every gate reads that object: the hazard close (`validateIndependentCritiqueReference`) accepts the critic root the closure names when its latest folded round's subject equals the terminal work round's subject, whatever round the stamp `independentCritiqueJobRef` names; the stamp stays as an index that must equal the closure's critic root (a mismatch refuses by name); the final-tree gate (`mergeCritique`) and landing (`chainCertifiedOutput`) read the closure's subject for the reviewed tree. This is the absorbed stamp clause: a follow-up critic round that provably reviewed the terminal subject counts, and no stamp-only dispatch is ever needed. Bar (a) of plans/two-bars-for-changes-design.md is restated in the same landing as: a chain lands only on a tree a critic read clean, named by the closure object.

## 4. Decision 2: design critique fails safe at round two

- A design-critic chain's `reviewRoundLimit` is 2 at every tier where design critique exists (tier 3; tier 2 and tier 1 have no design critique, per the tier ladder): set in the common budget owner (internal/dispatch/build.go's resolution by role) and refused above 2 by `critique-budget-rebind` for the role. R-97-m1e narrows R-60-m1's tier-derived budget for this role only; tier eligibility is unchanged.
- The critic return schema moves to version 5 with a named validator and normalizer: every material finding carries `grain: mechanical|invariant` and, for mechanical, the artifact, the behaviour and the fixture that would prove it; a version-4 return (every historical return) normalizes to `grain: invariant` for its material findings, never mechanical. The critic root keeps `materialByRound[]`.
- Round 2 folded with no material finding closes the loop.
- Round 2 folded with only mechanical findings AND a falling trajectory (round 2's material count below round 1's): `job critique-register-close` defers each as one review obligation on the goal naming its fixture (`test: prove: <fixture>`), the design chain closes with them open, and the implementation that follows discharges each against the chain, the artifact and the test; that implementation runs under its own code critique because its goal is tier 3 (critique-always). The page header records the exit ("closed at round 2 on N fixture obligations").
- Round 2 folded with an invariant finding, with a severe or unproven one, or with a trajectory that is not falling: the close refuses with `cap-exhausted-human-raise` naming the findings and the human's two verbs (`goal accept-risk`, or a re-scope by `goal edit`), and round 3 is refused by the limit. Never a third automatic round.
- The skill's Round Budget section and docs/orchestration.md state the rule in these words; the stop hook's turn verdict prints, for an open critic chain on the seat's goal, its fold-read cycle ("design critique round 2 of 2 folded: 0 material" or "3 mechanical, falling, deferred as obligations").

## 5. Decision 3: the measure

Three figures from `internal/metrics`, printed beside `critique_rounds`, over the landed chains of a window. The loader reads each landing commit's `Landing-Provenance: chain=` and verdict trailers (ordinary `pass bar=a` landings counted; carried landings reported separately, with their original chain), each critic root's `reviews`, each round's persisted subject and fold outcome, and the mirrored refusal events, deduplicated by id.

- `rounds_per_landing`: for each landed implementation chain, its implementer rounds plus the rounds of every critic chain reviewing it; mean and maximum. Target under 3.
- `redundant_reads`: the share of critic rounds whose subject equals a subject an earlier folded clean round on the same implementer chain read. Target under 10 percent.
- `reads_refused`: the count of `REDUNDANT_READ` events in the window.

The zero-yield share stays out: it penalizes clean first reads, which are the aim.

## 6. Scope

Not in scope: the tier ladder, the code-critique round ceiling, the artifact test, the accept-risk path, the landing bars beyond the one restatement of bar (a); and the certification record, which is goal certification-closes-a-folded-bounded-finding (design lane, starting from records/misc/critique-closes-on-folded-proof-design-critique-r1.md and -r2.md).

## 7. The absorbed clauses, restated in full

1. closing-read-follows-the-change-not-the-label: a final round that changes no file, or whose diff is empty against the previously read subject, closes without a new read; the restoration case is refused here (a restored tree is a tree no critic read); a behaviour change dressed as a no-op still demands the read; the refusal names which rule applied (section 3).
2. critique-stamp-follow-up: the closure object accepts a critic chain whose latest folded round reviewed the terminal subject even when the stamp names an earlier round, so no stamp-only critic dispatch is needed (section 3).
3. design-loop-exit-in-the-skill: the skill's Round Budget section and docs/orchestration.md state the enforced stop rule and the D81 exit, and the stop-hook turn verdict prints the current fold-read cycle for an open critic chain (section 4).
4. severity-tiered-rigor-p2 slice 2b: the material stop and the close table read the unresolved set after accepted risks leave it; certification joins that reading when its goal lands.

## 8. Proof

| # | Obligation | Fixture | Home |
| --- | --- | --- | --- |
| 1 | The engine persists a typed read subject per critic kind before launch; a live subject after a moved base differs from the one before; a live subject never equals a commit subject | Go test | internal/dispatch, internal/validate |
| 2 | A critic dispatch onto a subject already read clean is refused before any record exists, named, and written as a durable event that mirrors with the chain; a follow-up whose diff is empty is the same refusal; a changed subject is dispatched | Shell leg in dispatch-fixtures.sh and a Go test | scripts/agents, internal/dispatch |
| 3 | The chain closes on the prior clean read through one closure object; the hazard close, the final-tree gate and landing read it; a stamp that names another critic root refuses by name | Go test | internal/dispatch, internal/validate, internal/landing |
| 4 | A design critic chain's limit is 2 at tier 3; a rebind above 2 refuses; round 2 clean closes; round 2 with mechanical findings on a falling trajectory closes with one obligation per finding naming its fixture; round 2 with an invariant, or a trajectory not falling, refuses with the two verbs and refuses round 3; a version-4 return normalizes to invariant | Go test on the register and a shell leg | internal/dispatch, scripts/agents |
| 5 | `rounds_per_landing`, `redundant_reads` and `reads_refused` computed on a fixture window with known landings (ordinary and carried), chains, subjects and events | Go test | internal/metrics |
| 6 | The skill and docs carry the rule; the turn verdict prints the fold-read cycle for an open critic chain | Shell leg on the stop hook fixture; Go test on the verdict | scripts/agents, internal/goal |

## 9. Slices, each landing alone

0. Schemas and readers, no behaviour change: the typed subject written to `review.json` and returns (readers accept its absence on old rounds); the schema-5 return validator beside version 4, dormant until slice 2; the refusal-event file format and its mirror; the closure object's reader accepting absence; the metrics loader reading landing provenance trailers.
1. The redundant-read refusal with its event, and the close on a prior read through the closure object consumed by every gate (section 3), rows 1 to 3.
2. Design critique at round two (section 4), rows 4 and 6.
3. The measure (section 5), row 5.

## 9b. Build spec after round 3 (D81 exit, 2026-09-13)

Written by a Fable design delegate from revision 3 and the three reads. Under D81 this section and the page are the spec; there is no revision 4. Where this section differs from section 3 (the closure on the critic root rather than the implementation root; the subject key without the boundary base), this section is the build decision.

Spec: plans/critique-closes-on-folded-proof-design.md revision 3, sections 3 and 9, as amended below; the round-3 read is folded as fixtures, not a revision 4. Paths are relative to the metasystem module.

### 1. Decisions taken for the build

**F-1 closure shopping.** The closure lives on the critic root, not the implementation root: `closure: {criticRoot, round, subject, mechanism: "clean"}`, written by `CritiqueChainClose` (internal/dispatch/close.go) in the one record write that sets `chainClosed`, after `CloseCheck` passes, under the finding-register and record locks; `CritiqueRegisterClose` never writes it, and the write is idempotent (absent: write; equal: unchanged; different: refuse by name, never overwrite). No shared slot, so no compare-and-set across chains and no join. Every gate already dereferences the stamp `independentCritiqueJobRef` to the critic root (`validateIndependentCritiqueReference` in hazard.go, `closedCriticReviewedTree` in internal/landing/observe.go); one shared reader `ReadClosure(root map[string]any) (Closure, bool, error)` serves all three. `mergeCritique` (internal/validate/conformance.go, line 1016 region) keeps its union over every critic root, so a second root with an open finding still fails merge. Concurrency is refused, not joined: `CritiqueReadAdmission` refuses `CONCURRENT_READ` when another same-role chain has an equal live subject and an unfolded, non-cancelled latest round. Commit subjects are exempt (HCL-09; the fixture `commit-subject-two` stands). Cut: the reservation store; the residual dispatcher race is bounded by the merge union.

**F-3 subject not bound to the read.** The engine computes the subject from immutable artifacts and writes `rounds/N/subject.json` before launch. Live: `{kind: live, implementerRoot, reviewedProjectTree (from the reviewed member's review.json), diffDigest (sha256 of its diff.patch)}`, provenance `reviewedMember`. Design: `{kind: design, designPath, contentDigest (sha256 of the page bytes in the critic workspace at admission), declaredOutputsDigest}`, provenance `reviewedCommit`. Commit: `{kind: commit, commit, tree, diffDigest}`, provenance `parent`. Before launch, a live subject's reviewed `workspaceRoot` must exist and its project snapshot (`gittree.Workspace.Snapshot("HEAD")`, the projection `reviewStage` uses) must equal `reviewedProjectTree`, else `SUBJECT_MISMATCH` before any record. After the read, at fold, `critiqueSubjectForRound` compares the return's `reviewedTree` (live, commit) or `reviewedCommit` (design) with the persisted subject; a mismatch folds the round as unbound (a synthetic unproven finding beside `foldProtocolError`), never a clean read. The fake adapter (internal/adapter/fake.go) fills `reviewedTree` from `subject.json` when present. Cut: a detached materialization per critic and a post-return workspace check; `boundaryBaseProjectTree` leaves the key because the reviewed tree already names the bytes read, the diff the change judged, and a new review.json field would trip the immutable-review reuse rule.

**F-4 the two prior-read cases.** Equality is over identity fields only; `reviewedMember`, `reviewedCommit` and `parent` are provenance. A no-op implementer follow-up therefore yields an equal live subject, and the hazard close accepts a closure whose subject equals the terminal work round's subject even when `reviews` names the earlier member and `endedAt` precedes it (the fresh-chain, distinct-session and effort checks stay). Closure ownership is uniform, on the critic root for every kind, so a standalone design chain closes at round 1 when its folded register is clean, and a follow-up on the unchanged page is `REDUNDANT_READ`. The round-2 rule is slice 2.

**F-5 slice 0 activates behaviour.** Slice 0 is types and decoders only: readers accept absence, nothing is written, no schema or record whitelist changes, no refusal token appears. Cut from slice 0: the schema-5 validator (slice 2), the mirror source (slice 1), the metrics loader (slice 3). Subjects never enter model returns: the return's existing `reviewedTree` or `reviewedCommit` is the reference validated against `subject.json`, so schema v4 is untouched by both slices.

**F-6 refusal durability.** `CritiqueReadAdmission` appends the event to `artifacts/agents/<prior clean critic root>/reads-refused.jsonl` under the finding-register lock by read-modify-write through `atomicfile.WriteText` (fsync and directory sync), releases the lock, and dispatch.sh calls `mirror_record <prior critic root>`; `mirrorSources` (internal/dispatch/mirror.go) carries `reads-refused.jsonl` from the payload directory when present. That root is terminal, so the manifest-keyed mirror is the idempotent transaction. On mirror failure the refusal stands, the event stays local, `mirror_fail` marks the record as today, and the next close, reap or `job mirror` carries it. Cut: an event-keyed mirror transaction; deduplication by id lives in the reader.

**Deferred.** F-2 (obligations as proof, `goal discharge-review-obligation`): slice 2. F-7 (trajectory and tier enumeration in build.go and rebind): slice 2. F-8 (carried landings' original chain, internal/landing/carried.go): slice 3.

Consequence: after slice 1 every follow-up onto a clean fake critic chain is refused, so `leg_happy_follow_up`, `resume-collision` and `close-race` in scripts/agents/dispatch-fixtures.sh must change the design page before following up. That is the rule biting, not a defect.

### 2. Slice 0 fixture obligations

Home internal/dispatch unless stated. Every row asserts absence is accepted or nothing new is emitted.

| id | obligation | test | finding |
| --- | --- | --- | --- |
| S0-1 | Two subjects of different kinds are never equal; two live subjects differing only in `reviewedMember` are equal; differing in `reviewedProjectTree` or `diffDigest` are not | `TestReadSubjectEqualityIgnoresProvenance`, `TestReadSubjectKindsNeverEqual` | F-3, F-4 |
| S0-2 | `readRoundSubject` on a round directory without subject.json returns present=false and no error; a malformed file is an error | `TestReadRoundSubjectAbsentOnOldRounds` | F-5 |
| S0-3 | `ReadClosure` on a root record without `closure` returns present=false and no error; a closure with an unknown mechanism is an error | `TestReadClosureAbsentOnOldRoots` | F-5 |
| S0-4 | `LoadReadRefusals` over a missing file returns an empty slice; two files carrying the same id yield one event; a malformed line is an error | `TestLoadReadRefusalsMissingAndDeduplicated` | F-6 |
| S0-5 | Advancing and closing a register through the existing helpers (`writeCriticRound`, `setCriticSubject`) leaves the root record without a `closure` key, writes no subject.json and no reads-refused.jsonl | `TestSliceZeroEmitsNothing` | F-5 |
| S0-6 | `validate conformance --stage review` writes review.json with exactly the keys diffArtifact, implementerJob, reviewedTree | `TestReviewStageWritesOnlyThreeFields` (internal/validate) | F-5 |
| S0-7 | No new refusal token exists: `go test ./internal/refusal` passes with register.go untouched | existing HCL03 tests | F-5 |

### 3. Slice 1 fixture obligations

Shell legs live in scripts/agents/dispatch-fixtures.sh cluster c beside `leg_review_target_flag_runtime`.

| id | obligation | test | finding |
| --- | --- | --- | --- |
| S1-1 | Dispatch writes rounds/1/subject.json of the right kind for a code critic on an implementer member, a commit subject and a design critic, before the record exists | shell leg `subject-persisted`; `TestComputeReadSubjectByKind` | F-3 |
| S1-2 | A no-op implementer follow-up (same tree, same diff) computes an equal live subject; a changed tree computes a different one | `TestLiveSubjectFollowsTheChangeNotTheMember` | F-4 |
| S1-3 | Reviewed worktree edited after its review: fresh critic dispatch refuses `SUBJECT_MISMATCH`, exit 11, no record, no payload | shell leg `subject-mismatch` | F-3 |
| S1-4 | A return whose reviewedTree differs from subject.json folds as unbound: register gains a synthetic unproven finding; close refuses | `TestFoldRefusesUnboundReturn` | F-3 |
| S1-5 | Follow-up onto a chain whose last folded round was clean refuses `REDUNDANT_READ` naming round, subject digest and rule, exit 11, no child record or payload | shell leg `redundant-follow-up` on `happy`; `TestReadAdmissionRefusesCleanSubject` | row 2 |
| S1-6 | Fresh chain onto a live subject another chain read clean refuses `REDUNDANT_READ`; the same for a design page | shell leg `redundant-fresh` | row 2 |
| S1-7 | A changed subject is dispatched: editing the page makes the design follow-up run; the three existing follow-up legs are updated to edit first | shell legs updated | row 2 |
| S1-8 | Second same-role chain on an equal live subject while the first is unfolded refuses `CONCURRENT_READ`; `commit-subject-two` still runs | shell leg `concurrent-live`; `TestReadAdmissionExemptsCommitSubjects` | F-1 |
| S1-9 | The event has a stable id, lands in reads-refused.jsonl under the prior critic root and in that chain's mirror manifest; with `.mirror-fail-once` the refusal still exits 11, the local event exists, and a later `job mirror` carries it | shell leg `refusal-durable`; `TestReadRefusalAppendUnderLock` | F-6 |
| S1-10 | `CritiqueChainClose` writes the closure once on a clean root; a repeat is unchanged; a different closure refuses by name; a root with open findings gets none | `TestCloseWritesOneClosure` | F-1 |
| S1-11 | Hazard close accepts a stamped critic whose closure subject equals the terminal work round's subject although `reviews` names the earlier member and `endedAt` precedes it; refuses when the closure is absent or its subject differs | `TestHazardCloseReadsClosureSubject` | row 3 |
| S1-12 | Merge requires each closed critic root's closure subject tree to equal finalTree and still unions every root: a second root with an open finding fails merge | `TestMergeCritiqueClosureAndUnion` (internal/validate) | F-1 |
| S1-13 | Landing selects the review whose reviewedTree equals the stamped closure's subject tree, falling back to today's order | `TestChainCertifiedOutputPrefersClosure` (internal/landing) | row 3 |
| S1-14 | `REDUNDANT_READ`, `CONCURRENT_READ`, `SUBJECT_MISMATCH` are rowed in internal/refusal/register.go with real sites | existing HCL03 tests | hygiene |
| S1-15 | A design chain folded clean at round 1 closes with closure round 1 | `TestDesignChainClosesAtRoundOne` | F-4 |

### 4. Codex build brief, slice 0

You are building slice 0 of goal critique-closes-on-folded-proof in a git worktree of the metasystem module: readers and types only, provably behaviour-free. No existing production file changes, nothing new is written to disk by production code, and no UPPER_SNAKE string literal appears in non-test Go under internal/dispatch (the refusal register test walks those literals). Do not commit.

New files, package `dispatch`:

- `internal/dispatch/read_subject.go`
  - `type ReadSubjectKind string`; `const (SubjectLive ReadSubjectKind = "live"; SubjectDesign = "design"; SubjectCommit = "commit")`.
  - `type ReadSubject struct { Kind ReadSubjectKind; ImplementerRoot, ReviewedMember, ReviewedProjectTree, DiffDigest string; DesignPath, ContentDigest, DeclaredOutputsDigest, ReviewedCommit string; Commit, Parent, Tree string }` with lowerCamel json tags and `omitempty` on every string.
  - `func (s ReadSubject) Equal(o ReadSubject) bool`: kinds must match; live compares ImplementerRoot, ReviewedProjectTree, DiffDigest; design compares DesignPath, ContentDigest, DeclaredOutputsDigest; commit compares Commit, Tree, DiffDigest. Provenance fields never participate.
  - `func (s ReadSubject) Digest() string`: sha256 hex over `canonicalJSON` of the identity tuple `[kind, fields in the order above]`.
  - `func decodeReadSubject(value any) (ReadSubject, bool, error)`: nil or absent returns (zero, false, nil); a non-object or unknown kind is an error.
  - `func readRoundSubject(agents, rootJob string, round int64) (ReadSubject, bool, error)`: reads `<agents>/<rootJob>/rounds/<round>/subject.json` through `readObject`; `os.IsNotExist` returns (zero, false, nil).
- `internal/dispatch/closure.go`
  - `const closureField = "closure"`; `type Closure struct { CriticRoot string; Round int64; Subject ReadSubject; Mechanism string }`.
  - `func ReadClosure(root map[string]any) (Closure, bool, error)`: absent returns (zero, false, nil); Mechanism must be `"clean"` (slice 1 adds nothing else here; the certification goal adds `"certified"` later); Round must be a positive integer via `numInt`; CriticRoot must match `validJobID`.
- `internal/dispatch/reads_refused.go`
  - `type ReadRefusal struct { ID, Reason, Role, CriticRoot string; Round int64; Subject ReadSubject; RefusedAt string }`, lowerCamel json tags.
  - `func LoadReadRefusals(paths ...string) ([]ReadRefusal, error)`: one JSON object per line, blank lines skipped, missing files skipped, malformed line is an error naming path and line, deduplicated by ID keeping the first, returned in first-seen order.

Tests (section 2): `internal/dispatch/read_subject_test.go` holds S0-1 to S0-5; S0-5 uses `writeCriticRound`, `setCriticSubject`, `CritiqueRegisterAdvance` and `CritiqueRegisterClose` from finding_register_test.go, then asserts no `closure` key on the root record and no `subject.json` or `reads-refused.jsonl` in the payload. `internal/validate/conformance_review_shape_test.go` holds S0-6: after a review-stage run, unmarshal review.json into `map[string]any` and assert exactly three keys. Existing style: table tests, `t.Helper()`, package helpers for temp repos, no new utilities.

Verification, in order, all green:

```
go build ./...
go vet ./internal/dispatch ./internal/validate
go test -race -count=1 ./internal/dispatch ./internal/validate ./internal/refusal
scripts/agents/go-gate.sh --fast
git status --porcelain   # only the five new files
```

Write `artifacts/reports/codex-ccf-slice0-result.md`: files added, test names with pass lines, the verification tail, and any point where the spec could not be followed literally. Do not commit; do not edit any existing file; do not touch plans/ or records/.

### 5. Open questions for Wido

none

## 10. Critique record

Round 1 (2026-09-13, Codex gpt-5.6-sol, read-only): eight material findings, all folded. 1: the boundary was path-keyed (a fold that fixes the finding and changes unrelated policy in the same file passed; a hand-edited record could widen it; directories and renames were undefined): now derived from the register, one file per artifact, renames expanded, and every hunk attributed to exactly one finding. 2: the evidence model could not represent the proof contract (project subtree versus whole tree, joined attempts, no finding-to-test mapping, the owner validation bypassed): now both trees are recorded, evidence is sorted attempt ids validated by the existing owner validator, and the manifest maps each finding to a test by execution identity. 3: the restoration rule was unsafe (reversing approved hunks yields an unread tree) and tree equality conflated live and commit subjects and collided with the hazard close: now a typed read subject, exact equality only, no restoration rule, and a persisted closure binding every gate consumes. 4: the transition was incomplete (partial certification, crash half-state, status versus resolution, landing's patch replay dropped): now exact coverage of the eligible set, an idempotent write order under the lock, `resolved/certified`, `bindCertifiedChange` kept, bar (a) amended. 5: a test-only fold could prove nothing: now the `proof-gap` kind with additive test hunks, and a `correction` must change its production artifact. 6: decision 4 lacked the machinery: now the grain field, the trajectory, the role's cap in the budget owner and the rebind, R-97 over R-60 for the role, and the code critique after the fixture exit named. 7: the metrics were not computable: now the refusal event is persisted and the loader reads landing provenance, subjects and fold outcomes. 8: the slices did not land alone: reordered with a schema slice first. Sound as built, per the critic: no certification of severe or unproven findings; a certification is not a read; every consumer recomputes and fails closed; the stop at zero material findings and no fourth round; the refusal before a successor record; the landing patch replay kept.

Round 2 to revision 3 (2026-09-13, Wido's word: option (a)): the certification record and its four contract findings (1, 2, 4 in its certification half, 5) leave for goal certification-closes-a-folded-bounded-finding; the folds that apply to the kept rules are in: role-discriminated read subjects persisted by the engine before launch (finding 3); one closure object on the implementation root, the stamp kept as an index (finding 4); the falling-trajectory condition, the tier enumeration and a schema-5 return with grain, version-4 returns normalizing to invariant (finding 6); durable refusal events with ids, mirrored and deduplicated, carried landings reported separately (finding 7); a schema slice first, dormant until its consumer lands (finding 8).

## 11. Escalation after round 2 (2026-09-13, for Wido; answered the same day: option (a), this revision)

The round-2 findings, in the seat's words: (1) a textual hunk is not a stable or complete unit of change (binary, mode and whole-file changes have none; context drift changes its digest; one edit can serve two findings; the finding's test lives in a file other than its artifact); (2) a passing test does not prove a finding (an empty test passes; a whitespace edit can cite an old test; the owner validator proves execution, not relevance); (3) one read-subject shape cannot serve design critics, live code critics after a rebase, and commit critics; (4) the closure would have two authorities (the existing stamp and the new binding) and bar (a)'s wording binds a certification to a read that had findings; (5) the write order is not idempotent as written (the timestamp differs on retry; the register projection is undefined; the locks do not cover every record written); (6) the round-two exit omits R-97's falling-trajectory condition, the cap is not enumerated per tier, and the return schema (v4, strict) cannot carry optional grain fields; (7) the refusal events and landing provenance are not durable or scoped (mirroring, pruning, carried landings); (8) slice 0 is behavioural after all.

The seat's reading: findings 1, 2, 4 and 5 are the certification's proof contract, and a third revision would be a third prose budget on the same question (D81, and this page's decision 4); findings 3, 6, 7 and 8 are buildable amendments with the critic's shape. What is buildable now without a new proof contract: the redundant-read refusal and the close on a prior clean read under one closure object that every gate consumes (section 5, with finding 4's one-object amendment and finding 3's role-discriminated subjects); the design-critic cap at two with R-97's falling-trajectory condition and a schema v5 for the grain (section 6 with finding 6); the measure on durable events (section 7 with finding 7). Those are the second and third rules of the goal and the stamp clause, and they remove the reads the audit counted as redundant. The certification record (decisions 1 and 2) is the first rule and the largest saving, and it is the part whose proof contract is not settled.

The decision, with a recommended default: (a) narrow this goal to the buildable parts (sections 5 to 7 with the round-2 amendments), and open the certification record as its own design-lane goal that starts from the critic's amendments (change units keyed by blob and mode, counterfactual test runs, a deterministic core digest, one closure object), scheduled behind this one; or (b) fund revision 3 of this page with all eight amendments, a third prose round on the certification; or (c) accept the risk of building decisions 1 and 2 as revision 2 states them. The seat recommends (a): it lands the two rules Wido has already ruled on and the measure that shows whether they pay, and puts the unsettled contract on its own page where a falling trajectory can be seen.
