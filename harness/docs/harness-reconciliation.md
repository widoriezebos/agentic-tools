# Reconciling an Existing Repository

The manual for an agent asked to install or upgrade this harness in a repository that already has instruction assets: agent contracts, skills, prompts, rule files, guidelines, or quality scripts. The goal is one instruction system, the harness, with every surviving project rule in its one canonical home and everything superseded removed in the same change.

Fresh repositories with no existing assets skip this manual and follow `docs/project-adaptation.md` directly. This manual wraps those steps with inventory, classification, cutover, and proof.

## Ground rules

- This work is repo-wide and consequential. Run it on a dedicated branch, and treat every deletion of an existing guideline as a decision the human reviews. The reconciliation ledger below is that review guide.
- Read every asset completely before deciding its fate. Never delete, replace, or merge unread content.
- The harness wins on duplication; the project wins on facts. When an existing rule genuinely conflicts with a harness rule, do not silently pick a side. Record the conflict in the ledger and escalate.
- Follow the reviewable-increment rules in `docs/collaboration.md`: mechanical copies and semantic merges land as separate commits.

## Phase 0: contract and inventory (read-only)

1. Fix the scope with the human: target repository, harness template version (record the commit SHA), runtimes in use, and explicit authorization to remove superseded assets.
2. Inventory every instruction asset. Search at minimum:
   - Runtime contracts: `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.cursorrules`, `.cursor/rules/`, `.github/copilot-instructions.md`, `.windsurfrules`.
   - Skills and workflows: `skills/`, `.claude/skills/`, `.claude/agents/`, `.claude/commands/`, `.devin/agents/`, prompt or template directories.
   - Guidance: agent-directed sections in `README`, `CONTRIBUTING`, and `docs/`; style, review, or ways-of-working documents written for agents.
   - Enforcement: hooks, CI jobs, and scripts that encode agent behavior rules.
3. Open the reconciliation ledger at `plans/harness-reconciliation.md` and give every inventoried asset a row:

```markdown
| Asset | Kind | Disposition | Destination owner | Conflicts/notes | Status |
| --- | --- | --- | --- | --- | --- |
```

The ledger carries this work across sessions; do not rely on memory. It doubles as the stream's handoff note.

## Phase 1: classify

Decide the fate of every rule with the table below. A file is not the unit of decision; a rule is. Split files that contain several kinds.

| Content found | Disposition |
| --- | --- |
| Project commands, paths, invariants, ownership facts | Move to `docs/project-rules.md`. Run each command before recording it; copy nothing unverified |
| Authorization boundaries, protected areas, "never touch X" | `docs/project-rules.md`, in the reserved decisions and external state sections |
| Judgment rules duplicating harness guidance | Drop in favor of the harness owner; note the mapping in the ledger |
| Judgment rules the harness lacks, backed by evidence | Keep as a project delta in the correct owner; propose upstream to the template in the final report |
| Repeated specialist workflows | Keep as a project skill validated by `scripts/validate-skill.sh`, or map to `verify`, `refactor`, or `take-a-step-back` and deprecate |
| Machine-verifiable rules stated as prose | Convert to a script or CI check, or map to an existing harness script. Delete the prose |
| Incident history, one-off lessons, session logs | Delete. Git history is the archive. Promote only lessons with repeated evidence, through the change gate |
| Provider- or model-specific prompt recipes | Delete unless repeated product-specific evidence justifies a per-runtime adapter |
| Stale content: dead paths, retired tools, obsolete processes | Delete; record in the ledger |
| Runtime compatibility files | Replace the body with a pointer to `AGENTS.md` |

Default when torn between keep-as-delta and drop: drop, and record it in the ledger. The receipts-and-retro loop will re-add a rule that reality asks for, and restoring from the ledger is cheaper than carrying an unused rule in every context window.

Known template-level gaps and their reopen triggers live in the template repository's `meta/source-analysis.md`. Consult it before concluding that a missing capability is yours to invent.

## Phase 2: install or upgrade

**First installation:** copy the harness payload excluding `meta/`, then follow `docs/project-adaptation.md` steps 2 through 9, filling `docs/project-rules.md` from the Phase 1 harvest.

**Upgrade** (harness already present at an older recorded SHA): diff the project against the template at that SHA and apply the three-bucket rule:

- Project-owned, never overwrite: `docs/project-rules.md`, `plans/` (handoff notes, receipts), registered runtime profiles, and any file local retros changed.
- Template-owned, take upstream: `scripts/`, `docs/examples/`, and skills without local modifications.
- Merge deliberately: `AGENTS.md`, `wow.md`, and retro-modified docs. Re-apply the local changes on top of the new template text, never the reverse.

Record the new template SHA in `docs/project-rules.md` when done.

## Phase 3: cut over

Clean cutover, per the design principles: the same change that installs a harness owner removes what it supersedes. No parallel instruction sources, no deprecated-but-present folders, no commented-out rules. Update every reference to removed paths: READMEs, CONTRIBUTING, CI configuration, hooks, documentation links.

## Phase 4: prove and hand over

1. `scripts/validate-harness.sh` passes in the repository. Outside the template repository (detected by the absence of the template marker `meta/harness-design.md`) the audit also fails on unreplaced `docs/project-rules.md` placeholders, along with the always-loaded cap.
2. The commands recorded in `docs/project-rules.md` each actually ran, at minimum the focused test and the build.
3. A final sweep finds no orphaned instruction files and no dangling references to deleted ones.
4. Report with the ledger first: dispositions by bucket, deletions with reasons, conflicts escalated, deltas kept, and upstream proposals for the template. Append a receipt (`scripts/receipt.sh add`) and recommend the first retro after a handful of tasks rather than at the default cadence.

Reconciliation is complete when the human has accepted or vetoed every ledger row. Moving the files is not what finishes the job.
