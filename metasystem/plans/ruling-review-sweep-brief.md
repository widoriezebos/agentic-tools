Working Mode: implement
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal ruling-review-sweep)
Date: 2026-08-31

# Goal

The steward's ruling review sweep conforms to the recorded goal intent
(plans/goals/ruling-review-sweep.md) in two ways it currently does not:

1. A register row that lacks an accountable owner or carries a malformed
   review condition no longer degrades the steward tick. The sweep surfaces
   that row to the human inside its existing daily digest entry — listed
   BEFORE the due reviews — while every healthy row is still swept. Today
   `readRulingReviews` returns an error for such a row, `sweepRulingReviews`
   propagates it, and `tick.go` degrades the whole tick: the sweep silences
   itself exactly when it has something to say.

2. A row whose review condition carries a FUTURE due date is not yet due.
   Today `eligibleRulingReviews` surfaces a row with an unobservable event
   token even when its due date has not arrived (the R-29 rows, review date
   2026-09-06, appear in the digest today). The due date is the mechanical
   backstop for an event the sweep cannot observe; surfacing before it is a
   defect against the goal's "surfaces rulings past review".

# Workspace

The dispatch-created job worktree, branched from main. May touch EXACTLY
two files:

- internal/steward/ruling_sweep.go
- internal/steward/ruling_sweep_test.go

Everything else is out of bounds, including internal/steward/tick.go
(the fix must make its degradation path simply not fire for register
content defects), the register memory/rulings.md itself, and plans/.

# Inputs

- plans/goals/ruling-review-sweep.md — the prescribing intent: "a sweep
  surfaces rulings past review; ownerless entries go to Wido for adoption
  or withdrawal".
- internal/steward/ruling_sweep.go — current mechanism: row parsing
  (`readRulingReviews`), review-condition grammar (`parseReviewCondition`,
  four schedulable classes, due=YYYY-MM-DD and/or event=token),
  eligibility (`eligibleRulingReviews`, `rulingEventEvidence` with its
  unobservable-event default), the 5-item ceiling with ID rotation, the
  24-hour digest cadence, and the sweep state file (schema 1, afterId,
  lastDigestAt).
- internal/steward/ruling_sweep_test.go — existing coverage to keep green.
- records/narrator-digest.log — examples of the emitted entry shape.

Mechanical rules — these leave no judgment calls:

ROW CLASSIFICATION. While scanning `| R-` rows, each of the three current
error cases becomes a surfaced defect item instead of an error:
- wrong column count → defect, labeled by the row's 1-based position among
  `| R-` rows (the id cell cannot be trusted), reason naming the count;
- empty owner cell → defect, labeled by the row's id, reason "no
  accountable owner";
- `parseReviewCondition` failure → defect, labeled by the row's id, reason
  from the parse error.
Healthy rows parse exactly as today. A real file-read failure (anything
other than the file not existing) remains an error, as does the scanner
error path — content defects never error, I/O defects always do.

ELIGIBILITY. For each healthy row with a review condition:
- due set and due <= today → due, evidence `due=<date>` (unchanged);
- due set and due > today → NOT due, with one exception: an event that is
  mechanically observed (a positive observation from `rulingEventEvidence`'s
  known-event cases, never its unobservable default) makes the row due with
  its event-observed evidence;
- no due and event set → unchanged behavior (observed → due; unobservable →
  due with the needs-attention evidence).

DIGEST COMPOSITION. One entry per 24 hours, exactly as today. Defect items
lead the entry text, before any due review (the weakest item leads). An
ownerless defect carries a `choice=adopt|withdraw` tail; a malformed-
condition or malformed-column defect carries its reason. At most 5 defect
items are rendered; more than 5 append a `+N more` tail, mirroring the due
list's existing overflow phrasing. The due list keeps its existing
formatting, ceiling, and afterId rotation, and the rotation state advances
only over due reviews, never over defects (defects are always shown while
present — they have no rotation). An entry is emitted when defects exist
even if no review is due. The sweep state schema does not change.

# Constraints

- Non-goals: no new delivery or notification path (the file's "Rows never
  acquire their own delivery or notification path" comment stays true); no
  change to the sweep state schema; no change to the digest cadence,
  ceiling value, or entry kind; no edits to tick.go or any file outside
  the two named; no register content edits; no test weakened to pass.
- Wall-clock budget: 30 minutes.
- Exact digest wording within the facts above is yours; code structure is
  yours.

# Expected Return

Version-2 implementer JSON: `schemaVersion` (2), `jobId`, `round`,
`runtime`, `sessionId`, `model`, `evidence`, `gaps`, `mode`,
`riskiestPart`, `diffBoundary`, `whatWasDone`, and `claimed` (null
sessionId/model unless different from what the harness observed). Lead
with the riskiest part. `diffBoundary` lists exactly the two files.
Evidence entries keep the `{command, observed, level}` schema; each
command is replayable verbatim from the worktree root, including:

- `go test ./internal/steward/ -run Ruling -count=1`
- `go test ./internal/steward/ -count=1`

# Acceptance Criteria

1. `go test ./internal/steward/ -count=1` passes in the worktree.
2. A test proves: given a register fixture containing one ownerless row
   and one malformed-review-condition row alongside healthy rows,
   `sweepRulingReviews` returns nil and the appended digest entry names
   both defective rows before any due review, the ownerless one with
   `choice=adopt|withdraw`.
3. A test proves: a row with a schedulable class, a due date after the
   sweep's `now`, and an unknown event token does NOT appear in the digest.
4. A test proves: a row with a due date after `now` and the event
   `superseded-by-r22-m1`, in a register where R-22-m1 exists, DOES appear
   with event-observed evidence.
5. Existing tests pass unmodified, except tests whose expectations the two
   prescribed behavior changes legitimately alter — name each such change
   in `whatWasDone`.
6. The diff touches exactly internal/steward/ruling_sweep.go and
   internal/steward/ruling_sweep_test.go.

# Gap Rule

stop and report a gap; never fill it silently.
