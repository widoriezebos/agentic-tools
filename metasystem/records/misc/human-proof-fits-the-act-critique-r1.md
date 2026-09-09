# human-proof-fits-the-act, design critique round one (register)

Critic human-proof-crit1-20260909 (design-critic, codex gpt-5.6-sol, xhigh, read-only) on revision 1 of plans/human-proof-fits-the-act-design.md (commit b7fb35a2d, sha256 18c60e5d80457e5aed4138287e08fedd06b0d2cebcf78535f8d885dc959a278c), authored on the Fable lane the same hour. Nine material findings, six high; the critic's runtime was read-only so this register is built by the orchestrator from the critic's returned JSON, verbatim in claim and evidence.

## HPA-01 (high, material)

Claim: Section 2, “The two proofs,” and section 8, “Scope”: the design correctly identifies the exported-lineage bypass but does not close every human entry point. Goal migrate and goal repair bypass syncReq entirely, and RecordFleetEnrollment has no Actor.Human guard for the proposed replacement to find. An implementer following the named files can leave these routes accepting a human name without Authority. Amend metasystem/plans/human-proof-fits-the-act-design.md with a positive inventory of every human-only entry point, explicit proof plumbing for migrate, repair, and fleet enrollment, and negative tests showing Actor.Human alone is never sufficient.

Evidence: metasystem/cmd/metasystem/goalsync_mutations.go proves only when lineage is absent. metasystem/cmd/metasystem/goalsync_verbs.go invokes migration and repair through separate paths; metasystem/internal/goal/accepted.go checks only that the human name is nonempty; and metasystem/internal/goal/approval.go records fleet enrollment without the proposed Authority field. These files are missing from the implementation scope.

## HPA-02 (high, material)

Claim: Section 1, “Proof grade table”: set-pin cannot use terminal grade. Clearing the pin permits every machine to claim, and moving it permits the destination machine, so both operations widen authority under the goal’s settled rule. Amend metasystem/plans/human-proof-fits-the-act-design.md to require enrolled proof for every set-pin mutation and add fixtures proving terminal proof cannot clear or move a pin. Set-priority may remain terminal because it only orders already eligible work.

Evidence: metasystem/internal/goal/verbs.go states that only the pinned machine may claim and implements “-” by clearing the pin. The design’s assertion that set-pin permits nothing new is therefore false.

## HPA-03 (high, material)

Claim: Section 1, “Proof grade table,” is not verb-by-verb and its terminal catch-all misgrades state-dependent transitions. Human unpark can restore Approved state, set-arc can create a claim, and discharge-review-obligation removes a completion barrier; split and risk-lowering open or edit paths already carry full proof but have no explicit grade. Amend metasystem/plans/human-proof-fits-the-act-design.md with a complete transition-and-precondition matrix instead of “all other --by rows,” preserving enrolled proof wherever the transition grants eligibility, creates a claim, or weakens a guard.

Evidence: metasystem/internal/goal/verbs.go restores the saved resting state during unpark and can assign a claim during set-arc. Repository searches also find conditional human overrides and existing proof-bearing split, OpenRisked, and edit paths that the table and proof-parameter migration list do not fully enumerate.

## HPA-04 (high, material)

Claim: Sections 1 and 8 assign or imply terminal-only classification for several acts that reopen execution rather than stop, park, or release it. Metasystem arm, steward arm or restart, brain withdraw, and the omitted mission resolve-taint adopt-disputed-tree path remove fences or restrictions and can resume work. Amend metasystem/plans/human-proof-fits-the-act-design.md to require enrolled proof for these widening transitions, retain terminal grade for the corresponding stopping or narrowing transitions, and include the mission-runner files and fixtures in scope.

Evidence: metasystem/internal/missionrunner/resolve.go says adopt-disputed-tree begins a new sequence segment after resolving taint. The command implementations show arm and restart reopening stopped process surfaces, while brain withdraw removes a declaration. None is merely a stop, park, or release.

## HPA-05 (high, material)

Claim: Sections 1 and 2 contain an unresolved migration bootstrap contradiction. Migration can import approvals and budgets, so terminal grade violates the settled rule, but current enroll-terminal refuses before a synced backlog exists, making enrolled proof unavailable before migration. Amend metasystem/plans/human-proof-fits-the-act-design.md to permit local enrollment before migration and defer fleet-cutoff publication until the backlog exists, then require enrolled proof for migrate. If that lifecycle is rejected, the bootstrap exception needs an explicit human ruling rather than an implementer’s guess.

Evidence: The design assigns terminal grade to migrate because it is a bootstrap path. metasystem/cmd/metasystem/goalsync_mutations.go currently rejects enroll-terminal before synchronization, while metasystem/cmd/metasystem/goalsync_verbs.go performs migration without an Authority proof.

## HPA-06 (high, material)

Claim: Section 6, “The relayed word,” does not make temporary words unambiguously recorded-but-never-binding. It misses the steward-arm caller of ArmTemporary, top-level help and recovery text that advertise the removed flags, and steward re-arm logic that copies a previous temporary word and enrollment into a newly minted identity. Amend metasystem/plans/human-proof-fits-the-act-design.md to name every deletion site and decide that re-arm may preserve historical fields only as inert provenance, never mint a fresh temporary authority generation from them.

Evidence: The repository-wide production search finds ArmTemporary in metasystem/cmd/metasystem/steward_verbs.go, removed flag documentation in metasystem/cmd/metasystem/main.go and metasystem/internal/up/up.go, and temporary-word carry-forward in metasystem/internal/steward/runner.go. These sites are absent from section 6 and section 8.

## HPA-07 (medium, material)

Claim: Sections 2 and 3 cannot render the proposed no-enrollment refusal while keeping full Prove unchanged. With no enrollment, Prove returns an ordinary error and no TERMINAL_NOT_REACHED outcome, so the renderer has no specified code on which to select that row. Amend metasystem/plans/human-proof-fits-the-act-design.md to define one structured no-enrollment outcome shared by Prove and ProveTerminal, and test its exact mapping to the enroll-terminal recovery command.

Evidence: metasystem/internal/humanauthority/authority.go returns a zero proof plus “human authority has no readable terminal enrollment” when enrollment is absent. metasystem/internal/refusal/register.go defines TERMINAL_NOT_REACHED, but the existing Prove path does not return it.

## HPA-08 (medium, material)

Claim: Sections 2 and 3 do not fully satisfy the one-enrollment recovery alternative. The active-other-terminal refusal says to enroll locally but does not state that no prior enrollment or authority is required, that this silently retires the first terminal, or that the refused command must then be retried. Separately, when the forgiving-human-verbs dependency lands second, APPROVAL_REQUIRED can print literal “<your name>,” which is not a command that succeeds. Amend metasystem/plans/human-proof-fits-the-act-design.md to make the recovery statement explicit and either order this implementation after the name provider or report that no complete command is available when no human name can be derived.

Evidence: The design’s enrollment algorithm atomically replaces the previous record and needs no prior proof, so the one-terminal architecture itself is defensible. Its active-enrollment refusal omits those facts. metasystem/cmd/metasystem/goal_refusal.go is absent, and the design explicitly permits a literal placeholder until the dependent goal lands.

## HPA-09 (medium, material)

Claim: Section 7’s four canaries are not all red on the untouched tree for their stated reason. Canary one already refuses both agent approval and agent session stop; canary four’s core operation—fresh Enroll replacing an old enrollment without prior proof—already succeeds. Terminal liveness also lies outside the described tree-reader fixture, so alive-versus-dead refusal tests are not deterministic as written. Amend metasystem/plans/human-proof-fits-the-act-design.md to make the exported-lineage bypass the primary red security canary, retain current agent refusals as regression guards, make recovery execute the printed dead-terminal command and verify retirement of the old terminal, and inject a liveness probe.

Evidence: Current approval and session-stop paths already require full proof or terminal classification, while metasystem/internal/humanauthority/authority.go already permits a fresh Enroll to replace the record. metasystem/internal/humanauthority/authority_test.go models ancestry through treeReader, but process liveness is a separate kernel-reader behavior.

## Gaps the critic named

- No bed, live human verb, or mutation test was run, as required by the brief; canary pre-change behavior is established by code reading rather than execution.
- The critique register file was not created because the binding design-critic role forbids edits and the workspace is read-only; this JSON is the returned register.
- The runtime exposed no independent session identifier, so sessionId is recorded as “unobserved” and no differing session or model is claimed.
