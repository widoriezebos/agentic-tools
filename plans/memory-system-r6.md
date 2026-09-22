# The memory system: what a project remembers, where, and in what shape

- Kind: design
- Id: 01K5S0MEM0RYSYSTEMDESIGN000
- Readiness: proposed
- Revision: 6
- Areas: project
- Contributors: human:wido claude-fable-5-1 gpt-6-astra
- Author: Claude on Fable, 2026-09-22, on Wido's order of the same day
- Subject: the record kinds behind the Project section — intent, architecture and its decisions, designs, rulings, open questions, facts, proposals, and the principals who own them — their grammar, homes, identity, standing, authority and writers, for an application built with the MetaSystem and for the MetaSystem itself
- Cites: `metasystem/plans/memory-architecture-design.md` (concluded, `records/goals/memory-architecture.md`), R-6 to R-11, the master `plans/user-interface-design.md` (`:57` to `:83`, `:207` to `:232`, `:388` to `:418`, `:473` to `:490`, `:737` to `:777`, `:803` to `:823`), the paper's chapters 2, 8, 15 and 19, `metasystem/skills/inception/SKILL.md`, [Astra's round 1](memory-system-critique-astra.md), [round 2](memory-system-critique-astra-r2.md), [round 3](memory-system-critique-astra-r3.md)
- Revision 2, 2026-09-22: intent and doctrine became books; hand-maintained lists removed; facts narrowed to selected facts; scale section.
- Revision 3, 2026-09-22: areas, owners, the newcomer's test, after Wido's directive on large enterprise systems built by many humans.
- Revision 4, 2026-09-22, after Astra's round 1 (redesign, 16 findings): immutable ids; standing derived from bindings; areas as a field; principals; historical resolution by blob; a page grammar of its own; two layouts; carriage before migration.
- Revision 5, 2026-09-22, after Astra's round 2 (redesign, 12 findings, one mechanism named): versioned current records plus a narrow append-only authority history; readiness, applicability and completion as three properties; proven acts.
- Revision 6, 2026-09-22, after Astra's round 3 (**sound after the listed changes**, 11 findings, no mechanism replaced): the authority verb is the publication boundary and consumes its own proof; enrolled proof only, the handle derived from enrolment, never asserted; `attorney:` and `reopens` removed; invalidation is a prospective whole-ruling correction with a last-change token; duplicate-identity and contributor corrections exist; candidate-wide admission per path for P1; a separate installation root for P3; P2's page edits named unverified and its authority published through the same boundary; historical path-and-blob provenance checked; citations carried as triples; question resolutions validated on assignment and retained; fixtures per correction, exercising the real publisher; brain shared writers at gate 5. **This is the design of record, pending Wido's rulings in section 18; the next arbiter is the implementation fixtures and code review** (round 3, section 7).

Paths are relative to the checkout root unless stated; `<root>` is the state root the engine resolves (`metasystem/internal/stateroot`). Every claim about existing code or files was read in source at `ui-development` `b712f2f03` on 2026-09-22; nothing was executed.

## 1. Why now: the gap, as found by building

The Project pane (`g1-s21`, merged `fdac4a889`) was built as six subsections over "documents that already exist at their canonical locations". Building it showed that for four of the six there is no canonical location, only habit. The catalogue (`metasystem/internal/ui/project/catalogue.go:97-124`) decides that a file is *architecture* because inception says it writes `docs/app-doctrine.md`, that a file is a *design* because its name ends in `-design.md`, and, honestly, that *open questions* are "Not yet recorded" while pointing at the nearest registers (`catalogue.go:73`, `:121`). Nothing in the kit declares a document's kind; nothing gives a design an identity that survives its move from `plans/` to `records/`; nothing records an architectural decision, a fact, or an open question as a thing with an id that a design, a goal, or a later sitting can cite. The pane did what it could with filenames, which is the invented agreement the master forbids (`:488`).

The paper is explicit about what has to exist. Intent is "the outcome, its constraints, the open questions, the freedoms left to construction and the observations that will later count for or against it" (`19-appendix-functional-design.md:9`). A sitting deposits facts anchored to where they can be checked, decisions recorded then and there with their reasons, and open questions named so nobody settles them by default (`15-the-sitting.md:33`). Rulings are precedents, institutional memory beyond the people who decided (`08-memory-and-coordination.md:61`). And the test: "End the sitting at any moment, without warning. A fresh worker and the same human must be able to continue from the records alone" (`15:35`). Today that test fails for everything but goals and rulings.

The memory-architecture goal (concluded 2026-08-28) designed the **trees**: four places per owner, an ownership oracle, `stateRoot()` routing, app and MetaSystem material kept apart. It deliberately stopped there — "No memory database, no index machinery, no per-file ownership tags" (`memory-architecture-design.md:148-154`). This design is the next layer: the **records** in those trees. Where the trees' rules cannot hold a record honestly, section 11 says what changes and why.

## 2. What already exists

Everything above the line has a grammar, an id and a writer; everything below has at most a filename.

| Kind | Home | Grammar | Id | Writer | Enforced by |
| --- | --- | --- | --- | --- | --- |
| Goal | `plans/goals/<id>.md`, concluded `records/goals/<id>.md` | `# <id>`, `- Key: value` head, `History:`, `Integrity:` trailer; unknown fields, free prose and an empty History are refused (`internal/goal/file.go:536`, `:583`, `:611`, `:1523`) | file name | goal engine only | ledger rules, opids, integrity hash; human-only verbs proven in-process (`internal/humanauthority/authority.go:1-3`): `Prove` walks the process ancestry and checks enrolment (`:813`), `ProveTerminal` walks it without an enrolment lookup and `TerminalValidFor` accepts any agent-free terminal (`:633-637`, `:201`); `goal approve` enters through `ProveOrTemporaryGoalAuthority`, which has a relayed-word fallback (`cmd/metasystem/goalsync_mutations.go:2237`, `authority.go:371`); an explicit `--by` skips deriving the human's name from enrolment (`goalsync_mutations.go:1379-1382`); parsed proof has no authority (`authority.go:107-110`, `:194-195`) |
| Power of attorney | `plans/goals/backlog.md`, `PowerOfAttorney:` | `by= tiers= verbs= since= expires=`; verbs `approve`, `set-budget`, `unpark` only; an act under it is the seat's own, with no human actor or proof (`internal/goal/root.go:46-58`, `verbs.go:1424`, `:1537-1548`) | opid | `goal grant` | the goal engine |
| Ruling | `memory/rulings.md` | table `id \| date \| ruling \| context \| owner \| review condition`; `owner` is the accountable owner (`rulings.md:5`); ordered by file position (`:17-18`) | `R-<n>-<machine>` (`dispatch/slice.go:18`) | humans and seats by hand | append-only carriage that checks appended rows' shape, not who decided (`landing/observe.go:1141`, `:1369-1372`); bare-id refusal; `merge=union` (`.gitattributes:3`); `--approved-ref` reads it |
| Known issue | `memory/known-issues.md` | table, `KI-<n>`; **two column sets**, shipped (`adopt.sh:310`) and live (`known-issues.md:7`) | `KI-<n>` | hand | ordinary `memory/` carriage under a held goal (`observe.go:1191`); `merge=union` (`.gitattributes:5`) |
| Instruction change | `memory/instruction-ledger.md` | table, `IL-<n>` | `IL-<n>` | retro | retro verdicts |
| Receipt | `memory/receipts.log` | pipe-delimited (`receipt.go:253`) | offset | engine | append-only, retro debt |
| Covenant | `covenant.json` | JSON schema v1, exact keys (`covenant.go:237`) | requirement ids `[A-Za-z0-9._-]+` | inception, human | `covenant validate` |
| Evidence table | `docs/covenant-evidence.md` | one table, one count line (`SKILL.md:82-104`, `evidencetable/evidence.go:46`) | criterion id = requirement id | inception, human | `covenant evidence` gate |
| Counselor register | `records/counselor/*.jsonl` | `{id, kind, class, specimenFacts[{fact, citations[{kind,target,detail}]}], reviewLinks}`; citation kinds `commit`, `goal`, `ruling`, `job`, `record`, `job-record`, each an independent triple with selectors in `detail` (`counselor/sources.go:66`, `:245`; `register.go:205`, `:220`) | `ar-…`, `mc-…` (`register.go:189`, `:222`) | engine | the validator accepts non-empty targets; it does not prove retention (`sources.go:211-218`) |
| Goal draft | `plans/goals-drafts/<name>.md` | free-form, no grammar (`backlog-mechanism.md:121`) | none | hand | intake checklist at `goal open`, which records origin, not a proposal reference (`goalsync_mutations.go:1014`, `:1760`; `goal/verbs.go:816`, `:848`) |
| The paper | `metasystem/docs/paper/index.md` and nineteen chapters | a book | chapter file name | hand | — |
| — | — | — | — | — | — |
| Intent (project) | none — the `Purpose:` line of `docs/project-rules.md:8`; vision goes into "the doctrine's opening" (`SKILL.md:60`) | none | none | inception | — |
| Architecture | `docs/app-doctrine.md`, declared with three content obligations and no template (`SKILL.md:69-80`); **no instance exists** | none | none | inception; the `app-doctrine` goal (queued, approval withdrawn 2026-09-11) | — |
| Architecture decision | none | none | none | — | — |
| Design | `plans/*-design.md`: 102 in `metasystem/plans`, 14 under `plans/user-interface/`, 11 in `records/misc`, three unrelated head shapes | none | none | hand | only the UI's glob (`project.go:466-476`); a design read is admitted only at a `metasystem/` path (`dispatch/read_subject_compute.go:199`) |
| Open question | none | none | none | — | — |
| Fact | `records/*/*-facts.md`, prose; every design's "existing code" table | none | none | hand | — |
| Proposal | `memory/backlog-notes.md` P-section (`:466`), `memory/proposal-drafts.md` — two homes, neither shipped | numbered prose | `P-<n>`, `D-<n>` | hand | R-93-m1e names the P-section as the home |
| Principal | none; `human:<name>` on ledger lines, a name configured per seat (master `:65`) | — | a display name | — | — |
| Sitting | the server-local store, gate 3 (master `:376`) | — | — | brain process | not built |

What the landing does with records today, because the write paths depend on it. The landing works in the installation's tree: the CLI passes the installation as the root (`landing_verbs.go:62`; `scripts/agents/land.sh:12` selects it) and the workspace's trees are scoped to that prefix (`gittree/gittree.go:171-174`, `:250-253`). It classifies each changed path by joining the installation prefix and asking the oracle **with the workspace directory as the installation** (`observe.go:1045-1056`), and loads its policy manifests from that same workspace (`:1257`, `:1269`). The candidate policy applies the record carriage law to `record`-class paths only (`:190`); the direct register carriage refuses `behavior`-class paths outright (`:1062-1068`); app-owned paths in an adopted layout resolve to `Outside` (`pathclass.go:223`) and are refused unless the caller allows outside paths (`observe.go:1096-1098`). Under `plans/`, a new file may be created, an existing file is frozen unless a goal owns it, and ownership is inferred only for files directly under `plans/` (`:1167-1188`, `:1230`); `memory/` files change under a held goal (`:1191`); `records/` files may be created, and an existing one requires a held goal and is append-only (`:1198-1208`). `docs/` is `behavior` class (`path-classes.txt:17`) and has no record carriage at all. A `Goal-Plan` commit may carry `metasystem/plans/` and `metasystem/records/` paths of class plan, not `records/misc/`, `records/reads/`, the counselor registers or concluded goals (`goal/branch/range.go:176-185`, `:225`; `landing/registers.go:40`); a path without the `metasystem/` prefix is class unit (`range.go:185`).

## 3. The four properties, and the principles that serve them

Wido's directive, 2026-09-22, which every contract below is measured against: many humans will use the interface and the MetaSystem to build large enterprise systems together; intent will evolve over time and the system must facilitate that by design; the structure of the memory types must be **robust against change over time**, must **scale**, must be **intuitive for humans**, and must be **easy to use for the interface and the agents**.

1. **Identity is in the record, never around it.** Every page and every row carries an immutable id minted at creation. Paths, titles, areas, owners and homes are mutable; the id is not.
2. **Three layers, three laws.** *Content* — a page's bytes, a row's claim — is versioned: every landed revision is retrievable by blob and a row's claim cells never change. *Metadata* — title, areas, owner, readiness, a question's resolution, a proposal's disposition, a principal's name — is ordinary: edited in place, with Git as its history. *Authority* — acceptance, withdrawal, supersession, retirement, and the corrections to them — is an append-only history of acts in the rulings register, each act naming exactly what it applies to.
3. **An act of authority is a checked publication, not a row.** The verb that records a human's act proves at the enrolled terminal that the human is present, derives who they are from that enrolment, composes the exact candidate, checks it against the live tip, and publishes it — in one process. A row is what that leaves behind; later readers trust admitted history, never a row's own claim to have been proven.
4. **Declared, never inferred.** A record says what it is, what it is about and what it rests on, in its own fields. Nothing is classified by filename, directory or neighbour; nothing is listed by hand; every list the interface or the brain shows is a query over those fields.
5. **Relationships describe; acts change.** `Cites`, `Affects`, `about` are relationships and change nothing. Only an act changes what governs; only an edit changes metadata.
6. **One structure, walked two ways.** Areas are declared once with stable slugs; every record names its areas; the interface's tree and the brain's answers are the same query.
7. **Mistakes are corrected, never rewritten.** A wrong act, a duplicated identity, a false attribution each has an explicit, human-published correction that preserves what was there.
8. **R-11.** If the layout needs explaining, it fails. Four trees, two books, four registers, one resolver, one check, two verbs of authority.

## 4. The kinds

| The question a human asks | Kind | Shape | Home (`<root>/…`) | Readiness (author) | Applicability (acts) | Who writes | Who decides |
| --- | --- | --- | --- | --- | --- | --- | --- |
| What are we building, for whom, and what counts as success? | **intent** | book: index and chapters, each a page | `docs/intent/` | draft, proposed | accepted, withdrawn, superseded, retired | inception, the human, the brain | a human, by a proven act, per page |
| How is it shaped? | **doctrine** | book; a chapter may bind an existing document | `docs/doctrine/` | the same | the same | the same, the `app-doctrine` loop | the same |
| Why that shape; what over what? | **decision** | page | `docs/decisions/` | draft, proposed | accepted, withdrawn, superseded | the human, the brain, a seat | a human, by a proven act |
| What change, and how? | **design** | page | `plans/designs/` while live, `records/designs/` once concluded | draft, proposed | accepted, withdrawn, superseded | a seat, the brain, the human | a human, by a proven act |
| What did a human decide, when, why, with what authority? | **ruling** | row | `memory/rulings.md` (unchanged shape) | — | is the authority | the verb; seats for recorded words | — |
| What is deliberately unsettled? | **question** | row | `memory/questions.md` | — | resolved by a ruling, a fact or a decision (metadata) | anyone | a human sets the resolution |
| What is relied on as true, and where can I check? | **fact** | row | `memory/facts.md` | — | superseded / refuted (metadata) | anyone; the brain | — |
| What might we do? | **proposal** | row | `memory/proposals.md` | — | taken up by / dropped (metadata) | anyone | a human sets the disposition |
| Who is who? | **principal** | row | `memory/principals.md` | — | active / departed (metadata) | the human | — |
| What is authorised, with what budget? | goal | page | `plans/goals/` (unchanged) | the ledger's | | goal engine | human |
| What is accepted as broken? | known issue | row | `memory/known-issues.md` (unchanged; both column sets read) | — | | hand | — |
| What did a sitting say? | sitting | transcript | the server-local store (gate 3) | — | | brain process | — |

Constraints and assurance keep their homes: `covenant.json`, `docs/covenant-evidence.md`, `docs/project-rules.md`. Handoff notes, the instruction ledger, the flake registry, receipts and retro debt are the machinery's working memory and are untouched.

## 5. The grammars

### 5.1 Pages

A page is a Markdown file: a title line `# <title>`, one blank line, a **head** of contiguous `- Key: value` lines ended by the first blank line, then free prose. The head's syntax is the goal file's; its rules are its own (the goal parser demands a trailer and a History block and refuses prose and unknown keys, `file.go:536`, `:583`, `:611`, `:1523`): a value is one line; a key appears at most once, a duplicate is refused; an unknown key is kept and reported, never refused; nothing follows the body.

```markdown
# Events are the source of truth

- Kind: intent | doctrine | decision | design
- Id: 01K5R2H3ZQ8V6M7N9P0A1B2C3D            immutable, a ULID minted at creation; never derived from the path
- Readiness: draft | proposed                the author's claim about these bytes, and nothing more
- Revision: 7                                a label for humans; increments on every landed change; never identity
- Areas: billing security                    declared slugs, one or more, or `project`; metadata
- Cites: 01K5…@blob:4b82… criterion:3        what this record rests on; required on decisions and designs, `none` allowed
- Affects: 01K6… 01K7…                       decision: what it changes; required on decisions
- Governs: goal:g1-s21                       design: the goals it governs, as declared by the design; optional
- Contributors: human:wido m1+coordinator session:01K5…   cumulative; required at readiness `proposed` (5.7)
- Contributors-corrected: session:01K5… at=r3 by=human:wido reason:"copied head"   a correction (5.7); optional
```

`Kind`, `Id`, `Readiness`, `Revision` and `Areas` are required. Nothing in a page says whether it is accepted: applicability is the authority history's answer (section 7), read through `project show`. The title and the path change freely; the resolver finds a page by `Id` in any of its kind's homes. A concluded design's head is frozen with the rest of its bytes; its historical `Areas:` stay as they were. Two files with one id, an id edited after creation, a page copied with its id — each is a refusal by `project check`; a duplicate that has already landed is repaired by the correction of 5.5, never by silent re-minting.

### 5.2 Books

A book is a home whose pages read in an order. Its index carries, after the head, a `## Chapters` list in reading order:

```markdown
## Chapters
- 01K5… — Users
- 01K6… — Billing
- doc:metasystem/docs/paper/01-the-shift.md@blob:8f1c… — 1. The Shift
```

A chapter is a page of the same kind, by id; a chapter of another kind is refused. A chapter may also be a **bound document**: an existing file in this checkout, named by checkout-relative path and pinned to exact bytes by blob — inception's retrofit rule (`SKILL.md:77-79`) and how the MetaSystem's own intent book reads the paper. A bound document has no head and no applicability of its own; the acceptance it enjoys is the index's, at the revision that bound it. `project check` verifies a binding's **provenance**: that a tree reachable from the default branch, or the candidate tree itself, has that blob as a regular file at that path — never merely that the blob exists, since any landed blob would pass that. The resolver then reports two things separately: the accepted bytes, always retrievable by blob, and whether the source is still at its path. A path that leaves the checkout, by `..` or by symlink, is refused as `g1-s21` refuses it.

Every chapter has its own revision and its own acceptance; the index's revision moves when its chapter list moves. A small application's book is one `index.md`. Reading order is the one hand-authored list here, because reading order is a human choice; it is not structure (5.6).

### 5.3 Rows and registers

A register is a Markdown file with a lead paragraph stating its law and one table. A row has **claim cells** and **metadata cells**, identified by the table's declared header, not by position: the check refuses a register whose header was relabelled or whose rows have the wrong cell count, so a claim cannot be changed by moving columns. Claim cells — the id, the date, the question, the claim, the proposal, the ruling — never change after the row lands; `project check --base` refuses a landing that alters one. Metadata cells are ordinary and edited in place; Git is their history. Rows are appended, never deleted. The registers keep the kit's `merge=union` convention (`.gitattributes:1-8`), which detects conflicts and settles none: concurrent appends never conflict, and if two writers edit one row's metadata in one window the union leaves two rows with one id, which the check refuses and a human reconciles — visible, bounded at human editing rates, never silent. Ids are `<PREFIX>-<ULID>`; persisted references carry the full id; a prefix is a command-line convenience and nowhere else.

```markdown
# Open questions
| id | opened | question | areas | about | owner | resolution |
   Q-<ULID>  date   claim: one paragraph   metadata: slugs   refs   handle   empty | resolved by <ref> | withdrawn <date> <reason>

# Facts
| id | date | claim | anchor | areas | about | observed by | status |
   F-<ULID>  date   claim   claim: a locator (5.4)   metadata   refs   handle, job or session   current | superseded by F-… | refuted by <ref>

# Proposals
| id | date | proposal | areas | about | raised by | disposition |
   P-<ULID>  date   claim: any length, or a short entry citing a `doc:` note that holds the body   metadata   refs   handle   open | taken up by <ref> | dropped <date> <reason>

# Principals
| handle | kind | name | state | since | notes |
   human:<handle> | team:<handle>   claim   metadata: display name   active | departed   date   free text, never parsed
```

A question's `resolution` names the record that settled it: a ruling when the answer is a human's choice, a fact when it is an observation, a decision — **at the accepted blob**, `01K7…@blob:…` — when it is an architectural choice. The check validates a resolution **when it is newly set or changed**: the named ruling or decision must be applicable and the named fact current. It never re-validates an unchanged resolution, so that withdrawing a decision later is not refused because a question still names it; the resolver derives *reopened* for a question whose answer has lost standing, and a human may then set a new resolution. A proposal's `disposition` is set by a human when they take it up; a citation from a draft has no effect on it. Both `resolution` and `disposition` are human-only edits and are published through the boundary of section 10. A principal's handle is chosen once; name and state change in place; a team is a handle that can own an area and nothing more until the authority owner shapes membership.

The rulings register is unchanged in columns and ids. A **new** ruling that exercises authority carries its acts in the grammar of 5.5 at the head of its `context` cell, and is written only by the verb of section 10.

### 5.4 References and locators

One spelling everywhere — head fields, cells, acts, the interface's routes, a goal's future `DefinitionRefs`, a brief:

| Reference | Resolves to |
| --- | --- |
| `<ULID>` | a page, wherever it sits in its kind's homes |
| `<ULID>@blob:<sha1>` | that page's exact bytes at one revision, from the Git object store |
| `intent`, `doctrine` | the book's index page |
| `R-…`, `Q-…`, `F-…`, `P-…`, `KI-…`, `IL-…` | a row, by full id, in the register its prefix names |
| `goal:<id>` | `plans/goals/<id>.md`, else `records/goals/<id>.md` |
| `criterion:<id>[@blob:<sha1>]` | a criterion id in the evidence table, optionally at one revision of that table |
| `human:<handle>`, `team:<handle>` | a principal |
| `area:<slug>` | a declared area |
| `doc:<path>[@blob:<sha1>]` | a Markdown file in the checkout, by checkout-relative path; with a blob, provenance-checked as in 5.2 |
| `src:<path>@blob:<sha1>[#L<a>-L<b>]` | any file at exact bytes, provenance-checked, optionally a line range |
| `commit:<sha1>`, `record:<path>@blob:<sha1>` | a retained observation in the repository |
| `job:<id>`, `job-record:<path>[#<selector>]` | an observation under `artifacts/`: a locator, not proof of retention; availability is *retained here* or *not available here* |
| `cite:<kind>:<target>[#<detail>]` | a counselor citation carried as its triple, reversibly: `kind` one of `commit`, `goal`, `ruling`, `job`, `record`, `job-record`; `detail` opaque; a legacy target without a blob is *unpinned*, a state, not a defect |
| `cmd:"<command>"` | a recipe to observe again; never evidence of a past observation |
| `session:<id>`, `sitting:<id>` | provenance in the server-local store; never required to resolve; *not available here* elsewhere |

References in a field or cell are whitespace-separated; a value containing whitespace is double-quoted; inside quotes, `"` and `\` are backslash-escaped; a pipe inside a cell is `\|` as the registers already require. Anything with `@blob:` resolves against the object store even after the file moved or was deleted; `project check` verifies every referenced blob is landed or present in the candidate. The availability states the checker exempts by rule, and that the remote handoff fixture expects, are exactly: `session:`, `sitting:`, `job:`, `job-record:`, and `unpinned` legacy citations. All paths are checkout-relative; the MetaSystem's own documents are `metasystem/docs/…`. Cross-project references are out of scope (master `:453`).

### 5.5 Authority acts

An act is the only thing that changes what governs. Acts live in one place — the head of a ruling's `context` cell, one packet per ruling — in one versioned, delimited grammar:

```
[acts/1 by=human:wido recorded=m1+coordinator basis=terminal:01K5…OPID;
 accepts 01K5R2H3ZQ8V6M7N9P0A1B2C3D@blob:4b825dc642cb6eb9a060e54bf8d69288fbee4904 given none;
 supersedes 01K3… by 01K5R2H3ZQ8V6M7N9P0A1B2C3D given R-120-m1]
```

- **Header.** `by` is the deciding principal, **derived from enrolment by the verb, never supplied**; `recorded` is the seat lineage or the human; `basis` is `terminal:<opid>` for an act published by the verb, or `word` for a human's word a seat wrote down. There is no `attorney:` basis in this delivery: the kit's power of attorney covers three goal verbs at tiers 1 and 2 and an act under it is the seat's own act with no human actor (`root.go:58`, `verbs.go:1424`, `:1537-1548`); extending it to memory acts is a grant-schema change of its own, deferred.
- **Lexis.** One packet, `[acts/1 …]`, at the very start of the decoded cell; acts separated by `;`, the packet ends at the first unquoted `]`; values with whitespace or `;` or `]` are double-quoted with `"` and `\` backslash-escaped; ids are full, never prefixes; everything after `]` is prose, and a second packet or an act-shaped phrase in prose is prose. The cell stays one physical table row.
- **Verbs.** `accepts <id>@blob:<sha1>`; `withdraws <id>`; `supersedes <id> by <id>`; `retires <id>` (intent and doctrine chapters); `corrects R-<id> …` (below); `repairs-identity <id> …` (below). Withdrawing a superseded page, superseding a withdrawn one, accepting a page at a blob that is not one of its revisions, an unknown verb, or a `word` basis carrying any act — each is refused. Accepting after withdrawal is `accepts` again at a named blob; there is no `reopens`.
- **`given`** names the target's **last-change token**: the id of the latest ruling that changed that target's history — an act or a correction — or `none`. The token is distinct from the effective act: after R1 accepts D, R2 withdraws D and R3 corrects R2, D's effective act is R1's acceptance and its token is R3; the next act on D says `given R3`, and a request prepared before the withdrawal (`given R1`) is refused. An act whose `given` is not the token at the publication tip is refused; the lander rebases, reads what happened, and decides again.
- **Order** is the register's own: file position on the default branch, acts within a packet in listed order; two acts on one target in one candidate are applied in order and the second names the first's ruling as `given`.
- **Corrections.** `corrects R-<id> reason:"…" expects:<id>@blob:…,<id>@blob:…` voids **the whole ruling** R-<id> prospectively — every act it carried — and states, for every page that ruling touched, the applicability the corrector expects to result, which the check verifies against the fold; a correction is admitted even when R-<id> fails the grammar, because the check treats a ruling named by a correction as opaque, and `project check --quarantine` lists malformed packets so one can be named. A correction cannot be corrected; a mistaken correction is repaired by fresh explicit acts (`accepts … given <token>`). Old acts' preconditions are never re-validated against corrected history.
- **Identity repair.** `repairs-identity <id> keeps:<path>@blob:<sha1> retires:<path>@blob:<sha1> reason:"…" rebinds:R-<id>,R-<id>` resolves two landed files claiming one id: the kept file keeps the id, the retired file's bytes stay in history, and every act that named the id before the repair is either listed in `rebinds` — re-bound to the kept file at its blob — or voided by the same repair; nothing is chosen by enumeration order and no id is re-minted.

Acts bind pages only. Rows change by metadata, never by acts.

### 5.6 Areas

Areas are declared once, in the intent index, under `## Areas`:

```markdown
## Areas
- billing — Billing and invoicing — owner:team:payments
- security — Security and identity — owner:human:priya
- refunds — Refunds — parent:billing
- legacy-portal — Legacy portal
```

A slug is immutable and is the area's identity; the display name changes in place; a misspelt slug keeps its spelling behind a corrected name, or a replacement area receives explicitly migrated memberships. An area may have a parent (acyclic, checked) or none; the tree derives from parents; when a parent is split, its children are explicitly reparented. An area may have no owner and is shown as unowned. `project` is the reserved slug for project-wide records. Every live page's `Areas:` and every row's `areas` cell names declared slugs; an undeclared one is refused. Splitting billing into payments and invoicing is: declare the two, edit the `Areas:` of the live pages and the `areas` cells of the rows that move, reparent refunds — ordinary metadata edits, one landing. Concluded pages keep their historical membership.

### 5.7 Principals and contributors

A principal is a stable handle in `memory/principals.md`, assigned once, never a display name: `human:priya`, `team:payments`. A seat's configuration names its human's handle, and the verb of section 10 derives the deciding handle from the enrolled seat, never from a flag. Two humans called Alex have two handles because someone assigned them. A handle's display name and state are metadata; departure is a state, not a deletion. What a handle is to the server — an account, a sign-in — is gate 3's and the authority owner's.

`Contributors:` on a page is the cumulative set of principals, seat lineages and sessions that shaped it: required from readiness `proposed` on; a later revision's list must contain the earlier landed revision's, and the check refuses a landing that drops an entry — unless a `Contributors-corrected:` line names that entry, the revision it was wrongly added at, the correcting human and the reason, published through the boundary of section 10. An assignment owner deciding who may examine work built from a design reads both lines; a page without `Contributors:`, or with an earlier revision missing it, yields *independence not established*, never an empty set taken as nobody.

## 6. The homes, kind by kind

### 6.1 Intent — the book at `docs/intent/`

Inception writes the index at readiness `draft` from steps 2 and 3 of the interview (vision, then criteria with stable ids), declares the areas the interview surfaced, writes a chapter per area when there are areas, and on the human's confirmation in step 8 publishes **one ruling** through the verb, whose acts accept the index and each chapter at their blobs, in the same candidate that creates them (section 10). Recommended shape:

```markdown
# <application name>

- Kind: intent
- Id: 01K5…
- Readiness: proposed
- Revision: 1
- Areas: project
- Contributors: human:priya session:01K5…

## Vision            what the application is for, in a paragraph
## Boundaries        what it is not
## Areas             the declared areas (5.6)
## Chapters          reading order (5.2)

(for a book of one page, the chapters' sections follow here)
## Users             who it serves and what they need
## Outcomes          what must be true when it works
## Constraints       what must hold throughout
## Freedoms          what construction may decide
## Criteria          `criterion:1`, `criterion:2` — ids; the evidence table holds the text and the interface renders it
## Observations      what will later count for or against it in production
## Open questions    `Q-…` references
```

Criteria are referenced by id and never copied: the evidence table is the one home of a criterion's text (`SKILL.md:82-104`). Exactness about what a criterion said when a design cited it is `criterion:3@blob:<sha1>` of the table.

In the self-hosted checkout the MetaSystem's own intent book is `metasystem/docs/intent/`: an index whose chapters are the paper's, bound by path and blob, and whose criteria are the **full confirmed intent** — not only the proven commitments `plans/covenant-draft.json` admits (`plans/covenant-harvest-survey.md:52`) but the gaps the survey excluded because no proof exists yet (`:101`, `:123`), each with its status as inception requires (`SKILL.md:132`). The author prepares that harvest; Wido adjudicates it; it is not a mechanical slice.

### 6.2 Architecture — the book at `docs/doctrine/` and the pages at `docs/decisions/`

The doctrine keeps its three obligations (`SKILL.md:69-80`) and becomes a book at `docs/doctrine/index.md`; `docs/app-doctrine.md`, declared but never instantiated, is renamed in the skill and the adoption fixture (`adopt-fixtures.sh:952-975`); an adopted project that already has architecture documents binds them as chapters where they are. Recommended shape: Context; Chapters; and, for a book of one page, Components, Interfaces and data flows, Dependencies, Patterns chosen and refused, Conventions delegates must honour. There is no list of decisions in it: the decisions that shape it are the pages whose `Affects:` names it, listed by the resolver by applicability.

An **architecture decision** is a page when it has context, options and consequences worth keeping — the ADR shape:

```markdown
# Events are the source of truth

- Kind: decision
- Id: 01K7…
- Readiness: proposed
- Revision: 1
- Areas: billing
- Affects: 01K6… criterion:4 01K8…
- Cites: F-01K5… 01K9…@blob:…
- Contributors: human:wido session:01K5…

## Context
## Options considered
## Decision
## Consequences
```

The file's name is free and carries no identity. A decision that needs no page is a ruling and nothing else. Reversing a decision years later is a ruling whose act `supersedes` it by its successor or `withdraws` it; its bytes never change. In the self-hosted checkout the doctrine binds `metasystem/docs/architecture.md`, `metasystem/docs/concepts.md`, `metasystem/docs/design/design-principles.md` and the rest of `metasystem/docs/design/` by path and blob; nothing moves.

### 6.3 Designs — `plans/designs/` while live, `records/designs/` once concluded

A design page lives under `plans/designs/`, in whatever subdirectory the project likes. Its `Cites:` names what it rests on, at blobs where the wording matters; its `Governs:` names the goals it governs, which the resolver labels *declared by the design, not yet by the goals* until `DefinitionRefs` exists (gate 5, master `:509`). Critique rounds, dispositions and reviews are records and go where they go today.

Three independent properties: **readiness** (`draft`, `proposed`), **applicability** (none, accepted at a blob, withdrawn, superseded), **completion** (live under `plans/designs/`, concluded under `records/designs/`). Concluding is a landing that deletes the file at one home and adds it at the other, paired by id and identical bytes; under `records/` it is append-only like everything there (`memory-architecture-design.md:46`), head included; a later act may still supersede or withdraw a concluded design, because acts touch the rulings register and not the page. An accepted design whose goals were never opened stays live (master `:490`).

### 6.4 Rulings — `memory/rulings.md`, unchanged in shape

The register keeps its grammar, ids, minting law, append-only carriage, union merge and `--approved-ref` consumers. New rulings that exercise authority carry acts (5.5) and are published by the verb (section 10), the only writer of a `terminal` basis. Existing rulings carry no acts and grant no acceptance to any page. One file, one id space, one lookup path through the resolver; whether it is ever split is a question for measured pressure.

### 6.5 Open questions — `memory/questions.md`

Seeded empty by adoption. Inception's unresolved items are the first rows. A design page's `## Open questions` section lists ids and never hosts the only copy (`rulings.md:21-22`).

### 6.6 Facts — `memory/facts.md`, for facts kept beyond the page that uses them

Most facts live where they are used — a design's "existing code" table, a decision's context, the counselor's specimen facts inside the record they support — pinned by that page's blob. The register holds the facts chosen to outlive one page, with a locator that says whether it is a retained observation or a recipe. A fact meant to be portable binds retained bytes (`src:`, `commit:`, `record:`); a fact anchored under `artifacts/` is *not available here* on another machine and says so. A fact carried over from the counselor keeps its citation as the triple, `cite:<kind>:<target>#<detail>`, reversibly; a legacy citation without a blob is *unpinned* and shown as such; nothing is flattened and no retention is invented.

### 6.7 Proposals — `memory/proposals.md`

Seeded empty by adoption; the P-section of `memory/backlog-notes.md` and `memory/proposal-drafts.md` fold into it. A proposal's `disposition` is set by the human who takes it up, through the boundary of section 10, naming the goal, draft or design; `goal open` records nothing about proposals (`goalsync_mutations.go:1014`, `:1760`) and this design does not promise that it will. A citation from a design that argues against a proposal changes nothing.

### 6.8 Principals — `memory/principals.md`

Seeded at adoption with the handle each seat's configuration names. One row per handle.

## 7. Standing: three properties, read together

| Property | Where it lives | Who changes it | Values |
| --- | --- | --- | --- |
| Readiness | the page's `Readiness:` | the author, by an ordinary edit | draft, proposed |
| Applicability | the authority history, folded per target: the effective act is the latest act on the target, in register order, that no correction has voided | a human, by a proven act | none; accepted at blob B; withdrawn; superseded by S; retired |
| Completion | the page's home | the seat or human landing the move | live, concluded (designs only) |

The interface and `project show` present the three side by side and one derived word for the common cases: *current* (intent or doctrine accepted and bytes equal the accepted blob), *accepted*, *revised* (accepted at an earlier blob; the accepted bytes remain the authority and are retrievable; the current bytes are `proposed`), *withdrawn*, *superseded*, *retired*, *concluded*. Beside each target the resolver shows its last-change token, which is what the next act must cite.

| Act | Who | Recorded as |
| --- | --- | --- |
| Create or revise a page; append a row; edit ordinary metadata | a human, a seat, the brain | a landing on the record's publication path (section 10) |
| Accept, withdraw, supersede, retire, correct, repair an identity, correct attribution | a human at the enrolled terminal | a ruling published by `project rule` |
| Resolve a question, dispose of a proposal | a human | a metadata edit published by `project set` |
| Record a human's word that decides nothing mechanical | a seat | a ruling with `basis=word` and no acts |
| Conclude a design | the seat landing the last governed goal, or the human | the paired move to `records/designs/` |

## 8. Identity and history

A record's identity is its `Id`. Its path is where it is kept today. Its `Revision` is a label that increments on every landed change to its bytes, enforced at landing on the default branch and provisional on a branch. The exact bytes at any landed revision are a Git blob, retrievable from the object store for as long as the repository exists; every act and every citation that must be exact names that blob, and the resolver answers `show <id>@blob:<sha1>` by `git cat-file`. The default branch is the canonical ancestry; a blob is landed when reachable from it; the check also accepts blobs present in the candidate it is validating.

## 9. Scale

| Mechanism | Comfortable at | Limit | Valve |
| --- | --- | --- | --- |
| Pages, one file each | thousands per home | a directory a human cannot scan | subdirectories at will; the live/concluded split; navigation by area |
| Books | one page; tens of chapters | one page revised as a whole | chapters with their own id, revision and acceptance |
| Areas | one, `project` | reorganisation | immutable slugs, mutable names, parents; area is a field, so a split is a metadata edit |
| Registers | hundreds of rows a year; 154 rulings in four weeks | a file every reader parses whole | append + in-place metadata; union merge with duplicate-id refusal and human reconciliation; a register may be a directory of files |
| The facts register | human pace | machine pace | the sitting store holds working facts; pages hold the facts they use |
| The rulings register | one file, fixed path (`dispatch/slice.go:111`, `landing/observe.go:1278`) | many decision-makers | one lookup path now; measured before any split |
| Authority | — | reversals, years, concurrent deciders | append-only acts with `given` against a last-change token; whole-ruling corrections; the fold is per target |
| Ids | — | many seats, many worktrees | ULIDs with duplicate refusal and an explicit repair; full ids persisted |
| `project check` | thousands of pages | — | reads whole files; builds id, act and reference maps once per run; hashes only what changed |
| Many humans deciding | one human per seat (master `:65`) | who may accept what, where | handles derived from enrolment; area owners recorded; enforcement of *who may* is the authority owner's; an old act is judged by its own basis |
| Finding things by meaning | a tree a newcomer can walk | "the decision about async refunds" | a rebuildable full-text index; never a source (master `:767`) |
| The Project section | — | 500 designs on one screen | the pane's design: area tree, lists by property, filters, paging |

## 10. The resolver, the check, the two verbs, and the three publication paths

**`internal/project`** owns the grammar: parse pages, indexes and registers; mint ids; resolve references and locators; fold acts per target; list by kind, area and property; answer what cites and what affects a record; retrieve bytes by blob; check a tree and a candidate against its base. It imports `stateroot`, `covenant`, `evidencetable` and `humanauthority`, nothing of the UI; `internal/ui/project` becomes a thin adapter and the catalogue table is deleted.

**Read verbs.** `project show <ref>`; `project list <kind> [--area] [--readiness] [--applicability]`; `project tree`; `project check [--base <tree>] [--candidate <tree>] [--quarantine]`.

**Two verbs of authority, one boundary.** `project rule --reason "…" <acts>` publishes acts; `project set <row-id> <cell> <value>` publishes a human-only metadata edit (a question's resolution, a proposal's disposition, a contributor correction). Each is one transaction in one process:

1. **Prove.** `humanauthority.Prove` and `ValidFor` — the enrolled-terminal walk (`authority.go:813`, `:194-195`); not `ProveTerminal`, which performs no enrolment lookup (`:633-637`), and not the goal verbs' `ProveOrTemporaryGoalAuthority`, whose relayed-word fallback (`:371`, `goalsync_mutations.go:2237`) this design does not adopt. The proof is consumed here and nowhere else.
2. **Bind the principal.** The deciding handle is the one the enrolled seat's configuration names; there is no `--by` (the goal verbs' `--by` skips derivation, `goalsync_mutations.go:1379-1382`, and this verb has no such flag).
3. **Compose.** The ruling row (or the metadata edit) plus whatever pages the human is publishing in the same act, as one candidate over the complete checkout for the layout in question.
4. **Check** that exact candidate against the live tip: the three-tree check below.
5. **Publish** atomically on the record's publication path — the landing for P1 and P3, the checkout's default branch for P2 — or refuse and leave the tip untouched.

A row that looks like the verb's but was not published by it is indistinguishable by bytes, and the later structural check does not try: it trusts admitted history — what the verb published at the tip — and treats a `terminal` basis in a candidate that did not come through the verb as a refusal at step 4, where the proof is live. Nothing that runs later authenticates anything; it only reads what was admitted.

**The check's inputs** are the base the candidate was composed on, the candidate itself, and the publication tip. It verifies: grammar; unique ids across every home; book indexes; declared areas; every reference resolves, the availability states of 5.4 excepted and reported; every `accepts` blob is a revision of its target, landed or present in the candidate; every `doc:`/`src:` binding has provenance in a reachable tree or the candidate; every `given` equals the target's last-change token at the tip; corrections state expected outcomes that the fold reproduces; claim cells unchanged; header and cell counts intact; `Revision` incremented where bytes changed; `Contributors:` monotone modulo corrections; newly set resolutions valid. A target the candidate cannot see is the typed refusal `target-not-visible`. A refused candidate leaves the accepted state exactly as it was.

**Three publication paths.** The two supported layouts hold three kinds of material, and the landing treats them differently today (section 2's last paragraph):

| Path | Material | What must exist before pages are written there |
| --- | --- | --- |
| P1 — the installation's own records in the self-hosted checkout: `metasystem/docs/intent/`, `metasystem/docs/doctrine/`, `metasystem/docs/decisions/`, `metasystem/plans/designs/`, `metasystem/records/designs/`, `metasystem/memory/*.md` | in the landing's tree (`gittree.go:171-174`); `docs/` is `behavior` class and today has no record carriage at all (`path-classes.txt:17`, `observe.go:1062-1068`, `:190`); nested `plans/` files are frozen on revision (`:1167-1188`, `:1230`); `records/` is append-only (`:1198-1208`) | **candidate-wide, path-by-path admission** for the recognised memory surface — pages under the five homes and the registers — at the candidate-policy stage where classes are resolved (`:1045-1056`), not inside `recordCarriageError`: each changed path is admitted by a held goal that owns the change, or by the proven human's publication of that exact candidate through the verb; never by adjacency to a ruling in the same candidate. The archive move is admitted as a pair, delete and add, matched by id and identical bytes. A candidate mixing `docs/` pages with record paths is a `Goal-Unit` commit under the branch classes (`range.go:185`) and this design does not change that. The read admission takes a design at any home (`read_subject_compute.go:199`) |
| P2 — the host repository's own records in the self-hosted checkout: `plans/designs/` and `records/designs/` at the checkout root, Wido's placement of the interface design (master `:739`) | outside the landing's tree entirely (`landing_verbs.go:62`, `land.sh:12`, `gittree.go:250-253`); the interface designs have landed by plain Git; class unit to the branch classifier (`range.go:185`), which does not apply | **authority on P2 pages is published through the verb like everyone else's**, over the complete checkout as the candidate, so acts on host-root pages carry the same guarantee as P1. **Page edits themselves stay plain Git**, and the design says what that is worth: `project check` at the checkout root is advisory for them; a P2 page's revision is labelled *unverified publication* by the resolver until an act names its blob, because two humans can each check against one tip and combine commits unchecked. This is the one decision left to Wido (18.5), and placement is not read as permission for less |
| P3 — an adopted application's records: `<app>/docs/intent/`, …, `<app>/memory/*.md` | app-owned, `Outside` to the classes (`owner.go:117`, `pathclass.go:223`), refused by the carriage unless allowed (`observe.go:1096-1098`); running the landing at the application root does not help, because the landing takes its workspace directory as the installation for ownership and policy (`observe.go:1052`, `:1257`, `:1269`) and would then class the application's `memory/`, `plans/` and `records/` by the unvendored inventory (`owner.go:106`, `:143`) | the landing **carries the installation root separately from the application's candidate and state root**: policy manifests load from the installation, ownership resolves with the installation, and the candidate is the application's tree; then the same path-by-path admission as P1 applies to the recognised application records in both landing routes. **The oracle is unchanged.** Until this lands, an adopted application's records are created by adoption and inception, edited by plain Git, and checked advisorily — the P2 condition, and labelled as such |

`stateroot` gains `Docs` (→ `docs`), `Designs` (→ `plans/designs`), `ConcludedDesigns` (→ `records/designs`). The brain's own writers arrive with the master's gates: browser human authority at gate 3 (`:811`), shared working-material persistence at gate 5 (`:378`, `:813`); until then the human runs the two verbs at the terminal.

## 11. What changes in the memory-architecture design, and why

1. **`docs/` holds standing documents.** Intent, doctrine and decisions carry an id, a readiness and a revision; a proven act makes a revision current.
2. **Intent and doctrine are books;** `docs/app-doctrine.md` and the `Purpose:` line are replaced.
3. **`plans/designs/` and `records/designs/`** are the two homes of a design, resolved by id; the paired archive move changes no reference.
4. **`docs/decisions/` exists.** The ruling is memory, the page is documentation, the page never claims acceptance.
5. **Four registers join `memory/`:** questions, facts, proposals, principals — immutable claims, ordinary metadata, union-merged like the registers already there.
6. **A record declares its kind, id and areas.** Ownership stays the oracle's.
7. **Authority is an append-only history of proven, checked publications** in the rulings register; every other change is an edit; mistakes have explicit corrections.
8. **Adoption ships none of the MetaSystem's own subject material** (`adopt.sh:246`); the touch list and write-tracer fixture change with it (`memory-architecture-design.md:100-112`).
9. **`stateroot` gains `Docs`, `Designs`, `ConcludedDesigns`.**
10. **The landing gains candidate-wide admission for the memory surface (P1), a separate installation root (P3), and the read admission at every home;** P2 is named as what it is.
11. **The UI's catalogue is retired.**

Not changed: the oracle; `stateRoot()` routing; the four-tree placement rule; the rulings register's shape and ids; the goal ledger and its power of attorney; `records/` as append-only history; the path-class manifest.

## 12. The two supported layouts, and the one that is not

| Layout | State root | Records | Paths |
| --- | --- | --- | --- |
| Adopted, vendored (`<app>/metasystem/`) | `<app>/` | `<app>/docs/intent/`, `<app>/docs/doctrine/`, `<app>/docs/decisions/`, `<app>/plans/designs/`, `<app>/memory/*.md`, `<app>/records/designs/` | P3 |
| Self-hosted (template) | `metasystem/` | `metasystem/docs/intent/`, …, `metasystem/memory/*.md`; plus the checkout root's `plans/designs/` and `records/designs/`, read as a second design home, labelled by root | P1 and P2 |

**Adopted at the repository root is not supported.** There the oracle classes `memory/`, `plans/` and `records/` as MetaSystem-generic (`owner.go:143`) — the interleaving R-9 rejects (`rulings.md:34`); making it hold is a change to the oracle's unvendored answer, a prerequisite goal of its own. Until then such an application sees the Project section read-only, with the layout named as unsupported (18.3).

## 13. Adoption and inception

Adoption seeds the four registers with their lead paragraphs and headers, principals with the seat's handle, adds their `merge=union` lines, creates nothing under `docs/` or `plans/designs/`, excludes the kit's subject material from the payload, and corrects `plans/README.md`. Inception writes the intent index with its areas and chapters, the doctrine index binding whatever the project already has, the interview's unresolved items as questions, the principals it met, and, on the human's confirmation in step 8, runs `project rule` at the human's terminal with one act per written page, over the same candidate. `docs/app-doctrine.md` becomes `docs/doctrine/index.md` in the skill and the fixture.

## 14. The Project section over this

- **The tree first.** `project tree`: the declared areas, nested by parent, with counts by kind and property; a breadcrumb; every node and record addressed by id.
- **Intent** and **Architecture**: the books, chapters in reading order, bound documents opening in the reading view with their source availability, each page with its three properties, its last-change token and the ruling that governs it; the decisions that affect the doctrine, derived.
- **Designs**: by area and property, with what each cites and governs and how sure the resolver is of the latter; P2 pages labelled when unverified.
- **Constraints and assurance**: unchanged.
- **Open questions**: by area, unresolved and reopened first, each with what resolved it.
- **Sittings**: gate 3.
- Facts and proposals beside what they are about; proposals also in the Backlog's Draft lane.
- Everything else that is Markdown is browsable by path as `doc:`, with no subsection claimed.
- **The brain reads the same tree** through the same verbs. Search is a rebuildable index, never a record.

## 15. What this deliberately does not do

No database and no index that correctness depends on. No new tree and no new top-level directory. No front matter, YAML or JSON a human hand-edits. No ownership tags. No new roles. No hand-maintained list of anything. No History block and no `Accepted:` line on pages. No acts on rows. No `reopens`, no `attorney:` basis, no correction of corrections. No prescribed sharding. No retrospective typing of the kit's 102 historical designs: the live ones get heads, the rest stay browsable as `doc:`. Not the sitting store (gate 3). Not `DefinitionRefs` (gate 5), though section 5.4 is its grammar. Not the oracle's unvendored answer. Not accounts and sign-in; only handles. Not team membership; only team handles. Not a tree-only check that pretends to authenticate a writer.

## 16. Acceptance

**Parser and derivation tests** — `internal/project`, temporary repositories, no landing: bytes at three revisions retrieved by blob after rename, move and deletion; a one-page book becoming chapters; a bound chapter whose source moves, with provenance still verified in history; an area split by metadata edits and a reparented child; a principal's name change; the newcomer's three questions answered by `list`, `show` and `tree` from a fixture with three areas; the fold under accept–withdraw–correct with the last-change token; the refusals — two files with one id, an edited id, a copied page, an undeclared area, a chapter of another kind, a bound path that leaves the checkout by `..` or symlink, a bound blob that never sat at its path, a relabelled header, a wrong cell count, a claim cell changed, a `Contributors:` line that shrinks without a correction, a packet with an unescaped `]` in a quoted value, a `word` ruling carrying an act, an act-shaped phrase after `]` treated as prose.

**Publication tests** — the real publisher, `project rule` and `project set` driving the landing on P1 and P3 fixtures and the checkout on a P2 fixture, never a simulated check: inception's pages and acceptance in one candidate, accepted; a stale `given`, refused, the accepted state unchanged; two candidates superseding one decision by different successors, the second refused; an accepted page edited, shown *revised*; a whole-ruling correction with expected outcomes the fold reproduces, and one whose expectations it does not, refused; a correction of a correction, refused; a malformed packet quarantined and then corrected; a duplicate identity that has landed, repaired with rebinding and no re-mint; the archive move with identical bytes, accepted, with changed bytes or an unpaired half, refused; a `terminal` basis in a candidate that did not come through the verb, refused; a ruling naming a host-root page from an installation-scoped candidate, refused as `target-not-visible`, and the same acts through the verb over the checkout, accepted; a P3 candidate with the installation root carried separately, accepted, and without it, refused with the rule named; a question's stale resolution surviving the withdrawal of its decision and shown *reopened*; a contributor correction admitted and its target excluded from the monotone rule.

**Human walkthroughs** — the pane's slice, in a browser, Claude then Wido: the newcomer's three clicks and three questions to the brain; the handoff test on a second checkout with exactly the availability states of 5.4 reported unavailable and nothing else missing.

## 17. Slices

Each at most four hours (R-15, `rulings.md:40`) or flagged; none starts before Wido's word on section 18.

1. **Grammar, resolver, derivation.** `internal/project` read-only: pages, books, registers, references and locators, the packet grammar parsed, the fold with tokens and corrections, areas, principals, provenance checks, `show`, `list`, `tree`, `check` without `--base`; `stateroot`'s three kinds; the parser and derivation tests. ~4h.
2. **The boundary.** `project rule` and `project set`: enrolled proof, handle from the seat, composition, the three-tree check with `--base --candidate`, atomic publication on the checkout (P2 form first, because it needs no landing change); the publication tests that need no carriage change. Touches `internal/humanauthority` (tier-1 floor, `path-classes.txt:75`). **Flagged over four hours**: the proof reuse is available (`authority.go:813` has no verb whitelist) but publication binding is new; estimate ~6h, split as verb-and-check then publication if it runs long. On Wido's word.
3. **P1 admission.** Candidate-wide, path-by-path admission for the memory surface at the class-resolution stage; the paired archive move; the read admission at every home; the verb publishing through the landing. `internal/landing`, `internal/dispatch`, `internal/goal/branch` (tier-1). ~4h, on Wido's word.
4. **P3 admission.** The separate installation root in the landing; the same admission for application records; the P3 fixtures. ~4h, on Wido's word.
5. **Adoption and inception.** Registers, principals, merge attributes, payload exclusion, touch list, `plans/README.md`, the rename, inception's writes and its one ruling; the adoption fixture. ~3h.
6. **The kit's live record, mechanical part.** Heads on the live designs (kit and root); the doctrine index binding the architecture documents; principals from the rulings register's owners; proposals folded. ~3h. **The intent harvest** is prepared by the author and adjudicated by Wido; it lands when he says it is right.
7. **The Project section**, `g1-s21` revision 3, over the resolver; the human walkthroughs. Designed separately.
8. **The oracle's unvendored answer** — a goal of its own.
9. **Brain writers and sitting persistence** — gates 3 and 5, in the master's order.

## 18. Decisions for Wido

1. **Metadata changes in place under Git history; authority acts and claims are immutable, with explicit corrections.** Recommended, with Astra: yes.
2. **Acts of authority go through the enrolled terminal now**, via `project rule` and `project set`, with the enrolled proof and a checked publication; no relayed word, no asserted name. Recommended, with Astra: yes. Until gate 3 you run the verbs yourself.
3. **Root-installed adopters are read-only until the oracle changes.** Recommended, with Astra: yes, as a supported-scope decision.
4. **ULIDs, full ids in durable references, titles in views.** Engineering, not a ruling, unless you object to reading them; "humans never read ids" is an aspiration, not a guarantee.
5. **The host-root design home.** Astra's line, which the author now recommends: *"Keep host-root designs where they are; authoritative publication must use the same checked boundary as other project records."* Page edits there stay plain Git and are labelled unverified until an act names them. The alternative — the weaker guarantee for those pages — is yours to accept, not the design's to assume.
6. **The kit's 102 historical designs stay untyped** and browsable; only live designs get heads. Recommended: yes.
7. **The intent harvest is yours to adjudicate**, prepared by the author. Recommended: schedule it as a sitting, not a build.

## 19. Dispositions of Astra's round 3, and closure

Verdict *sound after the listed changes*, twelve changes, eleven findings, seven corrections; "with these changes adopted, the design is the design of record; the next arbiter is the implementation fixtures and code review. I am not requesting another general prose critique." Every code claim adopted was re-read in source. All twelve changes are made:

| Change | Finding | What changed |
| --- | --- | --- |
| 1 | C1 | The verb is the publication boundary: it consumes its own live proof, derives the handle from enrolment (no `--by`), composes, checks and publishes in one process; `project set` routes human-only metadata through the same boundary; a later check trusts admitted history and never a textual basis (3, 5.5, 10). |
| 2 | C2 | P1: candidate-wide, path-by-path admission at the class-resolution stage, `docs/` included; never adjacency; paired archive move; mixed-candidate commit kind stated (10). |
| 3 | C3 | P3: the installation root carried separately from the application's candidate and state root; policy and ownership from the installation; oracle unchanged (10). |
| 4 | C4 | P2: authority published through the boundary over the complete checkout; page edits named unverified; "your bypass" withdrawn; the decision put to Wido as Astra phrased it (10, 18.5). |
| 5 | C5 | `attorney:` removed from this delivery; the grant's real shape recorded (2, 5.5). |
| 6 | C6 | Corrections are prospective and whole-ruling with expected outcomes; the last-change token is distinct from the effective act; no correction of corrections; `reopens` removed; quoting and escaping specified; `--quarantine` (5.5, 10). |
| 7 | C7 | `repairs-identity` with explicit keep/retire by path and blob and explicit rebinding (5.1, 5.5). |
| 8 | C8 | Resolutions validated when set or changed, retained afterwards, *reopened* derived; decision answers bound at the accepted blob (5.3, 7). |
| 9 | C9 | Bindings checked for provenance in a reachable or candidate tree, entry type included; bytes still resolved by blob (5.2, 5.4). |
| 10 | C10 | `cite:<kind>:<target>#<detail>` carries the triple reversibly; *unpinned* legacy state; the checker's exemptions and the remote walkthrough name the same availability states (5.4, 6.6, 16). |
| 11 | C11 | `Contributors-corrected:` by a human through the boundary, preserving history; independence still conservative (5.1, 5.7). |
| 12 | — | A fixture per correction; publication tests drive the real publisher; slice 2 flagged and split; brain writers placed at the master's gates (16, 17, 10). |

Corrections, all taken, in section 2 and section 10: `Prove` with `ValidFor`, not `ProveTerminal`; `goal approve`'s relayed-word fallback not adopted; `--by` not offered; grants do not cover memory acts; `docs/` has no record carriage; P3 needs the installation root; shared brain writers at gate 5. R-11 cuts taken: attorney, `reopens`, recursive invalidation, any claim that a tree-only check authenticates, separate guarantees for identical records by path. Kept, as Astra advised: the editable tables.

**Closure.** Under Wido's criterion — the author and Astra agree when the remaining differences have no impact on the implementation — the loop is closed. The one open item is a policy decision, 18.5, on which the author now recommends Astra's line; the rest of section 18 records Wido's decisions with both reviewers' recommendations attached. What remains after his word is the fixtures and the code review of each slice, which is where Astra asked to meet next.
