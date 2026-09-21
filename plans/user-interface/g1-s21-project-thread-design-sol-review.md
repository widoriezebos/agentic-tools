# g1-s21 Project thread: Sol's review of the document route

Reviewer: Codex on `gpt-5.6-sol`, read-only, 2026-09-21. Scope: the document route and the Markdown path only. Two earlier attempts at a broader review of this design exhausted their turn and delivered nothing, so this one was deliberately narrowed to the slice's one genuinely dangerous mechanism; the rest of the design is reviewed by the planner and stands.

Verdict: the boundary holds after three fixes. The planner accepts all three. The worst case is a confused deputy: resolving a path, checking containment, then opening it by name leaves a window in which the name can be swapped, so the server would read a file the caller cannot reach and return its contents as parsed document JSON.

The planner also rules, rather than referring it to the human, that the route stays broad: it serves any Markdown under the checkout so that links between designs resolve, which is the point of the thread. The review's own judgement is that this adds little exposure once the fixes land, because a hostile page cannot read the responses and an agent with filesystem tools already has that access. The human was told, and may narrow it to the catalogue plus linked documents on return.

## 1. Verdict the boundary holds after the listed fixes Worst case: a pathname swap between `...
[codex] Turn completion inferred after the main thread finished and subagent work drained.
## 1. Verdict

the boundary holds after the listed fixes

Worst case: a pathname swap between `EvalSymlinks`/`Lstat` and `open` can make the server return an arbitrary readable, UTF-8 file outside the checkout as parsed document JSON.

## 2. Cases

| Case | Holds or fails | What reaches the browser if it fails | Evidence |
|---|---|---|---|
| Symlinked middle directory | Fails the stated “symlinks are refused” claim. An in-checkout target passes containment; an outside target is refused absent a race. `Lstat` of the resolved pathname cannot identify the original symlink. | The target Markdown as title, absolute path, revision, headings and blocks. | `plans/user-interface/g1-s21-project-thread-design.md:62`, payload at `:80`; [`EvalSymlinks` resolves every component](https://pkg.go.dev/path/filepath#EvalSymlinks). |
| Resolve/check/open window | Fails. Resolution and containment only describe the name at that instant; replacing a parent or leaf before open redirects the later pathname lookup. Putting `Lstat` before open narrows but does not close this window. | Any replacement file below 1 MiB that is valid UTF-8—even non-Markdown content behind a `.md` request name—becomes parsed blocks in the JSON payload. | Inference from the separate operations required at `plans/user-interface/g1-s21-project-thread-design.md:62`; Go’s rooted API instead guarantees components remain beneath an opened root ([`os.Root`](https://pkg.go.dev/os#Root)). |
| Hard link | Holds only as a pathname boundary: a hard link is a regular in-checkout directory entry, although the same file also has an outside name. | Its complete valid UTF-8 contents are intentionally served. | [`os.Link` creates another hard link to the same file](https://pkg.go.dev/os#Link); the design admits every in-checkout `.md` at `plans/user-interface/g1-s21-project-thread-design.md:62`. |
| `.GIT` / `.Git` on macOS | Fails on default case-insensitive APFS: the textual segment check misses the spelling while filesystem lookup reaches `.git`. The same applies to the other three rejected names. | Any `.md` already beneath the rejected directory. This checkout currently has zero `.md` files under `.git`, so exploitation here needs one placed first. | Apple says APFS is case-insensitive by default ([Apple](https://support.apple.com/en-kw/guide/disk-utility/dsku19ed921c/mac)); exact-name rule at `plans/user-interface/g1-s21-project-thread-design.md:62`. |
| Unicode normalization | Holds for the `.git` exclusion: ASCII `.git` has no distinct canonical normalization. Compatibility lookalikes may normalize under NFKC, but that does not establish filesystem aliasing. Canonically equivalent non-ASCII names can address one APFS file, producing two request IDs for one document, not access to `.git`. | No forbidden file; at worst duplicate identities for an allowed document. | APFS is normalization-insensitive ([Apple](https://developer.apple.com/library/archive/documentation/FileManagement/Conceptual/APFS_Guide/FAQ/FAQ.html)); canonical versus compatibility normalization ([Unicode UAX 15](https://www.unicode.org/reports/tr15/)). |
| Other dot directory | Holds, broadly: only four names are denied. | Full contents of files such as `.claude/agents/design-critique.md`, including agent instructions. | `.claude/agents/design-critique.md:1-10`; allowlist rule at `plans/user-interface/g1-s21-project-thread-design.md:62`. |
| Existing sensitive Markdown | Holds as designed, but the surface is large. Rulings, proposals, handoffs, goals, plans and records are served; `artifacts` is blocked. Adopted projects similarly expose arbitrary `.md`. | For example, human rulings (`metasystem/memory/rulings.md:24-35`), machine/session handoff details (`metasystem/plans/handoff-m1b-2026-09-03.md:1-15`), and spend records (`metasystem/records/waits/waits-run-outside-spend-report-2026-09-19.md:1-14`). | The route deliberately ignores the catalogue: `plans/user-interface/g1-s21-project-thread-design.md:62`. |
| Size and binary data | Fails as a resource bound because the design does not require a bounded read. A pre-open size check can be invalidated by growth or replacement. | Normally a 200 `too-large` or `unreadable` document with metadata and no blocks; an unbounded implementation may first allocate/read the entire file. | `plans/user-interface/g1-s21-project-thread-design.md:62,100,105-106`. |
| HEAD | Holds for disclosure: HEAD must return no body. The current handler admits HEAD without a route-specific branch, so a straightforward implementation may still perform the full read/hash/parse and merely discard the body. | Status and headers only. | `metasystem/internal/ui/httpd/httpd.go:104-134`; [RFC 9110 §9.3.2](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.3.2). |

## 3. The check order

Stated: clean and validate name → reject segments/suffix → `EvalSymlinks` and containment → `Lstat` → size → UTF-8.

Required: validate the original ID, case-fold rejected segments, then open through an anchored checkout directory while refusing symlinks component-by-component; `fstat` that opened descriptor, enforce regular-file and size rules, read at most 1 MiB plus one byte from that same descriptor, then validate UTF-8 and parse. Never resolve or inspect one pathname and reopen it later.

## 4. Residual risk at this gate

Loopback is adequate for the intentionally broad Markdown surface after those fixes, but it is not authentication. The hostile page cannot read responses: cross-site fetches are rejected, although it can trigger a top-level GET navigation under the explicit exemption (`plans/user-interface/g1-s1-server-lifecycle-design.md:184-189,252-254`).

An agent with checkout tools can directly request the route and can place hidden files, links, or aliases. For ordinary Markdown this adds little because that agent already has the granted filesystem access. The race is different: it turns the server into a confused deputy capable of reading beyond that access, so it must be fixed. Middle symlinks, hard links, `.GIT`, and hidden-directory cases otherwise require a suitable local file or link already to exist.

## 5. Fixes that must land before build

1. Replace `EvalSymlinks` plus pathname reopen with one anchored, component-safe open that enforces the chosen all-symlinks-refused policy.
2. Compare every rejected segment case-insensitively and test mixed-case forms on a case-insensitive filesystem.
3. `fstat` and bounded-read the same opened descriptor, using a 1 MiB-plus-one limit before UTF-8 validation or Markdown parsing.

## 6. Not checked

The sandbox refused `/tmp` creation with `Operation not permitted`, so filesystem experiments could not run; the route does not exist yet, and `metasystem/metasystem.conf.local` was not opened.
