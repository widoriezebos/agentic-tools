# Astra's critique of g1-s66

Produced 2026-09-28 by Codex on `gpt-6-astra`, read-only, against [g1-s66-the-loop-from-the-room.md](g1-s66-the-loop-from-the-room.md) at `3e1262633`, under R-121 and R-124, brief `~/LocalStorage/agentic-tools-evidence/review-room-20260928/astra-design-brief-loop-from-the-room.md`. Verbatim; the dispositions are at the design's foot.

## Round 1

---

Five material findings. Step 1 and its deferrals are explicit, but the proposed wrapper does not yet match the engine’s start, findings and closure contracts. Evidence below was checked by reading code at `3e1262633`; failures are inferred, not runtime reproductions.

**S66-01 — High — material: yes — The specified act cannot start a review.**

**Claim/evidence:** D1 specifies two command forms without `--tool-calls`, and §6 supplies only `{goal, after?}`. The engine unconditionally requires a positive reader tool-call budget, then checks for an existing approved goal budget: [intent_delivery.go:771](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_delivery.go:771). Its test explicitly expects the budgetless design-review command to refuse: [intent_delivery_test.go:223](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_delivery_test.go:223).

**Concrete failure:** The first Send to critique press, implemented with the documented arguments, starts nothing. The story also opens its implementation goal only afterwards, without establishing which existing approved goal funds critique.

**Change to the design:** Specify the reader-budget source and pass it on both forms. Name how the human selects or confirms the existing funding goal, including when the design names several goals. Keep the engine’s approval and claim refusals visible.

**Test 1 — DIFFERENT/WRONG:** Yes; the invocation and admission contract must change.  
**Test 2 — WORKS/SAFE:** WORKS: no. SAFE: yes—the engine refuses rather than inventing a budget.

**S66-02 — High — material: yes — The findings register cannot supply the promised round cards.**

**Claim/evidence:** D2 requires each round’s material and non-material findings, including severity, requested change and both tests. The register skips new non-material findings and combines findings by stable id across rounds: [finding_register.go:1329](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/dispatch/finding_register.go:1329). Its serialized entries omit severity, requested change and the two tests: [finding_register.go:1119](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/dispatch/finding_register.go:1119). Even the structured return schema provides only id, severity, material, claim and evidence: [design-critic.schema.json:38](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/scripts/agents/schemas/design-critic.schema.json:38).

**Concrete failure:** A valid return containing one material and one non-material finding produces only one register-backed card. The page can say every card is answered while the engine’s join still requires the omitted finding. This occurs with structured returns, so §9’s prose fallback does not address it. A recurring finding also retains its id: a document-wide `[f:<id>]` mark can make round 2 appear already answered using round 1’s decision.

**Change to the design:** Read cards from each round’s retained return; use the register for chain status. Specify where the additional card fields come from, without inventing missing values. Scope disposition identity and duplicate suppression to chain, round and finding.

**Test 1 — DIFFERENT/WRONG:** Yes; the read source and identity contract change.  
**Test 2 — WORKS/SAFE:** WORKS: no. SAFE: no—the page can silently omit findings or reuse an earlier adjudication.

**S66-03 — High — material: yes — “Compose the dispositions file from the table” omits mandatory engine data.**

**Claim/evidence:** D2/D4 describe rows containing “folded”, “refuted” and “deferred”. Continuation first requires an exact `Review binding` naming goal, design record, examination, round, reviewed subject digest and return digest: [intent_review_binding.go:37](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_review_binding.go:37), [intent_design_review.go:157](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_design_review.go:157). The table requires four named columns and accepts only `accepted`, `refuted`, `noted` and `out-of-scope`: [critiqueclosed.go:13](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/validate/critiqueclosed.go:13), [critiqueclosed.go:246](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/validate/critiqueclosed.go:246). `noted` cannot answer a material finding.

**Concrete failure:** After answering the first round, a file composed solely from the described table is refused for its missing binding. Adding a guessed header still leaves Fold/Defer unmapped and cannot safely associate the decisions with the exact examination.

**Change to the design:** Base export on the engine’s retained binding/template; export exactly that round’s rows. Define the canonical columns and action mappings, including when Defer is allowed and what evidence it needs. Create the table on first use: the shipped design template has no Dispositions section ([write.go:120](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/write.go:120)).

**Test 1 — DIFFERENT/WRONG:** Yes; the export payload and validation change.  
**Test 2 — WORKS/SAFE:** WORKS: no. SAFE: yes—the current engine rejects the incomplete export.

**S66-04 — High — material: yes — Recording decisions changes the reviewed subject, so D4 does not close the loop.**

**Claim/evidence:** D2 writes every disposition into the design itself. The engine hashes the entire design file: [intent_delivery.go:894](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_delivery.go:894). With dispositions, it attempts closure only when that hash still equals the reviewed subject; otherwise it requests a follow-up: [intent_design_review.go:178](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_design_review.go:178). D4 specifies Send round 2 but no explicit final-close operation.

**Concrete failure:** Even an all-refuted round changes the design through its table rows and therefore requests another paid examination instead of closing. After round 2, recording decisions changes the hash again; the same route attempts a prohibited continuation. Merely reading chain state cannot perform the missing close. The skill requires stopping at the first round without material residue: [SKILL.md:47](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/skills/design-critique/SKILL.md:47).

**Change to the design:** Define an explicit engine-owned close path, distinct from requesting another examination, that accommodates the in-record adjudication trail. Specify early closure, final-round closure and human-required refusal. Return the engine’s confirmed close outcome and exit wording to the page.

**Test 1 — DIFFERENT/WRONG:** Yes; this changes control flow and the engine/interface boundary.  
**Test 2 — WORKS/SAFE:** WORKS: no. SAFE: no—the design can buy an unnecessary round and cannot establish the promised final closure.

**S66-05 — High — material: yes — A heading string does not identify exactly one replacement target.**

**Claim/evidence:** D3/§6 identifies a target only by document and heading, reusing Outcome replacement. That implementation selects the first matching heading and appends a new section when none exists: [sitting.ts:399](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/sitting.ts:399), [sitting.ts:434](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/sitting.ts:434). The writer validates the document revision, not which section the human intended: [edit.go:89](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/edit.go:89).

**Concrete failure:** A suggestion intended for the second occurrence of a heading can overwrite the first, with a valid revision. A missing heading can create a section while the card claims to have replaced one.

**Change to the design:** Bind the preview and write to one unambiguous section. The smallest rule is to refuse absent or duplicate headings and retain the draft. After a revision conflict, refresh the comparison before offering Use again; do not silently apply an old comparison against newly read words.

**Test 1 — DIFFERENT/WRONG:** Yes; target resolution and conflict behavior change.  
**Test 2 — WORKS/SAFE:** WORKS: no for these explicitly scoped inputs. SAFE: no—human text can be replaced under the wrong heading.

**Deferred and non-material**

- Two build lanes are not, by themselves, a violation of “smallest thing.” They can deliver one usable loop. Reducing visual polish or separating D5 is a scope preference: Test 1 DIFFERENT, but Test 2 WORKS/SAFE remains yes.
- Richer diffs, multiple simultaneous design loops and alternative critics remain deferred on the named card and chain surfaces.

**What I verified holds**

- The premise preserves human authority: the Partner drafts, the critic reports, and the human adjudicates. Critique does not accept the design.
- The launch precedent explicitly requires a live, checkout-valid session proof ([launch.go:72](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/httpd/launch.go:72)).
- Review continuation retains operation identity and rejoins its child; it rejects a child belonging to another chain ([intent_design_review.go:231](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_design_review.go:231)).
- The sitting recorder serializes writes, rejects another record and retains words after a revision conflict ([recording.ts:159](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/recording.ts:159)).
- D5 has an existing goal-opening and association path, including explicit partial-failure reporting ([DocumentPane.tsx:483](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/project/DocumentPane.tsx:483)). Goal ids are appended space-separated ([write.go:402](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/write.go:402)).
- The engine owns the two-round limit and the fixture-versus-human exhaustion decision; the page should consume those outcomes.

VERDICT: 5 material findings (fail test 2): S66-01, S66-02, S66-03, S66-04, S66-05

Commit reviewed: `3e1262633880efe47927f499772d97895f5f2814`.

Limitations: Read-only review; no tests, paid runs, subagents or edits. Excluded paths were not read. The checkout advanced during review; cited files were checked against the dispatched commit and were unchanged.


## Round 2, the declared failsafe

---

Four prior findings are answered. **S66-04 remains open**, with three material closure defects detailed below. Evidence was checked by reading source at the dispatched commit; failure scenarios are inferred, not runtime reproductions.

**Round 1 findings**

| Finding | Status | Revision 2 and code evidence |
|---|---|---|
| S66-01 | **CONFIRMED ANSWERED** | [D1:129](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:129) supplies the funding goal and editable reader budget. This addresses [the mandatory admission inputs](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_delivery.go:771). |
| S66-02 | **CONFIRMED ANSWERED** | [D2:146](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:146) reads each retained return, limits guaranteed fields to the schema, and identifies cards by chain/round/id. This avoids the register’s [non-material filtering](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/dispatch/finding_register.go:1365). The erroneous filename in §6 is noted below. |
| S66-03 | **CONFIRMED ANSWERED** | [D2:157](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:157) uses the engine’s bound decisions template and canonical columns/values, matching [its writer](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_review_binding.go:68). D4 creates the record’s appendix at close. |
| S66-04 | **STILL OPEN** | [D2:169](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:169) fixes decision rows changing the design digest. However, [D4:189](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:189) still specifies incorrect closure behavior; see S66-06 and S66-07. Its new appendix also introduces S66-08. |
| S66-05 | **CONFIRMED ANSWERED** | [D3:178](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:178) refuses absent/duplicate headings, retains the draft and refreshes the comparison after conflict. These requirements address the [first-match Outcome helper](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/partner/sitting.ts:399) and [document-level revision check](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/edit.go:89). |

**S66-06 — High — material: yes — A valid refutation does not make the unchanged-design close succeed.**

**Claim and evidence:** [D4:193](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:193) assumes an unchanged design closes after its findings are decided. The cited helper checks for accepted material findings, then [calls the whole-chain close](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_design_review.go:340). That path invokes `CritiqueClosedWithRegister`, which [persists only `out-of-scope` resolutions](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/validate/critiqueclosed.go:55), not refutations. The close owner then [checks the canonical register](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/delegation/phases.go:369); unresolved bounded findings before exhaustion [refuse closure](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/dispatch/finding_register.go:662).

**Concrete failure:** Round 1 contains one registered bounded material finding. The human refutes it with valid evidence and presses Answer the round. The table joins successfully and the design digest is unchanged, but the register still contains an open finding. The engine refuses instead of completing this ordinary first-use path.

**Change to the design:** Add an explicit engine obligation connecting validated author refutations to the register’s closure decision. Preserve the distinction between refuting a finding and accepting its risk. Require a fixture exercising this path through the real register close.

**Test 1 — DIFFERENT/WRONG:** DIFFERENT: the engine must apply adjudications, not merely validate their table.  
**Test 2 — WORKS/SAFE:** WORKS: **no**. SAFE: **yes**, because the existing engine refuses.

**S66-07 — High — material: yes — The blanket final-round “human residue” rule replaces the required exit decision.**

**Claim and evidence:** [D4:203](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:203) closes any changed final-round design with all material findings decided as “human residue.” The binding [skill:67](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/skills/design-critique/SKILL.md:67) distinguishes clean closure, closure on fixture obligations, and human-required refusal. The engine [implements that distinction](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/dispatch/finding_register.go:651), including [publishing obligations before marking findings deferred](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/dispatch/finding_register.go:678).

**Concrete failure:** Material findings fall from two to one; the remaining finding is bounded, mechanical and names a fixture. The human folds it after round 2. D4 sends this valid fixture-exit case to “human residue,” without specifying the required obligation. Conversely, an accepted invariant finding also satisfies “all decided”; that condition cannot authorize closing a chain which requires explicit human risk acceptance or re-scoping.

**Change to the design:** Send final-round decisions through the engine’s existing exit classification. Return clean closure, fixture closure with its obligations and exit wording, or human-required refusal with the chain left unclosed. Remove “all decided plus changed digest” as a sufficient closure condition.

**Test 1 — DIFFERENT/WRONG:** WRONG: the prescribed outcome mapping changes both closure and obligation creation.  
**Test 2 — WORKS/SAFE:** WORKS: **no**, because a lawful fixture exit becomes the wrong outcome. SAFE: **no** if “closes” marks unresolved human-required residue closed; retaining the refusal avoids that risk but still fails the promised outcome.

**S66-08 — Medium — material: yes — Appending only the closing round’s file omits earlier human adjudications.**

**Claim and evidence:** [D2:157](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:157) makes decisions examination-specific. [D4:199](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:199) appends the Dispositions section from “the decisions file” supplied at close. The engine [creates a separate template beside each round’s return](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_design_review.go:369), populated with [that examination’s findings only](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_review_binding.go:78).

**Concrete failure:** Round 1 records an accepted amendment and the human’s reason for refuting another finding. Round 2 confirms those findings withdrawn; its cards receive `noted`. Appending round 2’s file produces the confirmations but omits the original amendment and refutation reasoning. The earlier files survive elsewhere, but the promised record-alone handoff lacks the human’s decisions.

**Change to the design:** Compose the appendix from every answered round in the chain, retaining round identity, reasoning and amendment. Verify that recurring ids preserve both adjudications and that a quiet final round does not erase the earlier trail.

**Test 1 — DIFFERENT/WRONG:** DIFFERENT: appendix composition must consume the chain’s answered rounds, not just the closing file.  
**Test 2 — WORKS/SAFE:** WORKS: **no** for the complete-record handoff. SAFE: **no** for that handoff’s information integrity: it silently omits human decisions, although their engine files remain.

**Deferred and non-material**

- §6’s `outputs.md` citation is wrong: it contains the [declared design path](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_delivery.go:913); the retained return is [round-specific `return.json`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_delivery.go:512). D2 already establishes the correct source unambiguously. Material: no; Test 1: no substantive difference; Test 2: WORKS/SAFE yes following D2.
- Two build lanes can deliver one usable step. Reducing polish or separating D5 is a scope preference. Material: no; Test 1: DIFFERENT; Test 2: WORKS/SAFE yes.
- Richer diffs, alternative critics and simultaneous designs remain deferred on their existing surfaces.

**What I verified holds**

- The premise preserves human adjudication and approval; Partner suggestions and proposals prepare actions without applying them.
- The launch precedent [requires live, checkout-valid session proof](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/httpd/launch.go:72).
- Continuation [checks the examination binding and `--after`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_design_review.go:163) and [rejoins its retained child rather than requesting another](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_design_review.go:231).
- D5 has an existing association path with [explicit partial-failure reporting](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/web/_app/src/project/DocumentPane.tsx:483) and [space-separated goal ids](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/ui/project/write.go:402).

Proposed receipt, unwritten: g1-s66 revision 2 failsafe design review; four prior findings answered, closure finding remains open, three material findings reported.

**VERDICT: 3 material findings (fail test 2): S66-06, S66-07, S66-08**

Commit reviewed: `0324a0cc4b0a3ce71265aee42d67e829995c3b41`.

Limitations: Read-only source review; no tests, commands executing the engine, paid runs, subagents or edits. Prohibited paths were not read. Reviewed files were checked unchanged against the dispatched commit; unrelated concurrent edits were left untouched.


## Confirmation read on S66-06 to S66-08

---

No new findings. All three are **CONFIRMED ANSWERED as design obligations**; this does not certify their implementation.

| Finding | Confirmation and evidence |
|---|---|
| **S66-06 — CONFIRMED ANSWERED** | [D4:201](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:201) requires validated decisions to reach the register before closure, distinguishing refutation from risk acceptance. This addresses the [join persisting only `out-of-scope`](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/validate/critiqueclosed.go:72) and the [close refusing unresolved findings before exhaustion](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/dispatch/finding_register.go:667). |
| **S66-07 — CONFIRMED ANSWERED** | [D4:211](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:211) explicitly requires clean closure, fixture closure with published obligations, or human-required refusal leaving the chain open. These match the [engine’s classification](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/dispatch/finding_register.go:643) and [obligation publication](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/dispatch/finding_register.go:689). “All decided” no longer independently authorizes closure. |
| **S66-08 — CONFIRMED ANSWERED** | [D4:217](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:217) composes the appendix from every answered round, retaining round identity, reasoning and amendment. This accounts for the [separate round-local decisions files](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_design_review.go:369), whose [templates contain that examination’s findings](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/intent_review_binding.go:78). |

What I verified holds, by reading:

- [§8:281](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:281) requires the unchanged-design refutation fixture through the real register close, all three final-round outcomes, and preservation of recurring findings across answered rounds.
- [D2:169](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:169) keeps decision presses from changing the design digest; [§6:251](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/designs/user-interface/g1-s66-the-loop-from-the-room.md:251) now names each round’s retained `return.json`.
- Within these folds, I found no remaining specified first-use path that wrongly blocks a closable round, closes human-required residue, or omits a human’s earlier decision from the handoff. Human-required refusal intentionally remains open.

Under **R-124**, the folds address first-use requirements: removing them would change implementation and restore failed closure, incorrect exit classification, or an incomplete handoff. No residual issue warrants another material finding under both tests.

Proposed receipt, unwritten: scoped S66 confirmation; S66-06–08 answered; zero new material findings.

**VERDICT: 0 material findings: none**

Commit reviewed: `c93f0483d0eed4d701ff2128b1f4aaf0ec70ce11` (`ui-development`); reviewed files match that commit. Limitations: scoped source reading only; no runtime tests, edits or subagents. Prohibited paths were not read.

