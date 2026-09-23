# Astra's critique of g1-s30 revision 1

Produced 2026-09-23 by Codex on `gpt-6-astra`, read-only, against revision 1 of [g1-s30-the-partner-knows-the-territory.md](g1-s30-the-partner-knows-the-territory.md) at `af3ded932`, under R-121. Verbatim; the dispositions are the design's revision 2.

---

The draft passes the **step-1 gate**: it names a usable first slice and a deferred list. I found **six material findings** under R-121.

Evidence: repository code, designs, critiques, and tests read at clean `ui-development`, `af3ded932`. Scenarios below are inferred failure cases. No edits, tests, or agents; `metasystem/metasystem.conf.local` remained unopened.

1. **F1 — High: the proposed knowledge routes do not establish expertise in the metasystem itself.**

   **Scenario:** Wido asks, “What is a lease epoch?”, “Which ruling changed the agent roster?”, or “How does `goal next` choose work?” The skill routes terms to interface help, past choices to project decisions, and goals to ledger tools. Those sources do not necessarily explain these questions.

   The kit already names the owners: [AGENTS.md](../../../metasystem/AGENTS.md:14) points to the glossary and backlog laws; [wow.md](../../../metasystem/wow.md:5) routes operating guidance; the [command catalogue](../../../metasystem/cmd/metasystem/main.go:18) owns verb names and descriptions. Rulings live outside the resolver’s decisions home. This checkout’s [doctrine index](../../../metasystem/docs/doctrine/index.md:13) happens to expose concepts and architecture, but an adopted application’s doctrine describes that application.

   **Step-1 change:** give the Partner skill an explicit route into the installed kit’s existing knowledge owners, separate from project memory and interface vocabulary. Distinguish explanations of ledger rules from observations returned by ledger tools. Reuse `wow.md` for workflow routing rather than recreating its table. Reading a skill to explain a workflow must also remain distinct from undertaking that workflow.

   Without this, the Partner may find the answers through exploratory reading, but the supporting system has not made that expertise dependable.

2. **F2 — High: the manifest needs an explicit boundary between TypeScript facts, Go facts, and runtime facts.**

   **Scenario:** the same built interface serves this self-hosted repository and an adopted application. A bundle-generated concrete record-home path is correct for one and wrong for the other. Alternatively, a Go-only change updates a model default or an act requirement while the old manifest remains “current.”

   Record homes are computed by [the Go resolver](../../../metasystem/internal/project/project.go:172). Settings live in [config/ui.go](../../../metasystem/internal/config/ui.go:11), with model and command fallbacks in [partner/runtime.go](../../../metasystem/internal/ui/partner/runtime.go:68). Act authorization is conditional: [mayAct](../../../metasystem/internal/ui/httpd/acts.go:212) considers session state, Partner configuration, and boot authority.

   Generating from these owners is feasible, but **the design has not chosen the data flow**. Scraping or separately restating their rules in TypeScript creates the second account it promises to avoid. Including `interface.json` in output hashes also does not solve source staleness: [bundle generation](../../../metasystem/internal/ui/web/_app/scripts/bundle.mjs:189) and [the currentness test](../../../metasystem/internal/ui/web/bundle_test.go:117) digest `_app`, not the Go owners.

   **Step-1 change:** specify whether Go exports static descriptions into the build or enriches the frontend manifest when serving it. Resolve actual homes at runtime; distinguish defaults from effective values and general requirements from current eligibility. Cover every static generation input in the staleness check.

   The maintenance contract should be explicit:

   | Addition | Authoritative changes |
   |---|---|
   | Section | Route, rendered pane/availability, and reference to its help description |
   | Term | Help-register entry and its use by the interface |
   | Setting | Go configuration owner; runtime-specific fallback owner where applicable |
   | Record kind | Resolver’s kind/home contract and the corresponding readers/writers/views |
   | Skill | Canonical `SKILL.md` and its `wow.md` route |

   These changes can span files without duplicating knowledge. The requirement should be **one owner per fact**, with no additional Partner-specific account.

3. **F3 — High: complete descriptions can still teach capabilities this build does not have.**

   **Scenario:** Wido asks, “Can you edit this with me?” The generated manifest faithfully repeats that the Partner “can write with you,” while the standing rule says it cannot write. Asked where to inspect Fleet health, it describes an operational Fleet page that currently displays a placeholder.

   These conflicts already exist: [Partner help](../../../metasystem/internal/ui/web/_app/src/help/terms.ts:146) promises writing; [the standing rule](../../../metasystem/internal/ui/partner/context.go:82) forbids it. [Empty-pane descriptions](../../../metasystem/internal/ui/web/_app/src/panes/empties.ts:46) identify several unbuilt sections. The [lane register](../../../metasystem/internal/ui/web/_app/src/backlog/lanes.ts:75) also distinguishes defined lanes from lanes actually shown.

   **Step-1 change:** make the manifest distinguish purpose, implemented availability, and the Partner’s own capabilities. Reconcile contradictory source descriptions at their owners. A missing-description check can establish coverage; it cannot establish correctness. Include a check that the Partner accurately explains an unavailable section and its inability to act.

4. **F4 — High: the scripted answer check does not test knowledge composition.**

   **Scenario:** an implementation accidentally stops supplying the index and skill, or never registers `interface()`. Every proposed fake-runtime answer still passes because its expected facts were placed directly in the scripted response.

   The current fake explicitly [discards prompt parameters](../../../metasystem/internal/ui/partner/fakeacp/fakeacp.go:181) and [streams configured chunks](../../../metasystem/internal/ui/partner/fakeacp/fakeacp.go:221). It proves transport behavior, not that an answer followed from supplied knowledge.

   **Step-1 change:** separate two kinds of evidence:

   - Deterministic tests inspect the actual protocol prompt, advertised tools, and tool responses. Removing an input or breaking retrieval must fail the relevant assertion.
   - The manual real-runtime check runs through the production composition path and retains the questions, answers, source revisions, runtime/model, and relevant reads. Its expectations must reject contradictions, unsupported claims, and incorrect citations—not merely count matching words.

   A dozen real questions can provide useful smoke-test evidence. They cannot certify general expertise, and a dozen scripted answers provide no evidence of expertise at all.

5. **F5 — High: refreshing after session loss does not address an index becoming stale during conversation.**

   **Scenario:** Wido asks about open questions, answers one through the interface, then returns to the same Partner conversation. The startup index still calls it open. A newly added decision is absent altogether.

   The [project pane reads afresh per request](../../../metasystem/internal/ui/project/project.go:152), while [the host reuses a live session](../../../metasystem/internal/ui/partner/host.go:193). Its hour limit is an **idle** limit, restarted after turns—not a maximum age for an actively used session.

   The size bounds are sensible, but make the index incomplete by construction: summary-rich rows can exhaust 12,000 characters long before 300 lines. A record missing from that block may be omitted, newly created, or absent.

   **Step-1 change:** define the index as a dated discovery aid, never authoritative evidence of current status or nonexistence. Require current listing/search and source reads for those claims, using the already planned tools. Specify selection and omission behavior, including read failures.

   Also locate the composition in the actual lifecycle: [the standing rule is currently composed every turn](../../../metasystem/internal/ui/partner/service.go:296); session opening does not install a separate standing-rule message. State which material accompanies the first prompt, what subsequent prompts refresh, and what recovery rebuilds. This needs no background synchronization system.

6. **F6 — Medium: the promised uniform index is not the index the resolver currently produces.**

   **Scenario:** the Partner receives the doctrine chapter “The metasystem in concepts” and tries to fetch it by record ID. That chapter is actually a `doc:` binding with a path and no independently declared record ID or status.

   [Resolver chapters](../../../metasystem/internal/project/project.go:94) distinguish record IDs from document paths. Questions are register rows. The [list verb](../../../metasystem/cmd/metasystem/project_verbs.go:75) emits record metadata and paths, but no summary. Summaries already have an owner in [the UI project projection](../../../metasystem/internal/ui/project/project.go:210), using [first-paragraph extraction](../../../metasystem/internal/ui/project/summary.go:10).

   **Step-1 change:** define retrieval references for records, path-backed chapters, and question rows; preserve absent metadata rather than inventing it. Reuse the existing summary rule. Otherwise the builder must choose between dropping valid chapters, fabricating metadata, or creating a second index/summary interpretation.

**Outside step 1**

- Appending the canonical Partner skill text is a reasonable loading mechanism for this dedicated role. A generic skill engine, automatic loading of every skill, or a new routing registry is unnecessary.
- The read-tool handoff and permission exception belong to the accepted g1-s29 dependency. At this commit they are not built: [session opening supplies no tool servers](../../../metasystem/internal/ui/partner/host.go:263). Do not redesign that mechanism here. Its implementation must also replace the old “supplied goals are all you know” instruction; the new skill cannot make unavailable tools exist.
- Large-scale retrieval systems, embeddings, background indexing, and automatic context compaction are not justified by this slice.
- Scheduled evaluations, cost displays, exhaustive source links, and adopted-application domain teaching remain deferred.
- Fixing every historical documentation inconsistency is outside this slice. Correcting descriptions newly presented as authoritative build capabilities is F3.

Proposed receipt, not written: `Read-only critique of g1-s30 revision 1 at af3ded932; six material step-1 findings; repository evidence read; no edits, tests, or agents.`

**Verdict: build after listed changes — 6 material findings.**

