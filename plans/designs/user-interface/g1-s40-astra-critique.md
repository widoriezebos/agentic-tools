# Astra's critique of g1-s40 revision 1

Produced 2026-09-24 by Codex on `gpt-6-astra`, read-only, against revision 1 of [g1-s40-every-design-knows-its-goal.md](g1-s40-every-design-knows-its-goal.md) at `481d0e21b`, under R-121. Verbatim; the dispositions are the design's revision 2.

---

The draft names step 1 and a deferred list, so it passes “Step 1 Before Anything.” I assessed all seven numbered changes as step 1 because the draft places them there. Findings below are based on source inspection; failure scenarios are inferred, not runtime-tested.

1. **S40-01 — High: step 1 combines work that Wido explicitly ordered separately.**  
   The [draft:23–27](g1-s40-every-design-knows-its-goal.md:23) includes the historical home, migration command, migration execution and register closure in the same slice. Wido’s supplied ruling says to implement the forward rule **and then** reverse-engineer old metadata. [R-121:180](../../../metasystem/memory/rulings.md:180) requires the first useful result before iteration.

   **Failure:** a seat cannot receive the corrected authoring instructions until historical classification disputes are resolved, although its next design needs none of that migration machinery.

   **Change:** make the first deliverable “the next seat-authored design is a discoverable, goal-linked record.” Put historical discovery and typing in the subsequent pass. The migration findings below remain material to revision 1 as written; separating that pass removes them from the first slice’s blockers.

2. **S40-02 — High: the typing rule manufactures false scope and status.**  
   [Draft:26](g1-s40-every-design-knows-its-goal.md:26) equates conclusion with `done`, and everything else with `accepted`. But [memory-system:28](../../../plans/designs/memory-system.md:28) defines `done` as work shipped. The concluded [process-steward goal:7](../../../metasystem/records/goals/process-steward.md:7) explicitly says it was superseded by the counselor. The proposed pass would label its unbuilt design `done`.

   “Unmatched” also does not mean project-wide: [channel-landing-notice-design.md:3](../../../metasystem/plans/channel-landing-notice-design.md:3) explicitly names `channel-tells-me-when-something-lands`. And [metasystem-stop-design.md:3](../../../metasystem/plans/metasystem-stop-design.md:3) explicitly says `superseded`; the pass would instead publish it as project-wide and `accepted`.

   **Failure:** the pane reports shipped or accepted designs incorrectly and removes known goal associations. A misleading exact filename match produces the opposite error: association with an unrelated existing goal passes validation because [the checker:196](../../../metasystem/internal/project/answers.go:196) checks existence, not meaning.

   **Change:** treat filename and ledger state as proposed metadata requiring a reviewed mapping, with overrides and unresolved entries. Leave unresolved files untyped rather than asserting acceptance or project-wide scope. No semantic inference engine is needed.

3. **S40-03 — High: “headless” conflicts with the existing parser, and insertion is underspecified.**  
   None of the 102 files has `Kind`, but **22 already satisfy the parser’s head-detection rule**. For example, [brief-declares-the-round-boundary-design.md:3](../../../metasystem/plans/brief-declares-the-round-boundary-design.md:3) starts with `- Owner:`. [record.go:122](../../../metasystem/internal/project/record.go:122) treats that as a record head, then refuses missing required keys and malformed continuation lines. Meanwhile, [disk-hygiene-design.md:1](../../../metasystem/plans/disk-hygiene-design.md:1) starts with a status paragraph, not a title.

   **Failure:** implementing [draft:26](g1-s40-every-design-knows-its-goal.md:26) using `ParseRecord` skips or refuses 22 migration candidates. Simply inserting metadata into their existing bullet blocks creates duplicate statuses or invalid heads. The titleless file can remain invisible despite the pass reporting success.

   **Change:** distinguish an existing record declaration from legacy introductory bullets. Specify insertion and separation from old prose, including the titleless case. Preserve original content byte-for-byte outside the inserted title/head; never normalize or rewrite bodies.

4. **S40-04 — High: the prescribed home is wrong for adopted applications.**  
   [Draft:18](g1-s40-every-design-knows-its-goal.md:18) directs every seat to `plans/designs/` under the installation. [stateroot.go:108](../../../metasystem/internal/stateroot/stateroot.go:108) instead puts adopted application state at the containing repository root, and [project.go:172](../../../metasystem/internal/project/project.go:172) uses that state root for designs.

   **Failure:** a seat with an installation at `vendor/metasystem` follows the instructions and writes `vendor/metasystem/plans/designs/foo.md`. The resolver never sees it. The existing [adopted-layout fixture:53](../../../metasystem/internal/project/project_test.go:53) deliberately treats that path as a decoy.

   **Change:** prescribe the resolved **state-root** home. Preserve the self-hosted checkout-root home. Explicitly restrict the installation’s historical glob to the self-hosted history being migrated, so adopted applications do not acquire the kit’s history.

5. **S40-05 — High: `project check` does not enforce the promised design requirement.**  
   [Draft:18–20](g1-s40-every-design-knows-its-goal.md:18) promises that a design without a head is invalid and malformed designs cannot land. However, [record.go:117](../../../metasystem/internal/project/record.go:117) explicitly ignores documents without recognizable heads, while [record.go:18](../../../metasystem/internal/project/record.go:18) permits designs without `Goals`.

   **Failure:** a seat writes a title and prose in `plans/designs/foo.md`; `project check` remains green. A syntactically valid head that omits the claimed goal also passes. The manual critique rule catches the latter only when the critic knows the intended goal.

   **Change:** name the boundary that verifies the particular authored design exists as a record in a resolved home, and checks its expected goal. Keep generic headless documents lawful. Do not imply that a successful project-wide grammar check proves either requirement.

6. **S40-06 — High: adding the check to the fast script does not establish the landing guarantee.**  
   [Draft:20](g1-s40-every-design-knows-its-goal.md:20) omits the testing contract. [testing.json:50](../../../metasystem/testing.json:50) excludes design homes, the question register and other resolver inputs from `fast-static-build`’s declared inputs. [commit.sh:406](../../../metasystem/scripts/agents/commit.sh:406) consumes retained proof and disables static re-proof.

   **Failure:** after a green run, a design gains an invalid status or duplicate ID. Its change does not invalidate that group’s execution identity, so the earlier result can authorize delivery. Additionally, [go-gate.sh:134](../../../metasystem/scripts/agents/go-gate.sh:134) skips adopted installations.

   **Change:** specify the actual delivery boundary and bind its result to everything the resolver reads. Keep the check cheap—reuse the freshly built executable at [go-gate.sh:588](../../../metasystem/scripts/agents/go-gate.sh:588), or run a separately keyed read-only check, rather than requiring another compilation. State that a global check failure blocks every delivery consuming it, including unrelated code changes. Cost remains unmeasured.

7. **S40-07 — Medium: the migration has no defined safe retry or pre-write validation.**  
   [Draft:26](g1-s40-every-design-knows-its-goal.md:26) writes first, checks afterward, and “refuses to touch” existing heads without saying whether that skips a file or aborts the pass. IDs share a namespace across pages and question rows: [answers.go:174](../../../metasystem/internal/project/answers.go:174), [answers.go:264](../../../metasystem/internal/project/answers.go:264).

   **Failure:** a collision is discovered after files have changed, or an I/O failure leaves half the collection typed. A retry either stops at the first existing head or risks reminting identities.

   **Change:** validate proposed heads and their complete ID namespace before writing; leave valid existing records unchanged; report malformed existing declarations distinctly; define successful repeat application as a no-op. Require recoverable per-file writes and an explicit write set confined to selected historical files. Never move files or touch either goal-ledger directory.

8. **S40-08 — Medium: the confirmation hint needs an output-stream contract.**  
   [Draft:21](g1-s40-every-design-knows-its-goal.md:21) adds a line to confirmations, but [goalsync_mutations.go:725](../../../metasystem/cmd/metasystem/goalsync_mutations.go:725) emits JSON on stdout. Both open paths and the claim wrapper use it.

   **Failure:** appending prose to stdout makes a confirmed mutation’s output invalid JSON for callers.

   **Change:** put the human hint on stderr, only after confirmed success, including `open --claim`. Point to the canonical head instructions rather than maintaining another independent schema description.

**Outside step 1**

- Proven acceptance, automatic status synchronization, reverse `Design:` links, adoption seeding and history archival remain deferred. The existing fields and homes support them; they need not expand this slice.
- A complete human classification of unresolved history can remain deferred. Publishing guessed scope or status before that classification is the material problem.
- Calling the historical location a “second” home is merely naming; this checkout already has two. The proposed **flat** `plans/*-design.md` selector is appropriately narrow. Preserve it literally: recursively adding `plans/` would also read ledger files and reread `plans/designs/`.
- No pre-commit bypass is needed for modifying the 102 tracked files. The [guard:80](../../../metasystem/scripts/agents/pre-commit-guard.sh:80) forbids ledger edits, while its [new-plan acknowledgment:104](../../../metasystem/scripts/agents/pre-commit-guard.sh:104) concerns additions. Extending that acknowledgment to nested designs is separate work.
- The instruction routing is otherwise sound: [wow.md:14](../../../metasystem/wow.md:14) loads design principles during design work, so the proposed slicing-law pointer reaches the canonical gate before authoring. More always-on instructions are unnecessary.

Reviewed `ui-development` at `481d0e21b`; inventoried all 102 historical files and inspected relevant bodies and ledger records. No edits, tests or agents; `metasystem.conf.local` was not opened. Proposed receipt, not written: `review | g1-s40 revision 1 | design-critique | source inspection | 8 material findings`.

**Verdict: build after the eight listed changes.**

