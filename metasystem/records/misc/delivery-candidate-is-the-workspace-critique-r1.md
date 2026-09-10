# Critique register: delivery-candidate-is-the-workspace design, read 1

Job dcwl-crit1-20260910 (codex, gpt-5.6-sol, design-critic). Reviewed the page at 440081320 (sha256 0977891a22c591090d75d69f0476826c632d579114717e5f950b1a45d30413ac), read at 2fbd77535. Material findings: 6 of 7. The critic's runtime is read-only, so it returned the register in its job return and the coordinator projected it here verbatim; the JSON of record is artifacts/agents/dcwl-crit1-20260910/rounds/1/return.json.

## DCW-01 (high)

**Claim.** Sections 0, 2, 4, 7, 8, and 9 use workspace tree W as the universal cross-tip equivalence key and deliberately reject every non-ledger change. That fails the stated goal of never waiting for quiet: another seat's critique or design record under metasystem/records/** still invalidates the receipt even when every selected component identity remains equal. The fold should make selected component execution identities—including declared and retained implicit inputs, tools, environment, contract, and policy—the acceptance key; W should remain recorded as an auditable minimum projection. The proof loss is that an undeclared real read will not be noticed, but the existing component-reuse contract already accepts exactly that declaration boundary. The negative fixture should continue changing a declared input such as metasystem/scripts/agents/coverage-delta.sh, while a new records-only, non-input move must land. If W remains the key instead, DONE also requires a supported path for pushing the already-created rebased commit, because revision 1's re-receipt-and-land-again recovery cannot recommit an existing commit.

**Evidence.** Metasystem testing-contract design line 231 requires comparison of component input identities, not irrelevant whole-tree equality. Metasystem proofrun test_build.go lines 478-542 already revalidate declared inputs, retained implicit inputs, environment, and executable bytes. Design lines 344-360 knowingly add refusal for a change outside every declared input, and lines 362-366 admit land.sh has no way to push the existing rebased commit after that refusal. Metasystem scripts/agents/land.sh lines 525-567 always stage and commit before reaching its rebase-and-push path.

## DCW-02 (high)

**Claim.** Section 2.3 handles old readers seeing the new JSON field, but Section 4 does not handle the reverse command-line cutover. The first landing runs revision 1's land.sh with the older enrolled engine and an old schema-2 receipt without workspace; revision 1 nevertheless always adds --receipt after rebase, which that engine rejects even when the tip did not move. The fold must give legacy receipts a shell-side exact post-rebase tree check and call old test verify without the new flag; only receipts carrying the new workspace field may use test verify --receipt. A cutover fixture must exercise this older-engine/new-land.sh combination.

**Evidence.** Design lines 249-258 discuss only unknown JSON fields, while lines 334-345 always pass --receipt when a receipt was supplied. At 2fbd77535, metasystem/cmd/metasystem/test.go lines 139-155 define every test verify option and contain no receipt option. Metasystem/scripts/agents/land.sh lines 12-15 select the enrolled engine. Metasystem/scripts/agents/commit.sh lines 271-278 use that same engine when testing.contract is present; the candidate proof-engine build at line 313 is in the opposite branch and therefore does not read this receipt.

## DCW-03 (high)

**Claim.** Section 5 constraint 2 assigns retainedCandidateEngineDigest to W, but an engine digest belongs to the engine build projection, not all workspace bytes. A records-only landing outside the five W exclusions changes W without changing the engine, so the proposed lookup again loses all section-group evidence. The fold must key retained engine evidence by a complete ENGINE build identity, together with the applicable build environment, and test both an irrelevant-record success and an engine-input refusal.

**Evidence.** Design lines 410-418 require matching result.WorkspaceTree and receipt.workspace.tree. The neighboring patch at metasystem/artifacts/agents/dcwl-context/rbce-build1-r4.patch lines 314-360 currently looks up retained engine evidence by CandidateTree. Metasystem/internal/behaviorsurface/policy.v2.json lines 3-10 defines the narrower ENGINE projection. The design itself states at lines 397-409 that only that projection should determine the engine stamp.

## DCW-04 (high)

**Claim.** Section 5 constraint 1 assumes the current ENGINE projection is a complete deterministic build key, but it is not. metasystem/vendor/** is outside enginePaths, the neighboring patch clears GOFLAGS, and go-build.sh does not force module-readonly mode; Go can therefore compile different vendored dependency bytes from two candidates with equal ENGINE projections. Toolchain and platform inputs also remain outside the asserted byte-identity rule. Before an implementer keys reuse by ENGINE, the fold must either force -mod=readonly and bind the compiler/platform identity, or include every alternative dependency source in the projection. It must narrow the byte-identical claim to a fully identical build context and add a fixture that would fail if an unkeyed vendor or toolchain change alters the engine.

**Evidence.** Metasystem/internal/behaviorsurface/policy.v2.json lines 3-10 omits metasystem/vendor/**. The engine patch lines 95-108 clears GOFLAGS and pins GOTOOLCHAIN only to local; lines 153-158 call go-build.sh --trimpath. Its go-build change at patch lines 1352-1358 adds -trimpath but no -mod=readonly. The local go help build output states that a vendor directory is used automatically when go.mod declares Go 1.14 or later; metasystem/go.mod line 3 declares Go 1.27. The prior contract already requires GOFLAGS=-mod=readonly at metasystem/plans/application-testing-contract-design.md line 238.

## DCW-05 (high)

**Claim.** Section 8 does not provide proportionate end-to-end proof for a destructive landing-gate change and its promised two-minute leg ceiling is not enforced. The proposed shell scenarios bypass enrolled-engine resolution through the first-transition path, while this chain's own landing uses a legacy receipt without the new workspace field. Unit tests therefore cannot prove the production shell-to-enrolled-engine wiring, the schema cutover, or the ordinary new-receipt path. The fold must add an end-to-end enrolled-engine-equivalent receipt-and-land scenario, include the legacy cutover case from DCW-02, and name an actual per-leg watchdog. That may require adding metasystem/scripts/agents/fixture-budget.sh or metasystem/scripts/agents/fixture-bed-scenarios.sh to scope.

**Evidence.** Design lines 463-480 explicitly use the first-transition path and rely on unit tests plus this chain's landing for enrolled-engine coverage. Metasystem/scripts/agents/land-fixtures.sh lines 9-17 and 50-61 copy the source engine into each fixture. Metasystem/scripts/agents/fixture-bed-scenarios.sh lines 43-53 launch and wait for each child without a deadline. Metasystem/internal/proofrun/test_build.go lines 196-216 only records the sum of targetMs values as declared cost; it does not enforce a timeout.

## DCW-06 (medium)

**Claim.** Section 1.1's exclusion inventory is incomplete for the shipped goal command family. It omits metasystem/plans/goals.md and metasystem/plans/goals-accepted.json because they are absent at the reviewed revision, even though legacy goal verbs write them and goal migrate deletes them. Filtering absent names is already specified as a no-op, so omitting them makes a migration or legacy-ledger publish by another seat invalidate W contrary to the stated definition. The fold must include both legacy ledger paths in WorkspaceExclusions, documentation, and projection fixtures, or explicitly exclude legacy installations from the contract.

**Evidence.** Design lines 63-67 omit the two paths solely because they are absent at 2fbd77535. Metasystem/internal/goal/goal.go lines 118-153 routes unconverted repositories through the legacy Store, whose paths are defined at metasystem/internal/goal/goal.go lines 124-128. Metasystem/internal/goal/migrate.go lines 283-288 deletes both paths in the migration transaction. Design lines 147-152 already specify that an absent filtered path removes nothing.

## DCW-07 (low)

**Claim.** Section 1.1's statement that counselor register appends are part of the same ledger commit is factually wrong, but correcting it does not change the preferred implementation. Goal accept-risk and a tier-raising edit publish their goal transaction first and only then append the local counselor file. The counselor report is the other production reader, and no selected testing group declares the registers. The next revision should correct the rationale; this does not add a blocking implementation obligation once cross-tip acceptance uses component identities.

**Evidence.** Metasystem/cmd/metasystem/goalsync_mutations.go lines 341-350 publishes AcceptedRiskDecision before AppendAcceptedRisk; lines 1457-1461 likewise publish Edit before AppendMisclassification. Metasystem/internal/goal/txn.go lines 231-292 builds the published commit solely from explicit Change values. Metasystem/internal/counselor/sources.go lines 78-89 reads both registers for the advisory counselor report.

## Gaps the critic named

- The requested file metasystem/records/misc/delivery-candidate-is-the-workspace-critique-r1.md was not created because this review runtime has read-only filesystem permission and the critic role forbids repository edits.
- No fixture bed or candidate-engine build was run, as required by the brief. The engine determinism finding is supported by the reviewed build scripts, patch, projection policy, and the local Go tool's documented vendor-selection behavior rather than an empirical binary comparison.
- The brief's supplied 24-hour landing counts and settled twelve-site facts were accepted without re-derivation.

## Coordinator dispositions (m1d, 2026-09-10, binding on fold 1)

- DCW-01: accepted. The cross-tip acceptance key becomes the selected component execution identities; W stays recorded as the auditable floor. The residual refusal (a declared input moved) is a real re-proof, and the "push an existing rebased commit" gap becomes a one-line follow-up goal, not this goal's DONE.
- DCW-02: accepted. Site 9 first tries to need no new flag at all (today's `test verify` on the rebased tree already revalidates identities and composes); if a flag stays, it is gated by the receipt carrying the new field, and a cutover leg runs the older-engine plus new-land.sh pair.
- DCW-03: accepted as a constraint on the engine-bed chain: retained candidate-engine evidence is keyed by a complete engine build identity, never by T or W.
- DCW-04: accepted, narrowed by the tree: there is no `vendor/` at 2fbd77535 and go-gate.sh exports `GOFLAGS=-mod=readonly` (line 66) while go-build.sh does not. The constraint reads: `-mod=readonly` forced, toolchain and platform bound into the engine identity, byte-identity claimed only for an identical build context, one fixture that fails when an unkeyed input alters the engine.
- DCW-05: accepted in part: one bounded end-to-end leg on the production shell-to-engine path with a receipt carrying the new field, the cutover leg from DCW-02, and a named per-leg watchdog or the two-minute promise withdrawn. The first-transition legs stay for what they prove.
- DCW-06: accepted; `plans/goals.md` and `plans/goals-accepted.json` join the exclusions (absent paths filter as a no-op).
- DCW-07: accepted; the rationale is corrected, nothing else moves.
