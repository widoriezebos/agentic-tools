# Reviewers check the rulings

- Kind: design
- Id: 01M3V7CNH4QK6RSXE96JCSBQ1K
- Status: draft
- Goals: reviewers-check-the-rulings

Revision 1. Author: Fable (claude-fable-5-1), ui seat, 2026-10-01. Cites were read at origin/main `5c01d6918`; paths are under `metasystem/`.

Wido, 2026-09-30, verbatim: "When the reviewer, either design or code, is reviewing, does it get injected or referenced the project and goal level rulings? Because we need to make sure nothing violates these. Is that already happening? Should it happen? What do you think?" then "It should definitely be one of the tasks that the reviewer has to check the rulings." then "Create a goal with a high priority that this needs to be designed and implemented." and "This should not be a very complicated big goal." Wido, 2026-10-01, verbatim: "yes, make sure we stick to 'build the smallest thing possible that works'".

## 1. What is true today

- Neither critic is told about the rulings. The design critic's binding list is "the brief-named instruction documents, including this preamble, the skill, and the project rules" (`internal/protocol/roles/design-critic.md:20`; the code critic's is identical, `code-critic.md:18`). Neither skill names `memory/rulings.md` (`skills/design-critique/SKILL.md`, `skills/code-critique/SKILL.md`: no match), and no engine code reads the register for a critic: its readers are the slice-cap approval (`internal/dispatch/slice.go:115`), the `internal/rulings` parser (`rulings.go:149`) for the steward's sweep and the interface's decisions page, goal norms (`internal/goal/norm.go:58`) and landing's id-minting law (`internal/landing/landpath/land.go:572`).
- The register is referenced, never injected, and that is the only option: it is 175 rows and 181 KB, almost three times the 64 KiB packet ceiling (`internal/dispatch/composition.go:186`), so a packet could only carry a reference to it. A critic reads it from its checkout: rows start `| R-<id> |` (`internal/rulings/rulings.go:168`), so one id is one `grep`.
- The dispatched brief already names the goal. `reviewBrief` writes "goal G's approved review-round limit" (`cmd/metasystem/intent_delivery.go:872`); the design review takes the goal from `--goal` or the page's single `Goals` line and refuses without one (`:961-968`), the code review from the builder's job record and refuses without one (`:1073-1081`). Nothing in the off-limits files needs to change.
- A goal's human decisions are readable by one verb: `metasystem goal show G --history` prints the intent, the next step and every history line with its actor and reason (`plans/goals/reviewers-check-the-rulings.md:19-29` is this goal's).
- The packet is the brief plus three fixed sources per role (`internal/protocol/role-packets.json:38-53`): the role file, the skill, the schema. The role file is compiled in (`internal/protocol/protocol.go:20`); the skill is read from the checkout at dispatch (`composition.go:579`).
- The return cannot take a new field cheaply: the schema closes its top level (`internal/protocol/schemas/design-critic.schema.json:6`, enforced at `internal/returnschema/complete.go:556-563`) and every version since adds to it by code (`returnschema.go:81-109`). It already has an `evidence` row of `command`, `observed`, `level` (schema `:22-34`) and a free `gaps` list (`:35`); finding ids are free strings (`:45`), and `MOVED-EFFECTS-` is a prefix convention a role already uses (`design-critic.md:18`).
- A role quotes its skill between `<!-- quote source=... -->` markers; `PreambleQuotes` refuses a quote that is not byte-for-byte in the source (`internal/validate/preamblequotes_check_test.go:30-98`), and `TestShippedPreambleQuotesHoldAndNameEachDrift` runs it on the shipped roles and requires named blocks, today in the orchestrator only (`internal/validate/shipped_protocol_test.go:136-161`, helper `countQuoteBlocks` at `:84`).
- This seat's hand-written briefs already carry the rule (`agentic-tools-evidence/ask-follow-ups-20261001/sol-read-brief.md:5`); a critic dispatched by the engine does not get it.

## 2. Step 1: one paragraph, in four files, and one test

The same paragraph goes into both skills as a section headed `## Check the Rulings`, and each critic role quotes it from its own skill, so the role text binds the critic and the audit keeps the two copies identical. The paragraph:

> Before the materiality test, read the standing rulings register, `memory/rulings.md`, and the human decisions on the goal the brief names (`metasystem goal show <goal> --history`: the intent, the next step, and every history line a `human:` actor wrote, with its reason). Work that contradicts a ruling or a goal decision is a material finding whatever else is true, because it takes authority a human did not give: its id is `RULING-` followed by the ruling id (`RULING-R-124-m1u`) or the goal id, its claim quotes the ruling's words, and its evidence cites the text under review that breaks them. The return lists what was checked in one `evidence` row: `command` names the register and the goal, `observed` lists the ruling ids read and says `no conflict` or names the findings, and `level` is `read`. A brief that names no goal checks the register alone and says so in that row; a register or goal the critic could not read is a `gaps` entry, never silence.

- `skills/design-critique/SKILL.md`: the section goes between "Step 1 Before Anything" (`:24`) and "The Materiality Criterion" (`:28`). `skills/code-critique/SKILL.md`: between "The Materiality Criterion" (`:14`) and "Layer 1: Conformance" (`:25`).
- `internal/protocol/roles/design-critic.md` and `code-critic.md`: a second quote block, from the role's own skill, directly after the existing one (`:7-9` in both). The "binding instructions" sentence is not edited; the quoted paragraph binds by itself.
- Why a sentence and not a field: the evidence row costs nothing and is validated today; a `rulingsChecked` field is a schema version, a role-text change and validator code for the same information. Why the quote and not two wordings: the audit already refuses drift between a role and its skill, so one paragraph stays one.
- Why "always material" is no exception to R-124: the skill's SAFE test already names "authority a human did not give" (`skills/design-critique/SKILL.md:38`); a design that overrides a standing ruling is exactly that. A `RULING-` finding's rigor row marks `authorityBoundaryCrossed`, so it is `severe` and waits on the human (`:75`); the author may still refute it with evidence that there is no conflict.
- The test: the mandated-block table in `TestShippedPreambleQuotesHoldAndNameEachDrift` (`shipped_protocol_test.go:144-161`) gains a file column and two rows, `design-critic.md` from `skills/design-critique/SKILL.md` and `code-critic.md` from `skills/code-critique/SKILL.md`, marker `memory/rulings.md`, each expected exactly once. It is red when the block is deleted from a role (count 0) and, through the existing `PreambleQuotes` fatal at `:140`, when the paragraph is removed or reworded in a skill (the quote no longer occurs in its source). About ten lines in an existing test; no new package.
- No cheap test proves that a critic REPORTS a planted conflict: that is a model run. The goal's done-when is proven by hand on this change's own Sol review, the first dispatched critic after the skill text lands: its return carries the evidence row with ruling ids (clause 1); clause 2, a `RULING-` finding on a real conflict, is read at the first review that meets one. Precedent: `plans/designs/critique-findings-need-proof.md:76`.

Moved effects: none. No write, notice, count or state changes owner (text and one test only), so the page carries no inventory section; `design review --check-only` reports `inventory=absent rows=0 problems=0`.

## 3. Guardrails the build must pass

- `go test ./internal/validate -run TestShippedPreambleQuotes`: the quotes are byte-exact and the two new mandated blocks are present once.
- The traveling-surface vocabulary audit over roles and skills (`internal/audit/metasystem.go:208-260`, run by `internal/audit` and `internal/adopt` tests): the paragraph uses no bare "the job/runner/turn/gate/event"; it does not today.
- `go run ./cmd/devgate static`: the project check reads this head (`cmd/devgate/static.go:312-320`, keys at `internal/project/record.go:16,22`), plus the fast gate.
- Path classes: `internal/` and `skills/` are behavior, `plans/` is record (`internal/pathclass/path-classes.txt:19,21,34`); no new package, so no coverage-floor rows, and `internal/validate` only gains a test.
- Copies: none to mirror. `skills/*/agents/claude-profile.md` are byte-identical to `.claude/agents/*-critique.md` but only say "read the skill and follow it", so step 1 leaves them alone. The role file is compiled in, so a seat's binary must be rebuilt before a dispatched critic gets the new role text; the skill reaches it from the checkout at once, and the skill alone carries the step.

## 4. Later, when it hurts

Tagging or ranking rulings by topic and automatic relevance filtering (the field: the evidence row's id list). Superseded rulings (two rows say so in prose, R-139-m1e among them; the critic reads the row as written). A `rulingsChecked` return field with engine validation, if the evidence row proves unreliable. An engine-composed packet slot or digest for the register. The code critic reading the register in the builder's worktree, which can lag main by a few rulings. Project-wide designs that name no goal (today: register only).

## 5. Open questions for Wido

1. A `RULING-` finding is `severe` by construction and so waits on you at close; only your word (a superseding row in the register, or `goal accept-risk`) or the author's evidenced refutation clears it. Recommendation: yes, that is what a human decision means.
2. The critic reads the whole register (181 KB) in bounded views every round, a few tool calls out of its budget. Recommendation: accept it for step 1; filtering is on the later list.
3. Proof of the planted conflict: a hand check at the first real review that meets one, as above, or a one-off paid critic round on a throwaway page that contradicts a ruling. Recommendation: the hand check; the row with ruling ids is already proven by this change's own Sol review.
