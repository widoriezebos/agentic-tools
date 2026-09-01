# Hook-root design critique — round 3 (Sol)

Chain: revision 3 -> critic design-critic-c5d5e38862fc161961d7228d
(codex gpt-5.6-sol, xhigh, fresh context), 2026-09-02. Converging:
two material findings — the engine identity answer must validate the
same installation the shell consumers keep using under a
METASYSTEM_BIN override, and the linked-worktree mapper must not
trust inherited git steering environment variables. Revision 4 folds
both; closure expected.

## SHR-R3-ENGINE-INSTALLATION-PAIR-01 — high, material=True

CLAIM: The no-argument engine answer does not validate the installation that shell-owned consumers continue using, so the explicitly supported METASYSTEM_BIN override can split one hook turn across two worlds and regress an existing killed-attempt fixture.

EVIDENCE: Revision 3 says the world follows the override engine at metasystem/plans/supervision-hook-root-design.md:116-122, but still passes world_installation to up and invokes the collector from world_installation at metasystem/plans/supervision-hook-root-design.md:290-293 and metasystem/plans/supervision-hook-root-design.md:521-536. The shipped up command derives its state root from that explicit installation at metasystem/cmd/metasystem/up.go:104-144, while the collector derives root from its own script at metasystem/scripts/agents/evidence-gc.sh:16-48. The existing killed-attempt fixture at metasystem/scripts/agents/supervision-hook-fixtures.sh:191-221 runs a copied hook under a temporary wrapper supplied through METASYSTEM_BIN that forwards to the source engine. Under the prescribed design, path state-root therefore returns the source checkout while up and evidence collection retain the copied fixture installation. An implementer would build wrong-root local writes and violate the fixture's isolation unless the engine and installation are coupled or a mismatch is rejected.

## SHR-R3-GIT-STEERING-01 — high, material=True

CLAIM: The linked-worktree mapper trusts inherited Git steering variables, so a supported engine-less delegate worktree can be falsely classified as an ordinary checkout and lose its Stop hook before reaching the primary engine.

EVIDENCE: The normative commands at metasystem/plans/supervision-hook-root-design.md:237-255 invoke Git without clearing GIT_DIR, GIT_WORK_TREE, GIT_COMMON_DIR, or related variables. Running the exact common-directory query in metasystem/artifacts/agents/worktrees/implementer-4558bc46633bc3857152b218 normally produced distinct linked-worktree and common directories; setting GIT_DIR to the primary Git metadata directory and GIT_WORK_TREE to that worktree made both results equal. The proposed equality branch would keep world_installation at the sandbox, whose engine is absent, and exit after the missing-engine report. The compiled authority explicitly removes Git steering at metasystem/internal/stateroot/stateroot.go:32-49, so the new shell authority is weaker than the authority it claims to preserve. An implementer must change the mapper or will build a governed layout whose outcome depends on unrelated inherited process state.

## Critic-declared gaps (verbatim)

- The generated runtime and job record identify this fresh job as round 1, while metasystem/plans/hook-root-critique3-brief.md calls it round 3. This runtime cannot prove the required same-chain three-round budget or authorize design-phase closure.
- The launcher classified context isolation and independent examination as advisory rather than proven, and did not prove its complete provider-tool catalog.
- The m2 and m3 machines were not accessible. Neither material finding depends on those machines; both are established by committed local code and a local linked-worktree probe.
- metasystem/records/narrator-digest.log was modified by another process. This critique did not edit or revert it.
