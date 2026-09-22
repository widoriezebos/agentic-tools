# Astra's critique of the memory-system design, revision 4 (round 2)

Produced 2026-09-22 by Codex on `gpt-6-astra`, read-only, on Wido's order that the author and Astra converge. Round 1 is in [memory-system-critique-astra.md](memory-system-critique-astra.md); the design's dispositions are in [memory-system-r6.md](memory-system-r6.md), revision 6, section 19 (revision 5 was the reply to this round). The critique follows verbatim.

---

**redesign — 12 material findings remain; the universal immutable-row, “latest binding wins” mechanism conflates authority, applicability and ordinary metadata, and needs a narrower replacement.**

Read against source at `b712f2f0339765b158b4deac5e7457a270d083b5`. Code evidence was read; failure scenarios below are inferred from revision 4’s contracts. No files changed, tests run or paid work invoked. `M` denotes [memory-system.md](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/memory-system.md); `U` denotes [user-interface-design.md](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface-design.md).

**2. Disposition of A1–A16**

| Finding | Round-2 disposition |
|---|---|
| A1 | **Resolved.** Page identity survives the original rename, move and one-page-to-book scenarios. Prefixes need the persistence rule in B3. |
| A2 | **Resolved with a residue:** blobs identify the bytes; candidate publication and historical book validation still need B4/B8. |
| A3 | **Not resolved.** After withdrawal, §7 still identifies the latest accepted blob as authoritative; a draft decision can also answer a question without an activation rule. B2. |
| A4 | **Not resolved.** Splitting an area still cannot reclassify an immutable question row: no such binding exists. B6. |
| A5 | **Not resolved.** Conclusion and applicability still compete for one standing; reopening and invalidation of an answer remain unspecified. B2. |
| A6 | **Not resolved.** Contributors or a commit author can still be recorded as the deciding human; principal updates also contradict ID uniqueness. B1/B7. |
| A7 | **Resolved with a residue:** duplicate detection and competing-successor refusal address the original examples; other competing transitions and their ordering remain undefined. B3. |
| A8 | **Resolved.** The required subject/dependency fields and explicit incompleteness of goal impact address the original missing-relationship contract. |
| A9 | **Resolved.** Root-installed adoption is explicitly unsupported. The two retained layouts still need B5’s publication contract. |
| A10 | **Resolved with a residue:** enforcement now precedes migration, but the complete paths through classification, publication, archival and review remain unspecified. B5. |
| A11 | **Resolved with a residue:** imported content is pinned and drift is exposed, but historical bound chapters still collide with live-path validation. B8. |
| A12 | **Resolved.** The separate page grammar and whole-file checking remove both original contradictions. The new binding grammar has separate defects under B3. |
| A13 | **Not resolved.** Promotion still relies on a goal field/writer that does not exist, and the counselor adapter cannot preserve the cited evidence as specified. B10/B11. |
| A14 | **Resolved.** Full confirmed intent, including unproved and deferred criteria, is now required. |
| A15 | **Not resolved.** Optional, brain-only contributor recording still permits an accepted design with insufficient information to establish examiner independence. B9. |
| A16 | **Resolved.** An observation can answer a factual question without a human ruling. |

**3. New material findings**

**B1. Writable content is still being used as proof of human authority.**

- **Attacked:** §§5.6, 7, 10; `M:206`, `M:339`, `M:384`.
- **Evidence:** A seat appends an acceptance ruling and a doctrine edit, naming a human contributor. Nothing specified distinguishes that from the human’s own act. Current ruling carriage checks appended row shape, not the deciding human: `metasystem/internal/landing/observe.go:1141`, `:1369`, `:1387`. The existing authority owner explicitly rejects parsed proof as fresh authority: `metasystem/internal/humanauthority/authority.go:107`, `:194`.
- **Failure:** The proposed exception can authorize itself. `Contributors:` identifies participants, and commit authorship identifies an author; neither identifies who approved the particular act. This contradicts the master’s request-origin boundary (`U:207`).
- **Smallest change:** Specify the trusted publication entry point. Before browser principals exist, use an explicitly supported terminal-authority path. Record the actual deciding principal, recorder and authority basis separately from accountable owner. Bind authorization to the exact actions and reviewed basis. A ruling row records the act; its presence cannot authenticate it.

**B2. “Latest binding wins” is not a state model.**

- **Attacked:** §§5.5, 7; `M:181–191`, `M:320–342`.
- **Evidence:** Accept D, withdraw D, then reassign D. Which standing wins? §7 still answers authority using the latest `accepts`. Separately, accept a design already under `records/designs/`: its bytes are accepted and its execution is concluded. A proposed decision containing `Affects: answers Q-…` has no stated requirement to become accepted before answering Q.
- **Failure:** Independent properties overwrite one another or require implementation-invented precedence. Refuting the fact that answered a question also has no defined effect on that answer.
- **Smallest change:** Separate content readiness, authoritative applicability, execution completion and ordinary metadata. Define activation and reversal rules for each. Keep append-only authority acts; allow current assignment, classification and question-resolution metadata to change under Git history. `Cites` and `Affects` should describe relationships, not execute a general state-changing language.

**B3. The binding language lacks the information needed for deterministic publication and repair.**

- **Attacked:** §§5.4–5.5, 10; `M:171`, `M:177`, `M:191`, `M:382`.
- **Evidence:** One ruling may contain several bindings, but their delimiter, boundary before prose and escaping are unspecified. Facts and pages also carry bindings, yet ordering is defined as when “their rulings landed.” Two seats can concurrently accept different already-landed blobs of one page without either superseding it. A previously admitted ruling later found malformed must remain immutable while the whole-register checker refuses it.
- **Failure:** Readers can fold different orders; incompatible decisions can silently overwrite; one malformed historical row can make repair impossible under the same gate.
- **Smallest change:** Give authority actions one versioned, delimited grammar, including escaping and full persisted IDs. Define publication order and within-transaction order. Carry the expected prior effective action; compare it with the actual publication tip, including conflicts within one candidate. Define an append-only correction/invalidation operation for malformed historical acts, with visible diagnostics and a lawful repair path.

**B4. Initial acceptance cannot pass the stated gate, and the checker’s visibility is undefined.**

- **Attacked:** §§5.5, 8, 10, 13; `M:191`, `M:350`, `M:382`, `M:419`.
- **Evidence:** Inception creates a page and its accepting ruling together. Before publication, that page’s blob is not reachable from the default branch, so the stated acceptance check refuses it. A ruling carried from the installation root can also name a host-root design absent from an installation-scoped candidate. Existing workspace snapshots deliberately exclude changes outside the nested workspace: `metasystem/internal/gittree/gittree.go:171`, `:250`.
- **Failure:** Either atomic inception fails, or an implementer exempts candidate blobs without defining which bytes are actually being published. A partial view can neither validate nor safely ignore an unseen target.
- **Smallest change:** Define checker inputs as the reviewed basis, actual publication tip and complete candidate. Accept exact revisions already published **or included in that validated candidate**. Resolve every action target across all supported homes. Missing visibility is a typed refusal, never a skipped check. Revalidate after composition and before publication.

**B5. “Two layouts” still conceals three distinct publication paths.**

- **Attacked:** §§10–12, slice 2; `M:384`, `M:404`, `M:410–413`.
- **Evidence:** Vendored adoption’s outer app paths are app-owned and classified `Outside`: `metasystem/internal/stateroot/owner.go:117`; `metasystem/internal/pathclass/pathclass.go:223`. Register carriage refuses `Outside`: `metasystem/internal/landing/observe.go:1033`, `:1096`. The self-hosted checkout-root design is not a `Goal-Plan` path: `metasystem/internal/goal/branch/range.go:182`, `:185`, `:225`. Nested unowned plans remain frozen on revision: `metasystem/internal/landing/observe.go:1167`, `:1182`, `:1230`.
- **Failure:** A resolver can list all three homes while publication still treats them differently. The checkout-root second home has no demonstrated complete memory-record write/review path today. Keeping the oracle unchanged does not establish that path.
- **Smallest change:** Specify a path matrix for adopted app material, self-hosted installation material and self-hosted host-root material: candidate scope, classification, commit kind, authority route and read admission. Cover creation, revision and the deletion-plus-addition archive move. Preserve archived bytes: §10’s permission to “revise” `records/designs/` must not override §6.3’s immutability.

**B6. Area evolution is promised but cannot be encoded.**

- **Attacked:** §§5.5–5.6, 9, 14; `M:189`, `M:204`, `M:360`, `M:427`.
- **Evidence:** Split billing into payments and invoicing, moving an existing question. Its row is immutable; no binding updates its areas. Rename billing→commerce→revenue: neither alias-chain resolution nor cycle/reuse rejection is defined. `Areas: none` appears in the examples although every record must name declared slugs. An area without an owner and nested areas have no grammar.
- **Failure:** The enterprise reorganisation fixture cannot be written without inventing schema. Root-level or initially unassigned work can fail checking or disappear from navigation.
- **Smallest change:** Keep area identity stable when its display name changes. Define aliases as acyclic, non-reusable alternate names with one owner of the mapping. Explicitly allow project-wide/unassigned records and visibly unowned areas. Make current row membership editable metadata. Either define parent relationships or make the initial area list flat.

**B7. The principals register is neither a valid register nor an authority directory.**

- **Attacked:** §§5.3, 5.6, 6.8; `M:141`, `M:145`, `M:206`, `M:316`.
- **Evidence:** Changing a display name requires another row with the same handle, but `project check` requires unique IDs. “Gone” has no field or action. Team membership is buried in `notes`, with no join/leave semantics. Seeding from two seats whose humans are both named Alex does not determine two stable handles.
- **Failure:** Ordinary maintenance either fails checking or requires special duplicate-ID and prose-parsing rules. Membership cannot safely inform authority.
- **Smallest change:** Use one current row per stable principal, with explicit name and active/departed state; Git retains its history. Seat configuration references a deliberately assigned handle rather than deriving it from a display name. Keep team handles for ownership; defer membership semantics to the authority owner unless a concrete structured membership contract is supplied now.

**B8. Bound chapters still confuse historical resolution with live-file existence.**

- **Attacked:** §§5.2, 5.4, 10; `M:117–121`, `M:171`, `M:382`.
- **Evidence:** Accept an index binding `doc:docs/architecture.md@blob:H`, then move that source document. Historical resolution is promised after movement/deletion, but index validation requires its path to exist. The accepted blob survives while the book fails checking.
- **Failure:** A harmless documentation move invalidates preserved authority.
- **Smallest change:** Validate pinned references against their historical source, not current path existence. Display accepted content and current-source availability separately. State that a headless bound chapter’s acceptance belongs to the accepting index binding; it has no independent page acceptance. Typed pages must use the appropriate ID relationship, with the same-kind rule enforced. Out-of-checkout documents are explicitly unsupported under §5.2; retain that boundary and test escaped paths/symlinks rather than silently importing them.

**B9. Contributor provenance still cannot establish independence.**

- **Attacked:** §§5.1, 7, 16; `M:100`, `M:344`, `M:444`.
- **Evidence:** An agent shapes r1 in an editor; another agent revises r2 through the brain and records only itself. The field is optional and no inheritance rule preserves r1’s participation. Both revisions can be accepted. The master prohibits an agent that shaped the design from examining the result (`U:80`).
- **Failure:** A fresh assignment owner cannot distinguish an independent examiner from a previous contributor. Treating unavailable transcripts honestly does not fill the missing shared provenance.
- **Smallest change:** Preserve the effective contributor set for each adopted revision, including inherited contributions and stable agent/session identities. Missing provenance must yield “independence not established,” not an empty contributor set. Apply this to all authoring paths; do not infer participation or approval from a human name.

**B10. Citation is being mistaken for promotion, and the promised goal writer does not exist.**

- **Attacked:** §§5.3, 6.7; `M:145`, `M:312`.
- **Evidence:** A draft design cites P to discuss why it should be rejected; the resolver nevertheless marks P taken up. Current `goal open --origin human` records origin and ordinary goal fields, not a proposal ID: `metasystem/cmd/metasystem/goalsync_mutations.go:1014`, `:1760`; `metasystem/internal/goal/verbs.go:816`, `:848`.
- **Failure:** Discussion becomes a human promotion without a human act, and deleting the draft still lacks the promised durable provenance.
- **Smallest change:** Make promotion an explicit human-authorized relationship to the successor. Keep that relationship on the proposal/ruling initially; a citation alone has no promotion effect. Do not promise automatic goal-side provenance until its schema, writer and compatible rollout are included.

**B11. The evidence adapter cannot be lossless or guarantee remote availability.**

- **Attacked:** §§5.4, 6.6, 16; `M:167`, `M:308`, `M:444`.
- **Evidence:** Counselor emits `job-record` citations to `artifacts/agents/jobs/<id>.json`, with finding identity in `detail`: `metasystem/internal/counselor/register.go:205`. Goal citations also retain an operation ID in `detail` at `:220`. The proposed scalar grammar has neither a `job-record` spelling nor equivalent selectors. Counselor’s validator accepts nonempty targets; it does not prove retention: `metasystem/internal/counselor/sources.go:211–218`.
- **Failure:** Translation loses which evidence was cited, or invents a landed blob for an artifact. A second machine can lack more than private sittings despite the handoff promise.
- **Smallest change:** Preserve the counselor citation structure through an explicit adapter, including detail and availability. A job locator is not itself proof of a retained observation. Selected shared facts requiring portable evidence must bind retained bytes; legacy unavailable evidence remains visibly unavailable.

**B12. The fixture and slice plan cannot serve as the implementation contract.**

- **Attacked:** §§16–17; `M:448`, `M:454–457`.
- **Evidence:** Slice 1 promises the competing-publication refusal fixture while `check --base` and landing integration arrive in slice 2. Every fixture supposedly passes checking “before and after,” including rejected candidates. Slice 2 combines publication semantics, authority, three path arrangements, archival and review admission under a four-hour estimate. Its packages are protected from tier-1 shortcuts: `metasystem/scripts/agents/path-classes.txt:73`, `:76–77`.
- **Failure:** Builders must weaken fixtures, simulate the boundary being proved or exceed the stated slice. A temporary-repository test also cannot establish the newcomer’s click counts.
- **Smallest change:** Separate parser/history tests from publication tests and human navigation walkthroughs. Name expected refusals and assert that rejected publication leaves the accepted state unchanged. Split slice 2 into coherent publication/authority and layout/review work, each with its integration proof; flag any over-four-hour unit honestly. R-15 requires that distinction: `metasystem/memory/rulings.md:40`. Separate mechanical migration from human intent harvesting and historical classification.

**Identity and raw-file attacks**

ULIDs are acceptable opaque identities. Their practical safety comes from allocation **and duplicate refusal**, not an absolute “no collision” promise. Two files claiming one ID already violate §10; two `Id:` lines already violate §5.1. Make these named fixtures, together with changing an existing ID and copying a page. Never silently remint an existing identity to repair a collision.

Prefixes are lookup conveniences. They can become ambiguous as the repository grows; persisted references and stable routes must contain full IDs, as required by B3. Display titles beside them.

A raw page reveals its content, identity and author’s readiness claim. It cannot establish current authority, withdrawal or reassignment without consulting other records. Rename `State:` to something such as `Readiness:`. An optional `Accepted:` pointer cannot solve this: a later withdrawal leaves a perfectly matching historical acceptance on the page.

**Answers to the seven open questions**

| # | Answer |
|---|---|
| 1 | **Keep ULIDs.** Full IDs in durable records/routes; prefixes only at lookup boundaries; readable titles in results. |
| 2 | **Keep both human and held-goal paths, subject to B1/B5.** A co-carried ruling is evidence of the act, not the credential authorizing it. |
| 3 | **One rulings file initially.** Centralize lookup; measure before physical sharding. Union merge does not settle semantic conflicts. |
| 4 | **Remove `Accepted:`.** It becomes stale after withdrawal and cannot tell a raw-file reader what currently governs. Let `project show` report the effective ruling. |
| 5 | **Tracked principal directory, with Git history.** Configuration selects the seat’s handle; the directory describes that handle. Do not put authority-bearing membership in free-text notes. |
| 6 | **Read both existing schemas.** They are concretely different: `metasystem/scripts/adopt.sh:310` versus `metasystem/memory/known-issues.md:7`. |
| 7 | **Date in the filename as convention only.** No identity or ordering semantics should depend on it. |

**4. Simplicity under R-11**

Cut:

- Universal immutable metadata and its generic fold. Preserve immutable authority acts and historical bytes.
- State-changing expressions hidden inside `Cites`, `Affects` and `about`.
- `Accepted:` on pages.
- Team membership in prose.
- Mandatory single-row storage for arbitrarily long proposals; permit a short register entry referencing the retained body.
- Mandatory retrospective typing of every old design before the useful live surface works.

The four-tree layout can pass R-11. The complete model currently fails it. A newcomer must learn why an accepted document says `draft`, why an answered question has no answer/status cell, why current ownership requires replay, why a person appears repeatedly under an allegedly unique ID, and why accepting an index differs from accepting an ordinary chapter. That is operational knowledge required to avoid mistakes, not incidental implementation detail.

R-11 makes simplicity equal to separation, not subordinate to it: `metasystem/memory/rulings.md:36`. A growing event history is acceptable behind a current view; making humans hand-author and mentally replay that history for ordinary maintenance is the avoidable part.

**5. Incorrect code/source claims**

1. **§6.7: `goal open --origin human` records the proposal ID.** It does not. The flag records creation provenance; the open transaction has no proposal-reference input: `metasystem/cmd/metasystem/goalsync_mutations.go:1014`, `:1760`; `metasystem/internal/goal/verbs.go:816`, `:848`.

2. **§6.6: counselor citations map one-to-one onto the proposed grammar.** The claim is unsupported by the shapes. `job-record`, finding selectors and operation selectors need explicit preservation: `metasystem/internal/counselor/sources.go:66`, `:245`; `metasystem/internal/counselor/register.go:205`, `:220`.

3. **§§5.3/11: existing register merge configuration establishes the proposed immutability law.** `.gitattributes` configures particular merge drivers; it does not enforce append-only semantics. Ordinary existing `memory/` records can change under a held goal: `metasystem/internal/landing/observe.go:1191`. Special append-only handling is separately implemented at `:1141`, `:1146`.

4. **§11: intent, doctrine and decisions stay `behavior`, “as all of docs/ is today,” across the supported layouts.** `install:docs/` is behavior (`metasystem/scripts/agents/path-classes.txt:17`); adopted app-owned documentation is `Outside` (`metasystem/internal/pathclass/pathclass.go:223`). The distinction matters.

5. **§10: the current carriage law does not admit the proposed pages at all.** Too broad. New unowned plan files and new record files have creation paths; existing-plan revision and archival are the missing parts: `metasystem/internal/landing/observe.go:1182–1188`, `:1198–1208`.

6. **§18.2: gate 2 provides browser human acts.** Gate 2 supplies command foundations; authenticated browser access arrives at gate 3: `plans/user-interface-design.md:810–812`.

The corrected goal-parser, known-issue merge-driver, counselor-ID and design-read-prefix claims match the inspected source.

Proposed receipt, not written: `type=review | skills=design-critique | outcome=redesign | verify=source-read-only | note=memory-system revision 4; 12 material findings`.

**6. The way forward**

I agree to retain immutable record identity, exact accepted versions, separate current proposals, explicit areas, typed relationships, bound existing documentation, one resolver, the two-layout scope, and enforcement before migration.

The **single mechanism requiring redesign is universal immutable rows with latest-binding-wins standing**. Replace it with ordinary versioned current records plus a narrowly defined append-only authority history. This retains historical evidence without turning every rename, reassignment and classification change into a miniature ruling.

Revision 5 must:

1. Separate readiness, authoritative applicability, execution completion and mutable metadata; give each transition explicit effects and preconditions.
2. Define the trusted human publication path and record actual decision-maker, recorder and authority basis.
3. Specify authority-action grammar, full persisted references, deterministic ordering, expected-basis conflicts and historical correction.
4. Make acceptance and validation operate over the complete publication candidate, including atomic initial acceptance.
5. Define and prove the three physical path arrangements within the two supported layouts, including archival and review.
6. Make area and principal maintenance representable without prose parsing or duplicate-identity exceptions.
7. Resolve historical chapter bindings independently of live-path existence.
8. Preserve contributor provenance sufficiently to establish—or explicitly decline to establish—examiner independence.
9. Make promotion explicit and evidence adapters lossless about both selectors and availability.
10. Replace the blanket evolution assertion with named positive/refusal fixtures; split and re-estimate the delivery work around those boundaries.

With those contracts adopted, I would give **“sound after the listed changes”** and use implementation fixtures and code review as the next arbiter.

Decisions for Wido:

- **May ordinary register metadata change under Git history while authority acts and observation claims remain immutable?** Recommendation: **yes**; this is the simplifying replacement.
- **Should authoritative edits before browser access use a verified terminal path?** Recommendation: **yes**; recorded human names and ruling text must not substitute for caller authority.
- **Is read-only treatment of root-installed adopters acceptable until their separate prerequisite lands?** Recommendation: **yes**, explicitly as a supported-scope decision.

