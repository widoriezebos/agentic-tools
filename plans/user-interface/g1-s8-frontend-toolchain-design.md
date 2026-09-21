# g1-s8 Frontend toolchain and embedded bundle

- Gate 1, state `designed` pending Sol's design review (D12), author Claude on Fable as a D23 delegate, 2026-09-21.
- Refines the master read at `ui-development` commit `0a5ef07a5`: [Workspace identity](../user-interface-design.md#workspace-identity-the-subject-and-the-machinery), [Implementation of the shared interaction](../user-interface-design.md#implementation-of-the-shared-interaction), [Human acts from the browser](../user-interface-design.md#human-acts-from-the-browser), [Trustworthy state](../user-interface-design.md#trustworthy-state-and-interaction); and [Astra's second review, section 4](../user-interface-design-critique-astra-r2.md): placement and the Go boundary, licence notices, the Radix modal and the style nonce.
- Depends on `g1-s1` revision 6 and on D1, D7, D8, D13, D14, D15, D20, D21, D23.
- Discharges nothing by itself; it is the technical half of D21 step 2, on which `g1-s9` puts the shell.

Paths are relative to `metasystem/` unless they start with `plans/`. Code claims were verified by reading the source at `0a5ef07a5` and the Go 1.27.1 tree; nothing was executed. Registry facts were read on 2026-09-21.

## Outcome

A human runs `metasystem ui start` and opens the address. A React page, built ahead of time, committed, and embedded in the executable, shows the facts from `/-/health`, a button that opens a modal dialog, the path the browser asked for, and a link to the open-source notices. Reloading any path shows the same page. Nothing is fetched from outside the server, no inline script runs, and the dialog's injected scroll-lock style is allowed by a per-response nonce. `go test ./internal/ui/web/` fails when the committed bundle is older than the source or does not match its manifest. An executable built without a bundle says so.

## Scope and non-goals

In scope: the source layout and its fence from Go tooling; the npm toolchain, Node pin, and bundle script; the embedding package `internal/ui/web`; the manifest with its staleness and integrity tests; routes, fallback, content types, and the policy in `internal/ui/httpd`; development mode; licence notices; the Vitest harness; the modal check.

Not in scope: the shell, navigation, a router, themes, tokens (`g1-s9`, `g1-s18`); read models and `/api/` routes (`g1-s7`); caching beyond `no-store`; sign-in; any change to `internal/ui/lifecycle` or the engine.

## Existing code this builds on

| What, and what it means here | Where |
| --- | --- |
| Handler constructor, request checks, five headers on every response, `GET /-/health`, placeholder at `/`, 404 otherwise; `NewHandler` wired in `cmd/metasystem/ui.go` | [g1-s1 design](g1-s1-server-lifecycle-design.md) |
| `go build ./cmd/metasystem`, no pattern | `scripts/agents/go-build.sh:99` to `:101` |
| `./...` excludes `_`-prefixed directories and nested modules | Go 1.27.1 `src/cmd/go/alldocs.go:3208` to `:3213` |
| `gofmt -l internal cmd` walks every directory, skipping nothing; the parallel ratchet parses every `_test.go` under the working tree, skipping only `.git`, `artifacts`, `vendor` | `scripts/agents/go-gate.sh:520`; `src/cmd/gofmt/gofmt.go:423` to `:433`; `internal/parallelratchet/parallel.go:118` to `:130`; `cmd/metasystem/audit.go:143` |
| Package selection walks a materialised committed tree, skips nested modules, excludes `_` components; a change under `_app` selects `./internal/ui/web` | `internal/gopackages/select.go:87` to `:99`, `:166` to `:181`, `:273` to `:288`, `:505` to `:515` |
| Embed: no `..`, symlinks, or nested module; each pattern must match; `all:` includes dot and underscore names | `src/embed/embed.go:68` to `:96` |
| Adoption payload is `git archive HEAD` under an allowlist containing `internal`; nested `.gitignore` files are not copied, the top-level one is merged line by line | `scripts/adopt.sh:228` to `:234`, `:246`, `:365`, `:373` to `:379` |
| Ownership: vendored contents and `internal/` in a root installation are the MetaSystem's; the Go-only rule carries the frontend exception | `internal/stateroot/owner.go:103`, `:142`; `../development/project-rules-local.md:10` |

## Contracts

### Source layout

```
internal/ui/web/                 Go package web
  embed.go web.go digest.go contenttype.go *_test.go testmain_test.go
  bundle/                        //go:embed all:bundle
    README.txt                   committed, Go-owned: the pattern matches even when dist/ is empty or gone
    bundle.json                  the manifest, written by the bundle script
    dist/                        Vite output, emptied on every build
      index.html assets/app.js assets/app.css assets/*.woff2 THIRD-PARTY-NOTICES.txt
  _app/                          the React source, the npm project root
    package.json package-lock.json .nvmrc index.html vite.config.ts tsconfig.json
    src/ public/ scripts/ node_modules/ (ignored)
```

The fence has two layers, because two repository tools honour none:

1. `_app` begins with an underscore, so `go build`, `go vet`, `go list`, staticcheck, and govulncheck over `./...` never enter it, and `gopackages` excludes it by the same rule. A nested `go.mod` would add a fake module and the `ignore` directive would edit `go.mod`; neither is used.
2. `gofmt` and the parallel ratchet read every `.go` or `_test.go` under the working tree, so `node_modules` must hold none. `package.json` has `postinstall: node scripts/prune-go.mjs`, which deletes every `*.go` beneath `node_modules`, which no JavaScript runtime needs; `npm ci` runs it, and the bundle script refuses while any remains.

`bundle/` is inside the package directory of the engine's own module, so the pattern crosses no module and needs no `..`. `Dist()` is `fs.Sub(bundle, "bundle/dist")`; `README.txt` and `bundle.json` are never served.

Git: `metasystem/.gitignore` gains one line, `/internal/ui/web/_app/node_modules/`, anchored so a root installation does not ignore the application's own `node_modules`, and in the top-level file because adoption drops nested ones. Nothing else needs ignoring: Vite and Vitest cache under `node_modules/.vite`, TypeScript emits nothing, `dist` is committed; no existing rule touches these paths (`git check-ignore`).

### What ships

The payload is committed files under an allowlist that includes `internal` wholesale, so `_app`, `bundle/`, and the Go files ship with no install-set change and `node_modules` never does. Source ships as well as the bundle because the payload ships engine source and rebuilds it (`adopt.sh:242` to `:245`), the staleness test keeps its meaning there, and an adopter should not receive minified code alone; the bundle ships because the adopter has no Node.

### The build

Node is pinned in `_app/.nvmrc` to `24.21.0`, the current LTS; `engines.node` repeats it and the bundle script refuses any other version. Installs are `npm ci`; `package.json` pins exact versions (`--save-exact`); the lockfile is committed.

Scripts: `dev` (`vite`), `typecheck` (`tsc --noEmit`), `test` (`vitest run`), `bundle` (`node scripts/bundle.mjs`), `check:modal` (`node scripts/modal-check.mjs <address>`), `postinstall`. `bundle` runs, in order: the Node check; the `.go` check; `typecheck`; `vite build`; `scripts/notices.mjs`; then writes `bundle/bundle.json`. It calls `node_modules/.bin` binaries, never `npx`.

`vite.config.ts`: `base: "/"`; `plugins: [react(), tailwindcss()]`; `build.outDir: "../bundle/dist"` with `emptyOutDir: true` (explicit, being outside the Vite root); `assetsInlineLimit: 0` (no `data:` URLs, so the policy allows none); `sourcemap: false`; `manifest: false` (`bundle.json` is the manifest); `modulePreload: { polyfill: false }`; `rolldownOptions.output` with `entryFileNames: "assets/[name].js"`, `chunkFileNames: "assets/[name].js"`, `assetFileNames: "assets/[name][extname]"` (Vite 8 bundles with Rolldown; `rollupOptions` is deprecated there); `server.host: "127.0.0.1"`, `port: 5173`, `strictPort: true`, and the proxy under Development mode.

`tsconfig.json`: `strict`, `noEmit`, `lib: ["ES2022", "DOM", "DOM.Iterable"]`, `moduleResolution: "bundler"`, `jsx: "react-jsx"`, `verbatimModuleSyntax` and `isolatedModules` (the transpiler is per-file), `include: ["src", "scripts", "vite.config.ts"]`.

Runtime dependencies, each bundled or able to be:

| Package | Licence | Why |
| --- | --- | --- |
| `react`, `react-dom` 19 | MIT | D1 |
| `@radix-ui/react-dialog` | MIT | The modal that exercises the nonce path; further D15 primitives arrive with the slices that need them |
| `get-nonce` | MIT | Already transitive under the Radix scroll lock; declared so `main.tsx` calls `setNonce` on that instance |
| `lucide-react` | ISC | D15's open icon set; the dialog's close icon |
| `@fontsource-variable/inter` | OFL-1.1 (registry field and the package's `LICENSE`; upstream `rsms/inter`) | The interface face |

Named and licence-checked now, installed by the first slice that uses them: `react-resizable-panels` (MIT), which injects a cursor style element and exports `setNonce` for it, and `@fontsource-variable/jetbrains-mono` (OFL-1.1). Only Inter's Latin and Latin-extended `wght` faces are bundled, through a hand-written `@font-face` block in `src/fonts.css` referencing `@fontsource-variable/inter/files/inter-latin-wght-normal.woff2` and `inter-latin-ext-wght-normal.woff2`: two font files, not fourteen.

Development dependencies, never bundled: `typescript` 7 (Apache-2.0; type-checking only; the 6.x line is the fallback if the native compiler lacks an option named here), `vite` 8 and `@vitejs/plugin-react` (MIT), `tailwindcss` 4 and `@tailwindcss/vite` (MIT; configured in CSS), `vitest` 5 (MIT), `@types/react`, `@types/react-dom` (MIT), `playwright` (Apache-2.0; the modal check only).

Deliberately not included: a router (`g1-s9` decides); a DOM test environment or component testing library; a linter or formatter; source maps; a web manifest or favicon; any CDN, analytics, or remote font; a Markdown renderer (D6).

### The embedding package

```go
package web
//go:embed all:bundle
var bundle embed.FS
const NoncePlaceholder = "__METASYSTEM_CSP_NONCE__"
func Dist() fs.FS                             // bundle/dist; Open fails with fs.ErrNotExist when absent
type FileDigest struct{ Path, SHA256 string } // "/" separators; hex
type Manifest struct {
    SchemaVersion int               `json:"schemaVersion"` // 1
    SourceDigest  string            `json:"sourceDigest"`  // "sha256:<hex>" over _app, rule below
    Source        []FileDigest      `json:"source"`        // the same walk, for diagnosis
    Files         []FileDigest      `json:"files"`         // every file under dist/
    Tools         map[string]string `json:"tools"`         // node, vite, typescript, tailwindcss
}
func ReadManifest() (Manifest, error)         // ErrNoManifest when bundle.json is absent
func SourceDigest(dir string) (string, []FileDigest, error)
func ContentType(name string) (string, bool)  // false outside the table
```

No timestamps: two builds of one tree give identical `dist/` and `bundle.json`, which the walkthrough checks.

**Digest rule**, implemented in `digest.go` and `_app/scripts/digest.mjs`. Walk `dir`. Skip any directory named `node_modules` at any depth and any entry whose name begins with `.` except the file `.nvmrc`. A symbolic link anywhere is an error: its target could change the build without changing the digest. Each remaining regular file contributes its path relative to `dir` with `/` separators, ASCII only (else an error, so both sorts agree), and the SHA-256 of its bytes after every `\r\n` becomes `\n`; modes and times are ignored. Lines `<hex>  <path>\n` are sorted by path bytewise and concatenated; the digest is `sha256:` plus the hex SHA-256 of that text. For the fixture `.nvmrc` = `24.21.0\n`, `a.txt` = `x\r\n`, `b/c.txt` = `y\n`, `.hidden` = `z`, `b/node_modules/q.txt` = `q`, the digest is `sha256:dffa552ce1a1d682cb238886211fa28db48c8f7482629d8b0f4d55b55fd8b520`; both implementations assert this literal.

**Content types**, a fixed table, because `mime.TypeByExtension` reads the host's mime files and differs between machines: `.html` `text/html; charset=utf-8`, `.js` `text/javascript; charset=utf-8`, `.css` `text/css; charset=utf-8`, `.json` `application/json`, `.txt` `text/plain; charset=utf-8`, `.svg` `image/svg+xml`, `.woff2` `font/woff2`, `.png` `image/png`.

### Serving

`httpd.New(info Info, bound net.Addr, bundle fs.FS) http.Handler`; `Info` gains `BundleDigest string`, filled by `cmd/metasystem/ui.go` from `web.ReadManifest().SourceDigest`, empty without a manifest. An unexported constructor also takes `nonce func() string` for tests. The `g1-s1` checks and headers run first on every request, unchanged. Then:

1. `/-/health`: as before plus `"bundleDigest"`. Any other `/-/` path, and `/api/` and below (`g1-s7` takes the prefix): 404, never the page.
2. Static: `path.Clean`, strip the leading `/`, require `fs.ValidPath`, refuse `index.html` and directories; a regular file is served with its table content type, or 404 when its extension is outside the table. `/index.html` is therefore the page under the next rule, never a redirect.
3. Page: `/`, or any remaining path whose `Accept` lists `text/html` explicitly and that does not begin with `/assets/`: `index.html` with the nonce substituted, 200, so a client-side route reloads without the server knowing the routes React owns. Everything else: 404. Navigations send `text/html`; scripts, styles, fonts, `fetch`, and `curl` do not, so a missing asset is an honest 404 and `/assets/anything` is never the page.

Absent bundle: at construction the handler reads `index.html`; when that fails, every page response is 503 with the text `MetaSystem interface: this executable was built without the interface bundle. Rebuild it from a checkout that contains internal/ui/web/bundle/dist, then run: metasystem ui restart`; static lookups are 404; health answers with an empty `bundleDigest`. This replaces `g1-s1`'s placeholder line.

Caching: `Cache-Control: no-store` stays on every response. Names are stable, so a rebuild changes bytes at the same URL; the nonce forbids reusing a page; refetching a few hundred kilobytes over loopback costs nothing. Revalidation waits for a measured need.

### Content security policy

Header `Content-Security-Policy` on every response. On a page response:

```
default-src 'none'; script-src 'self'; style-src 'self' 'nonce-<nonce>'; img-src 'self'; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'
```

On every other response the same string without ` 'nonce-<nonce>'`. No `unsafe-inline`, `data:`, host source, `report-uri` (gate 1 accepts no POST), or Trusted Types.

The nonce is 16 bytes from `crypto/rand`, which since Go 1.24 cannot fail, in `base64.RawStdEncoding` (22 characters, a valid CSP `base64-value`), generated once per page response for header and body. `_app/index.html` carries `<meta property="csp-nonce" nonce="__METASYSTEM_CSP_NONCE__">` and nothing else inline: no `<script>` without `src`, no `style` attribute, no `<style>`. At construction the handler splits the embedded `index.html` on `web.NoncePlaceholder`; per response it writes the parts around the nonce. `src/nonce.ts` finds `meta[property="csp-nonce"]` and returns its `nonce` IDL property, not `getAttribute`, because browsers hide the attribute after parsing; `main.tsx` calls `setNonce(nonce)` from `get-nonce` before the first render, so `react-style-singleton`, which the dialog reaches through `react-remove-scroll` and `react-remove-scroll-bar`, sets it on the `<style>` it appends. React `style` props write CSSOM properties, which the policy does not govern, so `style-src-attr` is not needed. Vite's `html.cspNonce` is not used: the meta tag is hand-written.

### Development mode

`npm run dev` starts Vite on `127.0.0.1:5173` with hot reload and proxies `/api` and `/-` to `http://127.0.0.1:7878`, overridable by `METASYSTEM_UI_PROXY_TARGET`. The proxy sets `changeOrigin: true`, so the Go server sees `Host: 127.0.0.1:7878`, and removes `Origin`; the browser's `Sec-Fetch-Site: same-origin` is forwarded and passes. The Go server sees what a command-line client sends, from loopback; no check is loosened or made configurable, and nothing development-only exists in Go. Vite serves the page without the policy and with the placeholder literal; the policy belongs to the embedded build and is checked there.

### Licence notices

`scripts/notices.mjs` reads `package-lock.json`, walks the runtime closure from the root's `dependencies` (following nested `node_modules` resolution, ignoring `dev` and `optional` entries), and for each package reads `name`, `version`, `license`, and its `LICENSE*` or `LICENCE*` file from `node_modules`, refusing when one has none. It writes `dist/THIRD-PARTY-NOTICES.txt`: a header, then per package the name, version, identifier, and text. Every package that can contribute code or assets is listed, even one tree-shaking drops: over-attribution, never under-attribution, and no bundler hooks. The file is committed, embedded, served at `/THIRD-PARTY-NOTICES.txt`, and linked from the page as "Open-source notices", a link `g1-s9` keeps. Build tools are not listed; compiling with them redistributes nothing. The engine's Go dependencies are not this slice's subject.

### Frontend tests

Vitest runs in the `node` environment, `vitest run`, development-time only, never part of the executable or of a Go test. Three tests: `scripts/digest.test.ts` (the fixture literal, from a temporary directory); `scripts/lockfile.test.ts` (`package-lock.json` has exactly one `get-nonce` entry, so `setNonce` reaches the instance Radix uses); `src/nonce.test.ts` (`readNonce` returns the element's `nonce` property, or `""`).

## Behaviour

**Page.** `src/App.tsx` renders the heading `MetaSystem interface`; the facts fetched from `/-/health`; `path: <location.pathname>`; a button `Open a dialog` opening a Radix `Dialog` with a sentence and a close button with a lucide icon; the notices link. Tailwind utilities and Inter style it minimally; the look is `g1-s18`'s. **Bundle.** `bundle.json` is written last, so a failed build leaves no manifest claiming a bundle it did not finish.

| Case | Required result |
| --- | --- |
| `.go` under `node_modules` after `npm ci --ignore-scripts`; Node differs from `.nvmrc` | Bundle script refuses, naming the file and `npm run postinstall`, or both versions |
| Source changed after the last bundle | `TestBundleIsCurrent` fails listing changed, added, and removed paths and says `run npm ci && npm run bundle in internal/ui/web/_app` |
| A `dist/` file not matching `bundle.json`, after a partial commit or a mixed merge | `TestBundleMatchesManifest` names the file whose digest differs, is missing, or is extra |
| `dist/` empty or absent | `go build` succeeds through `README.txt`; the server serves the 503 statement; `TestBundleMatchesManifest` fails (`bundle missing` when there is no manifest either) |
| `/assets/missing.js`, also from the address bar; `/api/goals` before `g1-s7`; `curl /nothing` (`Accept: */*`); `/assets/`; `/assets` | 404 |
| Dialog opened in the embedded build | Scroll lock applies, one `<style nonce>` in `head`, zero `securitypolicyviolation` events |

## Change boundary

New: `internal/ui/web/` (Go sources, tests, `testmain_test.go`, `bundle/README.txt`, `bundle/bundle.json`, `bundle/dist/**`, `_app/**` except `node_modules`).

Edited, and nothing else in them: `internal/ui/httpd/` sources for routes, policy, and constructor; `internal/ui/httpd/*_test.go` from `g1-s1`, only the cases asserting the placeholder at `/`, the constructor's arity, the 404 rule, and the exact health payload, named here because they are this stream's and assert what this slice changes; `cmd/metasystem/ui.go`, the `NewHandler` wiring and `Info.BundleDigest`; `metasystem/.gitignore`, one line; `docs/architecture.md`, one package-map row for `ui/web`.

Must not be touched: `testing-parallel-ratchet.json`, `testing.json` (it names no surface under `internal/ui`, which falls to `residual`), `go.mod`, `go.sum`, `internal/testenv`, `internal/testutil`, `internal/parallelratchet`, `internal/testselect`, `internal/testpolicy`, `internal/testexec`, `internal/hostload`, `internal/proofrun`, `internal/gopackages`, `cmd/metasystem/test*.go`, `cmd/metasystem/audit.go`, `cmd/metasystem/main.go`, `internal/ui/lifecycle`, `internal/config`, `scripts/**`, `metasystem.conf`, and any other existing `*_test.go` or `testmain_test.go`.

No new Go module dependency: `embed`, `io/fs`, `crypto/rand`, `crypto/sha256`, `encoding/json`, `net/http`, `path` suffice.

Conventions, repeated because the builder sees only this document: every new package has `testmain_test.go` with `func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }`; tests never call `os.Setenv`; every top-level test and independent subtest calls `t.Parallel()`, the ratchet allowing an unlisted package zero serial tests; no sleeps, polling, or fixed ports (`httptest`); temporary trees under `t.TempDir()`; `testutil.Expect` and `Require`; the nonce generator injected; `cmd/metasystem/ui.go` wires and prints only. The Go tests read `_app` and `bundle` relative to the package directory, `go test`'s working directory, and need no Node. The bundle is rebuilt as the last commit of the branch after rebasing (D7).

## Verification

Go tests, package `web`: `TestSourceDigestFixture` (the literal, from `t.TempDir()`, plus symlink and non-ASCII errors); `TestBundleMatchesManifest`; `TestBundleIsCurrent` (skips, with a message, when `_app` is absent); `TestDistIsServable` (every extension in the table); `TestIndexIsStrict` (exactly one placeholder; the inline rules under Content security policy; no `http:` or `https:` in `src` or `href`); `TestReadManifestAbsent` on `fstest.MapFS`. Package `httpd`, on `fstest.MapFS` bundles with the fake nonce: each row above; the policy on page, static, health, 404, 403, and 405 responses; the five `g1-s1` headers still on all; body nonce equals header nonce and differs between responses; `bundleDigest` in health; the absent-bundle 503.

Obligations the code critique checks by name:

- **O1, fence.** `gofmt -l internal cmd`, `go vet ./...`, and `bin/metasystem audit parallel-ratchet` pass with `_app/node_modules` installed; `postinstall` names the prune script.
- **O2, no inline, nothing external.** `TestIndexIsStrict` as specified, passing on the committed bundle.
- **O3, nonce end to end.** Fresh per response, in header and body; `main.tsx` calls `setNonce` before rendering.
- **O4, honest 404.** No path under `/assets/`, `/api/`, or `/-/` receives the page, nor any request without `text/html` in `Accept`.
- **O5, staleness.** Both digest implementations assert the fixture literal; the exclusions are exactly those written here.
- **O6, absent bundle.** `go build ./cmd/metasystem` succeeds with `bundle/dist` removed; the server answers the 503 statement.
- **O7, notices.** Every package in the lockfile's runtime closure, with its licence text; the page links to the file.
- **O8, no loosening.** The `g1-s1` checks and headers are as designed; the proxy exists only in `vite.config.ts`.

Commands, from `metasystem/`: `go build ./...`; `go vet ./internal/ui/... ./cmd/metasystem/`; `go test ./internal/ui/...`; `gofmt -l internal cmd`; `bin/metasystem audit parallel-ratchet` without `--update`, noted if it does not work. From `internal/ui/web/_app/`: `npm ci`, `npm run typecheck`, `npm test`, `npm run bundle` twice with `git status --porcelain` clean between.

Walkthrough, by Claude after the code critique: build `bin/metasystem`; `ui start`; open the address in a real browser and see the page with the health facts; open the dialog and confirm in devtools a `<style nonce>` in `head`, no policy violation, and every request going to the server's origin; reload at `/some/deep/path`; open `/THIRD-PARTY-NOTICES.txt`; `curl -i /assets/missing.js` gives 404 and `curl -i /` gives 200 with a policy nonce that also appears in the body; `npm run check:modal -- http://127.0.0.1:7878` passes; `npm run dev`, open `127.0.0.1:5173`, see the same page through the proxy and its requests in `server.log`; remove `bundle/dist`, rebuild, `ui restart`, see the 503 statement, restore, rebuild, `ui restart`. The modal check is run by Claude, never by a Go test or the gate.

## Open questions

One, settled by the human's go unless raised. **Node line.** Recommended `24.21.0`, the LTS, supported to April 2028. The alternative, the current line `26.9.0` (LTS from October 2026), matches the development machine's Homebrew Node, `26.8.2` today. Either way the exact pin is enforced, so the builder installs it through a version manager.
