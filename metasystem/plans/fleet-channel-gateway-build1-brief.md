Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-04

# Goal

Build step 1 of goal fleet-channel-gateway (tier 3; approved by Wido
2026-09-04; design metasystem/plans/fleet-channel-gateway-design.md at
revision 4, main 757a31f8, three critique rounds closed). Step 1 is
FCG-BUILD-13 (1): the three fences, ValidateChannelTree with the three
record schemas and the refusal table, the transition predicate and the
matrix as library data, the inbox Mutate's three branches as a library
function, and the opid helper — NO writer, NO caller of the Mutate or
the predicate outside tests, NO change to any live behaviour, NO
recovery change. An engine with this step refuses a malformed
plans/channel/ tree and ignores an absent one; nothing else moves.

# Workspace

The dispatch workspace as given, on the branch given. Two files are
new; four existing files change in the places named under Inputs and
nowhere else; no plan, record, docs or other Go file changes. Do not
stage or commit; the seat lands the chain.

- May write: metasystem/internal/goal/channel.go
- May write: metasystem/internal/goal/channel_test.go
- May touch: metasystem/internal/goal/validate.go (the one call in ValidateCommit)
- May touch: metasystem/internal/goal/validate_test.go (only if an existing test asserts ValidateCommit's exact problem list)
- May touch: metasystem/scripts/agents/path-classes.txt (one row)
- May touch: metasystem/scripts/agents/pre-commit-guard.sh (one regexp)
- May touch: metasystem/scripts/agents/pre-commit-guard-fixtures.sh (one case)

# Inputs

Read design point FCG-INBOX-02 whole (the three field tables, the
`ref` and `answer` types, the refusal table, the tuple predicate and
the transition matrix), FCG-SECRET-15's last paragraph (the
validator's `channel-secret` row), and FCG-BUILD-13. Points 03-11 are
later steps; nothing from them is built here. The design's cites were
re-read at 757a31f8.

## The fences

1. scripts/agents/path-classes.txt: add `install:plans/channel/ ledger`
   beside the four ledger rows (lines 34-37), so a landing that touches
   the directory refuses `ledger-path-not-goal-verb`
   (internal/landing/observe.go:586) exactly as plans/goals/ does.
2. scripts/agents/pre-commit-guard.sh:80: the regexp
   `(^|/)plans/goals/` becomes `(^|/)plans/(goals|channel)/`; the message
   stays. Add one case to pre-commit-guard-fixtures.sh that stages a
   file under plans/channel/ and expects the refusal, shaped like the
   existing plans/goals/ case there.
3. ValidateCommit (internal/goal/validate.go:475-491): after the goal
   problems are computed and found empty, append
   `ValidateChannelTree(root, commit)`'s problems to the same list and
   refuse with the same message shape. A commit with no
   plans/channel/ entries yields no problems.

## The package: internal/goal/channel.go

Package goal (internal/channel imports goal, so goal imports nothing
from internal/channel; the last-field digit check is written here).

```go
const ChannelPrefix = "plans/channel/"

type ChannelRef struct {
    Provider string `json:"provider"`
    ID       string `json:"id"`
    ThreadID string `json:"threadId"`
}
type ChannelOption struct {
    Label       string `json:"label"`
    Consequence string `json:"consequence"`
}
type ChannelPosting struct {
    Kind string `json:"kind"`
    By   string `json:"by"`
    At   string `json:"at"`
}
type ChannelRejection struct {
    Ref     ChannelRef  `json:"ref"`
    Reason  string      `json:"reason"`
    At      string      `json:"at"`
    PostRef *ChannelRef `json:"postRef"`
    By      string      `json:"by"`
}
type ChannelAnswer struct { /* the answer table, keys in table order, json tags as the keys */ }
type ChannelQuestion struct { /* the question table, keys in table order; Budget *Budget `json:"budget,omitempty"`; closedAt/closedBy/closedBecause `omitempty` */ }
type ChannelInbound struct { /* the inbound table */ }
type ChannelListener struct { /* the listener table */ }
```

Times are strings in the structs (RFC 3339 UTC, second precision,
`2006-01-02T15:04:05Z`); the validator parses them and refuses
`channel-json` on a parse failure or a non-UTC offset. `step` is
`*int64`, `answer` and `posting` and `thread` are pointers, `facts`,
`options`, `orphanPosts`, `rejected` are slices that must be present
(`[]` when empty — refuse `channel-json` on `null`). Unknown keys:
decode with `json.Decoder.DisallowUnknownFields`. Canonical form:
`MarshalChannel(v any) ([]byte, error)` = `json.MarshalIndent(v, "",
"  ")` plus a trailing newline; the validator does NOT compare bytes
to the canonical form (a record is judged by its fields), but every
writer of later steps uses MarshalChannel, and a unit test round-trips
each struct through it.

```go
// ValidateChannelTree reads plans/channel/ at commit and applies the
// refusal table of FCG-INBOX-02. Absent directory: nil. Each Problem
// reads "<code>: <path>: <detail>", codes exactly as the table names them.
func ValidateChannelTree(root, commit string) []Problem
```

Read the tree with `gitIn(root, "ls-tree", "-r", "--name-only",
commit, "--", ChannelPrefix)` and the blobs through
readCommitGoalBlobs (validate.go:421), as ReadCommitGoals does. The
question's `goal` must exist on the same tip: check
`plans/goals/<goal>.md` in the goal listing of the same commit (call
ReadCommitGoals once and pass its key set in). Every row of the
refusal table is one test case; the `channel-secret` row's last-field
check is: split on whitespace, take the last field, drop a trailing
run of `.,;:!?`, refuse when what remains is exactly six ASCII
digits; the machine-written fields (facts, recommendation,
rejected[].reason, receipt) refuse any whitespace-delimited field of
exactly six ASCII digits after the same trim. The `channel-answer-state`
row is applied as written in revision 4 (closed/null is legal when
closedBecause is not `answered`; the verified-record clause applies
only when `question` is a question id, that is, neither `unbound` nor
`unmatched`); when the question's lineage is `migrated` the
verified-record clause is skipped (FCG-MIGRATE-10) but every other
clause applies.

```go
// ChannelTuple is the (state, phase, posting, thread null, receiptRef null)
// tuple of FCG-INBOX-02. Phase is "" for a null answer; Posting is nil for null.
type ChannelTuple struct {
    State          string
    Phase          string
    Posting        *ChannelPosting
    ThreadNull     bool
    ReceiptRefNull bool
}
func (q *ChannelQuestion) Tuple() ChannelTuple

// ChannelTransition is one matrix row. From and To are the row's
// predicates over a tuple; "me" is the calling machine for the rows
// that name it.
type ChannelTransition struct {
    Name string
    From func(t ChannelTuple, me string) bool
    To   func(t ChannelTuple, me string) bool
}

// ChannelMatrix holds every row of FCG-INBOX-02's matrix by name:
// "ask", "migrate", "post-ref question", "answer budget", "answer",
// "approve-intent", "approved", "receipt-intent", "receipted",
// "rejection intent", "list intent", "silence intent", "rejection ref",
// "list ref", "silence ref", "take-over", "orphan-post", "close".
// Rows with an "any" position accept every value there; "ask" and
// "migrate" have From = the file is absent (the caller passes
// present=false and From is not consulted); "rejection intent" admits
// state closed only when reason == "late" — pass the reason through
// the row's own field: RejectionReason string on ChannelTransition,
// "" for every other row.
var ChannelMatrix map[string]ChannelTransition

// ClassifyChannelTransition applies the predicate on a fetched tip:
// From matches current -> (true, nil): apply;
// TrailerPresent(e, tip, opid) -> (false, nil): AlreadyApplied, the caller
//   returns nil changes and Publish classifies its own opid as applied;
// To matches current and writer != "" and writer != opid ->
//   (false, LostToCompetitor{Winner: writer});
// otherwise (false, fmt.Errorf("channel-transition: %s is %s, expected %s",
//   qid, current, row.Name)) with the tuple printed as
//   "(state, phase|null, posting kind by|null, thread null|set, receiptRef null|set)".
// writer is what the TO field names at the tip: answer.opid for the
// answer rows, record.opid for ask/migrate, posting.by (a machine) for
// the intent and take-over rows, "" when the field carries nothing.
func ClassifyChannelTransition(e Endpoint, tip, qid, opid, me, writer string, present bool, current ChannelTuple, row ChannelTransition) (bool, error)

// ChannelInboxMutate is the inbox Mutate's three branches for one
// record path (FCG-INBOX-02): absent at tip -> one Change writing
// content; present and TrailerPresent(e, tip, <the record's opid>) ->
// LostToCompetitor{Winner: that opid}; present without its trailer ->
// errors.New("inbox record present without its transaction").
// Read the blob with gitIn(root, "show", tip+":"+path); a missing path
// is the absent branch, any other git error is returned as is.
func ChannelInboxMutate(e Endpoint, tip, path string, content []byte) ([]Change, error)

// ChannelOpid mints a fresh operation ULID (NewOperationULID,
// identity.go:11) and returns it with Opid(ulid, machine, lineage).
func ChannelOpid(machine, lineage string) (ulid, opid string, err error)
```

## Tests: internal/goal/channel_test.go

Build the bed with the helpers txn_test.go already has (mustGit,
cloneBed, oneClone at txn_test.go:14-59; do not write a second bed
helper), commit a minimal valid goal and a plans/channel/ tree, and
assert ValidateChannelTree and ValidateCommit: (a) a valid tree with
one open question, one verified inbox record bound to it as its
answer, one listener — no problems; (b) every refusal row, one case
each, including the three secret cases (last-field code refused;
"order 123456 now" accepted; six digits in facts refused) and the
closed/null cases (closedBecause "closed before the ledger inbox"
accepted; closedBecause "answered" with answer null refused); (c) the
absent directory yields nil and ValidateCommit stays silent.
ClassifyChannelTransition: every matrix row's From tuple applies; each
row's To tuple under another writer is LostToCompetitor with that
writer; the own-opid case is AlreadyApplied (write the trailer with
the same helper TrailerPresent's tests use); one foreign tuple gives
the channel-transition error with the printed tuple. ChannelInboxMutate:
the three branches. ChannelOpid: shape through validOpidShape.

# Verification

`go build ./...`, `gofmt -l` clean, `go vet ./internal/goal/...`, `go
test ./internal/goal/... ./internal/landing/...`, `go run
honnef.co/go/tools/cmd/staticcheck@2025.1 ./internal/goal/...`, and
`scripts/agents/pre-commit-guard-fixtures.sh` green. Every path in
your return (diffBoundary, files) is relative to the repository root,
so it starts with `metasystem/`.

# Constraints

Wall-clock budget: 45 minutes. Nothing under plans/channel/ is written
by any verb in this step; no verb, flag or config key is added; the
provider contract, StripCode, Poll, the steward and recovery are
untouched.

# Gap Rule

Stop and report a gap only for a law-changing contract: a new
authority, a new refusal the design does not name, a landing bar, or
a fleet-read schema the design does not give. A mechanical choice —
a helper's name, a test fixture's shape, a struct's exact tag where
the design's table names the key, an error's wording beyond the codes
the table fixes — is made from what the tree does nearest the seam,
recorded in the return under `decisions`, and built; a choice recorded
in the return is not silent. Where an example in this brief contradicts
the tree's existing law, the law wins, the choice is recorded under
`decisions`, and the item is built.
