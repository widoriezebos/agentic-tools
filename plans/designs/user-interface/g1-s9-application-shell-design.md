# g1-s9 Application shell

- Kind: design
- Id: 01M348YTJ26D23C5903WBVR23E
- Status: done
- Goals: browser-interface

- Gate 1, state `designed`, **revision 3**, author Claude on Fable as a D23 delegate, 2026-09-21. Revision 3 closes Sol's round 2, J1 to J3 ([review](g1-s9-application-shell-design-sol-review.md), Round 2): the dock keeps its pixel width across a live window resize (`groupResizeBehavior`, the giving-way order, a wide-to-wide walkthrough step); every empty state names its kind under the master's two-kind rule; the second cut's invalidation subscription is scoped, specified, tested, and refused by the cut guard. The contrast table, the layout arithmetic, and the eight closed findings are untouched. Revision 2 applied Sol's nine material findings and F11: adoption reader, layout arithmetic, explicit cuts, dock persistence per the 4.13.1 API, keyboard contract, contrast tokens, component states, honest empty states, reserved paths, separator keys.
- Refines the [master](../user-interface-design.md) at `ui-development` commit `479c1ba54`: Agreed interface structure (`:19` to `:43`), The embedded agent workspace (`:84` to `:108`), The browser workspace (`:419` to `:449`), Workspace identity (`:451` to `:461`), the sections (`:463` to `:689`); and, for the empty-state rule, Rules for an intuitive structure at `:437` as the planner settled it at `5279a9d0c` (M9).
- Depends on `g1-s8`, `g1-s1` revision 6 (merged), D1, D11, D14, D15, D20, D21, D27, D28. The visible half of delivery step 2 (plan `:147`), `g1-s18` folded in. Contributes to scenarios 45 and 15.

Paths are relative to `metasystem/` unless they start with `plans/`. Code claims were verified in source at `479c1ba54`, library claims in the `react-resizable-panels` `4.13.1` tag, revision 3's in the `4.13.1` source archive from GitHub, read function by function, because the package is not installed in this checkout; the master and the plan are cited at `5279a9d0c` where revision 3 relies on them (the plan's rows moved by four lines between the two commits: delivery step 2 is now `:151`, the goal count `:152`, the `g1-s9` row `:187`); nothing was executed; registry read 2026-09-21. `g1-s8` is under revision, so it is cited by section.

## Outcome

A human opens `http://127.0.0.1:7878` and sees an application: a rail on the left with Brain above six project sections and Settings at the bottom, a header naming the workspace's subject and mode, a work area showing the selected section, and a Brain dock on the right that resizes, closes, and expands into the focused conversation view. Every section has a URL that survives reload and back, and is an honest empty state saying this build does not project it yet and which slice will. Light and dark follow the system, can be overridden, and never flash on reload.

## Scope and the two cuts

D28 settled revision 1's open questions; nothing is conditional. Two cuts (plan `:187` at `5279a9d0c`). **First cut**, this build: the shell at three widths; rail, header, panes, dock, focused view; routing and the resolver; the thin workspace resource with its adoption reader under `g1-s7`'s name; identity chip and title; empty states; tokens, type, motion, focus; the components; theme; four items of client state; one network call, `GET /api/workspace`, on load and on Retry, and no other request, stream, socket, timer, or window-event refetch. **Second cut**, after `g1-s7`: the connection indicator; `/-/health` refetched on window `focus` and `online`; the rebuilt-executable notice; and refetch on invalidation, which is a subscription to the event stream `g1-s7` publishes (plan `:185`: "a server-sent event stream carries invalidation only, naming the affected view") that refetches `/api/workspace` when the stream names the workspace view. All four are specified here, marked *second cut*, and absent from the first cut, whose guard refuses them under any name (Verification). Out of scope: other `/api/` routes; conversation and composer behaviour (gate 3); the stream itself, its path, transport, event names, and payload, which are `g1-s7`'s to design; automatic reload (D28); the command palette; sign-in; mutation.

## Existing code this builds on

| What it means here | Where |
| --- | --- |
| Checks and headers precede routing; `/-/health`; `/` placeholder; 404 otherwise | `internal/ui/httpd/httpd.go:29` to `:78` |
| Handler built from the record: `Roots{Checkout, Installation, StateRoot}`, build stamp, digest | `cmd/metasystem/ui.go:122` to `:135`; `internal/ui/lifecycle/roots.go:15`; `serve.go:150`; `state.go:24` to `:32` |
| `Layout.Template` only for `<repo>/metasystem` beside `development/metasystem-design.md`; else adopted | `internal/stateroot/stateroot.go:126` to `:173`, `:293` to `:299` |
| Key resolver with a seam; `Get` without a default errors | `internal/config/ui.go:20` to `:34`; `resolve.go:205` to `:208` |
| Adopted-from: one line `- Adopted from template SHA:` in `docs/project-rules.md` at the installation root, `<template sha>` until filled; read only by the script's `grep` and the reconciliation checklist; no Go reader; the payload is `git archive HEAD`, so every installation has the file | `scripts/adopt.sh:161` to `:172`, `:230`, `:232`, `:443`; `docs/project-rules.md:7`; `docs/metasystem-reconciliation.md:70`; `internal/contract/contract.go:624` |
| Page rule: any path accepting HTML outside `/-`, `/api`, `/assets` gets `index.html` with the nonce; those prefixes 404 unless health or a static file; `script-src 'self'`, `style-src 'self' 'nonce-…'` | [g1-s8](g1-s8-frontend-toolchain-design.md): Serving, Content security policy, The build |

Registry: `react-router` 8.4.0 (MIT); `react-resizable-panels` 4.13.1 (MIT); `@radix-ui/react-tooltip` 1.2.16 (MIT); `@fontsource-variable/jetbrains-mono` 5.3.0 (OFL-1.1). No development or Go dependency is added.

## Contracts

### Layout

Widths: **wide** from 960px, **compact** 600 to 959, **phone** below 600. Media queries lay out; `matchMedia` drives behaviour that differs. Inside wide the rail expands only from **1,128px**; below that it is collapsed, toggle hidden, stored choice untouched.

Wide, left to right: the rail, then a column holding the header over a horizontal `Group` of work area, separator, dock. Only the work area and dock scroll. Rail, header, and dock header are `surface-2`; the rail has a 1px `border` right, both headers a 1px `border` below.

| Part | Size |
| --- | --- |
| Rail | 240 expanded, 56 collapsed; full height; 8px padding; not resizable; choice remembered |
| Header | 48; rail edge to viewport edge; 16px side padding |
| Work area | fluid, minimum 480 in the docked layout; 24px padding, 16 on phone |
| Separator | 8px hit area, 1px line; 2px `accent` on hover, focus, drag |
| Dock | 400 default, 320 minimum, 640 maximum; pointer and keyboard resizable; width remembered; keeps its pixel width when the window resizes, the work area giving way first (State, Live resize); header 48, 12px side padding |

**Arithmetic**, rail + 8 + work area + dock: 56 + 8 + 480 + 320 = **864**; 56 + 8 + 480 + 400 = **944**; 240 + 8 + 480 + 320 = **1,048**; 240 + 8 + 480 + 400 = **1,128**; the 640 dock needs **1,184** collapsed, **1,368** expanded. So wide starts at 960 (work area 496 beside the default dock) and the expanded rail at 1,128 (exactly 480). The dock's effective maximum is `viewport − rail − 8 − 480`: 416 at 960, 400 at 1,128, 640 from 1,184 collapsed. Both minimums are library constraints (`validatePanelGroupLayout` clamps each panel and hands the remainder to the other), and the page clamps the stored width to that room before mounting (State).

Rail, top to bottom: a 32px row with the collapse toggle; Brain; a 1px `border` rule, 8px margins; Overview, Project, Backlog, Fleet, Decisions, Application; spacer; Settings; a 32px footer row with the theme control. Header: left, on phone a menu button; the identity chip; a 1px vertical `border` rule 16px tall, 12px margins; the section title 16px/600; right, the dock toggle; *second cut*: the connection indicator before it and the 36px notice bar under the header. Dock: header "Brain" 14px/600, chip "unavailable until gate 3", Expand (to `/brain`) and Close icon buttons; body `surface`, padding 16: the Brain empty state, the disabled composer beneath.

Focused mode, `/brain`. Wide: the conversation column (minimum 480, content max-width 720 centred; the same statement and composer) beside a fixed 400px subject panel on `surface`, 1px `border` left, holding "No subject selected": 56 + 480 + 400 = 936 ≤ 960, 240 + 480 + 400 = 1,120 ≤ 1,128. Compact and phone: one column under a 32px switch of two `Button`s "Conversation" and "Subject" with `aria-pressed`, Conversation on entry, not remembered (master `:88`). The dock is hidden; the header's dock toggle stays focusable with `aria-disabled="true"`, tooltip "The conversation is already in focus" on hover and focus, click inert; leaving `/brain` restores the dock.

Compact: rail 56, toggle hidden; the dock is a sheet from the right at `min(400px, 100vw − 56px)` over a scrim, no separator, closed on every load. Phone: rail hidden; the menu button opens the expanded rail rows as a 280px sheet from the left; the dock sheet is full width. Sheets are Radix `Dialog`, modal.

### Navigation

Paths, in rail order after Brain: `/overview` (and `/`, redirecting there with `replace`), `/project`, `/backlog`, `/fleet`, `/decisions`, `/application`, `/settings`; `/brain` is the focused view. Router: `react-router` 8 declarative mode (`BrowserRouter`, `Routes`, `Route`, `NavLink`, `Navigate`). A section owns everything beneath its prefix; later slices nest `/backlog/goals/<id>`. Links never emit trailing slashes; a pasted one matches.

Reserved prefixes `/-`, `/api`, `/assets`, exact or beneath, are the server's (g1-s8, Serving): no `sections` entry, link, or `routeFor` result starts with one, and the not-found contract excludes them; `/api/nope`, `/-/nope`, or `/assets/nope` typed shows the server's plain-text 404, no shell, on reload too. Every other path whose request accepts HTML gets the page with 200; the router renders the section or, unmatched, the not-found pane inside the shell, rail intact. Reload keeps the section; back restores the section, never dock or rail state.

`src/routes.ts` exports `sections` (path, title, icon), `sectionFor(pathname)`, `pathFor(section)`, `isReserved(pathname)`, and `routeFor(ref: {kind; id?}): string | null`, the master's resolver (`:325`) at its smallest: one kind, `section`; later slices register `goal`, `seat`, `question`; `null` means "no view yet", shown as text.

### Header and workspace identity

The chip reads `GET /api/workspace`, unconditional in this build (D28):

```json
{"schemaVersion":1,"subject":"MetaSystem","mode":"self-hosted","conflict":false,
 "checkout":"/abs","installation":"/abs","stateRoot":"/abs",
 "engineBuild":"dev-…","startedAt":"RFC 3339","executableDigest":"sha256:…",
 "sourceHead":"","adoptedFrom":"","adoptionRecord":"placeholder"}
```

`mode` is `self-hosted` when `stateroot.ResolveLayout(installation).Template` holds (`stateroot.go:159`, `:169`), else `adopted`. `subject` is the new key `ui.subject`, resolved like `ui.listen` with the same seam and an empty default; when empty, `MetaSystem` self-hosted and the checkout's base name adopted (D28). The other fields are the record's; `sourceHead` is `""` until `g1-s7` reads `git rev-parse HEAD`.

**Adoption reader.** `adoptedFrom` and `adoptionRecord` come from a reader in the resource, not from failing identity closed: closed identity would make every self-hosted first screen a refusal, the placeholder D28 rejected, and the reader is twenty lines over the one line `adopt.sh:162` to `:164` and `metasystem-reconciliation.md:70` already read. `workspace.ReadAdoption(installation)` opens `<installation>/docs/project-rules.md`, the path the script writes (`:161`, `:443`) in every layout (here `metasystem/docs/project-rules.md`): no file → `absent`; read error → `unreadable`; else the first line beginning `- Adopted from template SHA:`, the text between its first backticks: `<template sha>` → `placeholder`; forty lowercase hex → `recorded` with the SHA; else `unreadable`; no line → `absent`. `conflict` is `mode == "self-hosted" && adoptionRecord == "recorded"`, computed in Go so the rule has one owner: D20's copied-marker case. Genuine provenance stays `g1-s19`'s.

| Case | Chip, and tab title |
| --- | --- |
| Self-hosted, no conflict | subject 14px/600, `Chip` "self-hosted" in `marker-fg` on `marker-bg`; a 2px `marker` stripe along the header's top edge. `<Section> · <subject> · self-hosted` |
| Adopted | subject 14px/600, "built with MetaSystem" 12px `text-3`; no stripe. `<Section> · <subject> · built with MetaSystem` |
| `conflict` | "Workspace identity conflict" 14px/600 `danger`, then visible 12px `text-2` "self-hosted layout, adopted from `<seven of the SHA>`", never a tooltip; `danger` stripe. `Identity conflict · MetaSystem interface` |
| Loading; failed, 404, or 500 | A skeleton, `MetaSystem interface`; then "Workspace unknown" 14px `text-2`, a Retry `Button`, the message 12px `text-3` beneath; sections render; `<Section> · MetaSystem interface` |

The marker is ochre, not red: a calm constant mark is read more often than an alarm; the conflict row is the master's visible refusal (`:453`). *Second cut*: the indicator shows `connected` or `server unreachable` with Retry from the last `/-/health` fetch, refetched on `focus` and `online`, never on a timer; when `executableDigest` differs from the one seen at load, the notice reads "The interface server now runs a newer build. Reload to use it." with a Reload button; nothing reloads by itself. *Second cut*, invalidation: `src/shell/events.ts` opens one `EventSource` to the stream `g1-s7` publishes and, on an event naming the workspace view, calls `loadWorkspace()`, the function `workspace.ts` exports, the chip calls on mount, and Retry calls again, so the chip, the title, and Settings' About update in place; an event naming any other view, or one the page cannot parse, is ignored, because this build fetches nothing else; after the stream drops and reopens, `loadWorkspace()` and the health fetch run once each (master `:333`: on reconnect, read the relevant current records); the browser's own `EventSource` reconnection is the only retry, the page sets no timer. Unverified here, and marked so: the stream's path, its event and view names, and its payload, because `g1-s7` is `defined`, not designed (plan `:185`); the second cut's build takes them from `g1-s7`'s design and changes nothing else in this contract.

### Empty states

Every pane is `EmptyState` (Components). The master distinguishes two kinds of emptiness, because only one of them has a first action (master `:437` at `5279a9d0c`, the planner's M9): a section whose records do not exist yet offers the action that would create them; a section this build does not project yet is not empty at all, since the records may well exist, so it states that the view is absent and which delivery gate brings it, offers no action of its own, because every candidate would either mislead or send the human to a terminal, and may link to what this build does provide. Every section here is the second kind, **not projected**: nothing in this build reads the records, so no pane can claim they are absent, and they are not (plan `:152`: 156 live and 430 archived goals). So `none` in the action column is the rule, not an omission. A link, where the table offers one, is a text link to a surface this build serves, never a `Button` and never a first action. Overview gets one because a human who has just arrived lands there (`/` redirects to it) and About this workspace, with the checkout, installation, state root, engine build, and start time, is this build's one live answer to "what is this"; nowhere else does a live surface help more than the header already does, so no other link is invented. Not found is not a section and is neither kind: an address that matches nothing has one honest next step, and keeps its action. No pane renders an empty table, list, or endless skeleton.

| Pane | Kind | Heading; body; arrives with; link; action |
| --- | --- | --- |
| Overview | not projected | Overview is not projected yet; This build does not read the records. Overview will show what needs you and what changed since your last visit, linked to its records; `g1-s14`; link "About this workspace" to `/settings`; none |
| Project | not projected | Project is not projected yet; Intent, architecture, designs, constraints and assurance, open questions, and sittings are in the repository and will be read here; gate 5; no link; none |
| Backlog | not projected | The backlog is not projected yet; Goals exist at the accepted tip. Board, outline, dependencies, list, and one workspace per goal will read them here; `g1-s10`, detail `g1-s11`; no link; none |
| Fleet | not projected | Fleet is not projected yet; This machine's sessions, jobs, census, and health are recorded and will be shown with observation times and gaps; `g1-s13`; no link; none |
| Decisions | not projected | Decisions are not projected yet; Questions from every seat, approvals, delegations, and rulings are in the ledger and will be read here; `g1-s12`; no link; none |
| Application | not projected | Application is not projected yet; Implemented and released behaviour and its evidence are recorded and will be shown here; gate 6; no link; none |
| Settings | not projected (the nested state; the live cards above it are what this build provides) | Live: "About this workspace" (subject, mode, checkout, installation, state root, engine build, started at, the notices link, "source at HEAD: arrives with `g1-s7`", "adopted from": the SHA, "not recorded" for `placeholder` or `absent`, else "unreadable") and "Appearance" (the theme control); beneath, `EmptyState`: Settings pages are not built yet; Runtimes and models, connections and channels, identity and authority, execution defaults, storage and retention; gate 7; no link; none |
| Brain | not projected | The Brain is not connected in this build; Conversation, shared context, activity, and actions; gate 3; no link; none (the disabled composer beneath, with its reason) |
| Subject (focused) | not projected | No subject selected; The subject panel will show the artifact under discussion; gate 3; no link; none |
| Not found | neither; not a section | No page at this address; The address matches no section of this workspace; the address bar; no link; "Go to Overview" |

### Visual language

Warm neutral paper, a calm measure, one accent on neutrals, dense rows with chips, a collapsible sidebar, skeleton loading (D14). No logo, brand asset, or licensed font. Colour tokens are CSS custom properties `--ms-<name>`, mapped into Tailwind 4 by `@theme inline`, defined on `:root` (light) and `:root[data-theme="dark"]`; nothing else defines colour. Revision 2 changes `border-strong`, `text-3`, and the light `accent` pair.

| Token | Light | Dark |
| --- | --- | --- |
| `bg` | `#F7F6F3` | `#191917` |
| `surface` | `#FFFFFF` | `#201F1D` |
| `surface-2` | `#F1EFEA` | `#262523` |
| `surface-3` | `#E9E6DF` | `#2E2D2A` |
| `border` | `#DDD9D0` | `#35342F` |
| `border-strong` | `#8A857B` | `#7A776F` |
| `text` | `#1F1E1B` | `#ECEAE4` |
| `text-2` | `#5B5850` | `#B4B0A6` |
| `text-3` | `#6B675E` | `#9A958B` |
| `accent` / `accent-hover` | `#3462D9` / `#2A55BF` | `#7A9BF5` / `#93AEF8` |
| `accent-fg` | `#FFFFFF` | `#10131F` |
| `marker` | `#B7791F` | `#D9A441` |
| `marker-bg` / `marker-fg` | `#FBF1DC` / `#7A4F0E` | `#3A2E14` / `#F0CB7A` |
| `ok` | `#2E8B57` | `#57B37F` |
| `danger` | `#C0392B` | `#E06B5C` |
| `scrim` | `rgba(31,30,27,.4)` | `rgba(0,0,0,.6)` |
| `shadow` | `0 8px 24px rgba(0,0,0,.12)` | `0 8px 24px rgba(0,0,0,.5)` |

`bg` canvas and work area; `surface` cards, dock body, sheets, subject panel; `surface-2` rail, header, dock header; `surface-3` hover, current row, skeleton; `border` rules; `border-strong` input boundary and the separator's resting line; `accent` links, current icon, focus, active separator; `marker` the stripe only, never text; `ok` the connected dot; `danger` unreachable, conflict, error boundary. Layers: `--ms-z-chrome: 10` (rail, header), `--ms-z-scrim: 30`, `--ms-z-sheet: 31`, `--ms-z-tooltip: 40`.

**Contrast**, WCAG relative luminance from the values above, asserted by `contrast.test.ts` in both blocks; text needs 4.5:1, non-text 3:1.

| Pair | Needs | Light | Dark |
| --- | --- | --- | --- |
| `text` on `bg`, `surface`, `surface-2`, `surface-3` | 4.5 | 15.42, 16.67, 14.51, 13.37 | 14.63, 13.69, 12.73, 11.45 |
| `text-2` on the same four | 4.5 | 6.57, 7.10, 6.18, 5.70 | 8.13, 7.61, 7.07, 6.36 |
| `text-3` on the same four | 4.5 | 5.21, 5.63, 4.90, 4.52 | 5.91, 5.53, 5.14, 4.62 |
| `accent` on `bg`, `surface` (links) | 4.5 | 4.99, 5.39 | 6.57, 6.14 |
| `accent` on `surface-2`, `surface-3` (ring, current icon) | 3 | 4.69, 4.33 | 5.71, 5.14 |
| `accent-fg` on `accent`, `accent-hover` | 4.5 | 5.39, 6.67 | 6.90, 8.50 |
| `marker-fg` on `marker-bg` | 4.5 | 6.34 | 8.56 |
| `marker` on `surface-2` (stripe) | 3 | 3.17 | 6.81 |
| `danger` on `surface-2`, `surface` | 4.5 | 4.73, 5.44 | 4.69, 5.04 |
| `ok` on `surface-2` (dot) | 3 | 3.69 | 5.95 |
| `border-strong` on `bg`, `surface`, `surface-2` | 3 | 3.40, 3.67, 3.19 | 3.94, 3.68, 3.42 |
| `bg` on `text` (tooltip) | 4.5 | 15.42 | 14.63 |

`border` rules are decorative, the skeleton `aria-hidden`, disabled controls exempt (1.4.3, 1.4.11); none is asserted. The light `accent` darkened from `#3D6DE0` because link text on `bg` computed to 4.35:1; the focus ring, sound in Sol's calculation, only gains.

Type: Inter variable; JetBrains Mono variable for identifiers, digests, paths, the composer; both OFL-1.1, bundled as g1-s8 bundles Inter. Scale `xs 12/16`, `sm 13/18`, `base 14/20`, `md 16/24`, `lg 20/28`, `xl 24/32`; weights 400, 500, 600; `xl` letter-spacing −0.01em; 14px on `html`. Spacing on 4px: 4, 8, 12, 16, 20, 24, 32, 40, 48. Radii: 4 chips, tooltips, skeleton; 6 buttons, inputs, rail rows; 8 cards; 12 dialogs; sheets square. Surfaces flat. Focus: `:focus-visible`, a 2px solid `accent` outline at 2px offset following the element's radius; no component removes it. Motion: 120ms hover and colour; 180ms rail collapse, dock open, sheet slide; easing `cubic-bezier(.2,0,0,1)`; resize untransitioned; skeleton pulse 1.2s; under `prefers-reduced-motion: reduce` every duration is 0 and the skeleton static.

Icons: `lucide-react`, 18px in the rail, 16px in buttons, stroke 1.75; sections in rail order `MessageSquare`, `LayoutDashboard`, `BookOpen`, `SquareKanban`, `Server`, `CircleHelp`, `Package`, `Settings`; controls `PanelRight`, `PanelLeft`, `Menu`, `Maximize2`, `X`, `Monitor`, `Sun`, `Moon`; Sol verified the names for 1.47.0.

### Components

| Component | Accessibility contract |
| --- | --- |
| `Shell` | Landmarks; skip link "Skip to content" first in the DOM, visible on focus, targeting `main` |
| `Rail`, `RailItem` | `<nav aria-label="Sections">` of `NavLink`s with `aria-current="page"`; the label stays in the DOM when collapsed, visually hidden; the toggle has `aria-expanded`, `aria-controls` |
| `Header`, `WorkspaceIdentity` | `<header>`; badge and conflict text inside the chip; the dock toggle: `aria-label="Brain dock"`, `aria-expanded={open}`, `aria-controls="brain-dock"`, on `/brain` `aria-disabled="true"`, still focusable |
| `ConnectionIndicator`, `Notice` (*second cut*) | `role="status"`, `aria-live="polite"` |
| `Pane`, `EmptyState`, `NotFound` | `<main id="content" tabIndex={-1}>` with an `h1` naming the section; `main` is the skip link's target, never a tab stop |
| `Dock`, `Composer` | `<aside id="brain-dock" aria-label="Brain dock">`; buttons "Expand the Brain view", "Close the Brain dock"; textarea labelled "Message to the Brain", `disabled`, `aria-describedby="composer-reason"`, the reason visible beneath; Send `disabled` |
| `Separator` (library) | Renders `role="separator"`, `tabIndex=0`, `aria-orientation="vertical"`, `aria-controls="work"`, `aria-valuenow`/`min`/`max` as the work panel's percentage (`Separator.tsx:200` to `:231`; `calculateSeparatorAriaValues.ts`); our `aria-label="Resize the Brain dock"` passes through `...rest` |
| `Sheet` (Radix `Dialog`) | `aria-label` "Sections" or "Brain dock"; focus trapped, returned to the opener; Escape and scrim close; scroll-lock style takes the nonce via g1-s8's `setNonce` |
| `Tooltip` (Radix), `IconButton` | On every icon-only button and collapsed rail item, on hover and focus, never the only label; `IconButton` requires `aria-label` |
| `ThemeControl`, `FocusSwitch` | Groups labelled "Theme" and "View" of buttons with `aria-pressed` |
| `Skeleton`, `ErrorBoundary` | `aria-hidden`; a class component around `Pane` and `Dock` |

**Keyboard.** Sequential order: skip link; rail toggle (from 1,128), Brain, the six sections, Settings, System, Light, Dark; menu (phone); Retry (when shown); dock toggle; *second cut* Reload; the pane's focusables in DOM order (on Overview the link to About this workspace; on Not found, Go to Overview; on Settings the theme control and notices link; nothing in the other five section panes); the separator (wide); Expand; Close. `main`, the disabled textarea, and the disabled Send are not in the order. A sheet traps focus while open. Every reason is reachable: the `aria-disabled` toggle's tooltip opens on focus; the composer's reason and the conflict text are visible.

**Separator keys**, verified in `lib/global/event-handlers/onDocumentKeyDown.ts` at 4.13.1: `ArrowLeft` shrinks the work panel by five percentage points of the group (the dock grows), `ArrowRight` the reverse; `Home` gives the work panel its smallest allowed size (dock at its effective maximum), `End` its largest (dock at 320); `ArrowUp`/`ArrowDown` only `preventDefault`; `Enter` does nothing, no panel is collapsible; `F6` focuses the next separator, with one, itself. Each is a user interaction to `onLayoutChanged`. The library adopts one constructed `CSSStyleSheet` on first interaction (`updateCursorStyle.ts:21` to `:35`) and, with `disableCursor`, never inserts a rule (`getCursorStyle.ts`); it creates no `<style>` element, so the policy sees no inline style; O4's browser proof stays the runtime check. `cursor: col-resize` comes from our stylesheet.

**Visual states.** Focus is the ring above, everywhere.

- `IconButton`: 32×32, radius 6, 16px icon, no border; icon `text-2`; hover `surface-3`, icon `text`; pressed `border`; disabled icon `text-3`, no hover, `cursor: default`; `aria-disabled` looks the same, stays focusable.
- `Button`: 32 high, padding 0 12px, radius 6, 13px/500, 16px icon at 6px gap; `surface`, 1px `border-strong`, `text`; hover `surface-3`; pressed `border`; disabled `surface-2`, `text-3`, 1px `border`. Primary (Send): no border, `accent` with `accent-fg`; hover and pressed `accent-hover`; disabled `surface-3`, `text-3`.
- `RailItem`: 32 high, full width (40 collapsed), radius 6, padding 0 7px, 18px icon, 12px gap, label 14px/500, `text-2`; hover `surface-3`, `text`; pressed `border`; current `surface-3`, `text`, icon `accent`.
- `Chip`: 20 high, padding 0 6px, radius 4, 12px/500; badge `marker-fg` on `marker-bg`; dock chip `text-2` on `surface-3`.
- `Tooltip`: `bg` 12px/500 on `text`, padding 4px 8px, radius 4, max-width 240, `sideOffset` 6, `delayDuration` 300, no arrow, `shadow`, `z-tooltip`.
- `ThemeControl`, `FocusSwitch`: `IconButton`s, gap 4; on Settings and in the switch, `Button`s with labels; the pressed one `surface-3`, icon or text `accent`.
- `Skeleton`: 120×16, radius 4, `surface-3`, opacity 1 → .5 → 1.
- `Sheet`: `surface`, full height, width per Layout, radius 0, 1px `border` on the inner edge, `shadow`, `z-sheet` over `scrim`; 48px header, title 14px/600, Close; body scrolls; slides in 180ms.
- `Composer`: textarea full width, min-height 72, padding 8px 12px, radius 6, mono 13/18, `surface`, 1px `border-strong`, placeholder "Message to the Brain" in `text-3`; disabled `surface-2`, 1px `border`, `text-3`, `cursor: default`; below, 8px gap: the reason 12px `text-3` left, Send right.
- `EmptyState`: centred, 96px from the pane's top; icon 24 `text-3`; 12; heading 16/600 `text`; 8; body 14/20 `text-2`, max 480; 12; slice line 12/16 `text-3`; 16; then either the link, a `NavLink` in 13px/500 `accent` text, underlined on hover and focus (the `accent` on `bg` contrast row covers it), or the action `Button`; never both, and for most rows neither.
- `Notice` (*second cut*): 36 high, `marker-bg`, 1px `border` below, 16px side padding; 13px/500 `marker-fg` left; Reload `Button` 28 high right.
- `ConnectionIndicator` (*second cut*): 8px dot `ok` or `danger`, 6px gap, 12px `text-2` label; Retry `Button` after it.
- `ErrorBoundary`: card `surface`, radius 8, 1px `border`, padding 24, max-width 640; heading 16/600 `danger` "This pane could not be rendered"; the error mono 13/18 `text-2` in `surface-2`, radius 6, padding 12, `overflow-x: auto`; "Reload page" `Button`.
- `Separator`: 8px hit area, transparent, `cursor: col-resize`, 1px `border-strong` line centred; hover, focus, and drag (`data-separator`) a 2px `accent` line.

### Theme

Preference `system`, `light`, or `dark` under `ms.ui.theme`; absent means `system`. The effective theme is `data-theme="light|dark"` on `<html>`; CSS has one light block and one dark block and no media query: `src/theme.ts` resolves `system` with `matchMedia("(prefers-color-scheme: dark)")` and listens for `change`. `color-scheme` is set with the tokens.

No flash without inline script: `_app/public/theme.js`, a classic script of about twelve lines referenced as `<script src="/theme.js"></script>` in `<head>` before Vite's stylesheet link, reads the key inside `try`, resolves `system` the same way, and sets the attribute before first paint. Vite copies `public/` unchanged to the output root (Sol verified against Vite's `publicDir` documentation), so it is served as `/theme.js` under the extension table, allowed by `script-src 'self'`. `theme.ts` exports the key and attribute name; `themeScript.test.ts` asserts `theme.js` contains both.

### State

The section lives in the URL (default `/overview`), remembered by history. Four `localStorage` keys: `ms.ui.theme` (`system`); `ms.ui.rail` (`expanded`; ignored below 1,128); `ms.ui.dock` (`open`; wide only, sheets open closed); `ms.ui.dock.width` (pixels, `400`). Every access is in `try`; an invalid value yields the default. Each workspace server is its own origin, so keys carry no prefix. No server state, cookie, or authority: view state only (plan `:65`). The workspace response lives in React state.

**Dock width.** The library's `Layout` is a map of panel id to percentage of the group (`lib/components/group/types.ts:13` to `:17`, `:101` to `:113`); `defaultLayout` and `onLayoutChanged` carry that map, and the page never stores or passes one. Instead: `<Group id="shell" orientation="horizontal" disableCursor onLayoutChanged={save}>` holding `<Panel id="work" minSize={480} groupResizeBehavior="preserve-relative-size">`, `<Separator>`, `<Panel id="dock" defaultSize={width} minSize={320} maxSize={640} groupResizeBehavior="preserve-pixel-size" panelRef={dockRef}>`; numeric props are pixels (`lib/components/panel/types.ts:139`, `:146`) and the panel without a default receives the remainder (`calculateDefaultLayout.ts`). Read: the stored value clamped to 320 to 640, then to the room `innerWidth − rail − 8 − 480`. Save: in `save(layout, meta)`, only when `meta.isUserInteraction` (true for a pointer release and the resize keys; false for mount, window-resize recompute, `setLayout`; `group/types.ts:25` to `:35`), write `Math.round(dockRef.current.getSize().inPixels)` (`panel/types.ts:67` to `:70`). The ids `work` and `dock` are literals, so no generated id is persisted.

**Live resize** (J1). `groupResizeBehavior` takes `preserve-relative-size` or `preserve-pixel-size`, defaults to the first, and a group must keep at least one relative panel (`lib/components/panel/types.ts:158` to `:172`; the default at `Panel.tsx:53`, registered into the panel's constraints at `:106` to `:107`, carried into the derived constraints at `lib/global/dom/calculatePanelConstraints.ts:15` and `:85`; `work` is written relative so the pair reads as intended, the value being the default). Without the prop both panels are relative, and a mounted window narrowing from 1,368 with the rail expanded, panels near 720 and 400, reaches 1,128 near 560 and 320: the dock shrinking while 400 still fits, which the pre-mount clamp cannot prevent because it never runs on a mounted resize. With it: the group's `ResizeObserver` (`lib/global/mountGroup.ts:46` to `:109`) recomputes the pixel constraints as percentages of the new group (`:65`), re-expresses the dock's previous pixel width as a percentage of the new group and gives the work area the whole remainder (`lib/global/utils/preserveFixedPanelSizes.ts:30` to `:52`, `:58` to `:71`), then validates (`mountGroup.ts:80` to `:83`): a first pass clamps each panel to its constraints in panel order and collects the difference (`validatePanelGroupLayout.ts:45` to `:64`; `validatePanelSize.ts:30` to `:53`), a second hands that difference to the first panel, in order, that can take it (`:68` to `:90`). Panel order is `work` then `dock`, so as the window narrows **the work area gives way first**, down to 480, **then the dock**, down to 320; in wide the group is at least 880 (1,128 with the rail expanded; 896 at 960 collapsed), more than 480 + 320, so a resize never reaches the dock's floor. The resulting update carries `isUserInteraction: false` (`mountGroup.ts:97` to `:103` passes no meta; `lib/global/mutable-state/groups.ts:93` to `:109`), so nothing is written. Two consequences, stated so the walkthrough can read them: at the 1,128 crossing the group is 880 on the expanded side and 1,063 on the collapsed side, and the dock holds its width while the work area takes the difference; a dock squeezed by the work area's minimum keeps its squeezed width when the window widens again, until the human resizes it or the page loads and the untouched stored value applies through the pre-mount clamp.

## Behaviour

Main flow: `theme.js` sets the theme; `main.tsx` sets the nonce (g1-s8), reads the stored values, mounts `BrowserRouter` and `Shell`; the chip fetches `/api/workspace` behind a skeleton while the pane renders from the URL at once.

| Case | Required result |
| --- | --- |
| `/api/workspace` is 404 (server predates the route), 500, or the fetch fails | "Workspace unknown" with Retry and the message; sections render; title falls back |
| `/backlog/anything` before `g1-s10` | Not-found pane in the shell; 200 from the server |
| Stored width `12`, `9999`, `"abc"` | 320, 640, 400 |
| Viewport 960, rail collapsed, stored `640` | Dock 416; stored value untouched |
| `localStorage` throws | Defaults; no error surfaces; no write that load |
| Narrowed below 960 with the dock open | Sheet mode; stored value untouched; widening restores it |
| 960 to 1,127 with `ms.ui.rail` `expanded` | Rail collapsed, toggle hidden; from 1,128 expanded |
| Live, no reload: 1,368 with the rail expanded and the dock at 400, narrowed to 1,128 | Dock 400, work area 480; nothing written |
| Live: the dock set to 640 by pointer at 1,368 expanded, then narrowed to 1,128 | The work area gives way to 480, then the dock to 400; nothing written; widened back to 1,368 the dock stays 400 and the work area takes 720; a reload gives 640 |
| Live: 1,128 to 1,127 and back | Rail 240, 56, 240; dock unchanged; work area 480, 663, 480 |
| `/brain` at 599 | One column, Conversation shown, the switch offers Subject |
| A pane throws | Its boundary; rail, header, dock keep working |
| *Second cut*: digest changes between health fetches | The notice; no automatic reload |
| *Second cut*: the stream names the workspace view | `loadWorkspace()` once; chip, title, and About update in place; no reload |
| *Second cut*: the stream names another view, or an event the page cannot parse | Ignored; nothing fetched |
| *Second cut*: the stream drops and reopens | On `open` after `error`: one health fetch and one `loadWorkspace()`; the browser's reconnection is the only retry; no timer |

## Change boundary

**First cut**, frontend, new under `internal/ui/web/_app/`: `public/theme.js`; `src/routes.ts`, `theme.ts`, `storage.ts`, `title.ts`, `tokens.css` with the Tailwind mapping, `src/shell/**` (among them `workspace.ts`, the one network call site, exporting `loadWorkspace()`), `src/panes/**`, their `*.test.ts`. Edited there: `index.html` (one script tag), `package.json`, `package-lock.json`, `App.tsx` and `main.tsx` (the shell replaces the g1-s8 demo; the notices link moves to Settings), `fonts.css` (the mono face). Regenerated as the last commit after rebasing: `bundle/**` (D7).

**First cut**, Go, unconditional: new `internal/ui/workspace/` with `ReadAdoption(installation string) Adoption` and `Describe(roots lifecycle.Roots, rec lifecycle.Record, configuredSubject string) (Workspace, error)`, which resolves the layout, reads the adoption, applies the subject default, and computes `conflict`, with tests; `internal/ui/httpd/` gains `/api/workspace`, exact path, JSON, 404 beneath, an error as 500 `{"error":"…"}`, through `Info.Describe func() (workspace.Workspace, error)` called per request so a later fill of the adoption line needs no restart; `internal/config/ui.go` gains `UISubjectKey` and `UISubject(confPath string) (string, error)`, empty default, the environment seam; `cmd/metasystem/ui.go` wires it; `docs/architecture.md` one row; `metasystem.conf` `# ui.subject=`.

**Second cut**, after `g1-s7`: `src/shell/ConnectionIndicator.tsx`, `Notice.tsx`, `health.ts`, `events.ts` (the subscription, the only `EventSource`), their tests; `workspace.ts` unchanged, `loadWorkspace()` being the refetch; `cuts.test.ts` moved to its second-cut allowlist; no Go change. None of the four files exists in the first cut.

Must not be touched: `testing-parallel-ratchet.json`, `testing.json`, `go.mod`, `go.sum`, `internal/testenv`, `testutil`, `parallelratchet`, `testselect`, `testpolicy`, `testexec`, `hostload`, `proofrun`, `gopackages`, `stateroot`, `ui/lifecycle`, `cmd/metasystem/test*.go`, `audit.go`, `main.go`, `scripts/**`, g1-s8's `vite.config.ts`, `tsconfig.json`, `scripts/*.mjs`, its policy and page rule, and any existing `*_test.go` or `testmain_test.go` outside `internal/ui/httpd`, whose route cases may gain the new route only.

Conventions, repeated because the builder sees only this document: every new Go package has `testmain_test.go` calling `testenv.Main`; tests never call `os.Setenv`; every top-level test and independent subtest calls `t.Parallel()`, the ratchet allowing an unlisted package zero serial tests; no sleeps, polling, or fixed ports; `t.TempDir()`; `testutil.Expect` and `Require`; `cmd/metasystem/ui.go` wires and prints only. Frontend: exact versions, `npm ci`, the Node pin (D27), Vitest in `node` with no DOM library, no colour literal outside `tokens.css`, no `style` attribute in markup (React `style` props are CSSOM writes, allowed), the bundle rebuilt last.

## Verification

**First cut**, Vitest, `node`: `routes.test.ts` (sections both ways, `/`, trailing slash, nested prefix, unknown kind, no reserved prefix, `isReserved`); `title.test.ts` (six cases); `theme.test.ts` (effective theme; garbage to `system`); `storage.test.ts` (clamp rows, the room clamp, throwing storage); `themeScript.test.ts`; `tokens.test.ts` (every `--ms-` name in both blocks); `literals.test.ts` (no `#hex` or `rgb(` under `src/` outside `tokens.css`); `contrast.test.ts` (parses `tokens.css`, computes WCAG luminance, asserts every contrast row in both blocks); `empties.test.ts` (kind, heading, body, slice line, link, and action of every row verbatim; every section row is `not projected` with action `none`; the only link is Overview's, to `/settings`; none contains "nothing", "no goals", "no seats", "no decisions"); `cuts.test.ts` (the cut guard, over every file under `src/`: the only match for `fetch(`, `new EventSource`, `new WebSocket`, `XMLHttpRequest`, or `sendBeacon` is the one `fetch("/api/workspace"` in `src/shell/workspace.ts`; no `setInterval`; no `addEventListener` for `focus`, `online`, `offline`, `visibilitychange`, or `pageshow`; no `/-/health` string; `ConnectionIndicator.tsx`, `Notice.tsx`, `health.ts`, and `events.ts` do not exist). The guard names call sites and files rather than component names so that a subscription or a refetch written under any other name fails it; the browser step below is its runtime half.

**Second cut**, Vitest, `node`: `health.test.ts` (a fetch on `focus` and on `online`, never on a timer; a changed digest raises the notice); `events.test.ts` (with a fake `EventSource` class: an event naming the workspace view calls `loadWorkspace` once; another view, or an unparsable event, nothing; `error` then `open`, one health fetch and one `loadWorkspace`; no `setInterval`); `cuts.test.ts` revised to the second-cut allowlist, exactly three network call sites, `fetch("/api/workspace"` in `workspace.ts`, `fetch("/-/health"` in `health.ts`, `new EventSource(` in `events.ts`, and still no `WebSocket`, `XMLHttpRequest`, `sendBeacon`, or `setInterval`.

**First cut**, Go: g1-s8's `web` and `httpd` tests pass on the rebuilt bundle; `ReadAdoption` over `absent`, `unreadable`, `placeholder`, `recorded`, and a malformed line, under `t.TempDir()`; a table test over the three layouts for `mode`, default subject, and `conflict` in every `mode × record` cell; the route's payload, the 404 beneath, the 500 shape, headers and policy; `ui.subject` through the seam.

Walkthrough, in order, in a real browser (Playwright allowed, D24), by Claude then the human: build, `ui start`, open the address; every section against the empty-state table, URL and title watched; reload on `/decisions`, back three times; `/nothing/here`, then `/api/nope`; at 960, 1,127, 1,128, 1,184, and 1,368 measure rail, work area, and dock against the arithmetic; then live, without reloading, rail expanded, dock at 400: from 1,368 drag the window to 1,128 and read dock 400, work area 480; to 1,127, rail 56, dock 400, work area 663; to 960, dock 400, work area 496; back to 1,368, dock 400, work area 720; at 1,368 set the dock to 640 by pointer, narrow to 1,128 and read work area 480, dock 400, widen to 1,368 and read dock 400, work area 720, reload and read dock 640; separator to both limits by pointer and by `Home`/`End`, `ArrowLeft` twice, reload, dock closed, reload, reopened; Brain entry, Expand, the switch at 599, back; System, Dark, reload without flash, Light, System, OS theme flipped; 800 then 500 with both sheets; keyboard only through the sequential order, Escape on a sheet; VoiceOver over the landmarks, one rail item, the `aria-disabled` toggle on `/brain`; screenshots at 1,280 and 599 in both themes against the visual states; zero policy violations, no request off the origin; with the network panel open, one `/api/workspace` request on load and one per Retry, then a refocus, an offline and online toggle, and two idle minutes with no request at all; `check:modal` passes. *Second cut*: rebuild, `ui restart`, refocus, the notice, reload; `ui stop`, refocus, `server unreachable`, `ui start`, Retry; one open stream request in the network panel; the invalidation `g1-s7` documents for the workspace view, and Settings' About updating without a reload; `ui stop` then `ui start`, the stream reopening followed by one health and one workspace request.

Obligations the code critique checks by name:

- **O1, stable links.** Every destination has exactly its path, by rail and URL, restored on reload; no link targets a reserved prefix; `routes.test.ts` passes.
- **O2, honest empties.** Kind, heading, body, slice line, link, and action verbatim; every section is the not-projected kind with no action of its own (master `:437`); the only link is Overview's, a `NavLink`, not a `Button`; none asserts absence; no empty list, table, or endless skeleton; `empties.test.ts` passes.
- **O3, no inline, nothing external.** `TestIndexIsStrict` and `literals.test.ts` pass; no `style` attribute in markup.
- **O4, no violation on interaction.** `disableCursor` set; drag, sheet, and tooltip raise zero `securitypolicyviolation` events in the embedded build; the adopted constructed sheet holds no rule.
- **O5, theme without flash.** `theme.js` precedes the stylesheet in `<head>`; `themeScript.test.ts` and `tokens.test.ts` pass.
- **O6, keyboard and labels.** The sequential order holds, `main` and disabled controls absent; the dock toggle carries `aria-expanded` and `aria-controls` and on `/brain` is `aria-disabled` yet focusable with its reason on focus; every icon-only control has `aria-label`; the separator answers `ArrowLeft`, `ArrowRight`, `Home`, `End` as stated and updates `aria-valuenow`; sheets trap and return focus.
- **O7, bounded state.** `storage.test.ts` passes; every access wrapped; only the four keys written; the width written only under `isUserInteraction`, in pixels from `getSize().inPixels`, and never by a window resize; panel ids `work` and `dock`.
- **O8, identity cases.** The five chip cases render from faked responses, the conflict text visible.
- **O9, bundle current and attributed.** The last commit rebuilds the bundle; `go test ./internal/ui/web/` passes; the notices file names the four packages.
- **O10, resource.** Unconditional; `mode` from `ResolveLayout(...).Template`; `ReadAdoption` and the layout tests pass; `conflict` computed in Go; no check loosened, no header dropped.
- **O11, contrast.** `contrast.test.ts` passes; no colour outside the token table reaches the screen.
- **O12, geometry.** The five measured widths match the arithmetic; the rail collapses between 960 and 1,127; the focused switch below 960, the subject panel from 960; live, `dock` carries `groupResizeBehavior="preserve-pixel-size"` and `work` stays relative, the wide-to-wide steps of the walkthrough read as written, and the work area gives way before the dock.
- **O13, visual states.** Every bullet above matches in the browser in both themes; a deviation is a finding.
- **O14, cuts.** `cuts.test.ts` passes as written for the cut shipped. First cut: the only network call site under `src/` is the workspace fetch; no stream, socket, timer, or window-event refetch exists under any name; the four second-cut files are absent; in the browser a session issues one `/api/workspace` request on load and one per Retry, and none on refocus, on an offline and online toggle, or over two idle minutes. Second cut: the allowlist is exactly the three call sites, and `health.test.ts` and `events.test.ts` pass.

Commands, from `metasystem/`: `go build ./...`, `go vet ./internal/ui/...`, `go test ./internal/ui/...`, `gofmt -l internal cmd`, `bin/metasystem audit parallel-ratchet` without `--update`. From `_app/`: `npm ci`, `npm run typecheck`, `npm test`, `npm run bundle` twice with a clean `git status --porcelain` between.

## Open questions

None; D28 settled revision 1's five.
