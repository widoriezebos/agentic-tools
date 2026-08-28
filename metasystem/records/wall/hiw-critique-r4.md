Round 4 has not converged. The policy choices are made, but four structural gaps still permit unsafe or divergent implementations. I excluded editorial and bookkeeping issues under the materiality criterion.

### Material findings

1. **HIW-R4-01 — STRUCTURAL: the staleness predicate does not bind `B` to the E-sequence.**

   “Accepted changes between `B` and `E(j)`” is undefined unless `B == E(k)` for a unique prior sequence member. A natural implementation that scans turns since dispatch can exact-apply a hunk while preserving different, never-reviewed bytes elsewhere in the same file. Require `B == E(k)`—or equivalent equality of every changed tree entry—and compare the applied result’s object ID and Git mode with `R`. Add unknown-base, same-file drift, rename-endpoint, and multi-authorization fixtures. [Design](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:77>)

2. **HIW-R4-02 — STRUCTURAL: `RESTORE` and `ADOPT_DISPUTED_TREE` have no E-sequence transition.**

   Adoption permits later work but is not an accepted turn; restore may name an older safe tree. The design therefore does not define the next expected tree or whether changed paths from the resolution invalidate delayed authorizations. An implementer must invent whether resolution rebases, rewinds, or starts a new sequence segment. Define that transition, preserve prior consumption, count every adopted/restored path as an intervening change, and give it no delegation-floor credit. [Resolution contract](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:175>)

3. **HIW-R4-03 — STRUCTURAL: the machine-path refusal admits shippable machine bytes.**

   The rule permits machine paths that are “untracked or ignored,” but the shared snapshot uses `git add -A`, which includes untracked, non-ignored files. A tracked file can also match an ignore pattern. Moreover, current launch arms supervision—and can write artifacts—before contract preflight. The contract needs an exact machine-output inventory, no tracked entries, ignore coverage for every possible created descendant, and a read-only compatibility check before arming or any target write. Add untracked/non-ignored, tracked-and-ignored, tracked ancestor/symlink, and refusal-leaves-tree-unchanged fixtures. [Refusal](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:42>) [Snapshot implementation](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/validate/conformance.go:225>) [Current launch ordering](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/launch.go:270>)

4. **HIW-R4-04 — STRUCTURAL: the single-write model is sound in abstraction, but its authoritative recovery representation is unspecified.**

   The current state chain’s history entries contain only hashes, so consumption cannot literally be rebuilt “from the chain.” The design must choose the cumulative source—such as payload-bearing `turnLog` entries—and define duplicate/malformed-digest failure. It must also reconcile this acceptance write with the existing mission ledger, which is written before state and is documented as truth, and require the acceptance writer to preserve the durable two-outcome result rather than discard durability doubt. Without these choices, ledger-ahead, state-ahead, and cold-restart behavior differ by implementation. [Design](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:90>) [Current hash history](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/mission/state.go:457>) [Current conclusion order](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/loop.go:868>) [Ledger truth](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/glossary.md:138>)

5. **HIW-R4-05 — STRUCTURAL: host-artifact overlap can erase an authorization while preserving consumption credit.**

   The equation applies authorized patches and then the host-artifact delta, but does not explicitly require those path sets to be disjoint. A declared host artifact can overwrite every reviewed byte from an authorization; the digest is still consumed and can satisfy the delegation floor. Require pairwise disjointness or refuse; add complete-undo and partial-overlap fixtures. [Equation](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:33>) [Host-artifact rule](</Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:137>)

### Named contracts

Several implementation-changing details remain open:

- A content-addressed authorization cannot later mutate its own supersession status. Specify an immutable `supersedes[]` relationship and derived eligibility, including fork refusal.
- Pin the new mission-state version so valid version-1 state receives the exact legacy refusal before strict shape validation.
- Name the host-artifact contract key and exact protected-path table.
- Specify the acceptance, `openTurn`, taint/adoption, authorization, and `wall.json` field schemas and digest canonicalization.
- Enumerate emitters and required payload fields/types for all eight events.

These are mechanical-grain contract findings, but material because schemas and tests would otherwise differ.

### Obligation matrix

I ran the design-obligation gate. It parsed all 13 canonical rows and correctly reported every `MISSING` critical/high row as blocking completion.

The rows route every major body section nominally, but their proof targets do not cover all promised behavior. Missing explicit fixtures include:

- unknown authorization base and resolution-induced staleness;
- cold derived-index rebuild, duplicate consumption, and durability doubt;
- ledger-ahead/state-ahead recovery;
- untracked/non-ignored machine paths and preflight-before-write;
- authorization-versus-host-artifact overlap;
- crash after durable order recording but before integration;
- tree-ref cleanup at mission close;
- mutation on capped, nonzero, malformed-return, and recovery exits;
- a preserved version-1 state receiving the exact legacy error.

No files were modified. No runtime or product tests were run; only the matrix parser was executed.

Proposed receipt: `host-implementer-wall round 4 — revise; E-sequence membership and resolution transitions, machine-path exclusion, acceptance recovery authority, and host-artifact overlap remain structural; named schemas and matrix proofs remain mechanical-grain.`

REVISE — structural findings remain
