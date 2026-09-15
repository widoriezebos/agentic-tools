# brief-declares-the-round-boundary

- Owner: m1e (goal 28 of plans/delivery-efficiency-plan.md). **Revision 3, 2026-09-14**, the final prose fold, by the Codex gpt-6-astra design delegate from critique round 2 (records/misc/brief-declares-the-round-boundary-critique-r2.md, three material findings, all accepted; two round-1 findings reopened and discharged) under the seat's rulings; the seat integrated the page and edits only this header block.
- Goal and current status: the brief carries a paired Boundary and Ceiling or neither, parsed once at admission from the seat-authored bytes and persisted beside the round as admitted-brief.md and brief-bounds.json with the admitted digest; review reads the record, never prose, and refuses a candidate outside the bounds before a read is spent. Eleven builder units of at most 400 changed lines each (1a, 1b, 2, 3a, 3b, 3c, 4a, 4b, 4c, 6a, 6b), each with its own Boundary, Ceiling, Non-goals and the proof rule, and a complete rule-to-witness table with every row independently named. Nothing built yet.
- In flight right now: the two-round review budget is spent (eight material findings in round 1, three in round 2, all folded). Under R-97-m1e and D81 this page is the implementation spec; every standing finding is a named test or fixture leg in the unit that owns its rule, and each unit's independent code read carries the check from here. Unit 1a is being briefed.
- Decisions made (and who made them): Wido, 2026-09-14: scope drift and unproved rules are fixed upstream in the brief and the builder's return; units are at most 400 changed lines. The seat, on round 1: both headers or neither; every remove-one witness lives with its production rule. The seat, on round 2: bounds are parsed once at admission from the admitted seat-authored bytes and persisted with their digest, review reads the record; a Boundary member with a path.Match metacharacter or a backslash is a pattern and only a pattern, concrete-path authority equality applies only to literal members, a path containing such bytes is authorised only by a directory declaration, and an installation prefix containing such bytes is refused; one named witness per independently removable rule. The seat, on the budget: no third prose round.
- Waiting on the human: nothing. Headerless briefs keep today's behaviour; a default Ceiling would change DONE and is not proposed.
- Dead ends (do not retry without new evidence): partial-header admission; deriving admitted bounds from the augmented caller:brief bytes; matching across path spaces; whole-project permission admitting siblings; a pattern spelling as concrete input authority; a test-only unit that must mutate another unit's production rule; slot-only selection of a referenced brief.
- Next step: revision 3 is the implementation spec, amended after unit 1b's read (rulings A1 to A3). Units 1a (e461dfd1), 1b-i (fad3ace8) and 1b-ii (this landing) are landed; unit 2 (count fixed-tree text changes) is next, briefed to Codex gpt-5.6-sol with its own Boundary, Ceiling and proof rule and read by Opus, and units 3a, 3b, 3c, 4a, 4b, 4c, 6a and 6b follow in that order, each with its own read.

The seat runs review conformance before dispatching a code critic. The brief supplies the outer path and size limits. The return still declares the concrete changed paths. Both checks must pass.

This is the spec after the final prose fold. Current-code claims were re-checked in this worktree at `077eb09d3d68991a42cd4a018a4b23f60f2b3653`. The DONE contract is `plans/goals/brief-declares-the-round-boundary.md:8`; its review budget is two rounds (`plans/goals/brief-declares-the-round-boundary.md:14`). All source citations below are relative to `metasystem/`. Boundary values are relative to the Git repository root, so this repository's builder headers include `metasystem/`. Uncited test names and APIs are proposed. Allocations are estimates, never measured proof.

The bounded diff covers the entire returned candidate against its merge-base with the target checkout. Current base selection already uses that merge-base (`internal/validate/conformance.go:232-241`). A follow-up does not reset the diff to its own starting commit.

## Header contract

An implementer brief may carry:

```text
Working Mode: implement
Boundary: ["metasystem/internal/dispatch/brief.go", "metasystem/internal/validate/", "metasystem/scripts/agents/*.sh"]
Ceiling: 400
```

Recognize each exact, case-sensitive prefix at column zero. Use one physical line per header and trim whitespace around its value. Do not trim decoded path members. Scan the admitted seat-authored brief only. Fences do not change header recognition: a column-zero header inside a fence is live. A header with leading space, tab, or `>` is not live. Working Mode currently uses column-zero scanning and value trimming (`internal/dispatch/brief.go:47-53`).

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
func ParseBriefBounds(data []byte, installPrefix string) (BriefBounds, error)
func ProjectBriefBoundary(member, installPrefix string) (project string, directory bool, err error)
```

Successful parsing returns either `nil, nil` fields for absence, or a non-nil Boundary slice and non-nil Ceiling. A non-nil empty slice remains distinct from absence. Do not return a successful partial state.

Render parser errors as `BRIEF_BOUNDS_INVALID: <Header>: <detail>`. Besides the pairing details above, use `header occurs more than once`, `expected a JSON array of paths`, `invalid path or pattern <JSON string>`, and `expected a nonnegative decimal integer no greater than 9223372036854775807`.

Use one byte-based admission helper in `internal/dispatch/brief.go`. It reads the brief once, calls ParseBriefBounds once, and passes the parsed result and the same bytes to authority extraction. Return the admitted bytes, parsed bounds, and mode as an in-memory admission value. File wrappers delegate to this helper. They must not call one another and parse twice. With authority requested, preserve authority-before-mode refusal order (`cmd/metasystem/dispatch_verbs.go:2014-2027`). Without authority, preserve the mode refusal before bounds validation (`internal/dispatch/brief.go:41-55`). Authority-only admission enforces both-or-neither but does not require Working Mode.

At the shell's early mode discovery, keep Working Mode extraction separate from admission. Add `job brief-mode --mode-only`; it extracts only Working Mode and cannot be combined with authority or persistence flags. It is used by brief_mode at `scripts/agents/dispatch.sh:858-860`, called at `scripts/agents/dispatch.sh:1542`. Normal `job brief-mode` still checks paired bounds. Add `--root` for the installation root, defaulting to the current directory. The shell supplies `--root "$root"`. Reuse dispatch.projectInstallPrefix, which already preserves the literal prefix apart from the final newline and slash (`internal/dispatch/gitcmd.go:37-53`). Review has the same derivation at `internal/validate/conformance.go:263-274`. Share the header scan inside brief.go so admission knows pair presence before requesting the prefix; do not decode the pair again to make this decision. Headerless admission needs no prefix lookup. The actual initial paired parse occurs at the authority admission at `scripts/agents/dispatch.sh:1655`.

Change the initial shell failure suffix to `brief headers are invalid; Working Mode must be filled and Boundary and Ceiling must appear together`. Its current suffix names only Working Mode (`scripts/agents/dispatch.sh:1542`). Parsing stays in Go.

## Paths and matching

A decoded member ending in `/` is a literal directory declaration. For every other member, any byte in `*?[]` or a backslash makes it a pattern and only a pattern. Concrete-path authority equality applies only to non-directory members with none of those bytes. A changed Git path containing any of those bytes can be authorised only by a trailing-slash directory declaration, including an ancestor directory. An exact member or a pattern cannot authorise it, even if path.Match would match. Nested projection requires the literal installation prefix followed by `/` and strips it once, before any pattern interpretation. A bounded brief in an installation whose prefix contains any of those bytes refuses with `BRIEF_BOUNDS_INVALID: Boundary: installation prefix contains unsupported pattern bytes`. Headerless briefs retain their existing behavior. This is the seat's pattern-versus-concrete rule.

Use the repository-relative string after JSON decoding as the concrete Git path identity. Use slash separators. Preserve all other bytes. Do not clean paths, fold case, trim decoded members, resolve symlinks, or replace backslashes. Spaces, commas, quotes, tabs and newlines remain filename bytes. Pattern spellings are preserved too, but do not become concrete identities by equality.

Reject empty or absolute declarations, NUL, and empty, `.`, or `..` slash components. Only the final empty component of a directory declaration is allowed. ProjectBriefBoundary owns the brief's literal prefix projection and remembers the trailing-slash flag. ParseBriefBounds validates components, calls this helper, and only then validates non-directory pattern syntax with path.Match. Exact members need no pattern parser. Directory declarations bypass it completely. `metasystem/a[/` therefore permits descendants of the literal directory `a[`, while `metasystem/a[` is malformed pattern syntax.

The admission helper receives the installation prefix. Persist original repository-relative members, never the projected strings. Review applies ProjectBriefBoundary again to those structured members for the target installation. This projects paths, not prose. The existing return dialect remains separately owned by projectDeclaration, whose literal prefix algorithm is the precedent (`internal/validate/conformance.go:284-293`). Both algorithms strip exactly one prefix; fixtures pin their agreement for ordinary paths.

For matching, first guard each changed repository path against the raw `installPrefix + "/"`. A sibling is a Boundary offender immediately. Only then call installationPath and match in project space. A whole-project declaration such as `metasystem/` projects to an empty directory prefix. It permits every inside-project path, including names with pattern bytes, after that sibling guard. A doubled prefix such as `metasystem/metasystem/a.go` retains one `metasystem/` in project space. Diagnostics retain the original repository spelling. Pass project paths to the cumulative return check.

Make installationPath a literal, single-prefix removal. Today it replaces backslashes and trims slashes (`internal/validate/conformance.go:88-95`). Its current caller is waiver classification over repository paths (`internal/validate/conformance.go:97-107`). Preserve that caller's policy and its nested waiver guard.

Add `briefBoundaryMatches(projectDeclaration, projectPath string, directory bool) bool` in `internal/validate/brief_bounds.go`. A directory matches any descendant depth and respects its final slash. Otherwise, reject a changed path with pattern bytes before testing exact equality or a pattern. Patterns match the whole project path using Go's slash-based path.Match grammar. `*` stays within one component. `?`, classes, and backslash escapes retain their meanings for ordinary concrete paths. `**` has no recursive meaning. An unmatched pattern is valid and permits nothing. There is no shell expansion, Git pathspec expansion, brace expansion, or negation. Declarations need not exist on disk. Current return matching is exact map membership (`internal/validate/conformance.go:798-810`); keep directories and patterns out of that dialect.

A live Boundary line contributes no authority inputs. Skip its entire line, including all fragments. Do not let output permission suppress a citation elsewhere. Existing input use wins over output use (`internal/dispatch/brief.go:155-167`). An inert example is exactly one or more leading ASCII spaces or tabs followed immediately by `Boundary:` or `Ceiling:`. Skip that entire line in authority extraction and ignore it in bounds parsing. This exemption does not leak to another line. A `>` quote is not exempt. Fences alone are not exempt. Current extraction scans lines without fence state (`internal/dispatch/brief.go:128-160`).

For a valid pair, recognize complete single-backtick spans and complete JSON string literals before the old token scanner. Decode JSON strings exactly as Boundary strings. Backtick contents are literal unless the entire contents form a complete JSON string, which is decoded. Use JSON escapes for newlines. Consume each complete span so it cannot emit fragments. An incomplete backtick span and an incomplete or invalid JSON string are not consumed as structured spans. The old scanner processes that text unchanged. Each fallback has its own named test and exact expected authority paths.

Apply the existing directory eligibility rule to a structured citation's decoded first components. Also admit it to authority extraction if it equals a concrete Boundary member as defined above. That equality admits input checks for a repository-root filename or a file under an absent directory. A pattern or directory never supplies this equality, even if the citation has the same spelling. A special-byte input under an eligible existing directory is still checked by its full decoded identity. Keep the old scanner for unquoted citations and headerless prose; its token and directory rules are at `internal/dispatch/brief.go:13-24` and `internal/dispatch/brief.go:141-154`.

After decoding, keep existing Workspace output and Create classifications. A separate input use still wins. Keep committed-tree and runtime-artifact lookup owners (`internal/dispatch/brief.go:83-104`). Special-character witnesses assert the full MissingPaths value, then commit that exact file and prove admission. Fragment-only assertions are insufficient.

`TestBriefBackslashEndToEnd` in unit 4a uses one real nested Git path whose bytes are `metasystem/internal/a` followed by one backslash and `b.txt`. Decode the JSON member `"metasystem/internal/a\\b.txt"`. It is a pattern, gives no concrete-member authority equality, and cannot authorise that Git path. The declaration `"metasystem/internal/"` does authorise it. Cite the same JSON string on an ordinary input line. With internal present in the committed tree, authority refuses naming the full backslash path, then admits after that exact file is committed. Persist and read the admitted pair, project it, and match the Git-reported path in the same test. Assert that the projected path still contains exactly one backslash. In the same test, check equality eligibility with an absent new directory to show that the pattern spelling cannot manufacture eligibility. The byte-preserving path, JSON decoding, projection, authority equality and final matching are all observed together. Independent unit 1b cases own the authority-only removals.

## Which brief binds a round

Bind the supplied implementer job's admitted seat-authored bytes. Composition reads the file it is passed (`internal/dispatch/composition.go:237-240`) and labels it caller:brief (`internal/dispatch/composition.go:268`). Dispatch passes an augmented body at `scripts/agents/dispatch.sh:1817-1823` and `scripts/agents/dispatch.sh:2771-2775`. That source is delivered task direction. It is not proof of the bytes checked at admission.

Retain two files in `artifacts/agents/<rootJob>/rounds/<round>/`, beside prompt.md and composition.json: **admitted-brief.md** and **brief-bounds.json**. Use the same names for initial and follow-up rounds. Keep the existing root brief.md as the delivered augmented brief. It is currently copied at `scripts/agents/dispatch.sh:1918`.

brief-bounds.json has this closed schema. Every field is required. Keys use these spellings:

```json
{
  "schemaVersion": 1,
  "jobId": "impl-r2",
  "rootJob": "impl",
  "round": 2,
  "admittedSha256": "<64 lowercase hexadecimal digits>",
  "admittedBytes": 123,
  "boundary": ["metasystem/internal/"],
  "ceiling": 400
}
```

The digest and byte count name admitted-brief.md exactly, including its final newline or lack of one. Round is a positive int64. For an unbounded admission, boundary and ceiling are both JSON null. For a bounded admission, boundary is an array, including an empty array, and ceiling is a nonnegative int64. They cannot be a partial pair. Reject unknown, missing or repeated keys, trailing JSON data, wrong types, invalid identity fields and unsupported schema versions. Reuse the paired bounds value validator for field presence, ceiling range and member components. It does not scan retained prose. Pattern validation still follows literal projection at the consuming installation; the record codec does not interpret an unprojected pattern.

Add one optional **admittedBrief** member only to the task-direction CompositionSource:

```json
{"schemaVersion":1,"bounded":true,"recordSha256":"<sha256 of exact brief-bounds.json bytes>"}
```

It is absent for legacy composition callers. New dispatcher admissions write it for both bounded and headerless briefs. The boolean records pair presence independently of the sibling file. This makes a missing record for a composition-marked bounded round detectable. The record hash binds the parsed values as well as their admitted-byte digest. Use the existing digest helpers (`internal/dispatch/composition.go:565-568`, `internal/validate/recertification.go:115-118`). Do not reinterpret SourceDigest as the admitted digest.

Keep composition schemaVersion 1 and its top-level and reference shapes. Extend only the source shape from seven fields to seven plus this optional member for caller:brief task-direction. Admission currently requires exactly seven source fields (`internal/dispatch/build.go:1018-1028`) and a closed top-level shape (`internal/dispatch/build.go:959-978`). Update that check and CompositionSource together. Validate the optional member's exact three keys and types. Reject it on other slots or sources. Legacy seven-field sources remain valid. The typed reference reader disallows unknown fields (`internal/dispatch/references.go:59-70`), so the struct extension is required too. No return schema changes.

The persistence owner is unit 3c. Unit 3a owns the structured codec and composition marker. Unit 3b owns the review reader. Unit 3c owns the file writer, shell publication, CLI plumbing, and the S1 admitted-versus-augmented seam witness. None is a test-only unit.

Persistence is wired at these exact current sites:

1. At `scripts/agents/dispatch.sh:1655`, initial authority admission parses the admitted brief once. Extend brief_authority (`scripts/agents/dispatch.sh:862-864`) to pass `--admission-dir`, `--job`, `--root-job` and `--round` to job brief-mode. After authority succeeds, the Go writer writes the admitted byte copy and structured record into a private temporary directory. It receives the admission value, not a filename to re-read. Install temporary-directory cleanup before this call, rather than waiting for the full setup trap at `scripts/agents/dispatch.sh:1673`. Set the initial delivery source to that retained temporary admitted copy before augmentation. This prevents a caller-file edit after admission from changing the delivered starting bytes.
2. At `scripts/agents/dispatch.sh:2416`, follow-up captures authority_message from the caller's message. Preserve it through rebase or continuation message construction. At `scripts/agents/dispatch.sh:2643`, authority admission parses those bytes once and stages the same two files with child job id, root id and current round. It must never record the amended message or delivery_content. Later transformations may still deliver their own extra text.
3. At composition invocations `scripts/agents/dispatch.sh:1817-1823` and `scripts/agents/dispatch.sh:2771-2775`, pass the staged record through a new `--admitted-bounds` flag and ComposeRolePacketParams.AdmittedBounds. Composition validates structured record identity for its job and round, hashes its exact bytes, and adds admittedBrief to the selected source. It never calls ParseBriefBounds. Keep --brief pointing to the augmented delivered body. Its present CLI owner is `cmd/metasystem/dispatch_verbs.go:62-145`.
4. Publish the staged admitted copy and record after the winning claim's `mkdir -p "$round_dir"` at `scripts/agents/dispatch.sh:1906` for initial dispatch and `scripts/agents/dispatch.sh:2863` for follow-up, before the composition moves at `scripts/agents/dispatch.sh:1927` and `scripts/agents/dispatch.sh:2886`. These are insertion anchors, not claims that persistence already exists. A failed write stops setup. No adapter starts with a composition marker but missing admitted files. Extend the existing temporary cleanup owner (`scripts/agents/dispatch.sh:153-160`) for refused, losing, and failed setups. Publish only for the winning claim; a repeated or losing wrapper cannot replace standing round evidence.

Normal public job brief-mode validates bounds without persisting unless --admission-dir is supplied. That flag requires authority admission and the identity flags as a complete set. --mode-only conflicts with them. These are internal plumbing additions to the existing verb, not a second parser or a separate admission command.

Keep the exact delivered payload semantics. Initial implementer testing text is appended at `scripts/agents/dispatch.sh:1736-1742`, serving-goal text at `scripts/agents/dispatch.sh:1756-1764`, and return-path text at `scripts/agents/dispatch.sh:1780-1783`. Follow-up delivery is augmented at `scripts/agents/dispatch.sh:2697-2709`. Root brief.md, the composed task-direction body, and staged/task-direction.md continue to hold that augmented body. Thus the bed's exact payload-before-role assertion (`scripts/agents/dispatch-fixtures.sh:2182-2192`), serving-goal-in-payload and prompt-hash assertions (`scripts/agents/dispatch-fixtures.sh:2193-2225`), and referenced-body equality (`scripts/agents/dispatch-fixtures.sh:2338-2343`) stay true. The new files carry admission evidence separately.

Add `ReadRoundBriefBounds(root, rootJob, job, roundText string, jobComposition map[string]any) (dispatch.BriefBounds, error)` and the small conformanceRun.reviewBriefBounds wrapper in `internal/validate/brief_bounds_source.go`. Resolve the supplied job's r.rootJob and r.roundText. Never choose the latest round. The public Go helper permits the dispatch-to-review seam test before reviewStage enforcement lands.

The conformance wrapper passes the composition member of its already resolved r.record to the helper, or nil for a legacy record. resolveFacts already stores the supplied job record (`internal/validate/conformance.go:139-156`). Do not add a second job-file read. The helper compares that embedded marker with the marker in round composition.json. Current initial and follow-up records embed composition (`internal/dispatch/build.go:599`, `internal/dispatch/build.go:928`). If either copy claims admitted evidence, both copies and the two admitted files are required and their markers must agree. A missing round composition cannot hide the job record's bounded marker.

With composition evidence present, require valid job and round metadata and exactly one caller:brief task-direction source. Validate its prompt range and DeliveredDigest. These fields already name the delivered bytes (`internal/dispatch/composition.go:366-390`). This validates packet binding only. Never parse the delivered body, inline envelope, reference stub, retained admitted copy, prior slot, root brief.md or return for bounds.

Retain the source/reference binding checks from revision 2. If task-direction is referenced, select exactly one reference by slot and join SourceDigest and SourceBytes. In a pure validateBriefSourceReferenceBinding helper, separately require nonempty Path, repository-relative Path, no parent component, absolute OpenPath, and the slash-spelled OpenPath suffix `"/" + Path`. These guards follow current admission (`internal/dispatch/build.go:1050-1077`). Test each helper guard directly, so ReadVerifiedReference cannot mask removal of the absolute-path guard. Then call dispatch.ReadVerifiedReference for root containment, regular-file, actual length and digest checks (`internal/dispatch/references.go:24-49`). A source with no reference must have an inline body bound by SourceBytes, SourceDigest and the exact task-direction envelope (`internal/dispatch/composition.go:366-390`, `internal/dispatch/composition.go:555-562`). Verify that envelope to distinguish an inline body from a missing-reference stub. Neither branch parses headers.

If admittedBrief is present, require both sibling files to be readable regular files, with no symlink substitution. Check the record hash, its schema and supplied job/root/round, and its bounded state against the marker. Check admittedBytes and admittedSha256 against the retained admitted copy. Return only the structured bounds. Every absent, unreadable, malformed or mismatching admitted record, including a record missing while bounded is true, refuses BRIEF_BOUNDS_UNREADABLE. A digest mismatch never falls back. Invalid stored structure is unreadable evidence, not a fresh prose syntax refusal. BRIEF_BOUNDS_INVALID remains the admission diagnostic and the structured projection diagnostic.

If no marker exists in either composition copy and neither admitted file exists, return unbounded after validating any composition evidence that is present. Either legacy composition copy may be absent, including both. A nil jobComposition is valid on this legacy path; production job identity is already checked by resolveFacts. Do not recover bounds from old prose. Before this feature there is no retained authenticated admitted copy; the legacy root copy is augmented (`scripts/agents/dispatch.sh:1918`). A present but invalid composition, or orphaned admitted files without the marker, refuses BRIEF_BOUNDS_UNREADABLE. This replaces revision 2's legacy prose fallback. It preserves old jobs without retroactive bounds. Leave the exhaustion reader's separate prompt fallback unchanged (`internal/validate/conformance.go:925-954`).

Each round stands alone. A headerless follow-up has a recorded null pair and is unbounded. A partial follow-up refuses admission. Do not inherit prior pairs, union brief boundaries, or add ceilings. A bounded correction repeats both headers for the whole still-unlanded candidate. If it omits an earlier changed file or lowers Ceiling below the cumulative count, review refuses. A scope amendment uses a new round's brief.

Return declarations remain cumulative (`internal/validate/conformance.go:754-817`). Cap continuation currently tells the builder to declare predecessor changes too (`internal/dispatch/capcontinuation.go:22-28`, `internal/dispatch/capcontinuation.go:89`). Clarify that this is the return's concrete declaration and the current admitted pair governs the whole candidate. Adding a path to diffBoundary grants no permission.

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

The second return reports inability to evaluate, including an unsupported installation prefix or malformed projected declaration. Preserve concrete parser and violation types until rendering. Exact stderr examples are:

```text
conformance failure: BRIEF_BOUNDARY_EXCEEDED: changed paths outside Boundary: ["extra.txt"]; changedLines=1 additions plus deletions
conformance failure: BRIEF_CEILING_EXCEEDED: changedLines=3 exceeds Ceiling=2 additions plus deletions; changedPaths=["source.txt"]
```

Boundary.Paths includes every actual repository path outside the brief. Ceiling.Paths includes every changed repository path, including binary paths. Both carry the total text count for the repository candidate. Sort and deduplicate diagnostic arrays, encode as JSON, and never truncate. If both checks fail, report Boundary first and Ceiling second. Existing project and cumulative policy diagnostics follow.

Read bounds before evaluation. Compute paths and the count from the fixed trees, then collect policy failures before either success artifact is written. For bounded review, a sibling yields both the typed Boundary failure and the existing project-fence failure. It also contributes to the candidate count. The whole-project declaration cannot hide it. Keep the existing cumulative check over project paths, including its protected plans and control-plane rules (`internal/validate/conformance.go:700-725`, `internal/validate/conformance.go:798-817`).

For headerless review, preserve the existing outside-project-first refusal and existing immutable-evidence precedence. Current outside-project refusal precedes the cumulative check (`internal/validate/conformance.go:377-400`). The count branch is absent on this path.

Use `conformance failure: BRIEF_BOUNDS_UNREADABLE: <detail>` for source failures and `conformance failure: BRIEF_LINES_UNREADABLE: <detail>` for count failures. Projection errors retain `BRIEF_BOUNDS_INVALID`; malformed persisted evidence is `BRIEF_BOUNDS_UNREADABLE`. Stop immediately when facts cannot be computed. These failures return exit 1 and write no new success artifacts. Do not fabricate a count to complete a policy message.

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
| Brief-mode CLI, `cmd/metasystem/dispatch_verbs.go:1993-2027` | Add admission-record plumbing and mode-only discovery. Preserve normal mode stdout and authority-only behavior. |
| Shell preflight and persistence, `scripts/agents/dispatch.sh:858-864`, `scripts/agents/dispatch.sh:1542`, `scripts/agents/dispatch.sh:1655`, `scripts/agents/dispatch.sh:2416`, `scripts/agents/dispatch.sh:2643` | Separate mode discovery, admitted-byte capture, and augmented delivery. Persist only the winning round. Go owns parsing and record encoding. |
| Composition production and strict typed reader, `internal/dispatch/composition.go:50-58`, `internal/dispatch/composition.go:268`, `internal/dispatch/composition.go:358-390`, `internal/dispatch/references.go:59-70` | Add optional admittedBrief source metadata. Keep delivered bytes and references intact. Extend the typed source structure. |
| Composition admission, `internal/dispatch/build.go:959-978`, `internal/dispatch/build.go:1018-1077`, `internal/dispatch/build.go:1099-1103` | Permit the optional marker only on caller:brief task-direction. Preserve the other closed shapes and source/reference joins. |
| Evidence mirror, `internal/dispatch/mirror.go:117-139` | Keep the root delivered brief copy and existing recursive regular-file enumeration. The two new round files are already included by that enumeration. |
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

Each row below uses a fresh case. Use the real public verb. Create no critic. For bounded review legs, write a separate admitted brief and invoke the new admission-record plumbing. Compose it through the real composition helper with that record and copy the resulting composition into the fixture job, as dispatch does. Copy the existing recipe inputs into the temporary controller for composition. Do not insert headers into legacy brief.md and expect review to parse them. The fixture uses a root installation, so its repository paths are unprefixed. Headerless legs retain legacy fixture construction; the Go seam also proves a newly recorded headerless round.

| Leg | Candidate and brief | Required result |
| --- | --- | --- |
| `brief-boundary-unlisted` | Add untracked extra.txt containing `extra\n`. Declare extra.txt in the return. Boundary `["source.txt"]`, Ceiling 10. | Review exits 1 with the exact Boundary example above. No diff.patch or review.json. |
| `brief-ceiling-exceeded` | Append three lines to source.txt. Declare source.txt. Boundary `["source.txt"]`, Ceiling 2. | Review exits 1 with the exact Ceiling example above and count 3. No success artifacts. |
| `brief-both-admitted` | Append two lines to source.txt. Declare source.txt. Boundary `["source.txt"]`, Ceiling 2. | Exit 0 with both artifacts, exactly three review fields, and the correct project tree. |
| `brief-no-headers` | Append 401 lines to source.txt and add a one-line extra.txt. Declare both. Neither header. | Exit 0 despite 402 changed lines. |
| `brief-no-headers-undeclared` | Repeat the preceding candidate in a fresh case, declaring only source.txt. | Today's cumulative-boundary refusal names extra.txt. |
| `brief-missing-ceiling` | A Working Mode brief with Boundary `["source.txt"]` only. | Public `job brief-mode` exits 1 with `BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary`. No implementer or critic is dispatched. |
| `brief-missing-boundary` | A Working Mode brief with Ceiling 2 only. | Public `job brief-mode` exits 1 with `BRIEF_BOUNDS_INVALID: Boundary: required with Ceiling`. No implementer or critic is dispatched. |

These are seven executions: four DONE outcomes, the headerless guard, and both partial-pair refusals. The partial-pair Go test is `TestBriefBoundsRejectsPartialHeaders`, with boundary-only and ceiling-only cases; `TestDispatchBriefBoundsAdmission` proves both public admission routes refuse too.

Use real composed inline and referenced inputs plus admitted records in Go source-reader tests. Do not let legacy fixture convenience stand in for those tests. The existing section runs the bed through the engine (`scripts/validate-metasystem.sh:1129-1131`). Add no bed, runner, testing group, or test policy.

## Builder units and ordered proof

Land in this order: **1a, 1b, 2, 3a, 3b, 3c, 4a, 4b, 4c, 6a, 6b**. The old test-only unit 5 remains removed. Revision 3 splits source work into codec/marker, reader, and admission persistence, with each owner carrying its own witnesses. The former unit 5 review witnesses live in units 4b and 4c; its public CLI witness lives in 6b. Each unit owns the production behavior its remove-one runs change.

Each unit starts a fresh implementation chain from the preceding landed unit. Its 400-line ceiling includes additions plus deletions in production, tests, scripts, and prose. Do not combine several unlanded units into one supposedly small candidate. The allocations below are estimates with headroom. If an allocation cannot fit, stop and propose another split before exceeding 400 or touching an unlisted path. The seat owns integration receipts outside the returned builder patch.

Every unit receives the mandatory per-unit code read under the prose-loop exit. A fixture failure is corrected in its production-owning unit. It does not request a third design critique. Before enforcement lands in 4c, the seat measures the candidate manually. Afterwards it also uses the paired brief gate. The builder never commits, executes fixture beds, exports METASYSTEM_BIN for a Go run, or changes the provided GOCACHE, GOTMPDIR, or STATICCHECK_CACHE. The builder/seat proof split and cache rules are already stated in `internal/dispatch/build.go:1126` and `docs/orchestration.md:172`.

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

Allocate 150 production, 210 test, and 25 register/test lines, total 385. Add the byte-based admission value, paired parser, literal brief projection, structured value validator, mode extraction, and invalid-header register row. Parse once within each admission invocation. Own H1-H7 and D1.invalid below. Packages: `./internal/dispatch ./internal/refusal`. Shell files: none.

### Unit 1b: Preserve authority identity through admission

```text
Working Mode: implement
Boundary: ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go", "metasystem/cmd/metasystem/dispatch_verbs.go", "metasystem/cmd/metasystem/dispatch_brief_bounds_test.go", "metasystem/scripts/agents/dispatch.sh"]
Ceiling: 400
Non-goals: no parser-policy redesign, admitted-record persistence, diff calculation, review enforcement, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 130 production, 250 test, and 10 shell lines, total 390. Pass the already parsed bounds and admitted bytes into authority extraction. Add live and inert line handling, structured spans, concrete-member eligibility, CLI root and mode-only handling, and the shell suffix. Own A1-A5 and H8. Keep initial mode discovery free of the paired parse; authority admission performs it. Packages: `./internal/dispatch ./cmd/metasystem`. Shell: `scripts/agents/dispatch.sh`.

### Unit 2: Count fixed-tree text changes

```text
Working Mode: implement
Boundary: ["metasystem/internal/gittree/numstat.go", "metasystem/internal/gittree/numstat_test.go"]
Ceiling: 400
Non-goals: no snapshot-policy change, conformance wiring, waiver refactor, second Git runner, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 80 production and 250 test lines, total 330. Own N1-N5. Use Workspace.git. Packages: `./internal/gittree`. Shell files: none.

### Unit 3a: Define the admitted record and composition marker

```text
Working Mode: implement
Boundary: ["metasystem/internal/dispatch/brief_bounds_record.go", "metasystem/internal/dispatch/brief_bounds_record_test.go", "metasystem/internal/dispatch/composition.go", "metasystem/internal/dispatch/build.go", "metasystem/internal/dispatch/composition_brief_bounds_test.go"]
Ceiling: 400
Non-goals: no admission file writer, shell publication, review reader, return schema changes, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 155 production and 235 test lines, total 390. Own R1-R3: closed structured record codec, admittedBrief marker, optional source field admission, and composition's record identity check. This unit does not create runtime files from a brief. It uses the already parsed admission value. Test the legacy source and the new source with the actual job composition admission validator, not a second schema checker. Packages: `./internal/dispatch`. Shell files: none.

### Unit 3b: Read only the supplied round's persisted bounds

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/brief_bounds_source.go", "metasystem/internal/validate/brief_bounds_source_test.go", "metasystem/internal/refusal/register.go", "metasystem/internal/refusal/brief_bounds_register_test.go"]
Ceiling: 400
Non-goals: no prose parsing, admission persistence, review enforcement, full job-admission rewrite, exhaustion-reader change, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 140 production, 235 test, and 15 register/test lines, total 390. Own S2-S6, the S1 selection cases, and D1.bounds-unreadable. Expose ReadRoundBriefBounds and its conformance wrapper. Use table-driven corrupt-source cases and real ComposeRolePacket fixtures. The existing exhaustion fixture writes a reduced references-only object (`internal/validate/conformance_test.go:118-123`); it cannot stand in for production composition here. Test Path/OpenPath binding directly at the pure helper to observe each removable guard. Packages: `./internal/validate ./internal/refusal`. Shell files: none.

### Unit 3c: Persist admission before augmentation and prove the seam

```text
Working Mode: implement
Boundary: ["metasystem/internal/dispatch/brief_bounds_record.go", "metasystem/internal/dispatch/brief_bounds_persist_test.go", "metasystem/cmd/metasystem/dispatch_verbs.go", "metasystem/cmd/metasystem/dispatch_brief_bounds_persist_test.go", "metasystem/scripts/agents/dispatch.sh"]
Ceiling: 400
Non-goals: no record-schema redesign, changes to delivered brief content, reviewStage enforcement, production test hooks, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 105 Go production, 60 shell, and 225 test lines, total 390. Own R4 and all five S1 dispatcher-to-review cases. Add the writer that consumes the admission value and the CLI and shell sites listed above. Add `TestDispatchToReviewAdmittedBounds` in the command package. It invokes the same job brief-mode and compose-role-packet Go entrypoints called by dispatch.sh, then ReadRoundBriefBounds. Current command tests already call the composition entrypoint (`cmd/metasystem/dispatch_verbs_test.go:20-39`).

The seam case starts with admitted Boundary `["source.txt"]` and Ceiling 1. After admission, mutate the caller's source and add live conflicting headers to a generated delivered appendix. Compose through runDispatchComposeRolePacket with the captured record. The reader must return exactly the admitted pair. Repeat as separately named initial-inline, initial-referenced, follow-up-inline and follow-up-referenced cases, plus a recorded-headerless case whose delivered appendix contains a pair. Inspect admitted bytes, record hashes, delivery bytes and returned bounds. A test that hand-builds composition without the dispatcher helper does not satisfy S1.

`TestDispatchAdmittedBoundsCallsites` independently checks that initial and follow-up shell publication use the retained admission result, pass it to composition, and occur only after the winning claim. The writer failure and cleanup cases are named in R4. Remove the shell handoff for its own focused witness; remove the CLI record handoff for the actual seam witness. No test mutates the source reader outside this unit. The seam test supplies the composed job metadata to the same reader argument used by conformance. The seat also runs the existing dispatch bed to retain the three delivered-payload assertions cited above. Packages: `./internal/dispatch ./cmd/metasystem`. Shell: `scripts/agents/dispatch.sh`.

### Unit 4a: Match projected paths and build typed violations

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/brief_bounds.go", "metasystem/internal/validate/brief_bounds_test.go", "metasystem/internal/validate/conformance.go", "metasystem/internal/refusal/register.go", "metasystem/internal/refusal/brief_bounds_register_test.go"]
Ceiling: 400
Non-goals: no review-stage wiring, changed waiver policy, recertification change, schema change, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 125 production, 250 test, and 20 register/test lines, total 395. Own P1-P5, V1-V3, D1.boundary, D1.ceiling, and TestBriefBackslashEndToEnd. Change only installationPath in conformance.go. Reuse ProjectBriefBoundary for brief declarations; keep the return dialect unchanged. Register the two exceeded codes when their types land. Packages: `./internal/validate ./internal/refusal`. Shell files: none.

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

Allocate 85 production, 285 test, and 15 register/test lines, total 385. Own E1-E7 and D1.lines-unreadable. Bounds-source registration already landed in 3b. Integrate the source reader, fixed-tree count, typed checks, collected bounded diagnostics, and immutable evidence behavior. Reuse unit 4b's interposer without changing it. All mutation targets for these integration rules are in conformance.go. Packages: `./internal/validate ./internal/refusal`. Shell files: none.

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

Keep the current cap paragraph test (`internal/dispatch/capcontinuation_test.go:55-76`) and add the independently named TestCapContinuationBounds cases in that same file. Packages: `./internal/dispatch`. Shell files: none.

### Unit 6b: Prove the public verbs and existing bed

```text
Working Mode: implement
Boundary: ["metasystem/scripts/agents/conformance-fixtures.sh", "metasystem/cmd/metasystem/validate_verbs.go", "metasystem/cmd/metasystem/conformance_brief_bounds_test.go"]
Ceiling: 400
Non-goals: no new fixture bed or testing group, underlying limit-rule changes, extra CLI flags, builder fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate 165 fixture-plumbing lines, 15 help lines, and 210 Go test lines, total 390. Own W5-W6. Add the seven legs above, their admitted-record setup, and public-verb tests without a critic. Add this help sentence: "Review also enforces the current brief's paired Boundary and Ceiling before a code read; omitting both keeps the existing checks." The Go CLI test lives with the permitted CLI relay and help source. It does not mutate a limit rule in validate.

Packages: `./cmd/metasystem`. Shell: `scripts/agents/conformance-fixtures.sh`. Also run the unchanged-reader seam command `go test -race -count=1 ./internal/adapter ./internal/dispatch ./internal/gittree` after the unit package test and before bash and the fast gate.

The builder syntax-checks the bed. The seat executes `section/conformance-fixtures` and reads all seven outcomes, then the risk-selected dispatch proof including `section/dispatcher-adapter-and-mission-runner-fixtures` (`testing.json:96`). The seat uses its enrolled engine and retained structured results.

### Complete rule-to-witness table

Every row below has its own full test name. Each slash suffix is a separately named subtest. A row is a separate remove-one run for its owning rule. Positive cases detect removal of an allowance. Negative cases detect removal of a guard. Assert errors.As where types are promised and compare full fields or diagnostics, not fragments. No row delegates its witness to a test-only unit.

For S4, call the pure source/reference binding helper directly. The Path-empty, Path-absolute, Path-parent, OpenPath-relative and Path-suffix cases each keep the other binding facts valid where possible. This isolates the named guard even where a later file check would also reject. The digest case supplies a different valid self-consistent reference body. The bytes case changes only the joined source count. The slot case supplies one differently slotted reference with otherwise matching identity. S5 observes the review caller preserving each distinct verification refusal; it does not authorize mutation of references.go.

For S1's five unit 3c cases, run the real dispatcher Go entrypoints, retain their artifacts, then call the production review reader. These are the admitted-versus-augmented witnesses for BDRB-R2-01 and BDRB-R2-03. Their generated live headers would change or invalidate bounds if review parsed delivered prose. The source call-graph assertions in H7 and S6 additionally pin the single parser owner. They do not substitute for the seam execution.

All A3 special-character tests compare the full missing path and admit after committing that exact file. The P4 backslash end-to-end test adds projection, persistence, concrete equality eligibility and final matching in one execution. P4.question, P4.class and P4.escape each have an independent case. Both A3 incomplete-span cases have independent names and assert the exact legacy fallback paths.

| Rule id and rule | Owning unit and mutation target | Test that fails when only that rule is removed |
| --- | --- | --- |
| H1.case: Headers are case-sensitive. | 1a, brief.go header scanner | `TestBriefBoundsHeaderRecognition/case` |
| H1.column: Only column-zero headers are live. | 1a, brief.go header scanner | `TestBriefBoundsHeaderRecognition/column` |
| H1.physical-line: A header value cannot continue on a second physical line. | 1a, brief.go header scanner | `TestBriefBoundsHeaderRecognition/physical-line` |
| H1.trim: Trim value whitespace. | 1a, brief.go header scanner | `TestBriefBoundsHeaderRecognition/trim` |
| H1.fence: A column-zero header inside a fence is live. | 1a, brief.go header scanner | `TestBriefBoundsHeaderRecognition/fence` |
| H2.neither: An absent pair returns two nil fields. | 1a, brief.go pairing | `TestBriefBoundsPresence/neither` |
| H2.both: A present pair returns two non-nil fields. | 1a, brief.go pairing | `TestBriefBoundsPresence/both` |
| H2.empty-array: An empty Boundary remains non-nil and deny-all. | 1a, brief.go pairing | `TestBriefBoundsPresence/empty-array` |
| H2.duplicates: Duplicate Boundary members remain valid. | 1a, brief.go pairing | `TestBriefBoundsPresence/duplicates` |
| H2.boundary-only: Refuse naming the missing Ceiling. | 1a, brief.go pairing | `TestBriefBoundsRejectsPartialHeaders/boundary-only` |
| H2.ceiling-only: Refuse naming the missing Boundary. | 1a, brief.go pairing | `TestBriefBoundsRejectsPartialHeaders/ceiling-only` |
| H3.empty: Refuse an empty Boundary value. | 1a, brief.go JSON decoding | `TestBriefBoundsSyntax/empty` |
| H3.null: Refuse JSON null as a live Boundary value. | 1a, brief.go JSON decoding | `TestBriefBoundsSyntax/null` |
| H3.object: Refuse a non-array Boundary. | 1a, brief.go JSON decoding | `TestBriefBoundsSyntax/object` |
| H3.nonstring: Refuse a non-string member. | 1a, brief.go JSON decoding | `TestBriefBoundsSyntax/nonstring` |
| H3.trailing-json: Refuse trailing JSON data. | 1a, brief.go JSON decoding | `TestBriefBoundsSyntax/trailing-json` |
| H3.duplicate-boundary: Refuse duplicate Boundary headers. | 1a, brief.go JSON decoding | `TestBriefBoundsSyntax/duplicate-boundary` |
| H4.plus: Refuse a plus sign. | 1a, brief.go ceiling parser | `TestBriefBoundsCeilingSyntax/plus` |
| H4.minus: Refuse a minus sign. | 1a, brief.go ceiling parser | `TestBriefBoundsCeilingSyntax/minus` |
| H4.fraction: Refuse fractional numbers. | 1a, brief.go ceiling parser | `TestBriefBoundsCeilingSyntax/fraction` |
| H4.suffix: Refuse units after digits. | 1a, brief.go ceiling parser | `TestBriefBoundsCeilingSyntax/suffix` |
| H4.placeholder: Refuse a template placeholder. | 1a, brief.go ceiling parser | `TestBriefBoundsCeilingSyntax/placeholder` |
| H4.empty: Refuse an empty Ceiling. | 1a, brief.go ceiling parser | `TestBriefBoundsCeilingSyntax/empty` |
| H4.non-ascii: Refuse non-ASCII digits. | 1a, brief.go ceiling parser | `TestBriefBoundsCeilingSyntax/non-ascii` |
| H4.overflow: Refuse values above int64 maximum. | 1a, brief.go ceiling parser | `TestBriefBoundsCeilingSyntax/overflow` |
| H4.duplicate: Refuse duplicate Ceiling headers. | 1a, brief.go ceiling parser | `TestBriefBoundsCeilingSyntax/duplicate` |
| H4.zero: Admit zero. | 1a, brief.go ceiling parser | `TestBriefBoundsCeilingSyntax/zero` |
| H4.leading-zeroes: Admit leading zeroes. | 1a, brief.go ceiling parser | `TestBriefBoundsCeilingSyntax/leading-zeroes` |
| H4.maximum: Admit 9223372036854775807. | 1a, brief.go ceiling parser | `TestBriefBoundsCeilingSyntax/maximum` |
| H5.absolute: Refuse an absolute member. | 1a, brief.go member validation | `TestBriefBoundsPathSyntax/absolute` |
| H5.empty: Refuse an empty member. | 1a, brief.go member validation | `TestBriefBoundsPathSyntax/empty` |
| H5.nul: Refuse NUL in a member. | 1a, brief.go member validation | `TestBriefBoundsPathSyntax/nul` |
| H5.empty-component: Refuse an internal empty component. | 1a, brief.go member validation | `TestBriefBoundsPathSyntax/empty-component` |
| H5.dot: Refuse a dot component. | 1a, brief.go member validation | `TestBriefBoundsPathSyntax/dot` |
| H5.parent: Refuse a parent component. | 1a, brief.go member validation | `TestBriefBoundsPathSyntax/parent` |
| H5.member-whitespace: Preserve leading and trailing member spaces. | 1a, brief.go member validation | `TestBriefBoundsPathSyntax/member-whitespace` |
| H6.root: Root installation projection is identity. | 1a, brief.go ProjectBriefBoundary and syntax branch | `TestBriefBoundsProjectionSyntax/root` |
| H6.nested: Strip the literal nested prefix. | 1a, brief.go ProjectBriefBoundary and syntax branch | `TestBriefBoundsProjectionSyntax/nested` |
| H6.missing-prefix: Refuse a member without the literal installation prefix. | 1a, brief.go ProjectBriefBoundary and syntax branch | `TestBriefBoundsProjectionSyntax/missing-prefix` |
| H6.double-prefix: Strip exactly one prefix. | 1a, brief.go ProjectBriefBoundary and syntax branch | `TestBriefBoundsProjectionSyntax/double-prefix` |
| H6.whole-project: Preserve the directory flag when projection is empty. | 1a, brief.go ProjectBriefBoundary and syntax branch | `TestBriefBoundsProjectionSyntax/whole-project` |
| H6.unsupported-prefix: Refuse a bounded installation prefix containing pattern bytes. | 1a, brief.go ProjectBriefBoundary and syntax branch | `TestBriefBoundsProjectionSyntax/unsupported-prefix` |
| H6.projection-before-pattern: Refuse a nonliteral prefix before interpreting an escaped pattern prefix. | 1a, brief.go ProjectBriefBoundary and syntax branch | `TestBriefBoundsProjectionSyntax/projection-before-pattern` |
| H6.literal-directory: A trailing-slash directory bypasses pattern syntax, including a literal bracket. | 1a, brief.go ProjectBriefBoundary and syntax branch | `TestBriefBoundsProjectionSyntax/literal-directory` |
| H6.malformed-pattern: Refuse a malformed non-directory pattern. | 1a, brief.go ProjectBriefBoundary and syntax branch | `TestBriefBoundsProjectionSyntax/malformed-pattern` |
| H6.exact: A member with no pattern bytes stays concrete. | 1a, brief.go ProjectBriefBoundary and syntax branch | `TestBriefBoundsProjectionSyntax/exact` |
| H6.backslash-pattern: A non-directory backslash member is only a pattern. | 1a, brief.go ProjectBriefBoundary and syntax branch | `TestBriefBoundsProjectionSyntax/backslash-pattern` |
| H7.duplicates-first: Duplicate headers win over pairing and value errors. | 1a, brief.go precedence | `TestBriefBoundsErrorPrecedence/duplicates-first` |
| H7.boundary-duplicate-first: Duplicate Boundary wins over duplicate Ceiling. | 1a, brief.go precedence | `TestBriefBoundsErrorPrecedence/boundary-duplicate-first` |
| H7.pair-before-value: A missing pair member wins over malformed values. | 1a, brief.go precedence | `TestBriefBoundsErrorPrecedence/pair-before-value` |
| H7.boundary-before-ceiling: Invalid Boundary wins over invalid Ceiling. | 1a, brief.go precedence | `TestBriefBoundsErrorPrecedence/boundary-before-ceiling` |
| H7.mode-first-without-authority: Without authority, preserve the existing required-mode refusal. | 1a, brief.go precedence | `TestBriefBoundsErrorPrecedence/mode-first-without-authority` |
| H7.read-failure: A failed initial read cannot admit a brief. | 1a, brief.go byte admission helper | `TestBriefAdmission/read-failure` |
| H7.exact-bytes: Authority and the admission value use the same bytes, including final-newline state. | 1a, brief.go byte admission helper | `TestBriefAdmission/exact-bytes` |
| H7.parser-ownership: A source call-graph assertion permits one bounds parse in the admission path and none in its mode-only helper. | 1a, brief.go byte admission helper | `TestBriefAdmission/parser-ownership` |
| H8.normal: Normal public admission validates the pair. | 1b, brief.go authority integration and dispatch_verbs.go | `TestDispatchBriefBoundsAdmission/normal` |
| H8.authority-only: Authority-only admission validates the pair without requiring Working Mode. | 1b, brief.go authority integration and dispatch_verbs.go | `TestDispatchBriefBoundsAdmission/authority-only` |
| H8.mode-stdout: Successful normal admission prints only the mode. | 1b, brief.go authority integration and dispatch_verbs.go | `TestDispatchBriefBoundsAdmission/mode-stdout` |
| H8.authority-before-mode: Combined admission retains authority-before-mode precedence. | 1b, brief.go authority integration and dispatch_verbs.go | `TestDispatchBriefBoundsAdmission/authority-before-mode` |
| H8.mode-only: Early mode discovery performs no bounds admission. | 1b, brief.go authority integration and dispatch_verbs.go | `TestDispatchBriefBoundsAdmission/mode-only` |
| H8.mode-only-conflict: Mode-only cannot be combined with authority flags. | 1b, brief.go authority integration and dispatch_verbs.go | `TestDispatchBriefBoundsAdmission/mode-only-conflict` |
| H8.root-prefix: The CLI passes the actual installation prefix to bounded admission. | 1b, brief.go authority integration and dispatch_verbs.go | `TestDispatchBriefBoundsAdmission/root-prefix` |
| H8.headerless-no-prefix: Headerless admission performs no new prefix lookup. | 1b, brief.go authority integration and dispatch_verbs.go | `TestDispatchBriefBoundsAdmission/headerless-no-prefix` |
| H8.suffix: The shell suffix describes the paired headers. | 1b, dispatch.sh | `TestDispatchBriefBoundsFailureSuffix` |
| A1.file: An absent allowed file is not an input. | 1b, brief.go live Boundary exclusion | `TestBriefAuthorityBoundaryIsOutput/file` |
| A1.directory: An absent directory declaration is not an input. | 1b, brief.go live Boundary exclusion | `TestBriefAuthorityBoundaryIsOutput/directory` |
| A1.glob: A pattern declaration is not an input. | 1b, brief.go live Boundary exclusion | `TestBriefAuthorityBoundaryIsOutput/glob` |
| A1.special-name: The live header emits no special-name fragments. | 1b, brief.go live Boundary exclusion | `TestBriefAuthorityBoundaryIsOutput/special-name` |
| A2.spaces: Leading ASCII spaces make a bounds example inert. | 1b, brief.go inert line handling | `TestBriefAuthorityIndentedBoundsExampleIsInert/spaces` |
| A2.tabs: Leading tabs make a bounds example inert. | 1b, brief.go inert line handling | `TestBriefAuthorityIndentedBoundsExampleIsInert/tabs` |
| A2.headerless: Inert examples stay inert in a headerless brief. | 1b, brief.go inert line handling | `TestBriefAuthorityIndentedBoundsExampleIsInert/headerless` |
| A2.separate-input: The exemption cannot suppress a citation on another line. | 1b, brief.go inert line handling | `TestBriefAuthorityIndentedBoundsExampleIsInert/separate-input` |
| A2.quoted: A greater-than quote gets no indented-header exemption. | 1b, brief.go inert line handling | `TestBriefAuthorityIndentedBoundsExampleIsInert/quoted` |
| A3.space: Preserve a space through a structured input citation. | 1b, brief.go span decoder | `TestBriefAuthoritySpecialCharacterInput/space` |
| A3.comma: Preserve a comma through a structured input citation. | 1b, brief.go span decoder | `TestBriefAuthoritySpecialCharacterInput/comma` |
| A3.quote: Preserve an escaped quote through JSON input decoding. | 1b, brief.go span decoder | `TestBriefAuthoritySpecialCharacterInput/quote` |
| A3.tab: Preserve a tab through JSON input decoding. | 1b, brief.go span decoder | `TestBriefAuthoritySpecialCharacterInput/tab` |
| A3.newline: Preserve a newline through JSON input decoding. | 1b, brief.go span decoder | `TestBriefAuthoritySpecialCharacterInput/newline` |
| A3.backslash: Preserve a literal backslash under an eligible existing directory. | 1b, brief.go span decoder | `TestBriefAuthoritySpecialCharacterInput/backslash` |
| A3.json-in-backticks: Decode a complete JSON string enclosed by backticks. | 1b, brief.go span decoder | `TestBriefAuthoritySpecialCharacterInput/json-in-backticks` |
| A3.literal-backticks: Preserve non-JSON backtick contents literally. | 1b, brief.go span decoder | `TestBriefAuthoritySpecialCharacterInput/literal-backticks` |
| A3.consumed-span: A complete structured span cannot also emit token fragments. | 1b, brief.go span decoder | `TestBriefAuthoritySpecialCharacterInput/consumed-span` |
| A3.incomplete-backtick: An unterminated backtick falls through to the old scanner; assert its exact legacy path output. | 1b, brief.go incomplete-span fallback | `TestBriefAuthorityIncompleteSpan/incomplete-backtick` |
| A3.incomplete-json: An unterminated or invalid JSON string falls through to the old scanner; assert its exact legacy path output. | 1b, brief.go incomplete-span fallback | `TestBriefAuthorityIncompleteSpan/incomplete-json` |
| A4.ordinary: Separate ordinary input use still requires existence. | 1b, brief.go concrete eligibility and input accounting | `TestBriefAuthorityBoundaryInputStillRequired/ordinary` |
| A4.root-file: A concrete member makes a separately cited root filename eligible. | 1b, brief.go concrete eligibility and input accounting | `TestBriefAuthorityBoundaryInputStillRequired/root-file` |
| A4.new-directory: A concrete member makes a separately cited absent-directory path eligible. | 1b, brief.go concrete eligibility and input accounting | `TestBriefAuthorityBoundaryInputStillRequired/new-directory` |
| A4.pattern-no-equality: A pattern member cannot make a citation eligible by equality. | 1b, brief.go concrete eligibility and input accounting | `TestBriefAuthorityBoundaryInputStillRequired/pattern-no-equality` |
| A4.backslash-no-equality: A backslash member cannot make a citation eligible by equality. | 1b, brief.go concrete eligibility and input accounting | `TestBriefAuthorityBoundaryInputStillRequired/backslash-no-equality` |
| A4.directory-no-equality: A directory member cannot make a citation eligible by equality. | 1b, brief.go concrete eligibility and input accounting | `TestBriefAuthorityBoundaryInputStillRequired/directory-no-equality` |
| A4.output-then-input: Later input use wins over earlier output use. | 1b, brief.go concrete eligibility and input accounting | `TestBriefAuthorityBoundaryInputStillRequired/output-then-input` |
| A4.input-then-output: Earlier input use wins over later output use. | 1b, brief.go concrete eligibility and input accounting | `TestBriefAuthorityBoundaryInputStillRequired/input-then-output` |
| A5.unquoted: Ordinary unquoted input retains its existing scanner behavior. | 1b, brief.go bounded lexer integration | `TestBriefAuthorityBoundedCitationCompatibility/unquoted` |
| A5.workspace: Workspace output classification remains. | 1b, brief.go bounded lexer integration | `TestBriefAuthorityBoundedCitationCompatibility/workspace` |
| A5.create: Create output classification remains. | 1b, brief.go bounded lexer integration | `TestBriefAuthorityBoundedCitationCompatibility/create` |
| A5.artifact: Runtime artifact lookup uses the live lookup owner. | 1b, brief.go bounded lexer integration | `TestBriefAuthorityBoundedCitationCompatibility/artifact` |
| A5.headerless: Headerless ordinary prose has no new structured interpretation. | 1b, brief.go bounded lexer integration | `TestBriefAuthorityBoundedCitationCompatibility/headerless` |
| N1.adds-deletes: Count additions plus deletions. | 2, numstat.go parser | `TestChangedLines/adds-deletes` |
| N1.empty: An empty diff counts zero. | 2, numstat.go parser | `TestChangedLines/empty` |
| N1.binary: Exclude exactly a binary row's text count. | 2, numstat.go parser | `TestChangedLines/binary` |
| N1.odd-paths: Split only at the first two tabs, preserving tab and newline filenames. | 2, numstat.go parser | `TestChangedLines/odd-paths` |
| N2.missing-nul: Require complete NUL-terminated records. | 2, numstat.go parser | `TestChangedLinesRejectsMalformedAndOverflow/missing-nul` |
| N2.missing-path: Require a nonempty path. | 2, numstat.go parser | `TestChangedLinesRejectsMalformedAndOverflow/missing-path` |
| N2.count: Require decimal nonnegative counts. | 2, numstat.go parser | `TestChangedLinesRejectsMalformedAndOverflow/count` |
| N2.mixed-binary: A mixed dash/numeric row is invalid. | 2, numstat.go parser | `TestChangedLinesRejectsMalformedAndOverflow/mixed-binary` |
| N2.row-overflow: Check overflow when adding a row's two counts. | 2, numstat.go parser | `TestChangedLinesRejectsMalformedAndOverflow/row-overflow` |
| N2.total-overflow: Check overflow when adding a row to the total. | 2, numstat.go parser | `TestChangedLinesRejectsMalformedAndOverflow/total-overflow` |
| N3.root: Later worktree edits do not change the supplied-root-tree count. | 2, numstat.go ChangedLines invocation | `TestChangedLinesUsesSuppliedTrees/root` |
| N3.nested: The operation uses the supplied trees in a nested workspace too. | 2, numstat.go ChangedLines invocation | `TestChangedLinesUsesSuppliedTrees/nested` |
| N3.rename: Rename detection stays disabled; count deletion plus addition. | 2, numstat.go ChangedLines invocation | `TestChangedLinesUsesSuppliedTrees/rename` |
| N4.numstat: Request numstat output. | 2, numstat.go argv | `TestChangedLinesCommandPins/numstat` |
| N4.nul: Request NUL termination. | 2, numstat.go argv | `TestChangedLinesCommandPins/nul` |
| N4.no-renames: Pass the no-renames flag. | 2, numstat.go argv | `TestChangedLinesCommandPins/no-renames` |
| N4.no-ext-diff: Disable external diff. | 2, numstat.go argv | `TestChangedLinesCommandPins/no-ext-diff` |
| N4.no-textconv: Disable text conversion. | 2, numstat.go argv | `TestChangedLinesCommandPins/no-textconv` |
| N4.no-color: Disable color. | 2, numstat.go argv | `TestChangedLinesCommandPins/no-color` |
| N4.submodules: Include submodule changes. | 2, numstat.go argv | `TestChangedLinesCommandPins/submodules` |
| N4.end-options: Terminate tree arguments with the pathspec separator. | 2, numstat.go argv | `TestChangedLinesCommandPins/end-options` |
| N4.hostile-config: Real counts remain correct under hostile diff configuration. | 2, numstat.go existing runner integration | `TestChangedLinesHostileDiffConfig` |
| N5.git-failure: A Git failure is an error, never zero success. | 2, numstat.go error return | `TestChangedLinesGitFailure` |
| R1.version: Reject an unsupported record version. | 3a, brief_bounds_record.go codec | `TestBriefBoundsRecordSchema/version` |
| R1.required: Reject a missing required key. | 3a, brief_bounds_record.go codec | `TestBriefBoundsRecordSchema/required` |
| R1.unknown: Reject an unknown key. | 3a, brief_bounds_record.go codec | `TestBriefBoundsRecordSchema/unknown` |
| R1.duplicate-key: Reject a duplicate key. | 3a, brief_bounds_record.go codec | `TestBriefBoundsRecordSchema/duplicate-key` |
| R1.trailing-json: Reject trailing JSON. | 3a, brief_bounds_record.go codec | `TestBriefBoundsRecordSchema/trailing-json` |
| R1.types: Reject wrong JSON field types. | 3a, brief_bounds_record.go codec | `TestBriefBoundsRecordSchema/types` |
| R1.identity: Reject empty job or root identity and nonpositive round. | 3a, brief_bounds_record.go codec | `TestBriefBoundsRecordSchema/identity` |
| R1.digest: Require 64 lowercase hexadecimal digest digits. | 3a, brief_bounds_record.go codec | `TestBriefBoundsRecordSchema/digest` |
| R1.bytes: Require a nonnegative byte count. | 3a, brief_bounds_record.go codec | `TestBriefBoundsRecordSchema/bytes` |
| R1.pair: Reject a partial stored pair. | 3a, brief_bounds_record.go codec | `TestBriefBoundsRecordSchema/pair` |
| R1.null-pair: Preserve the unbounded null pair. | 3a, brief_bounds_record.go codec | `TestBriefBoundsRecordSchema/null-pair` |
| R1.empty-boundary: Preserve bounded empty-array denial. | 3a, brief_bounds_record.go codec | `TestBriefBoundsRecordSchema/empty-boundary` |
| R1.ceiling-range: Preserve the nonnegative int64 ceiling range. | 3a, brief_bounds_record.go codec | `TestBriefBoundsRecordSchema/ceiling-range` |
| R1.member-values: Validate stored members without parsing prose. | 3a, brief_bounds_record.go codec | `TestBriefBoundsRecordSchema/member-values` |
| R2.marker: Carry the exact record hash in the selected task-direction source. | 3a, composition.go marker and identity | `TestCompositionAdmittedBounds/marker` |
| R2.bounded: Set bounded from paired presence, including an empty Boundary. | 3a, composition.go marker and identity | `TestCompositionAdmittedBounds/bounded` |
| R2.unbounded: A null pair produces bounded false. | 3a, composition.go marker and identity | `TestCompositionAdmittedBounds/unbounded` |
| R2.job: Reject a record for another job. | 3a, composition.go marker and identity | `TestCompositionAdmittedBounds/job` |
| R2.round: Reject a record for another round. | 3a, composition.go marker and identity | `TestCompositionAdmittedBounds/round` |
| R2.legacy: Omit the marker when no admission record was supplied. | 3a, composition.go marker and identity | `TestCompositionAdmittedBounds/legacy` |
| R2.delivered-source: Retain augmented SourceDigest and SourceBytes unchanged. | 3a, composition.go marker and identity | `TestCompositionAdmittedBounds/delivered-source` |
| R2.inline-body: Preserve the exact inline delivered body. | 3a, composition.go marker and identity | `TestCompositionAdmittedBounds/inline-body` |
| R2.referenced-body: Preserve the exact staged delivered body. | 3a, composition.go marker and identity | `TestCompositionAdmittedBounds/referenced-body` |
| R3.legacy-seven-fields: Continue admitting the seven-field source. | 3a, build.go optional source admission | `TestCompositionAdmittedBoundsAdmission/legacy-seven-fields` |
| R3.eight-fields: Admit the seven fields plus a valid marker. | 3a, build.go optional source admission | `TestCompositionAdmittedBoundsAdmission/eight-fields` |
| R3.marker-shape: Reject missing or extra marker keys. | 3a, build.go optional source admission | `TestCompositionAdmittedBoundsAdmission/marker-shape` |
| R3.marker-version: Reject an unsupported marker version. | 3a, build.go optional source admission | `TestCompositionAdmittedBoundsAdmission/marker-version` |
| R3.marker-bounded: Require a boolean bounded field. | 3a, build.go optional source admission | `TestCompositionAdmittedBoundsAdmission/marker-bounded` |
| R3.marker-digest: Require a valid record SHA-256 digest. | 3a, build.go optional source admission | `TestCompositionAdmittedBoundsAdmission/marker-digest` |
| R3.wrong-slot: Refuse the marker on another slot. | 3a, build.go optional source admission | `TestCompositionAdmittedBoundsAdmission/wrong-slot` |
| R3.wrong-source: Refuse the marker on another source identity. | 3a, build.go optional source admission | `TestCompositionAdmittedBoundsAdmission/wrong-source` |
| R3.extra-source-field: Refuse unrelated source fields. | 3a, build.go optional source admission | `TestCompositionAdmittedBoundsAdmission/extra-source-field` |
| R3.reference-reader: The strict typed reference reader accepts the new declared source member. | 3a, build.go optional source admission | `TestCompositionAdmittedBoundsAdmission/reference-reader` |
| R4.bytes: Write the admission value's bytes without reopening the caller path. | 3c, brief_bounds_record.go writer | `TestPersistAdmittedBrief/bytes` |
| R4.sha256: Record SHA-256 of those exact bytes. | 3c, brief_bounds_record.go writer | `TestPersistAdmittedBrief/sha256` |
| R4.byte-count: Record their exact byte count. | 3c, brief_bounds_record.go writer | `TestPersistAdmittedBrief/byte-count` |
| R4.parsed-values: Write the parsed values without scanning headers again. | 3c, brief_bounds_record.go writer | `TestPersistAdmittedBrief/parsed-values` |
| R4.headerless: Persist both null fields for an unbounded admission. | 3c, brief_bounds_record.go writer | `TestPersistAdmittedBrief/headerless` |
| R4.copy-failure: Stop setup when the admitted copy cannot be written. | 3c, brief_bounds_record.go writer | `TestPersistAdmittedBrief/copy-failure` |
| R4.record-failure: Stop setup when the record cannot be written. | 3c, brief_bounds_record.go writer | `TestPersistAdmittedBrief/record-failure` |
| R4.identity-required: Persistence requires the complete job, root-job and round identity. | 3c, dispatch_verbs.go persistence flags | `TestDispatchAdmittedBoundsFlags/identity-required` |
| R4.authority-required: Persistence requires authority admission. | 3c, dispatch_verbs.go persistence flags | `TestDispatchAdmittedBoundsFlags/authority-required` |
| R4.mode-only-conflict: Mode-only cannot persist admission evidence. | 3c, dispatch_verbs.go persistence flags | `TestDispatchAdmittedBoundsFlags/mode-only-conflict` |
| R4.refused-no-files: Authority or bounds refusal publishes no admitted files. | 3c, dispatch_verbs.go persistence flags | `TestDispatchAdmittedBoundsFlags/refused-no-files` |
| R4.composition-handoff: Composition receives the persisted structure, not a re-read brief. | 3c, dispatch_verbs.go persistence flags | `TestDispatchAdmittedBoundsFlags/composition-handoff` |
| R4.initial-before-augmentation: Initial admission capture precedes every augmentation. | 3c, dispatch.sh admission and publication | `TestDispatchAdmittedBoundsCallsites/initial-before-augmentation` |
| R4.follow-up-original: Follow-up admission uses authority_message captured before transformations. | 3c, dispatch.sh admission and publication | `TestDispatchAdmittedBoundsCallsites/follow-up-original` |
| R4.initial-composition: Initial composition gets the captured admission record. | 3c, dispatch.sh admission and publication | `TestDispatchAdmittedBoundsCallsites/initial-composition` |
| R4.follow-up-composition: Follow-up composition gets the captured admission record. | 3c, dispatch.sh admission and publication | `TestDispatchAdmittedBoundsCallsites/follow-up-composition` |
| R4.initial-publication: Initial publication follows the winning claim and precedes composition publication. | 3c, dispatch.sh admission and publication | `TestDispatchAdmittedBoundsCallsites/initial-publication` |
| R4.follow-up-publication: Follow-up publication follows the winning claim and precedes composition publication. | 3c, dispatch.sh admission and publication | `TestDispatchAdmittedBoundsCallsites/follow-up-publication` |
| R4.losing-claim: A losing or repeated wrapper cannot overwrite admitted evidence. | 3c, dispatch.sh admission and publication | `TestDispatchAdmittedBoundsCallsites/losing-claim` |
| R4.early-cleanup: Initial capture failures and pre-claim refusals clean temporary admission files before the later setup trap exists. | 3c, dispatch.sh early cleanup | `TestDispatchAdmittedBoundsCallsites/early-cleanup` |
| R4.cleanup: Refused and failed setup clean the temporary admission directory. | 3c, dispatch.sh admission and publication | `TestDispatchAdmittedBoundsCallsites/cleanup` |
| S1.initial-inline: Initial inline delivery with generated live conflicting headers returns the admitted pair. | 3c, dispatch_verbs.go admission-to-composition handoff | `TestDispatchToReviewAdmittedBounds/initial-inline` |
| S1.initial-referenced: Initial referenced delivery returns the admitted pair despite generated conflicting headers. | 3c, dispatch_verbs.go admission-to-composition handoff | `TestDispatchToReviewAdmittedBounds/initial-referenced` |
| S1.follow-up-inline: Follow-up inline delivery uses that round's original admitted bytes. | 3c, dispatch_verbs.go admission-to-composition handoff | `TestDispatchToReviewAdmittedBounds/follow-up-inline` |
| S1.follow-up-referenced: Follow-up referenced delivery uses that round's original admitted bytes. | 3c, dispatch_verbs.go admission-to-composition handoff | `TestDispatchToReviewAdmittedBounds/follow-up-referenced` |
| S1.recorded-headerless: A generated pair cannot bound a headerless admitted round. | 3c, dispatch_verbs.go admission-to-composition handoff | `TestDispatchToReviewAdmittedBounds/recorded-headerless` |
| S1.current: Use the supplied job's round. | 3b, brief_bounds_source.go selection | `TestReviewBriefBoundsRoundIsolation/current` |
| S1.later: A later job cannot replace the supplied job's limits. | 3b, brief_bounds_source.go selection | `TestReviewBriefBoundsRoundIsolation/later` |
| S1.headerless-follow-up: A recorded null pair inherits no prior bound. | 3b, brief_bounds_source.go selection | `TestReviewBriefBoundsRoundIsolation/headerless-follow-up` |
| S1.job: Refuse source metadata naming another job. | 3b, brief_bounds_source.go selection | `TestReviewBriefBoundsRoundIsolation/job` |
| S1.round: Refuse source metadata naming another round. | 3b, brief_bounds_source.go selection | `TestReviewBriefBoundsRoundIsolation/round` |
| S1.missing-source: Require a task-direction source in present composition. | 3b, brief_bounds_source.go selection | `TestReviewBriefBoundsRoundIsolation/missing-source` |
| S1.duplicate-source: Require exactly one task-direction source. | 3b, brief_bounds_source.go selection | `TestReviewBriefBoundsRoundIsolation/duplicate-source` |
| S1.wrong-source: Require caller:brief for that source. | 3b, brief_bounds_source.go selection | `TestReviewBriefBoundsRoundIsolation/wrong-source` |
| S1.other-slot: Ignore header text in other packet slots. | 3b, brief_bounds_source.go selection | `TestReviewBriefBoundsRoundIsolation/other-slot` |
| S2.range: Validate the selected source range within prompt.md. | 3b, brief_bounds_source.go delivered-range checks | `TestReviewBriefBoundsRejectsCorruptSource/range` |
| S2.delivered-digest: Verify that range's DeliveredDigest. | 3b, brief_bounds_source.go delivered-range checks | `TestReviewBriefBoundsRejectsCorruptSource/delivered-digest` |
| S3.bytes: Verify inline SourceBytes. | 3b, brief_bounds_source.go inline verification | `TestReviewBriefBoundsInlineIdentity/bytes` |
| S3.digest: Verify inline SourceDigest. | 3b, brief_bounds_source.go inline verification | `TestReviewBriefBoundsInlineIdentity/digest` |
| S3.envelope: Verify the exact heading and newline envelope. | 3b, brief_bounds_source.go inline verification | `TestReviewBriefBoundsInlineIdentity/envelope` |
| S3.missing-reference: A missing reference cannot make its stub into an inline source. | 3b, brief_bounds_source.go inline verification | `TestReviewBriefBoundsInlineIdentity/missing-reference` |
| S3.no-final-newline: Accept a lawful body lacking its own final newline. | 3b, brief_bounds_source.go inline verification | `TestReviewBriefBoundsInlineIdentity/no-final-newline` |
| S4.slot: Select only a task-direction reference. | 3b, brief_bounds_source.go reference selector and pure binding helper | `TestReviewBriefBoundsRejectsUnboundReference/slot` |
| S4.duplicate: Refuse more than one task-direction reference. | 3b, brief_bounds_source.go reference selector and pure binding helper | `TestReviewBriefBoundsRejectsUnboundReference/duplicate` |
| S4.digest: Join reference digest to SourceDigest. | 3b, brief_bounds_source.go reference selector and pure binding helper | `TestReviewBriefBoundsRejectsUnboundReference/digest` |
| S4.bytes: Join reference bytes to SourceBytes. | 3b, brief_bounds_source.go reference selector and pure binding helper | `TestReviewBriefBoundsRejectsUnboundReference/bytes` |
| S4.path-empty: Require a nonempty Path. | 3b, brief_bounds_source.go reference selector and pure binding helper | `TestReviewBriefBoundsRejectsUnboundReference/path-empty` |
| S4.path-absolute: Require a repository-relative Path. | 3b, brief_bounds_source.go reference selector and pure binding helper | `TestReviewBriefBoundsRejectsUnboundReference/path-absolute` |
| S4.path-parent: Refuse a parent component in Path. | 3b, brief_bounds_source.go reference selector and pure binding helper | `TestReviewBriefBoundsRejectsUnboundReference/path-parent` |
| S4.openpath-relative: Require an absolute OpenPath, tested before file verification. | 3b, brief_bounds_source.go reference selector and pure binding helper | `TestReviewBriefBoundsRejectsUnboundReference/openpath-relative` |
| S4.path-suffix: Require OpenPath slash spelling to end with slash plus Path. | 3b, brief_bounds_source.go reference selector and pure binding helper | `TestReviewBriefBoundsRejectsUnboundReference/path-suffix` |
| S5.escape: Preserve a root-containment verification failure. | 3b, brief_bounds_source.go verified-reference integration | `TestReviewBriefBoundsRejectsCorruptReference/escape` |
| S5.nonregular: Preserve regular-file failure for a symlink to an otherwise valid body. | 3b, brief_bounds_source.go verified-reference integration | `TestReviewBriefBoundsRejectsCorruptReference/nonregular` |
| S5.bytes: Preserve actual referenced byte-count failure. | 3b, brief_bounds_source.go verified-reference integration | `TestReviewBriefBoundsRejectsCorruptReference/bytes` |
| S5.digest: Preserve actual referenced digest failure. | 3b, brief_bounds_source.go verified-reference integration | `TestReviewBriefBoundsRejectsCorruptReference/digest` |
| S5.referenced: Accept a fully bound real referenced source. | 3b, brief_bounds_source.go reference branch | `TestReviewBriefBoundsReferencedSource` |
| S6.missing-bounded-record: A bounded composition with no brief-bounds.json refuses BRIEF_BOUNDS_UNREADABLE. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/missing-bounded-record` |
| S6.missing-unbounded-record: A marked unbounded composition still requires its admission record. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/missing-unbounded-record` |
| S6.missing-copy: Require the retained admitted copy. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/missing-copy` |
| S6.copy-symlink: Refuse a symlink substituted for the retained copy. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/copy-symlink` |
| S6.record-symlink: Refuse a symlink substituted for the structured record. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/record-symlink` |
| S6.record-hash: Verify the record hash from composition. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/record-hash` |
| S6.record-schema: Invalid persisted structure is BRIEF_BOUNDS_UNREADABLE. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/record-schema` |
| S6.record-job: Bind the record's job to the supplied job. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/record-job` |
| S6.record-root: Bind the record's rootJob to the resolved chain root. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/record-root` |
| S6.record-round: Bind the record's round to the supplied round. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/record-round` |
| S6.pair-marker: Require marker bounded to agree with record pair presence. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/pair-marker` |
| S6.admitted-length: Check the retained copy's byte count. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/admitted-length` |
| S6.admitted-digest: Check the retained copy's digest; never fall back on mismatch. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/admitted-digest` |
| S6.job-marker: A marker in embedded job composition cannot disappear from round evidence. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/job-marker` |
| S6.marker-agreement: The job and round admitted markers must agree. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/marker-agreement` |
| S6.no-prose-parse: Read structured values only; a source call-graph assertion forbids ParseBriefBounds in review or composition. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/no-prose-parse` |
| S6.legacy-root: No legacy composition or admission artifacts means unbounded, without reading root brief.md. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/legacy-root` |
| S6.legacy-follow-up: An old follow-up without admitted evidence stays unbounded, without scanning prompt headers. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/legacy-follow-up` |
| S6.legacy-composition: Valid old composition with no marker and no admission artifacts stays unbounded. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/legacy-composition` |
| S6.orphan: Admission artifacts without a composition marker refuse. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/orphan` |
| S6.bad-composition: Invalid present composition cannot fall back to legacy behavior. | 3b, brief_bounds_source.go admitted record reader | `TestReviewBriefBoundsAdmittedRecord/bad-composition` |
| P1.root: Root members stay unchanged. | 4a, brief_bounds.go projection integration | `TestBriefBoundaryNestedPathProjection/root` |
| P1.nested: Match projected members against projected Git paths. | 4a, brief_bounds.go projection integration | `TestBriefBoundaryNestedPathProjection/nested` |
| P1.missing-prefix: Keep the missing literal-prefix refusal. | 4a, brief_bounds.go projection integration | `TestBriefBoundaryNestedPathProjection/missing-prefix` |
| P1.double-prefix: Strip the prefix once. | 4a, brief_bounds.go projection integration | `TestBriefBoundaryNestedPathProjection/double-prefix` |
| P1.unsupported-prefix: Refuse a bounded target prefix with pattern bytes before matching. | 4a, brief_bounds.go projection integration | `TestBriefBoundaryNestedPathProjection/unsupported-prefix` |
| P2.sibling: A whole-project directory admits the inside path and names only the sibling as outside. | 4a, brief_bounds.go raw-path guard | `TestBriefBoundaryWholeProjectExcludesSibling` |
| P3.root: Root path bytes remain unchanged. | 4a, conformance.go installationPath | `TestInstallationPathPreservesGitIdentity/root` |
| P3.nested: Strip one literal installation prefix. | 4a, conformance.go installationPath | `TestInstallationPathPreservesGitIdentity/nested` |
| P3.backslash: Preserve a Git filename backslash. | 4a, conformance.go installationPath | `TestInstallationPathPreservesGitIdentity/backslash` |
| P3.whitespace: Preserve leading and trailing filename whitespace. | 4a, conformance.go installationPath | `TestInstallationPathPreservesGitIdentity/whitespace` |
| P3.double-prefix: Preserve the second prefix. | 4a, conformance.go installationPath | `TestInstallationPathPreservesGitIdentity/double-prefix` |
| P3.waiver: The existing waiver caller still protects project plans. | 4a, conformance.go installationPath integration | `TestNestedWaiverProtectsProjectPlans` |
| P4.exact: A concrete member matches exactly one whole path. | 4a, brief_bounds.go matcher | `TestBriefBoundaryMatching/exact` |
| P4.case: Matching stays case-sensitive. | 4a, brief_bounds.go matcher | `TestBriefBoundaryMatching/case` |
| P4.directory-depth: A literal directory permits arbitrary descendant depth. | 4a, brief_bounds.go matcher | `TestBriefBoundaryMatching/directory-depth` |
| P4.directory-edge: A literal directory cannot match a similarly named sibling. | 4a, brief_bounds.go matcher | `TestBriefBoundaryMatching/directory-edge` |
| P4.literal-bracket-directory: Directory metacharacters remain literal. | 4a, brief_bounds.go matcher | `TestBriefBoundaryMatching/literal-bracket-directory` |
| P4.star: Star cannot cross a slash. | 4a, brief_bounds.go matcher | `TestBriefBoundaryMatching/star` |
| P4.question: Question mark matches exactly one non-slash character. | 4a, brief_bounds.go matcher | `TestBriefBoundaryMatching/question` |
| P4.class: A character class matches only its declared range. | 4a, brief_bounds.go matcher | `TestBriefBoundaryMatching/class` |
| P4.escape: Backslash escape retains path.Match semantics for an ordinary path, such as escaped a matching a. | 4a, brief_bounds.go matcher | `TestBriefBoundaryMatching/escape` |
| P4.double-star: Double star is not recursive. | 4a, brief_bounds.go matcher | `TestBriefBoundaryMatching/double-star` |
| P4.unmatched: An unmatched valid pattern permits no path. | 4a, brief_bounds.go matcher | `TestBriefBoundaryMatching/unmatched` |
| P4.pattern-byte-path: Patterns cannot authorise Git paths containing pattern bytes, even when path.Match would succeed. | 4a, brief_bounds.go matcher | `TestBriefBoundaryMatching/pattern-byte-path` |
| P4.backslash-path: A pattern spelling that would match a literal backslash path still cannot authorise it. | 4a, brief_bounds.go matcher | `TestBriefBoundaryMatching/backslash-path` |
| P4.directory-special-path: A trailing-slash ancestor may authorise a special-byte Git path. | 4a, brief_bounds.go matcher | `TestBriefBoundaryMatching/directory-special-path` |
| P4.backslash-end-to-end: Observe JSON decoding, projection, concrete authority eligibility, exact lookup, persistence and final matching for one real backslash Git path. | 4a, brief_bounds.go matching restriction | `TestBriefBackslashEndToEnd` |
| P5.empty: An empty Boundary denies every changed path. | 4a, brief_bounds.go offender collection | `TestBriefBoundaryDenies/empty` |
| P5.binary: A binary changed path remains subject to Boundary. | 4a, brief_bounds.go offender collection | `TestBriefBoundaryDenies/binary` |
| V1.fields: The typed Boundary error carries every outside repository path and the whole candidate count. | 4a, brief_bounds.go Boundary construction | `TestBriefBoundaryViolationFields` |
| V2.below: Counts below Ceiling pass. | 4a, brief_bounds.go Ceiling comparison and construction | `TestBriefCeilingViolation/below` |
| V2.equal: Counts equal to Ceiling pass. | 4a, brief_bounds.go Ceiling comparison and construction | `TestBriefCeilingViolation/equal` |
| V2.above: Counts above Ceiling refuse with exact fields. | 4a, brief_bounds.go Ceiling comparison and construction | `TestBriefCeilingViolation/above` |
| V2.zero: Zero permits no added or deleted text line. | 4a, brief_bounds.go Ceiling comparison and construction | `TestBriefCeilingViolation/zero` |
| V2.binary-path: The Ceiling diagnostic includes binary changed paths. | 4a, brief_bounds.go Ceiling comparison and construction | `TestBriefCeilingViolation/binary-path` |
| V3.json-escaping: Encode every diagnostic path as JSON. | 4a, brief_bounds.go formatting | `TestBriefBoundsViolationDiagnostics/json-escaping` |
| V3.sorting: Sort paths deterministically. | 4a, brief_bounds.go formatting | `TestBriefBoundsViolationDiagnostics/sorting` |
| V3.deduplication: Deduplicate diagnostic paths. | 4a, brief_bounds.go formatting | `TestBriefBoundsViolationDiagnostics/deduplication` |
| V3.full-list: Never truncate the full list; use 257 paths including odd characters. | 4a, brief_bounds.go formatting | `TestBriefBoundsViolationDiagnostics/full-list` |
| V3.both-order: Boundary precedes Ceiling when both fail. | 4a, brief_bounds.go formatting | `TestBriefBoundsViolationDiagnostics/both-order` |
| C1.snapshot: Exactly one repository snapshot supplies every tree and the patch, observed with the post-write-tree edit interposer. | 4b, conformance.go snapshot wiring | `TestReviewBriefSingleSnapshot` |
| C2.base: Use the merge-base against the target, not baseSha or current HEAD, for the complete candidate. | 4b, conformance.go base-tree arguments | `TestReviewBriefUsesMergeBaseCandidate` |
| C2.project: Derive project output from the captured repository tree. | 4b, conformance.go subtree wiring | `TestNestedReviewStageSpeaksProjectSpace` |
| C3.committed: Include committed changes since the merge-base. | 4b, conformance.go repository Snapshot integration | `TestReviewBriefSnapshotMembership/committed` |
| C3.staged-then-unstaged: Count the working copy once when it supersedes staged content. | 4b, conformance.go repository Snapshot integration | `TestReviewBriefSnapshotMembership/staged-then-unstaged` |
| C3.deleted: Include tracked deletion. | 4b, conformance.go repository Snapshot integration | `TestReviewBriefSnapshotMembership/deleted` |
| C3.untracked: Include untracked unignored work. | 4b, conformance.go repository Snapshot integration | `TestReviewBriefSnapshotMembership/untracked` |
| C3.ignored: Exclude ignored untracked work. | 4b, conformance.go repository Snapshot integration | `TestReviewBriefSnapshotMembership/ignored` |
| C3.tracked-ignored: Include ignored files already tracked. | 4b, conformance.go repository Snapshot integration | `TestReviewBriefSnapshotMembership/tracked-ignored` |
| C3.mode: Include mode changes as changed paths. | 4b, conformance.go repository Snapshot integration | `TestReviewBriefSnapshotMembership/mode` |
| C3.symlink: Include a changed symlink. | 4b, conformance.go repository Snapshot integration | `TestReviewBriefSnapshotMembership/symlink` |
| C3.gitlink: Include a changed gitlink. | 4b, conformance.go repository Snapshot integration | `TestReviewBriefSnapshotMembership/gitlink` |
| C3.real-index: Preserve the real index byte-for-byte. | 4b, conformance.go repository Snapshot integration | `TestReviewBriefSnapshotMembership/real-index` |
| E1.boundary: Invoke Boundary enforcement before artifact writes. | 4c, conformance.go review integration | `TestReviewBriefBoundsEnforced/boundary` |
| E1.ceiling: Invoke Ceiling enforcement before artifact writes. | 4c, conformance.go review integration | `TestReviewBriefBoundsEnforced/ceiling` |
| E1.both-admitted: Admit a candidate within both bounds. | 4c, conformance.go review integration | `TestReviewBriefBoundsEnforced/both-admitted` |
| E1.invalid-record: Refuse malformed recorded bounds before artifact writes. | 4c, conformance.go review integration | `TestReviewBriefBoundsEnforced/invalid-record` |
| E2.skip-numstat: A headerless review makes zero numstat calls, observed with the interposer. | 4c, conformance.go unbounded branch | `TestReviewBriefHeaderlessSkipsNumstat` |
| E2.fence: Keep outside-project-first diagnostics. | 4c, conformance.go unbounded ordering | `TestReviewBriefHeaderlessCompatibility/fence` |
| E2.declaration: Keep cumulative declaration refusal. | 4c, conformance.go unbounded ordering | `TestReviewBriefHeaderlessCompatibility/declaration` |
| E2.immutable: Keep immutable-evidence diagnostic precedence. | 4c, conformance.go unbounded ordering | `TestReviewBriefHeaderlessCompatibility/immutable` |
| E3.repository: A sibling contributes both the typed Boundary violation and project fence, and contributes to the total count. | 4c, conformance.go collected bounded policy | `TestReviewBriefNestedWholeRepository` |
| E3.order: Through review, emit Boundary before Ceiling and existing policy diagnostics after them. | 4c, conformance.go diagnostic collection | `TestReviewBriefBothViolationOrder` |
| E4.captured: Count the captured tree even when the worktree changes after snapshot. | 4c, conformance.go ChangedLines arguments | `TestReviewBriefCountUsesCapturedTree` |
| E4.ceiling: A smaller current Ceiling still counts earlier unlanded changes. | 4c, conformance.go current-round integration | `TestReviewBriefFollowUpCountsEarlierChanges/ceiling` |
| E4.boundary: A smaller current Boundary still sees earlier unlanded changed paths. | 4c, conformance.go current-round integration | `TestReviewBriefFollowUpCountsEarlierChanges/boundary` |
| E5.unchanged: An unchanged extra concrete declaration needs no Boundary permission. | 4c, conformance.go retained cumulative integration | `TestReviewBriefDeclarationIsNotPermission/unchanged` |
| E5.actual-outside: A return declaration cannot permit actual outside work. | 4c, conformance.go retained cumulative integration | `TestReviewBriefDeclarationIsNotPermission/actual-outside` |
| E5.undeclared-inside: A permitted but undeclared actual path still refuses. | 4c, conformance.go retained cumulative integration | `TestReviewBriefDeclarationIsNotPermission/undeclared-inside` |
| E5.plans: Broad permission does not permit protected plans changes. | 4c, conformance.go protected-path integration | `TestReviewBriefProtectedPathsStillRefuse/plans` |
| E5.control-plane: Broad permission does not permit agent control-plane changes. | 4c, conformance.go protected-path integration | `TestReviewBriefProtectedPathsStillRefuse/control-plane` |
| E6.source: Render exact BRIEF_BOUNDS_UNREADABLE and write neither artifact. | 4c, conformance.go error routing | `TestReviewBriefUnreadableWrappers/source` |
| E6.lines: Failed numstat renders BRIEF_LINES_UNREADABLE and writes neither artifact. | 4c, conformance.go error routing | `TestReviewBriefUnreadableWrappers/lines` |
| E6.projection: Unsupported target prefix retains BRIEF_BOUNDS_INVALID and writes neither artifact. | 4c, conformance.go error routing | `TestReviewBriefUnreadableWrappers/projection` |
| E7.first-refusal: A first bounded refusal writes neither success artifact. | 4c, conformance.go evidence and bounded failure paths | `TestReviewBriefLeavesEvidenceImmutable/first-refusal` |
| E7.success-shape: Successful review.json contains exactly three fields. | 4c, conformance.go evidence and bounded failure paths | `TestReviewBriefLeavesEvidenceImmutable/success-shape` |
| E7.identical: Identical review reuses existing bytes. | 4c, conformance.go evidence and bounded failure paths | `TestReviewBriefLeavesEvidenceImmutable/identical` |
| E7.later-boundary: A later Boundary failure names its type before the immutable diagnostic and preserves both files. | 4c, conformance.go evidence and bounded failure paths | `TestReviewBriefLeavesEvidenceImmutable/later-boundary` |
| E7.later-ceiling: A later Ceiling failure names its type before the immutable diagnostic and preserves both files. | 4c, conformance.go evidence and bounded failure paths | `TestReviewBriefLeavesEvidenceImmutable/later-ceiling` |
| E7.later-source-error: A later unreadable record names its type before the immutable diagnostic and preserves both files. | 4c, conformance.go evidence and bounded failure paths | `TestReviewBriefLeavesEvidenceImmutable/later-source-error` |
| E7.changed-success: A different otherwise-valid candidate cannot overwrite prior evidence. | 4c, conformance.go evidence and bounded failure paths | `TestReviewBriefLeavesEvidenceImmutable/changed-success` |
| D1.invalid: Register this code with its specified classification and remedy. | 1a, register.go owning row | `TestBriefBoundsRefusalRegistration/invalid` |
| D1.bounds-unreadable: Register this code with its specified classification and remedy. | 3b, register.go owning row | `TestBriefBoundsRefusalRegistration/bounds-unreadable` |
| D1.boundary: Register this code with its specified classification and remedy. | 4a, register.go owning row | `TestBriefBoundsRefusalRegistration/boundary` |
| D1.ceiling: Register this code with its specified classification and remedy. | 4a, register.go owning row | `TestBriefBoundsRefusalRegistration/ceiling` |
| D1.lines-unreadable: Register this code with its specified classification and remedy. | 4c, register.go owning row | `TestBriefBoundsRefusalRegistration/lines-unreadable` |
| W1.orchestration: The orchestration section requires the seat command before dispatch with the 0/1/2 mapping. | 6a, orchestration.md and code-critique/SKILL.md owning sentence | `TestBriefBoundsSeatOrderingInstructions/orchestration` |
| W1.code-critique: The skill section requires the seat command before dispatch with the 0/1/2 mapping. | 6a, orchestration.md and code-critique/SKILL.md owning sentence | `TestBriefBoundsSeatOrderingInstructions/code-critique` |
| W2.brief-pair: The initial template teaches pair-or-neither. | 6a, owning brief template | `TestBriefBoundsTemplateInstructions/brief-pair` |
| W2.follow-up-pair: The follow-up template teaches pair-or-neither. | 6a, owning brief template | `TestBriefBoundsTemplateInstructions/follow-up-pair` |
| W2.follow-up-cumulative: The follow-up template requires repeating whole-candidate limits. | 6a, owning brief template | `TestBriefBoundsTemplateInstructions/follow-up-cumulative` |
| W2.brief-example: The initial template example is paired and inert to parser and authority extraction. | 6a, owning brief template | `TestBriefBoundsTemplateInstructions/brief-example` |
| W2.follow-up-example: The follow-up example is paired and inert to parser and authority extraction. | 6a, owning brief template | `TestBriefBoundsTemplateInstructions/follow-up-example` |
| W3.permission: The implementer role distinguishes scope permission from concrete declarations. | 6a, owning role or skill sentence | `TestBriefBoundsRoleInstructions/permission` |
| W3.semantic-review: The critic skill retains semantic conformance review. | 6a, owning role or skill sentence | `TestBriefBoundsRoleInstructions/semantic-review` |
| W4.missing-return: Name the predecessor missing return diffBoundary. | 6a, capcontinuation.go paragraph | `TestCapContinuationBounds/missing-return` |
| W4.current-pair: Say the current pair covers the whole candidate. | 6a, capcontinuation.go paragraph | `TestCapContinuationBounds/current-pair` |
| W4.no-permission: Say diffBoundary grants no extra permission. | 6a, capcontinuation.go paragraph | `TestCapContinuationBounds/no-permission` |
| W4.cumulative: Preserve the requirement to list predecessor changed paths. | 6a, capcontinuation.go paragraph | `TestCapContinuationBounds/cumulative` |
| W5.admit: Review without a critic returns exit 0 and success artifacts. | 6b, validate_verbs.go relay | `TestConformanceBriefCLIWithoutCritic/admit` |
| W5.refuse: Review without a critic returns exit 1 with exact stderr. | 6b, validate_verbs.go relay | `TestConformanceBriefCLIWithoutCritic/refuse` |
| W5.usage: Bad public invocation returns exit 2. | 6b, validate_verbs.go relay | `TestConformanceBriefCLIWithoutCritic/usage` |
| W5.help: Public help names paired limits and headerless compatibility. | 6b, validate_verbs.go help | `TestConformanceBriefBoundsHelp` |
| W6.brief-boundary-unlisted: Keep this public-verb fixture leg, its inputs, status and artifact assertions. | 6b, conformance-fixtures.sh named block | `TestConformanceBriefFixtureLegs/brief-boundary-unlisted` |
| W6.brief-ceiling-exceeded: Keep this public-verb fixture leg, its inputs, status and artifact assertions. | 6b, conformance-fixtures.sh named block | `TestConformanceBriefFixtureLegs/brief-ceiling-exceeded` |
| W6.brief-both-admitted: Keep this public-verb fixture leg, its inputs, status and artifact assertions. | 6b, conformance-fixtures.sh named block | `TestConformanceBriefFixtureLegs/brief-both-admitted` |
| W6.brief-no-headers: Keep this public-verb fixture leg, its inputs, status and artifact assertions. | 6b, conformance-fixtures.sh named block | `TestConformanceBriefFixtureLegs/brief-no-headers` |
| W6.brief-no-headers-undeclared: Keep this public-verb fixture leg, its inputs, status and artifact assertions. | 6b, conformance-fixtures.sh named block | `TestConformanceBriefFixtureLegs/brief-no-headers-undeclared` |
| W6.brief-missing-ceiling: Keep this public-verb fixture leg, its inputs, status and artifact assertions. | 6b, conformance-fixtures.sh named block | `TestConformanceBriefFixtureLegs/brief-missing-ceiling` |
| W6.brief-missing-boundary: Keep this public-verb fixture leg, its inputs, status and artifact assertions. | 6b, conformance-fixtures.sh named block | `TestConformanceBriefFixtureLegs/brief-missing-boundary` |

The single-snapshot and no-numstat tests observe operations. The full diagnostic list is never abbreviated. Existing dependencies remain intact during the owning integration mutation. Instruction witnesses remove only the stated sentence from the relevant unit's Boundary. Static shell and instruction assertions prove the handoff is installed; the seat's actual bed and pre-read result supply execution evidence.

## Amendment after unit 1b's read (2026-09-15): citation identity narrowed, unit 1b split

Unit 1a landed at e461dfd1. Unit 1b's first build followed this page's citation paragraph in section "Paths and matching" literally. That paragraph says a backtick span or JSON string is one citation. Read literally, it changed authority results for bounded briefs in both directions. Backticked `path:line`, commands with arguments, placeholders and globs were refused. A quoted sentence hid a missing path. One real implementer brief (artifacts/agents/implementer-b76a0ddc11f73e99fa195d84/brief.md) flipped from admitted to refused. The read is records/misc/brief-declares-the-round-boundary-unit-1b-code-read.md, finding F-1.

Seat ruling A1 (amends that paragraph): a backtick span or JSON string is consumed as one authority citation only when its decoded content is exactly equal to a member of the brief's Boundary. Every other span, and all text outside spans, is scanned by the token scanner exactly as trunk does today. That includes its `*<>${` exclusions and its handling of `path:line`, commands with arguments, placeholders and globs. The input-wins rule still holds for a special-character Boundary member, and every other citation gives the same result as trunk.

Seat ruling A2 (size): unit 1b is split into two units, each at most 400 changed lines.
- Unit 1b-i lands the paired-bounds admission wiring, the `--mode-only` and `--root` operands, the carry-ins from unit 1a's reads, and the read's F-2, F-3 and F-5. In 1b-i, the token scanner stays exactly trunk's. The line handling of rows A1 and A2 lands with it: the live Boundary and Ceiling header lines and an indented bounds example are not scanned as citations. Those two rows change authority results only where this page intends, for a Boundary naming a new file and for an inert example line. Unit 1b-i owns rows A1, A2 and H8.
- Unit 1b-ii lands ruling A1 and F-4's incomplete-span cases. It owns rows A3, A4 and A5. Row A3 holds in the narrowed form of ruling A1: the special-character rows apply to a citation equal to a Boundary member, and every other span gets trunk's result. Row A4 is concrete-member eligibility and input accounting, and row A5 is compatibility with the bounded lexer. Its witnesses: each of the five shapes gives the same result as trunk, a quoted sentence containing a missing path is still refused, a backticked Boundary member containing a space satisfies the input-wins rule, and an unterminated backtick or JSON string falls back to the token scanner.

Seat ruling A3 (error precedence, 1b-i read finding F-13): the installation-prefix lookup runs only after the Working Mode check and a complete pair whose Ceiling and members are valid without the prefix. So a malformed Ceiling is now reported before a Boundary member error that needs the prefix. This amends the order in section "Header contract" ("validate Boundary, then Ceiling") for that one case.

Seat ruling A4 (unit 1b-ii read finding F-31; narrows ruling A1): a backtick span or JSON string is consumed and recorded as one authority citation only when its decoded content exactly equals a Boundary member that is a concrete file path. A member is not concrete when it is a directory declaration, a pattern (a glob metacharacter or a backslash), contains one of the token scanner's exclusions, or ends in a line suffix. A span equal to a member that is not concrete gets the token scanner's result, so a directory, glob, placeholder or path:line Boundary member repeated on an input line is admitted as on trunk. Row A3's backslash case therefore asserts the scanner's result in unit 1b-ii, and the backslash end-to-end test of unit 4a is re-derived under rulings A1 and A4 when that unit is briefed.

Landing note (read finding F-10): after 1b-i lands, rebuild every seat's engine (`scripts/agents/go-build.sh`, then `up --repo`). An engine older than 1b-i rejects `--mode-only`, and every dispatch would fail at dispatch.sh:1542.

## Critique record

Round 1 reported eight material findings. Round 2 reported three material findings and reopened R1-04 and R1-07. The seat's round-2 rulings settle the source contract, path classification, and witness granularity. These dispositions fold all remaining findings into this spec. They do not claim passing implementation proof.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| BDRB-R1-01 | accepted, retained | Current declarations project the prefix and Git reports repository paths (`internal/validate/conformance.go:284-293`, `internal/gittree/gittree.go:372-386`). | P1-P3 and E3 keep both sides in project space after the sibling guard. Whole-project directory permission cannot omit the sibling from the typed violation. |
| BDRB-R1-02 | accepted, retained | The seat binds both-or-neither to DONE (`plans/goals/brief-declares-the-round-boundary.md:8`). | H2.boundary-only and H2.ceiling-only name the two missing-header refusals. H8 and both missing-header bed legs prove public admission. |
| BDRB-R1-03 | accepted, retained | Revision 1's unit 5 allowed only tests while requiring production mutations (`artifacts/reports/bdrb-design.md:310-315`, repository-root citation). | Its witnesses stay in production-owning units 4b, 4c and 6b. New record and seam witnesses likewise stay with units 3a, 3b and 3c. |
| BDRB-R1-04 | accepted, reopened by R2-03, folded here | The round-2 missing cases concern distinct guards and transformations. | Every witness row is independently named. S4 has separate Path/OpenPath guards; P4 has question, class and escape cases; A3 has both incomplete-span cases; unit 3c owns the actual admitted-versus-augmented seam. |
| BDRB-R1-05 | accepted, retained | ReadVerifiedReference does not join source metadata (`internal/dispatch/references.go:24-49`); admission does (`internal/dispatch/build.go:1066-1077`). | Preserve source/reference slot, digest and byte-count binding. S4 tests the pure binding helper independently of file verification. |
| BDRB-R1-06 | accepted, retained | Current extraction scans indented lines as authority (`internal/dispatch/brief.go:128-160`). | A2 names the exact space/tab exemption and the separate-input and quote guards. |
| BDRB-R1-07 | accepted, reopened by R2-02, folded here | The old token grammar excludes special bytes (`internal/dispatch/brief.go:13-18`). Pattern spelling also differs from concrete path identity. | A3 preserves structured citation identity. A4 limits equality eligibility to concrete members. P4.backslash-end-to-end observes the full backslash path through decoding, persistence, projection, equality and matching. |
| BDRB-R1-08 | accepted, retained | Existing return matching provides no glob or directory expansion (`internal/validate/conformance.go:798-810`). | H6.literal-directory admits a literal bracket directory. H6.malformed-pattern refuses the non-directory form. P4.literal-bracket-directory proves matching. |
| BDRB-R1-N01 | noted, non-material | The admission and fake reference readers were checked (`internal/dispatch/build.go:1099-1103`, `scripts/agents/adapters/fake.sh:67`, `scripts/agents/adapters/fake.sh:231-236`). | No separate action. The material source-binding work has its own findings and owners. |
| BDRB-R1-N02 | noted, non-material | The fixture helper creates an isolated case (`scripts/agents/conformance-fixtures.sh:38-64`). | No separate action. The bed table names seven executions including compatibility and pairing guards. |
| BDRB-R2-01 | accepted, seat ruling folded | Admission precedes augmentation (`scripts/agents/dispatch.sh:1655`, `scripts/agents/dispatch.sh:1736-1742`, `scripts/agents/dispatch.sh:2643`, `scripts/agents/dispatch.sh:2697-2709`). caller:brief is the delivered body (`internal/dispatch/composition.go:237-268`). | R1-R4 specify the admitted copy, record schema, marker, hashes, write sites and cleanup. S6.missing-bounded-record and S6.admitted-digest enforce refusal. Unit 3c owns persistence and all five TestDispatchToReviewAdmittedBounds cases. Units 1a/1b avoid duplicate admission parsing. Delivered payload assertions remain unchanged. |
| BDRB-R2-02 | accepted, seat ruling folded | Literal prefix projection currently precedes comparison (`internal/validate/conformance.go:284-293`). Concrete return equality supplies no pattern-to-path conversion (`internal/validate/conformance.go:798-810`). | The single pattern/concrete paragraph binds classification and special-path permission. A4.pattern-no-equality and A4.backslash-no-equality pin authority. H6.unsupported-prefix and P1.unsupported-prefix refuse unsupported installations. P4.backslash-end-to-end covers the full seam in one test. |
| BDRB-R2-03 | accepted, seat ruling folded | Separate Path/OpenPath checks exist at `internal/dispatch/build.go:1050-1062`; current token scanning acts on fragments at `internal/dispatch/brief.go:141-160`. | S4.path-empty, S4.path-absolute, S4.path-parent, S4.openpath-relative and S4.path-suffix are separate helper subtests. P4.question, P4.class and P4.escape are separate. A3.incomplete-backtick and A3.incomplete-json are separate. S1's five real dispatcher-helper seam cases live with the writer in 3c. |
| BDRB-R2-N01 | noted, non-material | Headerless mode emitters need no bounds change. The root-brief mirror copies bytes (`internal/dispatch/mirror.go:117-118`), and its round walker includes regular round files (`internal/dispatch/mirror.go:120-139`). | No separate action on the omitted emitter inventory. Preserve the emitters and mirror. The optional marker's actual strict readers are dispositioned above. |
| BDRB-R2-N02 | noted, non-material | The prior allocations were estimates. No measured patch follows from their arithmetic. | No separate behavior change. R2-01 owns the required unit recut. All eleven resulting unit allocations are at or below 400 changed lines. |

All eleven material finding ids across the two rounds have accepted dispositions and named witnesses. All four non-material ids are retained. R1-04 and R1-07 remain visible as reopened findings discharged into specific build obligations. Every unit requires a code read and remove-one evidence. The prose loop ends here under R-97-m1e (`memory/rulings.md:156`) and D81 (`docs/reviews/2026-08-13-delegated-decisions.md:715-721`). There is no third critique round.

## Completion obligations and remaining choice

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| BDRB-1 | HIGH | Header contract; Paths and matching | Paired syntax, exact identity, inert examples, and legacy admission | dispatch | brief.go | H1-H8, A1-A5 | Missing-header legs and public admission Go tests | PARTIAL | Build 1a/1b and retain mutation evidence |
| BDRB-2 | HIGH | Which brief binds a round | Persist admitted bytes and parsed bounds before augmentation; review verifies their binding | dispatch owns codec and persistence; validate owns consumption | brief_bounds_record.go, composition.go, build.go, dispatch.sh, brief_bounds_source.go | R1-R4, S1-S6 | TestDispatchToReviewAdmittedBounds through dispatcher helpers; existing dispatch bed payload assertions | PARTIAL | Build 3a/3b/3c and retain record, digest and source witnesses |
| BDRB-3 | HIGH | Paths and matching; Review enforcement | Separate pattern spelling from concrete authority; refuse every actual outside path with its count | validate | brief_bounds.go and reviewStage | P1-P5, V1-V3, E1/E3/E5 | TestBriefBackslashEndToEnd and brief-boundary-unlisted | PARTIAL | Build 4a/4c and run seat bed |
| BDRB-4 | HIGH | Exact diff and line count | One full merge-base candidate; binary-excluding text count | gittree and validate | ChangedLines and reviewStage | N1-N5, C1-C3, E2/E4 | brief-ceiling-exceeded | PARTIAL | Build 2/4b/4c and inspect operation witnesses |
| BDRB-5 | HIGH | Review enforcement and refusal text | Preserve prior policy and evidence; diagnose bad inputs | validate and refusal | reviewStage and refusal register | E2/E5/E6/E7, D1 | Both-admitted and both headerless legs | PARTIAL | Build 4c and retain exact error/artifact results |
| BDRB-6 | HIGH | The seat's pre-read step | Seat obtains exit 0 before critic dispatch | orchestration and CLI | instruction owners and existing review verb | W1-W6 | Seat command result before the read; seven bed outcomes | PARTIAL | Build 6a/6b and execute seat proof |

No open question belongs to Wido. A default for missing headers would change DONE and is not proposed here.

Design verification: re-read revisions 1 and 2, both critique reports, the goal, current implementation and instruction owners, referenced tests and fixture registration. Joined the three round-2 material ids, both round-2 non-material ids, and all ten round-1 ids to this disposition table. Checked every unit's allowed files, proof ownership and estimated changed-line total. Checked the independent witness names and required header fields. This document is the requested output. No code, tracked file or runtime artifact was changed by this delegate.

Failure behavior is explicit: malformed headers refuse admission; unreadable or unbound admitted evidence and failed counting refuse review; policy failures name every offending path and the total count; prior success artifacts stay unchanged. Git timeouts remain with the bounded runners (`internal/gittree/gittree.go:128-141`, `internal/dispatch/gitcmd.go:47-49`).

No implementation test, Go gate, fixture bed or mutation experiment ran for this prose deliverable. All runtime obligations remain PARTIAL. The seat updates their status from actual per-unit proof and code reads. The remaining implementation risk is fitting code and witnesses within each hard ceiling. Split an implementation unit before exceeding 400; keep its rule and witness together. This is a mechanical unit split, not a new prose critique budget.

This delegate writes only `artifacts/reports/bdrb-design-r3.md`. The seat owns landing this complete replacement page, updating proof status and recording the receipt.

Proposed receipt for the integrating seat: `DESIGN brief-declares-the-round-boundary revision 3: folded all three material round-2 findings and both reopened round-1 findings; admitted bounds record, pattern/concrete rule, independent witnesses, and eleven units capped at 400 changed lines; prose loop closed under R-97-m1e and D81; evidence=read; runtime-proof=pending`.
