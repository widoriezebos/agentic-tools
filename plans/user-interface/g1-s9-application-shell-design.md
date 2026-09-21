# g1-s9 Application shell

- Gate 1, state `designed` pending Sol's design review (D12, D24), author Claude on Fable as a D23 delegate, 2026-09-21.
- Refines the master at `ui-development` commit `2972f1ec5`: [Agreed interface structure](../user-interface-design.md#agreed-interface-structure) (`:19` to `:43`), [The embedded agent workspace](../user-interface-design.md#the-embedded-agent-workspace) (`:84` to `:108`), [The browser workspace](../user-interface-design.md#the-browser-workspace) and its rules (`:419` to `:449`), [Workspace identity](../user-interface-design.md#workspace-identity-the-subject-and-the-machinery) (`:451` to `:461`), the section descriptions (`:463` to `:689`).
- Depends on `g1-s8`, `g1-s1` revision 6 (merged), D1, D11, D14, D15, D20, D21. The visible half of delivery step 2 (plan `:144`), with `g1-s18` folded in as that step says.
- Discharges nothing alone; contributes the header half of scenario 45 and the resolver's shape for scenario 15.

Paths are relative to `metasystem/` unless they start with `plans/`. Code claims were verified by reading the source at `2972f1ec5`; nothing was executed. Registry facts were read on 2026-09-21.

## Outcome

A human opens `http://127.0.0.1:7878` and sees an application: a rail on the left with Brain above six project sections and Settings at the bottom, a header naming the workspace's subject and mode, a work area showing the selected section, and a Brain dock on the right that resizes, closes, reopens, and expands into the focused conversation view. Every section has a URL that survives a reload and the back button, and is an honest empty state naming what belongs there and which slice fills it. Light and dark follow the system, can be overridden, and never flash on reload. Nothing is fetched from outside the server and no policy violation occurs.

## Scope and non-goals

In scope: the shell at three widths; rail, header, panes, dock, focused Brain view; routing and the resolver's shape; identity chip and tab title; empty states for eight destinations and not-found; tokens, type, spacing, motion, focus; the components below; theme choice and persistence; four items of client state; a connection indicator and a build-changed notice driven by fetches the page already makes.

Not in scope: any read model or `/api/` route beyond the workspace resource contract here, which `g1-s7` owes; conversation and composer behaviour (gate 3); the freshness loop, event stream, refetch on invalidation, and automatic reload (they need `g1-s7`; Open questions); the command palette; sign-in; mutation; Go changes beyond the resource, if folded in.

## Existing code this builds on

| What it means here | Where |
| --- | --- |
| Checks and five headers precede routing; `/-/health` returns `checkout`, `startedAt`, `engineBuild`, `executableDigest`; `/` placeholder; 404 otherwise | `internal/ui/httpd/httpd.go:29` to `:78`, `:146` to `:152` |
| Handler built from the record's checkout, start time, build, digest | `cmd/metasystem/ui.go:122` to `:135` |
| `Roots{Checkout, Installation, StateRoot}`; the record carries them | `internal/ui/lifecycle/roots.go:15`, `:31` to `:69`; `serve.go:150` |
| `Layout.Template` is true only for `<repo>/metasystem` beside `development/metasystem-design.md`; anything else that resolves is adopted | `internal/stateroot/stateroot.go:23` to `:29`, `:126` to `:173`, `:293` to `:299` |
| Key constant, default, typed resolver with a hermetic seam | `internal/config/ui.go:8` to `:35` |
| Engine build stamp | `internal/supervise/disk.go:144` |
| Adopted-from revision: one Markdown line `- Adopted from template SHA:` in `docs/project-rules.md`; this repository holds `<template sha>`; no Go reader | `scripts/adopt.sh:154`, `:161` to `:164`; `docs/project-rules.md:7` |
| Any navigation path outside `/assets/`, `/api/`, `/-/` gets `index.html` with the nonce; `script-src 'self'`, `style-src 'self' 'nonce-…'`; no inline script or style; an extension table | [g1-s8](g1-s8-frontend-toolchain-design.md) `:114`, `:117` to `:138` |
| Already named: React 19, Radix Dialog, `get-nonce`, `lucide-react`, Inter; JetBrains Mono and a panels library for later | g1-s8 `:73` to `:85` |

Registry, 2026-09-21: `react-router` 8.4.0, MIT, declarative components in the one package `react-router`; `react-resizable-panels` 4.13.1, MIT, components `Group`, `Panel`, `Separator`, pixel sizes, `autoSaveId` removed in 4.0 for `defaultLayout` and `onLayoutChanged`, a group prop that disables its cursor-style injection; `@radix-ui/react-tooltip` 1.2.16, MIT; `@fontsource-variable/jetbrains-mono` 5.3.0, OFL-1.1.

## Contracts

### Layout

Three viewport widths: **wide** from 900px, **compact** 600px to 899px, **phone** below 600px. Media queries lay out; `matchMedia` drives behaviour that differs.

Wide, left to right: the rail, then a column holding the header over a horizontal `Group` of work area, separator, dock. Only the work area and dock scroll.

| Part | Size |
| --- | --- |
| Rail | 240px expanded, 56px collapsed; full height; 8px padding; 1px border-right; not resizable; choice remembered |
| Header | 48px; rail edge to viewport edge; 1px border-bottom; 16px side padding |
| Work area | fluid remainder, minimum 480px; 24px padding |
| Separator | 8px hit area, 1px line; `accent` on hover, focus, drag |
| Dock | 400px default, 320px minimum, 640px maximum; 1px border-left; pointer and keyboard resizable; width remembered |
| Dock header | 48px, aligned with the header |

Rail, top to bottom: a 32px row with the collapse toggle; Brain; a 1px rule with 8px margins; Overview, Project, Backlog, Fleet, Decisions, Application; spacer; Settings; a 32px footer row with the theme control. Rows: 32px, 4px apart, 6px radius, 18px icon at 1.75 stroke, 12px gap, label 14px/500. Hover fills `surface-3`; the current row fills `surface-3` with `text` and an `accent` icon; others use `text-2`. Collapsed, labels are visually hidden and tooltips show them.

Header: left, the identity chip, a 1px vertical rule, the section title 16px/600; right, the connection indicator and a 32px icon button that opens or closes the dock. On phone a menu button precedes the chip. The build-changed notice is a 36px bar under the header.

Dock: header with "Brain" 14px/600, a chip "unavailable until gate 3" 12px, icon buttons expand (to `/brain`) and close; body with the unavailability statement and a disabled composer (textarea, disabled Send) with the reason beneath, so the master's shape is visible and the unavailable action carries its reason.

Focused mode is `/brain`: the work area holds the conversation column (max-width 720px, centred; the same statement and composer) and a fixed 400px subject panel on the right with the empty state "No subject selected". The dock is hidden there and the header's dock button is disabled with the tooltip "the conversation is already in focus"; leaving `/brain` restores the remembered dock state.

Compact: rail forced to 56px, toggle hidden; the dock is a sheet from the right at `min(400px, 100vw - 56px)` over a scrim, no separator, closed on every load. Phone: rail hidden; the menu button opens the expanded rail rows as a 280px sheet from the left; the dock sheet is full width. Sheets are Radix `Dialog`: modal, focus trapped, Escape and scrim close, focus returns.

### Navigation

Paths: `/overview` (and `/`, which redirects there with `replace`), `/project`, `/backlog`, `/fleet`, `/decisions`, `/application`, `/settings`, and `/brain` for the focused view. Rail order: Brain; Overview to Application as listed; Settings apart at the bottom.

Router: `react-router` 8 declarative mode (`BrowserRouter`, `Routes`, `Route`, `NavLink`, `Navigate`). A section owns everything beneath its prefix; later slices nest `/backlog/goals/<id>`. Links never emit trailing slashes; a pasted one matches. Anything unmatched renders the not-found pane inside the shell, rail intact, with a link to Overview; the server already answered 200 by g1-s8's rule, so the honesty lives in the pane. A reload keeps the section because the server serves the page for any navigation path; back restores the section through the history API, never dock or rail state.

`src/routes.ts` exports `sections` (path, title, icon), `sectionFor(pathname)`, `pathFor(section)`, and `routeFor(ref: {kind; id?}): string | null`. Today one kind, `section`; later slices register `goal`, `seat`, `question`. `null` means "no view for this reference yet", shown as text. This is the master's frontend-owned resolver (`:325`) at its smallest; nothing else consumes it yet.

### Header and workspace identity

The chip reads `GET /api/workspace`, which **`g1-s7` owes** and plan `:144` says comes with this step:

```json
{"schemaVersion":1,"subject":"MetaSystem","mode":"self-hosted",
 "checkout":"/abs","installation":"/abs","stateRoot":"/abs",
 "engineBuild":"dev-…","startedAt":"RFC 3339","executableDigest":"sha256:…",
 "sourceHead":"","adoptedFrom":""}
```

`mode` is `self-hosted` when `stateroot.ResolveLayout(installation).Template` holds (`stateroot.go:159`, `:169`), else `adopted`. `subject` comes from a new key `ui.subject` in `internal/config/ui.go`, resolved like `ui.listen`; default `MetaSystem` in self-hosted mode, the checkout's base name in adopted mode (Open questions). Path, time, build, and digest fields are the record's (`serve.go:150`). `sourceHead` and `adoptedFrom` are `""` until `g1-s7` reads them (`git rev-parse HEAD`; the `docs/project-rules.md` line, `""` for the placeholder).

| Case | Chip, and tab title |
| --- | --- |
| Self-hosted | subject 14px/600, badge "self-hosted" 12px/500 in `marker-fg` on `marker-bg`, 4px radius; a 2px `marker` stripe along the header's top edge. Title `<Section> · <subject> · self-hosted` |
| Adopted | subject 14px/600, then "built with MetaSystem" 12px in `text-3`; no stripe. Title `<Section> · <subject> · built with MetaSystem` |
| Self-hosted with non-empty `adoptedFrom` | "Workspace identity conflict" in `danger` with a tooltip naming both facts; `danger` stripe. Title `Identity conflict · MetaSystem interface` |
| Loading | 120px skeleton. Title `MetaSystem interface` |
| Fetch failed or 404 | "Workspace unknown" in `text-3` with a retry button; sections still render. Title `<Section> · MetaSystem interface` |

The marker is ochre, not red: a calm constant mark is read more often than an alarm. The conflict row is the master's visible refusal (`:453`) at its cheapest; provenance is `g1-s19`'s.

The connection indicator is an 8px dot with a 12px label: `connected` after a successful fetch, `server unreachable` with a retry button after a failure. The page refetches `/-/health` on window `focus` and `online`, never on a timer. When `executableDigest` differs from the one seen at load, the notice says "The interface server now runs a newer build. Reload to use it." with a Reload button; nothing reloads by itself.

### Empty states

Every pane is `EmptyState`: a 24px icon in `text-3`, heading 16px/600, body 14px in `text-2` at most 480px wide, "Arrives with `<slice>`" 12px in `text-3`, at most one action; centred horizontally, 96px below the pane's top. No pane renders an empty table, list, or endless skeleton. No action unless stated.

| Pane | Heading; body; arrives with |
| --- | --- |
| Overview | Nothing to show yet; what needs you and what changed since your last visit, linked to the records behind each entry; `g1-s14` |
| Project | No project material shown yet; intent, architecture, designs, constraints and assurance, open questions, sittings, from their canonical documents; gate 5 |
| Backlog | No goals shown yet; board, outline, dependencies, and list over the accepted ledger, one workspace per goal; `g1-s10`, detail `g1-s11` |
| Fleet | No seats shown yet; this machine's sessions, jobs, census, and health, each with its observation time and gaps named; `g1-s13` |
| Decisions | No decisions shown yet; open and answered questions from every seat, approvals with authority, delegations, rulings; `g1-s12` |
| Application | No behaviour recorded yet; implemented and released behaviour, observations, and the evidence behind them; gate 6 |
| Settings | Settings pages arrive with gate 7; the named pages listed. Above it, "About this workspace": subject, mode, checkout, engine build, started at, the notices link; "source at HEAD" and "adopted from" read "arrives with `g1-s7`"; an "Appearance" row holds the theme control |
| Brain | The Brain is not connected; conversation, shared context, activity, and actions arrive with gate 3, and the dock and this view show where they will be; the disabled composer |
| Not found | No page at this address; the address matches no section of this workspace; a link to Overview |

### Visual language

From claude.ai: warm neutral paper, a calm reading measure, conversation beside its subject. From the Codex web app: little chrome, one accent on neutrals, dense rows with chips, a collapsible sidebar, skeleton loading. Ours: the ochre marker, Brain fixed above the sections, the token values, the fonts. No logo, brand asset, or licensed font.

Colour tokens are CSS custom properties `--ms-<name>`, mapped into Tailwind 4 by `@theme inline`. Every token is defined on `:root` (light) and `:root[data-theme="dark"]`; nothing else defines colour.

| Token | Light | Dark | Use |
| --- | --- | --- | --- |
| `bg` | `#F7F6F3` | `#191917` | canvas, work area |
| `surface` | `#FFFFFF` | `#201F1D` | cards, dock body, sheets |
| `surface-2` | `#F1EFEA` | `#262523` | rail, header, dock header |
| `surface-3` | `#E9E6DF` | `#2E2D2A` | hover, current row |
| `border` | `#DDD9D0` | `#35342F` | 1px rules |
| `border-strong` | `#C8C3B8` | `#47453F` | inputs, separator line |
| `text` | `#1F1E1B` | `#ECEAE4` | primary |
| `text-2` | `#5B5850` | `#B4B0A6` | secondary, inactive rows |
| `text-3` | `#8A867C` | `#7F7B72` | muted, empty-state icons |
| `accent` | `#3D6DE0` | `#7A9BF5` | links, current icon, focus, separator active |
| `accent-hover` | `#2F5BC7` | `#93AEF8` | |
| `accent-fg` | `#FFFFFF` | `#10131F` | text on accent |
| `accent-soft` | `#E6EDFC` | `#22304F` | selected chips |
| `marker` | `#B7791F` | `#D9A441` | self-hosted stripe, warnings |
| `marker-bg` / `marker-fg` | `#FBF1DC` / `#7A4F0E` | `#3A2E14` / `#F0CB7A` | self-hosted badge |
| `ok` | `#2E8B57` | `#57B37F` | connected dot |
| `danger` | `#C0392B` | `#E06B5C` | unreachable, conflict, error boundary |
| `scrim` | `rgba(31,30,27,.4)` | `rgba(0,0,0,.6)` | behind sheets |
| `shadow` | `0 8px 24px rgba(0,0,0,.12)` | `0 8px 24px rgba(0,0,0,.5)` | overlays only |

Type: Inter variable everywhere; JetBrains Mono variable for identifiers, digests, paths, the composer; both OFL-1.1, bundled as g1-s8 bundles Inter. Scale `xs 12/16`, `sm 13/18`, `base 14/20`, `md 16/24`, `lg 20/28`, `xl 24/32`; weights 400, 500, 600; `xl` letter-spacing −0.01em; 14px on `html`.

Spacing on 4px: 4, 8, 12, 16, 20, 24, 32, 40, 48. Radii: 4 chips and badges; 6 buttons, inputs, rail rows; 8 cards and panels; 12 sheets and dialogs. Borders 1px `border`; surfaces flat; `shadow` only on tooltips, sheets, dialogs. Density: 32px rows and buttons, 48px bars, 36px table rows later.

Focus: `:focus-visible` gives a 2px solid `accent` outline at 2px offset following the element's radius; no component removes it. Motion: 120ms hover and colour, 180ms rail collapse and dock open, easing `cubic-bezier(.2,0,0,1)`; resize follows the pointer untransitioned; the skeleton pulses at 1.2s. Under `prefers-reduced-motion: reduce` every duration is 0 and the skeleton is static.

Icons: `lucide-react`, 18px in the rail, 16px in buttons: Brain `MessageSquare`, Overview `LayoutDashboard`, Project `BookOpen`, Backlog `SquareKanban`, Fleet `Server`, Decisions `CircleHelp`, Application `Package`, Settings `Settings`, dock `PanelRight`, rail `PanelLeft`, `Menu`, `Maximize2`, `X`, `Monitor`, `Sun`, `Moon`. Not verified against the installed version; the builder substitutes the nearest name and lists substitutions in the evidence note.

### Components

Ours unless marked.

| Component | Accessibility contract |
| --- | --- |
| `Shell` | Grid, breakpoints, landmarks; a skip link "Skip to content" first in the DOM, visible on focus, targeting `main` |
| `Rail`, `RailItem` | `<nav aria-label="Sections">`, a list of `NavLink`s with `aria-current="page"`; the label stays in the DOM, visually hidden when collapsed, so a reader hears "Backlog, link, current page" either way; the toggle has `aria-expanded` and `aria-controls` |
| `Header`, `WorkspaceIdentity`, `ConnectionIndicator`, `Notice` | `<header>`; badge text is part of the chip; indicator and notice are `role="status"` with `aria-live="polite"` |
| `Pane` | `<main id="content" tabIndex={-1}>` with an `h1` naming the section |
| `EmptyState` | Heading, body, slice line, optional action; nothing else |
| `Dock`, `DockHeader` | `<aside id="brain-dock" aria-label="Brain dock">`; buttons "Expand the Brain view", "Close the Brain dock"; the textarea labelled "Message to the Brain", `disabled`, `aria-describedby` the reason |
| `Group`, `Panel`, `Separator` (library) | Separator focusable, `role="separator"`, `aria-orientation="vertical"`, `aria-label="Resize the Brain dock"`, the library's value attributes; arrow keys change the width, verified by the builder in the installed version, else a key handler through the group ref's `setLayout`; cursor injection disabled by the group prop and `cursor: col-resize` from our stylesheet, so no style element is injected |
| `Sheet` (Radix `Dialog`) | Modal; `aria-label` "Sections" or "Brain dock"; focus trapped and returned; Escape and scrim close; its scroll-lock style takes the nonce through g1-s8's `setNonce` |
| `Tooltip` (Radix) | On every icon-only button and collapsed rail item; hover and focus; never the only label |
| `IconButton` | Requires `aria-label`; 32px; wrapped in `Tooltip` |
| `ThemeControl` | A group labelled "Theme" of three 32px buttons System, Light, Dark with `aria-pressed`; also on Settings |
| `Skeleton` | `aria-hidden` |
| `ErrorBoundary` | Class component around `Pane` and around `Dock`; shows "This pane could not be rendered", the error in mono, a "Reload page" button; logs to console |
| `NotFound` | The pane above |

Focus order: skip link; rail (toggle, Brain, sections, Settings, theme); header (menu on phone, retry when shown, dock button); notice button when shown; `main`; separator; dock buttons; composer.

### Theme

Preference `system`, `light`, or `dark` under `ms.ui.theme`; absent means `system`. The effective theme is `data-theme="light|dark"` on `<html>`; CSS has one light block and one dark block and no media query: `src/theme.ts` resolves `system` with `matchMedia("(prefers-color-scheme: dark)")` and listens for `change`. `color-scheme` is set with the tokens so native controls follow.

No flash without inline script: `_app/public/theme.js`, a classic script of about twelve lines referenced as `<script src="/theme.js"></script>` in `<head>` before Vite's stylesheet link, reads the key inside `try`, resolves `system` the same way, and sets the attribute before first paint. Vite copies `public/` to the output root (documented behaviour, not re-read), so it is served as `/theme.js` under the extension table, allowed by `script-src 'self'`, and passes `TestIndexIsStrict`. `theme.ts` exports the key and attribute name; a test asserts `theme.js` contains both literals.

### State

| State | Key in `localStorage` | Default | Remembered |
| --- | --- | --- | --- |
| Section | the URL, not storage | `/overview` | by history |
| Theme | `ms.ui.theme` | `system` | yes |
| Rail | `ms.ui.rail` | `expanded` | yes; ignored below wide |
| Dock open | `ms.ui.dock` | `open` | on wide; sheets open closed |
| Dock width | `ms.ui.dock.width` | `400` | clamped to 320 to 640 on read; written on release via `onLayoutChanged` |

Every access is in `try`; an invalid value yields the default. Storage is per origin and each workspace server is its own origin, so keys carry no workspace prefix. No server state, cookie, or authority: view state only (plan `:62`). Workspace and health responses live in React state.

### Dependencies added

Runtime, bundled and listed by g1-s8's notices script: `react-router` 8 (MIT), `react-resizable-panels` 4 (MIT), `@radix-ui/react-tooltip` (MIT), `@fontsource-variable/jetbrains-mono` (OFL-1.1). No development dependency. No Go module dependency.

## Behaviour

**Load.** `theme.js` sets the theme; `main.tsx` sets the nonce (g1-s8), reads the stored values, mounts `BrowserRouter` and `Shell`. The chip shows a skeleton and fetches `/api/workspace`; on success it renders the identity and tab title and the indicator turns `connected`. The pane renders from the URL at once and never waits. **Navigate.** A rail click or typed URL selects a section; `h1`, title, and tab title change; dock and rail keep their state; `/brain` hides the dock. **Dock.** The header button and the close button toggle `ms.ui.dock`; dragging or arrowing the separator changes the width live and release writes it; expand navigates to `/brain`.

| Case | Required result |
| --- | --- |
| `/api/workspace` is 404 (server predates the route) or the fetch fails | "Workspace unknown" with retry; `server unreachable` only for a failed fetch; sections render; title falls back |
| Self-hosted with non-empty `adoptedFrom` | Conflict chip and stripe; nothing else changes |
| Unknown path, including `/backlog/anything` before `g1-s10` | Not-found pane in the shell; 200 from the server |
| Reload on `/decisions`; back after three sections | The same section; the previous ones in order |
| Stored width `12`, `9999`, `"abc"` | 320, 640, 400 |
| `localStorage` throws | Defaults; no error surfaces; no write that load |
| Narrowed below 900px with the dock open | Dock closes into sheet mode; stored value untouched; widening restores it |
| A pane throws | Its boundary shows the statement; rail, header, dock keep working |
| Digest changes between health fetches | The notice; no automatic reload |
| OS theme changes under `system` | Attribute updates on `change`; a stored `light` or `dark` ignores it |
| Drag, sheet, tooltip | Zero `securitypolicyviolation` events |
| Keyboard only from the address bar | Skip link first; every control reachable with visible focus; Enter activates; Escape closes a sheet and returns focus |

## Change boundary

New under `internal/ui/web/_app/`: `public/theme.js`; `src/routes.ts`, `theme.ts`, `storage.ts`, `title.ts`, `tokens.css` with the Tailwind mapping, `src/shell/**`, `src/panes/**`, their `*.test.ts`. Edited there: `index.html` (one script tag), `package.json`, `package-lock.json`, `App.tsx` and `main.tsx` (mount the shell; the g1-s8 dialog demo goes, the notices link moves to Settings), `fonts.css` (the mono face). Regenerated as the last commit after rebasing: `bundle/**` (D7).

Only if the human folds the resource in: new `internal/ui/workspace/` with `Describe(roots lifecycle.Roots, rec lifecycle.Record, subject string) Workspace`, tests, `testmain_test.go`; `internal/ui/httpd/` gains `/api/workspace`, exact path, 404 beneath; `internal/config/ui.go` gains `UISubjectKey` and `UISubject(confPath, checkout string, selfHosted bool)` with a test; `cmd/metasystem/ui.go` wires it; `docs/architecture.md` one row; `metasystem.conf` `# ui.subject=` with one sentence.

Must not be touched: `testing-parallel-ratchet.json`, `testing.json`, `go.mod`, `go.sum`, `internal/testenv`, `internal/testutil`, `internal/parallelratchet`, `internal/testselect`, `internal/testpolicy`, `internal/testexec`, `internal/hostload`, `internal/proofrun`, `internal/gopackages`, `internal/stateroot`, `internal/ui/lifecycle`, `cmd/metasystem/test*.go`, `cmd/metasystem/audit.go`, `cmd/metasystem/main.go`, `scripts/**`, g1-s8's `vite.config.ts`, `tsconfig.json`, `scripts/*.mjs`, its policy and page rule, and any existing `*_test.go` or `testmain_test.go` outside `internal/ui/httpd`, whose route cases may gain the new route only.

Conventions, repeated because the builder sees only this document: every new Go package has `testmain_test.go` calling `testenv.Main`; tests never call `os.Setenv`; every top-level test and independent subtest calls `t.Parallel()`, the ratchet allowing an unlisted package zero serial tests; no sleeps, polling, or fixed ports; `t.TempDir()`; `testutil.Expect` and `Require`; `cmd/metasystem/ui.go` wires and prints only. Frontend: exact versions, `npm ci`, the Node pin, Vitest in the `node` environment with no DOM library, no colour literal outside `tokens.css`, no `style` attribute in markup (React `style` props for dock width and sheet size are CSSOM writes and allowed), no `title` attribute as a label, the bundle rebuilt by `npm run bundle` as the last commit.

## Verification

Vitest, `node`: `routes.test.ts` (every section both ways, `/`, trailing slash, nested prefix, unknown, `routeFor` for `section` and an unknown kind); `title.test.ts` (five cases); `theme.test.ts` (preference and system to effective; garbage to `system`); `storage.test.ts` (the clamp rows; a throwing storage yields defaults); `themeScript.test.ts` (`theme.js` holds the key and attribute literals from `theme.ts`, no `http`); `tokens.test.ts` (every `--ms-` name in `:root` appears in the dark block and vice versa); `literals.test.ts` (no `#hex` or `rgb(` under `src/` outside `tokens.css`).

Go: g1-s8's `web` and `httpd` tests pass on the rebuilt bundle, `TestIndexIsStrict` included. If the resource is folded in: a table test over the three layouts under `t.TempDir()` for `mode` and default subject; the route's payload, the 404 beneath it, the five headers and the policy on it; `ui.subject` default and override through the hermetic seam.

Only a human or a browser judges the rest. Walkthrough, in order, by Claude in a real browser (Playwright allowed, D24), then the human: (1) build, `ui start`, open the address, land on `/overview` with rail, header, identity, dock; (2) click each section, read each empty state against the table, watch URL and tab title; (3) reload on `/decisions`, press back three times; (4) open `/nothing/here`; (5) drag the separator to both limits, reload, close the dock, reload, reopen from the header; (6) Brain entry, expand from the dock header, back; (7) themes: System, Dark, reload and confirm no flash, Light, System, flip the OS theme; (8) narrow to 800px then 500px, open both sheets; (9) keyboard only from the address bar through the whole focus order, arrows on the separator, Escape on a sheet; (10) VoiceOver over the landmarks and one rail item; (11) rebuild, `ui restart`, refocus the tab, see the notice, reload; (12) `ui stop`, refocus, see `server unreachable`, `ui start`, retry; (13) devtools throughout: zero policy violations, no request off the origin, `/THIRD-PARTY-NOTICES.txt` names the four packages; (14) `npm run check:modal` still passes.

Obligations the code critique checks by name:

- **O1, stable links.** Every destination has exactly its path, reachable by rail and URL, restored on reload; `routes.test.ts` passes.
- **O2, honest empties.** Every pane renders its heading, body, and slice line verbatim; no empty list, table, or endless skeleton anywhere.
- **O3, no inline, nothing external.** `TestIndexIsStrict` passes with the `theme.js` tag; `literals.test.ts` passes; no `style` attribute in markup.
- **O4, no violation on interaction.** Cursor injection disabled; a drag, a sheet, and a tooltip produce zero `securitypolicyviolation` events in the embedded build.
- **O5, theme without flash.** `theme.js` precedes the stylesheet in `<head>`; `themeScript.test.ts` and `tokens.test.ts` pass.
- **O6, keyboard and labels.** The focus order holds; every icon-only control has `aria-label`; the separator resizes from the keyboard; sheets trap and return focus.
- **O7, bounded state.** `storage.test.ts` passes; every access is wrapped; the four keys are the only ones written.
- **O8, identity cases.** The five chip cases render as specified from a faked response, the conflict included.
- **O9, bundle current and attributed.** The last commit rebuilds the bundle; `go test ./internal/ui/web/` passes; the notices file names the four packages.
- **O10, resource**, only if folded in. `mode` comes from `ResolveLayout(...).Template`; the layout table test passes; the handler loosens no check and drops no header.

Commands, from `metasystem/`: `go build ./...`; `go vet ./internal/ui/...`; `go test ./internal/ui/...`; `gofmt -l internal cmd`; `bin/metasystem audit parallel-ratchet` without `--update`, noted if it does not run. From `internal/ui/web/_app/`: `npm ci`, `npm run typecheck`, `npm test`, `npm run bundle` twice with a clean `git status --porcelain` between.

## Open questions

1. **Who builds the workspace resource.** Recommended: fold the thin resource specified here into this step's build under `g1-s7`'s name, as plan `:144` says. Alternative: ship with "Workspace unknown" until `g1-s7`, so the first thing the human sees is a placeholder.
2. **Default subject in self-hosted mode.** Recommended: `MetaSystem`, matching the master's example and vocabulary, with `ui.subject` overriding. Alternative: the base name `agentic-tools-ui`, the master's sentence read literally, which puts a repository name where the product's is expected.
3. **Splitting the `g1-s9` row.** This is the static cut of step 2. Recommended: move freshness, refetch on invalidation, and automatic reload into a second cut after `g1-s7`, and mark `g1-s18` folded. Alternative: the row keeps claiming work no slice delivers.
4. **Router.** Recommended: `react-router` 8, because `g1-s10` and `g1-s11` need nested routes, parameters, and `NavLink` within weeks. Alternative: sixty lines over `pushState`, smaller today, replaced at `g1-s11`.
5. **Reload on a build change.** Recommended: the notice bar; a silent reload while reading is rude and nothing is lost by waiting. Alternative: the row's literal automatic reload.
