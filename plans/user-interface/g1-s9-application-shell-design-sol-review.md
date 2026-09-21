# g1-s9 Application shell: Sol's design review

Reviewer: Codex on `gpt-5.6-sol`, the design reviewer under D12 and D24, read-only, 2026-09-21. Subject: the [slice design](g1-s9-application-shell-design.md) at commit `675df4beb`. The report follows verbatim.

Nine material findings, all accepted. Three would have reached the human's screen: colour tokens that fail their contrast thresholds by calculation, layout arithmetic that cannot close at its own breakpoint, and empty states that assert nothing exists where the truthful statement is that this build does not project it yet.

## 1. Verdict build after the listed changes Riskiest: the shell can label a marker-collision...
[codex] Turn completion inferred after the main thread finished and subagent work drained.
## 1. Verdict

build after the listed changes

Riskiest: the shell can label a marker-collision checkout “self-hosted” because its proposed resource cannot yet read the adoption provenance needed to detect the conflict.

## 2. Findings

- **F1; MATERIAL; Header and workspace identity.** Problem: the visible identity-conflict case is unreachable in production: `adoptedFrom` remains empty until the later full `g1-s7`, while the conflict UI requires it to be non-empty. Evidence: [g1-s9:28](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:28), [g1-s9:78](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:78), [g1-s9:84](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:84), [master:453](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface-design.md:453), [adopt.sh:154](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/scripts/adopt.sh:154). Wrong/guess: an adopted checkout that also matches the template marker is presented as MetaSystem self-hosted instead of visibly refusing—the exact wrong-workspace hazard D20 addresses. Fix direction: include the minimal adopted-revision reader in the thin resource or fail identity closed until provenance can be compared.

- **F2; MATERIAL; Layout.** Problem: the responsive geometry cannot satisfy its own minimums: at the 900px wide breakpoint, expanded rail + minimum work area + separator + minimum dock requires 1,048px, while the default 400px dock requires 1,128px; focused `/brain` also keeps a fixed 400px subject pane without the narrow-screen conversation/subject switch required by the master. Evidence: [g1-s9:37](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:37), [g1-s9:43](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:43), [g1-s9:45](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:45), [g1-s9:47](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:47), [g1-s9:55](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:55), [master:88](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface-design.md:88). Wrong/guess: builders must invent overflow, forced rail collapse, dock shrinking, a different breakpoint, or a focused-mode mobile layout. Fix direction: reconcile breakpoints with the size equation and specify an explicit compact/phone switch between focused conversation and subject.

- **F3; MATERIAL; Change boundary and open questions.** Problem: D28’s decisions remain conditional or open: the resource is “only if” folded in, and the design scopes connection/build indicators into this build although the implementation plan assigns them to the second cut. Evidence: [g1-s9:16](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:16), [g1-s9:184](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:184), [g1-s9:215](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:215), [g1-s9:217](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:217), [implementation plan:47](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface-implementation-plan.md:47), [implementation plan:184](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface-implementation-plan.md:184). Wrong/guess: one builder can omit `/api/workspace`, while another can build second-cut behavior prematurely. Fix direction: make the thin resource unconditional and divide the change boundary and verification explicitly into the already-settled first and second cuts.

- **F4; MATERIAL; State and resizable panels.** Problem: the dock-width persistence algorithm confuses pixels with the library’s group layout units: `defaultLayout` and `onLayoutChanged` use panel-ID-to-percentage/flex-grow maps, while the design stores one pixel width and clamps it to 320–640. Evidence: [g1-s9:31](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:31), [g1-s9:164](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:164), upstream [Group types](https://github.com/bvaughn/react-resizable-panels/blob/main/lib/components/group/types.ts) and [Panel types](https://github.com/bvaughn/react-resizable-panels/blob/main/lib/components/panel/types.ts). Wrong/guess: a builder can persist a percentage as pixels or feed a pixel value into `defaultLayout`, producing a wrongly sized or rejected layout. Fix direction: specify stable panel IDs and an exact pixel conversion using `PanelRef.getSize().inPixels`, or persist percentages consistently and revise the storage contract.

- **F5; MATERIAL; Components and focus order.** Problem: the stated keyboard contract is internally impossible and omits the dock disclosure state: `main` has `tabIndex=-1` and the composer is disabled, yet both appear in sequential focus order; the dock toggle lacks required `aria-expanded`/`aria-controls`; and `/brain` puts the unavailable reason only in a tooltip on a disabled button that cannot receive focus. Evidence: [g1-s9:51](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:51), [g1-s9:55](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:55), [g1-s9:146](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:146), [g1-s9:147](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:147), [g1-s9:154](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:154), [MDN `tabindex`](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Global_attributes/tabindex), [MDN `disabled`](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Attributes/disabled), [WAI disclosure pattern](https://www.w3.org/WAI/ARIA/apg/patterns/disclosure/). Wrong/guess: O6 cannot pass as written, and keyboard users cannot discover the focused-mode reason or dock state. Fix direction: define the actual sequential order separately from skip-link focus, add dock disclosure attributes, and use a focusable `aria-disabled` control or adjacent visible reason where discoverability is required.

- **F6; MATERIAL; Visual language—colour.** Problem: `text-3` fails normal-text contrast everywhere it is used, and `border-strong` fails the 3:1 non-text threshold when it is the sole separator/input boundary; calculated from the specified values, `text-3` ranges from 2.91:1–3.63:1 in light and 3.26:1–4.17:1 in dark, while `border-strong` is 1.63:1–1.84:1 against its likely adjacent surfaces. The accent focus outline is sound, with a minimum calculated contrast of 3.78:1 light and 5.14:1 dark. Evidence: [g1-s9:91](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:91), [g1-s9:118](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:118), [g1-s9:121](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:121), [g1-s9:131](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:131), [g1-s9:135](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:135), [WCAG text contrast](https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum), [WCAG non-text contrast](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast). Wrong/guess: required secondary copy is inaccessible and the resting resize control may be visually unidentifiable. Fix direction: strengthen those tokens or remap small text and interactive boundaries, then add contrast assertions for every specified foreground/background pair.

- **F7; MATERIAL; Visual language and components.** Problem: the component inventory specifies accessibility but leaves conspicuous visuals undefined for icon-button states, tooltip geometry, theme selection, skeleton height/radius, sheet edge/shadow/layering, notice content layout, and the disabled composer; tokens exist, but their application to these components does not. Evidence: [g1-s9:109](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:109), [g1-s9:133](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:133), [g1-s9:141](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:141). Wrong/guess: two conforming builders would produce visibly different first screens and interaction states. Fix direction: add a component visual-state table or annotated reference covering exact token, spacing, typography, radius, elevation, overflow, and normal/hover/pressed/disabled states.

- **F8; MATERIAL; Empty states.** Problem: “Nothing to show yet” and “No behaviour recorded yet” assert absence without reading data, and the default rule supplies no first action even though the master requires one; the repository is already known to contain substantial records. Evidence: [g1-s9:91](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:91), [g1-s9:95](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:95), [g1-s9:100](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:100), [master:437](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface-design.md:437), [implementation plan:149](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface-implementation-plan.md:149). Wrong/guess: the shell tells the human nothing exists when the truthful statement is that this build does not yet project it, and builders must invent whether an action appears. Fix direction: phrase empties as unavailable/not-yet-connected views and define the appropriate currently usable action for each pane.

- **F9; MATERIAL; Navigation.** Problem: all eight declared routes are compatible with the SPA fallback, but “anything unmatched” receiving the shell with 200 is false for reserved unknown paths such as `/api/nope`, `/-/nope`, and `/assets/nope`, which correctly remain server 404s. Evidence: [g1-s9:61](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:61), [g1-s9:63](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:63), `g1-s8` section “Serving”. Wrong/guess: a builder or test author can assert shell rendering and reload stability for a path the server deliberately refuses. Fix direction: scope the React not-found contract to non-reserved navigation paths that satisfy `AcceptsHTML`.

- **F10; NOT MATERIAL; Theme and CSP.** Problem: none found—the classic same-origin `/theme.js` is copied to the output root by Vite, served as a static `.js` file, and allowed by `script-src 'self'`; React style-property writes are allowed, Radix’s style singleton receives the nonce, and fonts/icons remain local. Evidence: [g1-s9:133](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:133), [g1-s9:160](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:160), `g1-s8` sections “The build”, “Serving”, and “Content security policy”, [Vite `publicDir`](https://vite.dev/config/shared-options). Wrong/guess: none from the written serving chain. Fix direction: none.

- **F11; NOT MATERIAL; Components—separator keyboard behavior.** Problem: the fallback prose is underspecified, but pinned `react-resizable-panels` 4.13.1 already supplies deterministic behavior: Left/Right resize a horizontal group in five-unit steps, Home/End reach limits, F6 cycles separators, and the component updates its ARIA values. Evidence: [g1-s9:148](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:148), upstream [keyboard handler](https://github.com/bvaughn/react-resizable-panels/blob/main/lib/global/event-handlers/onDocumentKeyDown.ts) and [Separator](https://github.com/bvaughn/react-resizable-panels/blob/main/lib/components/separator/Separator.tsx). Wrong/guess: none with the pinned package, but the fallback would become a genuine guess if activated. Fix direction: replace “verify and fall back” with the verified exact behavior and tests.

## 3. The design's own unverified claims

- **Panels cursor injection:** source-verified for 4.13.1: `disableCursor` prevents a cursor rule from being inserted, but the package still constructs and adopts an empty `CSSStyleSheet`; it creates no `<style>` element. Actual CSP behavior still needs the specified browser proof. [Updater](https://github.com/bvaughn/react-resizable-panels/blob/main/lib/global/cursor/updateCursorStyle.ts), [disable check](https://github.com/bvaughn/react-resizable-panels/blob/main/lib/global/cursor/getCursorStyle.ts).
- **Panels keyboard behavior:** verified in current 4.13.1 source; the fallback is unnecessary for the pinned version. [Package version](https://github.com/bvaughn/react-resizable-panels/blob/main/package.json), [keyboard handler](https://github.com/bvaughn/react-resizable-panels/blob/main/lib/global/event-handlers/onDocumentKeyDown.ts).
- **Vite copying `public/theme.js`:** verified; `publicDir` defaults to `public`, is served at `/`, and is copied unchanged to the output root by default. [Vite shared options](https://vite.dev/config/shared-options), [Vite build options](https://vite.dev/config/build-options).
- **Lucide icon names:** verified against the current official catalog/source corresponding to the current 1.47.0 package; `CircleHelp` is an exported alias for the renamed `CircleQuestionMark`, so substitution is not presently needed. [Lucide React exports](https://raw.githubusercontent.com/lucide-icons/lucide/main/packages/lucide-react/src/lucide-react.ts), [CircleHelp catalog redirect](https://lucide.dev/icons/circle-help).
- **No adopted-from Go reader:** verified for that field; the adoption script records the SHA at [scripts/adopt.sh:154](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/scripts/adopt.sh:154), while the existing Go reader of `docs/project-rules.md` parses unrelated preauthorization categories at [internal/contract/contract.go:624](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/contract/contract.go:624).

## 4. The human's first reaction

The most likely requested change is that the default-open 400px Brain dock devotes a large part of the first screen to an unavailable feature beside another empty pane. Wait for the human’s reaction: default-open is an explicit master decision at [plans/user-interface-design.md:88](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface-design.md:88), and changing that subjective balance before the first visual review would pre-empt the feedback this slice exists to collect.

## 5. Not checked

- No browser, build, package installation, or running-server interaction was performed, so zero CSP violations, no theme flash, actual focus trapping/return, rendered contrast, and layout appearance remain runtime verification obligations.
- No installed dependency tree exists yet; package conclusions rest on the fetched upstream 4.13.1/current sources rather than a local lockfile artifact.

---

# Round 2, 2026-09-21

On revision 2 at commit `b19bf0cc4`. Eight of eleven findings closed, three material remain. Every row of the contrast table reproduced independently and passes, and the layout arithmetic was recomputed and holds at every stated breakpoint.

## 1. Verdict build after the listed changes another round needed because J1–J3 are material ...
[codex] Turn completion inferred after the main thread finished and subagent work drained.
## 1. Verdict

build after the listed changes  
another round needed because J1–J3 are material

## 2. Status of F1 to F9 and F11

F1: corrected — the reader targets the exact line written by `adopt.sh`; the placeholder is distinct from a lowercase 40-character SHA; `docs/` is included and copied in every successful adoption; and `self-hosted && recorded` catches D20’s copied-marker hazard. The future ownership rule requires one reader to be retired or adopted by the layout owner, which is sufficient. [template:7](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/docs/project-rules.md:7), [adopt.sh:161](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/scripts/adopt.sh:161), [adopt.sh:219](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/scripts/adopt.sh:219), [adopt.sh:246](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/scripts/adopt.sh:246), [adopt.sh:364](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/scripts/adopt.sh:364), [adopt.sh:443](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/scripts/adopt.sh:443), [design:75](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:75), [implementation plan:195](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface-implementation-plan.md:195).

F2: corrected — all static breakpoint and focused-layout minima work; see section 4. [design:34](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:34).

F3: NOT corrected — the scope contradicts itself about whether the event stream is out of scope, and the cut test does not reject the second-cut event subscription or refetch-on-invalidation code; see J3.

F4: NOT corrected — the stated props, callbacks and pixel units exist, but the contract omits the dock’s `groupResizeBehavior`, leaving live window-resize behavior different from the intended pixel-width persistence; see J1. [4.13.1 group types](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/components/group/types.ts), [4.13.1 panel types](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/components/panel/types.ts), [4.13.1 default-layout calculation](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/global/utils/calculateDefaultLayout.ts), [4.13.1 layout validation](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/global/utils/validatePanelGroupLayout.ts).

F5: corrected — the sequential order is internally consistent; disabled native controls are excluded; the skip target is programmatically focusable; and every reason is keyboard-reachable. Radix’s trigger opens on focus and conditionally supplies the tooltip content through `aria-describedby`; an `aria-disabled`, non-`disabled` button remains focusable. [design:156](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:156), [design:168](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:168), [Radix Tooltip source for 1.2.16](https://github.com/radix-ui/primitives/blob/9aebdd4/packages/react/tooltip/src/tooltip.tsx).

F6: corrected — every row reproduces and passes; the three changed tokens are used consistently and no other specified foreground/background pair is omitted. [design:105](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:105), [design:127](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:127), [design:172](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:172).

F7: corrected — the component geometry, tokens, states, overflow and stacking are now sufficient for two builders to produce materially the same first screen; nothing conspicuous remains undefined. [design:172](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:172).

F8: NOT corrected — the wording now distinguishes missing projections from missing records, but nearly every empty section still has no first action despite the master requiring one; see J2.

F9: corrected — the router contract matches g1-s8’s “Serving” rule: exact reserved prefixes and descendants remain server-owned, while only other HTML-accepting paths receive the page. `isReserved` and the route/link exclusions are tested. [g1-s8, “Serving”:135](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s8-frontend-toolchain-design.md:135), [design:58](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:58), [design:232](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:232).

F11: corrected — the separator keys match 4.13.1, and the constructed stylesheet is created on interaction but receives no cursor rule with `disableCursor`; no `<style>` element is introduced. [design:170](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:170), [4.13.1 key handling](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/global/event-handlers/onDocumentKeyDown.ts), [4.13.1 separator](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/components/separator/Separator.tsx), [4.13.1 stylesheet update](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/global/cursor/updateCursorStyle.ts), [4.13.1 cursor rule](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/global/cursor/getCursorStyle.ts).

## 3. The contrast table

Computed independently with WCAG relative luminance from the hexadecimal values in the token table. [design:107](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:107)

| Pair, in documented order | Required | Light | Dark | Result |
|---|---:|---|---|---|
| `text` on `bg`, `surface`, `surface-2`, `surface-3` | 4.5 | 15.4242, 16.6694, 14.5062, 13.3744 | 14.6350, 13.6893, 12.7294, 11.4469 | Reproduces; pass |
| `text-2` on the same four | 4.5 | 6.5723, 7.1029, 6.1811, 5.6989 | 8.1336, 7.6081, 7.0746, 6.3618 | Reproduces; pass |
| `text-3` on the same four | 4.5 | 5.2138, 5.6347, 4.9035, 4.5209 | 5.9071, 5.5255, 5.1380, 4.6203 | Reproduces; pass |
| `accent` on `bg`, `surface` | 4.5 | 4.9887, 5.3915 | 6.5684, 6.1440 | Reproduces; pass |
| `accent` on `surface-2`, `surface-3` | 3.0 | 4.6918, 4.3258 | 5.7132, 5.1376 | Reproduces; pass |
| `accent-fg` on `accent`, `accent-hover` | 4.5 | 5.3915, 6.6663 | 6.9029, 8.5029 | Reproduces; pass |
| `marker-fg` on `marker-bg` | 4.5 | 6.3401 | 8.5579 | Reproduces; pass |
| `marker` on `surface-2` | 3.0 | 3.1682 | 6.8087 | Reproduces; pass |
| `danger` on `surface-2`, `surface` | 4.5 | 4.7327, 5.4384 | 4.6868, 5.0402 | Reproduces; pass |
| `ok` on `surface-2` | 3.0 | 3.6942 | 5.9503 | Reproduces; pass |
| `border-strong` on `bg`, `surface`, `surface-2` | 3.0 | 3.3961, 3.6703, 3.1940 | 3.9369, 3.6825, 3.4243 | Reproduces; pass |
| `bg` on `text` | 4.5 | 15.4242 | 14.6350 | Reproduces; pass |

No row fails or materially differs from the documented rounded result. Decorative borders, hidden skeletons and disabled controls are explicitly excluded rather than accidentally omitted. [design:129](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:129), [design:146](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:146).

## 4. The layout arithmetic

The contract’s static arithmetic is correct. [design:38](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:38)

| Width and state | Independent calculation | Result |
|---|---|---|
| Docked absolute minimum, collapsed rail | `56 + 8 + 480 + 320 = 864` | Fits before the 960 wide breakpoint |
| Docked default, collapsed rail | `56 + 8 + 480 + 400 = 944` | Fits at 960 |
| 960, collapsed, default dock | Work `= 960 − 56 − 8 − 400 = 496`; effective dock maximum `= 960 − 56 − 8 − 480 = 416` | Both minima satisfied |
| 1,127, collapsed, default dock | Work `= 663`; effective maximum `= 583` | Both minima satisfied |
| 1,128, expanded, default dock | Work `= 1,128 − 240 − 8 − 400 = 480`; effective maximum `= 400` | Exact work minimum |
| 1,128, collapsed choice | Work with default `= 664`; effective maximum `= 584` | Both minima satisfied |
| 1,184, collapsed | `56 + 8 + 480 + 640 = 1,184` | Full 640 maximum fits |
| 1,184, expanded | Effective maximum `= 1,184 − 240 − 8 − 480 = 456` | Work minimum retained |
| 1,368, expanded | `240 + 8 + 480 + 640 = 1,368` | Full 640 maximum fits |
| Focused at 960, collapsed | Minimum sum `56 + 480 + 400 = 936`; actual conversation room `504` | 24px spare |
| Focused at 1,128, expanded | Minimum sum `240 + 480 + 400 = 1,120`; actual conversation room `488` | 8px spare |
| Compact at 600 | `600 − 56 = 544` for the one-column workspace; dock sheet `min(400, 544) = 400` | Defined behavior fits |
| Phone below 600 | Rail absent; focused view and dock are full-width one-column surfaces | No competing horizontal minima |

The phrase “400 at 1,128” applies to the expanded-rail case; a remembered collapsed rail permits 584. The formula makes this recoverable and does not create a material ambiguity. Static minima pass; live resizing remains J1.

## 5. Findings

- J1; MATERIAL; State / dock width; the contract specifies the initial pixel size and pre-mount clamp but omits `groupResizeBehavior`, whose default is `preserve-relative-size`. Consequently, when an already-mounted wide window narrows, the dock shrinks proportionally rather than retaining its pixel width until the work panel reaches 480. From an expanded 1,368px viewport, the panels have 1,120px available and begin approximately `720 + 400`; narrowing to 1,128 produces approximately `560 + 320`, rather than the intended `480 + 400`. The pre-mount clamp cannot govern this mounted resize, and the walkthrough does not state the resize history, so its measurements can pass after reload while live behavior remains wrong. [design:199](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:199), [design:236](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:236), [4.13.1 panel types](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/components/panel/types.ts). The human would see a remembered/default 400px dock collapse to 320px even though 400px still fits. Specify the dock as preserving pixel size, leave at least the work panel relative, state the narrowing priority, and exercise wide-to-wide live resizing.

- J2; MATERIAL; Empty states; truthful wording has been supplied, but Overview, Project, Backlog, Fleet, Decisions, Application, Settings, Brain and Subject all specify `none` as their empty-state action; only Not Found offers an action. [design:88](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:88), [design:90](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:90). The master requires empty sections both to explain their contents and “offer an appropriate first action.” [master:437](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface-design.md:437). Settings has live controls outside its nested empty state, but that does not resolve the other dead ends. The human would enter nearly every destination and have no offered next step. Specify a usable first action for each empty section, or explicitly amend the master requirement.

- J3; MATERIAL; Scope, cuts and verification; the scope names “refetch on invalidation” as second-cut work and then calls “the event stream” out of scope, while the change boundary explicitly includes an event-stream subscription. [design:15](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:15), [design:224](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:224). Behavior specifies only digest changes, and `cuts.test.ts` rejects only `/-/health`, `ConnectionIndicator`, and `Notice`, so an event subscription or invalidation-refetch implementation can enter the first cut without failing the promised guard. [design:216](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:216), [design:232](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:232), [design:253](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s9-application-shell-design.md:253). A builder could either omit the second-cut subscription or accidentally ship it in the first cut while all named checks pass. Reconcile the scope language, add invalidation behavior and verification, and make the cut guard reject the subscription/refetch code.

## 6. Not checked

F10 was not reopened, as instructed. Round-one-confirmed areas outside the stated fixes were not reswept. No implementation, build, tests, browser rendering, scripts, MetaSystem binary, package installation, or running interface server were exercised.

---

# Round 3, 2026-09-21: loop closed

On revision 3 at commit `21a6f9c9c`. Verdict: build as written. J1, J2 and J3 all corrected, no material findings. Three rounds: nine findings, then three, then none.

One non-material finding, L1, becomes an obligation on the builder rather than a fourth round: the cut guard must be token-aware, so that it cannot match its own source or be evaded by an alias or a property-handler spelling, and must not read comments or fixture text as calls.

## 1. Verdict build as written loop closed ## 2. Status of J1, J2 and J3 J1: corrected J2: co...
[codex] Turn completion inferred after the main thread finished and subagent work drained.
## 1. Verdict

build as written  
loop closed

## 2. Status of J1, J2 and J3

J1: corrected  
J2: corrected  
J3: corrected

## 3. J1 in detail

The props and units are correct. Numeric `defaultSize`, `minSize`, and `maxSize` are pixels; `groupResizeBehavior` supports `preserve-relative-size` and `preserve-pixel-size`, with relative as the default and at least one relative panel required. The design supplies both behaviors explicitly and keeps the work panel relative. `plans/user-interface/g1-s9-application-shell-design.md:199-201`; pinned 4.13.1 [panel types](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/components/panel/types.ts#L135-L202), [Panel registration](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/components/panel/Panel.tsx#L44-L134).

On a group resize, `preserveFixedPanelSizes` converts the dock’s previous percentage back to pixels, expresses those pixels against the new group, and assigns the remainder to the relative work panel. Constraints are then recomputed from pixels to percentages. `validatePanelGroupLayout` clamps panels in DOM order, accumulates the difference, and offers that difference again in the same order. Because `work` precedes `dock`, work absorbs contraction until its 480 minimum; once it cannot shrink further, dock absorbs the remainder. This is produced by the implementation, not merely stated by the design. [Preservation](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/global/utils/preserveFixedPanelSizes.ts#L4-L81), [constraint conversion](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/global/dom/calculatePanelConstraints.ts#L7-L96), [validation](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/global/utils/validatePanelGroupLayout.ts#L8-L96).

My recomputation:

- At 1,368 expanded, panel room is `1,368 − 240 − 8 = 1,120`: work 720, dock 400.
- At 1,128 expanded, room is `1,128 − 240 − 8 = 880`: preserving dock 400 leaves work 480.
- With dock initially 640, shrinking to 880 first proposes work 240 and dock 640. Validation clamps work to 480, then reduces dock to 400.
- At 1,127 the rail collapses, producing `1,127 − 56 − 8 = 1,063`: dock 400, work 663.
- The smallest collapsed-wide group is `960 − 56 − 8 = 896`; the smallest expanded-wide group is 880. Therefore the overall wide minimum is 880, greater than `480 + 320 = 800`. The dock may shrink from 640 to 400, but cannot reach its 320 floor while wide. `plans/user-interface/g1-s9-application-shell-design.md:34-46,201,216-218`.

The resize observer calls `updateMountedGroup` without interaction metadata; the state owner consequently emits `isUserInteraction: false`, which `Group` passes to `onLayoutChanged`. The design’s persistence condition therefore ignores window-resize recomputations. [Resize observer](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/global/mountGroup.ts#L46-L103), [metadata default](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/global/mutable-state/groups.ts#L93-L108), [callback propagation](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/components/group/Group.tsx#L309-L355). Upstream’s own pinned test also expects a pixel-preserving group resize to report `isUserInteraction: false`. [Group test](https://github.com/bvaughn/react-resizable-panels/blob/4.13.1/lib/components/group/Group.test.tsx#L557-L611).

The walkthrough catches the original defect. Without the dock prop, 720:400 remains proportional at 880, then the dock minimum produces approximately work 560/dock 320—not the required 480/400 at the first 1,368→1,128 step. Its 640→400→widen→reload sequence additionally proves that resize squeezing is not persisted while the earlier user-selected 640 is. `plans/user-interface/g1-s9-application-shell-design.md:246,261`.

## 4. The cut guard

It covers both original failure modes:

- Omission from the second cut is caught by the explicit `events.ts` boundary, the three invalidation cases, and `events.test.ts`. `plans/user-interface/g1-s9-application-shell-design.md:222-224,232,242`.
- Ordinary leakage into the first cut is caught by the single-call-site assertion, forbidden transport/timer/lifecycle checks, absence of the four second-cut files, and the browser network trace across load, Retry, refocus, offline/online, and two idle minutes. `plans/user-interface/g1-s9-application-shell-design.md:240,246,263`.

It is not an adversarial source-policy proof. Inference: aliases, computed properties such as `globalThis["fetch"]`, `onfocus = …`, or a recursive `setTimeout` can evade a literal implementation. A normally wired second-cut subscription or focus/online refetch would still fail the runtime half; evading both would require conditional or deliberately obscured behavior contrary to the stated implementation.

A naïve raw-text scan could also false-fail on its own test patterns, comments, strings, or an unrelated identifier such as `prefetch(`. A token-aware matcher, or constructed matcher literals while still scanning every file, avoids that without weakening the production assertion. This is implementation-level hardening, not a remaining design choice.

The dependency on `g1-s7` is narrow and honest: exactly the stream path, event shape, and workspace-resource view name. The transport is already fixed as server-sent events; reconnection uses browser `EventSource` behavior. Nothing else is delegated. `plans/user-interface-implementation-plan.md:185`; `plans/user-interface/g1-s9-application-shell-design.md:15,84`.

## 5. Findings

L1; NOT MATERIAL; Verification / cut guard; a naïve raw-substring implementation can self-match or be evaded by aliases and property-handler spellings; evidence: the design specifies textual matches over every file plus a runtime half at `plans/user-interface/g1-s9-application-shell-design.md:240,246,263`; no conforming first-cut product behavior would be built differently, and ordinary manifestations of the original regression remain observable; direction: make the implementation token-aware and include equivalent lifecycle-property and recursive-timer forms without scanning comments or fixture text as executable calls.

## 6. Not checked

No build, test suite, package installation, browser walkthrough, running-server interaction, or repository script was executed. The contrast table, settled layout arithmetic, F1, F2, F5, F6, F7, F9, and F11 were not reopened. The pinned source was read from the extracted 4.13.1 tag archive; its tests were inspected but not run.
