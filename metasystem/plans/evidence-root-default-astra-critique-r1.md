**ERD-01 — severity: material — Blanking `.local` does not make a launch use the default.**

- **Evidence (read):** `metasystem/internal/seat/launch/sequence.go:Sequencer.clone` copies the committed configuration; `Sequencer.configuration` copies the local override separately. U4 blanks only that override, while decision 2 makes blanks fall through to committed values. The existing `integration_test.go:fixtureRepository` already supplies this case: committed `evidence.root=/not/this/one`. The proposed resolver would select that path.
- **Smallest fix:** Before accepting a fresh clone’s configuration, reject an inherited concrete committed root with a clear remedy. Preserve existing destination settings on resume. Add a fixture asserting the resolved destination, not merely the blank local line.
- **Why step 1 is unsafe without it:** A first launch can write into the source seat’s root or another inherited location while claiming to have isolated the new machine.

**ERD-02 — severity: material — An unused checkout destination does not prove an unused evidence root.**

- **Evidence (read):** `metasystem/internal/seat/launch/preflight.go:destination` checks the full destination path, without constraining its basename. `Sequencer.evidenceRoot` currently rejects the source root, an existing directory this launch did not create, and a symlink redirect. U4 deletes those checks. For example, a fresh destination `/another-parent/ui` resolves to `$HOME/metasystem-evidence/ui`, which the design identifies as this seat’s existing explicit root.
- **Smallest fix:** Retain the fresh-launch isolation checks against the resolver’s result: compare canonical paths with the source root and reject an already occupied destination root or redirect. Preserve resume behavior and adapt the existing safety tests to default-derived paths.
- **Why step 1 is unsafe without it:** Supported launch inputs can immediately mix evidence with another checkout. The launch postcondition cannot be deferred on the premise that basename-derived paths are unique.

**ERD-03 — severity: material — The resolver cannot validate the configuration file explicitly requested.**

- **Evidence (read):** `metasystem/cmd/metasystem/config_verbs.go:runConfigValidate` accepts `--conf`; `configValidateTo` forwards that exact filename. `metasystem/internal/config/validate.go:validateWithRunner` currently judges its contents and its corresponding `.local`. U1’s `EvidenceRootParams` accepts only an installation directory, forcing resolution through `metasystem.conf` instead. Nonstandard filenames are an existing supported case, also exercised in `validate_test.go:TestContextBudgetConfigRefusesInvalidValues`.
- **Smallest fix:** Let the resolver accept the configuration filename, defaulting to the installation’s `metasystem.conf`. Validation must pass its actual `confPath`. Test two neighboring configuration files with different evidence settings.
- **Why step 1 does not work without it:** Validating `candidate.conf` can silently accept its unsafe evidence setting by judging the neighboring `metasystem.conf`, or reject a valid candidate because that neighboring file is absent.

**ERD-04 — severity: material — Legacy placeholders remain mandatory according to the structural audit.**

- **Evidence (read):** `metasystem/internal/audit/metasystem.go:AuditMetasystem` scans adopted repositories’ raw committed configuration with `auditPlaceholderRe`, which explicitly matches `<durable evidence root, outside the repository>`. It does not consult effective configuration. `metasystem/cmd/metasystem/audit.go:runAuditMetasystem` disables placeholder tolerance by default. Removing the template line in U1 does not remove it from existing adopted configurations, despite the migration section promising their compatibility.
- **Smallest fix:** Remove that obsolete evidence-placeholder alternative from the audit and add an adopted-repository fixture retaining the old line. Keep unrelated placeholder checks.
- **Why step 1 does not work without it:** A supported migrated installation resolves a valid default but still fails its ordinary audit because no explicit evidence root was supplied.

**Deferred**

- **ERD-05 — severity: non-material — The structural guard protects spelling, not ownership.** Evidence: U5 scans for the literal `evidence.root`, but `metasystem/internal/config/resolve.go:Get` accepts a key argument; a future reader can call it with `config.EvidenceRootKey` and retain a private fallback without triggering the guard. Smallest eventual fix: check unauthorized reads through the exported key as well. Step 1 works and is safe without that extension because U2–U4 explicitly migrate the current readers.

VERDICT: 4 material NOT YET EXECUTABLE

Codex session ID: 01a0e957-4359-7e52-b6ff-81c0b6ac943e
Resume in Codex: codex resume 01a0e957-4359-7e52-b6ff-81c0b6ac943e
