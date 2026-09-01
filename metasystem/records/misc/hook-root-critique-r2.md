# Hook-root design critique — round 2 (Sol)

Chain: revision 2 (installation-derivation) -> critic
design-critic-b05f299c4dc27c3b49766e7a (codex gpt-5.6-sol, xhigh,
fresh context), 2026-09-02. Five material findings — refinements of
the new derivation mechanism, not re-opens: invocation pathname is not
authoritative evidence; the worktree decision is not operational in
the real delegate layout; a mismatched engine silently disables a
governed Stop hook (the exact rebuild-drift situation this fleet
lives daily); the common-dir failure fallback inverts its proof
burden; evidence garbage collection is missing from the one-world
consumer claim. Revision 3 folds each by id.

## SHR-R2-INSTALL-01 — high, material=True

CLAIM: Round-one finding SHR-ROOT-01, the authoritative-root requirement, is only partially resolved because the invocation pathname is not authoritative evidence of an installation. A whole-installation directory symbolic link is physically normalized, but a final-component symbolic link to supervision-hook.sh is not: metasystem/scripts/agents/supervision-hook.sh:23 takes dirname before resolving that file. A copied or relocated hook under any Git checkout containing scripts/agents also passes the proposed verb's installation-shape check at metasystem/plans/supervision-hook-root-design.md:62-73, even when the copy is not the governing installation. The shipped hook checker does not establish stronger provenance. An implementer following the design would therefore accept some copied or linked invocation locations as worlds and can recreate wrong-root writes. The contract must distinguish supported directory links, terminal file links, and copied or relocated hooks, then authenticate or reject each case.

EVIDENCE: Read metasystem/plans/supervision-hook-root-design.md:58-73 and 89-101, metasystem/scripts/agents/supervision-hook.sh:23-25, metasystem/internal/stateroot/stateroot.go:137-153, and metasystem/internal/hooks/hooks.go:51-83. The Go executable owner calls symbolic-link evaluation on the complete executable path, whereas the shell canonicalizes only the invocation path's directory. The proposed engine verb validates only a directory and scripts/agents, so it cannot distinguish a complete installation from an in-repository hook copy.

## SHR-R2-WORKTREE-ENGINE-01 — high, material=True

CLAIM: Round-one finding SHR-WORKTREE-01, the linked-worktree reporting decision, is not operational in the real delegate layout because the hook requires a sandbox-local engine before it computes the primary mapping. Resolution remains ordered after the executable check at metasystem/plans/supervision-hook-root-design.md:199-203, so a worktree with the tracked hook but no ignored metasystem/bin/metasystem emits the missing-engine degradation and exits without reading the primary ledger or job records. The proposed worktree fixture explicitly stages an engine inside the worktree at metasystem/plans/supervision-hook-root-design.md:289-293, masking the production condition the design itself says it observed. An implementer would build a primary-world mapper that cannot be reached in the named real layout unless an unmentioned environment override happens to supply an engine.

EVIDENCE: Ran inspection of the real linked worktree at metasystem/artifacts/agents/worktrees/implementer-4558bc46633bc3857152b218: its tracked hook and plans exist, its corresponding primary job record exists, and metasystem/bin/metasystem and metasystem/artifacts do not. Read metasystem/scripts/agents/supervision-hook.sh:21-31, which checks the worktree-derived engine before any payload or root work, and the proposed ordering and fixture at metasystem/plans/supervision-hook-root-design.md:199-203 and 289-308.

## SHR-R2-ENGINE-SKEW-01 — high, material=True

CLAIM: A present but mismatched engine silently disables a governed Stop hook. The new hook calls path state-root and maps every engine error to exit 0 with no output at metasystem/plans/supervision-hook-root-design.md:176 and 225-245. An older executable can still pass runtime-list membership but lack that new verb, so it reaches this silent exit instead of the existing visible missing-engine message. Engine binaries are untracked artifacts, making source-versus-binary skew a normal upgrade or post-landing state rather than a malformed installation. The design has no capability or build-stamp handshake and its fixtures always pair the revised hook with a current engine. An implementer would therefore preserve a failure mode indistinguishable from a hook that never fired, contradicting the design's reason for rejecting worktree-hook suppression.

EVIDENCE: Ran metasystem/bin/metasystem path state-root --installation metasystem against the currently installed executable; it returned exit status 2 because the proposed verb is absent. Read metasystem/scripts/agents/supervision-hook.sh:26-30 and metasystem/scripts/agents/supervision-hook-fixtures.sh:116-134, where a missing engine produces a visible HEALTH unknown message, and metasystem/plans/supervision-hook-root-design.md:176 and 225-245, where an incompatible verb is suppressed into silent success. Git reports metasystem/bin/metasystem as ignored and untracked.

## SHR-R2-WORKTREE-FALLBACK-01 — high, material=True

CLAIM: The linked-worktree decision is not obtained by construction because failure to identify Git's common directory is interpreted as proof that the installation is not linked. The normative block sets an empty result on any rev-parse failure and then calls RootForInstallation on the worktree installation. For a template worktree, that authority deterministically returns the sandbox-local world—the outcome the design rejected as structurally false. The design explicitly acknowledges this behavior for Git versions below its stated 2.31 minimum, and the same branch covers any other query failure. This also contradicts its rule that unresolved inputs exit silently rather than guess. The implementation must not turn an unknown worktree identity into the non-worktree outcome.

EVIDENCE: Read metasystem/plans/supervision-hook-root-design.md:129-147, 154-178, 225-245, and 400-403. The proposed git_ids=$(...) || git_ids= branch bypasses worktree mapping but continues to path state-root. Read metasystem/internal/stateroot/stateroot.go:100-108 and 157-163: a template-mode installation returns itself, so the fallback selects the worktree-local sandbox rather than the primary world.

## SHR-R2-CONSUMER-01 — medium, material=True

CLAIM: The one-world consumer claim still misses evidence garbage collection. In a mapped linked worktree, repo and world_installation point to the primary checkout, but the hook continues to run evidence-gc.sh from the worktree script directory. That script independently derives its root from its own location and sends the worktree root to lease require-holder, lease run-held, and evidence gc. It will either fail against the empty sandbox or operate on sandbox state while the rest of the turn reads and writes the primary. The up-specific correction requested by round-one finding SHR-CONSUMER-01 is sound, but an implementer following the new consumer table would leave this newly introduced split intact.

EVIDENCE: Read metasystem/scripts/agents/supervision-hook.sh:260-265 and metasystem/scripts/agents/evidence-gc.sh:16-48. Evidence garbage collection derives metasystem_gc_root from its own script and never accepts the hook's resolved repo. The table at metasystem/plans/supervision-hook-root-design.md:329-347 claims the worktree turn uses the primary world but lists only the hooks.log redirection at the evidence-trail line, leaving the invoked collector unchanged.

## Critic-declared gaps (verbatim)

- The task describes this as design-critique round two, but the generated runtime notice and current job record identify it as round 1 of a new critic job; the prior critique came from a different job. The required chain-level three-round budget therefore cannot be verified from this dispatch. This return uses the harness-observed round number 1.
- Terminal symbolic-link and copied-hook scenarios were not executed because this review has read-only filesystem authority. Their behavior is inferred from the normative Bash and Go path-resolution code; no finding depends on an assumed live fleet layout.
- The live m2 and m3 machines were not accessible, so their engine and worktree layouts were not independently rechecked. The reported findings are demonstrated by local code and the inspected m0b delegate worktree.
- During the review, metasystem/records/narrator-digest.log acquired an unrelated appended line from another process. The cause was not established, and this critique neither reverted nor edited it. The reviewed design and hook remained unchanged.
