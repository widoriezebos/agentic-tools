Verdict: the design is not sound. The HMAC scheme has no trusted key boundary, and a delegate can mint a self-signed grant without reading the legitimate key.

1. **GAD-R1-001 — STRUCTURAL — CRITICAL — Every shipped delegate permission preset can read the proposed key.**

   Both `none` and `workspace` grant `readRoots: ["."]` ([none.json:2](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/permissions/none.json:2), [workspace.json:2](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/permissions/workspace.json:2)). `"."` expands to `repo_scope`, the Git toplevel—not the job worktree ([envelope.go:38](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/envelope.go:38), [dispatch.sh:28](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:28), [dispatch.sh:480](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:480)). Every configured role uses one of those two presets ([metasystem.conf:45](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/metasystem.conf:45)).

   The proposed key, `metasystem/artifacts/agents/genesis.key`, therefore lies beneath every shipped delegate’s read root. Worktree delegates still receive the main repository as a read root. Codex declares `readRoots` unenforced, while Devin declares all three permission dimensions unenforced ([runtimes.go:152](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:152), [runtimes.go:168](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:168)). Mode `0600` does not isolate processes running as the same operating-system user.

   The payload claim does hold: adoption exports tracked content and then retains only its explicit top-level allowlist, which excludes `artifacts/`; `artifacts/` is also gitignored ([adopt.sh:139](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:139), [adopt.sh:166](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:166), [.gitignore:1](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/.gitignore:1)). The key does not ship, but it is readable in the source checkout.

2. **GAD-R1-002 — STRUCTURAL — CRITICAL — The target trusts an attacker-selected verification key.**

   The grant supplies its own source-key path, and the target reads that key to verify the HMAC ([genesis-authority-design.md:74](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/genesis-authority-design.md:74)). Covering the path with that same HMAC is circular: a delegate can create key `K` in a fake source root and submit a grant containing `path-to-K` and `HMAC(K, grant)`. The target has no independently trusted key identity with which to reject it.

   The mint command also takes caller-controlled `--root <SOURCE>` ([genesis-authority-design.md:45](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/genesis-authority-design.md:45)). Against an empty or crafted root, classification falls through to `HUMAN`; `RequireHolder` admits that as holder ([verbs.go:221](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/verbs.go:221), [verbs.go:227](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/verbs.go:227)). Moreover, `RequireHolder` returns success with `Holder:false` for an actual delegate rather than returning an error ([verbs.go:230](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/verbs.go:230)). The design’s statement that “a delegate fails here” is false unless new code explicitly checks the result—and even then the fake-root `HUMAN` path passes.

   This recreates the original classification hole at the mint boundary. The design needs a trust anchor independent of attacker-selected target data and a secret-bearing issuer actually isolated from delegates.

3. **GAD-R1-003 — STRUCTURAL — HIGH — Target-local nonce consumption does not establish unreplayability.**

   A delegate capable of creating the target can also delete or restore its target-local consumed-nonce ledger. It can then replay a copied grant after deleting the baseline, subject only to the freshness window. A target-local mutable tombstone prevents ordinary duplicate calls, not rollback by the principal being excluded.

   The baseline and nonce ledger are also separate files. Holding one flock serializes concurrent calls, but cannot make both writes crash-atomic. The design does not choose whether nonce or baseline lands first or define recovery from either half-written state.

   Target binding has the same missing identity contract: an absolute canonical string blocks static relative-path aliases, but not directory replacement or symlink changes between canonicalization, lock acquisition, verification, and writing. Current `Store.Root` is used directly to derive the lock and state paths ([goalverbs.go:88](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:88), [goalverbs.go:134](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:134)). The design must name the stable target identity and anti-rollback owner, not leave both to path helpers.

4. **GAD-R1-004 — STRUCTURAL — HIGH — The reconcile authority split is not specified tightly enough to prove F3 closed.**

   Currently every goal mutation, including reconcile, goes through `goalCaller` before `Store.Reconcile`; genesis selection uses the pre-lock baseline check ([goal.go:48](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/goal.go:48), [goal.go:94](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/goal.go:94)). `Store.Reconcile` still takes a classified `Caller` and uses it in non-genesis replay ([goalverbs.go:551](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:551), [goalverbs.go:618](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:618)).

   The design must require two explicit paths:

   - Grant supplied: do not call classification at all; under the store lock require baseline absence and a goal-free ledger, then consume. If a baseline exists or appears, refuse rather than entering restoration or replay.
   - No grant: classify and require holder authority before any restoration, malformed-baseline recovery, or replay.

   Without this explicit split, retaining the shared `goalMutation` spine would make classification failures veto a valid grant, while letting the grant path reach initialized-state branches would recreate F3.

   The proposed steps 1–5 are correctly placed inside the store lock on paper. Only implementation can prove every verification and state read actually remains inside `withLock`; “same locked section” still does not solve the two-file crash transaction above.

5. **GAD-R1-005 — STRUCTURAL — HIGH — Removing the benchmark bridge breaks delegated validation.**

   `validate-kit.sh` explicitly supports running in delegate sandboxes ([validate-kit.sh:22](/Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/validate-kit.sh:22)). It stages a source snapshot without `artifacts/`, then currently supplies the live authority root through `METASYSTEM_GENESIS_AUTHORITY_ROOT` ([validate-kit.sh:238](/Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/validate-kit.sh:238), [validate-kit.sh:248](/Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/validate-kit.sh:248), [validate-kit.sh:261](/Users/wido/LocalStorage/GitHub/agentic-tools/benchmark/validate-kit.sh:261)). `adopt-fixtures.sh` likewise strips source artifacts ([adopt-fixtures.sh:69](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt-fixtures.sh:69)).

   Dropping that export leaves a delegated validator trying to mint from a snapshot with no lease or key. A sound mint must refuse; an implementation that lets it pass has retained the `HUMAN` fallthrough. The design names no replacement issuer for this required end-to-end path.

   A terminal-human adoption is nominally automatic, but not fully specified: the grant says it carries a “minting main identity” even though `HUMAN` is also admitted, and failure cleanup/retry after a minted-but-unconsumed grant is absent.

6. **GAD-R1-006 — STRUCTURAL — HIGH — Key and time lifecycle contracts are missing.**

   The design does not define:

   - Concurrent first mint: two targets can observe no key and create different keys unless creation is source-serialized or exclusive.
   - Existing-key validation: required length, regular-file/no-symlink rule, ownership and mode failures.
   - Rotation: authorization, key identifiers, whether old unconsumed grants remain valid, and recovery for an adoption racing rotation.
   - Freshness: a serialized “monotonic mint time” is not ordinarily portable between processes; the clock, expiry representation, rollback behavior, and skew outcome need definition.
   - Grant cleanup and bounded retention after successful or failed adoption.

7. **GAD-R1-007 — MECHANICAL-GRAIN — MEDIUM — Required proof targets are not enumerated.**

   This authorization change triggers the repository’s full design-obligation matrix, but the plan supplies only broad package names. Critical focused tests must include key-read denial, fake-root/self-key minting, missing/corrupt/rotated key, concurrent first mint, same-grant concurrent consumption, crash between the two writes, target symlink replacement, cross-target reuse, replay after target-state rollback, valid terminal-human adoption, valid main adoption, delegated `validate-kit.sh`, and a grant-path test proving classification is never called.

Blast-radius search found no hidden production user of `genesis` or `--genesis-from`: the mode is confined to the authority package/tests and goal command; the flag’s production caller is `scripts/adopt.sh`; the environment bridge is confined to `benchmark/validate-kit.sh`. Tests and historical documentation still require cleanup. Ordinary initialized `goal reconcile` remains a supported caller and must retain holder classification.

Evidence: source and plans read; no tests run because the checkout is read-only and the grant implementation does not exist. No files were modified.

Proposed receipt: `type=review|outcome=revise|skills=design-critique|verify=source-read-only|corrections=0|stop_loss=no|delegate=none|note=Genesis grant r1 has no trusted key boundary: shipped delegates can read the HMAC key, attacker-selected keys self-verify, target-local replay state is rollbackable, and delegated benchmark adoption has no legitimate issuer`

REVISE — structural findings remain
GAD1-DONE
