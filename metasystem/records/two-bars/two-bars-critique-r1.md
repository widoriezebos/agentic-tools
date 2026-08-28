The design is not enforceable as written. It would catch some accidental misclassification, but its core claims—semantic classification, proof bound to the committed tree, and durable auditability—do not hold. Seven material findings remain: six structural and one mechanical-grain.

1. **TB-R1-01 — HIGH — STRUCTURAL — The manifest cannot be both precise and complete as specified.**

   Concrete mixed file: [`internal/supervise/disk.go`](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/disk.go:101) declares the published `stateDocument` schema, but its contract also depends on the `SchemaVersion` and field population in `PublishState` at line 192. Marking only the struct lets a direct-fix commit change `SchemaVersion` or omit `EngineBuild` without touching the marker. Protecting the whole file forces an ordinary helper correction such as `now()` at line 38 through the loop.

   “Human rulings” are even less markable: they appear under inconsistent prose and headings across plans and owner documents. The instruction ledger maps rules to disparate owners; it does not provide a machine-readable ruling-to-code map.

   The manifest must be evaluated from both the base and candidate trees so a direct fix cannot delete its own marker or manifest entry. More importantly, the design must admit that this denylist is a conservative floor, not proof that every unlisted edit is mechanical. Otherwise either innocent edits nag or contract changes escape.

   Wrong outcome: commit `direct-fix: correct supervision state` changes only the schema producer at line 193 and passes an in-file marker around lines 101–127.

2. **TB-R1-02 — CRITICAL — STRUCTURAL — `commit.sh` sees neither the final message nor necessarily the final committed tree.**

   The wrapper forwards arbitrary arguments directly to Git ([`commit.sh`](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/commit.sh:38)). A precheck of the staged set is bypassed by `-a`, pathspec commits, `--include`, `--only`, and can be misbased by `--amend`. Message parsing limited to `-m` or `-F` misses `-C`, `-c`, `--fixup`, editor changes, and `--amend --no-edit`.

   Raw recognized-agent commits are normally stopped by the pre-commit token check, but that protection is deliberately fail-open when classification cannot positively identify an agent ([`pre-commit-guard.sh`](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/pre-commit-guard.sh:23)), is entirely bypassed by `--no-verify`, and is not installed when adoption finds an existing hook or a target that is not yet a repository ([`adopt.sh`](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:369)).

   `commit.sh` is the right orchestrator, but `-m` parsing is not a sufficient enforcement boundary. It must freeze and classify the actual candidate tree. A composed `commit-msg` hook should validate the final message; pre-commit should validate wrapper ancestry and a tree-bound classification token. Adoption must compose existing hooks or fail explicitly. If deliberate `--no-verify` resistance is required, local hooks cannot supply it; remote enforcement and durable agent identity are needed.

   Wrong outcome: `commit.sh -am "Change-Class: direct-fix …"` classifies an empty index, after which Git adds a protected contract file to the commit.

3. **TB-R1-03 — CRITICAL — STRUCTURAL — The proposed proof and gate checks are currently strings, not evidence.**

   `internal/gaterun` records only a live PID, start time, and gate name—not success or inputs ([`gaterun.go`](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/gaterun/gaterun.go:44)).

   The separate D33 witness covers the Go toolchain plus `cmd`, `internal`, `scripts/agents`, `go.mod`, and `go.sum` ([`go-gate.sh`](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/go-gate.sh:65)). It gates `git archive HEAD` only when those roots are clean ([`validate-metasystem.sh`](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:139)), so it does not describe a pending staged tree. It is emitted before the validator’s later fixture families, meaning the witness can exist even if the full suite subsequently fails. `commit.sh` neither locates nor reads it.

   A sound attestation needs:

   - immutable candidate tree OID and parent/base;
   - canonical gate identity, command/version, toolchain and final zero exit;
   - an authenticated, fresh, one-use run record;
   - for direct fixes, the same exact command against immutable baseline and candidate trees, with red and green outcomes and evidence hashes.

   Under a deliberate route-around threat, same-user JSON and file modes are forgeable. Either one supervising process freezes, gates, and commits the same tree without releasing custody, or a separate authority signs the attestation.

   Wrong outcome: an old HEAD has a valid D33 Go witness, the pending full suite is red, and a staged commit nevertheless passes with `Defect-Proof: artifacts/proof.txt`.

4. **TB-R1-04 — HIGH — STRUCTURAL — A per-commit scope budget controls review size, not design intent.**

   The proposed budget has no defect identity, cumulative base, or branch history. For example:

   - Commit A: `direct-fix: make Codex the adoption default`, changing [`runtimes=claude`](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:53).
   - Commit B: `direct-fix: align runtime fixtures and documentation`.

   Each can remain below any files/lines/subsystems threshold; together they make a user-visible policy change. Protecting all of `adopt.sh` merely transfers the problem back to manifest noise.

   Aggregate direct-fix scope against a declared defect identity and immutable pre-defect base, and audit repeated proofs/scopes. Even then, call the budget a growth fuse, not a semantic classifier.

5. **TB-R1-05 — HIGH — STRUCTURAL — “Declared and audited” has no durable audit join.**

   A `Design-Chain` path merely proving that a plan or critique file exists does not prove critique closure, dispositions, or that the implementation matches the reviewed design. Plans are task-local and may be deleted after shipping ([`plans/README.md`](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/README.md:3)); references therefore need immutable `commit:path` or blob identities.

   The instruction ledger records rule ownership, expected effect, and later verdict ([`instruction-ledger.md`](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/instruction-ledger.md:3)), while decisions documents remain the stated audit surface. No current validator joins either surface to commit trailers, proof artifacts, or critique closure. Commit `dc5edcd` also demonstrates why Git authorship is insufficient for retrospective classification: it has Wido’s author identity and a Claude co-author trailer. That is not a bypass, but it shows that a future missing classification cannot be distinguished from a sovereign human commit after the fact.

   The design needs a durable chain schema, closure check, tree binding, instruction-ledger entry for the new global rule, and a defined history audit. It must also state whether enforcement targets accidental omission or adversarial bypass; the latter cannot coexist with unverifiable human exemptions.

6. **TB-R1-06 — MEDIUM — STRUCTURAL — Human sovereignty does not cover an agent executing an approved emergency.**

   Existing policy lets only the human explicitly suspend gates and requires immediate reconciliation ([`collaboration.md`](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/collaboration.md:68)). A human personally committing is covered. An agent remains classified as MAIN or DELEGATE even after verbal approval, so an urgent safety-file repair is still refused.

   The design must choose explicitly:

   - emergencies are committed personally by the human; no agent override exists; or
   - the human mints a one-use authorization bound to the exact candidate tree, reason, expiry, skipped checks, and mandatory receipt/handoff reconciliation.

   A reusable environment variable or generic “emergency” trailer would reopen the escape hatch.

7. **TB-R1-07 — MEDIUM — MECHANICAL-GRAIN — The common path is not one extra line.**

   A direct fix requires two trailers, a persisted failing→passing proof, and a gate attestation. The draft’s own motivating cases—a stray build binary and a commit that left files unstaged ([`two-bars-design.md`](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/two-bars-design.md:21))—do not naturally have failing tests.

   Define structured non-test proofs such as before/after repository-state assertions, and let the wrapper append canonical trailers from CLI options. Otherwise ordinary cleanup will either take a ceremonial loop or acquire fabricated proof references—the exact skimming behavior the design is meant to prevent.

Verification: checked by reading the design and real commit, hook, adoption, gate, witness, ledger, and audit code; inspected the installed pre-commit hook and relevant Git history using read-only commands. No tests were run and no files were modified.

Proposed receipt, not written:

`1786921041|2026-08-16T22:57:21Z|RECEIPT|type=review|outcome=reworked|skills=design-critique|verify=skipped|corrections=0|stop_loss=no|delegate=commit_hook_trace,witness_audit_trace|critique_waived=none|waiver_stream=none|note=two-bars r1 critique: seven material findings; manifest precision, commit boundary, candidate-tree witness, cumulative scope, durable audit, agent emergency, and common-path friction require revision; read-only machinery trace, no tests`

REVISE — structural findings remain
