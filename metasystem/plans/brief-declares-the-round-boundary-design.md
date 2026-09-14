# brief-declares-the-round-boundary

- Owner: m1e (goal 28 of plans/delivery-efficiency-plan.md). **Revision 1, 2026-09-14**, designed by a Codex gpt-6-astra delegate at the seat's brief after Wido's word of the same day; the seat integrated the page unchanged and critiques it next.
- Goal and current status: the brief carries optional Boundary and Ceiling headers; conformance at review stage refuses a candidate outside them before a read is spent; six builder units of at most 400 changed lines each, in order, each with its own Boundary, Ceiling, Non-goals and the proof rule. Nothing built yet.
- In flight right now: revision 1 under its first critique read (Codex gpt-5.6-sol, design critic); the fold and unit 1 follow.
- Decisions made (and who made them): Wido, 2026-09-14: scope drift and unproved rules are fixed upstream in the brief and the builder's return, not by coordinator checks or the read; units are at most 400 changed lines. Wido, R-93-m1e and R-97-m1e still bind this goal's critique rounds. The design delegate: the bounded diff is the whole candidate against its merge-base with the target checkout; each round's headers apply independently; Boundary is output permission, not cited authority; absent headers keep today's behaviour.
- Waiting on the human: nothing. The one open policy choice, a default Ceiling for briefs that omit it, is proposed at 400 in the last section and needs no answer to build.
- Dead ends (do not retry without new evidence): none yet.
- Next step: revision 1 lands with this line, so the critique read has a landed subject; the read's material findings fold into revision 2, then unit 1 (parse the optional headers at admission) is briefed to Codex gpt-5.6-sol with its own Boundary, Ceiling and proof rule, read by Opus, landed by the seat, and units 2 to 6 follow in order.

The seat must run review conformance before dispatching a code critic. The brief supplies the outer path and size limits. The implementer's return still supplies the exact changed-file declaration. Both checks must pass.

This is a design proposal, checked by reading code at `b594392e`. No implementation or runtime proof is claimed. The contract is the DONE sentence in `plans/goals/brief-declares-the-round-boundary.md:8`. All source citations below are relative to `metasystem/`. Paths in builder Boundary headers are relative to the Git repository root and therefore include `metasystem/`.

The most important decision is the diff being bounded. It is the whole returned candidate against its merge-base with the seat's target checkout. It is not the work since the builder's last commit or last return. This follows the existing base selection in `internal/validate/conformance.go:232-241` and review projection in `internal/validate/conformance.go:368-390`.

## Header contract

An implementer brief may begin with:

```text
Working Mode: implement
Boundary: ["metasystem/internal/dispatch/brief.go", "metasystem/internal/validate/", "metasystem/scripts/agents/*.sh"]
Ceiling: 400
```

Use one physical line per header. Recognize the exact, case-sensitive prefix at column zero, as `BriefMode` does for `Working Mode:` today (`internal/dispatch/brief.go:41-55`). Trim whitespace around the value. Do not scan other packet slots for headers. A quoted or fenced example with a column-zero header inside the authored brief still counts as a header, just as it does for Working Mode today (`internal/dispatch/brief.go:47-53`). Documentation examples should indent their header lines when embedded in a real brief.

`Boundary:` is a JSON array of strings. JSON gives paths containing spaces, commas, tabs, quotes, or newlines an unambiguous spelling. `[]` explicitly allows no changed paths. An absent header has no boundary check. These states must remain distinct. A present but empty value, `null`, a non-array value, a non-string member, or a duplicate Boundary header refuses preflight. Duplicate path entries are harmless and may be deduplicated.

`Ceiling:` is an unsigned decimal spelling of a nonnegative signed 64-bit integer. Leading zeroes are accepted. Zero permits no added or deleted text lines. A sign, decimal point, unit suffix, placeholder, overflow, empty value, or duplicate Ceiling header refuses preflight. Compare with `>`, so equality passes.

With neither header, run neither new check. With only Boundary, run only the new path check. With only Ceiling, run only the new size check. Working Mode remains required at initial dispatch. No implicit maximum is introduced. This preserves the optional-header contract in `plans/goals/brief-declares-the-round-boundary.md:8` without changing the mandatory mode rule in `internal/dispatch/brief.go:52-55`.

The parser belongs in `internal/dispatch/brief.go`:

```go
type BriefBounds struct {
    Boundary []string // nil means absent; a non-nil empty slice means deny all
    Ceiling *int64    // nil means absent
}
type BriefBoundsRefusal struct { Header, Detail string }
func (e *BriefBoundsRefusal) Error() string
func ParseBriefBounds(data []byte) (BriefBounds, error)
func ReadBriefBounds(briefPath string) (BriefBounds, error)
```

Use `BRIEF_BOUNDS_INVALID: <Header>: <detail>` for syntax refusals. Fix the detail strings as `header occurs more than once`, `expected a JSON array of paths`, `invalid path or pattern <JSON string>`, and `expected a nonnegative decimal integer no greater than 9223372036854775807`. Preserve the existing mode diagnostic for a missing or malformed Working Mode. `BriefMode` calls the bounds parser after its mode validation. `ValidateBriefAuthority` calls it without requiring a mode, covering the authority-only path used by follow-ups. The CLI already returns dispatch errors through `recordExit` and keeps mode text on stdout (`cmd/metasystem/dispatch_verbs.go:2014-2027`). Change the shell's generic failure suffix to mention brief headers, so it does not describe a Ceiling refusal as a missing mode (`scripts/agents/dispatch.sh:1542`).

## Paths and matching

Boundary declarations use the repository-relative dialect already projected by `conformanceRun.projectDeclaration`. In a nested installation a declaration must start with the literal installation prefix and `/`. Strip that prefix exactly once. A bare `internal/a.go` does not name `metasystem/internal/a.go`. A doubled prefix names a genuinely nested path. At a repository-root installation no prefix is stripped. Those are the current projection rules in `internal/validate/conformance.go:276-293`.

The new match rule is:

1. A declaration ending in `/` is a literal directory prefix. After projection, `internal/validate/` matches every changed path beginning with that exact prefix. It does not match `internal/validator.go`. Preserve this directory interpretation when projection produces an empty string: `metasystem/` names the whole installed project and matches every project path. No filesystem existence check is involved, so a new directory and a deleted directory work. Glob characters in a trailing-slash declaration are literal directory-name characters; directory prefixes are not glob expansion.
2. A declaration with no glob metacharacters and no trailing slash matches one exact path.
3. Otherwise use Go's slash-based `path.Match` against the whole projected changed path. `*` matches within one path component; `?`, character classes, and backslash escapes have their `path.Match` meanings. `**` has no recursive meaning. Use a trailing-slash directory declaration for arbitrary depth. There is no shell expansion, Git pathspec magic, brace expansion, negation, or filesystem traversal.
4. Reject absolute paths, empty paths, NUL, and empty, `.` or `..` slash components, allowing only the final empty component that spells a directory. Validate glob syntax with `path.Match`. Require a literal project prefix before matching in a nested installation. An unmatched pattern is lawful and authorizes nothing.

Directory and glob matching are new behavior. Today's `diffBoundary` comparison uses exact membership in a map after projection; it does not expand directories or globs (`internal/validate/conformance.go:798-810`). Do not change that return dialect. A builder must still list concrete touched paths.

Add `func briefBoundaryMatches(declaration, changedPath string) bool` in `internal/validate/brief_bounds.go`. Project declarations with the existing `projectDeclaration`, then use this matcher. Do not move prefix policy into the CLI or shell. A declaration outside the installation refuses as `BRIEF_BOUNDS_INVALID: Boundary: <existing projectDeclaration diagnostic>`. Syntax is admitted by dispatch; installation scope is judged by conformance, which already derives the installation prefix (`internal/validate/conformance.go:242-246`).

Boundary is output permission, not cited authority. Teach `extractBriefAuthorityPaths` to treat a recognized Boundary line as output-only. A path also cited as an input elsewhere must still exist. This layers on its existing input/output accounting, where an input citation wins (`internal/dispatch/brief.go:155-167`). It prevents a new allowed file from being rejected by `ValidateBriefAuthority`'s committed-tree existence check (`internal/dispatch/brief.go:83-104`). Do not exempt the same path from genuine input citations, and do not require directories or globs to exist at admission.

## Which brief binds a round

Use the seat-authored task direction of the supplied implementer job. The dispatcher already composes both initial briefs and follow-up messages as the `task-direction` slot (`internal/dispatch/composition.go:268`, `scripts/agents/dispatch.sh:2771-2777`). It records source byte ranges and digests (`internal/dispatch/composition.go:380-390`) and persists each round's prompt and composition (`scripts/agents/dispatch.sh:1924-1929`, `scripts/agents/dispatch.sh:2883-2888`). Reuse that evidence. Do not add a job field, a return field, a second brief snapshot, or a new artifact schema.

Add `func (r *conformanceRun) reviewBriefBounds() (dispatch.BriefBounds, error)` in `internal/validate/brief_bounds_source.go`:

- Resolve only the supplied job's round. Do not choose the numerically latest round in the chain.
- When `composition.json` exists, require exactly one source with slot `task-direction` and source `caller:brief`. Read only its delivered byte range from `prompt.md`. Validate the range and its DeliveredDigest before parsing that section. The section includes a generated heading and blank lines, which do not look like either new header; its boundaries are defined by the existing renderer (`internal/dispatch/composition.go:368-390`).
- If task-direction is referenced, require one corresponding reference and read it through `dispatch.ReadVerifiedReference`. Do not parse the reference stub. That function checks location, regular-file status, byte count, and digest (`internal/dispatch/references.go:22-49`).
- For a legacy root round without composition, read `artifacts/agents/<rootJob>/brief.md`. Initial dispatch already stores this copy (`scripts/agents/dispatch.sh:1918`). A legacy follow-up without composition uses its `prompt.md`, matching the existing legacy task-direction fallback (`internal/validate/conformance.go:925-934`). If both legacy inputs are absent, treat the historical record as having no headers. Other read failures and malformed present metadata refuse. No present but unreadable source is converted into an unbounded brief.
- Parse through `dispatch.ParseBriefBounds`. Do not interpret headers found in prior-brief, prior-return, critic instructions, or the implementer's return. The existing successor reader returns the entire composed prompt when the task is inline, so it cannot be reused unchanged for this decision (`internal/validate/conformance.go:942-954`). Leave that reader's exhaustion behavior alone.

Each round's headers apply independently. A headerless follow-up has today's behavior. There is no hidden inheritance, union of brief boundaries, addition of ceilings, or automatic reset to a default. This is the literal optional-header contract. The seat must repeat Boundary and Ceiling in every bounded correction brief. Its values describe the entire candidate still awaiting landing, including earlier rounds. A focused follow-up Boundary that omits an earlier still-changed file refuses that candidate. A deliberate scope amendment is a new seat-authored brief for the new round, never an edit of old round evidence.

This choice needs to be explicit because the current return check unions immutable declarations across rounds (`internal/validate/conformance.go:754-817`), while a cap continuation inherits a worktree and is told to declare its predecessor's changes too (`internal/dispatch/capcontinuation.go:22-28`, `internal/dispatch/capcontinuation.go:89`). Update the cap paragraph to distinguish the predecessor's missing return declaration from the brief's allowed paths, and to tell the builder that the current brief's limits cover the whole candidate. It must not promise that adding a path to diffBoundary grants permission.

## Exact diff and line count

Let `T` be the invoking target checkout's `HEAD^{commit}`. Let `B` be `git merge-base T HEAD`, resolved in the implementer worktree. This is already `r.boundaryBase`, not `r.baseSha` (`internal/validate/conformance.go:232-241`). Let `R` be one repository-level `gittree.Workspace.Snapshot("HEAD")` of the returned worktree. Derive the project reviewed tree from that same tree through `r.projectWorkspace().TreeOf(R)`. `TreeOf` peels the installation subtree; Snapshot uses an isolated index and includes working-tree changes (`internal/gittree/gittree.go:199-228`, `internal/gittree/gittree.go:246-255`).

The bounded diff is the repository tree of `B` to `R`. It includes committed changes since B, staged and unstaged edits, deletions, and untracked unignored files. Ignored untracked files stay out; tracked ignored files stay in. Mode changes, symlinks, and gitlinks remain changed paths. These are the snapshot semantics already documented in `internal/gittree/gittree.go:246-254`. Snapshot once, then use those fixed tree IDs for paths, numstat, and the project patch. This also keeps the count and reviewedTree on the same candidate. Today's review takes separate project and repository snapshots (`internal/validate/conformance.go:368-376`); replace those two calls with the one repository snapshot and subtree derivation in the enforcement unit.

Add this tree primitive in `internal/gittree/numstat.go`:

```go
func (w Workspace) ChangedLines(fromTree, toTree string) (int64, error)
func changedLinesFromNumstat(data []byte) (int64, error)
```

Run through the existing bounded, environment-scrubbed `Workspace.git` runner:

```text
git diff --numstat -z --no-renames --no-ext-diff --no-textconv --no-color --ignore-submodules=none <repository-tree-of-B> <R> --
```

Parse NUL records. Split each record at its first two tabs only. Add the decimal additions and deletions with checked 64-bit arithmetic. Skip precisely `-\t-` binary rows. Reject any other malformed row or arithmetic overflow. Never turn an execution error or an uncountable row into zero. Renames count as deletion plus addition, consistent with `Workspace.Diff` and `ChangedPaths` (`internal/gittree/gittree.go:357-376`). Paths with tabs or newlines cannot corrupt the count. Boundary still covers binary paths even though Ceiling excludes their contents.

The addition-plus-deletion precedent is the prose waiver's numstat sum (`internal/validate/conformance.go:617-631`). Its rejection of binaries belongs to that waiver and stays unchanged (`internal/validate/conformance.go:632-637`). The new ChangedLines function serves this new check; do not refactor the waiver in this goal. The tier-1 landing class also stays unchanged: its separate contract requires MaxFiles 3 and MaxChangedLines 40 (`internal/landing/observe.go:1286-1290`). A Ceiling of 400 grants no tier-1 waiver.

## Review enforcement and refusal text

Add two concrete error types in `internal/validate/brief_bounds.go`:

```go
type BriefBoundaryViolation struct {
    Paths []string
    ChangedLines int64
}
func (e *BriefBoundaryViolation) Error() string
type BriefCeilingViolation struct {
    Paths []string
    ChangedLines, Ceiling int64
}
func (e *BriefCeilingViolation) Error() string
func (r *conformanceRun) briefBoundsViolations(
    bounds dispatch.BriefBounds, repositoryPaths []string, changedLines int64,
) ([]error, error)
```

The second return is a malformed declaration or other inability to evaluate, rather than a limit violation. Keep a typed dispatch parser error intact until the CLI rendering boundary. Use the following exact stderr forms, including the existing conformance prefix:

```text
conformance failure: BRIEF_BOUNDARY_EXCEEDED: changed paths outside Boundary: ["extra.txt"]; changedLines=1 additions plus deletions
conformance failure: BRIEF_CEILING_EXCEEDED: changedLines=3 exceeds Ceiling=2 additions plus deletions; changedPaths=["source.txt"]
```

For the boundary violation, Paths contains every changed repository path that matches no declaration. For the ceiling violation, Paths contains every changed repository path in the counted candidate, including binary paths with zero text contribution. Sort and deduplicate each list, then JSON-encode it without truncation. The count is the total candidate count, not just the out-of-bound files' subtotal. Report both violations when both fail, Boundary first. Inability to read the brief uses `conformance failure: BRIEF_BOUNDS_UNREADABLE: <detail>`. Inability to count uses `conformance failure: BRIEF_LINES_UNREADABLE: <detail>`. These return exit 1 and produce no success artifact.

Place this in `reviewStage`, after snapshot and path computation and before `diff.patch` or `review.json` is written. Keep `cumulativeBoundaryViolations(projectPaths)` as the exact-file declaration check. Its current call is at `internal/validate/conformance.go:392`; artifact writes follow at `internal/validate/conformance.go:421-425`.

For a bounded brief, collect the repository-wide path set from the same base and R using `Workspace.ChangedPaths`. Run the new outer checks and the existing project and control-plane checks, then report all applicable policy violations. Move the existing outside-project early policy return into this collected refusal path so a sibling path cannot hide other offending paths or their count. A failure to compute facts still stops immediately. With neither header, keep the existing outside-project refusal ordering and avoid the extra numstat call. The existing project fence currently returns before the declared-boundary check (`internal/validate/conformance.go:377-392`); preserving its policy while collecting its diagnostic is the only precedence change.

Count when either header is present, because a Boundary refusal must also name the count. The extra count is diagnostic when Ceiling is absent; it does not create a size limit. Pass repository paths to the new check and project paths to the old one. A repository path outside the installation is both outside any valid brief declaration and prohibited by the existing project fence. A broad Boundary never overrides trusted plans or control-plane protection (`internal/validate/conformance.go:700-717`).

Do not compare the two declaration sets for containment. A wider diffBoundary with only permitted actual changes passes this new check. A narrower Boundary refuses actual changed paths even when the implementer declared them. A declared-but-unchanged outside path does not by itself refuse. No diffBoundary-versus-Boundary refusal occurs before computing the diff. The existing wrong-prefix refusal for a return declaration still applies independently (`internal/validate/conformance.go:798-803`).

The successful review artifact retains exactly `diffArtifact`, `implementerJob`, and `reviewedTree`; both the writer and a focused test enforce that shape today (`internal/validate/conformance.go:403-407`, `internal/validate/conformance_review_shape_test.go:10-32`). Do not put violations into a successful review.json. Do not overwrite earlier evidence. An identical successful repeat reuses the existing bytes (`internal/validate/conformance.go:413-419`). If a later bounded invocation violates a limit while old review evidence exists, print the typed limit violations plus the existing immutable-review diagnostic and leave both artifacts untouched. Headerless invocations retain their existing immutable-review behavior (`internal/validate/conformance.go:361-365`, `internal/validate/conformance.go:393-400`).

Register the new refusal codes in `internal/refusal/register.go`. The parser and unreadable-input codes use the existing Question classification precedent for `BRIEF_AUTHORITY_REFUSED` (`internal/refusal/register.go:69`). Limit violations use Agent with the remedy `dispatch.sh follow-up with a corrected brief or candidate, then validate conformance --stage review`, Commands 2. The register requires agent refusals to name a remedy or a known defect (`internal/refusal/register_test.go:71-86`). This is a diagnostic remedy, not a new approval flow.

## The seat's pre-read step

From the merge-target installation, after the builder has returned:

```sh
metasystem validate conformance --stage review --job <returned-implementer-job>
```

The seat reads stderr and the exit code before dispatching the critic. Exit 0 admits the candidate to the read. Exit 1 stops that dispatch and returns the named violations to the builder. Exit 2 is a CLI usage error to repair at the seat. These codes already belong to the verb (`cmd/metasystem/validate_verbs.go:276-299`, `cmd/metasystem/validate_verbs.go:343-352`). The command must run from the target checkout, not the implementer worktree (`internal/validate/conformance.go:227-230`).

Yes, the seat can run it alone today. Review resolves an implementer record, return, workspace, and base, then branches directly into reviewStage without resolving a critic (`internal/validate/conformance.go:139-193`, `internal/validate/conformance.go:214-249`). The fixture already calls review before constructing any critic (`scripts/agents/conformance-fixtures.sh:175-179`). No new verb, flag, stage, or critic placeholder is needed.

Update `docs/orchestration.md` at the implementation-to-critique step and `skills/code-critique/SKILL.md` at Layer 1. They currently put implementation before critic dispatch, and the skill tells the reader to run conformance (`docs/orchestration.md:36-42`, `skills/code-critique/SKILL.md:27-31`). Require the seat to obtain exit 0 first. The critic reads the resulting canonical diff and may rerun the same command, which is idempotent for identical evidence. It still checks non-goals, intent, proof quality, and unrelated behavior inside allowed files. Mechanical bounds cannot judge those semantic questions.

This is an enforced review gate with an explicit seat ordering rule. It does not add a second diff generator or claim that every critic launch is newly fenced in dispatch. Missing review artifacts can currently yield no read subject instead of a dispatch error (`internal/dispatch/read_subject_compute.go:24-26`, `internal/dispatch/read_subject_compute.go:134-146`). Changing that separate admission contract is outside this goal. The seat uses this command before spending the read, as DONE requires.

## Reader inventory and seams

| Current reader or writer | Decision and reason |
| --- | --- |
| `BriefMode`, `internal/dispatch/brief.go:41-55` | Changes. Validate optional bounds after the unchanged required mode rule. |
| `ValidateBriefAuthority` and `extractBriefAuthorityPaths`, `internal/dispatch/brief.go:62-104`, `internal/dispatch/brief.go:121-171` | Changes. Validate optional syntax in authority-only admission; Boundary lines are outputs, while a separate input citation still wins. |
| `runDispatchBriefMode`, `cmd/metasystem/dispatch_verbs.go:1993-2027` | Left alone because its existing error and stdout relay suffices. Add focused CLI tests through this verb. |
| `brief_mode`, `brief_authority`, initial and follow-up call sites, `scripts/agents/dispatch.sh:858-864`, `scripts/agents/dispatch.sh:1542`, `scripts/agents/dispatch.sh:1655`, `scripts/agents/dispatch.sh:2643` | Changes only to the misleading initial failure suffix. Keep shell parsing absent. Both admission paths reach the Go parser. |
| `ComposeRolePacket` source records and references, `internal/dispatch/composition.go:268`, `internal/dispatch/composition.go:380-390` | Left alone because they already preserve the current round's task direction. The new conformance reader consumes them. |
| `successorTaskDirection`, `internal/validate/conformance.go:914-960` | Left alone because it owns exhaustion evidence, including its legacy full-prompt fallback. New bounds use the specific source range. |
| `missionStream`, `internal/validate/conformance.go:827` | Left alone because it reads the root brief for stream accounting, not the current round's allowed changes. |
| `workingMode` and `workingModeFrom` in the fake adapter, `internal/adapter/fake.go:137-167` | Left alone because they derive return mode, not admission scope. Fake return production at `internal/adapter/fake.go:111` keeps its empty exact-path declaration. |
| Brief and follow-up templates, `scripts/agents/templates/brief.md:1`, `scripts/agents/templates/brief.md:36`, `scripts/agents/templates/follow-up.md:1-18` | Changes. Explain optional headers near Working Mode and explicitly repeat cumulative limits in bounded follow-ups. Use indented examples, not live unfilled optional placeholders that would break mode-only callers. |
| Template mode consumers, `scripts/agents/fingerprint-harness.sh:187`, `scripts/agents/adapters/fake.sh:449-453`, `internal/adapter/selftestrun.go:135`, `internal/steward/stage.go:102`, `internal/steward/stage.go:155` | Left alone because missing new headers remains lawful. Their mode rewriting or emission stays valid. |
| `scripts/validate-metasystem.sh:1618` and return-schema checks at `scripts/validate-metasystem.sh:1967-2010` | Left alone because mandatory headers and return schemas do not change. New headers are optional. |
| `projectDeclaration` and `cumulativeBoundaryViolations`, `internal/validate/conformance.go:284-293`, `internal/validate/conformance.go:724-819` | Left alone as policy owners. The review-stage caller adds the outer check; existing exact declarations still apply. |
| `reviewStage`, `internal/validate/conformance.go:355-428` | Changes. One frozen repository snapshot supplies the project view, new bounds, and existing diff. Refusals occur before success artifacts. |
| `runValidateConformance`, `cmd/metasystem/validate_verbs.go:280-352` | Changes only to help text. Keep verb, flags, output relay, and exit codes. |
| Recertification boundary consumers, `internal/validate/recertification.go:809`, `internal/validate/recertification.go:1182` | Left alone because this goal limits review candidates, not the size of later target-side recertification. Do not broaden the shared cumulative-declaration helper. |
| `CapContinuationText`, `internal/dispatch/capcontinuation.go:26`, `internal/dispatch/capcontinuation.go:89` | Changes its wording and matching test. The return still lists the whole chain; that list grants no extra brief scope. |
| `followUpChainPaths`, `internal/dispatch/followup_rebase.go:123-149` | Left alone because concrete return paths identify possible rebase overlap. Permission patterns would be the wrong input for that decision. |
| `requireDesignCritiquePairing`, `internal/dispatch/review_reference.go:313-325` | Left alone because it requires one exact returned design artifact. An allowed directory does not replace that identity. |
| `returnChecker.checkDiffBoundary`, `internal/validate/returncomplete.go:239-302` | Left alone because it checks and normalizes return path spelling. It neither owns the brief nor computes its diff. |
| `TestAdjudicateTurnRepairsBareImplementerDiffBoundaryAtAcceptance`, `internal/adapter/adjudicate_test.go:205-260` | Left alone because acceptance-time path repair still precedes conformance. Rerun the adapter package as a seam check; do not extend this test into another diff owner. |
| Implementer role and schema, `scripts/agents/roles/implementer.md:3-13`, `scripts/agents/schemas/implementer.schema.json:7`, `scripts/agents/schemas/implementer.schema.json:38` | Role prose changes to explain allowed scope versus actual declarations. Schema stays unchanged because diffBoundary remains an array of concrete paths. |
| `role-packets.json`, `scripts/agents/role-packets.json:40-43`, `scripts/agents/role-packets.json:54-59` | Left alone because it already selects the critic skill and implementer orchestration guidance. Edit those selected owners, not the recipe. |
| Code-critique skill, `skills/code-critique/SKILL.md:29-31` | Changes. Seat checks mechanical bounds before dispatch; critic retains semantic conformance and defect review. |
| Prose waiver and tier-1 landing class, `internal/validate/conformance.go:617-637`, `internal/landing/observe.go:1286-1290` | Left alone because these are distinct waiver and landing contracts. Passing the new outer limits grants neither. |

## Exact fixture legs

Extend `scripts/agents/conformance-fixtures.sh`, group id `section/conformance-fixtures` in `testing.json:74`. The bed already has `new_case`, `write_implementer`, and an exit-1-plus-message assertion (`scripts/agents/conformance-fixtures.sh:38-64`, `scripts/agents/conformance-fixtures.sh:73-96`, `scripts/agents/conformance-fixtures.sh:153-169`). Use those helpers. Each leg starts in a fresh case and runs the real public conformance verb without creating a critic. The fixture's top-level layout correctly uses unprefixed repository paths.

| Leg name | Exact candidate and headers | Required result |
| --- | --- | --- |
| `brief-boundary-unlisted` | Add untracked `extra.txt` containing `extra\n`. Return diffBoundary `["extra.txt"]`. Frozen root brief: Boundary `["source.txt"]`, Ceiling `10`. | Exit 1 with the exact Boundary violation above, including `extra.txt` and changedLines 1. Neither success artifact exists. A broad self-declaration must not mask the brief violation. |
| `brief-ceiling-exceeded` | Append `one\ntwo\nthree\n` to source.txt. Declare source.txt. Boundary `["source.txt"]`, Ceiling `2`. | Exit 1 with the exact Ceiling violation above, count 3 and source.txt. Neither success artifact exists. |
| `brief-both-admitted` | Append `one\ntwo\n` to source.txt. Declare source.txt. Boundary `["source.txt"]`, Ceiling `2`. | Exit 0, nonempty diff.patch, three-field review.json, and emitted reviewedTree matching the project snapshot. Equality is admitted. |
| `brief-no-headers` | Use the bed's ordinary Working Mode brief with neither new header. Append 401 numbered lines to source.txt and add untracked extra.txt with one line. Declare both concrete paths. | Exit 0 despite 402 changed lines. In a fresh companion case leave extra.txt undeclared and require today's cumulative-boundary refusal. Optional headers must neither impose a default nor remove the old check. |

The four legs use the existing legacy root-brief path. Add Go tests with real composition for both inline and referenced current briefs, so fixture convenience cannot hide a broken production reader. Do not add another fixture bed, group id, runner, or testing policy. The existing section already launches this bed through the engine (`scripts/validate-metasystem.sh:1129-1131`).

## Builder units and ordered proof

Land units 1 through 6 in order. Unit 1 lands first. Every unit is a fresh implementation chain based on the preceding landed unit. Do not accumulate several units in one unlanded candidate and then call its combined diff a 400-line unit. Ceiling measures additions plus deletions, including tests and documentation. Each unit has a hard maximum of 400; the estimates below are planning allocations, not permission to exceed it. The seat owns integration receipts outside the builder's returned patch. If a unit does not fit, return the gap and a proposed split before writing outside its Boundary.

Every builder runs no fixture bed and never exports METASYSTEM_BIN for a Go run. Keep the provided GOCACHE, GOTMPDIR, and STATICCHECK_CACHE. Run focused Go tests and the fast gate only. The seat runs the beds through its enrolled engine on the returned candidate. This follows the existing builder/seat proof split (`internal/dispatch/build.go:1126`, `docs/orchestration.md:172`) and the fast gate's edit-loop role (`scripts/agents/go-gate.sh:97-99`). No builder commits.

For each unit, run the named negative tests once with just the corresponding rule removed, restore that rule, and run them again. Do not leave a removed rule in the return. Then run these commands in order from `metasystem/`, substituting the exact package and script sets listed for the unit:

```sh
go build ./...
go vet <touched packages>
go test -race -count=1 <touched packages>
bash -n <touched scripts>
scripts/agents/go-gate.sh --fast
```

Skip only the bash command when no shell script changed, and report that as not applicable. Include the focused mutation witness, command, and observed failure for each new rule in the builder return. Do not run a fixture script as a substitute for these commands.

### Unit 1: Parse optional headers at admission

```text
Working Mode: implement
Boundary: ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_bounds_test.go", "metasystem/internal/dispatch/brief_authority_test.go", "metasystem/cmd/metasystem/dispatch_brief_bounds_test.go", "metasystem/scripts/agents/dispatch.sh", "metasystem/internal/refusal/register.go"]
Ceiling: 400
Non-goals: no diff computation, review enforcement, return-schema change, new flags, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate about 130 production lines, 230 test lines, and 20 plumbing/register lines. Add BriefBounds, BriefBoundsRefusal, ParseBriefBounds, and ReadBriefBounds with the signatures above. Wire BriefMode and ValidateBriefAuthority. Mark Boundary lines output-only. Change only the shell failure suffix. Register BRIEF_BOUNDS_INVALID.

`TestBriefBoundsPresence` proves absence, each single header, both, empty-array deny-all, and Ceiling zero. `TestBriefBoundsSyntax` has independently failing rows for duplicate headers, wrong JSON shape, bad path components, bad glob syntax, invalid integer forms, and overflow. `TestBriefBoundsPreservesWorkingMode` keeps the existing required mode behavior. `TestBriefAuthorityBoundaryIsOutput` admits a new named file. `TestBriefAuthorityBoundaryInputStillRequired` separately refuses it when cited as an input. `TestDispatchBriefBoundsAdmission` calls the public verb function for normal and authority-only modes and proves that a malformed Ceiling cannot reach an exit-0 admission, while a valid mode is still the only stdout value.

Run in this order:

```sh
go build ./...
go vet ./internal/dispatch ./internal/refusal ./cmd/metasystem
go test -race -count=1 ./internal/dispatch ./internal/refusal ./cmd/metasystem
bash -n scripts/agents/dispatch.sh
scripts/agents/go-gate.sh --fast
```

### Unit 2: Count changed lines on fixed trees

```text
Working Mode: implement
Boundary: ["metasystem/internal/gittree/numstat.go", "metasystem/internal/gittree/numstat_test.go"]
Ceiling: 400
Non-goals: no snapshot policy change, conformance wiring, waiver refactor, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate about 80 production and 220 test lines. Implement Workspace.ChangedLines and changedLinesFromNumstat with the signatures above, using Workspace.git. Do not introduce a second Git runner.

`TestChangedLinesAddsAndDeletes` proves replacement costs two lines. `TestChangedLinesExcludesBinary` proves binary contents add zero beside countable text. `TestChangedLinesRenameCountsBothEndpoints` proves rename detection cannot reduce the budget. `TestChangedLinesOddFilenames` proves tabs and newlines do not split rows. `TestChangedLinesRejectsMalformedAndOverflow` proves unexpected rows and arithmetic overflow fail instead of disappearing. `TestChangedLinesUsesSuppliedTrees` changes the working tree after capture and proves the count remains tied to the supplied trees. `TestChangedLinesGitFailure` proves an invalid tree is an error. Include a nested installation case and a diff-driver configuration case using the existing runner's isolation.

Run in this order. No bash input.

```sh
go build ./...
go vet ./internal/gittree
go test -race -count=1 ./internal/gittree
scripts/agents/go-gate.sh --fast
```

### Unit 3: Read only the current round's authored bounds

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/brief_bounds_source.go", "metasystem/internal/validate/brief_bounds_source_test.go", "metasystem/internal/refusal/register.go"]
Ceiling: 400
Non-goals: no limit enforcement, new artifact schema, new snapshot, changed exhaustion reader, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate about 110 production and 250 test lines. Implement reviewBriefBounds. Reuse dispatch.CompositionRecord, source metadata, ReadVerifiedReference, and ParseBriefBounds. Register BRIEF_BOUNDS_UNREADABLE. This is a small preparatory landing; it does not claim enforcement until unit 4.

`TestReviewBriefBoundsInlineSourceOnly` places conflicting headers in prior-brief and role text and proves only current task-direction is read. `TestReviewBriefBoundsReferencedSource` composes an oversized brief and proves headers are read from the verified body. `TestReviewBriefBoundsRejectsCorruptSource` independently checks invalid byte ranges, duplicate/missing task source, bad delivered digest, duplicate reference, and changed referenced bytes. `TestReviewBriefBoundsLegacy` covers root brief, old follow-up prompt, and absent legacy input. `TestReviewBriefBoundsReadFailure` distinguishes absent legacy input from an unreadable present input. `TestReviewBriefBoundsRoundIsolation` proves that a headerless current round stays unbounded, a single current header activates only its own check, and a later round cannot replace the supplied job's brief.

Run in this order. No bash input.

```sh
go build ./...
go vet ./internal/validate ./internal/refusal
go test -race -count=1 ./internal/validate ./internal/refusal
scripts/agents/go-gate.sh --fast
```

### Unit 4: Enforce the outer boundary at review

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/brief_bounds.go", "metasystem/internal/validate/conformance.go", "metasystem/internal/validate/brief_bounds_test.go", "metasystem/internal/validate/conformance_brief_bounds_test.go", "metasystem/internal/refusal/register.go"]
Ceiling: 400
Non-goals: no merge-stage or recertification policy change, new CLI mode, new success-artifact fields, fixture execution, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate about 145 production and 230 test lines. Implement the two violation types, briefBoundaryMatches, and briefBoundsViolations. Wire reviewBriefBounds and ChangedLines into reviewStage. Derive project reviewedTree from the single repository snapshot. Keep the existing cumulative return check. Collect bounded-policy failures before the success-artifact write. Register the two limit codes and BRIEF_LINES_UNREADABLE.

`TestBriefBoundaryMatching` separately proves exact paths, directory component boundaries, literal directory-prefix handling, single-component globs, empty-array deny-all, unmatched patterns, and case sensitivity. `TestBriefBoundaryProjection` proves top-level paths, required nested prefix, the whole-project directory, and exactly one prefix strip. `TestReviewBriefBoundaryOuter` changes two undeclared-by-brief paths while declaring both in diffBoundary; it requires both sorted names and their total count. `TestReviewBriefCeiling` checks below, equal, above, and zero. `TestReviewBriefChecksAreIndependent` proves Boundary-only has no size cap, Ceiling-only has no new path restriction, both failing prints both types, and neither keeps the old check. `TestReviewBriefDeclarationIsNotPermission` admits a declared-but-unchanged outside path, refuses actual outside work, and keeps an undeclared actual path refused even inside Boundary. Use table rows and the existing newConformanceFixture to stay within the ceiling.

Run in this order. No bash input.

```sh
go build ./...
go vet ./internal/validate ./internal/refusal
go test -race -count=1 ./internal/validate ./internal/refusal
scripts/agents/go-gate.sh --fast
```

### Unit 5: Prove lifecycle and snapshot seams

```text
Working Mode: implement
Boundary: ["metasystem/internal/validate/conformance_brief_bounds_seams_test.go", "metasystem/cmd/metasystem/conformance_brief_bounds_test.go"]
Ceiling: 400
Non-goals: no production rules, fixture-bed execution, new runner, schema changes, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate at most 360 test lines. This unit independently strengthens the runtime proof of unit 4 without expanding its production patch. Use the existing Go conformance fixture (`internal/validate/conformance_test.go:48-85`) and the command package's public verb entrypoint.

`TestReviewBriefUsesMergeBaseCandidate` mixes a committed change, an unstaged replacement, a deletion, and an untracked file, then moves target HEAD without changing the merge-base. Assert the exact additions-plus-deletions count. `TestReviewBriefNestedWholeRepository` proves a sibling path is named in the typed refusal and cannot disappear through project projection. `TestReviewBriefBinaryPathStillBounded` proves binary exclusion does not exempt its path. `TestReviewBriefProtectedPathsStillRefuse` permits plans and the control-plane spelling in Boundary and proves existing protection wins. `TestReviewBriefFollowUpCountsEarlierChanges` supplies a later brief with a smaller Ceiling and proves prior candidate changes still count. `TestReviewBriefLeavesEvidenceImmutable` proves no artifacts on first refusal, exact three-field output on success, identical reuse, and typed refusal without overwrite after a changed candidate. `TestConformanceBriefCLIWithoutCritic` proves exit 0/1 and exact stderr through the public review verb with no critic record; malformed CLI use stays exit 2.

Run in this order. The second test command is the named unchanged-reader seam check. No bash input.

```sh
go build ./...
go vet ./internal/validate ./cmd/metasystem
go test -race -count=1 ./internal/validate ./cmd/metasystem
go test -race -count=1 ./internal/adapter ./internal/dispatch ./internal/gittree
scripts/agents/go-gate.sh --fast
```

### Unit 6: Add the four bed legs and document seat ordering

```text
Working Mode: implement
Boundary: ["metasystem/scripts/agents/conformance-fixtures.sh", "metasystem/scripts/agents/templates/brief.md", "metasystem/scripts/agents/templates/follow-up.md", "metasystem/scripts/agents/roles/implementer.md", "metasystem/skills/code-critique/SKILL.md", "metasystem/docs/orchestration.md", "metasystem/internal/dispatch/capcontinuation.go", "metasystem/internal/dispatch/capcontinuation_test.go", "metasystem/cmd/metasystem/validate_verbs.go"]
Ceiling: 400
Non-goals: no new fixture bed or testing.json group, no role-packet recipe or return-schema change, no automatic critic-launch gate, fixture execution by the builder, or commits.
for every rule you add, a test that fails when that rule alone is removed; run it before you return
```

Allocate about 110 fixture-plumbing lines, 150 documentation/help changed lines, and 60 cap-message/test changed lines. Add the four exact legs above using existing shell helpers; all new decision logic stays in Go. Update only the named instruction owners, template examples, conformance help text, and cap paragraph. Extend `TestCapContinuationTextTellsTheFactAndWhatTheWorktreeHolds` (`internal/dispatch/capcontinuation_test.go:55-76`) so removing the clarification fails its paragraph assertion. The runtime rule witnesses remain the Go tests from units 1 through 5; do not manufacture prose-mirroring Go tests for every documentation sentence.

Run in this order:

```sh
go build ./...
go vet ./internal/dispatch ./cmd/metasystem
go test -race -count=1 ./internal/dispatch ./cmd/metasystem
bash -n scripts/agents/conformance-fixtures.sh
scripts/agents/go-gate.sh --fast
```

The builder syntax-checks this bed but does not execute it. The seat runs `section/conformance-fixtures` through the engine and reads all four leg outcomes before accepting the goal's runtime proof. It also runs the risk-selected dispatch bed through `section/dispatcher-adapter-and-mission-runner-fixtures`, whose group is already declared in `testing.json:95`, to cover the shell preflight relay. Use the engine's public testing selection and retained results, not a direct fixture invocation or an exported replacement binary.

## Completion obligations and remaining choice

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| BDRB-1 | HIGH | Header contract | Optional headers preserve legacy briefs and refuse malformed declarations | dispatch | brief.go | Unit 1 parser, authority, and CLI tests | Existing dispatch bed | PARTIAL | Build unit 1; run seat proof |
| BDRB-2 | HIGH | Which brief binds a round | The current authored brief supplies the bounds | validate | reviewBriefBounds | Unit 3 source and round-isolation tests | Real composed inputs in Go tests | PARTIAL | Build unit 3; prove both source forms |
| BDRB-3 | HIGH | Review enforcement | Refuse every actual outside path before a read | validate | reviewStage, briefBoundsViolations | Units 4 and 5 path/projection/protection tests | brief-boundary-unlisted | PARTIAL | Build units 4 and 5; run bed leg |
| BDRB-4 | HIGH | Exact diff and line count | Count the exact merge-base candidate with binary exclusion | gittree and validate | ChangedLines and reviewStage | Unit 2 plus unit 5 snapshot tests | brief-ceiling-exceeded | PARTIAL | Build units 2, 4, and 5; run bed leg |
| BDRB-5 | HIGH | Reader inventory and seams | Keep old checks and proof ownership | validate | reviewStage | Independence, declaration, and immutability tests | brief-both-admitted and brief-no-headers | PARTIAL | Build units 4 and 5; run bed legs |
| BDRB-6 | HIGH | Seat's pre-read step | Obtain exit 0 before critic dispatch | orchestration and conformance CLI | Existing review verb, updated seat rule | CLI without critic test | All four legs have no critic | PARTIAL | Build units 5 and 6; run seat proof |

All high obligations have owners, code targets, and tests. None is marked implemented or proved by this design. Bad headers refuse before launch. Git and source-read failures refuse before success artifacts. The existing bounded Git runner supplies timeout handling (`internal/validate/conformance.go:70-78`, `internal/gittree/gittree.go:121-141`). Retry uses the same immutable inputs; correction requires a new round after a successful review has been recorded. No dependency, public return schema, or externally published API changes.

Wido's only policy choice is a future default for briefs that omit Ceiling. Recommend 400 changed lines if he later chooses mandatory default enforcement. It matches his unit-size rule and makes an omitted line no broader than an explicit unit brief. That would change the current DONE compatibility promise, so it is not included here. With no further decision, absent Ceiling stays unlimited and all six builder briefs explicitly say Ceiling: 400. No answer is needed to build this design.

Design verification performed: read the goal's DONE sentence, the required source paths, the CLI and snapshot owners, the current tests, the fixture bed, and testing.json. Implementation tests, Go gates, and fixture beds were not run because this deliverable changes no runtime code. The seat must obtain those proofs on the built candidate. The current task writes only this report and makes no commit or tracked-file edit.

Proposed receipt for the integrating seat: `DESIGN brief-declares-the-round-boundary: specified optional brief bounds, pre-read conformance refusals, exact merge-base numstat counting, reader seams, four existing-bed legs, and six separately landing units capped at 400 changed lines; evidence=read; runtime-proof=pending`.
