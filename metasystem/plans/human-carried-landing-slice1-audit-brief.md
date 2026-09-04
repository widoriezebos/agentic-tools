Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal human-carried-landing)
Date: 2026-09-04

# Goal

Slice 1 of goal human-carried-landing, the refusal audit: one new
package `internal/refusal` holding the register of every refusal code
in the audited set, each row classified and naming the human verb that
carries past it, with the three slice-1 tests of design point
HCL-AUDIT-03 green. The design is
metasystem/plans/human-carried-landing-design.md at revision 2.1 (main
4ad38918); points 01 and 03 govern this slice and nothing else in the
design is built here.

# Workspace

The dispatch workspace as given, on the branch given. May be touched:
`metasystem/internal/refusal/register.go` and
`metasystem/internal/refusal/register_test.go` (new). Nothing else:
no other Go file, no script, no plan, no record, no docs. A defect the
audit finds is RECORDED in the table (the `Defects` list), never fixed.
Do not stage or commit; the seat lands the chain.

# Inputs

Design point HCL-AUDIT-03 (design lines 88-149) is the contract; read
it whole before writing. This brief fixes every choice it leaves, so
nothing below is a judgment call.

## The table

```go
package refusal

type Shape string

const (
    Identity Shape = "identity" // 01 (a): the machine establishes the word is the human's
    Warning  Shape = "warning"  // 01 (b): printed and recorded, then the machine complies
    Question Shape = "question" // 01 (c): the word is stale or names nothing; the machine asks
    Agent    Shape = "agent"    // binds an agent's act only; Override is the human verb past it
)

type Row struct {
    Code     string // the token exactly as the walk collects it
    Owner    string // package or script that emits it, e.g. "internal/dispatch"
    Site     string // file:line of one emitting site, re-read at main 4ad38918
    Shape    Shape
    Override string // the human verb that carries past it; "" only for Identity, Question
    Commands int    // commands between the human's intent and the effect (design 08); 0 when Override is ""
    Pending  string // "human-carried-landing" while the Override is not yet in the tree
}

type Exclusion struct {
    Pattern string // an exact token, or "PREFIX_*" for a prefix
    Reason  string
}

type Shell struct{ Script, Line, Prose, Override string }

type Defect struct{ Code, Why string }

var Rows []Row
var Exclusions []Exclusion
var ShellRows []Shell
var Defects []Defect
var Slow []string // human verbs whose Commands is 3 or more, "verb: n commands"
```

Exported exactly as above; the names are the contract the slice-2
tests and the docs cite. Package doc comment: one paragraph naming the
design and point 03.

## The walk (HCL-03-EVERY-CODE-ROWED)

In `register_test.go`, a helper walks the source files of these eight
package directories, relative to the module root (find it by walking
up from the test's working directory to the directory holding
`go.mod`): `internal/dispatch`, `internal/goal`, `internal/goalbudget`,
`internal/landing`, `internal/steward`, `internal/channel`,
`internal/humanauthority`, `cmd/metasystem`. Files ending `_test.go`
are skipped. Each file is parsed with `go/parser.ParseFile` (mode 0)
and every `*ast.BasicLit` of kind `token.STRING` is visited; its value
is `strconv.Unquote`d (raw strings included). From each value:

1. every match of the regular expression
   `[A-Z][A-Z0-9]*(_[A-Z0-9]+)+` whose preceding byte and following byte
   are absent or not in `[A-Za-z0-9_]` (implement as
   `regexp.MustCompile("(?:^|[^A-Za-z0-9_])([A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+)(?:[^A-Za-z0-9_]|$)")`
   applied repeatedly, or a hand loop; either is fine, the collected
   set must equal the one the seat lists below);
2. the whole value when it matches
   `^[a-z0-9]+(-[a-z0-9]+)*-(refused|unreadable|malformed|unavailable)$`;
3. in `internal/landing` only: the first argument of every call whose
   function identifier is `wouldRefuse` when it is a string literal;
   the value of the `code` key of every composite literal whose type
   identifier is `carriageError`; and every string literal in a `case`
   clause of the function `knownRefusalCode` (promotion.go:90-122).

The collected set is a `map[string]struct{}`. The test fails, naming
every offender, when a collected token has neither a `Rows` entry with
that exact `Code` nor a matching `Exclusions` entry (exact, or prefix
when the pattern ends `*`). It also fails on a dead exclusion: an
`Exclusions` entry that matches no collected token.

The seat ran the walk (a Python approximation over the same files at
4ad38918) and collected 154 tokens: 118 UPPER_SNAKE and 36 hyphen
codes. Your go/ast walk should collect the same set within a few
tokens (raw strings and `case` literals may add some); every token it
collects gets a row or an exclusion, and the list below is the seat's
classification of the ones it saw. A token the walk finds that is not
listed below is classified by the same rules (a refusal an agent hits
whose condition a human verb resolves is `agent` with that verb; a
steward observation, environment name, wire key or usage placeholder
is an exclusion) and recorded under `decisions` with its site.

## Exclusions (with the one-line reason each)

- `GIT_*`: git environment variable names.
- `METASYSTEM_*`: this tree's environment variable names.
- `BASH_SOURCE`, `LC_ALL`, `STEWARD_MESSAGE`: environment names.
- `HUMAN_AUTHORITY_PROVEN`, `TEMPORARY_HUMAN_WORD`,
  `AUTHENTICATED_CHANNEL_WORD`: authority outcomes that admit, not refuse.
- `AUTO_HEAL_ELIGIBLE`, `AUTO_HEAL_ENDED`, `HEALING_FLAPPING`,
  `NO_LAWFUL_REMEDY`, `BREACH_STOP_COMPLETE`, `BREACH_STOP_INDETERMINATE`,
  `BREACH_STOP_OPEN`, `ASSUMPTION_DRIFT`, `BUDGET_MISSING`,
  `DURABILITY_PENDING`, `INTERRUPTED_BY_NEXT_TURN`, `ENROLLMENT_CHANGED`,
  `ENROLLMENT_DRIFT`, `NOT_ENROLLED`, `NO_ADAPTER`, `FETCH_FAILED`,
  `STATE_WRITE_FAILED`, `RENDER_FAILED`, `TICK_FAILED`, `WRITE_FAILED`,
  `PASS_COMPLETE`, `PASS_FAILED`, `TRANSPORT_FAILED`,
  `TRANSPORT_SUBMITTED`, `ledger-unavailable`: steward health and
  component observations; they report, they refuse nothing.
- `LOCAL_PENDING`, `LOCAL_TERMINAL`, `FOREIGN_REPORT_ONLY`,
  `ALREADY_TERMINAL`, `TERMINAL_DURING_STOP`: stop-verdict states of
  internal/dispatch/stop.go, outcomes of a stop already ordered.
- `MIGRATION_EPOCH`, `REVIEWED_SOURCE_SHA256`: manifest keys.
- `NON_NEGATIVE_INTEGER`, `POSITIVE_INTEGER`: usage-line placeholders.
- `usage-unavailable`: an adapter verb name, not a refusal.
- `creator-identity-unreadable`, `custody-entry-malformed`,
  `group-membership-unreadable`, `index-unreadable`,
  `indexed-record-unreadable`, `prefork-group-membership-unreadable`,
  `prefork-marker-unreadable`, `prefork-process-table-unreadable`,
  `prefork-supervisor-unreadable`, `process-enumeration-unavailable`,
  `process-table-unreadable`, `recorded-identity-unreadable`,
  `registry-unreadable`, `tag-position-proof-unavailable`: census and
  custody evidence codes, the reason a liveness observation is
  unknown; no human word passes through them. Verify each is not the
  code of a refusal returned to a caller (search its site); one that is
  becomes an `agent` row with Override `goal recover` instead, recorded
  under `decisions`.

## Rows (Code, Owner, Shape, Override, Commands, Pending)

internal/humanauthority, all Shape identity, Override "" (the human
path IS the identity check; the machine asks for the enrolled terminal
or the channel word): `AGENT_IN_AUTHORITY_CHAIN`, `ANCESTRY_CHANGED`,
`ANCESTRY_CYCLE`, `ANCESTRY_UNREADABLE`, `ARGV_UNREADABLE`,
`PROCESS_REUSED`, `TERMINAL_NOT_REACHED` (authority.go:28-36).

internal/goal:
- `APPROVAL_REQUIRED` agent, `goal approve`, 1.
- `APPROVAL_EXPIRED` agent, `goal approve` (a fresh word), 1.
- `RELAY_AFTER_ENROLLMENT` identity, "" (approval.go:329: relayed
  words end at the first enrolled terminal; the enrolled terminal or
  the channel is the path).
- `GOAL_NORM_REFUSED` agent, `goal set-budget` then `--approved-ref`
  naming its operation, 2.
- `GOAL_SPLIT_REFUSED` Defect: split.go:240 refuses a split after the
  first slice and nothing clears `Sliced` (file.go:843, sliced.go:34
  only set it); a human who wants the split has no verb past it. Row
  with Shape agent, Override "" and the Defects entry "no human verb
  clears the sliced mark; the only path is park plus new goals".
- `SPLIT_RATIFY_REFUSED` question, "" (split.go:334: the ratification
  bytes the human supplied do not validate; the machine says which).
- `SWEEP_DUPLICATE_GOAL`, `SWEEP_INCOMPLETE`, `SWEEP_LISTING_CHANGED`,
  `SWEEP_MALFORMED_ROW`, `SWEEP_UNKNOWN_GOAL` question, "" (the
  human-confirmed sweep's draft is malformed or stale; "preview again").
- `ELAPSED_LIMIT`, `CORRUPT_OVER_LIMIT` (file.go:264-265, stop
  reasons that close admission) agent, `goal resume`, 1.

internal/dispatch:
- `BUDGET_UNKNOWN` agent, `goal set-budget` (binds a revision the
  reader can read), 1.
- `BUDGET_REFUSED` agent, `goal set-budget`, 1.
- `HAZARD_REFUSED` agent, `goal edit --tier 2 --why` (admission.go:211
  names it); on an approved goal that edit is refused until `goal
  unapprove`, then `goal approve` again: Commands 3. Slow entry.
- `RISK_UNANSWERED` agent, `goal classify-sweep` for a legacy goal
  (one command); `goal edit --risk` on an approved goal needs
  unapprove, edit, approve: Commands 3, Slow entry, Override names both.
- `CLOCK_REGRESSED` agent, `goal set-budget` (a new revision at a new
  observation), 1.
- `ADMISSION_CLOSED_ELAPSED`, `ELAPSED_BREACH` agent, `goal resume`
  (after `goal set-budget` when the box itself is spent), 1.
- `BRIEF_AUTHORITY_REFUSED` question, "" (brief.go:34: the brief names
  paths the repository does not have).
- `OBLIGATION_REFUSED` agent, `goal set-obligation`, 1.
- `SLICE_CAP_REFUSED` agent, a human goal-history operation (`goal
  approve` or `goal set-budget`) passed as `--approved-ref`, 2.
- `SLICE_START_UNRECORDED` agent, `goal recover` (claim.go:366: the
  slicing fact did not land; recovery closes or completes it), 1.
- `slice-approval-refused` agent, `goal approve`, 1.

internal/landing, every code collected by rule 3 (the 28 `case`
literals of `knownRefusalCode` plus `chain-full-gate-refused` and any
other `wouldRefuse`/`carriageError` literal outside the switch), Owner
`internal/landing`, Shape agent, Override `land.sh --carried`,
Commands 1, Pending `human-carried-landing`, EXCEPT
`malformed-candidate-tree` and `candidate-tree-unreadable`, which are
Shape question, Override "", no Pending (design 05 keeps the
unprovable index as a question back to the human). `chain-full-gate-refused`
(observe.go:149) is additionally a Defects entry: "emitted by
observe.go:149 and absent from knownRefusalCode (promotion.go:90-122);
the promotion reader does not know it".

Shell rows, by hand, Override `land.sh --carried`, from the design's
own cites: land.sh:94-112 (the usage and argument refusals) and the
commit.sh gates the design names: :226-245 coverage, :270-271 the fast
gate, :383-387 the audit, :402-445 the landing observe verdict,
:519-533 the colon-trailer refusal. Read each site and put its first
prose line in `Prose`; a site that has moved is found by its prose and
the new line recorded, with the move under `decisions`. The design
says plainly no test proves the shell list complete; the package doc
comment repeats that sentence.

Slow: every row whose Commands is 3 or more, rendered `"<verb>: <n>
commands"`.

## HCL-03-EVERY-ROW-REAL

Parses `cmd/metasystem/main.go` with go/ast and collects the goal
family's verb names: inside the composite literal whose `name` field
is the string `"goal"` (main.go:409), each element of its verbs slice
is a composite literal whose first element is the verb string
(main.go:426-431 shows the shape). For every row whose Override begins
`goal `, the second word must be in that set; a row marked Pending
`human-carried-landing` is exempt from the check whatever its
Override; any other Override form (`land.sh --carried`, an
`--approved-ref` phrase) is checked for a non-empty string only. The
test fails naming every offender.

## HCL-03-PENDING-ROWS-NAMED

Every row with a non-empty Pending has Pending exactly
`human-carried-landing` and an Override beginning `land.sh --carried`;
every row with Shape agent and an empty Override has a Defects entry
with its Code; every Defects entry names a Code that has a row. Fails
naming every offender.

HCL-03-NO-PENDING-AFTER-SLICE-2 is NOT written in this slice.

# Constraints

Wall-clock budget: 25 minutes. Two files only. `go vet ./internal/refusal/`
and `go test ./internal/refusal/` must pass; run
`scripts/agents/go-gate.sh --fast` once at the end (it runs staticcheck
and the build). Do not run the full test suite. No new module
dependencies; go/ast, go/parser, go/token, regexp, strconv, os,
path/filepath, strings, testing only.

# Expected Return

The implementer return schema: `outputs` naming the two files;
`evidence` with the three commands above and their observed output
(`go test -run 'TestHCL03' -v ./internal/refusal/` listing the three
test names green); `decisions` listing every classification this brief
did not fix, every exclusion verified as evidence-only, every moved
shell site, and the count of tokens the walk collected versus the
seat's 154; `gaps` for anything below.

# Acceptance Criteria

1. `internal/refusal/register.go` exports exactly the types and
   variables of the table section.
2. Three tests named `TestHCL03EveryCodeRowed`,
   `TestHCL03EveryRowReal`, `TestHCL03PendingRowsNamed` exist and pass.
3. Every token the walk collects has a row or an exclusion; no dead
   exclusion.
4. `Defects` holds at least `GOAL_SPLIT_REFUSED` and
   `chain-full-gate-refused`; `Slow` holds at least the `HAZARD_REFUSED`
   and `RISK_UNANSWERED` entries.
5. No file outside the two named changed.

# Gap Rule

stop and report a gap; never fill it silently. A gap is a choice this
brief and design point 03 leave open that changes the table's contract
(a new Shape value, a new exported name, a token whose classification
the rules above cannot decide). A mechanical choice (the exact regular
expression form, how the module root is found, the order of rows) is
made, recorded under `decisions`, and built.
