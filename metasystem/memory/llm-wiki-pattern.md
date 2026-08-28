# LLM Wiki Pattern Evaluation

- Owner: current Codex session
- Status: critique closed in round 6; parked for human review until the lifecycle checkpoint below
- Review-by: 2026-09-03 or the next metasystem retro, whichever occurs first
- Goal: determine what Andrej Karpathy's "LLM Wiki" pattern is, whether it solves a current metasystem problem, and, only if the evidence supports it, design the smallest robust integration for later implementation approval.

## Decision to Make

Should this metasystem gain an LLM-oriented wiki/knowledge mechanism? A positive decision requires all of the following:

1. A primary-source description of the pattern and its intended benefits.
2. A named current metasystem consumer and workflow that existing owners do not already serve.
3. Clear ownership, lifecycle, retrieval, provenance, freshness, and failure contracts.
4. Benefits that outweigh context cost, stale-knowledge risk, duplicated policy, maintenance burden, and added ceremony.
5. Focused tests or checks capable of proving the useful behavior without relying on model self-judgment alone.

## Scope

- Research Karpathy's own writing, talks, repositories, or demonstrations first; use secondary accounts only to locate or contrast primary evidence.
- Trace the current metasystem's instruction routing, task-local plans, durable documentation, agent mission state, receipts/retrospectives, and validation paths.
- Compare three outcomes: no change, documentation/workflow reconciliation only, or a new executable mechanism.
- If implementation is justified, keep it local to one canonical owner and avoid a parallel source of policy truth.

## Non-goals

- Building a general-purpose RAG platform, vector database, or autonomous documentation system without a demonstrated metasystem consumer.
- Treating generated summaries as authoritative truth without source provenance and human-owned correction paths.
- Importing Karpathy's implementation shape verbatim when the metasystem has different ownership or lifecycle constraints.
- Adding dependencies or changing externally consumed schemas without explicit approval.

## Work Plan

### 1. Establish the reference pattern

- Locate the primary source(s) where Karpathy describes or demonstrates the LLM Wiki pattern.
- Record the pattern's problem statement, information flow, persistence model, retrieval behavior, update loop, trust model, and claimed results.
- Separate direct evidence from interpretation and later community extrapolation.

### 2. Trace the current metasystem

- Map canonical documentation and instruction routing from `AGENTS.md` and `wow.md`.
- Trace task-local and durable knowledge through `plans/`, receipts, retrospectives, meta documentation, and agent mission artifacts.
- Identify concrete failure modes a wiki could address, plus existing mechanisms that already address them.
- Preserve concurrent user/peer modifications outside this plan. Re-check the worktree before every later implementation step and do not fold unrelated changes into this stream.

### 3. Evaluate fit

- Score candidate use cases on consumer, frequency, consequence, evidence quality, existing owner, and lifecycle cost.
- Test whether simpler reconciliation, indexing, or deletion solves the same problem.
- Produce a verdict with evidence levels: observed by running, checked by reading, or inferred.

### 4. Design only if justified

- Name the mechanism's single owner and explicit non-responsibilities.
- Define source format, stable identity, provenance, freshness, conflict handling, bounded retrieval, update/promotion flow, and deletion/archival.
- Define how policy remains canonical in existing owners while wiki material remains evidence or orientation.
- Define deterministic validation, diagnostics, security boundaries, migration, rollback, and cleanup.
- Run the design-obligation gate before implementation.

### 5. Implement and verify only if the gate passes

- After an activation trigger and evaluation approval, implement the smallest vertical slice serving the named corpus and workflow.
- Add focused contract tests and end-to-end verification for its runnable surface.
- Update canonical documentation, append a receipt, and remove this task-local plan when all durable content has moved to its final owner.

## Evidence Log

| Evidence | Level | Consequence |
| --- | --- | --- |
| `AGENTS.md`, `wow.md`, `plans/README.md`, `docs/project-rules.md`, `docs/collaboration.md`, and `docs/design/design-principles.md` | Checked by reading | The metasystem already distinguishes canonical policy, task-local evidence, and durable ledgers; any wiki must respect those ownership boundaries. |
| The concurrent worktree and `HEAD` changed repeatedly during research as validation and orchestrator work landed; by the final verification checkpoint only this plan and its required task receipt were modified | Observed with repeated `git status`, `git diff`, and `git rev-parse` | The peer stream resolved without this task absorbing its files. Any later implementation must still re-baseline and treat unrelated changes as user/peer-owned. |
| Karpathy's primary `llm-wiki.md` gist, created 2026-04-04 | Checked by reading | The pattern is an intentionally abstract research knowledge-base workflow, not a specific agent-memory implementation. |
| WiCER, arXiv:2605.07068 | Checked by reading; preprint | Blind LLM compilation can discard decisive facts. Evaluation-driven refinement recovered much of the loss in its benchmark, so source retention and diagnostic probes are design requirements rather than optional polish. |
| Vector RAG vs LLM-Compiled Wiki, arXiv:2605.18490 | Checked by reading; small preregistered preprint | On 24 papers and 13 questions, the wiki improved cross-paper connection and claim-level citation support, but used more query tokens. Decomposition-based RAG recovered most synthesis benefit at lower LLM-token cost. There is no universal cost or quality win. |
| Repository word counts on 2026-08-04 | Observed by running `wc` | `plans/` and its ledgers contain about 51,000 words, while `development/source-analysis.md` contains 2,758. Automatically compiling all plans would amplify the metasystem's recorded evidence-substitution risk. |
| Git history since 2026-07 | Observed by running `git log --numstat` | This repository repeatedly performs source harvest and critique work, and its durable outcome is manually reconciled into `development/source-analysis.md`. History does not establish a repeated query workflow or a provenance-caused defect, so demand for a reusable wiki remains unproven. |

## Primary Research

### Karpathy's pattern

Primary source: [Andrej Karpathy, "LLM Wiki"](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f), created 2026-04-04.

Karpathy defines three layers:

1. Immutable raw sources are the source of truth.
2. An LLM-owned Markdown wiki integrates summaries, entities, concepts, comparisons, links, and contradictions.
3. A schema or agent-instruction document defines structure and workflows.

He defines three main operations:

- Ingest reads one or more new sources, discusses them with the human, and updates every affected wiki page.
- Query starts from the index, reads relevant pages, answers with citations, and may file useful synthesis back into the wiki.
- Lint looks for contradictions, stale claims, missing links, orphans, gaps, and missing concepts.

`index.md` is a content catalog and `log.md` is a chronological operation history. At his reported moderate scale of roughly 100 sources and hundreds of pages, a Markdown index is sufficient; he recommends search tooling only when actual scale demands it. The gist leaves directory structure, schemas, tools, and output formats deliberately open.

The useful principle is compile-once, refine-over-time synthesis. The risky literalism is the claim that the LLM owns the wiki entirely and that query outputs can be filed back as knowledge. This metasystem has a stricter truth rule: model output cannot become reference truth without direct evidence or human instruction.

### Evidence after the gist

- [WiCER](https://arxiv.org/abs/2605.07068) tested compilation over 17 RepLiQA domains. Its closest result for this design is negative: blind compilation scored far below raw full-context and suffered a 53 to 60 percent catastrophic-failure rate. Diagnostic probes and targeted refinement recovered 80 percent of lost quality in the reported setup. The paper equates LLM Wiki with a cached full-context serving architecture, so its latency numbers do not transfer directly to a Markdown-and-`rg` metasystem. Its compilation-loss result does transfer.
- [Vector RAG vs LLM-Compiled Wiki](https://arxiv.org/abs/2605.18490) compared a compiled Markdown wiki with single-round vector RAG over a small research corpus. The wiki connected findings across papers better and had stronger claim-level citation support, while RAG met the preregistered single-fact lookup test. The wiki used more query tokens, and decomposed RAG recovered most of its synthesis benefit. This supports a repeated synthesis use case, not a general replacement for retrieval.
- [Knowledge Compounding](https://arxiv.org/abs/2604.11243) reports large token savings over four sequential queries. Four queries are too few to establish a portable metasystem contract, so this is supporting context only.

These sources are recent preprints and project reports, not settled evidence. The design therefore keeps the mechanism optional and gives it a removal trigger.

## Current Metasystem Trace

| Current mechanism | Existing owner and contract | Relationship to an LLM wiki |
| --- | --- | --- |
| Always-loaded instructions and routing | `AGENTS.md:3`; `wow.md:3-29` | Already solved. A wiki must never become another instruction source or common-path context bundle. |
| Project facts and policy | `docs/project-rules.md`; canonical guidance in `docs/` | Already solved. Wiki content is derived evidence and has no normative authority. |
| Active task continuity | `plans/README.md:3,13-33` | Already solved. Handoff notes carry in-flight decisions and die when the stream ships. A wiki must not replace or auto-ingest them. |
| Durable instruction learning | `docs/project-adaptation.md:26-47`; `skills/retro/SKILL.md:10-52`; `memory/instruction-ledger.md` | Already solved. Receipts and retros measure repeated failures and human-gate instruction changes. A wiki must not bypass the change gate. |
| Durable dead ends | `memory/known-issues.md` | Already solved. A wiki can link to a known issue but cannot become a competing issue register. |
| Generated runtime evidence | `plans/README.md:3-9`; `artifacts/` and the durable evidence root | Already solved. Runtime artifacts remain evidence in their existing lifecycle. They are not bulk-ingested by default. |
| Template-maintenance research synthesis | Maintenance sessions that update `development/source-analysis.md:1-95`, especially its evidence list and keep/remove/defer decisions | Candidate consumer only. The output lacks claim-level provenance, but no receipt or observed failure shows that maintainers repeatedly query a stable corpus or that this gap caused rework. The artifact itself is not a consumer. |

The architecture already warns that plans become permanent prompt baggage when misused (`development/metasystem-architecture.md:19`) and requires a current consumer before adding support layers (`docs/design/design-principles.md:35-37`). The retro treats plan and ledger growth beyond shipped work as evidence substitution (`skills/retro/SKILL.md:27-31`). Those facts rule out an automatic repository-wide memory layer.

## Step-Back Analysis

### Frozen frame

- Symptom: cross-source template conclusions currently collapse into a single manually maintained `development/source-analysis.md`, with limited primary-source traceability and no deterministic health check. Repeated query demand and provenance-caused rework have not been observed.
- Impact: later template-maintenance sessions may have to re-establish which exact source supports a claim, whether the source changed, or whether a new source contradicts an older synthesis. The actual frequency and cost are unknown.
- Exact state at the final verification checkpoint: repository `0b9ca1b` with only this plan and its task receipt modified. Earlier checkpoints contained concurrent validation and orchestrator changes that landed independently during the research.
- Success: a bounded research corpus supports source-grounded ingest, navigation, query, and lint without becoming policy, project memory, or common-path prompt context.
- Non-goals: generic codebase memory, an autonomous documentation writer, RAG infrastructure, automatic ingestion of chat/plans/receipts, or policy promotion.
- Budget: research and design only in this task. No model-backed benchmark, dependency, or production implementation without a reviewed design.
- Cycle budget: 3

### Theories

| Id | Theory | Supporting evidence | Contradicting evidence | Status |
| --- | --- | --- | --- | --- |
| T1 | The metasystem needs a general persistent memory layer. | Session context dies; the README names forgotten lessons. | Handoffs, canonical docs, receipts, retros, the instruction ledger, and known issues already own each durable-memory class. A general layer would duplicate truth and load stale context. | Falsified dead end. Do not implement. |
| T2 | The pattern has no value because existing owners are complete. | Most proposed wiki jobs map directly to an existing owner. | Template-maintenance research has a real synthesis artifact but lacks claim-level source identity and deterministic integrity checks. | Falsified continue. Narrow the consumer. |
| T3 | An opt-in, source-grounded research workspace could add value after demand appears. | Candidate `development/source-analysis.md` maintenance workflow; empirical cross-document synthesis benefit; Git/Markdown fit; no retrieval dependency needed. | No repeated query baseline or provenance-caused defect; compilation loss, query-token cost, maintenance growth, and hallucination propagation. | Plausible but unproven. Park until the activation trigger fires. |
| T4 | The metasystem should ship vector RAG or a hosted knowledge service. | Could scale retrieval beyond a Markdown index. | No current scale problem, new dependencies and operations, weaker inspectability, and the small comparison found decomposition can close much of the synthesis gap. | Rejected until measured retrieval misses cross a declared threshold. |

### Alternatives

| Alternative | Decision | Reason |
| --- | --- | --- |
| No executable change now | Selected | A reusable mechanism would otherwise create its own pilot as its first consumer. Keep the reviewed design ready, then wait for observed demand or an explicit implementation request. |
| Add direct citations to `development/source-analysis.md` | Preferred immediate mitigation when the next source decision changes | The file remains the canonical template decision summary. Direct citations address provenance cheaply without adding state, scripts, or workflow. Repeated re-reading after that mitigation is evidence for reopening this design. |
| Use plans and handoffs as memory | Rejected | They are task-local by contract and should be deleted or archived after promotion. |
| Auto-compile all repository files and Git history | Rejected | It creates stale duplication, expands process records, and has no bounded source-selection contract. |
| Always load a wiki index | Rejected | Every task would pay context cost for a specialist concern. |
| Agent-maintained `index.md` and `log.md` | Reconciled | Build the index deterministically from page metadata. Reuse Git history and receipts for chronology. |
| File query answers back automatically | Rejected | Read-only questions do not authorize repository edits, and model output is not reference truth. Writeback requires an explicit repository-edit request and retains direct source citations. |
| Add embeddings, a database, MCP, or Obsidian | Rejected for the first slice | No measured need or current consumer justifies the dependency and operational surface. The deterministic literal selector is sufficient for a falsifiable pilot. |

## Verdict

The pattern has plausible value for repeated cross-source research, but the current metasystem does not yet show enough demand to justify executable integration. The decision for this task is **design and park**: retain an opt-in `research-wiki` design in this task plan for bounded human review, add no skill or script now, and reopen implementation only when one of these triggers fires:

1. Two completed research tasks against the same corpus record repeated source re-reading or a provenance-caused correction in receipts or handoff evidence.
2. A human explicitly requests implementation for a named corpus and accepts the pilot's evaluation and spend contract.

When reopened, the mechanism belongs under `optional-skills/`. Its first pilot candidate is template-maintenance research feeding `development/source-analysis.md`. It is not project memory, an instruction layer, or a RAG replacement.

### Parked-plan lifecycle

This file remains task-local evidence, not a standing design owner. It is waiting on one human decision: accept the parked verdict, request implementation for a named corpus under the evaluation contract, or reject the mechanism. If implementation is not activated by 2026-09-03 or the next metasystem retro, whichever comes first, the retro owner records a compact `defer` decision with the activation triggers and primary research links in `development/source-analysis.md`, then deletes this plan. A later trigger starts a fresh task from that durable decision and revalidates the design against the current metasystem. If implementation is activated sooner, this plan becomes the active implementation plan and is deleted after the accepted contract moves into the optional skill and canonical project guidance. An explicit rejection immediately records `reject` and the human's rationale in `development/source-analysis.md`, then deletes this plan; only an explicit human reversal or materially new evidence may reopen it.

The design intentionally adopts Karpathy's immutable-source, compiled-wiki, ingest/query/lint shape while rejecting three parts that conflict with this metasystem:

1. The LLM does not own truth. It owns derived claim blocks that cite immutable sources directly.
2. Query does not write by default. Repository writes require explicit authorization.
3. Index and operation history are not maintained semantically by the LLM. The index is rebuilt deterministically, while Git and receipts remain the chronological owners.

## Parked Mechanism: `research-wiki`

### Activation and placement

- Ship as `optional-skills/research-wiki/` with no third-party dependencies.
- Enable it only when a repository has a named repeated research corpus. Each corpus identifies itself with its own `schema.json`; commands take an explicit `--root`, so no new global configuration key is required.
- The template repository's first pilot may use `meta/research/llm-wiki/`; adopters choose a project-owned path. `meta/` remains template-only.
- Do not add behavior to `AGENTS.md`. `wow.md` already routes opt-in specialists through `optional-skills/`; an enabled skill is registered through the existing adaptation flow.
- In the template checkout, the pilot invokes `optional-skills/research-wiki/` in place; `adopt.sh` is not an in-place feature installer.
- For a fresh adopter, the existing `scripts/adopt.sh --enable research-wiki` path copies the skill and registers its metadata without any corpus or generated content. An existing adopter uses the repository's reconciliation/upgrade flow. Corpus initialization is always an explicit later operation.
- No new dispatcher role or permission profile is introduced. Read-only query behavior follows the existing action-matching contract in `AGENTS.md:8`; it is not presented as a hard sandbox guarantee.

### Ownership

| Responsibility | Owner |
| --- | --- |
| Workflow, source selection rules, query/write boundary, and promotion boundary | `skills/research-wiki/SKILL.md` after enablement |
| Corpus-specific page types, tags, and root | `<research.root>/schema.json` |
| Immutable evidence bytes and source identity | `<research.root>/sources/` plus `skills/research-wiki/scripts/research-wiki.py` after enablement |
| Derived synthesis | `<research.root>/wiki/*.md` |
| Deterministic navigation | Generated `<research.root>/index.md`, owned by `skills/research-wiki/scripts/research-wiki.py` after enablement |
| Policy and project facts | Existing canonical owners routed by `wow.md`; never the wiki |
| Chronology and task audit | Git history and `memory/receipts.log`; no new operation log |
| Semantic review candidates | `artifacts/research-wiki/lint/`; never an automatic wiki mutation |

### On-disk contract

```text
<research.root>/
  schema.json
  sources/
    <sha256>/
      source.json
      payload
  wiki/
    <page-id>.md
  index.md
```

`source.json` records the content hash, origins, media type, capture time, and an optional copying note. The directory name and payload hash must match. A changed source is a new source id; old bytes are never overwritten. A remote source must first become a permitted local UTF-8 file through the runtime's existing web and authorization boundaries. Version 1 has no URL fetcher and no reference-only source record.

The first-slice `schema.json` shape is:

```json
{
  "kind": "research-wiki",
  "format_version": 1,
  "corpus_id": "lowercase-kebab-case",
  "allowed_tags": ["lowercase-kebab-case"],
  "query_limits": {"max_pages": 6, "max_source_bytes": 200000}
}
```

The first-slice `source.json` shape is:

```json
{
  "format_version": 1,
  "sha256": "64-lowercase-hex-characters",
  "media_type": "text/plain; charset=utf-8",
  "origins": [
    {"kind": "file", "uri": "repository-relative-path", "captured_at": "RFC-3339 UTC"}
  ],
  "supersedes": [],
  "classification": "public",
  "copying_note": "optional human-supplied text"
}
```

Only a payload-bearing, human-classified public source is valid. Reference-only, internal, restricted, and unclassified records do not exist in version 1. Source identity is the exact payload bytes and therefore the full SHA-256. Capturing identical bytes again resolves to the same source and merges new origins by `(kind, uri)`, keeping the earliest capture time for a duplicate origin and sorting origins lexically. A changed byte sequence creates a new source directory. `copying_note` and origins are informational and changing them does not change evidence identity. The payload must decode as UTF-8; line anchors are one-based, inclusive ranges over Unicode lines as returned by Python `str.splitlines()`.

`supersedes` is a unique, lexically sorted array of existing source ids supplied explicitly during capture. It must form an acyclic graph. It means the newer source replaces the older source for current queries; `check` returns `stale` when a page cites a superseded source. Query stops on `stale` and requests an ingest update before answering. For an existing payload hash, capture atomically takes the set union of stored and newly supplied edges. Omitting a stored edge retains it; supplying only existing edges is idempotent. Version 1 has no removal or replacement operation. A nonexistent id, self-edge, or edge that would create a cycle rejects the entire capture and leaves origins and supersession metadata unchanged. Version 1 has no historical-claim exemption: a corpus needing historical use of an obsolete source must avoid the `supersedes` relationship or wait for a later claim-state design.

Every wiki page begins with one machine-readable HTML comment containing JSON metadata:

```markdown
<!-- research-wiki: {"format_version":1,"id":"llm-wiki-pattern","title":"LLM Wiki pattern","state":"derived","tags":["knowledge-systems"]} -->

# LLM Wiki pattern

Fact: Karpathy defines three layers. [S:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef:L10-L18]

Synthesis: The compiled layer helps repeated cross-source questions but remains vulnerable to source loss. [S:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef:L10-L18] [S:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789:L20-L29]

Uncertainty: The current metasystem does not yet show repeated query demand. [S:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789:L30-L33]
```

Version 1 requires metadata keys exactly `format_version`, `id`, `title`, `state`, and `tags`; `state` is exactly `derived`. It supports headings and blank-line-separated claim paragraphs only. Each claim paragraph is one physical line, begins with exactly `Fact:`, `Synthesis:`, or `Uncertainty:`, and ends with one or more direct anchors matching `[S:<full-sha256>:L<start>-L<end>]`. Lists, tables, block quotes, code blocks, images, and untyped prose are rejected. Wiki links use `[[lowercase-kebab-page-id]]`, may appear inside a claim, and never satisfy provenance. Page ids match `^[a-z0-9]+(?:-[a-z0-9]+)*$`; the filename must be `<id>.md`. `title` is a non-empty single line without `|`; `tags` must be unique, sorted, and declared in `schema.json`. The index derives title, tags, claim count, and distinct source count; page metadata does not duplicate a semantic summary or source set.

The claim kind tells query readers how to report the content; it does not certify semantic support. Deterministic lint proves that anchors exist and ranges resolve. Human review or an evaluation probe decides whether cited lines actually support a claim.

`rebuild-index` sorts pages by bytewise UTF-8 page id and writes LF-terminated UTF-8. Each entry has the exact shape ``- `<id>` | <title> | tags=<comma-separated tags or -> | claims=<integer> | sources=<integer>``. The file begins with `# Research Wiki Index` and one blank line. A title may not contain a newline or `|`. This makes a clean rebuild byte-reproducible without asking the LLM to summarize pages again.

### Operations

#### `init`

`skills/research-wiki/scripts/research-wiki.py init --root <root> --corpus-id <id> --tag <tag>...` is the only operation allowed to create a corpus root. It resolves the Git top level first. The requested root must be repository-relative, must not exist, and must have an existing parent whose real path is inside the Git worktree. The parent and every path component must be a real directory rather than a symlink and must not be ignored by Git. Initialization creates `schema.json`, empty `sources/` and `wiki/` directories, and a clean generated index through a temporary sibling followed by one atomic rename.

Every later command resolves the root with `realpath`, requires it to remain inside the same Git worktree, rejects any symlinked path component, requires a valid `schema.json`, and refuses an ignored root. An outside, ignored, missing, or symlinked root is `invalid` before any read or write. The outside-path fixtures cover both `--root` and `--file` independently.

#### `capture`

`skills/research-wiki/scripts/research-wiki.py capture --root <root> --file <path> --classification public [--supersedes <source-id>]... [--copying-note <text>]` copies allowed source bytes into a content-addressed directory, writes metadata atomically, and is idempotent for the same bytes and origin. The skill obtains explicit human confirmation that the selected source is public and may be copied before passing `--classification public`; it never infers that classification. The script refuses any other or missing classification, malformed existing metadata, a payload whose hash disagrees with its directory, an ignored or outside-repository input path, a symlinked input or path component, any input inside the corpus root, and non-UTF-8 content. The inside-root refusal prevents derived wiki text or captured payloads from being recaptured as raw evidence. Binary files, PDFs, images, archives, and non-public material remain unsupported until a later design names their storage and review contract.

Failure categories are explicit: invalid input, disallowed/outside path, unsupported source, hash conflict, and I/O failure. Partial directories are written in a temporary sibling and renamed only after validation. `check --fresh` hashes every existing origin file for sources that are not transitively superseded. Changed bytes return `stale-origin` and block query until the changed bytes are captured as a new source that supersedes the prior current source; a missing origin produces `origin-missing` as a warning because the captured payload remains the durable evidence. Transitively superseded sources still receive payload, metadata, graph, and citation-integrity checks, but their mutable origins are not freshness blockers. A page citation to any superseded source remains `stale` until ingest moves the claim to current evidence.

#### `ingest`

The skill, not the script, owns semantic compilation:

1. Check the corpus before editing.
2. Read one selected source and the deterministic index.
3. Select only affected wiki pages using metadata and `rg`.
4. Update or create derived pages with direct source anchors.
5. Run `rebuild-index` and `check`.
6. Present the source, claims, contradictions, and file diff for human review.

Batch ingest is outside the first slice. A failed ingest leaves source capture intact and synthesis changes reviewable in Git; it does not silently roll back or mark them verified.

#### `query`

1. Run `check --fresh --quiet`; stop on source-hash, index-integrity, superseded-citation, or changed-origin failure.
2. Pass the unchanged question to the deterministic `select` operation described below.
3. Read only the selected claims and their admitted full source payloads.
4. Answer with the same direct `[S:<full-sha256>:L<start>-L<end>]` anchors and distinguish checked `Fact`, derived `Synthesis`, and `Uncertainty` claims.
5. Return `partial` and the complete omitted-candidate list whenever a page or claim budget excludes evidence.
6. Make no repository edit unless the user explicitly requests writeback.

Query has deterministic `max_pages` and `max_source_bytes` budgets in `schema.json`. Hitting either returns a partial-result outcome naming omitted candidates; it does not silently widen scope or add a retrieval service. Provider token usage is measured after a query for evaluation, not used as a preflight bound that would require a model-specific tokenizer.

`select --root <root> --question <text>` uses this version-1 algorithm:

1. Unicode-casefold the question, extract ASCII tokens matching `[a-z0-9][a-z0-9-]{2,}`, deduplicate them, sort lexically, and keep the first twelve. No model-generated synonyms or retries enter selection.
2. Tokenize each page id, tag, title, and claim text the same way. For each distinct query token, add 3 for an exact id or tag token, 2 for an exact title token, and 1 for an exact claim token. A token scores at most once in each category.
3. Candidate pages have score greater than zero. Sort by descending score, then bytewise UTF-8 page id. Admit the first `max_pages`; report every remaining candidate page id and score as omitted.
4. Walk admitted pages in rank order and their claims in file order. Source accounting uses each complete UTF-8 payload byte length once per distinct source id, never cited-range length. Admit a claim only if adding all of its not-yet-counted sources stays at or below `max_source_bytes`. Otherwise omit that claim, report its page id, one-based claim ordinal, and required source ids, then continue deterministically to later claims.
5. Emit the normalized tokens, scored page list, admitted pages/claims/sources, omitted pages/claims, byte total, and `ok` or `partial` as JSON. Zero matching pages is `partial` with reason `no-literal-match`; version 1 never widens the terms itself.

#### `check`

`check` is the deterministic validation command. It checks:

- source directory and payload hash match;
- source classifications equal `public`, `supersedes` ids resolve, and the supersession graph is acyclic;
- every page id is unique and filename-stable;
- metadata parses and required fields exist;
- every non-heading body block is a version-1 claim paragraph with at least one direct source anchor;
- every source anchor resolves to a source and valid inclusive line range;
- wiki links resolve;
- page ids, tags, claim kinds, and anchor syntax match the version-1 schema;
- generated index matches a clean rebuild;
- capture inventory contains no ignored, outside-repository, symlinked, or inside-corpus origin path.

Optional `semantic-lint` is a skill-owned, model-backed workflow rather than a script command. It asks a model to propose contradiction, staleness, duplication, missing-concept, and gap candidates and writes a timestamped report under `artifacts/research-wiki/lint/`. It never edits the wiki or changes source state. Every accepted semantic change runs through normal ingest against raw anchors.

#### `promote`

Promotion is not a research-wiki command. When research supports a durable project fact, design decision, known issue, or instruction change, the agent invokes the existing owner and its gate. The canonical document cites the primary evidence where useful. The wiki remains derived and may link to the canonical result.

### Invariants

1. Raw evidence is content-addressed and immutable. Enforced by `capture` and `check`.
2. Every wiki claim paragraph cites at least one raw source directly. Enforced structurally by `check`; semantic support remains human/model-reviewed.
3. Wiki content has no policy authority. Enforced by routing ownership and an audit fixture that rejects `wow.md` or root-contract routes naming corpus pages as canonical guidance.
4. Query is read-only unless the user explicitly requests repository edits. This is owned by the existing action-matching contract and reinforced by the skill; the design does not claim a new mechanical permission boundary.
5. The index is derived, reproducible state. Enforced by `rebuild-index` and `check`.
6. Existing plans, receipts, artifacts, chat transcripts, and Git history are never bulk-ingested. A human or task explicitly selects each source.
7. A wiki page never becomes raw evidence for another factual claim. Only `sources/` anchors satisfy provenance.
8. No external search or model call happens inside the deterministic script. The runtime owns those boundary calls and their existing spend/permission rules.

### Observability and failure behavior

The script emits a JSON result with operation, corpus root, source/page ids, files changed, warnings, and one outcome from `ok`, `invalid`, `conflict`, `integrity-failed`, `stale`, or `partial`. `stale` names changed origins and superseded citations separately. Unexpected exceptions preserve a stack trace on stderr and leave no partially published source or index.

Model-backed operations record the model, source ids, affected page ids, and verification result in the task receipt note. Paid raw responses follow the metasystem's existing replay and durable-evidence rules. No separate telemetry store is introduced.

### Security and trust

- Raw sources and wiki text are untrusted data, including text that resembles agent instructions. The skill repeats the existing `AGENTS.md:11` boundary at the point of use.
- Capture rejects paths outside the repository, paths inside the corpus, symlinked paths, and ignored files. The first slice has no override flag.
- The first slice accepts files only. URL fetching remains the runtime's responsibility so network policy, authentication, robots, licensing, and secret handling stay with existing boundaries.
- Capture never stages or commits output. Before capture, the skill shows the human the repository-relative path, byte count, hash, and a bounded preview, then asks for the public/copying confirmation required by `--classification public`. The template has no secret scanner, and the mechanism does not claim to detect sensitive bytes; accidental public misclassification remains a human-review risk stated in every capture result.
- A source deletion is a normal reviewed Git change and is never performed by lint or query.

### Scale and stop conditions

- Start with the deterministic literal selector over Markdown pages.
- Revisit retrieval only after the evaluation shows literal selection misses required pages beyond the guard floor or the index cannot fit within the declared query budget.
- Split corpora by research topic before adding search infrastructure.
- Remove or leave the skill disabled if, after two retros, receipts show no repeated queries, no avoided re-reading, no research promoted to a canonical owner, or ceremony notes exceed observed value.

## First Vertical Slice

The first pilot candidate is this template repository's external-reference research. It does not start until an activation trigger in the verdict fires. When it does:

1. Name a builder, answering operator, blinded rater, and separate holdout custodian. Freeze the development evaluation under `meta/research/evaluation/`; the custodian seals holdout questions, expected answers, direct-source baselines, and packet hashes outside every builder-accessible workspace.
2. Fill the existing Improvement Evaluation, usage-source, spend-fence, and warning slots in `docs/project-rules.md`; a paid pilot is inadmissible while they remain placeholders.
3. Capture permitted local UTF-8 source snapshots for the development corpus. The holdout custodian retains the independent holdout sources until the development candidate is frozen. A source that cannot legally or operationally be captured is excluded.
4. Run the direct-source baseline and record its frontier and measured noise before implementing the wiki candidate.
5. Implement and invoke the optional skill in place from `optional-skills/research-wiki/`; do not run first-install adoption against the template checkout.
6. Compile pages for the core pattern, evidence, failure modes, and metasystem-fit decision, then generate the index and run deterministic `check`.
7. Content-hash each development candidate version before running its exact preregistered repetition set, no fewer than three. Each repetition initializes an empty corpus, independently compiles it from the same raw sources in a fresh session with that frozen version, and answers the fixed questions in fresh sessions. A code, prompt, schema, selector, model, or procedure change creates the next candidate cycle and requires a complete new repetition set. After a valid development gain, freeze the accepted version for holdout.
8. The custodian releases only the holdout raw sources to fresh builders using the frozen candidate. They independently build the exact preregistered number of holdout wikis, no fewer than three. Holdout questions, expected answers, and baseline artifacts remain inaccessible until every repeated wiki is sealed. Separate answering sessions then receive randomized question packets and the blinded rater scores anonymized answers.
9. Only after an independently replicated holdout win, exercise fresh adoption and use the corpus to update the keep/remove/defer decision in `development/source-analysis.md`.

The slice replaces no canonical owner. It replaces the ad hoc source-tracing work used to maintain `development/source-analysis.md`. If the pilot does not reduce repeated source reading or improve claim support, remove the corpus and keep only the source-analysis decision.

## Focused Proof

### Deterministic fixtures

- capture is idempotent and content-addressed;
- tampered payload fails hash verification;
- interrupted capture publishes no partial source;
- outside-worktree, ignored, symlinked, and inside-corpus capture inputs fail without writes;
- duplicate/conflicting ids fail;
- existing-source recapture unions new supersession edges, retains omitted edges, and rejects nonexistent, self, or cyclic edges atomically;
- changing an origin, capturing its new bytes with a supersession edge, updating cited claims, and rerunning query clears freshness without deleting the old source;
- missing and out-of-range anchors fail;
- an untyped body block or a claim paragraph without a raw anchor fails;
- wiki-only citation fails provenance;
- broken wiki link fails;
- stale generated index fails and rebuild is byte-deterministic;
- query preflight refuses an integrity-failed corpus;
- optional-skill adoption enables exactly the requested skill and leaves the default payload unchanged;
- fresh adoption without the option contains no corpus, route, or config placeholder.

### Pilot evaluation

The existing `skills/improve/SKILL.md` owns this evaluation. The first post-trigger deliverable is the scoring evaluation and frozen prompt packets, not the wiki. Version 1 deliberately has no script that launches models. A named human operator runs each generated packet in a fresh top-level session through the selected runtime's normal surface, exports provider usage from the authoritative source, and records the response. This avoids inventing a dispatcher role or bypassing the metasystem adapters.

The template-only evaluation exposes two deterministic commands:

```text
python3 meta/research/evaluation/research_wiki_eval.py prepare \
  --contract meta/research/evaluation/contract.json \
  --condition CONDITION \
  --corpus CORPUS \
  --output artifacts/research-wiki/evaluation/<run-id>/packets

python3 meta/research/evaluation/research_wiki_eval.py score \
  --contract meta/research/evaluation/contract.json \
  --responses artifacts/research-wiki/evaluation/<run-id>/responses \
  --usage artifacts/research-wiki/evaluation/<run-id>/provider-usage.json \
  --human-scores artifacts/research-wiki/evaluation/<run-id>/human-scores.csv \
  --output artifacts/research-wiki/evaluation/<run-id>/score.json
```

`CONDITION` is exactly `direct-source` or `compiled-wiki`; `CORPUS` is exactly `development` or `holdout`.

Evaluation type: stochastic. Each corpus has at least ten human-authored questions covering single-source lookup, cross-source synthesis, contradiction, staleness, and unanswerable cases. Before candidate work, a human freezes for every question the required source ids, acceptable line anchors, whether abstention is expected, a zero-to-four completeness rubric, and its minimum passing score. The wiki builder and answering sessions never see that expected-answer file. Direct-source packets contain the frozen raw snapshots permitted for that question; compiled-wiki packets contain the deterministic selection trace, selected claims, and admitted full payloads. `prepare` freezes packet hashes and randomized order. The answering operator uses fresh top-level sessions and the same declared runtime/model and budgets. `contract.json` freezes one exact repetition count of at least three for each condition and corpus before the first evaluation call. Every declared repetition must be present and scored; missing, extra, selectively replaced, or result-selected repetitions make the evaluation `invalid-run`. A candidate version is content-hashed before its repetition set; any version change starts a new cycle and cannot reuse results. Every compiled-wiki repetition starts from an empty corpus and invokes that frozen compilation procedure in a fresh session before its question sessions; no wiki or derived page is shared across repetitions. Human raters use the frozen rubric on condition-anonymized answers in randomized order; the scorer reveals conditions only after the score file is sealed. A response or usage record with a missing session id, packet hash, effective model, or authoritative usage join is `invalid-run`, never zero cost.

Holdout custody is a validity condition. Before candidate work, a human who is neither the builder nor a development answering operator creates and seals the holdout questions, expected-answer file, packet hashes, direct-source response and usage artifacts, and baseline scores outside the Git tree, artifacts directory, conversation, and every workspace accessible to the builder. After the development candidate is content-hashed and frozen, the custodian releases only holdout raw sources to fresh builder sessions. The candidate's code, prompts, schema, selector, model configuration, and corpus-building procedure cannot change afterward. Holdout questions become visible only to fresh answering sessions after every independently compiled holdout corpus is sealed; expected answers and baseline scores remain with the custodian until all candidate answers are sealed. Missing role separation or premature access makes the holdout run `invalid-run`.

Improvement contract:

- Per-answer pass: every cited source and line anchor resolves; required-source recall is exactly 1.0; every factual claim is marked supported by the blinded rater; completeness is at least that question's frozen minimum; and an expected-unanswerable question contains an explicit abstention and no unsupported factual answer. Required-source recall is `cited required ids / frozen required ids`; it is 1.0 for an expected-unanswerable case only when the answer correctly abstains. A source-integrity failure or query-caused repository write makes every answer in that run fail.
- Primary metric per repetition: fully source-supported answer efficiency, higher is better: `100000 * count(per-answer passes) / provider-reported input tokens`. For each compiled-wiki repetition, the denominator includes that repetition's actual independent compilation and maintenance input-token cost plus its query input tokens, preserving the frozen question-set amortization. For direct source it includes all source-reading and query input tokens in that repetition.
- Baseline statistics: use every exact preregistered direct-source repetition. `baseline_mean` is the arithmetic mean of their primary scores and `baseline_sd` is their sample standard deviation in the same score units. A zero baseline mean, a declared count below three, or any repetition-set mismatch makes the contract invalid. Record `noise_delta = max(0.05 * baseline_mean, 2 * baseline_sd)`.
- Candidate target: use every exact preregistered repetition of the frozen candidate and their arithmetic mean `candidate_mean`. Pass only when `candidate_mean >= baseline_mean + max(0.20 * baseline_mean, noise_delta)`. Apply this equation independently on development and holdout; no development statistic is substituted for the holdout baseline.
- Aggregate guard: across every final candidate answer for that corpus, mean required-source recall is at least the corresponding baseline mean; claim-level support is at least `max(0.95, baseline claim-support mean)` using total supported claims divided by total factual claims; and mean completeness is at least the baseline mean. All means and claim counts include failed answers rather than only per-answer passes.
- Run guard: zero source-integrity failures, zero unsupported factual answer on an expected-unanswerable case, zero query-caused repository writes, and every custody and usage join valid.
- Budget: exact repetition counts plus run, provider-cost, and wall-clock ceilings must be filled from `docs/project-rules.md` and copied into `contract.json` before the first evaluation call. Missing accounting or a post-result count change makes the run inadmissible.
- No-gain budget: 3
- Non-goals: optimizing model choice, adding retrieval infrastructure, or tuning against holdout questions.

The human-run response bundle records raw answers, selected source ids, cited anchors, provider usage, human scores, effective model/configuration, corpus hash, packet hash, session id, condition, and run id. `score` validates those joins, computes the metrics, and calls `scripts/frontier.sh` for the primary metric. A development gain is not accepted until the holdout replicates it with all guards passing. If the human-run protocol or scoring command cannot be made reproducible within its approved setup budget, or if the candidate misses the target or a guard after the no-gain budget, leave the skill unimplemented or remove the prototype.

## Implementation Sequence After Design Approval

| Step | Artifact | Proof |
| --- | --- | --- |
| 1 | After an activation trigger, name the four evaluation roles, freeze the development evaluation, and have the independent custodian seal holdout materials outside builder access | Evaluation self-tests; human-verified source anchors; custody manifest; no wiki exists yet |
| 2 | Fill the existing project evaluation, accounting, and budget facts | `docs/project-rules.md` has no relevant placeholder; accounting preflight passes |
| 3 | Run and record every preregistered development direct-source repetition; the custodian separately seals every declared holdout repetition | `scripts/frontier.sh`; exact count of at least three valid repetitions per baseline; custody hashes |
| 4 | Add `optional-skills/research-wiki/` and runtime metadata | `validate-skill.sh`; metadata fixtures |
| 5 | Add `optional-skills/research-wiki/scripts/research-wiki.py` with init, capture, select, rebuild-index, and check | Focused positive and negative fixtures |
| 6 | Content-hash each development candidate version, then independently rebuild the corpus in every preregistered repetition under improve mode | Exact declared count of fresh compilations per version; no result reuse across versions; accepted candidate mean meets the exact target and every aggregate/run guard passes |
| 7 | Content-hash and freeze the complete candidate, then independently build every declared holdout repetition from custodian-released raw sources without revealing questions | Candidate freeze manifest; exact declared count of fresh holdout compilations; holdout-access audit |
| 8 | Replicate with fresh holdout answering sessions and blinded scoring | Holdout candidate mean meets its independently calculated target and every guard passes |
| 9 | Exercise the existing generic `--enable research-wiki` fresh-adoption path and add project guidance | Paired fresh-adoption fixtures with and without the option |
| 10 | Record the keep/narrow/reject decision in `development/source-analysis.md` and the next retro | Link/audit check plus receipt evidence; remove the candidate on failure |

No implementation step edits a concurrently modified path without first re-reading and reconciling its diff. Test additions should prefer a focused fixture script so the large validation owner only needs a small integration call.

## Design Critique Record

Round 1 used an independent fresh-context critic under `skills/design-critique/SKILL.md`. The critic reported five material findings; all five are retained and adjudicated here.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| LLMW-001 | ACCEPT | `development/source-analysis.md` is an output, not a caller. Repository history shows source harvests but no repeated query baseline, provenance-caused defect, or avoided re-reading measurement. | Changed the verdict from implementable value to design-and-park; added two activation triggers and selected direct citations as the immediate mitigation. |
| LLMW-002 | ACCEPT | `adopt.sh:10,102-149,280-323` is a fresh-install path and cannot enable a feature in the template checkout or an existing adoption. Skill registration does not create a read-only dispatch role. | The template pilot invokes the optional skill in place; fresh adopters use generic `--enable`; existing adopters reconcile/upgrade. Removed the claimed mechanical permission boundary and new role. |
| LLMW-003 | ACCEPT | The earlier contract required a payload while also permitting reference-only records and left identical bytes from different origins ambiguous. | Version 1 now permits payload-bearing UTF-8 sources only and defines the exact source schema, content identity, origin merge, and line-anchor rules. |
| LLMW-004 | ACCEPT | Natural-language factuality cannot be decided by a deterministic Markdown linter, and tables/lists/mixed paragraphs had no acceptance rule. | Version 1 now uses explicit `Fact:`, `Synthesis:`, and `Uncertainty:` claim paragraphs; unsupported Markdown constructs fail; metadata source duplication was removed. |
| LLMW-005 | ACCEPT | The earlier pilot had no single metric, baseline, noise floor, guards, exact command, holdout, or spend fence despite `skills/improve/SKILL.md` owning those requirements. | Made the evaluation the first post-trigger deliverable, added the exact planned command, stochastic protocol, primary metric, target, noise formula, guard floors, budgets, frontier, and independent holdout replication. |

Round 1 verdict after join: five findings, five dispositions, zero dropped.

Round 2 re-read the amended design and reported seven material findings. Six are accepted. One is refuted from the current plan text rather than discarded.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| LLMW2-001 | ACCEPT | The first draft constrained source paths but did not define who creates the corpus root or prevent an ignored, symlinked, or outside-worktree root. | Added an `init` operation as the sole root creator, real-path and same-worktree checks for every command, atomic initialization, and independent outside-root and outside-input fixtures. |
| LLMW2-002 | ACCEPT | Content identity alone did not say when a newer capture makes prior claims stale. | Added explicit acyclic `supersedes` relationships, stale superseded-citation failures, and origin re-hashing through `check --fresh`. |
| LLMW2-003 | ACCEPT | The proposed evaluator named model sessions without an executable owner in the metasystem. | Assigned model execution to a named human operator using fresh normal-runtime sessions. The planned script only prepares immutable packets, validates exported usage and response joins, and scores results. |
| LLMW2-004 | ACCEPT | A token-only primary metric did not measure the candidate consumer's provenance problem. | Replaced it with fully source-supported answers per 100,000 provider-reported input tokens, with required-source, claim-support, completeness, unanswerable, integrity, and no-write guards. |
| LLMW2-005 | ACCEPT | `max_pages` and `max_source_bytes` were named without deterministic ranking, byte accounting, or an omission contract. | Specified exact token normalization, field weights, stable tie-breaking, source-byte accounting, claim admission, complete omission reporting, and a JSON selection trace. |
| LLMW2-006 | ACCEPT | The template has no secret scanner, so delegating capture safety to one would create a fictitious control. | Limited version 1 to explicitly human-confirmed public/copyable UTF-8 files, added path, size, hash, and bounded-preview confirmation, prohibited automatic staging, and recorded public misclassification as residual risk. |
| LLMW2-007 | REFUTE | The finding said page and index serialization remained incomplete. The current plan already fixes exact metadata keys and `state` (`:263`), page-id and filename grammar plus link syntax (`:263`), title and tag constraints (`:263`), and index header, entry shape, order, encoding, and line endings (`:267`). | No amendment. These constraints were present in the version reviewed but appear to have been missed while the file changed during critique. Re-check them in the next fresh-context round. |

Round 2 verdict after join: seven findings, seven dispositions, zero dropped. The accepted amendments remove six underspecified contracts. Another fresh-context round is required before this parked design can be considered critique-closed.

Round 3 reported five material findings. All five are accepted and adjudicated here.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| LLMW3-001 | ACCEPT | Parking a complete design indefinitely in `plans/` would violate the task-local lifecycle and create the baggage the verdict warns about. | Added a human-review decision, a 2026-09-03 or next-retro deadline, `development/source-analysis.md` as the compact durable decision owner, and deletion rules for both defer and activation outcomes. |
| LLMW3-002 | ACCEPT | Callers and proofs named `check`, while the operation heading called the same deterministic validation `lint` and also described optional model review. | Renamed the deterministic operation `check` throughout and separated model-backed `semantic-lint` as a skill workflow with no script command or mutation authority. |
| LLMW3-003 | ACCEPT | The prior metric mixed per-answer and aggregate guards and compared percentages with score-unit standard deviations without one verdict equation. | Defined exact per-answer predicates, aggregate and run guards, repetition accounting, sample deviation units, `noise_delta`, and the development/holdout candidate threshold equation. |
| LLMW3-004 | ACCEPT | Preparing both corpora in builder-visible storage made the claimed untouched holdout vulnerable to question-shaped compilation. | Added a separate custodian, external sealed holdout materials and baselines, a content-hashed candidate freeze, staged raw-source release, fresh builder and answer sessions, blinded scoring, and `invalid-run` on custody failure. |
| LLMW3-005 | ACCEPT | Re-capturing an existing payload did not define how new or omitted supersession edges interact with stored metadata. | Made supersession updates atomic monotonic unions, retained omitted edges, prohibited version-1 removal, and added idempotence plus nonexistent/self/cycle rejection fixtures. |

Round 3 verdict after join: five findings, five dispositions, zero dropped. A fourth fresh-context pass checks the joined amendments; zero new material findings closes critique, while another material contract defect requires one further adjudication or the documented diminishing-returns stop.

Round 4 reported two material findings. Both are accepted and adjudicated here.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| LLMW4-001 | ACCEPT | Hashing the mutable origin of every preserved source would keep queries blocked after changed bytes were captured as a new source and the old source was superseded. | `check --fresh` now skips origin freshness only for transitively superseded sources while retaining their integrity checks and stale-citation failures. Added the full change, recapture, supersede, ingest, and successful-query fixture. |
| LLMW4-002 | ACCEPT | Repeating answers against one lucky compiled artifact measured answering variance while leaving the central stochastic compilation-loss risk untested. | Every candidate repetition now starts empty, independently compiles with the frozen procedure in a fresh session, charges its actual compilation tokens, and then runs fresh question sessions. Holdout questions remain sealed until every repeated holdout wiki is sealed. |

Round 4 verdict after join: two findings, two dispositions, zero dropped. Finding count has fallen from five to two rather than plateauing, so one round-5 re-read is still justified. Zero new material findings closes critique; another finding is adjudicated only if it changes safety or the falsifiable verdict.

Round 5 reported two material findings. Both are accepted and adjudicated here.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| LLMW5-001 | ACCEPT | The lifecycle named human rejection but only specified activation and deadline-triggered defer outcomes, so a rejection could later be misread as a deferral. | Explicit rejection now records `reject` and rationale, deletes the plan immediately, and requires explicit reversal or materially new evidence before reopening. |
| LLMW5-002 | ACCEPT | “At least three” repetitions let an operator stop on favorable results or add runs after unfavorable ones. | Exact counts of at least three per condition and corpus are frozen in `contract.json` before the first evaluation call. Every declared run is scored; missing, extra, selectively replaced, or result-selected runs invalidate the evaluation. |

Round 5 verdict after join: two findings, two dispositions, zero dropped. The count plateau and findings' narrower scope establish the critique skill's diminishing-returns condition. One final round-6 re-read checks that these small amendments join cleanly; after it, critique closes whether it reports zero material findings or only lower-value refinements that do not change safety, lifecycle, or the falsifiable verdict.

Round 6 returned **ZERO MATERIAL FINDINGS**. The independent critic confirmed that the joined design closes lifecycle outcomes, freshness recovery, candidate-version provenance, exact repetition counts, holdout custody, safety boundaries, and the deterministic keep/remove equation. Remaining details are implementation-level refinements. Critique is closed by join after six rounds: 21 material findings, 20 accepted amendments, one evidence-backed refutation, and zero dropped findings.

## Design Obligations

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| RW-SOURCE | CRITICAL | Karpathy raw layer; compilation-loss evidence; metasystem truth rule | Raw evidence is immutable, content-addressed, and directly cited | `skills/research-wiki/scripts/research-wiki.py` | Planned `capture` and `check` commands | Planned tamper, conflict, partial-write, and anchor fixtures | Planned pilot source-integrity report | PARTIAL | Implement after design acceptance |
| RW-AUTHORITY | CRITICAL | `wow.md:3-29`; `design-principles.md:33,42` | Wiki output never becomes policy or reference truth by itself | enabled `skills/research-wiki/SKILL.md` plus `wow.md` ownership | Planned promotion boundary and absence of a corpus route | Planned audit and wiki-only-citation fixtures | Planned fresh-session authority probe | PARTIAL | Implement fixtures only after human activation and the evaluation gate |
| RW-QUERY | HIGH | User action boundary; Karpathy query; small comparison | Query is bounded, source-checking, explicit about partial results, and read-only by default under the existing action contract | enabled `skills/research-wiki/SKILL.md` and `<research.root>/schema.json` | Planned query workflow; no new permission claim | Planned integrity-preflight, no-write, and budget scenarios | Planned fixed-question pilot | PARTIAL | Implement only after activation and evaluation gates pass |
| RW-INDEX | HIGH | Karpathy index; metasystem deterministic-control rule | Navigation is deterministic derived state | `skills/research-wiki/scripts/research-wiki.py` | Planned `rebuild-index` command | Planned stale-index and byte-reproducibility fixtures | Planned pilot index inspection | PARTIAL | Implement after design acceptance |
| RW-FAILURE | HIGH | `design-principles.md:28-31,46-51` | Failures are explicit and partial writes are recoverable | `skills/research-wiki/scripts/research-wiki.py` | Planned atomic source/index writes and JSON outcomes | Planned failure-category fixtures | Planned interrupted-operation inspection | PARTIAL | Implement after design acceptance |
| RW-OPTIONAL | HIGH | `metasystem-architecture.md:14-17`; current-consumer gate | Default adopters pay no corpus, instruction, or dependency cost | `optional-skills/research-wiki/` and `scripts/adopt.sh` | Planned opt-in copy path | Planned paired adoption fixtures | Planned fresh-adoption inspection | PARTIAL | Implement only after human activation and the evaluation gate |
| RW-SECURITY | HIGH | `AGENTS.md:10-15`; source content is untrusted; template has no secret scanner | Source-borne instructions cannot gain authority; capture accepts only explicitly human-confirmed public files, rejects ignored/outside paths, and never stages a commit | enabled `skills/research-wiki/SKILL.md` plus `skills/research-wiki/scripts/research-wiki.py` | Planned untrusted-data reminder, public-classification gate, bounded preview, path refusal, and no Git mutation | Planned malicious-source, missing-classification, ignored-path, outside-root, and Git-status fixtures | Planned human capture confirmation and pilot diff review | PARTIAL | Implement only after activation and evaluation gates pass; report residual misclassification risk |
| RW-VALUE | HIGH | Current-consumer gate; two research preprints; improve and retro removal rules | An activation trigger names a real consumer, then the candidate shows replicated measured value or is removed | template evaluation plus `skills/improve/SKILL.md` and `skills/retro/SKILL.md` | Planned frozen evaluation, baseline, candidate, and holdout | Planned source-derived development and holdout sets | Planned provider-usage and human-scoring artifacts | PARTIAL | Build evaluation first after a trigger; do not implement now |
| RW-SCALE | MEDIUM | Karpathy moderate-scale advice; comparison cost result | Retrieval infrastructure appears only after a measured deterministic-selector miss | `<research.root>/schema.json` and design gate | No search dependency in the planned first slice | Planned page-budget partial-result fixture | Planned retrieval-miss measurement | MISSING | Revisit only if the pilot crosses the declared trigger |

Pre-implementation gate:

- Critical and high obligations have named owners and focused proof targets.
- The mechanism has a concrete consumer activation trigger and a removal condition; no current consumer is claimed.
- No new dependency, external API contract, or always-loaded instruction is proposed.
- Independent design critique is closed. Human activation and evaluation approval remain required before implementation.

## Open Questions

- Should remote snapshots ever be committed? Recommendation: version 1 accepts only permitted local UTF-8 files; later support for URLs, Git blobs, PDFs, or images needs its own source-identity and extraction design.
- Should the pilot live permanently under `meta/research/`, or be removed after its decision is promoted? Recommendation: keep it only if the fixed-question pilot shows repeated value through two retros.
- Does the human want the next task to stop at the reviewed design, or proceed into implementation after critique? This plan assumes design first and no implementation before acceptance.

## Completion Evidence Required

- Linked primary research sources with claims tied to each source.
- File-and-line map of the relevant current metasystem behavior.
- Written go/no-go decision and rejected alternatives.
- If changed: focused checks, default completion check, end-to-end verification, and a receipt.
- If unchanged: exact recommendation and the evidence showing why implementation would add more lifecycle cost than value.
