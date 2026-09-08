# A reviewed chain can land after main changes the same file

Revision: 1 — 2026-09-08. Design authoring only; independent critique and the orchestrator's dispositions remain pending.

Contract: `metasystem/plans/goals/chain-landing-after-base-move-recertifies.md`, revision 4. The observable acceptance is one previously reviewed chain, with main subsequently changing a different part of its file, passing the landing evaluator at bar a. Changing that merged candidate without certification must still produce `chain-output-mismatch`.

## Decision and limits

Choose the **mechanical no-overlap proof**, performed through a named conformance stage before landing. The coordinator merges the current main tree into the stopped chain's worktree, recomputes conformance against that main tree, and obtains a separate immutable recertification record. The critic's original reviewed tree and round artifacts remain unchanged. Landing compares against the mechanically derived merged tree.

There is no merge-only critic lane in this revision. Proof failure refuses and parks; it does not dispatch a critic, reopen a register, or acquire another review allowance. An overlapping change can proceed only through the existing implementation and review process with its existing budget and human-reserved remedies. This decision addresses the clean, disjoint changes in the reported incidents without adding an exception to review accounting.

The proof establishes preservation of two textual changes, not their semantic independence. Candidate verification remains necessary. Binary changes on both sides, structural conflicts, and ambiguous provenance cannot use the exemption. Those limits are explicit refusals, not invitations to hand-merge and label the result certified.

“Main” means the coordinator's actual integration target, resolved to a full commit object identifier. It does not mean an arbitrary caller-supplied branch. A node refreshes its normal remote-tracking state before preparation, under its existing authority. Neither this stage nor the evaluator pulls, pushes, resolves peer conflicts, changes a claim or approval, or widens delegate permissions.

## Grounding in this tree

The following are code-reading evidence, not a reproduction run:

| Existing owner | Fact that constrains this design |
| --- | --- |
| `metasystem/internal/landing/observe.go:394` | `bindCertifiedChange` applies the stored patch to current HEAD, then compares its changed-path entries with the critic's old `reviewedTree` at lines 408–417. The refusal precedes the candidate equality check. |
| `metasystem/internal/landing/observe.go:443` | `pathChangeDigest` already binds sorted path names, before/after object identifiers, modes, and absence. The new path can retain that candidate binding and existing extra-path carriage rules. |
| `metasystem/internal/validate/conformance.go:218` | Review computes its boundary from `merge-base(target HEAD, worktree HEAD)`. The dispatch record's `baseSha` is not necessarily that base. |
| `metasystem/internal/validate/conformance.go:321` and `:487` | Review artifacts are immutable. Their stored fields are `diffArtifact`, `implementerJob`, and `reviewedTree`; the actual boundary base is absent. Re-running ordinary review after merging cannot replace them. |
| `metasystem/internal/validate/conformance.go:528` and `:1067` | Merge conformance reads a committed final tree and requires a closed independent code-critic return for that exact tree. Fixing the evaluator alone would leave this gate blocked. |
| `metasystem/internal/validate/authorization.go:59` | Mission integration authorization is issued only after conformance passes. Its existing equation is exact patch applied to exact base equals certified output. Its guardrail and expected-tree checks remain requirements. |
| `metasystem/internal/dispatch/followup_rebase.go:31` and `metasystem/scripts/agents/dispatch.sh:2238` | Follow-up preparation concerns a new work round; it rebases when intervening commits touch a chain path and otherwise prints `WORKTREE-BEHIND`. It is not the owner of closed-chain recertification. |
| `metasystem/internal/dispatch/build.go:615` and `metasystem/internal/dispatch/finding_register.go:156` | Dispatch freezes `reviewRoundLimit`; register advancement updates `criticRoundsConsumed`. This design neither writes nor bypasses either field. |
| `metasystem/scripts/agents/land.sh:404` | The driver currently commits through evaluation and subsequently fetches/rebases, including push retries. The recertified path must not take that post-evaluation rebase. |

The three sightings in the brief are motivating evidence supplied by the coordinator. This authoring round did not rerun their historical landings.

## Named path and ownership

Add `recertify` to the existing conformance command:

```text
metasystem validate conformance --root <integration-project-root> --stage recertify --job <certified-implementer-job>
```

The job must be the implementer job named by the original conformance artifact and covered by the closed code-critic chain. A wrong round is refused with the expected job identifier; it is not silently replaced. The root implementation chain must be closed, and no implementation or critic member may be active. Resolve its recorded worktree rather than accepting an unrelated writable directory. Run from the coordinator's integration checkout, with the same checkout authority and mission fences as ordinary conformance. The command is not a delegate permission to write coordinator evidence.

Responsibilities have three existing homes:

- `internal/validate` owns source selection, conformance, the recertification record, and verification of that record. Its new `recertification.go` exposes a read-only verifier for the landing consumer. Ordinary review and merge behavior without a recertification remain intact.
- `internal/gittree` owns the deterministic three-tree operation and safe materialization. Its new `disjoint_merge.go` has no job, budget, claim, or critic policy.
- `internal/landing` owns candidate binding. The CLI and the two landing scripts carry the explicit record reference and stop transport from changing a proved result. They do not implement overlap policy in Bash.

All paths in those three bullets are beneath `metasystem/`. No new service, dispatcher role, review-register schema, or generic proof registry is needed.

### Frozen inputs and the missing historical base

Use these names throughout the implementation:

| Name | Meaning |
| --- | --- |
| `H` | Recorded source worktree HEAD commit, captured before materialization. |
| `T` | Integration checkout HEAD commit captured at preparation; `Ttree` is its project tree. |
| `A`, `B` | The sole merge-base commit of `H` and `T`, and its project tree. |
| `R`, `P` | Original critic-reviewed project tree and the exact stored conformance patch. |
| `S` | Literal changed paths between `B` and `R`, including both endpoints of a rename represented as deletion/addition. |
| `M`, `Q` | Mechanically merged project tree and freshly computed patch from `Ttree` to `M`. |
| `C` | Actual project candidate supplied to landing, possibly with separately permitted register carriage. |

Resolve the root's `independentCritiqueJobRef`, its actual terminal critic round, and the immutable return under that critic root's round directory. Require a code critic, its closed register and normal independence/exhaustion checks, and a return naming `R`. Join that return to an original review whose `implementerJob` is the requested job. Do not use landing's highest-round fallback for this new path. Several storage copies with identical review and patch bytes are equivalent; select the lowest numeric storage round. Conflicting copies refuse.

Recover the historical base by running `merge-base --all H T` with the repository's scrubbed Git environment. Require exactly one result `A`. Then require **`Apply(B, P) == R` over the entire project tree** and derive `S` from `B → R`. This is the proof that the recovered base belongs to the stored patch. Do not substitute `baseSha`, assume HEAD is the review base, reverse-engineer a plausible base from hunk text, or search arbitrary ancestors until something applies. Missing objects, multiple merge bases, an empty change, or failed equality produce `chain-recertification-base-unproven`.

This gives existing chains a path without rewriting their review files. If a historical chain's worktree history has already been rewritten so this base cannot be established, it cannot take the shortcut. That is a disclosed compatibility limit; reconstruction without evidence is not part of this design.

Before changing the worktree, require its project snapshot to equal `R`, or exactly the independently computed `M` for an idempotent retry. Verify the whole-repository snapshot too: an original `R` worktree has no changes outside the project relative to `A`; a retry at `M` has none relative to `T`. Refuse unresolved index entries, active Git operations, and other worktree changes. Preserve a real index containing ordinary staged portions of the reviewed work: its entries must come from `H` or the admitted snapshot, with no third, hidden staged version. Do not discard staged-only edits.

### Precisely what counts as overlap

Compare **net** changes `B → R` and `B → Ttree`. An intervening main change that was fully reverted has no remaining hunk. This proof concerns the tree being integrated, not historical touches that no longer exist.

For a literal path changed on only one side, copy that side's entire resulting entry, including deletion, mode, and blob. For a path changed on both sides, proof is available only when all three entries exist, have the same regular-file mode (`100644` or `100755`), and all three blobs contain no NUL byte. Bytes need not be UTF-8. A both-sided mode change, addition, deletion, symlink, gitlink, binary blob, or rename endpoint is unsupported. Identical both-sided edits are not deduplicated into an exemption.

Hunks have one canonical source: Git's Myers minimal line diff over the raw blobs, with zero context. The `gittree` operation must fix these settings explicitly, in addition to its existing scrubbed environment and object-replacement protections:

```text
--text --no-renames --diff-algorithm=myers --minimal
--no-indent-heuristic --unified=0 --inter-hunk-context=0
--no-ext-diff --no-textconv --no-color
--src-prefix=a/ --dst-prefix=b/
```

Run blob comparisons outside attribute-bearing worktrees, with system/global Git configuration disabled and no repository-local diff configuration or attributes loaded. In particular, local `diff.algorithm`, indent heuristics, whitespace-ignore settings, external drivers, and text conversion must not select different hunks. Record the Git version and proof version. The canonical invocation defines the hunks; implementations must not substitute another diff library or enlarge them by context. Consumption recomputes them. A different Git version may consume a proof only if recomputation reproduces its canonical hunk manifest and merged tree; otherwise refuse as unproven, without silently reissuing evidence.

For each header `@@ -u,n +v,m @@`, default omitted counts to one. Express its edit using **base line-boundary coordinates**: a nonempty old range consumes `[u-1, u-1+n)` and produces the indicated new-side lines; a zero-count old range inserts at boundary `u`. Lines retain their literal LF bytes; an unterminated final line remains distinct. Validate all ranges and require reconstruction of each side from its base and parsed edits to reproduce that side's blob exactly, including final-newline changes.

For the overlap decision only, the footprint of a consuming edit is the **closed** interval `[u-1, u-1+n]`; an insertion's footprint is the single point `{u}`. A chain hunk overlaps a main hunk iff their closed footprints intersect. Apply this comparison only across sides in the same path.

Thus edits to the same base line overlap; insertions at the same gap overlap; an insertion on either boundary of a replaced/deleted range overlaps; even immediately adjacent consuming edits overlap. Two replacements separated by at least one unchanged base line do not overlap. Boundary contact deliberately refuses because the shortcut must not decide insertion order or attachment to a deleted range. Empty files, the first boundary, and the last boundary obey the same formula.

If no cross-side footprints intersect, construct the merged blob by walking `B` once in base order, copying unchanged bytes and emitting each side's replacement bytes at its own edit. Reconstructing either input separately has already been checked. Do not use fuzzy patch application, rerere, custom merge drivers, `ours`/`theirs`, or a model's interpretation of clean merging. Build `M` from `Ttree` with these merged entries and the unchanged chain-only entries. Tree path collisions and any unsupported materialization shape refuse. Require `ChangedPaths(Ttree, M) == S` and `Apply(Ttree, Q) == M`; accepting an empty or silently reduced chain is not this path.

The hunk manifest is an ordered array by raw path bytes, then side (`chain`, `main`), then old/new hunk ranges. It records path, side, old start/count and new start/count. Blob identities in `B`, `R`, and `Ttree` bind replacement bytes. Malformed diff output and a Git execution failure are proof failures, never an empty hunk list.

### Merge, conformance, and publication

This is a **content merge into the chain worktree**, not a requirement for a delegate to create a commit. Delegates here normally leave dirty worktrees. Preserve `H`, the real index, branch refs, and the original round artifacts. The coordinator materializes the proved result using `gittree.MaterializePaths`; it does not run ordinary review against an imported-main diff. On a nested installation, graft `M` into `T`'s whole-repository tree so sibling changes are exactly main's, and check conformance's outside-project boundary against `T`.

Before the first write, anchor the original whole-repository snapshot and the intended whole-repository result under `refs/metasystem/landing/recertifications/<input-digest>/source` and `/merged`. These are object-retention refs, not shippable branch commits. Preflight every materialized path: an ignored/untracked obstruction absent from the admitted snapshot must not be overwritten; symlink traversal, unsupported gitlinks, and unsafe file/directory transitions refuse. Use the existing no-follow materialization protections. After writing, snapshot with the intended tree as the membership seed and require equality with the complete intended result. Any partial write leaves the anchors and a named recovery refusal; it publishes no passing record or authorization. Never clean a foreign worktree or reset away an unknown edit.

Recompute the actual `Q` and its path boundary from `Ttree` to the resulting snapshot, using the same cumulative declarations, path restrictions, control-plane check, and outside-project fence as conformance review. Imported main changes are the base, not new implementer output. Re-run normal code-critic closure, independence, and exhaustion validation against **`R`**. The added proof links `R` to `M`; do not change the critic's return to pretend it reviewed `M`.

Publish under:

```text
metasystem/artifacts/agents/landing/recertifications/<chain>/<input-digest>/record.json
metasystem/artifacts/agents/landing/recertifications/<chain>/<input-digest>/diff.patch
```

For adopted repositories the same locations are installation-relative. Stored path fields are repository-relative, preserving a nested installation's prefix. `internal/validate` is the sole record writer. A record has schema version 1 and contains:

- Proof kind `disjoint-hunks-v1`, Git version, canonical hunk manifest, and its digest.
- Root chain and certified implementer job; original review/patch locations and byte hashes; critic root, terminal round, return location and byte hash.
- `sourceHead=H`, `baseCommit=A`, `baseTree=B`, `reviewedTree=R`, `targetCommit=T`, `targetTree=Ttree`, `mergedTree=M`, and the two anchored whole-repository snapshot trees.
- Sorted `certifiedPaths=S`, the relative `diffArtifact`, and `mergedPatchDigest` for `Q`.
- `inputDigest` and `recordDigest`. Use the repository's canonical `wiredoc` encoding and SHA-256. `inputDigest` covers the immutable source references/hashes, `H`, `A`, `B`, `R`, `T`, `Ttree`, and proof kind. `recordDigest` covers all record fields except itself. A hash detects inconsistency; it does not replace the recomputed proof.

Write the patch and record into an owned temporary directory, flush them, and publish the directory atomically under a per-chain lock. A pre-existing identical record is idempotent; different bytes for the same inputs refuse, never overwrite. Recheck source posture, target commit, and source evidence hashes before publication. Locks and all Git calls use existing bounded local-operation facilities; the entire preparation has a five-minute wall-clock ceiling including lock waits, and a smaller enclosing job/mission limit still wins. On timeout, terminate only owned children, retain recovery anchors, and publish no success. This is a hang detector, not a retry budget.

The stage returns exit zero plus `recertification=<record path>`, `certifiedTree=<M>`, and `diffArtifact=<Q path>` only after all these checks. Failures return nonzero with a stable reason and specific explanation. The original review bytes, review round number, `reviewRoundLimit`, `criticRoundsConsumed`, and goal budget are unchanged even when all ordinary review rounds have been spent.

Recertification artifacts and their anchored objects remain chain evidence. Include them in the existing durable-mirror retention boundary; worktree removal must not remove the only copy. This change adds no automatic pruning of those refs. Missing evidence later means refusal.

## Re-aim both gates without relaxing their binding

Add the optional `--recertification <record path>` argument to `validate conformance --stage merge`, `landing observe`, `scripts/agents/commit.sh`, and `scripts/agents/land.sh`. It requires the same `--chain` on landing and cannot combine with a direct-fix class other than already permitted register carriage. It is illegal on ordinary review and on direct fixes. Record paths must resolve to the canonical location for the declared chain and digest; reject symlinks, traversal, unknown fields, or conflicting identifiers.

The read-only verifier in `internal/validate` loads the referenced original evidence, rechecks its closure and hashes, verifies `B + P = R`, recomputes the no-overlap merge and manifest, and verifies `Ttree + Q = M`. It recomputes the conformance boundary as well; a JSON `pass` field is never authority. It needs retained objects and source evidence, not a still-present delegate worktree. Both merge conformance and landing consume this one verifier.

**Merge conformance:** without the argument, keep the existing exact committed-tree rule. With a valid record, require the final source tree to be `M`, either as its committed tree with no extra dirty changes or as the exact materialized worktree snapshot described above. Pass normal critic checks over `R` and require the independent mechanical link to `M`. This narrowly allows the existing noncommitting delegate workflow; an arbitrary dirty tree remains inadmissible. Use `T` as the boundary for downstream authorization, never `A` or unchanged source HEAD `H`.

Mission authorization remains at its existing issuance point and keeps its schema and consumers: it binds `baseTree=Ttree`, `reviewedTree=M`, and `patch=Q`, with the existing exact-apply equation, sequence-point, supersession, claim, incarnation, and guardrail requirements. The authorization's output-tree field describes the certified merged output; the original code critic still names `R` in its own immutable evidence. A mission whose target is not an existing lawful expected-tree point parks. A required warden review is not carried forward by this code-critic proof: existing exact-tree warden checks must pass for `M`, or authorization refuses. No mission wall or approval exception is added.

**Landing:** freeze the integration HEAD once and require its full commit to be `T`. After verifying the record, compare:

```text
expected = pathChangeDigest(Ttree, M, S)
actual   = pathChangeDigest(Ttree, C, S)
require actual == expected
```

The reviewed-postimage check now occurs at **`B → R`**, and the candidate check at **`Ttree → M`**. It does not compare merged file entries with pre-merge entries. Use the original `S`, not a candidate-derived path list. All blobs, modes, additions, deletions, and absences in `S` remain bound. A missing chain edit, extra edit anywhere in a certified file, reverted main hunk, or altered merged patch therefore refuses. Changes outside `S` remain subject to the existing class, goal ownership, and register-carriage checks. No projection excludes a part of a certified file.

Keep successful bar-a classification `closed-chain`; add the recertification digest and target commit to provenance. Candidate mismatch retains `chain-output-mismatch`. Invalid proof and stale target use the named reasons below and are hard refusals for this explicitly selected path even where ordinary observation mode is enabled. An invalid or absent explicitly named record cannot fall back to the old path, direct-fix bar, or observation-only admission.

A recertified landing requires a fresh existing-format test receipt for the **actual `C`**, after permitted carriage is added. The old receipt for `R`, or one for bare `M` when `C` differs, is invalid. Run the same required checks for that chain's gate width; the full-width exact-command condition remains unchanged. Do not replace it with the authoring brief's fast gate. This design does not make textual disjointness a substitute for execution evidence.

### Main moving again, including during push

For the explicit recertified path, `land.sh` fetches and checks the intended remote branch **before** commit. Its remote-tracking tip and local integration HEAD must both equal `T`; divergent/unpushed local bases are parked rather than reconciled by this command. Recheck local HEAD before committing and verify the resulting commit has parent `T` and the evaluated candidate tree. A race yields no push and no success report. Use the wrapper's existing guarded recovery for its own newly created commit; never rewind a concurrently moved ref.

After commit, push that exact commit through the existing normal fast-forward transport. **Do not invoke `rebase_origin` on this path.** A non-fast-forward rejection parks with `chain-recertification-target-moved`; retain the local commit and proof for inspection, and do not take the three-attempt fetch/rebase loop. Network failure retains the existing failed-transport behavior. The ordinary landing path's transport policy is not redesigned by this change.

An unattended node makes one preparation attempt for one frozen target. It does not automatically pull and loop after a refusal. It records the reason through its existing stream-park mechanism and may work another claimable item. A later externally initiated retry must refresh the target, reconstruct the original admitted source posture if necessary, issue a new proof, create a receipt for the new candidate, and pass the evaluator again. Neither `--no-verify`, observation-only acceptance of a failed proof, force pushes, nor a manually asserted merged tree is a recovery path.

## Failure contract

These are structured outcomes of the new path; each diagnostic also names the chain, target, relevant path when applicable, and the preserved recovery objects. Register new operator refusals in the existing refusal inventory; do not create a second catalogue.

| Reason | Meaning and required action |
| --- | --- |
| `chain-recertification-base-unproven` | Original patch/base/review linkage cannot be established. Leave evidence intact and return to the existing reviewed-work process. |
| `chain-recertification-source-changed` | The chain is active, its source evidence changed, or its worktree/index contains additional edits. Stop before overwriting anything; the coordinator must account for those edits. |
| `chain-recertification-overlap` | Canonical cross-side hunk footprints intersect. Emit both ranges and park for the existing review/conflict process. |
| `chain-recertification-unproven` | Unsupported file shape, malformed/missing proof, unreadable Git objects, ambiguous source selection, or an operation that could not run. Explain which condition; no inferred clean merge. |
| `chain-recertification-worktree-incomplete` | Materialization failed after writes began. No passing record; preserve original/result anchors and park for controlled recovery. |
| `chain-recertification-timeout` | The preparation or lock wait reached its ceiling. Preserve evidence, stop owned work, and park without automatic retry. |
| `chain-recertification-target-moved` | Target identity changed during preparation, verification, commit, or push. No reuse of old proof or receipt on a new tree. |
| Existing `chain-output-mismatch` | Candidate entries do not match the mechanically certified merged output. Refuse the candidate. |

Conformance, receipt, warden, authorization, class, and goal-ownership failures retain their existing reasons. The unattended caller parks on them too. A human-present coordinator can use existing lawful correction paths; this design supplies no special override.

## Canary, refusal twin, and run ceilings

Implementation and fixture logic belongs in Go. The three runs below are proposed future tests, not evidence executed in this design round. Run each only for the behavior it proves; do not wrap a correction loop around a full bed or battery.

| Fixture and smallest run, from `metasystem/` | Required observation | Ceiling |
| --- | --- | --- |
| New `TestChainLandingRecertifiesAfterBaseMove` in `metasystem/cmd/metasystem/landing_verbs_test.go`; `go test ./cmd/metasystem -run '^TestChainLandingRecertifiesAfterBaseMove$' -count=1 -timeout=120s` | A real temporary Git repository and recorded implementation/code-critic chain exercise review creation, the new recertify command, merge conformance, a candidate receipt, and `landing observe`. Main and chain change well-separated lines in one source file. Before recertification the same merged candidate yields `chain-output-mismatch`; afterward it contains both edits and passes bar a. The paired subtest edits a third line of that certified file, creates an otherwise valid receipt for the tampered tree, and still gets `chain-output-mismatch`. | 120 seconds for the whole test; all subprocesses inherit the remaining bound. |
| New `TestDisjointMergeProof` in `metasystem/internal/gittree/disjoint_merge_test.go`; `go test ./internal/gittree -run '^TestDisjointMergeProof$' -count=1 -timeout=60s` | One compact table pins disjoint versus same-line changes, two insertions at one boundary, insertion on a deletion boundary, adjacent consuming hunks, unterminated final-line changes, repeated-line alignment under hostile diff configuration, and a both-sided non-text/mode refusal. Successful results equal explicit expected bytes, not the implementation's own patch output. | 60 seconds total, no polling or provider calls. |
| New `TestRecertifiedLandingParksOnOriginMove` in `metasystem/cmd/metasystem/landing_verbs_test.go`; `go test ./cmd/metasystem -run '^TestRecertifiedLandingParksOnOriginMove$' -count=1 -timeout=120s` | Drive the real landing script against owned local bare remotes, following the existing landing fixture's reduced commit-wrapper technique solely for driver ordering. Advance origin at the controlled push boundary. Observe a named park, unchanged retained local candidate, no rebase command, and no second push. This test proves transport handling, not a mocked certification verdict. | 120 seconds total, one induced race and one push attempt. |

The main canary uses a nested installation and an **area-width fixture chain**, so its candidate receipt runs only a tiny source test that checks both independently specified values. It does not claim to exercise full-width execution. Give the closed critic a limit of three rounds and three consumed rounds. In the successful and refusal runs, compare those fields, original review/patch bytes, critic return bytes, and goal budget before/after. They must be unchanged. Include a main-only file outside the project to verify nested grafting, and check an uncarried new candidate path still refuses under existing rules. Reuse the prepared canary state for a changed target and a changed proof payload; neither may return bar a. These are refusal checks on the same observation, not separate scenario families.

The required authoring-brief gate remains exactly:

```sh
scripts/agents/go-gate.sh --fast && scripts/agents/dispatch-fixtures.sh && scripts/agents/goal-cli-fixtures.sh
```

The orchestrator runs that chain from `metasystem/` outside the delegate sandbox when the work is settled, through the existing governed-run boundary with a 40-minute outer ceiling and each bed's normal scaled internal ceilings. A ceiling hit is an incomplete gate, never a passing log tail. This specified gate is not a replacement for the chain's existing full-width receipt requirement. The design author runs neither beds nor a full battery: this deliverable has no runtime change, and the delegated-run contract reserves those process-visible checks for the orchestrator.

## Implementation boundary and handoff

The build is confined to the conformance/recertification owner, the Git merge operation, landing binding and CLI plumbing, landing transport handling for the explicit proof reference, their named Go tests, and the existing command-help/refusal/evidence-retention entries needed to expose that path. Concrete primary targets are:

```text
metasystem/internal/validate/recertification.go
metasystem/internal/validate/conformance.go
metasystem/internal/validate/authorization.go
metasystem/internal/gittree/disjoint_merge.go
metasystem/internal/gittree/disjoint_merge_test.go
metasystem/internal/landing/observe.go
metasystem/internal/landing/promotion.go
metasystem/cmd/metasystem/validate_verbs.go
metasystem/cmd/metasystem/landing_verbs.go
metasystem/cmd/metasystem/landing_verbs_test.go
metasystem/cmd/metasystem/main.go
metasystem/scripts/agents/land.sh
metasystem/scripts/agents/commit.sh
```

No edits to `reviewRoundLimit`, `criticRoundsConsumed`, finding registration, goal budgets, approval semantics, or the mission wall are planned. The follow-up rebase policy remains unchanged; its existing hint/help may point coordinators with a closed chain to the named command. Do not add a fake implementer round to store the merge.

The critical obligations for the orchestrator's build review are original-review preservation, exact base recovery, deterministic no-overlap construction, exact merged-candidate binding, and refusal after target movement. Each has an owner and a smallest proof target above. None is claimed implemented or certified by this page.

There is no unresolved choice between review and proof in this revision. The historical-base limitation, conservative overlap boundary, and no-automatic-retry policy are deliberate decisions for the independent critique to examine. If that critique exposes a material gap, the orchestrator must report that a second design revision is needed before building the affected behavior; this authoring round does not write one or disposition its own critique.
