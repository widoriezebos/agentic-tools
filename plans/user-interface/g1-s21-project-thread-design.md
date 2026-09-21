# g1-s21 Project: the thread of intent, read-only

- Gate 1, state `designed`, revision 1, author Claude on Fable as a D23 delegate, 2026-09-21. Full pipeline: a Go dependency and a data contract.
- Refines the [master](../user-interface-design.md) at `ui-development` commit `6715daf01`: Project `:473` to `:490`, Rules `:433` to `:449`, Workspace identity `:451` to `:461`, Existing homes `:737` to `:767`, Trustworthy state `:769` to `:777`, `DefinitionRefs` `:509` to `:511`, gates 3 and 5 `:811`, `:813`, scenario 29 `:855`. Plan: D31 `:50`, D6 `:130`, step 4 `:156`, conventions `:112` to `:121`, must-not-touch `:108`.
- Depends on `g1-s9` revision 3 and `g1-s8` revision 5, both merged; D1, D6 (settled below), D11, D14, D20, D28, D31. Not on `g1-s7`, `g1-s2`, or the concurrent `g1-s10`.
- Discharges: foundation for scenario 29 (an architecture document through its canonical record, no copy) and for 22 and 28, discharged at gate 5; contributes to 15.
- Naming: the plan's row `:193` still calls `g1-s21` "Goal detail"; D31 and step 4 put the Project thread here. The planner reconciles the row.

Paths are relative to `metasystem/` unless they start with `plans/` or `development/`. Code claims were read in source at `6715daf01`; nothing was executed but three `curl` reads of the running server; registry facts from pkg.go.dev, 2026-09-21.

## Outcome

A human opens `/project` and sees the master's six subsections. Five list what this checkout already holds at its canonical locations, each document with its path, the engine's ownership answer, and when it last changed; a subsection with no source says "Not yet recorded" and names where it looked; Sittings says it arrives with gate 3. A document opens at `/project/doc/<path>` in a calm reading layout with a contents outline, tables, code, and links that open other documents in the checkout. No duplicate document was written to make this appear.

## Scope and non-goals

In scope: a Go catalogue of canonical locations over the three roots; a Go Markdown reader producing a typed tree; two read routes; the Project and document panes; the `document` kind in the resolver; the cut guard's second allowlist. Not: editing, drafts, `DefinitionRefs` (gate 5); the sitting store (gate 3); the goal view (`g1-s10`); invalidation on file change (`g1-s7`); search; images.

## Existing code this builds on

| What it means here | Where |
| --- | --- |
| Exact-path dispatch after the checks; `/api/workspace` answered per request through `Info.Describe`; 500 as `{"error":…}`; 404 beneath reserved prefixes; headers and policy on every response | `internal/ui/httpd/httpd.go:20` to `:31`, `:43`, `:48`, `:126` to `:135`, `:211` to `:230`, `:344` to `:351` |
| The wiring; a package declaring its own `Roots` to avoid the lifecycle test import loop | `cmd/metasystem/ui.go:125` to `:140`; `internal/ui/workspace/workspace.go:11` to `:18` |
| Three roots; the state root is the installation when self-hosted, else the Git top; state kinds `memory`, `plans`, `plans/goals`, `records` | `internal/ui/lifecycle/roots.go:15`; `internal/stateroot/stateroot.go:111` to `:120`, `:247` to `:264`, `:293` to `:299` |
| Ownership oracle: the vendored prefix is `metasystem-generic`; `artifacts/`, `bin/` are `runtime`; unvendored, `docs/design/`, `memory/`, `plans/`, `records/`, `docs/project-rules.md` are generic and everything else, `docs/architecture.md` included, is `app-owned`; each call shells to `git rev-parse` | `internal/stateroot/owner.go:44` to `:118`, `:142` to `:156` |
| The covenant: one home `covenant.json` at the app root; `Load` refuses symlinks and irregular files; typed identity, requirements, battery, budgets, guards, guardrails. **No `covenant.json` exists** at the checkout root or under `metasystem/` | `internal/covenant/covenant.go:33` to `:45`, `:92`, `:200` to `:222`; `internal/mission/guardrails.go:34`; `cmd/metasystem/covenant_verbs.go:23`; `find` at depth 2 |
| Inception writes `covenant.json`, `docs/app-doctrine.md` (architecture intent; a retrofit's doctrine indexes the canonical documents by path), `docs/covenant-evidence.md`, and the `docs/project-rules.md` facts, whose `- Purpose: <one paragraph>` sits beside the adoption line the shell parses | `skills/inception/SKILL.md:76` to `:80`, `:216` to `:218`; `docs/project-rules.md:7` to `:8`; `internal/ui/workspace/adoption.go` |
| The goal record: `Intent string` required; `Origin` is `human|main`; `ReadItem` is a review finding; **no `DefinitionRefs` field or type exists** under `internal/` or `cmd/` | `internal/goal/file.go:23` to `:87`, `:178` to `:188`, `:614`, `:617`; `grep -rn DefinitionRef` empty |
| What exists: `docs/architecture.md` ("MAP, not design"), `docs/concepts.md`, 12 files in `docs/design/`, the paper's `index.md` and 19 chapters (263,893 bytes, the largest 27,569), all shipped by adoption; `plans/`: 1,038 `.md` files, 102 ending in `-design.md`, subdirectories including `goals/`; host `plans/`: 12 files, the master (910 lines), three slice designs under `user-interface/` | `docs/architecture.md:1` to `:6`; `docs/paper/index.md:24` onward; `scripts/adopt.sh:246`; listings |
| Living registers: `known-issues.md` (recorded, not scheduled), `proposal-drafts.md` (awaiting the human's word), `rulings.md`, `backlog-notes.md`; no register of open questions exists | `memory/README.md:3`; `known-issues.md:3`; `proposal-drafts.md:1` to `:12` |
| Shell: the `Routes` block; `Pane`, `EmptyState`; the empties table (Project "Arrives with gate 5", superseded by D31) and its test; `routeFor`; the tab title; `.ms-pane-stack` at 720px; the cut guard's one-site assertion | `src/shell/Shell.tsx:108` to `:116`; `src/panes/Pane.tsx:17` to `:47`; `src/panes/empties.ts:46` to `:54`; `empties.test.ts:133` to `:161`; `src/routes.ts:56` to `:91`; `src/title.ts:14` to `:28`; `src/shell/shell.css:747`; `src/cuts.test.ts:313` to `:326` |
| The policy on every response, observed live; live `stateRoot` equals the installation and `sourceHead` is `""` | `httpd.go:54` to `:57`; `curl`, 2026-09-21 |

## Contracts

### The catalogue: a convention fixed in Go, resolved against the roots

The catalogue is a table in `internal/ui/project`, not a file in the repository and not a scan. Each entry names a subsection, a root (installation `I`, state root `S`, checkout `C`), a path or one-level glob, and its modes. Nothing is filtered by ownership; ownership is shown. The kit's own documents (the paper, `docs/design/`, `docs/architecture.md`, `docs/concepts.md`) are the subject's only when the subject is the MetaSystem; in an adopted workspace they are the machinery's, Settings' at gate 7, and omitted here (D20).

| Subsection | Source | Root | Mode | Shown as |
| --- | --- | --- | --- | --- |
| Intent | `covenant.json`: identity and requirements | `S`, and `C` when `C ≠ S` | both | typed facts |
| Intent | the `- Purpose:` line of `docs/project-rules.md`; `<one paragraph>` or absent is not recorded | `I` | both | one paragraph |
| Architecture | `docs/app-doctrine.md` | `I` | both | document |
| Architecture | `docs/architecture.md`, `docs/concepts.md` | `I` | self-hosted | documents |
| Designs, "The paper" | `docs/paper/index.md` first, then `docs/paper/[0-9][0-9]-*.md` by name | `I` | self-hosted | documents |
| Designs, "Design documents" | `docs/design/*.md` by name | `I` | self-hosted | documents |
| Designs, "Live designs" | `plans/*-design.md` and `plans/*/*-design.md` by path, never under `plans/goals` or `plans/goals-drafts` | `S`, and `C` when `C ≠ S` | both | documents |
| Constraints and assurance | `covenant.json`: battery, budgets, guards, guardrails | as Intent | both | typed facts |
| Constraints and assurance | `docs/covenant-evidence.md`, `docs/project-rules.md` | `I` | both | documents |
| Constraints and assurance | `development/project-rules-local.md` | `C` | self-hosted | document |
| Open questions | no register exists; `memory/known-issues.md` and `memory/proposal-drafts.md` are the nearest | `S` | both | "Not yet recorded", then the two under "Nearest living registers" |
| Sittings | none; the sitting store arrives with the brain process (master `:65`, `:811`) | – | – | not projected, gate 3 |

This survives an adopted project that keeps its architecture elsewhere because inception's declared home for architecture intent is `docs/app-doctrine.md`, which for a retrofit indexes the canonical documents by path, and every relative Markdown link opens its document (below); no declaration file is added. A subsection whose every source is absent is `not-recorded` and lists what it looked for, with absolute paths. Nothing records which designs are live, so the 102 `-design.md` documents plus the host's four are listed by path with modification times; the distinction is not invented.

### Documents: identity and the reading boundary

A document is `{"kind":"document","id":"<checkout-relative path, / separators>","revision":"blob:<40 hex>"}`. The revision is the Git blob object id of the bytes read (SHA-1 over `blob <size>\0` and the content, `crypto/sha1`), equal to `git hash-object <file>`: the repository's own name for the content, which a later `DefinitionRef` can pin. Whether HEAD holds it is not claimed; `sourceHead` is `g1-s7`'s.

The reading route serves any Markdown file in the checkout, not only catalogued ones, because that is how the doctrine's and the master's links open. The boundary, checked in Go in this order: the id is `path.Clean`ed and refused if it starts with `/` or has a `..` or empty segment; no segment is `.git`, `node_modules`, `artifacts`, or `bin`; the extension is `.md`; the absolute path after `filepath.EvalSymlinks` lies under the checkout's own `EvalSymlinks`; `os.Lstat` reports a regular file, refusing a symbolic link as the final entry; at most 1 MiB; valid UTF-8. Every refusal is a 404 `{"error":"no document at <id>"}` except size and encoding, which are 200 with `state` `too-large` or `unreadable` and no blocks. The trust is the goal reads' (loopback, D2, the human's own checkout); `metasystem.conf.local` is not `.md` and is never served.

Title: the first line beginning `# ` in the first 4 KiB, else the file name. Ownership: `stateroot.OwnerForInstallation(I, path)` once per catalogue directory and per single file, about twelve `git rev-parse` subprocesses per thread request, carried as the oracle's strings and labelled "the MetaSystem's", "the application's", "runtime state" (master `:459`); an oracle error is `unknown` with its reason.

### Markdown: D6 settled

**Decision: Go parses, the browser renders a typed tree, and no HTML string exists anywhere.** Dependency: `github.com/yuin/goldmark` **v1.8.6**, MIT, "depends only on standard libraries" (pkg.go.dev, 2026-09-21; the builder confirms the tag). The third module dependency (`go.mod` has two), justified: a CommonMark parser of our own is an open-ended defect surface, and goldmark is the most used Go implementation (Hugo). Only its parser and `ast` package are used, with the `Table`, `Strikethrough`, and `TaskList` extensions; its HTML renderer is never called and `WithUnsafe` never set, so the API carries no HTML.

The alternatives. Source text (the plan's placeholder at `:130`) is risk-free, but the paper and the 910-line master are not readable as raw text, which defeats D31, and D6 returns unchanged at gate 5. A browser renderer: `react-markdown` brings the unified, remark, micromark, and hast graph, roughly fifty packages, into the runtime closure where any advisory of any severity blocks every `npm run bundle` (`g1-s8`, Audit); `marked` with `DOMPurify` makes a browser sanitiser the trusted component behind `dangerouslySetInnerHTML`. Against "no vulnerabilities please", the Go tree has the smallest surface: one pure-Go dependency, React's own escaping, Go tests as the primary proof (plan `:73`). Cost: one `go.mod` line, some binary size (not measured), and a schema of ours that flattens what it does not model.

**Who sanitises: nobody has to, because nothing is interpreted.** Raw HTML in the source (`ast.HTMLBlock`, `ast.RawHTML`) becomes a node of type `html` whose `text` is the source; the browser shows it in a code block captioned "HTML in the source is shown, not run", so a `<script>` tag is visible characters. A link's `href` is carried verbatim and classified in Go: `external` for `http:` and `https:`; `fragment` for `#…`; `document` for a relative `.md` path under the checkout, with its `id`; everything else (`javascript:`, `data:`, `mailto:`, `file:`, paths outside the checkout or to non-Markdown files) is `unresolved`. The browser renders `external` as an anchor with `target="_blank" rel="noopener noreferrer"` and a 12px `ExternalLink` icon, `document` as a `NavLink` to `routeFor`, `fragment` in-page, and `unresolved` as mono text; as a belt it renders text for any `external` not starting with `http://` or `https://`. Images become the text `[image: <alt>] <src>`; no `<img>`. The policy (`httpd.go:54` to `:57`) already blocks inline script, `javascript:` navigation, inline style, external images, forms, and frames; it is the second wall. The first is React elements only: the cut guard gains a rule that `dangerouslySetInnerHTML`, `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `DOMParser`, `createContextualFragment`, and `write` name no identifier under `src/`.

Schema, `internal/ui/markdown`, `Parse(source []byte) Document`, `Document{Headings []Heading; Blocks []Block}`. Blocks: `heading` (`level`, `id`, `inlines`), `paragraph`, `code` (`lang`, `text`), `quote` (`blocks`), `list` (`ordered`, `start`, `items[]` with `checked` null or bool and `blocks`), `table` (`align[]`, `head`, `rows`, cells as inlines), `rule`, `html` (`text`). Inlines: `text`, `emph`, `strong`, `strike`, `code`, `link` (`href`, `target`, `id`, `inlines`), `image` (`src`, `alt`), `break`, `html`. Heading ids are GitHub-style slugs (lower-case, punctuation removed, spaces to `-`, duplicates suffixed `-1`), computed in Go so cross-document `#anchors` resolve. Footnotes and definition lists are not enabled; a mermaid fence is a `code` block with `lang` `mermaid`; CRLF is accepted. The keys are fixed for the brain's tools later (plan `:70`).

### Routes and payloads

`GET /api/project`, exact; 404 beneath; per request through `Info.Project func() (project.Thread, error)`, the `Describe` pattern; nil or error is a 500 `{"error":…}`. Payload: `{"schemaVersion":1,"readAt":"RFC 3339","subsections":[{"id","title","state":"recorded|not-recorded|not-projected","lookedFor":["/abs/…"],"covenant","purpose","groups":[{"id","title","documents":[{"kind":"document","id","title","owner","bytes","modifiedAt","state":"readable|unreadable|too-large","reason"}]}]}]}`. `covenant`, when present, is `{"path","identity":{"name","entryPoint","sourcePaths"},"requirements":[{"id","ref","proof"}],"battery","budgets","guards","guardrails"}` from `covenant.Load`; a parse failure is `{"path","error"}`, shown, never hidden. `purpose` is `{"path","text"}` or null.

`GET /api/documents/<id>`, prefix; `/api/documents` and `/api/documents/` are 404; through `Info.Document func(id string) (project.Document, error)`; `project.ErrNotFound` is the 404 above, any other error a 500. Payload: `{"kind":"document","id","title","revision":"blob:…","owner","path":"/abs/…","bytes","modifiedAt","readAt","state","reason","headings":[{"level","id","text"}],"blocks":[…]}`.

Go: `internal/ui/project` with its own `Roots{Checkout, Installation, StateRoot}`, `Thread(roots, now time.Time)`, `Read(roots, id, now)`, the mode from `stateroot.ResolveLayout(roots.Installation).Template`. `internal/ui/markdown` imports goldmark and the standard library only. `httpd` imports `project`; `project` imports `markdown`, `stateroot`, `covenant`; `goal` imports nothing of this. `cmd/metasystem/ui.go` wires two closures.

### Frontend

New under `src/project/`: `api.ts` (one `fetch` inside `getJSON`, used by `loadThread()` and `loadDocument(id)`; the strings `/api/project` and `/api/documents/`), `ProjectPane.tsx`, `DocumentPane.tsx`, `Markdown.tsx`, `links.ts` (the belt; relative resolution against the document's directory), `outline.ts`, `reading.css`, tests. `Shell.tsx` maps five sections to `SectionPane` and Project to `<Route path="/project">` and `<Route path="/project/doc/*">`. `routes.ts`: `routeFor({kind:"document", id})` returns `/project/doc/` plus the id, each segment `encodeURIComponent`-encoded; `sectionFor` already lights Project beneath `/project/` (`:56` to `:64`); the header's title must read Project on the document route, so `activeSection` (`:66` to `:78`, which its comment says changes with a nested route) and `routes.test.ts:73` change. Titles: `titleFor("Project", identity)` and `titleFor(`${document.title} · Project`, identity)`. `empties.ts`: the `project` row is removed; a `sittings` row is added, kind `not projected`, heading "Sittings are not projected yet", body "A sitting is a human and an agent working together; its working records live in the sitting store the brain process keeps.", note "Arrives with gate 3", no link, no action. No stored state and no `localStorage` key; the thread and each document live in the pane's React state, refetched per page view.

### Reading layout

From the shell's tokens. The article is a column of max-width **720px** (the `.ms-pane-stack` measure) inside the work area's padding. Body `md` 16/24 `text`, paragraphs 16 apart. Headings 600: h1 `xl` 24/32; h2 `lg` 20/28, 32 above; h3 `md` 16/24, 24 above; h4 to h6 `base` 14/20, 16 above. Lists indent 24, items 4 apart; task items a disabled native checkbox. Code blocks: JetBrains Mono `sm` 13/18 on `surface-2`, 1px `border`, radius 6, padding 12, `overflow-x: auto`, `tab-size: 4`, the language in 12px `text-3`. Inline code: mono 13 on `surface-2`, radius 4, padding 1px 4px. Tables: `sm` 13/18, header 600 on `surface-2`, cells 6px 10px, 1px `border` rules, in an `overflow-x: auto` container, so wide tables scroll rather than break the column. Blockquote: 3px `border-strong` left rule, padding-left 12, `text-2`. Rule: 1px `border`, 24 above and below. Links `accent`, underlined on hover and focus.

Document page, top to bottom: a `NavLink` "Project" back, 13px/500 `accent`; the title `xl`; a facts line 12px `text-3`: the path in mono, the ownership `Chip`, "changed <date time>", "read at <time>", the revision's first seven characters, a Reload `Button`; a `<nav aria-label="Contents">` of headings to level 3 when there are at least three, `sm` 13/18, 16 per level; the article. A fragment on load scrolls its heading into view after render, with no timer. `Skeleton` rows while fetching; on failure the message and Retry.

Navigable: headings by outline and fragment; Markdown in the checkout; external links in a new tab. Not: goals (`plans/goals/<id>.md` opens as a document; the goal route is `g1-s10`'s), images, non-Markdown files, anything outside the checkout, previous and next chapter (not recorded; the index links every chapter). A 900-line design is one article with an outline; the paper is twenty documents reached through `index.md`.

The Project pane: `.ms-pane-stack` of six `.ms-card`s in the master's order, "read at <time>" and Reload at the top. Each card holds the "Not yet recorded" statement with its looked-for paths in mono 12px `text-3`, or its groups: a 13px/600 group title and 32px rows of the title 14px/500 as a `NavLink`, the path mono 12 `text-3`, the ownership `Chip`, the date right-aligned; an unreadable or oversized row shows its reason in `danger` and no link. The covenant renders as `.ms-facts` rows and a requirements table.

## Behaviour

Main flow: `/project` fetches `/api/project` behind skeletons; Go, per request, resolves the layout, walks the catalogue (`os.Lstat` and a 4 KiB head read per entry), asks the oracle per directory, reads the covenant and the purpose line. A document route fetches `/api/documents/<id>`; Go checks the boundary, reads, computes the blob id, parses, classifies links.

| Case | Required result |
| --- | --- |
| A catalogued file is absent | Not listed; `lookedFor` names it; every source absent → `not-recorded`, "Not yet recorded" |
| A symlink, unreadable, or non-UTF-8 file | Listed `unreadable` with the reason; no link |
| A file over 1 MiB | Listed `too-large` with its size; the page says so and shows the path |
| `covenant.json` present but invalid | Intent and Constraints show "Covenant at <path> could not be read: <reason>" |
| `/api/documents/../../etc/passwd`, `…/metasystem/metasystem.conf.local`, a path under `.git`, a directory, a symlink out of the checkout | 404; the page shows "Not found in this checkout", the id, a link to Project |
| A relative link to a missing `.md` | `document` target; opening it shows the not-found card |
| `[x](javascript:alert(1))`, `<script>`, `<img onerror>`, `<a href=…>` in the source | Text; the three tags as text in code blocks; zero `securitypolicyviolation` events |
| The file changes on disk while open | Nothing changes; "read at" says how old the view is; Reload refetches and the revision changes |
| `/api/project` 404 or 500 | "Project could not be read", the message, Retry; the rail works |
| Adopted fixture | Architecture lists `docs/app-doctrine.md` only; Designs lists `plans/**-design.md` only; the kit's documents absent |
| Self-hosted, `C ≠ S` | Live designs from both `metasystem/plans` and `plans`, labelled by root |

## Change boundary

New: `internal/ui/markdown/` and `internal/ui/project/` with their tests, `testmain_test.go`, and `testdata/`; `src/project/**`. Edited, and nothing else in them: `internal/ui/httpd/httpd.go` (two routes, two `Info` fields) and its `*_test.go` only for the new route cases; `cmd/metasystem/ui.go` (two closures); `go.mod`, `go.sum` (goldmark only); `docs/architecture.md` (two package-map rows, the `ui` family row); `src/routes.ts`, `routes.test.ts`, `src/panes/empties.ts`, `empties.test.ts`, `src/shell/Shell.tsx` (the route block), `src/cuts.test.ts`. `package.json` unchanged. Regenerated as the last commit after rebasing: `bundle/**` (D7).

Must not be touched: `testing-parallel-ratchet.json`, `testing.json`, `internal/testenv`, `testutil`, `parallelratchet`, `testselect`, `testpolicy`, `testexec`, `hostload`, `proofrun`, `gopackages`, `stateroot`, `covenant`, `goal`, `mission`, `ui/lifecycle`, `ui/workspace`, `ui/web` (Go), `cmd/metasystem/test*.go`, `audit.go`, `main.go`, `scripts/**`, `g1-s8`'s `vite.config.ts`, `tsconfig.json`, `scripts/*.mjs`, its policy and page rule, `src/shell/workspace.ts`, and any existing `*_test.go` or `testmain_test.go` outside `internal/ui/httpd`. No frontend dependency is added; no Go dependency but goldmark.

Conventions, because the builder sees only this document: every new Go package has `testmain_test.go` with `func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }`; tests never call `os.Setenv`; every top-level test and independent subtest calls `t.Parallel()`, the ratchet allowing an unlisted package zero serial tests; no sleeps, polling, timers, or fixed ports; `httptest`, `t.TempDir()`, `testutil.Expect` and `Require`; Git fixtures with `git init -q -b main` as `workspace_test.go:164` does; `cmd/metasystem/ui.go` wires only. Frontend: exact versions, `npm ci --ignore-scripts`, the Node pin, Vitest in `node` with no DOM library, no colour literal outside `tokens.css`, no `style` attribute, no `setTimeout`, the bundle rebuilt last. Commands, from `metasystem/`: `go build ./...`, `go vet` and `go test` over `./internal/ui/...` and `./cmd/metasystem/`, the gate's Git-fed gofmt, `bin/metasystem audit parallel-ratchet` without `--update`; from `_app/`: `npm run typecheck`, `npm test`, `npm run bundle` twice with a clean `git status --porcelain` between.

## Verification

**Go, `markdown`**: golden JSON fixtures under `testdata/` for every block and inline kind, slug duplicates, a mermaid fence, CRLF, and empty input; the hostile fixture (`<script>`, `<img onerror>`, an HTML block, `[x](javascript:…)`, `[y](data:…)`, `![z](http://…)`) asserting `html` nodes with source text, `unresolved` targets, the image as `image`; a 1 MiB input parses. **`project`**: two `git init` fixtures under `t.TempDir()`, self-hosted (`<tmp>/metasystem/` beside `development/metasystem-design.md`, `stateroot.go:293`) and adopted at the root, planted with the catalogue's files; the catalogue row by row; absent, symlink, oversized, non-UTF-8, invalid-covenant; the boundary cases above, all `ErrNotFound`; the blob id of `hello\n` against `ce013625030ba8dba906f756967f9e9ca394464a`; every link target kind. **`httpd`**: both payloads with faked `Info` functions; nil functions 500; `ErrNotFound` 404; 404 for `/api/documents` and beneath `/api/project`; headers and policy unchanged.

**Vitest, `node`**: `links.test.ts` (the belt; relative resolution); `outline.test.ts`; `routes.test.ts` gains the `document` kind and encoding; `empties.test.ts` reflects the table (Project gone, Sittings present); `cuts.test.ts` **revised**: the network allowlist becomes exactly `[["project/api.ts", 1], ["shell/workspace.ts", 1]]`, `project/api.ts` names `/api/project` and `/api/documents/`, the DOM-injection identifiers name no file, and every other rule stands.

**Walkthrough**, in a real browser, Claude then the human: build, `ui restart`; `/project`: six cards, Intent and Open questions "Not yet recorded" with paths, Constraints with both rules files and their ownership chips, Sittings' empty state; `docs/architecture.md`, the paper's index, a chapter, back; the master: outline, three anchors, a wide table scrolling within the column; `/project/doc/nothing.md`, `/api/documents/../../etc/passwd`, `/api/documents/metasystem/metasystem.conf.local` refused; edit a document, nothing changes, Reload, the revision changes; one request per page view, none on refocus or over two idle minutes; zero policy violations; keyboard through outline and links; VoiceOver; both themes at 1,280 and 599.

Obligations the code critique checks by name:

- **O1, no HTML anywhere.** The API carries no HTML; goldmark's renderer and `WithUnsafe` unused; the injection identifiers name no file; the hostile fixtures pass in Go and in the browser with zero violations.
- **O2, the boundary.** Every refusal case is a Go test; `EvalSymlinks` containment and `Lstat` regular-file both present; only `.md` served.
- **O3, honest subsections.** "Not yet recorded" only when every source is absent, with the paths; unreadable and too-large rows show their reason; nothing summarises a document; the kit's documents absent in the adopted fixture.
- **O4, identity.** `kind`, `id`, `revision` as specified; the blob literal passes; `routeFor` resolves `document`.
- **O5, one dependency.** `go.mod` gains goldmark at the pinned tag and nothing else; `package.json` unchanged.
- **O6, cuts.** The revised guard passes as written; one request per page view in the browser.
- **O7, layout.** The numbers under Reading layout hold in both themes; tables scroll inside the column.
- **O8, conventions.** `testmain_test.go`, `t.Parallel()` everywhere, no serial test, no timer, Git fixtures under `t.TempDir()`.

## Reconciliation with `g1-s10`

Named for the planner, not settled here: (1) how `/api/` routes join `httpd`, this design following the exact-path and `Info` function pattern, merged if `g1-s10` introduces a mux; (2) the cut guard's allowlist, this design contributing `project/api.ts`; (3) the identity triple, so a goal payload that later carries `DefinitionRefs` uses the same `id` and `blob:` revision; (4) the goal's intent: `Intent` is prose with no typed link to any design (`file.go:30`), so a goal view carries it as source text or, recommended, as this slice's tree, with the line "Governing designs: not recorded until typed references arrive at gate 5" and no link inferred from prose (master `:507`); (5) the `Roots` triple, each package declaring its own; (6) the invalidation view names for the thread and a document, `g1-s7`'s to fix.

## Open questions

Three for the human, each with the recommendation and the consequence; empty before implementation.

1. **D6: the Go tree, or source text at gate 1.** Recommended: the tree, as under Markdown. Source text keeps `go.mod` at two dependencies and leaves the paper and the master unreadable until gate 5 reopens D6.
2. **Where an adopted project's intent lives.** Recommended: the covenant and the `Purpose` line, the two homes the kit declares; a project with neither sees "Not yet recorded" with both paths (master `:488`). A configured list would add a key and a second source of truth.
3. **Whether `plans/**-design.md` is the rule for live designs.** Recommended: yes, because nothing records which designs are live and a narrower rule would guess. Listing only designs referenced from goals is empty until `DefinitionRefs` exists.
