One structural finding remains. Under the stated stop rule, this should pivot directly to D81 implementation-first, not another prose round.

### Structural finding

**HIW-R5-01 — Expected-tree membership lacks occurrence identity.**

The design binds an authorization base to a tree object ID and requires it to equal a “unique prior” `E(k)` ([design](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:89)). Valid histories can repeat that ID:

- An accepted no-change turn produces `E(i+1) == E(i)`.
- `RESTORE` can deliberately return to an older recorded tree.
- A resolution inserts an expected-tree transition without a corresponding host-turn ordinal ([resolution](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:214)).

Consequently, a fresh authorization issued after restoration may match several sequence members. The implementation must either reject legitimate post-resolution work indefinitely or invent which occurrence and segment the authorization belongs to. That choice changes the intervening-change set and acceptance outcome.

D81 needs an explicit sequence-point identity—such as state-chain sequence plus segment/ordinal—bound into each authorization alongside the base tree ID. Add positive fixtures for authorization after a no-change turn and after restoring a previously seen tree. The existing resolution-staleness fixture only proves overlap refusal; it does not prove that fresh post-resolution work remains consumable.

### Round-4 fold verification

| Round-4 requirement | Assessment |
| --- | --- |
| E-sequence membership and object-ID/mode equality | Present, but exposes HIW-R5-01 when composed with repeated trees and resolution segments. |
| Resolution starts a segment, preserves consumption, and counts its diff as intervening | Present. |
| Machine paths ignored—not merely untracked—with read-only preflight before arming/writes | Present and unambiguous. |
| Payload-bearing acceptance recovery, duplicate/malformed refusal, ledger-ahead unaccepted, durability-doubt reread | Present. |
| Host-artifact/authorization pairwise disjointness | Present and composes correctly with prior-turn staleness. |

Ignored machine transport and shippable host artifacts do not conflict: tracked-and-ignored machine output refuses, while authorized host artifacts remain inside the tree equation. Same-turn artifact/authorization overlap refuses; earlier artifact and resolution changes participate in staleness.

### Mechanical-grain contracts still open

These should be settled by D81 schemas and golden fixtures:

- The document says the field families “get a JSON schema” but does not actually specify their exact keys, types, discriminators, or conditional requiredness ([named contracts](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:323)).
- The wiredoc encoder fixes canonical bytes, but the authorization digest domain remains unspecified—particularly whether an embedded `authorizationDigest` is omitted, blanked, or exists only as the filename.
- The eight recorder events name fields but not their registry types or whether identifiers belong under `ids` versus payload. `mission` also differs from the existing `missionId` convention ([events](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/host-implementer-wall-design.md:329)).

The supersession graph, version-2 pin, `hostArtifacts` key, and protected-path table otherwise state deterministic outcomes.

Evidence: checked by reading the design, round-4 critique, relevant state/wiredoc/event code, and repository contracts. The obligation-gate script recognized all 13 rows and exited only because every implementation proof is correctly still `MISSING`. The prior round’s named fixtures are carried. No runtime tests ran and no files were modified.

Proposed receipt: `host-implementer-wall round 5 — revise; all five r4 folds are present, but expected-tree occurrence identity remains structural; concrete schemas and event typing remain mechanical-grain; pivot to D81.`

REVISE — structural findings remain
