# g1-s8 Frontend toolchain: Sol's design review

Reviewer: Codex on `gpt-5.6-sol`, the design reviewer under D12 and D24, read-only, 2026-09-21. Subject: the [slice design](g1-s8-frontend-toolchain-design.md) at commit `c38f82cb4`. The report follows verbatim.

The planner verified E1's load-bearing chain in source before accepting it: `enginePaths` includes `internal/**` (`internal/behaviorsurface/policy.v2.json:3`), and the digest walk skips only `.git` plus a prefix filter (`policy.go:347` to `:374`), so every file beneath `internal/` is hashed. It also confirmed the property that makes a fix possible: a symbolic link is recorded as one entry holding its target string (`policy.go:396` to `:402`), and `filepath.WalkDir` does not descend through it.

## 1. Verdict build after the listed changes Single riskiest issue: `_app/node_modules` remai...
[codex] Turn completion inferred after the main thread finished and subagent work drained.
## 1. Verdict

build after the listed changes

Single riskiest issue: `_app/node_modules` remains part of the live ENGINE behavior surface and several proof-input digests, so merely installing frontend dependencies can drift or refuse unrelated MetaSystem work.

## 2. Findings

E1; MATERIAL; Source layout / Go fence; the underscore directory and `.go` pruning do not fence `node_modules` from all repository machinery. The stated `gofmt` and parallel-ratchet premise is correct: `gofmt` recursively visits every directory, and the ratchet skips only `.git`, `artifacts`, and `vendor` (`metasystem/scripts/agents/go-gate.sh:515-540`; `metasystem/internal/parallelratchet/parallel.go:109-164`; `metasystem/cmd/metasystem/audit.go:130-165`). But ENGINE includes all of `internal/**`, and its live digest walks everything except `.git` and hashes every included regular file (`metasystem/internal/behaviorsurface/policy.v2.json:3-9`; `metasystem/internal/behaviorsurface/policy.go:347-425`); that digest governs assumption matching and active admission (`metasystem/internal/dispatch/governed.go:21-50`, `:192-195`). Proof inputs have the same problem: several groups declare `metasystem/internal/**` (`metasystem/testing.json:50-52`, `:75-79`), path expansion walks without Git-ignore or underscore exclusions (`metasystem/internal/pathpattern/pattern.go:111-176`), and proof identity hashes every match (`metasystem/internal/proofrun/test_build.go:1028-1050`, `:1780-1808`). The stop-surface audit is another prune-dependent walker over every `_test.go` outside only `.git` (`metasystem/internal/audit/stopsurface.go:289-323`, `:351-358`). A true fresh clone is initially safe because ignored `node_modules` is absent, but a restored dependency cache or `npm ci --ignore-scripts` exposes Go files before the bundle refusal runs, while an ordinary `npm ci` still leaves the entire non-Go dependency tree in ENGINE and proof digests. The fix must either place installed dependencies outside these live projections or explicitly exclude `_app/node_modules` from every relevant walker and expand the change boundary and verification accordingly.

E2; MATERIAL; The build / dependency table; the document says `package.json` pins exact versions, but supplies only majors or no version at all and even leaves TypeScript as “7” with a 6.x fallback (`plans/user-interface/g1-s8-frontend-toolchain-design.md:65-85`). The builder must choose the Radix, React, `get-nonce`, Vite, TypeScript, panel-library, and other exact APIs, so the lock graph, nonce chain, build output, and licence closure are not determined by the design. List every direct dependency and development dependency at an exact version and remove the TypeScript fallback choice.

E3; MATERIAL; The build / bundle manifest; “write `bundle.json` last” does not ensure a failed build leaves no manifest. The manifest is outside `dist/`, while Vite empties `dist/`; the stated sequence never removes the old manifest before Vite or notices run (`plans/user-interface/g1-s8-frontend-toolchain-design.md:40-43`, `:67-69`, `:154`). A Vite or notices failure can therefore leave an old, apparently valid manifest beside cleared or partially replaced assets, and a plain Go build can embed that mixed state. Remove or invalidate the manifest before the first destructive build step, or stage `dist` and the manifest together and publish them atomically.

E4; MATERIAL; Content security policy / Behaviour; the current Radix nonce chain is real, but the design does not require the component that activates it. Current `@radix-ui/react-dialog` mounts `RemoveScroll` inside `DialogOverlay`, not merely `DialogContent`; the application contract only says to render a dialog (`plans/user-interface/g1-s8-frontend-toolchain-design.md:138`, `:152-154`, `:163`). A builder can validly render Trigger, Portal, Content, and Close without Overlay, producing no scroll-lock style and failing the named nonce proof. Specify that this slice renders `Dialog.Overlay` and pin the verified package versions. [Radix Dialog source](https://github.com/radix-ui/primitives/blob/main/packages/react/dialog/src/dialog.tsx), [Radix package dependencies](https://github.com/radix-ui/primitives/blob/main/packages/react/dialog/package.json).

E5; MATERIAL; Serving / SPA fallback; the rule excludes paths beginning `/assets/`, but the required outcomes also say exact `/assets` is 404 (`plans/user-interface/g1-s8-frontend-toolchain-design.md:120-122`, `:162`). A browser navigation to `/assets` has `Accept: text/html`, does not begin `/assets/`, and therefore receives the page under the written routing rule. “Lists `text/html` explicitly” also leaves token parsing, parameters, case, and `q=0` undefined. Reserve exact `/assets` as well as `/assets/…`, and define an accepted media range as a parsed `text/html` range with positive quality rather than substring matching.

E6; MATERIAL; Staleness digest / deterministic manifest; the recorded fixture digest is reproducible, and ASCII paths plus bytewise sorting make the source digest OS-independent, but CRLF normalization applies to every regular file, including binary assets, so two build-relevant byte sequences can hash identically; moreover `Manifest.Files` and the JSON serialization order are not specified even though cross-machine-identical `bundle.json` is required (`plans/user-interface/g1-s8-frontend-toolchain-design.md:98-112`). A builder can produce platform-dependent manifest ordering or miss a source change consisting of binary `0d0a` bytes becoming `0a`. Define which text extensions receive newline normalization, hash other files byte-for-byte, and prescribe canonical ordering and JSON rendering for `Source`, `Files`, and `Tools`.

E7; NOT MATERIAL; Changes to g1-s1; replacing the placeholder, adding the health field, passing `fs.FS`, and editing the exact affected tests are necessary and safely scoped: the current handler owns the fixed checks before routing and the five headers (`metasystem/internal/ui/httpd/httpd.go:22-59`, `:146-152`), while the existing tests hard-code the old constructor, payload, and root result (`metasystem/internal/ui/httpd/httpd_test.go:61-69`, `:109-159`). The implementation plan permits existing-file touches that a slice design names with its reason (`plans/user-interface-implementation-plan.md:96-102`), and this design names only those assertions (`plans/user-interface/g1-s8-frontend-toolchain-design.md:165-171`).

E8; NOT MATERIAL; Embed / absent bundle; `all:bundle` matches committed `README.txt`, so compilation does not depend on a nonempty `dist`; Go ignores empty directories but accepts the nonempty bundle tree, and `fs.Sub` does not require its target to exist until opened (`plans/user-interface/g1-s8-frontend-toolchain-design.md:40-55`, `:89-107`, `:161`; `/opt/homebrew/Cellar/go/1.27.1/libexec/src/embed/embed.go:64-99`; `/opt/homebrew/Cellar/go/1.27.1/libexec/src/io/fs/sub.go:20-47`, `:90-96`). The specified constructor-time index read is sufficient to select the 503 path and clear the health digest (`plans/user-interface/g1-s8-frontend-toolchain-design.md:118-124`).

E9; NOT MATERIAL; CSP and caching; the nonce is 128 random bits, Raw Base64 uses CSP-valid characters, one nonce is shared by the response header and template, and `no-store` remains on every response (`plans/user-interface/g1-s8-frontend-toolchain-design.md:126-138`; `metasystem/internal/ui/httpd/httpd.go:146-152`). Current upstream confirms `get-nonce`’s module-local setter/getter and `react-style-singleton`’s nonce-bearing `<style>` insertion; direct `element.style.property` writes are not blocked by `style-src`. The exact-one-placeholder test protects the trusted committed template; the nonce alphabet itself cannot break out of the quoted attribute. [get-nonce source](https://raw.githubusercontent.com/theKashey/get-nonce/master/src/index.ts), [react-style-singleton source](https://raw.githubusercontent.com/theKashey/react-style-singleton/master/src/singleton.ts), [CSP specification](https://www.w3.org/TR/CSP/), [MDN style-src behavior](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Security-Policy/style-src).

E10; NOT MATERIAL; Other repository walkers; current package selection materializes committed trees and excludes nested modules and underscore packages from runnable inventory (`metasystem/internal/gopackages/select.go:42-99`, `:273-338`, `:502-515`), test-environment scans skip underscore directories (`metasystem/internal/testenv/protection_test.go:406-470`), coverage consumes Go output rather than scanning arbitrary source (`metasystem/cmd/metasystem/audit.go:20-72`), landing drift consumes ordinary Git status, which omits ignored paths (`metasystem/internal/landing/drift.go:21-67`; `metasystem/internal/gittree/snapshotscope.go:81-105`), and census enumerates processes rather than source (`metasystem/internal/census/run.go:124-178`). `internal/testselect` would enter `_app`, but review search found no production caller; that is an inference from the source search, not a runtime proof.

E11; NOT MATERIAL; Adoption / source absence; current adoption archives tracked `HEAD` and explicitly ships `internal` wholesale, so adopted projects receive both `_app` and the bundle, not only the bundle (`metasystem/scripts/adopt.sh:219-246`, `:342-379`). If another packaging path omits `_app`, the design deliberately skips `TestBundleIsCurrent` while retaining manifest-to-dist integrity checking (`plans/user-interface/g1-s8-frontend-toolchain-design.md:175-179`); that is coherent because there is then no source tree to compare.

E12; NOT MATERIAL; Future `react-resizable-panels`; the design’s claim is false for current upstream 4.12.4: its public index exports no `setNonce`, and cursor styling now uses `CSSStyleSheet` plus `adoptedStyleSheets`, not a nonce-bearing `<style>`. Version 2.1 added `setNonce`, but the API changed later. This does not change this slice because installation is explicitly deferred, but the later slice must re-evaluate its CSP integration. [Current package](https://raw.githubusercontent.com/bvaughn/react-resizable-panels/main/package.json), [current exports](https://raw.githubusercontent.com/bvaughn/react-resizable-panels/main/lib/index.ts), [current cursor implementation](https://raw.githubusercontent.com/bvaughn/react-resizable-panels/main/lib/global/cursor/updateCursorStyle.ts), [changelog](https://github.com/bvaughn/react-resizable-panels/blob/main/CHANGELOG.md).

## 3. The author's unverified claims

Go fence — Verified: neither nested `go.mod` nor `ignore` protects direct `gofmt` or the parallel/stop-surface filesystem walks; `_app` does protect ordinary `./...`, but E1 shows the claimed fence is incomplete.

Repository-wide walkers — Partly disproved: behavior-surface and proof-input walkers include all of `_app/node_modules`; package selection, test-environment inventory, coverage, landing drift, census, and adoption do not create the same problem for the reasons and source locations in E1, E10, and E11.

Radix → scroll lock → singleton → nonce — Verified for current upstream: Dialog Overlay uses `react-remove-scroll`; 2.7.2 depends on `react-remove-scroll-bar` and `react-style-singleton`; the singleton reads `get-nonce` and writes the nonce attribute. What remains unproved is the actual one-instance resolution in the future lockfile and produced bundle because exact versions are absent.

`react-resizable-panels` nonce behavior — Disproved for current 4.12.4: it no longer exports `setNonce` and uses constructable stylesheets; the old claim describes the 2.x API.

Vite 8 `rolldownOptions` — Verified in Vite’s current official documentation; `build.rollupOptions` is the deprecated alias. [Vite 8 build options](https://v8.vite.dev/config/build-options).

Staleness fixture — Verified independently: the stated fixture reproduces `sha256:dffa552ce1a1d682cb238886211fa28db48c8f7482629d8b0f4d55b55fd8b520`; cross-platform ordering is defined, subject to E6’s binary-newline and manifest-canonicalization defects.

Empty `dist` embed — Verified from Go’s embed and `fs.Sub` implementations: `README.txt` keeps the pattern compilable and missing `dist/index.html` can select the 503 path.

## 4. Node line

Pin 24.21.0. It is the latest 24.x Active LTS release and is supported through April 2028, whereas 26.9.0 is still Current until its scheduled LTS transition on 28 October 2026; matching one developer machine is not worth choosing a pre-LTS line. [Node release status](https://nodejs.org/en/about/previous-releases), [official release schedule](https://github.com/nodejs/release/blob/main/README.md), [24.21.0 archive](https://nodejs.org/en/download/archive/v24.21.0).

## 5. Not checked

I did not run npm, install dependencies, generate a lockfile or bundle, execute repository scripts or the MetaSystem binary, start or contact the server, or perform the browser modal walkthrough. Consequently I could not verify actual Vite output reproducibility, the eventual lockfile’s single `get-nonce` instance, generated licence notices, committed bundle integrity, or live CSP behavior; upstream conclusions refer to current source and must be repeated against the exact versions added after E2 is resolved.

---

# Round 2, 2026-09-21

On revision 2 at commit `9b8c05e3f`. Four material findings; E2 to E6 and E12 confirmed corrected, E1 not yet. The planner verified the three concrete ones in source: `scripts/agents/brain-fixtures.sh:338` walks `internal` with `find` and compares against an exact allowlist, and `section/brain-fixtures` is an acceptance surface in `testing.json`; `git ls-files --cached` lists a tracked file that has been deleted only in the working tree; and npm's `info` level sits below `low`, so `--audit-level=low` permits an informational advisory.

## 1. Verdict build after the listed changes sound after the listed changes ## 2. Status of E...
[codex] Turn completion inferred after the main thread finished and subagent work drained.
## 1. Verdict

build after the listed changes  
sound after the listed changes

## 2. Status of E1 to E6 and E12

E1: NOT corrected — the final verification still invokes recursive `gofmt`, another verdict-bearing walker is omitted, and the replacement gofmt pipeline refuses valid unstaged deletions.

E2: corrected

E3: corrected

E4: corrected

E5: corrected

E6: corrected

E12: corrected

## 3. Findings

G1; MATERIAL; The supply-chain gate; `npm audit --audit-level=low` does not implement “refuse on any advisory”: npm defines `info` below `low`, and `audit-level` is the minimum severity producing a nonzero exit, so an informational advisory permits the bundle (`plans/user-interface/g1-s8-frontend-toolchain-design.md:60`, `:160`, `:190`); evidence: [npm audit documentation](https://docs.npmjs.com/cli/v11/commands/npm-audit/); what would be built wrong: a bundle can pass while npm reports a known informational vulnerability, contradicting the human’s requirement; direction: use the threshold that includes `info`, or inspect structured audit output and refuse whenever its advisory count is nonzero.

G2; MATERIAL; E1 / Verification; revision 2 replaces the gate’s recursive gofmt invocation, but its final command list still instructs the builder to run `gofmt -l internal cmd` (`plans/user-interface/g1-s8-frontend-toolchain-design.md:50`, `:193`); Go’s formatter recursively enters every directory and only filters non-Go or dot-prefixed filenames (`/opt/homebrew/Cellar/go/1.27.1/libexec/src/cmd/gofmt/gofmt.go:93`, `:419`); what would be built wrong: verification after dependencies exist again exposes `node_modules` to precisely the traversal the exclusion slice is meant to remove; direction: make the verification command use the new Git-fed boundary or invoke the gate that owns it.

G3; MATERIAL; E1 / Other walkers; `scripts/agents/brain-fixtures.sh` performs `find cmd/metasystem internal`, greps every non-test `.go` file, and compares the result with an exact allowlist, without pruning `node_modules` (`metasystem/scripts/agents/brain-fixtures.sh:338`, `:382`); the section is an actual acceptance/cadence surface (`metasystem/testing.json:102`, `:128`); what would be built wrong: dependency Go source containing one of the searched expressions changes or refuses the fixture verdict, contradicting the claim that such files are invisible to every tool (`plans/user-interface/g1-s8-frontend-toolchain-design.md:38`); direction: add a name-based `node_modules` prune to this scan, with a fixture, and include that script in the exclusion slice’s change boundary.

G4; MATERIAL; E1 / gofmt; `git ls-files --cached` lists every index entry, including a tracked `.go` file deleted only from the working tree, so the proposed pipeline sends a nonexistent path to gofmt (`plans/user-interface/g1-s8-frontend-toolchain-design.md:50`); Git documents `--cached` as all tracked files and unstaged deletion as a separate working-tree state ([git-ls-files documentation](https://git-scm.com/docs/git-ls-files)); read-only probing confirmed gofmt exits 2 for a missing explicit path, and the gate treats a nonzero gofmt status as a refusal (`metasystem/scripts/agents/go-gate.sh:519`, `:526`); what would be built wrong: an ordinary unstaged Go-file deletion cannot pass the gate although the old directory walk omitted the absent file; direction: preserve NUL-safe enumeration while filtering cached entries that are no longer extant regular files.

G5; NOT MATERIAL; E1 / Policy membership and neutrality; the proposed pattern is accepted by the exact-or-`/**` grammar, matches its base and descendants, and `NormalizePath` removes the installation prefix (`metasystem/internal/behaviorsurface/policy.go:147`, `:183`, `:215`); none of the existing non-repository entries newly changes the current ENGINE digest, while the two entries inside PAYLOAD’s `docs/**` root are absent from this checkout (`metasystem/internal/behaviorsurface/policy.v2.json:3`, `:11`, `:76`); current callers all consume `Includes`, and no caller dependency on ENGINE ignoring this list was found (`metasystem/internal/behaviorsurface/policy.go:278`, `:347`, `:442`, `:565`); what would be built wrong: nothing; direction: none.

G6; NOT MATERIAL; E1 / Remaining walkers; two further walks meet the installed tree: the metasystem audit’s repository-root instruction inventory (`metasystem/internal/audit/metasystem.go:129`, `:393`) and working-unit import discovery (`metasystem/internal/landing/batch/unitgate.go:87`, `:300`); the former only adds matching instruction filenames to a report, and the latter filters `.go` files against Git’s allowed set before parsing (`metasystem/internal/landing/batch/unitgate.go:102`, `:322`), so no current semantic change was established; `testselect` also enters underscore directories but remains explicitly acknowledged and has no production caller found by source search (`metasystem/internal/testselect/select.go:158`, `:194`); what would be built wrong: nothing demonstrated; direction: optional name-based pruning would avoid needless traversal but must not block this round.

G7; NOT MATERIAL; E1 / gofmt mechanics; on the current checkout, the old and proposed commands cover the same 1,601 files; generally the old command covers non-dot `.go` files under `internal` and `cmd`, including ignored files, while the new one covers tracked and non-ignored untracked `.go` files anywhere beneath `metasystem/`, including dot-named files and any future Go files outside those two directories; `-z`/`-0` preserves spaces and non-ASCII names, this Darwin platform accepts `xargs -r`, empty input starts no gofmt process, and tracked unformatted files still produce output that makes the gate red (`metasystem/scripts/agents/go-gate.sh:525`); what would be built wrong beyond G4: nothing in the current tree; direction: none beyond correcting G4.

G8; NOT MATERIAL; E3 through E6 and E12; the manifest is removed before typecheck/Vite/notices and published last (`plans/user-interface/g1-s8-frontend-toolchain-design.md:62`); `Dialog.Overlay` is mandatory (`:156`) and upstream 1.1.23 mounts `RemoveScroll` there ([Radix source](https://github.com/radix-ui/primitives/blob/main/packages/react/dialog/src/dialog.tsx)); reserved paths and parsed positive-quality `text/html` are explicit (`:124`–`:126`); text-only newline normalization and canonical manifest ordering are explicit (`:114`–`:116`); independent recomputation reproduced `sha256:f8c8104462398072962c697d4b860b1e76da903a0fa41933ffc541bcbb3e1399`, including the stated raw `d.bin` digest; E12 now correctly defers and requires browser proof (`:90`); what would be built wrong: nothing; direction: none.

## 4. The supply-chain gate

The exact direct pins, committed lockfile, `save-exact=true`, Node check, and `npm ci --ignore-scripts` are coherent. Explicit `npm run` commands still execute with `ignore-scripts=true`; only lifecycle pre/post scripts are suppressed, as npm documents ([npm configuration](https://docs.npmjs.com/cli/v11/using-npm/config/#ignore-scripts)). If a pinned transitive dependency needs an install-time build, the subsequent typecheck, tests, and bundle are intended to refuse; this proves operability only on the platform actually exercised.

The scroll-lock constraints converge on `react-style-singleton` 2.2.3 and `get-nonce ^1.0.0`; direct `get-nonce` 1.0.1 satisfies that range, so one instance is resolvable. The specified lockfile test correctly refuses duplication. Upstream confirms Radix 1.1.23 uses `react-remove-scroll ^2.7.2`, which in turn resolves the stated scroll-lock packages ([Radix manifest](https://github.com/radix-ui/primitives/blob/main/packages/react/dialog/package.json), [react-remove-scroll](https://www.npmjs.com/package/react-remove-scroll), [react-style-singleton](https://www.npmjs.com/package/react-style-singleton), [get-nonce](https://www.npmjs.com/package/get-nonce)).

A transitive advisory with no fix blocks every frontend rebuild until the dependency graph or advisory changes. That is operationally harsh but consistent with the human’s fail-closed requirement; there is deliberately no bypass. Existing committed Go builds remain possible because npm is not part of the deployed or Go-build runtime.

As written, however, the gate still permits `info` advisories. After G1 is corrected, it delivers “no vulnerabilities” only in the practical sense of “no vulnerabilities known to the configured npm registry at audit time,” not an absolute absence of unknown vulnerabilities.

## 5. Not checked

I did not run npm, install the dependency tree, generate or inspect the eventual lockfile, run an audit, execute repository scripts or the MetaSystem binary, or contact the interface server. Therefore I could not verify the current advisory result, every transitive package’s lifecycle scripts, the actual one-instance `get-nonce` layout, native optional-package loading on each platform, generated notices, bundle reproducibility, or live CSP behavior.

---

# Round 3, 2026-09-21

On revision 3 at commit `14a29e347`. Two material findings. G2, G3 and G4 confirmed corrected; G1 not, because the gate can be turned off by ambient configuration. The planner verified H2 in source: `scripts/agents/path-class-fixtures.sh` greps the behaviour roots of `scripts/agents/path-classes.txt`, which include `install:internal/`, and exits 1 on a match.

This is the second sweep of the scripts to claim completeness and be wrong. The round-4 brief therefore stops relying on a grep to prove the exclusion complete and adds an empirical guard instead.

## 1. Verdict build after the listed changes another round needed because G1 remains bypassab...
[codex] Turn completion inferred after the main thread finished and subagent work drained.
## 1. Verdict

build after the listed changes  
another round needed because G1 remains bypassable and the scripts sweep missed a verdict-bearing traversal.

## 2. Status of G1 to G4

G1: NOT corrected — the JSON fields are correct and an `info` advisory is refused; `total` and object-key checks are redundant for current npm output but safely fail closed. However, ambient `offline=true` produces a clean zero-vulnerability report without an audit, while `--include=dev` does not override ambient omission of optional or peer dependencies (`plans/user-interface/g1-s8-frontend-toolchain-design.md:73`, `:75`, `:205`).

G2: corrected — verification names the Git-fed pipeline or its owning gate and explicitly forbids `gofmt -l internal cmd` (`plans/user-interface/g1-s8-frontend-toolchain-design.md:208`).

G3: corrected — reasoning from documented [`find` precedence](https://www.gnu.org/software/findutils/manual/html_node/find_html/Combining-Primaries-With-Operators.html), the expression groups as `(directory && node_modules && prune) || (file && name && !test && exec)`; `-exec` belongs only to the file branch. Without the prune, both planted files match and the one-line expectation fails. The script is in the slice and must-not-touch lists (`plans/user-interface/g1-s8-frontend-toolchain-design.md:45`, `:51`, `:187`), and its change selects `section/brain-fixtures` without a `testing.json` edit (`metasystem/testing.json:17`, `:102`).

G4: corrected — by [Bash pipeline and conditional semantics](https://www.gnu.org/software/bash/manual/bash.html), corroborated by read-only probes, a false `[[ -f ]]` with no `else` leaves the `if` and loop successful; empty input runs no `gofmt`; deleted working-tree entries are skipped; NUL-delimited spaces and non-ASCII names survive; unformatted output still reaches the refusal at `metasystem/scripts/agents/go-gate.sh:528`; and `pipefail` propagates a failing `git ls-files` into `gofmt_rc` (`plans/user-interface/g1-s8-frontend-toolchain-design.md:58`, `:63`).

## 3. Findings

H1; MATERIAL; G1 / Audit; ambient npm configuration can turn the mandatory audit into no audit or a partial audit. The command passes npm’s flattened configuration into Arborist and overrides only `audit:true` ([npm audit command](https://github.com/npm/cli/blob/v11.19.1/lib/commands/audit.js#L57-L95)); Arborist returns no report when `offline===true`, leaves its vulnerability map empty, and serializes that as `total: 0` plus `{}` ([AuditReport JSON and offline handling](https://github.com/npm/cli/blob/v11.19.1/workspaces/arborist/lib/audit-report.js#L25-L99), [audit selection](https://github.com/npm/cli/blob/v11.19.1/workspaces/arborist/lib/audit-report.js#L294-L341)). Inference from those sources: `NPM_CONFIG_OFFLINE=true`, `--offline`, or user/global `offline=true` satisfies all four design checks without contacting a registry; npm documents those configuration sources and offline mode ([npm configuration](https://docs.npmjs.com/cli/v11/using-npm/config/)). Separately, `--include=dev` removes only `dev` from the effective omit set; configured `omit=optional` or `omit=peer` survives and Arborist excludes those nodes ([omit construction](https://github.com/npm/cli/blob/v11.19.1/workspaces/config/lib/definitions/definitions.js#L21-L52), [include definition](https://github.com/npm/cli/blob/v11.19.1/workspaces/config/lib/definitions/definitions.js#L967-L980)). What would be built wrong: a frontend bundle could pass after no registry audit, or after auditing less than the intended dependency tree, contradicting “no bypass of any kind” (`plans/user-interface/g1-s8-frontend-toolchain-design.md:75`, `:205`). Direction: force online mode and explicitly neutralize all relevant omit types at the audit invocation, with fixtures proving ambient offline and omit configuration cannot make the gate green.

H2; MATERIAL; Exclusion slice / scripts sweep; `path-class-fixtures.sh` is another verdict-bearing traversal of `internal/`, so the claim that `brain-fixtures.sh` is the only script doing this is false. `TestDeletedListsHaveNoReader` derives behavior roots from the manifest and recursively greps them, exiting 1 on a match (`metasystem/scripts/agents/path-class-fixtures.sh:87`, `:134`); those roots include `cmd/` and `internal/` (`metasystem/scripts/agents/path-classes.txt:15`, `:18`), and the test is invoked unconditionally (`metasystem/scripts/agents/path-class-fixtures.sh:368`, `:374`). It is a real validation section (`metasystem/scripts/validate-metasystem.sh:1185`, `:1191`) and is selected for changes to that script (`metasystem/testing.json:13`, `:108`). A text file under `_app/node_modules` containing one of its three forbidden tokens changes the verdict. What would be built wrong: the exclusion slice would still leave installed dependency content able to refuse validation, contradicting the stated invisibility to every verdict (`plans/user-interface/g1-s8-frontend-toolchain-design.md:39`, `:45`, `:53`). Direction: add `path-class-fixtures.sh` to the exclusion slice and exclude `node_modules` from this recursive grep with a failing-without-the-exclusion fixture; no `testing.json` edit is needed.

H3; NOT MATERIAL; G4 / xargs status; “123 under POSIX” is incorrect. POSIX specifies only a nonzero value in 1–125 when an invoked utility fails ([POSIX `xargs`](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/xargs.html)); GNU and several BSD implementations choose 123, while macOS documents 1 for other errors ([GNU `xargs`](https://man7.org/linux/man-pages/man1/xargs.1.html), [macOS `xargs`](https://man.freebsd.org/cgi/man.cgi?manpath=macOS+10.15.0&query=xargs&sektion=1)). What would be built wrong: nothing, because the gate distinguishes only zero from nonzero (`metasystem/scripts/agents/go-gate.sh:526`, `:529`). Direction: state portable “nonzero”; describe 123 only as an implementation-specific value.

H4; NOT MATERIAL; Known traversals; the four individually recorded traversals are correctly characterized, notwithstanding H2’s incomplete sweep. Instruction inventory adds only `Report` entries while the command’s verdict depends on `Violations` (`metasystem/internal/audit/metasystem.go:129`, `:134`; `metasystem/cmd/metasystem/audit.go:92`, `:101`). Working-unit discovery walks broadly but parses only Git-enumerated files (`metasystem/internal/landing/batch/unitgate.go:102`, `:115`; `:300`, `:354`). `testselect` enters underscore directories (`metasystem/internal/testselect/select.go:158`, `:204`), while repository-wide source search found no production caller and only its test literal (`metasystem/internal/testselect/select_test.go:268`). `fixture-budget.sh` finds the first newer Go file, prints a warning, and returns successfully (`metasystem/scripts/agents/fixture-budget.sh:182`, `:197`). What would be built wrong: nothing from those four traversals. Direction: none beyond H2.

## 4. The author's unverified claims

`xargs` exit 123 — corrected: GNU and several BSDs use 123, but POSIX guarantees only 1–125 and macOS documents 1.

`--include=dev` — corrected: it prevents `dev` from being omitted, including under `NODE_ENV=production`; it does not clear configured `optional` or `peer` omissions.

npm's error stream — corrected: the normal JSON report and buffered JSON endpoint-error object go to stdout, while warning/log text goes to stderr ([audit error handling](https://github.com/npm/cli/blob/v11.19.1/lib/utils/audit-error.js#L17-L34), [display routing](https://github.com/npm/cli/blob/v11.19.1/lib/utils/display.js#L268-L305)); “prints whatever npm wrote” therefore requires preserving both streams.

npm's default `audit-level` — corrected: the configuration default is `null`, but `npm-audit-report` converts that to an effective `low` threshold ([npm documentation](https://docs.npmjs.com/cli/v11/commands/npm-audit/), [reporter default](https://github.com/npm/npm-audit-report/blob/v7.0.0/lib/index.js#L18-L20)); an `info` advisory exits 0 but is caught by the proposed JSON-content checks.

scripts sweep — corrected: false; `path-class-fixtures.sh` is the additional verdict-bearing recursive traversal described in H2.

## 5. Not checked

No npm or npx command, install, audit execution, repository script, MetaSystem binary, or server was run. The eventual dependency tree, live advisory result, generated lockfile and bundle remain unchecked. `plans/user-interface/g1-s9-application-shell-design.md` and `metasystem/metasystem.conf.local` were not read.

---

# Round 4, 2026-09-21

On revision 4 at commit `50a54027b`. H2 and H3 resolved. Four material findings, all judged blocking by the reviewer.

The planner's ruling, made here rather than passed to another round: K2, K3 and K4 are accepted as written. K3 is the most serious of the four, because a guard that passes when both runs are equally broken proves nothing.

K1 is accepted in part. Pinning the registry at the invocation is right and cheap, so it is taken. Defending the gate against an adversary who already controls the npm configuration is not, because that adversary controls what `npm ci` installed in the first place, and an audit of compromised packages against a registry of their choosing is not the defence. The design's "no bypass of any kind" is therefore an overclaim, and the fix is to pin the registry and state the boundary honestly rather than to chase it further.

Round 5 is declared the last design round for this slice. Anything material left after it becomes a named obligation on the builder and is settled in code review.

## 1. Verdict build after the listed changes another round needed because K1–K4 leave the har...
[codex] Turn completion inferred after the main thread finished and subagent work drained.
## 1. Verdict

build after the listed changes  
another round needed because K1–K4 leave the hardened audit and dependency-tree guard materially incomplete.

## 2. Status of H1, H2, H3 and the guard

H1: Not resolved; the flags close offline and omission paths, but ambient registry/proxy configuration can still produce a green empty report, and the peer fixture is not discriminating.

H2: Resolved; both recursive greps need the directory exclusion, the proposed fixture produces two lines without it and one with it, and `internal/` is correctly cited at `:16`.

H3: Resolved; only portable non-zero is relied upon, with 123 correctly described as an implementation-specific choice.

Guard: Not resolved; most planted-tree effects are covered, but the selected test digest cannot see the live poison, identical-red runs can pass the comparison, and the adversarial corpus omits reasonable file classes.

## 3. The guard

The guard directly catches removal of:

- The behavior-surface membership rule through the ENGINE, LANDING, and PAYLOAD digests.
- The parallel-ratchet skip through `serial_test.go`, which the current scanner parses and inventories ([metasystem/internal/parallelratchet/parallel.go:109](metasystem/internal/parallelratchet/parallel.go#L109)).
- The stop-surface skip through both planted `_test.go` files ([metasystem/internal/audit/stopsurface.go:289](metasystem/internal/audit/stopsurface.go#L289)).
- The Git-fed gofmt change through the unformatted and unparsable Go files; the gate treats either non-zero gofmt status or output as red ([metasystem/scripts/agents/go-gate.sh:515](metasystem/scripts/agents/go-gate.sh#L515)).
- The brain and path-class shell exclusions through their planted tokens. The current path-class scans recurse over the selected roots ([metasystem/scripts/agents/path-class-fixtures.sh:87](metasystem/scripts/agents/path-class-fixtures.sh#L87)).
- The `.gitignore` rule through `git status --porcelain`, assuming the initial clean-status precondition.

It does not directly catch removal of `pathpattern.Pattern.Expand` or `proofrun.readManifest`. A default test run chooses `workspace.StagedTree()` ([metasystem/cmd/metasystem/test.go:486](metasystem/cmd/metasystem/test.go#L486)); its candidate manifest is computed in a detached checkout of that tree ([metasystem/cmd/metasystem/test.go:1708](metasystem/cmd/metasystem/test.go#L1708), [metasystem/internal/gittree/detached.go:139](metasystem/internal/gittree/detached.go#L139)). The ignored live poison therefore cannot reach either the group-input expansion that calculates `inputDigest` ([metasystem/internal/proofrun/test_build.go:1006](metasystem/internal/proofrun/test_build.go#L1006), [metasystem/internal/proofrun/test_build.go:1780](metasystem/internal/proofrun/test_build.go#L1780)) or the manifest walk ([metasystem/internal/proofrun/manifest.go:64](metasystem/internal/proofrun/manifest.go#L64)). Their dedicated fixtures would fail in both runs if a skip were removed; the proposed equality-only comparison can accept that identical failure.

The four-file metasystem-audit difference is correct. The inventory walks the root, does not follow directory symlinks, and reports matching basenames ([metasystem/internal/audit/metasystem.go:391](metasystem/internal/audit/metasystem.go#L391)). Its names include `wow.md`, `SKILL.md`, generic `AGENT.md`, and runtime instruction names ([metasystem/internal/audit/metasystem.go:503](metasystem/internal/audit/metasystem.go#L503)); the runtime declarations add `AGENTS.md` and `CLAUDE.md` ([metasystem/internal/runtimes/runtimes.go:192](metasystem/internal/runtimes/runtimes.go#L192), [metasystem/internal/runtimes/runtimes.go:251](metasystem/internal/runtimes/runtimes.go#L251)). Of the planted files, exactly `AGENTS.md`, `wow.md`, `SKILL.md`, and `CLAUDE.md` therefore add stdout inventory lines. They do not enter the verdict-bearing audit roots ([metasystem/internal/audit/metasystem.go:30](metasystem/internal/audit/metasystem.go#L30)), so stderr and status should remain unchanged.

A reasonable missing adversary is an executable shell payload—both `*.sh` and an extensionless `bin` entry—with content that would trip a shell/version/static scanner. A hidden configuration file such as `.npmrc` is also missing. This is an inference about plausible unenumerated walkers, but those are common dependency-tree contents and are materially distinct from the planted Go, text, Markdown, JSON, and symlink entries.

## 4. Findings

- **K1; MATERIAL; H1 hardened audit.** The command leaves `registry`, proxy, and TLS trust configuration ambient. npm sends the bulk POST to the configured registry; a configured registry returning an empty advisory object produces no vulnerability entries while dependency metadata is still derived from the local tree, so all five checks can pass. The design acknowledges this at [plans/user-interface/g1-s8-frontend-toolchain-design.md:104](plans/user-interface/g1-s8-frontend-toolchain-design.md#L104), contradicting O9’s “no bypass” requirement at [plans/user-interface/g1-s8-frontend-toolchain-design.md:237](plans/user-interface/g1-s8-frontend-toolchain-design.md#L237). npm configuration priority confirms that command-line settings outrank environment and npmrc settings, but no registry or trust setting is supplied ([npm configuration documentation](https://docs.npmjs.com/cli/v11/using-npm/config/)); the audit implementation uses the configured registry for its POST and calculates dependency totals locally ([npm 11.19.0 audit-report source](https://github.com/npm/cli/blob/v11.19.0/workspaces/arborist/lib/audit-report.js)). A builder following this design would ship an audit gate that ambient configuration can make green without consulting the intended advisory authority. Fix direction: define and pin the trusted registry and network trust boundary at higher priority than ambient configuration, and add a fixture in which an ambient registry returns `{}` and must not make the gate green. **Blocks building.**

- **K2; MATERIAL; H1 audit fixture.** The `omit=peer` case contains no peer-only node, so removing `--include=peer` leaves its request body unchanged; the design admits this at [plans/user-interface/g1-s8-frontend-toolchain-design.md:198](plans/user-interface/g1-s8-frontend-toolchain-design.md#L198). The fixture therefore would not fail without that part of the fix, despite the production reasoning correctly identifying `peer` as one of the three omittable types at [plans/user-interface/g1-s8-frontend-toolchain-design.md:100](plans/user-interface/g1-s8-frontend-toolchain-design.md#L100). A builder would produce a regression test that cannot protect one of the required flags. Fix direction: give the fixture lockfile a peer-only node or assert the exact spawned argument vector independently. **Blocks building.**

- **K3; MATERIAL; dependency-tree guard.** The comparison requires equality of statuses and verdict lines but never explicitly requires the absent-tree run to be green ([plans/user-interface/g1-s8-frontend-toolchain-design.md:82](plans/user-interface/g1-s8-frontend-toolchain-design.md#L82), [plans/user-interface/g1-s8-frontend-toolchain-design.md:84](plans/user-interface/g1-s8-frontend-toolchain-design.md#L84)). Removing the `pathpattern` or `readManifest` skip makes its dedicated fixture fail in both runs, while the selected test’s ignored, staged-tree candidate cannot observe the live poison. Equal red statuses and equal red section lines can therefore satisfy the stated comparison. A builder would implement a guard capable of reporting success over two identically broken validations. Fix direction: require every absent-tree command’s expected successful status and verdict before planting poison, then either construct a candidate tree containing the poison for the test-run comparison or state that these two skips are covered only by their mandatory fixtures. **Blocks building.**

- **K4; MATERIAL; dependency-tree guard corpus.** The planted tree contains no executable shell file and no hidden configuration file ([plans/user-interface/g1-s8-frontend-toolchain-design.md:72](plans/user-interface/g1-s8-frontend-toolchain-design.md#L72)). An unenumerated walker restricted to `*.sh`, executable files, package `bin` entries, or dotfiles would see none of the poison, allowing the guard to support a completeness claim it did not exercise. A builder would construct an insufficient structural test for the exact class of failure the guard replaces search-based completeness claims against. Fix direction: add at least an executable `*.sh`, an extensionless executable `bin` entry, and a hidden configuration file carrying verdict-triggering content. **Blocks building.**

The three earlier corrections are folded in correctly:

- `--include=dev` is described as clearing only `dev`, with all three include values required ([plans/user-interface/g1-s8-frontend-toolchain-design.md:100](plans/user-interface/g1-s8-frontend-toolchain-design.md#L100)).
- JSON reports and JSON endpoint errors are assigned to stdout and logs to stderr, preserving both on refusal ([plans/user-interface/g1-s8-frontend-toolchain-design.md:98](plans/user-interface/g1-s8-frontend-toolchain-design.md#L98); [npm 11.19.0 display source](https://github.com/npm/cli/blob/v11.19.0/lib/utils/display.js), [audit-error source](https://github.com/npm/cli/blob/v11.19.0/lib/utils/audit-error.js)).
- A null `audit-level` is read as `low`, and the design consequently relies on JSON counts rather than a threshold ([plans/user-interface/g1-s8-frontend-toolchain-design.md:102](plans/user-interface/g1-s8-frontend-toolchain-design.md#L102); [npm-audit-report source](https://github.com/npm/npm-audit-report/blob/v7.0.0/lib/index.js)).

Node 24.21.0 bundles npm 11.19.0, not 11.19.1 ([Node archive](https://nodejs.org/en/download/archive/v24.21.0)). The relevant audit, configuration, display, and error-handling files are unchanged between the fetched 11.19.0 and 11.19.1 tags, so the earlier version mismatch does not change these conclusions.

H2’s line correction is confirmed: `cmd/`, `internal/`, and `scripts/` are at [metasystem/scripts/agents/path-classes.txt:15](metasystem/scripts/agents/path-classes.txt#L15), [metasystem/scripts/agents/path-classes.txt:16](metasystem/scripts/agents/path-classes.txt#L16), and [metasystem/scripts/agents/path-classes.txt:18](metasystem/scripts/agents/path-classes.txt#L18). The script is listed in both the exclusion slice and must-not-touch boundary at [plans/user-interface/g1-s8-frontend-toolchain-design.md:46](plans/user-interface/g1-s8-frontend-toolchain-design.md#L46) and [plans/user-interface/g1-s8-frontend-toolchain-design.md:219](plans/user-interface/g1-s8-frontend-toolchain-design.md#L219).

The fifth audit check is sound for the specified non-workspace, registry-only lockfile shape and worth keeping as a fail-closed tree-identity backstop. Because it is derived rather than executed, it could refuse the first correct run if npm’s inventory differs from that assumed shape; the design correctly treats that as a finding. It cannot detect an empty successful registry response, omission filtering, a workspace filter, or a different project having the same package count.

## 5. Not checked

No npm command, repository script, metasystem binary, validation, or guard transcript was executed. The derived first-run dependency-count equality, actual request bodies, exact future guard implementation, real installed dependency tree, and platform grep/xargs executions remain unchecked. The prohibited local configuration and concurrently revised G1-S9 design were not read.

---

# Round 5, 2026-09-21: closed with obligations

On revision 5 at commit `3cc0f9318`. Verdict: build with the listed obligations. Nothing was judged unsafe to carry into code review.

Revision 5's claim that `Pattern.Expand` is reached live through a delivery test plan is **false**, and the planner verified the refutation in source: a trailing `/**` is parsed into a subtree flag, `HasComponentWildcard` looks for `*` or `?` only within the remaining components (`internal/pathpattern/pattern.go`), so `SnapshotRelevant` takes its non-wildcard subtree branch and never calls `Expand` (`internal/gittree/snapshotscope.go:523` to `:539`). The forced staging still admits ignored files and makes parity refuse, but that proves the subtree declaration, not the skip. The planner had repeated that claim to the human before checking it.

Seven obligations carry to the builder and are checked in code review. Five design rounds: six material findings, then four, two, four, and none unsafe to carry.

## 1. Verdict build with the listed obligations ## 2. Status of K1 to K4 K1 — Buildable as wr...
[codex] Turn completion inferred after the main thread finished and subagent work drained.
## 1. Verdict

build with the listed obligations

## 2. Status of K1 to K4

K1 — Buildable as written: the boundary is accurate within the planner’s ruling, states the ambient proxy and TLS exclusions honestly, and the pin-removal control makes the registry fixture non-vacuous ([design:117](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s8-frontend-toolchain-design.md:117), [design:121](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s8-frontend-toolchain-design.md:121), [design:221](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s8-frontend-toolchain-design.md:221)).

K2 — Builder obligation: all five controls are discriminating if npm accepts the synthetic lockfile—the offline control makes no request, the optional/dev/peer controls respectively remove `o`, `d`, and `p`, and removing the registry pin sends the successful empty-report request to the ambient server ([design:219](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s8-frontend-toolchain-design.md:219), [design:221](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s8-frontend-toolchain-design.md:221), [npm load-virtual](https://github.com/npm/cli/blob/v11.19.0/workspaces/arborist/lib/arborist/load-virtual.js#L58-L97), [npm audit selection](https://github.com/npm/cli/blob/v11.19.0/workspaces/arborist/lib/audit-report.js#L294-L358)).

K3 — Builder obligation: the green-first precondition closes the equal-red hole, but the claimed live `Pattern.Expand` chain is false and the `readManifest` reachability characterization is only conditional.

K4 — Builder obligation: the corpus now exercises the important extension, executable-bit, extensionless-bin, `.bin` symlink, nested-tree, dotfile, hidden-directory, YAML, contract-name, malformed-source, and symlink classes, but it should also plant `metasystem.conf`, this repository’s other primary exact-name contract beside `testing.json` ([test.go:424](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/test.go:424), [adopt.sh:336](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/scripts/adopt.sh:336)).

## 3. The Expand chain

Read from source: both `test plan` and `test run` enter `prepareTesting`; delivery preparation calls `checkDeliveryInputParity`, which derives declarations from selected groups and passes them to `SnapshotRelevant` ([test.go:148](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/test.go:148), [test.go:412](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/test.go:412), [test.go:600](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/test.go:600), [test.go:801](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/test.go:801), [test.go:840](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/test.go:840)).

The chain then diverges from the design: parsing `metasystem/internal/**` removes `/**` into the separate `subtree` flag, while `HasComponentWildcard` checks only `*` or `?` inside the remaining components; consequently `SnapshotRelevant` takes its non-wildcard subtree branch at lines 538–539 and never calls `Pattern.Expand` at line 533 ([pattern.go:21](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/pathpattern/pattern.go:21), [pattern.go:102](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/pathpattern/pattern.go:102), [snapshotscope.go:509](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/gittree/snapshotscope.go:509), [snapshotscope.go:523](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/gittree/snapshotscope.go:523), [testing.json:102](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/testing.json:102)). The forced `git add -A -f` still admits ignored poison and makes parity refuse, but that proves the subtree declaration and forced staging, not the `Expand` skip ([snapshotscope.go:569](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/gittree/snapshotscope.go:569)).

The absent-tree checklist covers every command and semantic verdict compared later: status, three digests, both audits, audit stderr, gate verdict, validation section verdicts, plan exit, and test-run group status; because planting follows those checks, an equal-red first pass cannot pass as written ([design:73](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s8-frontend-toolchain-design.md:73), [design:93](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s8-frontend-toolchain-design.md:93), [design:95](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s8-frontend-toolchain-design.md:95)).

`readManifest` is called three times by `Freeze`, and `gate witness-freeze` calls `Freeze`, but the script paths that activate this on a live tree subsequently execute `go-gate.sh` inside the exported tree; therefore “nothing reads the export’s contents” is false if that branch is reached ([freeze.go:18](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/proofrun/freeze.go:18), [gate.go:97](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/gate.go:97), [go-gate.sh:366](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/scripts/agents/go-gate.sh:366), [witness-gate.sh:250](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/scripts/agents/witness-gate.sh:250)). Conversely, the clean witness path explicitly uses `git archive` and does not call `witness-freeze`, so source alone does not prove that the guard actually enters the live `readManifest` path ([witness-gate.sh:103](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/scripts/agents/witness-gate.sh:103)).

The poisoned-candidate alternative is refused under delivery because the candidate must equal the real index; diagnostic purpose avoids that parity check, and its detached-candidate `FullDigest` does call `readManifest`, contrary to the claim that it reaches it nowhere ([test.go:486](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/test.go:486), [test.go:597](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/test.go:597), [test.go:1708](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/test.go:1708), [manifest.go:237](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/internal/proofrun/manifest.go:237)).

`readManifest` may rest fixture-covered only: it is one known walker with a mandatory focused regression fixture, provided code review verifies that removing its skip makes that fixture fail and makes no unsupported guard-coverage claim; `Expand` must receive the same ruling unless an actual component-wildcard live probe and mutation test demonstrate guard coverage.

## 4. Obligations for the builder

1. Implement the guard so every absent-tree command and semantic verdict listed at design line 73 is green before any poison path is created, and have code review map that checklist one-to-one to every command and result compared at lines 93–95.

2. Prove `Pattern.Expand` guard coverage by removing its skip and observing the guard refuse through an actual component-wildcard declaration, or record it as fixture-covered only and prove its focused fixture fails without the skip.

3. Keep `proofrun.readManifest` fixture-covered only unless a transcript plus skip-removal mutation proves a live guard route, and require its focused fixture to fail when the skip is removed.

4. Add a poisoned `metasystem.conf` to the K4 corpus and require the two guard passes to remain identical.

5. Author proposal agreed: record the exact npm version driven and confirm the relied-on npm 11.19.0 behavior rather than assuming the cited 11.19.1 files are unchanged ([design:10](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s8-frontend-toolchain-design.md:10)).

6. Author proposal agreed: code review must verify the guard’s exact `test plan` and diagnostic `test run` argument vectors against `parseTestingSelection` ([design:93](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/plans/user-interface/g1-s8-frontend-toolchain-design.md:93), [test.go:197](/Users/wido/LocalStorage/GitHub/agentic-tools-ui/metasystem/cmd/metasystem/test.go:197)).

7. Author proposal agreed with correction: retain the delivery-plan transcript, but treat selection of a group declaring `metasystem/internal/**` as proof of forced subtree parity only, not proof that `Pattern.Expand` ran.

8. Author proposal agreed: run the synthetic lockfile fixture and require the production, dev-only, optional-only, and peer-only controls plus the literal spawned argument-vector assertion to pass.

9. Carry the fifth unverified item as an additional obligation: test ambient `workspace`, `workspaces`, and `include-workspace-root` configuration and require either npm refusal or the unchanged exact four-package bulk body, hardening the command if any setting narrows it.

10. Require the ambient-registry control to remove only `--registry=`, observe one bulk POST at the ambient server and none at the pinned server, and observe the empty response produce the otherwise-green report that makes the control discriminating.

## 5. Findings that are NOT safe as obligations, if any

None.

## 6. Not checked

No npm or npx command, installation, repository script, MetaSystem binary, guard, validation, audit, server, or browser walkthrough was run; the actual delivery selection, guard transcript, synthetic-lockfile acceptance, workspace-configuration behavior, generated dependency tree, advisory result, bundle, and CSP behavior remain unexecuted, and `metasystem/metasystem.conf.local` was not read.
