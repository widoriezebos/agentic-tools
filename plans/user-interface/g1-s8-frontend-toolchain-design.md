# g1-s8 Frontend toolchain and embedded bundle

- Gate 1, state `designed`, revision 2 after Sol's design review (D12), author Claude on Fable as a D23 delegate, 2026-09-21.
- Revision 2 answers E1 to E6 and E12 of [Sol's review](g1-s8-frontend-toolchain-design-sol-review.md): an engine-side exclusion for the dependency tree from a slice that lands first; exact versions, `--ignore-scripts`, an audit gate; the manifest as publish point; `Dialog.Overlay`; reserved paths and `Accept` parsing; the digest's text rule and canonical manifest; Node `24.21.0`.
- Refines the [master read](../user-interface-design.md) at `ui-development` commit `0a5ef07a5` and [Astra's second review, section 4](../user-interface-design-critique-astra-r2.md). Depends on `g1-s1` revision 6, the exclusion slice below, and D1, D7, D8, D13, D14, D15, D20, D21, D23; the technical half of D21 step 2, on which `g1-s9` puts the shell.

Paths are relative to `metasystem/` unless they start with `plans/`. Code claims were verified by reading the source at `762446dfe` and the Go 1.27.1 tree; nothing was executed. Registry and upstream facts were read on 2026-09-21.

## Outcome

A React page, built ahead of time, committed, and embedded in `metasystem`, shows the facts from `/-/health`, a button opening a modal dialog, the path requested, and a link to the open-source notices; any path reloads to it. Nothing is fetched from outside the server, no inline script runs, and the dialog's scroll-lock style is allowed by a per-response nonce. `go test ./internal/ui/web/` fails when the bundle is stale or mismatches its manifest. A build without a bundle says so. Installing the dependencies changes no engine digest.

## Scope and non-goals

In scope: the layout and its fence, including what the exclusion slice must deliver; the npm toolchain, Node pin, and bundle script; `internal/ui/web`; the manifest and its tests; routes, fallback, content types, and the policy in `internal/ui/httpd`; development mode; notices; Vitest; the modal check. Not: the exclusion slice's code; the shell, a router, themes, tokens (`g1-s9`, `g1-s18`); `/api/` (`g1-s7`); caching beyond `no-store`; sign-in; `internal/ui/lifecycle`.

## Existing code this builds on

The [g1-s1 design](g1-s1-server-lifecycle-design.md): checks, five headers, `GET /-/health`, placeholder at `/`, 404 otherwise, wired in `cmd/metasystem/ui.go`. The build has no pattern (`scripts/agents/go-build.sh:99` to `:101`); `./...` excludes `_` directories and nested modules (Go 1.27.1 `src/cmd/go/alldocs.go:3208` to `:3213`). ENGINE is `cmd/**`, `internal/**`, `scripts/agents/**`, `go.mod`, `go.sum`, one record (`internal/behaviorsurface/policy.v2.json:3` to `:10`); its walk skips only `.git` and the prefix filter (`policy.go:347` to `:436`) and governs admission (`internal/dispatch/governed.go:27`). Proof groups declare `metasystem/internal/**` (`testing.json:50`, `:52`, `:60`, `:75`, `:76`, `:79`, `:102`); every match is hashed (`internal/proofrun/test_build.go:1780` to `:1808`). The gate runs gofmt, the ratchet, and the stop-surface audit (`scripts/agents/go-gate.sh:520`, `:170`, `:584`; `cmd/metasystem/audit.go:132`, `:143`). Embed refuses `..`, symlinks, and nested modules; `all:` includes dot and underscore names (`src/embed/embed.go:68` to `:96`). Adoption is `git archive HEAD` under an allowlist containing `internal`, merging the top-level `.gitignore` (`scripts/adopt.sh:228` to `:234`, `:246`, `:365`, `:373` to `:379`).

## Contracts

### Source layout

```
internal/ui/web/                 Go package web
  embed.go web.go digest.go accept.go contenttype.go *_test.go testmain_test.go
  bundle/                        //go:embed all:bundle
    README.txt                   committed, so the pattern always matches
    bundle.json                  the manifest, written last
    dist/                        Vite output, emptied on every build
      index.html assets/app.js assets/app.css assets/*.woff2 THIRD-PARTY-NOTICES.txt
  _app/                          the React source, the npm project root
    package.json package-lock.json .nvmrc .npmrc index.html vite.config.ts tsconfig.json
    src/ public/ scripts/ node_modules/ (ignored; excluded below)
```

**The fence.** Go tooling over `./...` never enters an underscore directory; no nested `go.mod`, no `ignore` directive. The engine's own walkers do enter it: installed dependencies would join the ENGINE digest and the proof-input digests (E1); underscore, `.gitignore`, and pruning `.go` files fence none of them. A separate slice, specified next, adds the exclusion; until it is on `ui-development`, no dependency may be installed in this repository: no `npm ci`, no `npm install`, no restored cache. Revision 1's `postinstall` prune of `.go` files is dropped: `--ignore-scripts` skips lifecycle scripts, the prune left a window after install, and once the walkers are excluded and gofmt is fed from Git, a `.go` file inside `node_modules` is invisible to every tool.

`bundle/` is inside the package directory of the engine's own module: no module crossing, no `..`. `Dist()` is `fs.Sub(bundle, "bundle/dist")`; `README.txt` and `bundle.json` are never served. `metasystem/.gitignore` gains one line, `/internal/ui/web/_app/node_modules/`, anchored so a root installation does not ignore the application's own `node_modules`, in the top-level file because adoption drops nested ones. Adoption ships `_app`, `bundle/`, and the Go files, never `node_modules`; source ships because the payload rebuilds engine source (`adopt.sh:242` to `:245`), the bundle because the adopter has no Node.

### The exclusion slice

Lands before this slice, by its own design, critique, and build; the planner numbers it. It edits `internal/behaviorsurface`, `internal/pathpattern`, `internal/proofrun`, `internal/parallelratchet`, `internal/audit`, and `scripts/agents/go-gate.sh`, none of which `g1-s8` touches. It must deliver:

**Membership.** `policy.v2.json` `nonRepositoryPaths` gains `internal/ui/web/_app/node_modules/**`, and `Policy.Includes` (`policy.go:279` to `:312`) returns false for a path matching `nonRepositoryPaths` in every projection, not only LANDING (`:306`). The existing list is the home: its class already means content on disk that is not repository content (`.git/**`, `bin/**`, rendered docs, `metasystem.conf.local`; `policy.go:42`, `policy.v2.json:76` to `:82`), and ENGINE (`:304`) and PAYLOAD (`:308`) not honouring it is the gap E1 fell through. The glob is exact because the grammar allows an exact path or a `/**` prefix and nothing between (`:54` to `:56`, `:147` to `:156`), and installation-relative, naming the same tree nested or at the root (`NormalizePath`, `:183` to `:213`). Every consumer of `Includes` follows unchanged: the digest walk (`:347` to `:436`), `ListPaths` (`:442` to `:475`), admission (`internal/dispatch/governed.go:27`), the export (`internal/proofrun/manifest.go:164`), and the classifiers at `cmd/metasystem/proof_run.go:2157`, `landing_batch_land.go:359`, `rearm_on_landed.go:243`, `internal/steward/rearm_resolver.go:576`, `internal/gaterun/weight.go:248`. A walk may `SkipDir` a matched directory; that is speed, not membership.

**Walkers without a policy** skip a directory named `node_modules` at any depth, as `vendor` is skipped by name in `parallel.go:123`, `internal/testselect/select.go:199`, `internal/gopackages/select.go:278`, and `internal/landing/batch/unitgate.go:307`: `pathpattern.Pattern.Expand` (`pattern.go:139` to `:171`, beside its `.git` rule at `:151`; `testing.json`'s `metasystem/internal/**` declarations reach it through `test_build.go:1786`); `parallelratchet.ScanParallelTests` (`parallel.go:123`); `audit.discoverStopSurfaceFiles` (`stopsurface.go:304`; its Git-tree twin at `:326` to `:349` needs nothing); `proofrun.readManifest` (`manifest.go:96` and `:101` through `hardExcluded`, `:148` to `:154`), so a frozen candidate receives no copy of the tree. One name repeated four times beats a shared predicate: `pathpattern` imports nothing internal, `parallelratchet` only `atomicfile`, `audit/stopsurface` only `gittree`, and a predicate would thread a `behaviorsurface` import through packages another agent has in flight. One fixture test per walker: a `node_modules/x_test.go` beneath the root is not seen. `testselect` (`select.go:194` to `:202`) has no production caller (E10, an inference) and is left alone.

**gofmt.** `gofmt.go:423` to `:433` recurses into every directory and `:93` to `:95` skips only dot-prefixed file names, so `go-gate.sh:520` becomes a Git-fed list, from `metasystem/`: `git ls-files -z --cached --others --exclude-standard -- '*.go' | xargs -0 -r gofmt -l`: every Go file Git sees, tracked or new, never an ignored tree; `-r` keeps gofmt off stdin on an empty list (BSD and GNU both accept it). A "no `.go` beneath `node_modules`" rule alone is not enough, for the reasons under the fence.

**Digest neutrality**, what makes this safe beside in-flight work: an exclusion whose glob matches nothing changes no digest. The glob matches nothing while `_app` is absent; no existing `nonRepositoryPaths` entry lies under an `enginePaths` root; under PAYLOAD, `docs/paper/.obsidian/**` and `docs/paper/rendered/**` lie under `docs/**` but are absent from this checkout. The slice records, with `_app` absent, that for each projection `bin/metasystem behavior-surface digest --root <toplevel> --prefix metasystem --projection P --endpoint check` (`cmd/metasystem/behavior_surface.go:145`) prints the same digest from binaries built at the base commit and at the slice's commit, over the same tree, and that `audit parallel-ratchet` and `audit stop-decision-surface` agree. That slice lands; only then does the first `npm ci` run.

### The build

**Node `24.21.0`**, decided: the Active LTS (Krypton, 2026-09-07, npm 11.19.0), supported to April 2028; `26.9.0` is Current until its LTS transition on 28 October 2026. Recorded in `_app/.nvmrc` and `engines.node`; enforced by the bundle script, which refuses unless `process.version` is `v24.21.0`, naming both. A developer on `26.8.2` installs `24.21.0` with a version manager that honours `.nvmrc` (`nvm`, `fnm`, `mise`) and leaves the Homebrew Node alone.

**Installs** are `npm ci --ignore-scripts`; `_app/.npmrc` carries `ignore-scripts=true` and `save-exact=true`; `package-lock.json` is committed; `package.json` pins every version exactly. No direct dependency declares an install script (registry, 2026-09-21); the native binaries of TypeScript, Rolldown, Lightning CSS, and `@tailwindcss/oxide` are optional platform packages. Transitive install scripts are not verified here: the first `npm ci --ignore-scripts` with green `npm test` and `npm run bundle` proves none is needed; a package that needs one is a finding, never a reason to drop the flag. Playwright's browser: `node_modules/.bin/playwright install chromium`, into the user cache.

**Audit.** The bundle script runs `npm audit --audit-level=low` over the whole tree, development dependencies included, and refuses on any advisory: the human's rule is no vulnerabilities, so there is no exceptions file; a builder facing an advisory without a fix stops and reports. Offline, it refuses rather than skips.

Scripts: `dev` (`vite`), `typecheck` (`tsc --noEmit`), `test` (`vitest run`), `bundle` (`node scripts/bundle.mjs`), `check:modal` (`node scripts/modal-check.mjs <address>`); no `postinstall`; `node_modules/.bin` binaries, never `npx`. `bundle` runs, in order: the Node check; `npm audit`; removes `bundle/bundle.json`; `typecheck`; `vite build`, which empties `dist/`; `scripts/notices.mjs`; then writes `bundle/bundle.json`. The manifest is the publish point (E3): removed before the first destructive step, written after the last, so a failed build leaves no manifest, and no manifest, or an unparsable one, is an absent bundle everywhere: `ReadManifest` errors, `TestBundleMatchesManifest` fails, and `cmd/metasystem/ui.go` passes no bundle to the handler.

`vite.config.ts`: `base: "/"`; `plugins: [react(), tailwindcss()]`; `build.outDir: "../bundle/dist"`, `emptyOutDir: true`; `assetsInlineLimit: 0` (no `data:` URLs); `sourcemap: false`; `manifest: false`; `modulePreload: { polyfill: false }`; `rolldownOptions.output` (Vite 8 bundles with Rolldown): `entryFileNames: "assets/[name].js"`, `chunkFileNames: "assets/[name].js"`, `assetFileNames: "assets/[name][extname]"`; `server.host: "127.0.0.1"`, `port: 5173`, `strictPort: true`, and the proxy under Development mode. `tsconfig.json`: `strict`, `noEmit`, `lib: ["ES2022", "DOM", "DOM.Iterable"]`, `moduleResolution: "bundler"`, `jsx: "react-jsx"`, `verbatimModuleSyntax`, `isolatedModules`, `include: ["src", "scripts", "vite.config.ts"]`. Not verified: that TypeScript 7.0.2 accepts every option named; the first `npm run typecheck` proves it. No fallback compiler.

Runtime dependencies, exact versions from the registry:

| Package | Licence | Why |
| --- | --- | --- |
| `react` 19.3.0, `react-dom` 19.3.0 | MIT | D1 |
| `@radix-ui/react-dialog` 1.1.23 | MIT | The modal; its scroll lock is the nonce path: `react-remove-scroll` `^2.7.2` (2.7.2) → `react-remove-scroll-bar` `^2.3.7` (2.3.8) and `react-style-singleton` `^2.2.3` (2.2.3) → `get-nonce` `^1.0.0` |
| `get-nonce` 1.0.1 | MIT | `main.tsx` calls `setNonce` on the instance Radix uses; 1.0.1 satisfies `^1.0.0`, so the lockfile holds one copy (`lockfile.test.ts`) |
| `lucide-react` 1.47.0 | ISC | D15's icon set; the close icon |
| `@fontsource-variable/inter` 5.3.0 | OFL-1.1 (registry field, `LICENSE`, upstream `rsms/inter`) | The interface face |

Development dependencies, exact:

| Package | Licence | Why |
| --- | --- | --- |
| `typescript` 7.0.2 | Apache-2.0 | `tsc --noEmit`; the native compiler, its binary an optional platform package |
| `vite` 8.3.0 | MIT | The bundler; Rolldown 1.2.9 beneath, no esbuild |
| `@vitejs/plugin-react` 6.1.1 | MIT | Peer `vite ^8.0.0` |
| `tailwindcss` 4.3.3, `@tailwindcss/vite` 4.3.3 | MIT | Peer `vite ^5.2 \|\| ^6 \|\| ^7 \|\| ^8` |
| `vitest` 5.0.1 | MIT | Peer `vite ^6.4 \|\| ^7 \|\| ^8`; engines include `^24.0.0` |
| `@types/react` 19.3.0, `@types/react-dom` 19.3.0 | MIT | Types |
| `playwright` 1.63.0 | Apache-2.0 | The modal check only |

Reaches the shipped bundle: the runtime table and its lockfile closure, exactly what `notices.mjs` lists, plus the CSS Tailwind emits. Build-time only, never in `dist/`: the development table and its closure, including Lightning CSS (MPL-2.0). Only Inter's Latin and Latin-extended `wght` faces are bundled, through a hand-written `@font-face` block in `src/fonts.css` naming `inter-latin-wght-normal.woff2` and `inter-latin-ext-wght-normal.woff2` under `@fontsource-variable/inter/files/`. Not included: a router (`g1-s9` decides), a DOM test environment, a linter or formatter, source maps, a favicon, any CDN or remote font, a Markdown renderer (D6).

Deferred to the first slice that uses them: `@fontsource-variable/jetbrains-mono` 5.3.0 (OFL-1.1) and `react-resizable-panels` 4.13.1 (MIT). E12: current `react-resizable-panels` exports no `setNonce` and applies its cursor style through `CSSStyleSheet` and `document.adoptedStyleSheets` (`lib/index.ts`, `lib/global/cursor/updateCursorStyle.ts` on `main` at 4.13.1; the review found the same at 4.12.4). The slice that installs it must prove in the browser, under this policy, that the cursor style applies with zero `securitypolicyviolation` events; whether the policy governs constructed sheets is not verified here.

### The embedding package

```go
package web
//go:embed all:bundle
var bundle embed.FS
const NoncePlaceholder = "__METASYSTEM_CSP_NONCE__"
func Dist() fs.FS                             // bundle/dist
type FileDigest struct{ Path, SHA256 string } // "/" separators; lower-case hex
type Manifest struct {
    SchemaVersion int               `json:"schemaVersion"` // 1
    SourceDigest  string            `json:"sourceDigest"`  // "sha256:<hex>" over _app
    Source        []FileDigest      `json:"source"`
    Files         []FileDigest      `json:"files"`         // every file under dist/
    Tools         map[string]string `json:"tools"`         // node, vite, typescript, tailwindcss
}
func ReadManifest() (Manifest, error)         // ErrNoManifest when absent; an error when unparsable
func SourceDigest(dir string) (string, []FileDigest, error)
func AcceptsHTML(accept string) bool
func ContentType(name string) (string, bool)  // false outside the table
```

**Digest rule**, in `digest.go` and `_app/scripts/digest.mjs`. Walk `dir`. Skip any directory named `node_modules` and any entry whose name begins with `.` except `.nvmrc` and `.npmrc`. A symbolic link anywhere is an error: its target could change the build without changing the digest. Each remaining regular file contributes its path relative to `dir` with `/` separators, ASCII only, and the SHA-256 of its content: for a text file, after every `\r\n` becomes `\n`; otherwise byte for byte. A text file is `.nvmrc`, `.npmrc`, or one whose extension is among `.ts .tsx .mts .cts .js .mjs .cjs .jsx .json .css .html .svg .txt .md`; everything else (`.woff2`, `.png`, ...) is bytes. Modes and times are ignored. Lines `<hex>  <path>\n` are sorted by path bytewise and concatenated; the digest is `sha256:` plus the hex SHA-256 of that text. Fixture: `.nvmrc` = `24.21.0\n`, `a.txt` = `x\r\n`, `b/c.txt` = `y\n`, `d.bin` = the two bytes `0d 0a`, `.hidden` = `z`, `b/node_modules/q.txt` = `q`; the digest is `sha256:f8c8104462398072962c697d4b860b1e76da903a0fa41933ffc541bcbb3e1399`, `d.bin` contributing `7eb70257593da06f682a3ddda54a9d260d4fc514f645237f5ca74b08f8da61a6`, its raw bytes. Both implementations assert the literal.

**Manifest canonical form** (E6). `Source` and `Files` sorted by path bytewise ascending, paths ASCII only; `Files` digests are of the bytes as served, never normalised; `Tools` has exactly the four keys, package versions from each `package.json`, `node` from `process.version`. The script writes `JSON.stringify(manifest, null, 2) + "\n"` with keys in the declared order; no timestamps anywhere. Go only reads it.

**Content types**, a fixed table, since `mime.TypeByExtension` reads host files: `.html` `text/html; charset=utf-8`, `.js` `text/javascript; charset=utf-8`, `.css` `text/css; charset=utf-8`, `.json` `application/json`, `.txt` `text/plain; charset=utf-8`, `.svg` `image/svg+xml`, `.woff2` `font/woff2`, `.png` `image/png`.

### Serving

`httpd.New(info Info, bound net.Addr, bundle fs.FS) http.Handler`; `Info` gains `BundleDigest string`, filled by `cmd/metasystem/ui.go` from `web.ReadManifest().SourceDigest`; when `ReadManifest` errors, the digest is empty and the bundle passed is `nil`, which the handler treats as absent without opening it. An unexported constructor also takes `nonce func() string` for tests. The `g1-s1` checks and headers run first, unchanged. Then:

1. Reserved: `/-/health` as before plus `"bundleDigest"`. The prefixes `/-`, `/api` (`g1-s7` takes it), and `/assets`, exact or with anything beneath, are never the page (E5): what is not health or a served static file under them is 404.
2. Static: `path.Clean`, strip the leading `/`, require `fs.ValidPath`, refuse `index.html` and directories; a regular file is served with its table content type, or 404 when its extension is outside the table. `/index.html` is therefore the page under the next rule, never a redirect.
3. Page: `/`, or any remaining path whose request accepts HTML: `index.html` with the nonce substituted, 200. Everything else: 404. `AcceptsHTML` splits `Accept` on commas and parses each element with `mime.ParseMediaType`, which lower-cases the type (`src/mime/mediatype.go:144`) and separates parameters; a malformed element is skipped. The request accepts HTML iff some element's type is exactly `text/html` (not `text/*`, not `*/*`) and its `q` parameter is absent or parses with `strconv.ParseFloat` to more than zero. No `Accept` header: not accepted.

Absent bundle: at construction the handler reads `index.html`; when the bundle is `nil` or that fails, every page response is 503 with the text `MetaSystem interface: this executable was built without the interface bundle. Rebuild it from a checkout that contains internal/ui/web/bundle/dist, then run: metasystem ui restart`; static lookups are 404; health answers with an empty `bundleDigest`. `Cache-Control: no-store` stays on every response.

### Content security policy

Header `Content-Security-Policy` on every response. On a page response:

```
default-src 'none'; script-src 'self'; style-src 'self' 'nonce-<nonce>'; img-src 'self'; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'
```

On every other response the same string without ` 'nonce-<nonce>'`. No `unsafe-inline`, `data:`, host source, `report-uri`, or Trusted Types.

The nonce is 16 bytes from `crypto/rand` in `base64.RawStdEncoding` (22 characters, a valid CSP `base64-value`), generated once per page response for header and body. `_app/index.html` carries `<meta property="csp-nonce" nonce="__METASYSTEM_CSP_NONCE__">` and nothing else inline: no `<script>` without `src`, no `style` attribute, no `<style>`. At construction the handler splits the embedded `index.html` on `web.NoncePlaceholder`; per response it writes the parts around the nonce. `src/nonce.ts` returns the meta element's `nonce` IDL property, not `getAttribute`, because browsers hide the attribute after parsing; `main.tsx` calls `setNonce(nonce)` from `get-nonce` before the first render, so `react-style-singleton` sets it on the `<style>` it appends. React `style` props write CSSOM properties, which the policy does not govern. Vite's `html.cspNonce` is not used.

### Development mode

`npm run dev` starts Vite on `127.0.0.1:5173` with hot reload and proxies `/api` and `/-` to `http://127.0.0.1:7878`, overridable by `METASYSTEM_UI_PROXY_TARGET`, with `changeOrigin: true` (the Go server sees `Host: 127.0.0.1:7878`) and `Origin` removed; the browser's `Sec-Fetch-Site: same-origin` is forwarded and passes. No check is loosened; nothing development-only exists in Go. Vite serves the page without the policy and with the placeholder literal.

### Licence notices

`scripts/notices.mjs` reads `package-lock.json`, walks the runtime closure from the root's `dependencies` (nested `node_modules` resolution, ignoring `dev` and `optional` entries), and for each package reads `name`, `version`, `license`, and its `LICENSE*` or `LICENCE*` file, refusing when one has none. It writes `dist/THIRD-PARTY-NOTICES.txt`: a header, then per package the name, version, identifier, and text: over-attribution, even of a package tree-shaking drops, and no bundler hooks. The file is committed, embedded, served at `/THIRD-PARTY-NOTICES.txt`, and linked from the page as "Open-source notices", a link `g1-s9` keeps. Build tools are not listed.

### Frontend tests

Vitest runs in the `node` environment, `vitest run`, development-time only. Three tests: `scripts/digest.test.ts` (the fixture literal, from a temporary directory); `scripts/lockfile.test.ts` (`package-lock.json` has exactly one key ending in `/get-nonce`, at 1.0.1); `src/nonce.test.ts` (`readNonce` returns the element's `nonce` property, or `""`).

## Behaviour

**Page.** `src/App.tsx` renders the heading `MetaSystem interface`; the facts fetched from `/-/health`; `path: <location.pathname>`; a button `Open a dialog`; the notices link. The dialog is `Dialog.Root`, `Dialog.Trigger`, and `Dialog.Portal` containing `Dialog.Overlay` and `Dialog.Content` with `Dialog.Title`, `Dialog.Description`, a sentence, and `Dialog.Close` with a lucide icon. `Dialog.Overlay` is required (E4): at 1.1.23 the scroll lock is mounted by `DialogOverlayImpl`, which wraps its element in `RemoveScroll` (`packages/react/dialog/src/dialog.tsx:216` to `:251` on `main`, whose `package.json` reads 1.1.23), so a dialog without `Overlay` never appends the `<style>` and would pass the nonce proof by never exercising it.

| Case | Required result |
| --- | --- |
| Node differs from `.nvmrc`; `npm audit` reports any advisory or cannot run | Bundle script refuses, naming both versions, the advisories, or the failed audit |
| Source changed after the last bundle | `TestBundleIsCurrent` fails listing changed, added, and removed paths and says `run npm ci --ignore-scripts && npm run bundle in internal/ui/web/_app` |
| A `dist/` file not matching `bundle.json` | `TestBundleMatchesManifest` names the file whose digest differs, is missing, or is extra |
| `dist/` empty or absent; or `bundle.json` absent or unparsable | `go build` succeeds through `README.txt`; the server serves the 503 statement; `TestBundleMatchesManifest` fails (`bundle missing` without a manifest) |
| `/assets/missing.js`, also from the address bar; `/assets/`; `/assets`; `/api/goals` and `/api`; `/-`; `/nothing` with `Accept: */*`, `text/html;q=0`, `text/*`, or no `Accept` | 404 |
| `/some/deep/path` with `Accept: TEXT/HTML;level=1, */*;q=0.1` | The page, 200 |
| Dialog opened in the embedded build | `body` overflow hidden, one `<style nonce>` in `head` whose nonce equals the meta's, zero `securitypolicyviolation` events |

## Change boundary

New: `internal/ui/web/` (Go sources, tests, `testmain_test.go`, `bundle/README.txt`, `bundle/bundle.json`, `bundle/dist/**`, `_app/**` except `node_modules`). Edited, and nothing else in them: `internal/ui/httpd/` sources for routes, policy, `Accept`, and constructor; `internal/ui/httpd/*_test.go` from `g1-s1`, only the cases asserting the placeholder at `/`, the constructor's arity, the 404 rule, and the exact health payload; `cmd/metasystem/ui.go`, the `NewHandler` wiring and `Info.BundleDigest`; `metasystem/.gitignore`, one line; `docs/architecture.md`, one package-map row for `ui/web`.

Must not be touched: the exclusion slice's files (`internal/behaviorsurface`, `internal/pathpattern`, `internal/proofrun`, `internal/parallelratchet`, `internal/audit`, `scripts/agents/go-gate.sh`); `testing-parallel-ratchet.json`, `testing.json` (it names no surface under `internal/ui`, which falls to `residual`), `go.mod`, `go.sum`, `internal/testenv`, `internal/testutil`, `internal/testselect`, `internal/testpolicy`, `internal/testexec`, `internal/hostload`, `internal/gopackages`, `cmd/metasystem/test*.go`, `cmd/metasystem/audit.go`, `cmd/metasystem/main.go`, `internal/ui/lifecycle`, `internal/config`, `scripts/**`, `metasystem.conf`, and any other existing `*_test.go` or `testmain_test.go`. No new Go module dependency: `embed`, `io/fs`, `crypto/rand`, `crypto/sha256`, `encoding/json`, `mime`, `strconv`, `net/http`, `path` suffice.

Conventions, because the builder sees only this document: every new package has `testmain_test.go` with `func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }`; tests never call `os.Setenv`; every top-level test and independent subtest calls `t.Parallel()`, the ratchet allowing an unlisted package zero serial tests; no sleeps, polling, or fixed ports (`httptest`); temporary trees under `t.TempDir()`; `testutil.Expect` and `Require`; the nonce generator injected; `cmd/metasystem/ui.go` wires and prints only. The Go tests read `_app` and `bundle` relative to the package directory and need no Node. The first install happens only after the exclusion slice is on the branch; the bundle is rebuilt as the last commit after rebasing (D7).

## Verification

Go tests, package `web`: `TestSourceDigestFixture` (the literal, symlink and non-ASCII errors); `TestBundleMatchesManifest`; `TestBundleIsCurrent` (skips when `_app` is absent); `TestDistIsServable`; `TestIndexIsStrict` (one placeholder; nothing inline; no `http:` or `https:` in `src` or `href`); `TestReadManifestAbsent`, `TestReadManifestUnparsable`; `TestAcceptsHTML`. Package `httpd`, on `fstest.MapFS` bundles with the fake nonce: each Behaviour row; the policy and the five `g1-s1` headers on every response class; body nonce equals header nonce and differs between responses; the 503 for `nil` and for a bundle without `index.html`.

Obligations the code critique checks by name:

- **O1, exclusion.** The exclusion slice is on the branch before the first install. With `node_modules` installed and absent, the same binary prints the same ENGINE, LANDING, and PAYLOAD digests, `audit parallel-ratchet` and `audit stop-decision-surface` agree, and the gate's gofmt, vet, and staticcheck pass with the tree installed.
- **O2, no inline, nothing external.** `TestIndexIsStrict` passes on the committed bundle.
- **O3, nonce end to end.** Fresh per response, in header and body; `main.tsx` calls `setNonce` before rendering; `App.tsx` renders `Dialog.Overlay`; the modal check asserts the dialog row.
- **O4, honest 404.** The reserved prefixes, exact and beneath, never receive the page; `AcceptsHTML` is the parsed rule, not a substring test.
- **O5, staleness.** Both digest implementations assert the fixture literal; the text-extension list and the exclusions are exactly those written here.
- **O6, absent bundle.** `go build ./cmd/metasystem` succeeds with `bundle/dist` removed; the server answers the 503 statement, also when only `bundle.json` is missing.
- **O7, notices.** Every package in the lockfile's runtime closure, with its licence text; the page links to the file.
- **O8, no loosening.** The `g1-s1` checks and headers are as designed; the proxy exists only in `vite.config.ts`.
- **O9, supply chain.** `package.json` versions equal the tables; `.npmrc` as written; no `postinstall`; the lockfile committed; the bundle script runs the Node check and `npm audit --audit-level=low` and refuses as specified.
- **O10, deterministic manifest.** Two bundles from one tree are byte-identical in `dist/` and `bundle.json`; `Source` and `Files` sorted as written.

Commands, from `metasystem/`: `go build ./...`; `go vet ./internal/ui/... ./cmd/metasystem/`; `go test ./internal/ui/...`; `gofmt -l internal cmd`; `bin/metasystem audit parallel-ratchet` without `--update`; the digest comparisons of O1. From `internal/ui/web/_app/`: `npm ci --ignore-scripts`, `npm run typecheck`, `npm test`, `npm run bundle` twice with `git status --porcelain` clean between.

Walkthrough, by Claude after the code critique: build `bin/metasystem`; `ui start`; in a real browser see the health facts; open the dialog and confirm in devtools a `<style nonce>` in `head`, `body` overflow hidden, no policy violation, every request to the server's origin; reload at `/some/deep/path`; open `/THIRD-PARTY-NOTICES.txt`; `curl -i /assets` gives 404, `curl -i /` gives 200 with the nonce in header and body; `npm run check:modal -- http://127.0.0.1:7878` passes; `npm run dev` shows the page through the proxy; remove `bundle/dist`, rebuild, `ui restart`, see the 503 statement, restore. The modal check is Claude's, never a Go test's or the gate's.

## Open questions

None. The Node line is decided under The build.
