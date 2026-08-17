Verdict: D86 requires structural revision. Rejecting the HMAC design and pausing for an external trust-boundary decision were correct; the claimed impossibility, the two-option framing, and the description of D84 were not.

1. **STRUCTURAL — CRITICAL — D86’s premises are narrower than its impossibility conclusion.**

Premise (a) is true for shipped Devin behavior. The retained capability record declares reads and writes `notEnforced`, and the live probe demonstrated an out-of-root shell write ([capability record](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/artifacts/agents/capabilities/devin-3000.3.27-d1451b184dcf4823884a33a4-20260808-046.json:24), [devin-support.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/devin-support.md:107)). Because the baseline is ordinary JSON with an unkeyed SHA-256, replacing both files bypasses every CLI check ([goalverbs.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:74), [goalverbs.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:197)).

Premise (b) is also true for secrets inside this repository: both profiles grant `readRoots: ["."]`, and `"."` expands to the Git toplevel containing the control plane ([none.json](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/permissions/none.json:2), [workspace.json](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/permissions/workspace.json:2), [envelope.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/envelope.go:38)).

The inference is invalid as stated. These facts rule out:

- CLI-only authorization;
- caller classification derived from reproducible repository files;
- symmetric secrets stored within the delegate’s accessible privilege domain.

They do not rule out a private signing authority held by another operating-system principal, hardware key, user-presence keychain, or external service. Nor do they rule out public verification: a public key may be completely readable without enabling forgery.

An exact 3000.4.x Devin snapshot is absent, but the current registry and adapter still conservatively declare all three boundaries `notEnforced`.

2. **STRUCTURAL — CRITICAL — The two architectural directions are incomplete and mischaracterized.**

Direction A is insufficient as written. Enforcing the current `writeRoots: ["<worktree>"]` envelope still permits writes to `plans/goals-accepted.json` when that control plane is inside the writable worktree ([workspace.json](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/permissions/workspace.json:5)). Soundness requires control-plane write exclusion, an authority-owned store outside the worktree, or a mediated service—not merely “cannot write outside its worktree.”

Direction B does not require hiding the compiled public key or signed records. It requires:

- an inaccessible private signer;
- a pinned, integrity-protected verifier;
- signed durable genesis provenance, not merely a CLI grant;
- verification whenever the baseline is trusted, so direct replacement cannot bypass it.

The current target binary is copied into the same writable target and is not independently authenticated ([adopt.sh](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:268)). Protecting verifier integrity is therefore also unresolved.

Control-plane read-exclusion is needed only if the private key is stored there. An external signer makes asymmetric authorization independent of Direction A’s read-exclusion. A third legitimate human option is to retain the repository’s existing doctrine—same-user controls are cooperative defense-in-depth and the operator supplies the real isolation—and explicitly drop local “unforgeable genesis” as a product contract ([agent-orchestration-design.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agent-orchestration-design.md:677)).

3. **STRUCTURAL — HIGH — D84 is not accurately scoped as “HUMAN or MAIN, never machinery.”**

The pure authority matrix has that rule, but `goalCaller` does not reliably produce those identities.

A crafted source root can copy a live main announcement while omitting the nearer delegate signature. Authentication is only live process identifier, start time, and deterministic command hash; it is not root-bound ([classify.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/classify.go:129), [goal.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/goal.go:64)). D84 did not introduce that hole, but it failed to close it.

D84 did introduce an adapter-supervisor regression: every source classification other than `MAIN` is discarded. A correctly identified source `ADAPTER-SUPERVISOR` can therefore be reclassified against a virgin target as `HUMAN` under the documented ancestry condition and admitted ([goal.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/goal.go:65), [genesis-authority-review.md](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/genesis-authority-review.md:11)).

The rewritten plan’s “not a boundary against a rogue same-user delegate” disclaimer is directionally honest, but not sufficient: the durable D84 record and code comments still claim caller-controlled roots cannot forge upward, and the adapter-supervisor path does not require the crafted-root mechanism.

The `HasGoals` guard itself is useful: it correctly prevents sequential non-holder re-baselining of a populated ledger. Direct-file bypass was pre-existing, not created by D84.

4. **STRUCTURAL — HIGH — D84’s authorization-to-lock race remains open.**

Genesis mode is chosen by a pre-lock `os.Stat` ([goal.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/goal.go:53)). If a baseline appears before `Reconcile` acquires its lock, the `state.base == nil` genesis guard is skipped. The previously admitted non-holder can then reach restoration, malformed-baseline replacement, or ordinary replay without renewed holder authorization ([goalverbs.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:561), [goalverbs.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:601), [goalverbs.go](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:618)).

This defect was already recorded in the cited review but omitted from the parked rewrite. It is independent of Devin’s direct-write capability and should not be described as closed.

5. **STRUCTURAL — HIGH — Parking was defensible, but the parked conclusion was not.**

The agent was right not to ship the HMAC proposal: its key boundary was circular and readable. No complete sound solution was deployable solely from existing repository primitives—there is no external signer, keychain integration, separate delegate user, asymmetric verifier, or protected executable boundary.

Accordingly, pausing before inventing key custody or an external authority was reasonable despite D85. But the plan should have parked on the actual human decision: where the external trust/integrity boundary lives, or whether local genesis remains advisory. It should not have declared signatures impossible, treated A/B as exhaustive, or left the independently fixable D84 adapter-supervisor and lock-race defects under a “kept and closing accidental misuse” description.

Evidence level: checked by reading source, retained runtime records, tests, and history; no tests were run and no files were modified. Existing unrelated working-tree changes were preserved.

Proposed receipt: `type=review|outcome=revise|skills=take-a-step-back,design-critique|verify=source-read-only|corrections=0|stop_loss=no|delegate=native-read-only|note=D86 premises hold narrowly but do not prove asymmetric authorization impossible; directions A/B and the D84 keep omit control-plane write protection, adapter-supervisor laundering, and the pre-lock race`

REVISE — structural findings remain
