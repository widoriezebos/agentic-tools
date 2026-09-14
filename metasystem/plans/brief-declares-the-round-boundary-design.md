# brief-declares-the-round-boundary

- Owner: m1e (goal 28 of plans/delivery-efficiency-plan.md). **Revision 2, 2026-09-14**, folded by the Codex gpt-6-astra design delegate from critique round 1 (records/misc/brief-declares-the-round-boundary-critique-r1.md, eight material findings, all accepted) under two seat rulings; the seat integrated the page and edits only this header block.
- Goal and current status: the brief carries a paired Boundary and Ceiling or neither; review-stage conformance refuses a candidate outside them before a read is spent; nine builder units of at most 400 changed lines each (1a, 1b, 2, 3, 4a, 4b, 4c, 6a, 6b), in order, each with its own Boundary, Ceiling, Non-goals and the proof rule, and a complete rule-to-witness table. Nothing built yet.
- In flight right now: revision 2 under critique round 2 (Codex gpt-5.6-sol, design critic), the last round inside this tier-2 goal's review budget; a clean round closes the design, a material round folds once more and then escalates under R-97-m1e.
- Decisions made (and who made them): Wido, 2026-09-14: scope drift and unproved rules are fixed upstream in the brief and the builder's return; units are at most 400 changed lines. The seat, 2026-09-14, on critique round 1: both headers or neither, a partial pair refuses naming the missing header; every remove-one witness lives in the unit that owns its production rule. The design delegate: the bounded diff is the whole merge-base candidate; the supplied round's authored brief binds; one candidate snapshot; one byte-preserving path identity shared by Boundary members and authority citations; trailing-slash declarations are literal directory prefixes.
- Waiting on the human: nothing. Headerless briefs keep today's behaviour; a default Ceiling would change DONE and is not proposed.
- Dead ends (do not retry without new evidence): partial-header admission; matching projected declarations against unprojected repository paths; an empty projected prefix matching sibling paths; a test-only unit that must mutate another unit's production rule; slot-only selection of a referenced brief.
- Next step: revision 2 lands with this line, so critique round 2 has a landed subject; on a clean round, unit 1a (parse the paired headers) is briefed to Codex gpt-5.6-sol with its own Boundary, Ceiling and proof rule, read by Opus, landed by the seat, and the remaining units follow in the stated order.

The seat runs review conformance before dispatching a code critic. The brief supplies the outer path and size limits. The return still declares the concrete changed paths. Both checks must pass.

This is a design proposal. The fact pass began at `91f097429f138cf51d427d2666fba4958899047e`. The shared checkout advanced during the pass. Cited source locations were checked against the final checkout at `bf59fd0d699c13e3f606d808434238c77fa70d8a`, including the moved refusal-register and testing-group lines. The DONE contract is `plans/goals/brief-declares-the-round-boundary.md:8`. All source citations below are relative to `metasystem/`. Boundary values are relative to the Git repository root, so this repository's builder headers include `metasystem/`. Test names without a current source citation are proposed tests.

The bounded diff covers the entire returned candidate against its merge-base with the target checkout. Current base selection already uses that merge-base (`internal/validate/conformance.go:232-241`). A follow-up does not reset the diff to its own starting commit.

## Header contract

An implementer brief may carry:

```text
Working Mode: implement
Boundary: ["metasystem/internal/dispatch/brief.go", "metasystem/internal/validate/", "metasystem/scripts/agents/*.sh"]
Ceiling: 400
```

Recognize each exact, case-sensitive prefix at column zero. Use one physical line per header and trim whitespace around its value. Do not trim decoded path members. Scan the authored task direction only. Fences do not change header recognition: a column-zero header inside a fence is live. A header with leading space, tab, or `>` is not live. Working Mode currently uses column-zero scanning and value trimming (`internal/dispatch/brief.go:47-53`).

A brief carries both Boundary and Ceiling or neither. Neither means no new path or size check. Exactly one means `BRIEF_BOUNDS_INVALID` naming the missing header:

```text
BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary
BRIEF_BOUNDS_INVALID: Boundary: required with Ceiling
```

Check duplicate occurrences first, Boundary then Ceiling. Next check pairing. Next validate Boundary, then Ceiling. This gives a deterministic error when several inputs are wrong.

Boundary is a JSON array of strings. `[]` is a valid deny-all boundary. Reject an empty value, `null`, a non-array, a non-string member, trailing JSON data, or a duplicate header. Duplicate members are harmless. Keep the decoded array; matching needs no deduplication step.

Ceiling is one or more ASCII decimal digits representing a value from 0 through 9223372036854775807. Leading zeroes are valid. Reject signs, fractions, units, placeholders, overflow, empty values, and duplicate headers. Zero permits no added or deleted text lines. Equality passes.

Add these definitions beside Working Mode in `internal/dispatch/brief.go`:

```go
type BriefBounds struct {
    Boundary []string
    Ceiling  *int64
}
type BriefBoundsRefusal struct { Header, Detail string }
func (e *BriefBoundsRefusal) Error() string
func ParseBriefBounds(data []byte) (BriefBounds, error)
func ReadBriefBounds(briefPath string) (BriefBounds, error)
```

Successful parsing returns either `nil, nil` fields for absence, or a non-nil Boundary slice and non-nil Ceiling. A non-nil empty slice remains distinct from absence. Do not return a successful partial state.

Render parser errors as `BRIEF_BOUNDS_INVALID: <Header>: <detail>`. Besides the pairing details above, use `header occurs more than once`, `expected a JSON array of paths`, `invalid path or pattern <JSON string>`, and `expected a nonnegative decimal integer no greater than 9223372036854775807`.

BriefMode validates its existing Working Mode rule, then calls the bounds parser. ValidateBriefAuthority calls the same parser without requiring Working Mode. This covers the authority-only follow-up route. Current CLI admission checks authority before mode when both are requested; retain that order (`cmd/metasystem/dispatch_verbs.go:2014-2027`). Do not promise a missing-mode diagnostic takes precedence over an authority or bounds error on that route. Mode-only admission keeps its existing mode refusal (`internal/dispatch/brief.go:41-55`).

Change the initial shell failure suffix to `brief headers are invalid; Working Mode must be filled and Boundary and Ceiling must appear together`. The current suffix names only Working Mode (`scripts/agents/dispatch.sh:1542`). Parsing stays in Go.

## Paths and matching

Use one path identity throughout this feature: the repository-relative string after JSON decoding, with slash separators and all member bytes preserved. Do not clean paths, fold case, trim member whitespace, resolve symlinks, or replace backslashes. Thus a space, comma, quote, tab, or newline inside a filename remains part of that filename. JSON provides a one-line spelling for those characters.

Reject empty or absolute declarations, NUL, and empty, `.`, or `..` slash components. Only the final empty component of a trailing-slash directory is allowed. Syntax validation has two exclusive branches:

1. A declaration ending in `/` is a literal directory prefix. Validate its components, then stop. Do **not** call `path.Match` to validate it. All glob characters and backslashes in this branch are literal. For example, `metasystem/a[/` is valid and permits descendants of the literal directory `a[`.
2. Every other declaration is a `path.Match` pattern. Validate its syntax with that function, then use it against the whole changed path. An ordinary filename matches exactly. `*` stays within one component; `?`, classes, and backslash escapes use Go's slash-based pattern grammar. `**` has no recursive meaning. There is no shell expansion, Git pathspec expansion, brace expansion, or negation.

An unmatched pattern is valid and allows nothing. No declaration needs to exist on disk. A directory prefix includes arbitrary descendant depth and respects its final slash: `internal/validate/` cannot match `internal/validator.go`.

Perform nested projection inside `conformanceRun.briefBoundsViolations`, before calling `briefBoundaryMatches`:

1. For each declaration, remember whether it ended in `/` before projection. Call `r.projectDeclaration` once. An outside-project declaration returns a typed `BriefBoundsRefusal` with Header `Boundary` and the existing projection diagnostic. Current projection requires the literal installation prefix and strips it exactly once (`internal/validate/conformance.go:284-293`).
2. For each changed repository path, first test the raw prefix `r.installPrefix + "/"` when the installation is nested. A sibling fails immediately. Add its original repository spelling to the Boundary offenders without calling the matcher.
3. Only after that guard, call `r.installationPath(repositoryPath)`. Match that project-relative path against the projected declarations. At a root installation both projections are identities.
4. A trailing-slash declaration equal to `metasystem/` projects to the empty string with its directory flag still true. It permits every **inside-project** path. The prior sibling guard prevents it from permitting a sibling. A declaration `metasystem/metasystem/a.go` projects to `metasystem/a.go`, with one prefix retained.
5. Keep original repository strings for diagnostics. Pass project paths to the existing cumulative return check.

Make `installationPath` a literal, single-prefix removal for Git paths. Today it also replaces backslashes and trims slashes (`internal/validate/conformance.go:88-95`). That would corrupt the identity promised above. Its only current call is waiver classification over repository paths (`internal/validate/conformance.go:97-107`). Keep that caller and its waiver policy. Test ordinary and unusual Git spellings at this shared helper, plus the existing nested waiver guard.

Use `briefBoundaryMatches(projectDeclaration, projectPath string, directory bool) bool` in `internal/validate/brief_bounds.go`. The explicit directory flag keeps whole-project declarations unambiguous. Current return matching uses exact map membership (`internal/validate/conformance.go:798-810`). Keep directories and patterns out of that return dialect.

Authority extraction shares the decoded identity. A live Boundary line contributes no input citations. Skip the whole line, so neither its member nor a fragment can become an input. Do not use Boundary permission to suppress a citation on another line. Existing input use wins over output use (`internal/dispatch/brief.go:155-167`).

Recognize an inert example by this exact lexical form: one or more leading ASCII spaces or tabs followed immediately by `Boundary:` or `Ceiling:`. Skip that entire line in authority extraction and ignore it in bounds parsing. This does not set output permission or suppress input use on another line. Examples in templates use this indented form. Fences alone are insufficient. A `>` quote is not this exemption; indent the example instead. The current extractor scans all lines without a fence state (`internal/dispatch/brief.go:128-160`).

For a brief with a valid pair, extend authority token extraction on other lines as follows. Recognize complete single-backtick path spans and complete JSON string literals before the old token scanner. JSON escapes are decoded with the same string decoding as Boundary members. Backtick contents are literal, except that a complete JSON string inside the span is decoded. Use JSON escapes for embedded newlines. Preserve spaces, commas, quotes, tabs, and backslashes. Consume the whole recognized span before scanning the rest of the line, so it cannot also emit fragments.

Apply the existing directory eligibility rule to a structured citation's decoded first components. Also recognize a structured citation equal to a decoded concrete Boundary member, including a repository-root filename or a file under a new directory. This makes a separately cited exact member an input even when its parent is absent. Directory and pattern declarations themselves remain output-only. Keep the old scanner for ordinary unquoted citations and headerless prose. It currently restricts tokens and directory eligibility (`internal/dispatch/brief.go:13-24`, `internal/dispatch/brief.go:141-154`). Do not claim arbitrary unquoted prose containing whitespace is an unambiguous path citation.

Use the existing input/output accounting after decoding. Input spans on Workspace output lines or after Create retain those existing output classifications; an ordinary input citation elsewhere wins. The committed-tree and runtime-artifact lookup owners stay unchanged (`internal/dispatch/brief.go:83-104`). Special-character tests must assert the full `MissingPaths` string, then commit the exact file and prove admission. A fragments-only assertion is insufficient.

Named witnesses for these decisions include `TestBriefBoundaryNestedPathProjection`, `TestBriefBoundaryWholeProjectExcludesSibling`, `TestInstallationPathPreservesGitIdentity`, `TestBriefBoundsTrailingSlashIsLiteral`, `TestBriefAuthoritySpecialCharacterInput`, and `TestBriefAuthorityIndentedBoundsExampleIsInert`. Their independent cases are listed in the rule table.

## Which brief binds a round

Use the seat-authored task direction of the supplied implementer job. Current composition stores it as source `caller:brief` in slot `task-direction` (`internal/dispatch/composition.go:268`). Delivered source ranges and digests are recorded (`internal/dispatch/composition.go:380-390`). Initial and follow-up prompts and compositions are persisted per round (`scripts/agents/dispatch.sh:1924-1929`, `scripts/agents/dispatch.sh:2883-2888`).

Add `func (r *conformanceRun) reviewBriefBounds() (dispatch.BriefBounds, error)` in `internal/validate/brief_bounds_source.go`. Resolve `r.rootJob` and `r.roundText` from the supplied job. Do not select the latest round in the chain.

When the round's `composition.json` exists:

- Require readable, valid metadata for that job and round and exactly one `task-direction` source whose Source is `caller:brief`. Require a valid range within `prompt.md` and verify its DeliveredDigest before using it.
- If a task-direction reference exists, require exactly one. Join it to the selected source by **slot, digest, and byte count**: Reference.Slot equals Source.Slot, Reference.Digest equals Source.SourceDigest, and Reference.Bytes equals Source.SourceBytes. The slot comparison is the reference selector itself; do not add a redundant second slot comparison that cannot have an independent remove-one witness. Also require a nonempty repository-relative Path without parent components, an absolute OpenPath, and OpenPath's slash spelling ending in `"/" + Path`. These are the existing binding facts in `internal/dispatch/build.go:1050-1077`.
- Then call `dispatch.ReadVerifiedReference(r.root, reference)`. It checks root containment, regular-file status, actual bytes, and digest (`internal/dispatch/references.go:24-49`). It does not perform the source join or the Path/OpenPath binding. The bounds reader must do those checks before the call.
- With no task-direction reference, require an inline source body that matches SourceBytes and SourceDigest. The renderer uses `# Task Direction\n\n`, the raw body, a newline if needed, and one final newline (`internal/dispatch/composition.go:366-390`, `internal/dispatch/composition.go:555-562`). Recover the body by that envelope and SourceBytes, check its digest, and verify the envelope. Parse only the recovered body. A missing reference must not let a reference stub masquerade as a headerless inline brief.
- With a reference, parse only the verified body. Do not parse the stub or any other slot.

Use the existing SHA-256 helper for byte comparisons (`internal/validate/recertification.go:115-118`). Validate only the evidence needed for this source selection; do not create another full job admission validator or change the composition schema.

For a legacy round with no composition, select the root's `brief.md` if the supplied job is the root. Otherwise select that round's `prompt.md`. Call `dispatch.ReadBriefBounds` on that selected file. Initial dispatch stores the root copy (`scripts/agents/dispatch.sh:1918`); the existing successor reader has the legacy prompt fallback (`internal/validate/conformance.go:925-934`). If the selected legacy file is absent, return unbounded. An unreadable present file, malformed present composition, invalid source, or broken reference refuses. Do not fall back from bad present evidence to legacy evidence.

Preserve `BriefBoundsRefusal` from parsing. Wrap other source failures at review as `BRIEF_BOUNDS_UNREADABLE`. Keep the exhaustion reader unchanged; its inline branch currently returns the whole prompt (`internal/validate/conformance.go:942-960`).

Each round stands alone. A headerless follow-up is unbounded. A partial follow-up refuses. Do not inherit a prior pair, union brief boundaries, or add ceilings. A bounded correction brief repeats both headers for the whole still-unlanded candidate. If it omits an earlier changed file or lowers Ceiling below the cumulative count, review refuses. A scope amendment uses a new round's brief.

Return declarations remain cumulative (`internal/validate/conformance.go:754-817`). Cap continuation currently tells the builder to declare the predecessor's changes too (`internal/dispatch/capcontinuation.go:22-28`, `internal/dispatch/capcontinuation.go:89`). Clarify that this is the return's concrete declaration and that the current brief's pair governs the whole candidate. Adding a path to diffBoundary grants no permission.

## Exact diff and line count

Let T be the target checkout's HEAD commit. Let B be its merge-base with the implementer worktree's HEAD. Current conformance stores B as `r.boundaryBase` (`internal/validate/conformance.go:232-241`).

Capture one repository-level `gittree.Workspace{Dir: r.workspace}.Snapshot("HEAD")` as R. Derive the project tree with `r.projectWorkspace().TreeOf(R)`. Use B's repository tree and R for repository paths and numstat. Use B's project tree and R's project subtree for the canonical patch, project paths, and reviewedTree.

TreeOf can peel the installation subtree from a supplied tree (`internal/gittree/gittree.go:199-228`). Snapshot seeds an isolated index from HEAD and runs add -A (`internal/gittree/gittree.go:236-276`). Its candidate therefore includes committed changes since B, then the current working-tree contents of tracked and untracked unignored files. A staged version superseded by an unstaged edit is not counted twice. Tracked ignored files remain included. Mode, symlink, gitlink, and binary changes remain changed paths. Retain these existing snapshot semantics.

Replace review's current two Snapshot calls (`internal/validate/conformance.go:368-376`) with the single repository snapshot for both bounded and headerless review. No second candidate capture may supply any review fact.

Add in `internal/gittree/numstat.go`:

```go
func (w Workspace) ChangedLines(fromTree, toTree string) (int64, error)
func changedLinesFromNumstat(data []byte) (int64, error)
```

Use Workspace.git to run:

```text
git diff --numstat -z --no-renames --no-ext-diff --no-textconv --no-color --ignore-submodules=none <repository-tree-of-B> <R> --
```

The runner already pins configuration, scrubs Git steering variables, and bounds execution (`internal/gittree/gittree.go:90-99`, `internal/gittree/gittree.go:121-141`). Do not add another production Git runner.

Parse complete NUL-terminated records, splitting only at the first two tabs. An empty output means zero. Require a nonempty pathname and two nonnegative decimal counts, or exactly `-\t-` for a binary row. Skip only that binary row form. Add both counts with checked int64 arithmetic. Reject incomplete records, malformed counts, and overflow. A Git failure is an error, never zero.

Renames count as deletion plus addition. This follows the existing no-renames diff and path operations (`internal/gittree/gittree.go:362-376`). Binary contents cost zero text lines, while their paths remain bounded. Count only when a valid pair is present. A headerless review makes no numstat call.

The prose waiver separately sums lines and refuses binary or uncountable input (`internal/validate/conformance.go:617-637`). Tier 1 separately limits delivery to three files and 40 changed lines (`internal/landing/observe.go:1286-1290`). Change neither policy.

## Review enforcement and refusal text

Add in `internal/validate/brief_bounds.go`:

```go
type BriefBoundaryViolation struct {
    Paths []string
    ChangedLines int64
}
type BriefCeilingViolation struct {
    Paths []string
    ChangedLines, Ceiling int64
}
func (e *BriefBoundaryViolation) Error() string
func (e *BriefCeilingViolation) Error() string
func (r *conformanceRun) briefBoundsViolations(
    bounds dispatch.BriefBounds, repositoryPaths []string, changedLines int64,
) ([]error, error)
```

The second return reports inability to evaluate, including a malformed projected declaration. Preserve concrete parser and violation types until rendering. Exact stderr examples are:

```text
conformance failure: BRIEF_BOUNDARY_EXCEEDED: changed paths outside Boundary: ["extra.txt"]; changedLines=1 additions plus deletions
conformance failure: BRIEF_CEILING_EXCEEDED: changedLines=3 exceeds Ceiling=2 additions plus deletions; changedPaths=["source.txt"]
```

Boundary.Paths includes every actual repository path outside the brief. Ceiling.Paths includes every changed repository path, including binary paths. Both carry the total text count for the repository candidate. Sort and deduplicate diagnostic arrays, encode as JSON, and never truncate. If both checks fail, report Boundary first and Ceiling second. Existing project and cumulative policy diagnostics follow.

Read bounds before evaluation. Compute paths and the count from the fixed trees, then collect policy failures before either success artifact is written. For bounded review, a sibling yields both the typed Boundary failure and the existing project-fence failure. It also contributes to the candidate count. The whole-project declaration cannot hide it. Keep the existing cumulative check over project paths, including its protected plans and control-plane rules (`internal/validate/conformance.go:700-725`, `internal/validate/conformance.go:798-817`).

For headerless review, preserve the existing outside-project-first refusal and existing immutable-evidence precedence. Current outside-project refusal precedes the cumulative check (`internal/validate/conformance.go:377-400`). The count branch is absent on this path.

Use `conformance failure: BRIEF_BOUNDS_UNREADABLE: <detail>` for source failures and `conformance failure: BRIEF_LINES_UNREADABLE: <detail>` for count failures. Parser errors retain `BRIEF_BOUNDS_INVALID`. Stop immediately when facts cannot be computed. These failures return exit 1 and write no new success artifacts. Do not fabricate a count to complete a policy message.

Evaluate actual changed paths. Do not require diffBoundary to be contained in Boundary. An unchanged extra return declaration is harmless if its existing dialect is valid. An actual changed path inside Boundary must still appear in the cumulative concrete return declaration. An actual changed path outside Boundary refuses even if the return declares it.

Successful review.json keeps exactly diffArtifact, implementerJob, and reviewedTree. Current writing and shape tests establish that contract (`internal/validate/conformance.go:403-407`, `internal/validate/conformance_review_shape_test.go:10-32`). Identical repeat review reuses existing bytes (`internal/validate/conformance.go:413-419`). On a bounded failure with prior evidence, emit the typed failure before the existing immutable-review diagnostic and preserve both files. The old generic failure closure currently masks new failure detail when a review exists (`internal/validate/conformance.go:361-365`); bounded failures need an explicit path around that masking. Successful changed evidence still refuses overwrite.

Register `BRIEF_BOUNDS_INVALID`, `BRIEF_BOUNDS_UNREADABLE`, and `BRIEF_LINES_UNREADABLE` as Question. `BRIEF_AUTHORITY_REFUSED` is the existing precedent (`internal/refusal/register.go:80`). Register the two exceeded codes as Agent with remedy `dispatch.sh follow-up with a corrected brief or candidate, then validate conformance --stage review` and Commands 2. Agent rows need a remedy or named defect (`internal/refusal/register_test.go:71-86`). No new approval step is introduced.

## The seat's pre-read step

From the merge-target installation, after the builder returns, run:

```sh
metasystem validate conformance --stage review --job <returned-implementer-job>
```

Read stderr and the exit code before dispatching the critic. Exit 0 permits that read. Exit 1 stops it and returns the violations for correction. Exit 2 means repair the seat's command. The CLI already relays these outcomes (`cmd/metasystem/validate_verbs.go:276-299`, `cmd/metasystem/validate_verbs.go:343-352`). Conformance refuses invocation from the implementer checkout (`internal/validate/conformance.go:227-230`).

Review resolves the implementer and enters reviewStage without a critic record (`internal/validate/conformance.go:139-193`, `internal/validate/conformance.go:214-249`). The existing fixture does the same (`scripts/agents/conformance-fixtures.sh:175-179`). No critic placeholder is needed.

Place this binding sentence in both `docs/orchestration.md` at implementation-to-critique and `skills/code-critique/SKILL.md` at Layer 1:

> Before dispatching a code critic, the seat must run `metasystem validate conformance --stage review --job <returned-implementer-job>` from the merge-target installation, read stderr, and obtain exit 0; exit 1 returns the violations for correction, and exit 2 requires fixing the command.

Those current instruction sites are `docs/orchestration.md:36-42` and `skills/code-critique/SKILL.md:25-31`. Replace the skill's claim that review checks only return declarations. Keep the critic's semantic review of acceptance criteria, non-goals, unrelated work, and proof. The critic consumes the canonical patch and reviewedTree and can rerun identical review.

`TestBriefBoundsSeatOrderingInstructions` reads each named section independently and requires that command, actor, ordering, and exit mapping. Its remove-one runs delete the binding sentence from one owner at a time. This proves the shipped instruction exists. It does not prove that an arbitrary seat obeyed it. The seat's actual pre-read command result is the execution evidence.

This design adds no automatic critic-launch gate. Missing review artifacts currently return an absent read subject in some cases (`internal/dispatch/read_subject_compute.go:24-26`, `internal/dispatch/read_subject_compute.go:134-146`). Keep that separate admission behavior.

## Reader inventory and seams

| Current reader or writer | Decision and reason |
| --- | --- |
| BriefMode and authority extraction, `internal/dispatch/brief.go:41-104`, `internal/dispatch/brief.go:121-171` | Change admission and bounded citation handling. Mode and existing input precedence remain. |
| Brief-mode CLI, `cmd/metasystem/dispatch_verbs.go:1993-2027` | Keep flags, mode stdout, and authority-only behavior; test the new errors through this relay. |
| Shell preflight, `scripts/agents/dispatch.sh:858-864`, `scripts/agents/dispatch.sh:1542`, `scripts/agents/dispatch.sh:1655`, `scripts/agents/dispatch.sh:2643` | Change the initial suffix. Initial and follow-up routes use Go; add no shell parser. |
| Composition production, `internal/dispatch/composition.go:268`, `internal/dispatch/composition.go:358-390` | Keep source and reference format. Consume it with explicit binding checks. |
| Composition admission, `internal/dispatch/build.go:1042-1077`, `internal/dispatch/build.go:1099-1103` | Keep the existing admission validator. Its source/reference facts also bind review's narrow reader. |
| Successor task direction and mission stream, `internal/validate/conformance.go:914-960`, `internal/validate/conformance.go:827-839` | Leave exhaustion and stream policy alone. Neither chooses review bounds. |
| Review and projection, `internal/validate/conformance.go:88-107`, `internal/validate/conformance.go:276-428` | Change literal path projection, candidate capture, and bounded review evaluation in their owning units. Keep the project fence. |
| Cumulative return check and recertification, `internal/validate/conformance.go:724-819`, `internal/validate/recertification.go:809`, `internal/validate/recertification.go:1182` | Keep exact declarations and recertification policy. New brief limits enter reviewStage only. |
| Follow-up overlap and design pairing, `internal/dispatch/followup_rebase.go:123-149`, `internal/dispatch/review_reference.go:313-325` | Leave concrete return path readers unchanged. A permission pattern cannot replace a changed file or an exact design identity. |
| Return normalization, `internal/validate/returncomplete.go:239-302` | Leave spelling repair unchanged. It does not own permission or the actual diff. |
| Cap paragraph, `internal/dispatch/capcontinuation.go:89` | Clarify cumulative candidate bounds and concrete return declarations. |
| Implementer role and schema, `scripts/agents/roles/implementer.md:3-13`, `scripts/agents/schemas/implementer.schema.json:7`, `scripts/agents/schemas/implementer.schema.json:38` | Clarify the role text. Preserve schema and concrete diffBoundary. |
| Brief and follow-up templates, `scripts/agents/templates/brief.md:1-36`, `scripts/agents/templates/follow-up.md:1-18` | Explain paired optional headers, inert examples, and repeating the whole candidate's limits. |
| Selected instruction owners, `scripts/agents/role-packets.json:40-43`, `scripts/agents/role-packets.json:54-59` | Keep the recipe. Update the instruction documents it already selects. |
| Fake adapter, `internal/adapter/fake.go:109-112`, `internal/adapter/fake.go:137-167` | Leave simulated returns and mode discovery alone. Admission owns bounds. |
| Fake reference discovery and tamper hook, `scripts/agents/adapters/fake.sh:62-67`, `scripts/agents/adapters/fake.sh:231-236` | Leave alone. They read or alter reference evidence for fixtures, not header semantics. |
| Template mode emitters, `scripts/agents/fingerprint-harness.sh:187`, `scripts/agents/adapters/fake.sh:449-453`, `internal/adapter/selftestrun.go:135`, `internal/steward/stage.go:102`, `internal/steward/stage.go:155` | Leave alone. Mode-only callers remain valid; templates get no live optional placeholders. |
| Static header and return audits, `scripts/validate-metasystem.sh:1614-1630`, `scripts/validate-metasystem.sh:1967-2010` | Leave existing mandatory header and return tests alone. New Go tests cover the added instruction contract. |
| Review CLI help, `cmd/metasystem/validate_verbs.go:280-352` | Explain the paired limits and pre-read use. Preserve flags and output relay. |
| Waiver and tier-1 limits, `internal/validate/conformance.go:617-637`, `internal/landing/observe.go:1286-1290` | Keep independent policy. Broad Boundary does not grant a waiver. |

The later member-size goal owns pre-build file counts (`plans/goals/member-size-gate.md:8`). A directory or glob here can cover many files, so it is not a concrete member inventory. The later reader-inventory goal owns Readers and Layers on (`plans/goals/cross-cutting-change-inventories-its-readers.md:8-10`). Boundary stays output permission. These are seam dispositions, not additional mechanisms in this goal.

## Exact fixture legs

Extend the existing `section/conformance-fixtures` group (`testing.json:75`). Its `new_case`, `write_implementer`, and `expect_failure` helpers already provide isolated worktrees, root briefs, and exit-1 assertions (`scripts/agents/conformance-fixtures.sh:38-96`, `scripts/agents/conformance-fixtures.sh:153-169`).

Each row below uses a fresh case. Use the real public verb. Create no critic. Apply bounded headers to the frozen root brief after write_implementer writes its default. The fixture uses a root installation, so its repository paths are unprefixed.

| Leg | Candidate and brief | Required result |
| --- | --- | --- |
| `brief-boundary-unlisted` | Add untracked extra.txt containing `extra\n`. Declare extra.txt in the return. Boundary `["source.txt"]`, Ceiling 10. | Review exits 1 with the exact Boundary example above. No diff.patch or review.json. |
| `brief-ceiling-exceeded` | Append three lines to source.txt. Declare source.txt. Boundary `["source.txt"]`, Ceiling 2. | Review exits 1 with the exact Ceiling example above and count 3. No success artifacts. |
| `brief-both-admitted` | Append two lines to source.txt. Declare source.txt. Boundary `["source.txt"]`, Ceiling 2. | Exit 0 with both artifacts, exactly three review fields, and the correct project tree. |
| `brief-no-headers` | Append 401 lines to source.txt and add a one-line extra.txt. Declare both. Neither header. | Exit 0 despite 402 changed lines. |
| `brief-no-headers-undeclared` | Repeat the preceding candidate in a fresh case, declaring only source.txt. | Today's cumulative-boundary refusal names extra.txt. |
| `brief-missing-ceiling` | A Working Mode brief with Boundary `["source.txt"]` only. | Public `job brief-mode` exits 1 with `BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary`. No implementer or critic is dispatched. |
| `brief-missing-boundary` | A Working Mode brief with Ceiling 2 only. | Public `job brief-mode` exits 1 with `BRIEF_BOUNDS_INVALID: Boundary: required with Ceiling`. No implementer or critic is dispatched. |

These are seven executions: four DONE outcomes, the headerless guard, and both partial-pair refusals. The partial-pair Go test is `TestBriefBoundsRejectsPartialHeaders`, with Boundary-only and Ceiling-only cases; `TestDispatchBriefBoundsAdmission` proves both public admission routes refuse too.

Use real composed inline and referenced inputs in Go source-reader tests. Do not let legacy fixture convenience stand in for those tests. The existing section runs the bed through the engine (`scripts/validate-metasystem.sh:1129-1131`). Add no bed, runner, testing group, or test policy.

## Builder units and ordered proof

Land in this order: **1a, 1b, 2, 3, 4a, 4b, 4c, 6a, 6b**. The old test-only unit 5 is removed. Its review witnesses live in units 4b and 4c; its public CLI witness lives in 6b. Each unit owns the production behavior its remove-one runs change.

Each unit starts a fresh implementation chain from the preceding landed unit. Its 400-line ceiling includes additions plus deletions in production, tests, scripts, and prose. Do not combine several unlanded units into one supposedly small candidate. The allocations below are estimates with headroom. If an allocation cannot fit, stop and propose another split before exceeding 400 or touching an unlisted path. The seat owns integration receipts outside the returned builder patch.

Before enforcement lands in 4c, the seat measures the candidate manually. Afterwards it also uses the paired brief gate. The builder never commits, executes fixture beds, exports METASYSTEM_BIN for a Go run, or changes the provided GOCACHE, GOTMPDIR, or STATICCHECK_CACHE. The builder/seat proof split and cache rules are already stated in `internal/dispatch/build.go:1126` and `docs/orchestration.md:172`.

For each row assigned to a unit below, run its named focused test with only that rule disabled. Keep the mutation inside that unit's Boundary. Observe the specified assertion fail, restore the rule, and observe the test pass. A compile failure is not a witness. A broader mutation that disables another refusal is not a witness. Use an existing guard test when it directly detects the removed integration rule; do not mutate an untouched dependency outside the unit. Instruction witnesses remove only the specified sentence from an instruction file inside that unit.

Report rule id, mutated file, exact command, observed assertion failure, and restored pass. No mutation remains in the return. Then, from `metasystem/`, run this order with the package and shell lists given per unit:

```sh
go build ./...
go vet <unit packages>
go test -race -count=1 <unit packages>
bash -n <unit shell files>
scripts/agents/go-gate.sh --fast
```

Omit bash only for units with no shell files. The fast gate is an edit-loop check, not landing proof (`scripts/agents/go-gate.sh:97-103`). The seat runs the risk-selected public testing plan on the candidate and retains the result.

### Unit 1a: Parse the paired headers

```text
Working Mode: implement
Boundary: ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_bounds_test.go", "metasystem/internal/refusal/register.go", "metasystem/internal/refusal/brief_bounds_register_test.go"]
Ceiling: 400
Non-goals: no authority lexer change, review wiring, shell change, schema change, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 130 production, 180 test, and 25 register/test lines, total 335. Add the parser and reader, pair invariant, path syntax, BriefMode call, and invalid-header register row. Own H1-H7 and the invalid-code case of D1 below. Packages: `./internal/dispatch ./internal/refusal`. Shell files: none.

### Unit 1b: Preserve authority identity through admission

```text
Working Mode: implement
Boundary: ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go", "metasystem/cmd/metasystem/dispatch_verbs.go", "metasystem/cmd/metasystem/dispatch_brief_bounds_test.go", "metasystem/scripts/agents/dispatch.sh"]
Ceiling: 400
Non-goals: no parser-policy redesign, diff calculation, review enforcement, new CLI flags, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 110 production, 230 test, and 10 shell lines, total 350. Add the authority parser call, live and inert line handling, structured path spans, and shell suffix. Own A1-A5 and H8. Keep the CLI relay unless a minimal correction is required by its named test. Packages: `./internal/dispatch ./cmd/metasystem`. Shell: `scripts/agents/dispatch.sh`.

### Unit 2: Count fixed-tree text changes

```text
Working Mode: implement
Boundary: ["metasystem/internal/gittree/numstat.go", "metasystem/internal/gittree/numstat_test.go"]
Ceiling: 400
Non-goals: no snapshot-policy change, conformance wiring, waiver refactor, second Git runner, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 80 production and 250 test lines, total 330. Own N1-N5. Use Workspace.git. Packages: `./internal/gittree`. Shell files: none.

### Unit 3: Bind the supplied round's source

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/brief_bounds_source.go", "metasystem/internal/validate/brief_bounds_source_test.go"]
Ceiling: 400
Non-goals: no review enforcement, new artifact schema, full composition-admission rewrite, exhaustion-reader change, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 125 production and 245 test lines, total 370. Own S1-S6. Use table-driven corrupt-source cases and small real ComposeRolePacket fixtures with their required recipe inputs copied into temporary fixture roots. Do not use the exhaustion fixture's reduced composition object as production evidence. It currently writes only references (`internal/validate/conformance_test.go:118-123`). Packages: `./internal/validate`. Shell files: none.

### Unit 4a: Match projected paths and build typed violations

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/brief_bounds.go", "metasystem/internal/validate/brief_bounds_test.go", "metasystem/internal/validate/conformance.go", "metasystem/internal/refusal/register.go", "metasystem/internal/refusal/brief_bounds_register_test.go"]
Ceiling: 400
Non-goals: no review-stage wiring, changed waiver policy, recertification change, schema change, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 115 production, 225 test, and 25 register/test lines, total 365. Own P1-P5, V1-V3, and D1's two exceeded-code cases. Change only installationPath in conformance.go. Register the two exceeded codes when their types land. Packages: `./internal/validate ./internal/refusal`. Shell files: none.

### Unit 4b: Capture one review candidate and prove its seams

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/conformance.go", "metasystem/internal/validate/conformance_brief_snapshot_test.go", "metasystem/internal/validate/conformance_brief_git_test.go"]
Ceiling: 400
Non-goals: no bounds enforcement yet, changed snapshot contents, production test hooks, new Git runner, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 60 production and 310 test/helper lines, total 370. Own C1-C3. Rewire review to one repository snapshot and its project subtree. Reuse the existing real-worktree fixture (`internal/validate/conformance_test.go:48-85`).

Add a test-only Git interposer in Go. A temporary shell shim may only exec the Go helper process. Resolve real Git before setting PATH. The helper forwards arguments and records operations for the chosen worktree. After the first successful write-tree, it preserves that stdout and changes the fixture worktree before returning it. A second Snapshot is therefore observable, even if all its contents would otherwise match. The helper also supports refusing only numstat for unit 4c. No production hook is added.

Packages: `./internal/validate`. Shell files: none in the returned patch.

### Unit 4c: Enforce limits before writing evidence

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/conformance.go", "metasystem/internal/validate/conformance_brief_bounds_test.go", "metasystem/internal/refusal/register.go", "metasystem/internal/refusal/brief_bounds_register_test.go"]
Ceiling: 400
Non-goals: no new path dialect, merge or recertification policy change, extra success fields, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 85 production, 285 test, and 20 register/test lines, total 390. Own E1-E7 and D1's two unreadable-code cases. Integrate the source reader, fixed-tree count, typed checks, collected bounded diagnostics, and immutable evidence behavior. Reuse unit 4b's interposer without changing it. All mutation targets for these integration rules are in conformance.go. Packages: `./internal/validate ./internal/refusal`. Shell files: none.

### Unit 6a: Publish the seat and builder instructions

```text
Working Mode: implement
Boundary: ["metasystem/scripts/agents/templates/brief.md", "metasystem/scripts/agents/templates/follow-up.md", "metasystem/scripts/agents/roles/implementer.md", "metasystem/skills/code-critique/SKILL.md", "metasystem/docs/orchestration.md", "metasystem/internal/dispatch/capcontinuation.go", "metasystem/internal/dispatch/capcontinuation_test.go", "metasystem/internal/dispatch/brief_bounds_instructions_test.go"]
Ceiling: 400
Non-goals: no automatic critic-launch gate, new mandatory default, role-packet or schema change, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 125 instruction changed lines, 15 cap-message lines, and 180 test lines, total 320. Own W1-W4. Use the pre-read sentence above in both seat instruction owners. Use these short sentences for the remaining instruction witnesses:

- Both templates: "Declare Boundary and Ceiling together, or omit both; a single header refuses admission." Each carries an indented, paired example.
- Follow-up template: "Repeat both headers in each bounded follow-up; they cover every change still awaiting landing, including earlier rounds."
- Implementer role: "Boundary and Ceiling limit the whole candidate; diffBoundary lists concrete changed paths and grants no extra permission."
- Critic skill: "The critic still checks acceptance criteria, non-goals, unrelated work, and the quality of the proof inside the permitted files."
- Cap paragraph: "The predecessor wrote no return diffBoundary. The current brief's Boundary and Ceiling, when present, cover the whole candidate; diffBoundary grants no extra permission." Retain its requirement to list every changed path of the chain.

Extend the current cap paragraph test (`internal/dispatch/capcontinuation_test.go:55-76`). Packages: `./internal/dispatch`. Shell files: none.

### Unit 6b: Prove the public verbs and existing bed

```text
Working Mode: implement
Boundary: ["metasystem/scripts/agents/conformance-fixtures.sh", "metasystem/cmd/metasystem/validate_verbs.go", "metasystem/cmd/metasystem/conformance_brief_bounds_test.go"]
Ceiling: 400
Non-goals: no new fixture bed or testing group, underlying limit-rule changes, extra CLI flags, builder fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 120 fixture-plumbing lines, 15 help lines, and 210 Go test lines, total 345. Own W5-W6. Add the seven legs above and public-verb tests without a critic. Add this help sentence: "Review also enforces the current brief's paired Boundary and Ceiling before a code read; omitting both keeps the existing checks." The Go CLI test lives with the permitted CLI relay and help source. It does not mutate a limit rule in validate.

Packages: `./cmd/metasystem`. Shell: `scripts/agents/conformance-fixtures.sh`. Also run the unchanged-reader seam command `go test -race -count=1 ./internal/adapter ./internal/dispatch ./internal/gittree` after the unit package test and before bash and the fast gate.

The builder syntax-checks the bed. The seat executes `section/conformance-fixtures` and reads all seven outcomes, then the risk-selected dispatch proof including `section/dispatcher-adapter-and-mission-runner-fixtures` (`testing.json:96`). The seat uses its enrolled engine and retained structured results.

### Complete rule-to-witness table

Each slash case below is a separately named subtest and a separate remove-one run when it encodes a distinct rule. Positive admission cases detect removal of an allowance or an overbroad refusal. Refusal cases detect removal of a guard. Existing dependencies stay intact while the owning unit's integration rule is removed. Table rows cover the feature's rules, including its instruction surfaces; historical facts and explicit non-goals add no new rule.

| Rule id and rule | Owning unit and mutation target | Test that fails when only that rule is removed |
| --- | --- | --- |
| H1: exact case, column zero, one physical line, trimming of values, live fenced headers | 1a, brief.go scanner | `TestBriefBoundsHeaderRecognition/case`, `/column`, `/physical-line`, `/trim`, `/fence`. Assert parsed values as well as refusals. |
| H2: both or neither; no successful partial pair | 1a, pair check | `TestBriefBoundsRejectsPartialHeaders/boundary-only` and `/ceiling-only` require the exact missing-header error; `TestBriefBoundsPresence/neither` and `/both` guard both allowed forms. |
| H3: empty array differs from absence; duplicate members allowed | 1a, array decoding | `TestBriefBoundsPresence/empty-array` asserts non-nil empty Boundary; `/duplicates` admits repeated members. |
| H4: Boundary JSON shape and duplicate-header refusal | 1a, parser checks | `TestBriefBoundsSyntax/empty`, `/null`, `/object`, `/nonstring`, `/trailing-json`, `/duplicate-boundary` assert their typed error. |
| H5: decimal Ceiling grammar, int64 range, zero and leading zeroes | 1a, number parsing | `TestBriefBoundsCeilingSyntax/sign`, `/fraction`, `/suffix`, `/placeholder`, `/empty`, `/overflow`, `/duplicate`, `/zero`, `/leading-zeroes`, `/maximum`. |
| H6: component validation; directory branch bypasses glob validation; other declarations validate glob syntax | 1a, path syntax | `TestBriefBoundsPathSyntax/absolute`, `/empty`, `/nul`, `/empty-component`, `/dot`, `/parent`, `/member-whitespace`; the last case preserves leading and trailing member spaces; `TestBriefBoundsTrailingSlashIsLiteral` admits `metasystem/a[/` and refuses `metasystem/a[`. |
| H7: deterministic parser errors; required mode preserved; readable-file wrapper; bounds run after valid mode | 1a, parser/BriefMode/ReadBriefBounds | `TestBriefBoundsErrorPrecedence`; `TestBriefBoundsPreservesWorkingMode`; `TestReadBriefBounds/read` and `/missing`; `TestBriefModeRejectsInvalidBounds`. Compare exact error text and use errors.As. |
| H8: public normal and authority-only admission relay bounds; mode stdout remains mode only; suffix describes bounds | 1b, authority call and CLI/shell relay | `TestDispatchBriefBoundsAdmission/normal`, `/authority-only`, `/mode-stdout`, `/authority-before-mode`; `TestDispatchBriefBoundsFailureSuffix` checks the shell's failure suffix. |
| A1: Boundary permission alone performs no authority existence lookup or fragment lookup | 1b, live-line exclusion | `TestBriefAuthorityBoundaryIsOutput/file`, `/directory`, `/glob`, `/special-name` admit absent outputs. |
| A2: exact indented example is inert, even with missing or placeholder paths; no exemption leaks to other lines | 1b, inert-line exclusion | `TestBriefAuthorityIndentedBoundsExampleIsInert/spaces`, `/tabs`, `/headerless`, `/separate-input`; the last case must still refuse the cited input. |
| A3: structured special-character inputs share decoded identity and emit no fragments | 1b, span decoder | `TestBriefAuthoritySpecialCharacterInput/space`, `/comma`, `/quote`, `/tab`, `/newline`, `/backslash`. Exercise JSON strings and backticks where representable. Each case asserts the full missing path, then admission after committing that exact file. |
| A4: separate input wins, including a root file and a member under an absent directory | 1b, decoded identity eligibility and input accounting | `TestBriefAuthorityBoundaryInputStillRequired/ordinary`, `/root-file`, `/new-directory`, `/output-then-input`, `/input-then-output`. |
| A5: preserve old unquoted, Workspace/Create, and artifact lookup behavior; no structured reinterpretation of ordinary headerless prose | 1b, bounded lexer branch | `TestBriefAuthorityBoundedCitationCompatibility/unquoted`, `/workspace`, `/create`, `/artifact`, `/headerless`. Use the current lookup owner, not a new file probe. |
| N1: additions plus deletions; empty diff zero; only binary rows excluded | 2, numstat parser | `TestChangedLinesAddsAndDeletes`, `TestChangedLinesEmpty`, `TestChangedLinesExcludesBinary`. |
| N2: NUL records and first-two-tab split; nonempty path; malformed and overflow errors | 2, numstat parser | `TestChangedLinesOddFilenames`; `TestChangedLinesRejectsMalformedAndOverflow/missing-nul`, `/missing-path`, `/count`, `/mixed-binary`, `/overflow`. |
| N3: rename delete/add endpoints; all supplied trees and nested workspace path space | 2, ChangedLines invocation | `TestChangedLinesRenameCountsBothEndpoints`; `TestChangedLinesUsesSuppliedTrees/root` and `/nested`, with a later worktree edit that must not alter the count. |
| N4: use the declared Git command pins for external diff, text conversion, color, and submodules | 2, ChangedLines flags | `TestChangedLinesCommandPins` records Git argv through a temporary Go helper and requires each declared pin independently. `TestChangedLinesHostileDiffConfig` also checks real counts under hostile configuration. Removing a pin fails the invocation assertion even when Git would produce the same numstat without it. |
| N5: Git failure propagates through the existing runner | 2, error return | `TestChangedLinesGitFailure` requires an error for an invalid tree, never zero success. |
| S1: supplied job and round select the source; only caller:brief task-direction; no other packet slot | 3, source selector | `TestReviewBriefBoundsInlineSourceOnly`; `TestReviewBriefBoundsRoundIsolation/current`, `/later`, `/headerless-follow-up`, `/partial-follow-up`; `TestReviewBriefBoundsRejectsCorruptSource/job`, `/round`, `/missing-source`, `/duplicate-source`, `/wrong-source`. |
| S2: delivered range and digest must bind the selected prompt section | 3, range checks | `TestReviewBriefBoundsRejectsCorruptSource/range` and `/delivered-digest`. |
| S3: inline source bytes, digest, and envelope bind the body; missing reference cannot parse a stub | 3, inline extraction | `TestReviewBriefBoundsInlineIdentity/bytes`, `/digest`, `/envelope`, `/missing-reference`, `/no-final-newline`. |
| S4: exactly one reference joined by slot, digest, byte count and Path/OpenPath | 3, reference selection and join | `TestReviewBriefBoundsRejectsUnboundReference/slot`, `/digest`, `/bytes`, `/path`, `/duplicate`. Slot case supplies only a differently slotted reference with otherwise matching source identity and fails when the slot selector alone is removed. Digest case uses a different, valid, self-consistent referenced body. Byte case changes only the source byte count. |
| S5: referenced bytes must pass root, regular-file, length, and digest verification | 3, ReadVerifiedReference call | `TestReviewBriefBoundsReferencedSource`; `TestReviewBriefBoundsRejectsCorruptReference/escape`, `/nonregular`, `/bytes`, `/digest`. Nonregular uses a symlink to a valid body inside the root, so a plain read would succeed. Removing only the verification call must admit a bad body and fail its test. |
| S6: selected legacy root/follow-up fallbacks; absent differs from unreadable; bad present composition cannot fall back; typed syntax preserved | 3, fallback and error returns | `TestReviewBriefBoundsLegacy/root`, `/follow-up`, `/absent`; `TestReviewBriefBoundsReadFailure/present` and `/composition`; `TestReviewBriefBoundsPreservesSyntaxType`. |
| P1: nested declarations require prefix and strip once; root declarations are unchanged | 4a, projectDeclaration integration | `TestBriefBoundaryNestedPathProjection/root`, `/nested`, `/missing-prefix`, `/double-prefix`. |
| P2: guard siblings before installationPath; whole-project directory matches only inside | 4a, raw-path guard and directory flag | `TestBriefBoundaryWholeProjectExcludesSibling` feeds both an inside path and a sibling and requires only the sibling in Boundary.Paths. |
| P3: installationPath preserves all Git filename bytes and removes exactly one literal prefix | 4a, installationPath | `TestInstallationPathPreservesGitIdentity/root`, `/nested`, `/backslash`, `/whitespace`, `/double-prefix`; rerun `TestNestedWaiverProtectsProjectPlans` as the existing caller guard (`internal/validate/nested_conformance_test.go:229`). |
| P4: exact case, directory depth and component edge, literal glob characters in directories, whole-path patterns, no recursive **, unmatched pattern | 4a, matcher | `TestBriefBoundaryMatching/exact`, `/case`, `/directory-depth`, `/directory-edge`, `/literal-bracket-directory`, `/glob`, `/double-star`, `/unmatched`, `/escaped-metacharacter`. |
| P5: empty boundary denies all actual paths, including binary paths | 4a, unmatched-path collection | `TestBriefBoundaryMatching/empty` and `TestBriefBoundaryBinaryPathStillBounded`. |
| V1: typed boundary error carries all offending repository paths and the total count | 4a, boundary construction | `TestBriefBoundaryViolationFields` uses errors.As and checks the complete fields, including a permitted file's text contribution. |
| V2: Ceiling compares with > and carries all changed paths | 4a, ceiling predicate/construction | `TestBriefCeilingViolation/below`, `/equal`, `/above`, `/zero`, `/binary-path` require exact fields. |
| V3: exact JSON diagnostics, sorting, deduplication, complete long list, Boundary before Ceiling | 4a, error formatting and violation order | `TestBriefBoundsViolationDiagnostics/json-escaping`, `/sorted-unique`, `/full-list`, `/both-order` compare complete output; full-list uses 257 paths including odd characters. |
| C1: one candidate Snapshot supplies every tree and patch | 4b, review capture | `TestReviewBriefSingleSnapshot` mutates after the first write-tree, requires exactly one snapshot, and checks the returned project tree and patch against that first captured repository tree. Two snapshots fail by count even without drift. |
| C2: merge-base remains the full candidate base; project output derives from the repository snapshot | 4b, review tree wiring | `TestReviewBriefUsesMergeBaseCandidate` separates baseSha, merge-base, and current HEAD, advances target on a disjoint branch, and checks the complete patch and repository path set; `TestNestedReviewStageSpeaksProjectSpace` is the existing guard (`internal/validate/nested_conformance_test.go:44-101`). |
| C3: preserve candidate contents and the real index through the new snapshot wiring | 4b, repository Snapshot call | `TestReviewBriefSnapshotMembership` includes committed, staged-then-unstaged, deleted, untracked, ignored, tracked-ignored, mode, symlink, and gitlink cases and compares the real index before/after. |
| E1: valid pair invokes source, paths, count and both limits before writes; partial or malformed bounds refuse | 4c, review integration | `TestReviewBriefBoundsEnforced/boundary`, `/ceiling`, `/both-admitted`, `/partial`, `/invalid` assert outcomes and absence of both artifacts on refusal. |
| E2: headerless path skips numstat and preserves old fence, declaration and immutable precedence | 4c, pair branch | `TestReviewBriefHeaderlessSkipsNumstat` makes the interposer fail on any numstat and asserts zero such calls; `TestReviewBriefHeaderlessCompatibility/fence`, `/declaration`, `/immutable` compare old diagnostics. |
| E3: sibling, every outside path, and total count survive collected diagnostics | 4c, collected refusal path | `TestReviewBriefNestedWholeRepository` permits `metasystem/`, changes an inside file and a sibling, and requires typed Boundary plus the project fence and the repository-wide count; `TestReviewBriefBothViolationOrder` asserts exact Boundary/Ceiling order through review. |
| E4: counted and reviewed trees remain identical; earlier rounds still count | 4c, ChangedLines arguments and current-round integration | `TestReviewBriefCountUsesCapturedTree` reuses the post-capture edit interposer; `TestReviewBriefFollowUpCountsEarlierChanges/ceiling` and `/boundary` prove a smaller current pair refuses the earlier candidate work. |
| E5: permission cannot replace concrete declarations or protected paths; unchanged declarations do not cost permission | 4c, retained cumulative call | `TestReviewBriefDeclarationIsNotPermission/unchanged`, `/actual-outside`, `/undeclared-inside`; `TestReviewBriefProtectedPathsStillRefuse/plans` and `/control-plane`. Mutate the retained call, not its earlier policy owner. |
| E6: source and count failures have exact review wrappers; typed parser failure remains named; no artifacts | 4c, failure rendering | `TestReviewBriefUnreadableWrappers/source` uses a directory as the source file; `/lines` fails only numstat; `/syntax` supplies a partial pair. Compare full stderr prefixes and artifact absence. |
| E7: bounded refusal preserves prior evidence and reveals typed reason; success has three fields; identical success reuses bytes | 4c, bounded failure and artifact branch | `TestReviewBriefLeavesEvidenceImmutable/first-refusal`, `/success-shape`, `/identical`, `/later-boundary`, `/later-ceiling`, `/later-source-error`. Check exact diagnostics and unchanged bytes. |
| D1: every new refusal code has the stated classification and remedy | 1a, 4a, 4c, each owning its register rows | `TestBriefBoundsRefusalRegistration` gains only the owning unit's code cases at that landing. Remove that row or change its Shape/Override/Commands to fail. Existing `TestHCL03EveryCodeRowed` also checks coverage (`internal/refusal/register_test.go:20-40`). |
| W1: seat command, actor, ordering, and exit mapping appear in both instruction owners | 6a, orchestration and code-critique text | `TestBriefBoundsSeatOrderingInstructions/orchestration` and `/code-critique`. Delete the binding sentence from one owner per run. |
| W2: templates teach pair-or-neither, inert examples, and whole-candidate follow-up repetition without live placeholders | 6a, both templates | `TestBriefBoundsTemplateInstructions/pair`, `/follow-up-cumulative`, `/example`. Each file is checked independently; run the parser/authority extractor over its examples and assert the instruction sentence is present. |
| W3: role distinguishes allowed scope from concrete return declaration; critic retains semantic review | 6a, implementer role and critic skill | `TestBriefBoundsRoleInstructions/permission` and `/semantic-review` check the short normative sentences in their owning sections. |
| W4: cap paragraph names predecessor's missing return declaration and current whole-candidate limits | 6a, capcontinuation.go | Extend `TestCapContinuationTextTellsTheFactAndWhatTheWorktreeHolds`. Removing either clarification fails its corresponding paragraph assertion. |
| W5: public review relay needs no critic, preserves 0/1/2, exact stderr, and help for paired limits | 6b, validate_verbs.go | `TestConformanceBriefCLIWithoutCritic/admit`, `/refuse`, `/usage`; `TestConformanceBriefBoundsHelp`. Remove only the relevant CLI relay or help clause for the witness. |
| W6: all seven fixture legs invoke the intended public verb and retain their required assertions | 6b, conformance-fixtures.sh | `TestConformanceBriefFixtureLegs` checks each named shell block for its verb, fixture data, expected status, and refusal/artifact assertions. Removing one leg or its key assertion fails. The seat then executes the real bed; this static witness alone is not runtime proof. |

The single-snapshot and no-numstat tests observe operations, not just final admission. The unbound-reference test supplies a valid alternative file so file verification alone cannot make it pass. The special-character tests check exact full identities. These are deliberate counterexamples to the missing witnesses in round 1.

## Critique record

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| BDRB-R1-01 | accepted | Declarations and changed paths currently use different spaces (`internal/validate/conformance.go:284-293`, `internal/gittree/gittree.go:372-386`). | Guard siblings, then call installationPath before matching; preserve the whole-project directory flag; P1-P3 and E3 name the witnesses. |
| BDRB-R1-02 | accepted | The seat binds both-or-neither to DONE (`plans/goals/brief-declares-the-round-boundary.md:8`). | Partial pairs refuse naming the missing header; H2/H8 and both missing-header fixture legs prove it. |
| BDRB-R1-03 | accepted | The old unit 5 could not mutate review production inside its test-only Boundary. | Remove unit 5; move its witnesses to production-owning units 4b, 4c, and 6b, each capped at 400 lines. |
| BDRB-R1-04 | accepted | The old named tests did not observe all stated rules. | Add the complete rule table, operation-observing snapshot/count tests, exact diagnostic tests, and instruction witnesses in their owning units. |
| BDRB-R1-05 | accepted | ReadVerifiedReference does not join source metadata (`internal/dispatch/references.go:24-49`); admission already does (`internal/dispatch/build.go:1066-1077`). | Require slot, digest, byte-count and path binding; add TestReviewBriefBoundsRejectsUnboundReference. |
| BDRB-R1-06 | accepted | Current extraction scans indented lines as ordinary authority (`internal/dispatch/brief.go:128-160`). | Define the inert indented-header form and skip only its own line; add TestBriefAuthorityIndentedBoundsExampleIsInert. |
| BDRB-R1-07 | accepted | Current token grammar cannot preserve special-character members (`internal/dispatch/brief.go:13-18`). | Share decoded byte identity, consume structured citations without fragments, retain separate input use, and add TestBriefAuthoritySpecialCharacterInput. |
| BDRB-R1-08 | accepted | Current exact return matching supplies no directory/glob precedent (`internal/validate/conformance.go:798-810`). | Trailing-slash declarations are literal and bypass glob validation; TestBriefBoundsTrailingSlashIsLiteral fixes the accepted and refused examples. |
| BDRB-R1-N01 | noted, non-material | The omitted admission and fake reference readers were checked (`internal/dispatch/build.go:1099-1103`, `scripts/agents/adapters/fake.sh:67`, `scripts/agents/adapters/fake.sh:231-236`). | No separate production change; the binding change belongs to R1-05, and the inventory records these seams. |
| BDRB-R1-N02 | noted, non-material | The old four outcomes included a fifth isolated guard case (`scripts/agents/conformance-fixtures.sh:38-64`). | No separate behavior change; the table now names seven executions after the required pair-refusal additions. |

All eight material identifiers have accepted dispositions and named amendments. Both non-material identifiers are retained. No finding is refuted by restating revision 1. This fold does not declare critique closure or consume another review round.

## Completion obligations and remaining choice

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| BDRB-1 | HIGH | Header contract; Paths and matching | Paired syntax, exact identity, inert examples, and legacy admission | dispatch | brief.go | H1-H8, A1-A5 | Missing-header legs and public admission Go tests | PARTIAL | Build 1a/1b and retain mutation evidence |
| BDRB-2 | HIGH | Which brief binds a round | Supplied current source binds the pair | validate | brief_bounds_source.go | S1-S6 | Real composed inline and referenced fixtures | PARTIAL | Build 3 and verify source joins |
| BDRB-3 | HIGH | Paths and matching; Review enforcement | Refuse every actual outside path with its count | validate | brief_bounds.go and reviewStage | P1-P5, V1-V3, E1/E3/E5 | brief-boundary-unlisted | PARTIAL | Build 4a/4c and run seat bed |
| BDRB-4 | HIGH | Exact diff and line count | One full merge-base candidate; binary-excluding text count | gittree and validate | ChangedLines and reviewStage | N1-N5, C1-C3, E2/E4 | brief-ceiling-exceeded | PARTIAL | Build 2/4b/4c and inspect operation witnesses |
| BDRB-5 | HIGH | Review enforcement and refusal text | Preserve prior policy and evidence; diagnose bad inputs | validate and refusal | reviewStage and refusal register | E2/E5/E6/E7, D1 | Both-admitted and both headerless legs | PARTIAL | Build 4c and retain exact error/artifact results |
| BDRB-6 | HIGH | The seat's pre-read step | Seat obtains exit 0 before critic dispatch | orchestration and CLI | instruction owners and existing review verb | W1-W6 | Seat command result before the read; seven bed outcomes | PARTIAL | Build 6a/6b and execute seat proof |

No open question belongs to Wido. A default for missing headers would change DONE and is not proposed here.

Design verification: re-read the goal, both input reports, the current implementation and instruction owners, referenced tests, and fixture registration. Joined all eight material finding ids to the dispositions above. Checked each proposed rule's mutation target against its unit Boundary and each planned allocation against 400. No implementation test, Go gate, fixture bed, or mutation experiment was run for this document-only deliverable. All runtime obligations remain PARTIAL.

Failure behavior is explicit: malformed headers refuse admission; unreadable or unbound evidence and failed counting refuse review; policy failures name every offending path and the total count; prior success artifacts stay unchanged. Existing Git execution timeouts remain owned by the bounded runner (`internal/gittree/gittree.go:128-141`). The remaining implementation risk is fitting the planned tests and production changes within each hard ceiling; a builder must split before exceeding it.

This delegate writes only `artifacts/reports/bdrb-design-r2.md`. The seat owns landing the replacement, updating proof status, and any receipt.

Proposed receipt for the integrating seat: `DESIGN brief-declares-the-round-boundary revision 2: accepted all eight material findings; paired headers, exact path projection and authority identity, bound source references, complete mutation witnesses, and nine separately landing units capped at 400 changed lines; evidence=read; runtime-proof=pending`.
