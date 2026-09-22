# Astra's critique of the memory-system design, revision 5 (round 3)

Produced 2026-09-22 by Codex on `gpt-6-astra`, read-only, on Wido's order that the author and Astra converge. Rounds 1 and 2 are in [memory-system-critique-astra.md](memory-system-critique-astra.md) and [memory-system-critique-astra-r2.md](memory-system-critique-astra-r2.md); the design's dispositions are in [memory-system-r6.md](memory-system-r6.md), revision 6, section 19. The critique follows verbatim.

---

**sound after the listed changes — eleven material corrections remain, but none requires replacing versioned current records plus a narrow authority history.**

Source read at `b712f2f0339765b158b4deac5e7457a270d083b5`; failure scenarios below are inferred. No files changed, tests run or paid work invoked. `M` denotes [memory-system.md](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/memory-system.md); `U` denotes [user-interface-design.md](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface-design.md).

**2. Round 2’s ten required changes**

1. **Made with a residue:** properties are separated; invalidation, reopening and validation of an existing question resolution still conflict — C6, C8.
2. **Made with a residue:** a trusted entry point is named, but its proof does not survive into the promised publication check, and `--by` is not an authenticated identity binding — C1.
3. **Made with a residue:** grammar, ordering and expected basis exist; correction semantics and recovery from duplicated identities remain incomplete — C6, C7.
4. **Made with a residue:** candidate-visible initial acceptance is specified; atomic checked publication remains unspecified across the actual paths — C1–C4.
5. **Made with a residue:** all three arrangements are named; P1’s admission hook, P3’s root handling and P2’s supposed gate do not establish the promised paths — C2–C4.
6. **Made:** current area membership and principal maintenance are representable without aliases, prose membership or duplicate principal rows.
7. **Made with a residue:** live-path dependence is removed; historical path-to-blob provenance is not checked — C9.
8. **Made with a residue:** missing provenance fails conservatively, but a false contributor attribution becomes permanent — C11.
9. **Made with a residue:** promotion is explicit; its human-origin rule needs C1, and the citation adapter still lacks a lossless encoding — C10.
10. **Made with a residue:** fixtures and slices are separated; publication fixtures must exercise the real publisher, and the remote walkthrough still contradicts declared evidence availability — C1–C4, C10.

**3. B1–B12**

| Finding | Disposition |
|---|---|
| B1 | **Not resolved:** a later check receives identical trees whether `terminal:<opid>` was written by the verb or copied by a seat — C1. |
| B2 | **Resolved with a residue:** corrections lack deterministic effects, and withdrawing an answer can make its unchanged question fail checking — C6, C8. |
| B3 | **Resolved with a residue:** invalidation scope, correction tokens and malformed-history recovery remain unspecified — C6. |
| B4 | **Resolved with a residue:** candidate objects are allowed, but checking and publishing that exact composed candidate are not yet one contract — C1–C4. |
| B5 | **Not resolved:** a P1 doctrine edit bypasses the record-only loop; running P3 at the app root supplies the wrong installation context — C2, C3. |
| B6 | **Resolved:** stable identities, editable memberships, parents and unowned areas suffice; archived memberships remain historical. |
| B7 | **Resolved:** one current row per assigned handle; authority-source binding is the separate C1 residue. |
| B8 | **Resolved with a residue:** an arbitrary landed blob can still masquerade as the historical contents of the named document — C9. |
| B9 | **Resolved with a residue:** false contributor entries cannot be corrected — C11. |
| B10 | **Resolved with a residue:** citation no longer promotes, but the human-only disposition edit lacks an admission boundary — C1. |
| B11 | **Resolved with a residue:** selectors, unpinned legacy records and unavailable `job-record:` references still lack a consistent contract — C10. |
| B12 | **Resolved with a residue:** real publication coverage and the second-checkout expectation need correction; re-estimate those named integration tasks rather than assuming four hours — C1–C4, C10. |

**4. Material findings**

**C1. An in-process proof does not authenticate a subsequently published row.**

- **Attacked:** §§3, 7, 10; `M:62`, `M:316`, `M:344–346`.
- **Evidence:** `project rule` proves ancestry, appends a row and exits; another process later checks three trees. A seat can produce identical bytes. Parsed proof deliberately cannot authorize a new mutation: [authority.go:194](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/humanauthority/authority.go:194). Nor does ancestry prove the supplied handle: the goal helper accepts an explicit `--by` without comparing it with enrollment’s human name — [goalsync_mutations.go:1379](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/goalsync_mutations.go:1379).
- **Failure:** “The check refuses any other writer” is unimplementable from the specified inputs. The recorded deciding principal can also be an asserted name.
- **Smallest change:** Consume the live proof at an owner-controlled publication boundary, binding the verified seat/enrollment handle, exact acts, reviewed basis and candidate; revalidate against the actual tip before atomic publication. A later structural check trusts previously admitted history, not a textual basis as fresh authority. Route metadata mutations advertised as human-only through the same caller boundary.

**C2. P1’s proposed carriage rule misses documentation and grants unrelated changes by adjacency.**

- **Attacked:** §10 P1; `M:352`.
- **Evidence:** `install:docs/` is `behavior` — [path-classes.txt:17](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/scripts/agents/path-classes.txt:17). The candidate carriage loop processes only `record` paths — [observe.go:190](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/observe.go:190); direct register carriage rejects behavior paths earlier — [observe.go:1066](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/observe.go:1066). Existing ordinary memory files require a held goal — [observe.go:1191](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/observe.go:1191).
- **Failure:** Merely extending `recordCarriageError` never admits the doctrine edit. Conversely, interpreting “beside a terminal ruling” literally lets acceptance of A authorize unrelated changes to B. A human resolving a question before any goal exists still lacks the promised path.
- **Smallest change:** Specify candidate-wide admission for the recognized memory surface, covering documentation and ordinary register edits; authorize each changed path through a held goal or the proven human’s reviewed candidate, never through ruling adjacency. Pair archive deletion and addition by identity and identical bytes. State the commit-kind treatment of mixed candidates.

**C3. P3 needs separate installation and application roots, not just an `Outside` exception.**

- **Attacked:** §10 P3; `M:354`.
- **Evidence:** Classification passes `workspace.Dir` as the installation — [observe.go:1052](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/observe.go:1052). At the application root, that makes ownership follow the unvendored inventory, including `memory/`, `plans/` and `records/` — [owner.go:106](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/stateroot/owner.go:106), [owner.go:143](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/stateroot/owner.go:143). Policy loading also looks for installation-relative manifests in that workspace — [observe.go:1257](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/observe.go:1257), [observe.go:1269](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/observe.go:1269).
- **Failure:** Changing the command’s root does not produce the assumed correctly classified application candidate.
- **Smallest change:** Carry the actual installation root separately from the application candidate/state root, load policy from the installation and resolve ownership with that installation. Then admit recognized application records in both relevant landing routes. **The oracle itself can remain unchanged.**

**C4. P2 is honestly outside the wrapper, but its optional check is not an authority gate.**

- **Attacked:** §§10, 18.5; `M:353`, `M:430`.
- **Evidence:** Two humans check against the same tip, commit separately and combine their commits without another check; neither optional hook nor prior command validates the resulting publication. The installation workspace excludes host-root changes — [gittree.go:250](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/gittree/gittree.go:250). The master records Wido’s requested placement, not permission to bypass authority enforcement (`U:739`).
- **Failure:** Stale acts, duplicate identities or altered claims can enter the shared authority history despite every stated local instruction having been followed. Calling this “your bypass” attributes an unrecorded decision to Wido.
- **Smallest change:** Keep host-root placement and ordinary Git editing, but publish authority through C1’s checked boundary using the complete checkout candidate. If publication remains unmanaged, label its validation as advisory and its new authority as unverified; do not promise the same guarantees as P1.

**C5. `attorney:` has no compatible grant to consume.**

- **Attacked:** §§5.5, 7, 10; `M:183`, `M:314`, `M:344`.
- **Evidence:** Current grants name tiers and goal verbs; the allowed verbs are `approve`, `set-budget`, `unpark` — [root.go:46](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/root.go:46), [root.go:58](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/root.go:58). Unsupported verbs are refused — [verbs.go:1546](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/verbs.go:1546). Attorney execution is explicitly the seat’s act, without a human actor or proof — [verbs.go:1424](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/verbs.go:1424).
- **Failure:** A page has no goal tier; no existing grant authorizes its acceptance; recording the grantor as the deciding human misattributes delegated execution.
- **Smallest change:** Defer `attorney:` for memory acts. Supporting it now requires an explicit grant-schema extension defining memory verbs, subjects, scope, revocation checks and actual actor attribution, with compatible readers; it is not reuse alone.

**C6. Invalidation still has no deterministic correction contract.**

- **Attacked:** §§5.5, 7; `M:185–190`, `M:305–307`.
- **Evidence:** R1 accepts D, R2 withdraws D, R3 invalidates R2. Does the next act say `given R1` or `given R3`? Using R1 admits a stale request prepared before withdrawal. R4 invalidating R3 can resurrect R2 unless explicitly forbidden. If R1 accepts two pages, `invalidates R1` does not identify whether one act or the whole ruling is void. A malformed R1 may fail grammar before its correction is considered.
- **Failure:** Different implementations restore different authority, accept different stale requests or permanently refuse the repair. `reopens D` adds another ambiguity: restore which blob, when §7 already permits explicit acceptance after withdrawal?
- **Smallest change:** Define invalidation as a prospective, whole-ruling correction with expected state for every affected page and a fresh last-change token distinct from the restored applicability; do not revalidate old acts’ preconditions against rewritten history. Provide a recovery scan that can quarantine the named malformed payload. Refuse invalidation of corrections; repair mistaken corrections with explicit new acts. Delete `reopens` and use `accepts <id>@blob:…` after withdrawal.

The remaining lexical choice is bounded: specify escaping of quotes/backslashes and quote-aware handling of `;` and `]`, within one physical table row. This needs parser fixtures, not another storage mechanism.

**C7. Duplicate refusal has no repair path once the duplicate has landed.**

- **Attacked:** §§5.1, 5.5, 10; `M:107`, `M:185`, `M:346`.
- **Evidence:** An old writer or unmanaged P2 commit publishes two different pages with ID D; an acceptance already names D at one blob. Changing either ID is forbidden, while the complete-tree checker refuses the duplicate before admitting its correction.
- **Failure:** Neither normal publication nor authority repair can restore a valid repository; choosing a file by enumeration order would transfer authority arbitrarily.
- **Smallest change:** Define a human-authorized repair transaction for an already-invalid base, identifying the prior paths/blobs and the surviving or replacement identities; preserve old evidence and explicitly invalidate ambiguous acts. Never silently rebind an act or silently remint an identity.

**C8. The question-resolution check prevents the promised automatic reopening.**

- **Attacked:** §§5.3, 7; `M:146`, `M:307`.
- **Evidence:** Q’s unchanged resolution names accepted decision D; the next candidate withdraws D. The whole-register rule now requires Q’s target to be applicable, while the resolver is supposed to show Q reopened without editing it.
- **Failure:** Either the withdrawal is refused or the checker silently abandons its stated invariant.
- **Smallest change:** Check applicability/currentness when a resolution is newly assigned or changed; retain existing references when their answer loses standing and derive `reopened`. Bind a decision answer to the selected accepted revision so a later acceptance cannot silently replace what answered Q.

**C9. Object existence does not establish historical document provenance.**

- **Attacked:** §§5.2, 5.4; `M:120`, `M:171`.
- **Evidence:** An index binds `doc:docs/architecture.md@blob:H`, where H is a landed blob from an unrelated file and never occupied that path.
- **Failure:** The proposed object-store check passes while presenting false source provenance. Removing live-path dependence did not establish the historical association.
- **Smallest change:** Verify the path/blob association in a reachable historical tree or the exact candidate tree, including entry type and containment; continue resolving accepted bytes independently of the current path.

**C10. The evidence adapter still promises more than its grammar encodes.**

- **Attacked:** §§5.4, 6.6, 10, 16; `M:159–169`, `M:289`, `M:346`, `M:409`.
- **Evidence:** Counselor goal citations contain a path and an operation selector — [register.go:220](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/counselor/register.go:220); all supported citation kinds retain independent `kind`, `target`, `detail` fields — [sources.go:66](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/counselor/sources.go:66). Revision 5 defines selectors for `job-record:` but not `goal:` or ruling references; `record:` requires a blob that a legacy citation need not supply.
- **Failure:** A same-name mapping still loses information or invents retention. Moreover, §10 exempts unavailable `job:` but omits `job-record:`, and §16 demands nothing missing except sittings although §5.4 also permits unavailable sessions and jobs.
- **Smallest change:** Preserve the source triple as structured citation data and define a reversible encoding for each kind, including opaque detail and explicitly unpinned/unavailable legacy targets; make checker exceptions and the remote fixture match those availability states.

**C11. Monotone contributor metadata makes an ordinary recording error permanent.**

- **Attacked:** §5.7; `M:210`.
- **Evidence:** A copied head falsely lists session X as a contributor; that revision lands. Evidence later establishes that X never participated.
- **Failure:** X remains disqualified forever, or the human must create a new page identity to correct attribution. Neither follows from preserving genuine contribution history.
- **Smallest change:** Permit an explicit human provenance correction naming the prior revision, mistaken attribution and reason, preserving the prior bytes in Git; unresolved provenance still means independence is not established. Do not require another general event system.

**Attacks that do not justify another finding**

- **Cell-head parsing and `word` smuggling:** the outer grammar already forbids unescaped pipes (`M:171`). Parse the decoded context cell’s single leading packet and stop at its terminator; a second packet or `accepts` after `]` is prose (`M:175–188`). The specified semantics do **not** permit smuggling; test that readers and publishers obey them.
- **Mutable rows under union merge:** duplicate-row refusal followed by explicit reconciliation is workable at the stated human editing rate (`M:126`, `M:330`). It provides conflict detection, not automatic reconciliation. Claim immutability is checkable by row ID and the declared column schema; fixtures must reject header relabeling, malformed cell counts and claim changes disguised by column movement.
- **Area typos and split parents:** a misspelled slug can remain an opaque identity behind a corrected display name, or a replacement area can receive explicitly migrated memberships; children of a split parent must be explicitly reparented (`M:204`). No alias machinery is necessary. Concluded pages retain historical membership under §6.3; do not describe those frozen heads as freely editable current metadata.
- **Proof reuse:** the new verb can call the exported `Prove`; there is no verb whitelist in its signature — [authority.go:813](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/humanauthority/authority.go:813). The difficulty is C1’s publication binding, not API availability.

**5. Simplicity under R-11**

The sentence at `M:459` works as a navigation explanation, but fails as a maintenance law: identities and claims are immutable, concluded heads are frozen, contributor corrections are forbidden, and some ordinary metadata edits still require a held goal.

R-11 makes intuitive maintenance a convergence requirement — [rulings.md:36](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/memory/rulings.md:36). I would cut:

- Memory powers of attorney from this delivery.
- `reopens`, since explicit reacceptance already exists.
- Recursive invalidation.
- Any claim that a tree-only checker authenticates writers.
- Separate conceptual publication guarantees for identical records merely because their paths differ.

Keep the editable tables. Their reconciliation cost is visible and bounded; replacing them now would reopen the mechanism already settled in round 2.

**6. Incorrect or overstated code/source claims**

1. **“`Prove`/`ProveTerminal` establish enrolled-terminal authority interchangeably.”** They do not: `ProveTerminal` explicitly performs no enrollment lookup, and `TerminalValidFor` accepts unenrolled terminal grade — [authority.go:635](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/humanauthority/authority.go:635), [authority.go:201](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/humanauthority/authority.go:201). The proposed enrolled-only path should use `Prove` and `ValidFor`.

2. **“Exactly as `goal approve` does” means enrolled-only.** Current approval enters through `ProveOrTemporaryGoalAuthority`, which has a relay fallback — [goalsync_mutations.go:2237](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/goalsync_mutations.go:2237), [authority.go:371](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/humanauthority/authority.go:371). Reusing its enrolled proof mechanism is available; copying its complete admission policy is a different choice.

3. **Existing proof establishes the supplied deciding handle.** It establishes ancestry; explicit `--by` bypasses name derivation — [goalsync_mutations.go:1380](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/goalsync_mutations.go:1380).

4. **Existing `goal grant` supplies memory-act authority.** Its allowed verbs and tier scope do not — [root.go:58](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/root.go:58), [verbs.go:1537](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/goal/verbs.go:1537).

5. **P1’s “creation already passes” covers all listed homes.** The cited creation allowances concern plans and records; documentation is behavior and register carriage rejects it — [observe.go:1182](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/observe.go:1182), [observe.go:1198](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/observe.go:1198), [observe.go:1066](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/observe.go:1066).

6. **P3 needs only an `Outside` rule after running at the application root.** That overlooks installation-root classification and policy loading — [observe.go:1052](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/observe.go:1052), [observe.go:1257](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/landing/observe.go:1257).

7. **Master support for gate-3 shared brain writers.** The master places working material in the private sitting store at gate 3 and shared persistence at gate 5 (`U:378`, `U:811`, `U:813`). Browser human authority arriving at gate 3 is correct; moving shared writers earlier needs an explicit sequencing amendment.

The cited CLI assignment `RepoRoot: *root` is accurate but does not itself prove an installation-only API; the wrapper selects the installation root — [landing_verbs.go:62](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/landing_verbs.go:62), [land.sh:12](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/scripts/agents/land.sh:12). Source also cannot establish which command created **every** historical host-root commit.

**7. The way forward**

1. **§10:** Make authority admission and exact-candidate publication consume the same live proof, bind the principal, and enforce human-only metadata permissions — C1.
2. **§10 P1:** Define candidate-wide, path-specific admission for documentation, registers, plans and paired archival moves — C2.
3. **§10 P3:** Separate installation policy/ownership context from application candidate/state scope, preserving the oracle — C3.
4. **§§10, 18.5:** Retain host-root placement while giving authority publication the same checked boundary — C4.
5. **§§5.5, 7, 10:** Defer memory `attorney:` until a deliberately scoped grant extension exists — C5.
6. **§§5.5, 7:** Specify prospective correction scope, expected tokens, malformed-history recovery and quoting, and remove `reopens` and recursive invalidation — C6.
7. **§§5.1, 10:** Add explicit repair of already-published duplicate identities without implicit authority transfer — C7.
8. **§§5.3, 7:** Validate newly selected resolutions, retain historical answers and derive reopening when their standing changes — C8.
9. **§§5.2, 5.4:** Validate historical path/blob associations while keeping resolution independent of live paths — C9.
10. **§§5.4, 6.6, 16:** Preserve citation triples losslessly and align unavailable-evidence checking and walkthrough expectations — C10.
11. **§5.7:** Allow evidenced human correction of false contributor attribution while preserving history and conservative independence judgments — C11.
12. **§§16–17:** Give each correction a named positive/refusal fixture, exercise actual publication, re-estimate the affected slices, and align shared brain writers with the master’s gates.

**With these changes adopted, the design is the design of record; the next arbiter is the implementation fixtures and code review. I am not requesting another general prose critique.**

| §18 decision | Genuinely Wido’s? | My recommendation |
|---|---|---|
| 1. Mutable metadata, immutable claims and acts | **Yes:** the governing maintenance policy. | **Agree**, with explicit repair rather than permanent recording errors. |
| 2. Terminal authority before browser access | **Yes:** the human workflow and trust boundary. | **Agree**, using enrolled proof and checked publication. |
| 3. Root-installed adopters read-only | **Yes:** supported scope. | **Agree.** |
| 4. ULIDs, full durable references, titles in views | **Primarily engineering:** no further human ruling is needed absent a usability objection. | **Agree**; “humans never read IDs” should remain an aspiration, not a guarantee. |
| 5. P2 outside landing with optional checking | **Yes:** accepting weaker publication guarantees would be his decision. | **Disagree as recommended:** placement does not authorize the bypass. |
| 6. Historical designs remain untyped | **Yes:** migration scope and priority. | **Agree.** |
| 7. Intent harvest as Wido’s sitting | **Yes:** he confirms intent and schedules the work. | **Agree**, with the author preparing the harvest and Wido adjudicating it. |

The remaining policy disagreement with the author is §18.5. Wido can rule in one line: **“Keep host-root designs where they are; authoritative publication must use the same checked boundary as other project records.”** The other findings are corrections to the implementation contract, not requests that Wido design parsers or publication plumbing.

Proposed receipt, not written: `type=review | skills=design-critique | outcome=sound-after-listed-changes | verify=source-read-only | note=memory-system revision 5; 11 material findings`.

Codex session ID: 01a0c814-e83d-7571-8fb9-443306b6668d
Resume in Codex: codex resume 01a0c814-e83d-7571-8fb9-443306b6668d
