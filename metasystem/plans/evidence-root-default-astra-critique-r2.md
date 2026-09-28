**ERD-02 — reopened — severity: material — Missing roots can still identify the same directory through a symlinked parent.**

- **Evidence (read):** [U4](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/plans/evidence-root-default-design.md:241) compares nonexistent roots only after cleaning their paths. `internal/seat/launch/host.go:OSHost.Canonical` follows symlinks only when called. A nonexistent leaf can nevertheless have a symlinked ancestor.
- **Failure trace (inferred):** Let `<bed>/alias` link to `<bed>/real`, with `new` absent. The source’s local root is `<bed>/alias/new`; the inherited committed root selected by the clone is `<bed>/real/new`. The initial comparison finds different strings. Creating the clone root succeeds, and its canonical path equals its spelling. Both seats now resolve to the same directory.
- **Smallest fix:** Compare roots after resolving their existing ancestors, including when the leaf is absent; `internal/config/validate.go:resolvePath` already implements that algorithm. Preserve the clone’s redirect refusal. Add fixture `TestLaunchRefusesMissingSourceRootAlias`.
- **Why step 1 is unsafe without it:** A fresh launch can immediately assign another seat’s root to the clone. Restoring the checks closes the ordinary collision cases, but the fold’s missing-path exception leaves the original isolation finding open.

**ERD-06 — new — severity: material — The restored directory creation fails on a fresh default evidence tree.**

- **Evidence (read):** U4 retains `MakeDir(clone root)`. [OSHost.MakeDir](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/seat/launch/host.go:55) uses `os.Mkdir`, which requires the parent to exist. Decisions 3–4 permit an unused default and create nothing during resolution. The proposed integration fixture precreates `evidence/m1u`, so its sibling’s parent already exists.
- **Failure trace (inferred):** With valid configuration and no `$HOME/metasystem-evidence` directory, resolution succeeds but creating `$HOME/metasystem-evidence/<clone>` fails because its parent is missing.
- **Smallest fix:** Create missing parents recursively, then retain the exclusive leaf `Mkdir` and its created-versus-existing result. Add a filesystem fixture, `TestLaunchCreatesDefaultRootWithoutEvidenceParent`, asserting successful creation and continued refusal of an occupied leaf.
- **Why step 1 does not work without it:** The first launch using the advertised default can fail before any writer has created the evidence tree. This is introduced by restoring directory creation without its former existing-parent prerequisite.

The other round-1 folds are confirmed:

- **ERD-01 — closed.** U4 explicitly recognizes committed-value fall-through, judges and reports the resolved destination, and changes `integration_test.go:TestAClonedMachineIsAFleetSeatBeforeItIsEnrolled` to assert that destination. The remaining collision defect is retained under ERD-02.
- **ERD-03 — closed.** U1 passes the actual `confPath` into the resolver and derives its corresponding `.local`, matching `config_verbs.go:configValidateTo` and `config/validate.go:validateWithRunner`. Neighboring-file fixtures cover the original failure. No further amendment.
- **ERD-04 — closed.** U1 removes the obsolete alternative from `audit/metasystem.go:auditPlaceholderRe`, which `AuditMetasystem` applies to adopted configurations, and adds fixtures preserving unrelated placeholder refusals. No further amendment.

**Deferred**

- **ERD-05 — severity: non-material — The structural guard protects spelling, not ownership.** `config/resolve.go:Get` accepts an arbitrary key, so an unauthorized reader could use `config.EvidenceRootKey`. The smallest eventual fix remains checking those reads. Step 1 works and is safe without that extension because its current readers are explicitly migrated; the revision correctly retains this deferral.

VERDICT: 2 material NOT YET EXECUTABLE

Codex session ID: 01a0e961-e02b-7061-98f3-d4f7e05936d0
Resume in Codex: codex resume 01a0e961-e02b-7061-98f3-d4f7e05936d0
