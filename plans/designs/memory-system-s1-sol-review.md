# Sol's code review of memory-system slice 1 (the resolver), commit 81372a3cc

Produced 2026-09-22 by Codex on `gpt-5.6-sol`, read-only, against the design at [memory-system.md](memory-system.md) sections 2–4 and 7. Verbatim.

---

**Verdict: accept after the listed fixes** — path containment, question visibility, and project-wide identity uniqueness are required before this slice is safe to ship.

1. **High — Chapter and record paths can escape the checkout.**
   Evidence: `internal/project/answers.go:168`, `project.go:237`, `project.go:285`.
   Scenario: an existing `doc:../outside.md` passes `os.Stat`; a checkout symlink to an outside file also passes; a directory passes as a "document." Markdown or `questions.md` symlinks in a home are followed by `os.ReadFile`, so the resolver can read and expose bytes outside the supplied roots. On Windows, `..\outside.md` is another traversal spelling.
   Smallest fix: resolve chapter paths against the canonical checkout, reject absolute or escaping paths after symlink evaluation, and require a regular file. Read home entries and the register without following symlinks outside their home.

2. **High — Questions are read only for `check`; the other four query surfaces omit the `question` kind.**
   Evidence: `project.go:35`, `answers.go:18`, `answers.go:104`, `cmd/metasystem/project_verbs.go:63`.
   Scenario: `project list question` is rejected as usage, `project show Q-1` reports no record, and `project tree` excludes open questions from kind and status counts. An `answered: decision-id` row is also absent from that decision's "referenced by" output. This contradicts the fifth kind/home and makes the questions register invisible except to `check`.
   Smallest fix: project questions into the common list/show/tree/reference query model, normalizing question statuses to `open`, `answered`, and `withdrawn`.

3. **High — Duplicate identity checking excludes question IDs.**
   Evidence: `answers.go:131`, `answers.go:189`.
   Scenario: a page with `Id: Q-1` and the register row `Q-1` pass, as do two question rows with the same ID. The design says IDs are unique across the project, and `check` must refuse duplicate IDs.
   Smallest fix: use one identity index for pages and register rows, reporting every later declaration at its own line.

4. **Medium — A wrong-kind `index.md` still becomes a book and can declare areas.**
   Evidence: `project.go:269`.
   Scenario: `docs/intent/index.md` declaring `Kind: design` is nevertheless treated as the intent book, and its `## Areas` entries authorize page and question areas. The contract requires the index to be a record of the book's kind. This can be fixed without adding a wrong-home refusal.
   Smallest fix: form a book only when `index.Kind == home.Kind`; otherwise leave the record readable under its declared kind but do not use it as that book's index.

5. **Medium — Valid records in dot-prefixed files or directories are silently excluded.**
   Evidence: `project.go:230`.
   Scenario: `plans/designs/.draft.md` or `plans/designs/.team/security.md` can carry a valid head but will never be listed or checked. The design allows any filename and design subdirectories without a hidden-path exception.
   Smallest fix: remove the dot-entry filter, while retaining regular-file and containment checks.

6. **Medium — `check` has semantic refusals beyond the design's exact list.**
   Evidence: `record.go:121`, `record.go:128`, `answers.go:179`.
   Scenario: it additionally refuses a malformed nonblank head line, any duplicate key—including an unknown key that the design says is kept and ignored—and an existing chapter record of another kind. The tests deliberately assert all three. Unreadable files also produce an operational refusal, which is sensible fail-closed behavior but is not a listed semantic class.
   Smallest fix: restrict semantic findings to the six listed classes, or amend the accepted design before retaining these extra rules.

7. **Medium — `project id` does not mint a standard ULID.**
   Evidence: `project.go:181`, `internal/goal/identity.go:11`.
   Scenario: the reused goal function emits 26 independent random Crockford characters. It contains no timestamp, and 75% of outputs have a first character above `7`, which a standard ULID decoder rejects as 128-bit overflow.
   Smallest fix: centralize a genuine timestamp-plus-randomness ULID minting function and reuse it without adding a module dependency.

8. **Low — New durable comments contain slice and design-history language.**
   Evidence: `project.go:1`, `project.go:29`, `record.go:18`, `project_verbs.go:3`.
   Scenario: future readers are told about "step 1," the "memory-system design," and an interface "slice," contrary to the repository rule that source comments describe standing behavior without project history.
   Smallest fix: restate these as permanent boundaries: references are intentionally unvalidated, roots are locally owned to avoid a UI dependency, and the family is read-only.

Non-material observations:

- Builder decisions 1–5 and 7–11 are sound: candidate detection, case-insensitive head keys, empty-required handling, register-area validation, no wrong-home refusal, output formats, no `--root` for `id`, subject-before-flags parsing, suppressing a repeated `project` bucket, and treating an absent/headless index as no book. Decision 6 is finding 7. Decision 12—one package-map row and no family-table row—matches the existing architecture document's incomplete family table.
- An em dash inside a chapter title is retained because parsing splits at the first dash. Repeated `## Chapters` headings are concatenated in file order. A relabelled register header hides the table entirely; short rows may produce the listed ID/status refusals, while extra cells are ignored. The exact refusal list does not authorize separate header or cell-count findings.
- Missing home directories are accepted. Relative and trailing-slash roots are canonicalized before `stateroot.templateMode`; when checkout root equals state root, no duplicate design home is added. List output is path-sorted, tree output follows declared area order with fixed tally order, and partial semantic failures leave list/show/tree at exit 0 while `check` exits 1.
- Coverage is missing for the material scenarios above, case-insensitive heads, undeclared question areas, headless indexes, an explicit `project` area row, relative/trailing-slash CLI roots, and `project id --root`. The two-layout fixture does exercise the real self-hosted marker at `project_test.go:41` and an adopted vendored installation.
- Read-only static checks found no formatting or whitespace errors, no new module dependency, and correct `pathFlag` use. Changed tests use `testenv.Main`, every test and independent subtest calls `t.Parallel()`, and none adds `os.Setenv`, sleeps, timers, or fixed ports.
- Tests were not executed because Go compilation would create cache and temporary files. The known path-helper failure predates this commit: `git log -1 -- cmd/metasystem/ui.go` reports `e5be8faef`, and the offending declaration remains at `ui.go:36`; this slice does not touch that file.

## Dispositions (the designer, 2026-09-22)

Accepted as written: 1, 2, 3, 4, 5, 7, 8 — one fix-up commit after the pane lands, canary-verified (`go test ./internal/project/...` and the fast gate only). Finding 6: the three extra refusals stay and the design's list gains them (a malformed head line, a duplicate key, a chapter of another kind are all things a human would want refused, and the design's section 4 already refuses "a chapter of a different kind" through the book rule); the design is amended in the same commit rather than the code weakened.
