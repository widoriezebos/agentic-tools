# Astra's critique of the memory-system design, revision 3

Produced 2026-09-22 by Codex on `gpt-6-astra`, read-only, on Wido's order, with his directive that many humans will use the interface and the MetaSystem to build large enterprise systems together, that intent will evolve over time and the system must facilitate that by design, and that the structure of the memory types must be robust against change over time, scale, be intuitive for humans, and be easy to use for the interface and the agents. The design's dispositions are in [memory-system-r6.md](memory-system-r6.md) (revision 6, section 19; revision 4 was the reply to this round). The critique follows verbatim.

---

**redesign — 16 material findings: identity, acceptance and classification are coupled to mutable files and organizational structure, so the design fails the required evolution, scale, intuitiveness and resolver guarantees.**

Source review at `b712f2f03`; failure scenarios below are inferred from the proposed contracts. No files changed, tests run or paid work invoked. `M` denotes [plans/memory-system.md](designs/memory-system.md).

## 2. Material findings

**A1. Evolution: identity is a path despite the claim that it is not.**
- **Attacked:** §§5.1, 5.4, 8, 12; `M:84`, `M:305`, `M:370`.
- **Evidence:** Rename `billing` to `commerce`: every nested page ID changes. Turning `billing.md` into `billing/index.md` also changes its ID under §5.1, although §5.5 treats them as interchangeable. Immutable ruling rows retain the old references. The additional checkout-root design home has no reference qualifier or explicit corresponding archive home.
- **Failure:** Ordinary restructuring breaks citations, accepted-version identities and supposedly stable UI addresses.
- **Smallest change:** Give records immutable IDs independent of location; keep titles, paths and area membership mutable. Define the project namespace and both live/archive roots explicitly. Preserve old references through recorded aliases or successor records, without rewriting historical citations.

**A2. Evolution: `@r<n>` does not identify retrievable historical bytes.**
- **Attacked:** §§5.4, 8, 10; `M:159`, `M:305`, `M:307`.
- **Evidence:** After a page advances from r3 to r4, hashing the current file cannot answer `show …@r3`. Two branches can contain different r3 blobs. The design specifies neither historical lookup nor which ancestry defines the canonical revision. `criterion:<id>` has no version at all.
- **Failure:** A fresh worker cannot reliably recover what an earlier approval, design or criterion meant.
- **Smallest change:** Specify retained immutable source coordinates and historical resolution—such as repository identity plus Git commit/path and blob verification. Treat revision numbers as labels unless their unique mapping is enforced. Historical references must resolve against their recorded snapshot, including after deletion. An integrity trailer would not fix this.

**A3. Acceptance can bless the wrong record and leave unaccepted changes looking authoritative.**
- **Attacked:** §§5.3, 6.4, 7, 10; `M:141`, `M:272`, `M:299`, `M:338`.
- **Evidence:** A new design can say `State: accepted` and cite an unrelated existing ruling; the stated check passes. A substantive r3 rewrite remains accepted using r2's ruling. A later reversal need not change either result because ruling supersession has no required structured relationship.
- **Failure:** "Current" and "accepted" cease to answer which content has authority.
- **Smallest change:** Require acceptance records to bind the action, exact subject/version and recorded actor; validate the join. Preserve the last accepted version while exposing newer content as a proposed revision. Represent withdrawal and supersession explicitly. Historical, unstructured rulings must not manufacture new acceptance.

**A4. Intuitiveness: reading order, domain structure, team ownership and storage are different things.**
- **Attacked:** §§5.2, 5.3, 5.5; `M:111`, `M:139`, `M:163`.
- **Evidence:** A security question affecting billing and authentication can inhabit only one shard; an authentication newcomer consequently misses it. A doctrine chapter called "Data" need not represent an organizational area. When a one-area project splits, existing rows cannot move, yet their area is defined exclusively by their old shard.
- **Failure:** Users must learn filing history and arbitrary placement decisions to find their work. Splits and reorganizations require either invalid layouts or permanent obsolete areas.
- **Smallest change:** Let intent records declare stable areas, without making every chapter an area. Give records explicit, potentially multiple area associations. Derive navigation from those associations; treat reading order and physical sharding separately.

**A5. The state model cannot represent retirement and reversal consistently.**
- **Attacked:** §§4, 6.3, 7, 11; `M:66`, `M:266`, `M:294`, `M:360`.
- **Evidence:** Intent/doctrine chapters have no retired or superseded state. A concluded design must remain append-only, yet superseding it requires changing its predecessor state. Withdrawn and superseded designs have no specified archival transition. The existing history law permits only the goal engine's named exceptions: `metasystem/plans/memory-architecture-design.md:46`.
- **Failure:** Years-later reversals either rewrite preserved history, leave obsolete intent current, or keep dead plans indefinitely.
- **Smallest change:** Define retirement, replacement, reopening and archival transitions for every kind. Derive an archived record's later applicability from successor/withdrawal records instead of modifying its historical bytes. Separate execution completion from whether a decision still applies.

**A6. Ownership cannot survive a team reorganization.**
- **Attacked:** §§5.3, 5.5, 7, question 13; `M:119`, `M:176`, `M:432`.
- **Evidence:** Two humans named Alex are indistinguishable. Reassigning a question is forbidden because its owner cell is immutable. Replacing an area owner leaves no specified authority history. The existing ruling `owner` means **accountable owner**, not necessarily the person who spoke: `metasystem/memory/rulings.md:5`.
- **Failure:** The records cannot distinguish historical decision authority, current responsibility and display names.
- **Smallest change:** Define stable human/team identifiers now, while allowing authentication implementation later. Record decision-maker and authority basis separately from current assignment; permit dated reassignment. Never judge an old acceptance against today's area owner.

**A7. Scale: neither ID allocation nor concurrent decisions are safe as specified.**
- **Attacked:** §§5.3, 6.2, 9, 10; `M:119`, `M:258`, `M:322`.
- **Evidence:** Two worktrees using the same machine ID and visible maximum mint the same `Q-42-m1`, even in different shards. Two seats can choose the same date/slug. Separately, two accepted decisions can both supersede the same predecessor; their separate files and identical predecessor-state edit merge without a textual conflict.
- **Failure:** Legitimate concurrent work collides, or two incompatible successors become current while every stated check passes.
- **Smallest change:** Use allocation that distinguishes concurrent writers, preserve existing IDs, and check duplicates globally. Define conflicts for acceptance, supersession and status transitions. Validate the composed candidate against the actual publication base and refuse stale competing transitions; "keep both lines" is insufficient.

**A8. Ease of use: the records do not contain the relationships needed for the advertised queries.**
- **Attacked:** §§5.5, 6.3, 6.6, 14, 16; `M:178`, `M:264`, `M:282`, `M:393`.
- **Evidence:** `Cites`, `Governs` and decision `Affects` are optional. Facts have no subject field; proposal `source` records provenance, not necessarily what the proposal concerns. A design may mention billing only in prose. Existing goal labels and arcs carry no typed intent-reference contract: `metasystem/internal/goal/file.go:39`.
- **Failure:** Reverse citations cannot answer "what would change?" completely; area lists miss relevant material; facts/proposals cannot reliably appear beside their subjects. A search index cannot reconstruct missing authoritative relationships.
- **Smallest change:** Define the minimum typed subject and dependency relationships required by each promised query, distinguishing dependency from incidental citation. Expose incomplete linkage explicitly. Until `DefinitionRefs` exists, label goal impact as incomplete rather than substituting labels/arcs.

**A9. The three-layout claim rests on a broken ownership premise.**
- **Attacked:** §§11–13; `M:356`, `M:360`, `M:367`.
- **Evidence:** In a root-installed adoption, the oracle marks all `memory/`, `plans/` and `records/` as generic shipped material: `metasystem/internal/stateroot/owner.go:106`, `:143`. App-owned paths are classified `Outside` before installation path-class rows apply: `metasystem/internal/pathclass/pathclass.go:220`. Root installation also retains the interleaving R-9 explicitly rejects: `metasystem/memory/rulings.md:34`.
- **Failure:** Excluding three documentation directories does not establish ownership or equivalent enforcement across the claimed layouts.
- **Smallest change:** Correct ownership and state-path classification as an explicit prerequisite, with app-owned defaults and precise generic exceptions. Either provide visible separation for root installation or narrow the supported-layout claim. The oracle cannot remain unchanged.

**A10. The proposed lifecycle has no complete lawful write and review path.**
- **Attacked:** §§10–13, 17; `M:340`, `M:372`, `M:413`.
- **Evidence:** Existing unowned plans are frozen, and inferred goal ownership only examines files directly under `plans/`: `metasystem/internal/landing/observe.go:1167`, `:1230`. Thus nested `plans/designs/` pages cannot use that carriage route for ordinary revision. Existing `record`-class files outside the recognized trees hit the refusal at `:1214`. Design read admission rejects checkout-root/adopted design paths lacking `metasystem/`: `metasystem/internal/dispatch/read_subject_compute.go:198`.
- **Failure:** Parsing and listing records works before the required editing, acceptance, archival and review operations work. Naming the prefix defect does not remove its effect on the proposed locations.
- **Smallest change:** Specify the layout-aware carriage and review contracts before migration. Cover creation, revision, acceptance and archival in all three layouts, including host-root designs. Move the necessary reader/policy changes ahead of slice 3's moves.

**A11. Referenced chapters evade the book's version and acceptance guarantees.**
- **Attacked:** §§5.2, 6.1, 6.2; `M:111`, `M:113`, `M:212`, `M:260`.
- **Evidence:** A `doc:` chapter has neither the required typed head nor its own acceptance. Editing its target can change the displayed accepted doctrine without changing any book revision. The self-hosted examples also use `docs/…` paths despite §5.4 defining checkout-relative paths; the cited files live under `metasystem/docs/`.
- **Failure:** Retrofit books have materially weaker semantics than newly authored books, while presenting the same states.
- **Smallest change:** Give each imported chapter a typed identity and explicit immutable source binding, with acceptance scoped to that binding—or display it explicitly as unaccepted external material. Preserve the canonical source document. Resolve all source paths in one declared coordinate system.

**A12. The parser and checker contracts are internally unimplementable.**
- **Attacked:** §§5.1–5.3, 9, 10; `M:84`, `M:321`, `M:338`.
- **Evidence:** The goal parser rejects arbitrary prose and unknown fields and requires integrity/history: `metasystem/internal/goal/file.go:536`, `:578`, `:611`, `:1523`. Meanwhile a checker reading only the first 4 KiB cannot validate chapter lists or criteria in the body, and can miss a long enterprise `Cites` field.
- **Failure:** Reusing the actual goal parser rejects the proposed pages; obeying the advertised read bound makes the checker incomplete.
- **Smallest change:** Define a small new grammar borrowing the familiar header syntax, including header termination, escaping, duplicate fields and compatibility rules. Read every authoritative field and required body section. Treat 4 KiB as an optimization hint, never a correctness boundary.

**A13. The reference grammar cannot express its own evidence and promotion contracts.**
- **Attacked:** §§5.3–5.4, 6.6–6.7, 10; `M:134`, `M:137`, `M:156`, `M:338`.
- **Evidence:** Facts may anchor to commands, code, jobs and observations, but `doc:` accepts Markdown and the grammar has no code/observation locator. Whitespace separates references but also appears in commands and legitimate paths. A proposal can point to a goal draft that lawful promotion deletes: `metasystem/docs/backlog-mechanism.md:121`.
- **Failure:** Implementers must invent incompatible locators; historical promotions become broken references; a command can masquerade as evidence of a past observation.
- **Smallest change:** Specify typed source locators with immutable revision and optional selector, plus escaping. Distinguish a verification recipe from a retained observation. Preserve promotion provenance through durable IDs or pinned historical paths, and validate historical references against their historical source.

**A14. The kit migration mistakes already-proven requirements for complete intent.**
- **Attacked:** §§6.1, 13, slice 3; `M:212`, `M:380`, `M:413`.
- **Evidence:** The cited harvest explicitly admits only commitments with named existing proof into the draft: `plans/covenant-harvest-survey.md:52`. Seventeen further gaps are excluded, including revisable intent itself: `:101`, `:123`. Inception requires every confirmed criterion to remain represented, including explicitly deferred gaps: `metasystem/skills/inception/SKILL.md:132`.
- **Failure:** The initial memory omits desired outcomes precisely because the software does not yet achieve them.
- **Smallest change:** Harvest the full confirmed intent and retain unproved/deferred criteria with their status. Use the covenant draft as evidence of current coverage, not the boundary of what the project intends.

**A15. A local sitting ID cannot provide portable provenance or examiner independence.**
- **Attacked:** §§5.1, 5.4, 7, 16; `M:97`, `M:157`, `M:301`.
- **Evidence:** A design shaped in three sittings has one optional `Sitting` value and no contributor set. On another machine that sitting is unavailable by construction; raw history is server-local and need not replicate: `plans/user-interface-design.md:376`, `:378`.
- **Failure:** The resolver cannot both require every reference to resolve and pass the remote handoff test. The field cannot establish who participated and is therefore ineligible to examine.
- **Smallest change:** Preserve shared, transcript-free provenance for relevant contributors/sessions per revision. Treat unavailable private sitting material as a named availability state, not a missing shared record. Give the assignment owner enough durable identity information to enforce independence.

**A16. Every answered question requiring a human ruling creates an unnecessary enterprise bottleneck.**
- **Attacked:** §§4, 5.3, 6.5, 7; `M:71`, `M:137`, `M:293`.
- **Evidence:** "Does the production database support this index operation?" is answered by an observation. Under this schema it remains open until a human manufactures a ruling.
- **Failure:** Thousands of factual investigations consume decision authority, blur observation with choice and make the question register harder to operate.
- **Smallest change:** Let a question's resolution cite evidence, a decision or a ruling as appropriate. Require human authority when the answer settles intent or a reserved choice, not merely when evidence resolves uncertainty.

## 3. Non-material observations
None additional; factual corrections are listed in section 6.

## 4. Answers to the thirteen open questions

| # | Answer and reasoning |
|---|---|
| 1 | **Keep evidence-table criterion text canonical initially, but remove handwritten copies.** Intent can reference criterion IDs and the interface can render their text; A2 must preserve historical versions. |
| 2 | **Disagree.** Keep the accepted version authoritative and expose the newer version as a proposed revision; a typo need not revoke the previously accepted content. |
| 3 | **Disagree with soft checking.** The existing context cell can carry a strict, versioned subject/action binding for new rulings without adding a seventh column; malformed acceptance must fail. |
| 4 | **Disagree: retain the governed change path for all three initially.** A binding architecture decision can change obligations as substantially as doctrine, and a `record` classification must not bypass that boundary. |
| 5 | **Keep Wido's checkout-root placement, conditionally.** Define its namespace, archive destination and supported review/write paths rather than adding only a second search location. |
| 6 | **Yes, selected facts can exist before sittings.** Selection limits duplication; durable anchors and explicit subject relationships are still required. |
| 7 | **Defer semantic migration, but support both existing schemas explicitly.** A new resolver must not silently discard legitimate known-issue records. |
| 8 | **Yes for human design acceptance, subject to A3.** Record the human act once; one act may accept several explicitly identified versions. |
| 9 | **Per chapter, without requiring one separate human act or ruling row per chapter.** A single confirmation may enumerate independently pinned chapters while preserving their separate acceptance identities. |
| 10 | **Yes as the new authoring default; do not assume adopted projects have no existing file.** Retrofitting must retain their canonical documents and bind them explicitly. |
| 11 | **Centralize ruling lookup now; defer physical sharding until measured.** Disagree with prescribing area shards as the inevitable solution, because authority history should not depend on organizational boundaries. |
| 12 | **Disagree.** Declare areas explicitly in intent records and derive their map; reading-order chapters need not all be areas, and this requires no second handwritten catalogue. |
| 13 | **Disagree with deferring identity shape.** Stable principal/team IDs are needed before durable enterprise records accumulate; sign-in and richer authorization can arrive later. |

## 5. Cuts under R-11, and missing necessities

R-11 requires simplicity alongside separation: `metasystem/memory/rulings.md:36`.

**Cut:**
- Mandatory mirroring of one chapter hierarchy across every storage kind.
- Handwritten criterion-text duplication and its synchronization gate.
- A second per-page History ledger merely because goals have one; require it only for a named consumer that Git history and decision records cannot serve.
- Human rulings for purely factual answers.
- Forced promotion because a proposal exceeds one paragraph.
- The blanket warning that every concluded design needs a citation from `docs/`; many completed changes require no new standing explanation.
- Predetermined area/year sharding without demonstrated pressure.

**Missing:**
- Stable record, area and principal identities independent of names and locations.
- Retained, resolvable historical versions and exact acceptance bindings.
- Explicit retirement, reassignment, replacement and reversal semantics.
- Typed relationships sufficient for the promised queries, with honest incomplete results.
- Concurrent publication rules and migration compatibility before record relocation.
- Fixtures covering rename, split, merge, imported chapters, competing acceptances, organizational change and remote handoff across all supported layouts.

## 6. Incorrect code and source claims

1. **"Exactly the goal file's grammar"; arbitrary prose and optional History (§5.1).** False. Integrity is required at `metasystem/internal/goal/file.go:536`; arbitrary body lines are rejected at `:583`; History is required at `:611`; unknown fields are rejected at `:1523`.
2. **The catalogue claims open questions live in the two nearby registers (§1).** False. It explicitly marks that subsection `alwaysNotRecorded` at `metasystem/internal/ui/project/catalogue.go:73`, labels the files "Nearest living registers" at `:121`, and preserves the unrecorded state at `metasystem/internal/ui/project/project.go:196`.
3. **Counselor register IDs are ULIDs (§2).** False. Writers construct IDs such as `ar-human-carried-<finding>`, `ar-<job>-<finding>` and `mc-<goal>-<opid>`: `metasystem/internal/counselor/register.go:189`, `:215`, `:222`.
4. **Fact anchors use the counselor's existing spelling (§5.3).** False as a compatibility claim. Counselor citations are structured `{kind,target,detail}` values at `metasystem/internal/counselor/sources.go:66`; accepted kinds are `commit`, `goal`, `ruling`, `job`, `record`, `job-record` at `:245`. The proposed `doc:`, `criterion:` and command forms need an explicit adapter.
5. **Known issues are enforced by normal merging (§2).** False in the checked-out configuration. `metasystem/.gitattributes:5` selects `merge=union`; read-only `git check-attr` confirmed it. The contradictory normal-merge prose is at `metasystem/plans/README.md:9`.
6. **`Goal-Plan` allows `records/` generally outside `records/reads/` (§2).** Too broad. `metasystem/records/misc/` is `ClassReadProse` at `metasystem/internal/goal/branch/range.go:179`, which `Goal-Plan` does not accept at `:225`. Counselor and concluded-goal paths are excluded through `metasystem/internal/landing/registers.go:40`. The classifier also hard-codes the `metasystem/` prefix.
7. **Goal drafts have a `Status:` line grammar sourced to `backlog-mechanism.md:119` (§2).** The cited source says "free-form files, no grammar": `metasystem/docs/backlog-mechanism.md:121`. "Draft" names their status; it does not establish a required field.

Proposed review receipt, not written: `type=review | skills=design-critique | outcome=redesign | verify=source-read-only | note=memory-system revision 3; 16 material findings`.
