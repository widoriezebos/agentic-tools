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

