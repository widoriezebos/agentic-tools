# spend-fence-reports-tokens-per-model-and-cause: design, revision 3

- Kind: design
- Id: 01M3A2YHDNJGCMYACYCAMFRRB1
- Status: accepted
- Goals: spend-fence-reports-tokens-per-model-and-cause

Owns units U5a, U5b and U5c of plans/seats-spend-tokens-in-bounded-sessions-design.md
(P7) and SSTB-114, 115, 116, 204, 205, 206 and 207. Revision 3, 2026-09-15,
by a fresh bounded delegate, answers the seven findings round 2 left open;
the eight it closed (SF-1 to SF-8) are unchanged.

## What changed in revision 3

- SF-9: witness 13 (count 2). SF-10: section 8, m1e 389.2M; section 9.
  SF-11: sections 6 and 9.
- SF-12: 2.3; witnesses 4, 9. SF-13: 2.1, 5; witness 3. SF-14: 2.6, 5;
  witness 18. SF-15: 2.6; witnesses 13, 14.
- Sweep: witness 1's result order (now main, delegate, engine, main) and
  witness 11's early `spend.zone` (moved to U5a-3), both fixed; nothing
  else contradicts its predicate. Non-material notes taken.

## 1. What the fence cannot say today

The health line prints one UTC-day number (internal/steward/health.go:492-496)
from a ledger that reads only top-level `<session>.jsonl` files under the
checkout's slugs (internal/spend/transcript.go:80-95) and skips every
session a job record names (transcript.go:100-104): no `subagents/`, no
other seat, no model, no cause. The diagnosis
(records/misc/token-diagnosis-2026-09-15.md) showed 871.7M against 301.8M;
its script is the reference for what to read, not rerun for DONE.

## 2. The mechanism

### 2.1 The call, its identity, and the reader seam (SF-8, SF-6, SF-2, SF-13)

The attributed unit is one API call: one assistant transcript entry. Its
identity is `message.id`, then `requestId` when absent, then `<file>:<line>`:
the prototype's order (token-diagnosis-2026-09-15.py:128, `m.get('id') or
e.get('requestId')`), which counted the 871.7M baseline; shipped code uses
`requestId` then `file:line` (transcript.go:335-338). The key is global
across every file one run reads, as in the prototype: a repeated key in the
same file keeps the first entry's fields and raises `output_tokens` to the
maximum seen; a repeated key in another file is ignored whole, the
earliest-stamped entry being kept. Entries with model `<synthetic>` or no
usage object stay unmeasured requests, as today. Every call carries the
canonical model, the timestamp, five raw classes (`input`, `cacheCreation`,
`cacheRead`, `output`, `reasoning` from `thinking_tokens`), file, line,
session, kind, cause and detail. The legacy `Tokens` row keeps its shape
and meaning: its `input` is still input plus cache creation
(transcript.go:462-464).

The seam is `internal/spend/reader.go`. A registration is
`reader{name, capability, inScope, discover, scan}`: `capability` is
`per-call` or `none`; `inScope` says whether the runtime is in P4's scope.
The Claude reader registers `name=claude`, `per-call`, `inScope=true`, and
is the only registration. Nothing above the seam holds the string `claude`:
health, the ledger's `attribution.scope` and the verbs print
`scope=<label>`, the in-scope names joined by `+`. A record whose `runtime`
names no in-scope reader is skipped by that field and counted as
`out-of-scope` (today every Codex job). A record whose runtime is in scope but
whose reader's capability is `none` adds no tokens and counts as `unknown`.
Adding a reader in scope is a code change that changes the printed label,
never a silent widening.

Scope filters the attribution pass only (SF-13): every terminal job record
of any runtime still enters `measured` with `dayEligible`
(internal/spend/measure.go:217-223), `aggregateRows` and `summarizeDay`
(measure.go:343-378), so `Rows`, `DayScope`, `Seat.DayTokens`, goal scopes,
crossings and episodes hold every token they hold today; `out-of-scope` is
a count, never a subtraction (witness 3).

`discover(seatRoot)` returns `{files []transcriptFile, gaps
[]UnmeasuredEntry, counters}`; a `transcriptFile` carries path, session, the
`subagents/` flag and parent session.
`scan(file, cursor, jobs)` returns `{calls, unmeasured []UnmeasuredEntry,
foreign, aged, cacheWriteFailed bool, err error}`; `err` is the one fatal
route, the file could not be read, recorded as `recordUnreadable` does
today (transcript.go:46-56). Every seat counter (`Files`, `AgedFiles`,
`UnreadableFiles`, `SkippedForeignFiles`, `CacheWriteFailures`,
`UnmeasuredRequests`) is filled from these results, so no gap visible today
goes dark. `reader_claude.go` is transcript.go renamed, the only file that
knows the `~/.claude/projects` slug layout, `subagents/`, `isSidechain`,
`promptSource`, `queued_command` attachments and the `Stop hook feedback:`
text.

### 2.2 Kinds, engine ownership and the window stamp (SF-1)

- `main`: a top-level file under a seat slug that no in-scope record owns,
  plus every file owned by a record with role `steward-continuation`
  (cmd/metasystem/steward_verbs.go:464, goal.go:821).
- `delegate`: a file under `<session>/subagents/`, at any depth.
- `engine`: every other file an in-scope record owns.

Ownership: a record owns the file named by its `sessionId`. When several
records name one session, the shipped shape for a resumed follow-up
(internal/dispatch/build.go:856-861 copies the parent's `sessionId`, line
927 gives the child its own `startedAt`), the records are ordered by
`startedAt`, ties by `jobId`, and each call belongs to the record with the
latest `startedAt` not after the call's timestamp; a call stamped exactly at
a `startedAt` belongs to that record; calls before the earliest `startedAt`
belong to the earliest record. Each call has one owner, so nothing counts
twice, and each job's calls move whole with its own start.
`resumedSessionId` never owns: for a resumed child it equals the child's own
`sessionId` (internal/dispatch/hazard.go:499); for a fresh-context child it
names the parent's session, which the parent owns. Both fields keep a file
out of `main`. Every engine call takes its owner's `startedAt` as its window
stamp. A record without `startedAt` stays unmeasured as today
(measure.go:203-208). A running job's calls count with the tokens read so
far; the ledger lists it under `inflight` as today.

### 2.3 Causes, turn starters and delegate kind (SF-3, SF-12, SSTB-207)

The cause vocabulary is closed: `human`, `stop-hook`, `notification`,
`peer`, `compaction`, `usage-limit-resume`, and `unstarted` before a file's
first starter; delegate and engine calls take `delegate:<kind>`. A main
call takes the cause of the latest starter before it in its file, even one
before `from`: the whole file is parsed. A turn counts in `turns` only when
its starter lies in `[from, to)`, so a row may show calls with zero turns
(token-diagnosis-2026-09-15.py:156-167).

A starter is a user entry with no `tool_result` block matching a rule
below, first match wins, on `origin.kind` (o), `promptSource` (ps) and
trimmed text (s) (py:48-61):

1. s starts `Stop hook feedback`: `stop-hook`; detail the block reason
   between `Stop blocked;` and the next `;` (`unparsed` when absent), so
   the proof step can exclude `context-over-trigger` (P5).
2. o `task-notification`, or s starts `<task-notification>` or
   `[SYSTEM NOTIFICATION`: `notification`; detail the source (`bash`,
   `monitor`, `agent`, `other`).
3. o `peer`, or s starts `Another Claude session sent a message`, or
   `<cross-session-message` in the first 400 characters: `peer`; o
   `coordinator` or s starts `The coordinator sent a message`: `peer`,
   detail `coordinator`.
4. s starts `This session is being continued`: `compaction`; o
   `auto-continuation` or s starts `Your claude.ai usage limit`:
   `usage-limit-resume`.
5. ps `sdk` or s starts `# Task Direction`: `human`, detail `sdk`.
6. o `human`, ps `typed` or `queued`, or s starts `<local-command`,
   `<command-name>`, `<bash-input>`, `<bash-stdout>` or `[Request
   interrupted`: `human`; detail names which.
7. Else `isMeta`: not a starter (a skill expansion, py:60).
8. Else: `human`, detail `unclassified`, counted as
   `cause-unclassified=<n>` in health; the diagnosis found none.

A `queued_command` attachment (a mid-turn delivery) never starts a
segment; in a delegate file the first user message is the brief.

The kind domain is one closed set of four values, `design`, `build-read`,
`critique`, `other`, for native delegates and engine jobs alike; revision
1's `build` is withdrawn. For a native delegate the kind is the first
non-blank line of the file's first user message, matched by
`^Kind: (design|build-read|critique|other)$` after trimming; any other
first line is `other` and increments `kind-missing`, shown in health; later
messages never change it. For an engine job the kind comes from `roleKinds`
in the new internal/spend/attribute.go, whose value type is the four-value
enum, so a fifth value does not compile. The complete pre-U4 map, from the
role strings shipped code writes: `design-critic`, `code-critic` and
`critic` to `critique`; `implementer` to `other`; `steward-continuation` is
`main`, not a delegate kind; any other string to `other`. Before U4,
`design` and `build-read` come only from native delegates' `Kind:` lines.
U4 may append rows mapping its launcher's role names to `design` or
`build-read`; the domain does not change and nothing else does. The 09-15
baseline delegates predate rule S3 and are `other`; the baseline is a total,
unaffected.

### 2.4 Cursor schema 2 and the job digest (SSTB-116, SF-7)

The per-transcript cursor (cache.go:39-51) becomes schema 2: file-level
`kind`, `parentSession`, `delegateKind`, `kindLine` (`present`, `missing`,
`pending` until the first user message arrives), `lastCause`, `lastDetail`,
a `starters` list of (line, timestamp, cause, detail), and per request
`messageId`, `timestamp`, `cause`, `detail` and the five raw classes.
`DelegateDigest` becomes `jobDigest`: for every job record under the root,
sorted by record path, the line
`path|jobId|sessionId|resumedSessionId|role|runtime|startedAt`, an absent
value written `-`, lines joined by newline, hashed with sha256. A record
that appears, disappears or changes any of those fields invalidates every
cursor of that root (as transcript.go:192 does today). Status and `endedAt`
are left out so a running job's growth does not force reparses; the size
and mtime check covers growth. A schema-1 file is rebuilt from offset 0
through the existing invalid-cache path (cache.go:98-108). `spend window`
keeps its own cursors under
`<root>/artifacts/agents/steward/spend/window-cache/`, pruned by the verb,
out of `Measure`'s prune (transcript.go:244-270).

### 2.5 The zone day inside Measure (SF-5)

The ledger becomes schema 2 and gains `attribution`: `scope`, `window`
(`from`, `to`, `zone`, `day`), by kind, by model (five classes and total),
by cause (turns, calls, tokens), `outOfScope`, `unknown`, `kindMissing`,
`causeUnclassified`, `landings`, `tokensPerLanding`. It is windowed, not
lifetime: `Measure` computes `window` as `[midnight of now in spend.zone,
next midnight)` and puts a main or delegate call in it by its own
timestamp, an engine call by its owner's `startedAt`. `spend.zone` (beside
internal/config/spend.go:13-18) defaults to `UTC`, so an unset zone gives
the attribution the legacy day. The legacy fields keep their meaning and
UTC key: `Path` (measure.go:115), `ledger.Day`, `DayScope` and
`Seat.DayTokens` (transcript.go:180-182) are not touched. Health prints
`day=<attribution.window.day>` for the split; the legacy `day=` stays.

### 2.6 Health, ceilings and landings (U5b, SSTB-114, 206, SF-9, SF-14, SF-15)

The spend-fence reason gains, after the existing text: `; scope=<label>
day=<zone day> main=<M> delegate=<M> engine=<M> out-of-scope=<n>
unknown=<n> kind-missing=<n> cause-unclassified=<n>; model
<m>=<tokens>/<ceiling> ...; cause human=<M> stop-hook=<M> notification=<M>
peer=<M> compaction=<M> usage-limit-resume=<M> unstarted=<M>
delegate:design=<M> delegate:build-read=<M> delegate:critique=<M>
delegate:other=<M>; landings=<n> per-unit=<M>`. A model with no ceiling
prints `<tokens>/-`; every observed model is shown.

Per-model ceilings: `spend.ceiling.model.<canonical model>.tokens`, beside
the day and goal keys, set by Wido, absent by default. A model over its line
joins the crossing list as scope `model-<m>`, ceiling `tokens`; the alert
episodes take it unchanged (health.go:486-490 renders any crossing).

Landings: a landed unit is a landing commit, one that adds to
memory/receipts.log one or more lines with `outcome=shipped` and `type=`
one of `implement`, `refactor`, `improve`, `design`. Revivals are
`type=other` with `skills=steward` and never count; a goal History row is
not a receipt. Resolution in internal/spend/landings.go, with `os/exec`
git: `git log --format=%H %ct --since=<from-1d> --until=<to+1d> --
memory/receipts.log` per root, the date options only a traversal bound; the
in-process predicate `from <= ct < to` on the parsed `%ct` instants, the
same `[from, to)` the token window uses, decides membership; `git show
--format= --unified=0 <sha> -- memory/receipts.log` keeps added lines that
parse as landing receipts. Commits are deduplicated by sha across roots and
attributed by the receipt's new `seat=` field. A line without `seat=` falls
back to the goal's claimant at the receipt's stamp from the goal file's
History; one it cannot place counts under seat `unknown`.

`seat=` (SF-14): `receipt add` calls `goal.ResolveMachine(root)`
(internal/goal/actor.go:22-28, an error when unenrolled) and passes the
name to `receipt.Add` (internal/receipt/receipt.go:153) as an optional
field like `read_tokens` (receipt.go:227-229); `--seat` overrides. On an
error the receipt is written as today, without the field, exit 0, one
stderr line; enrollment never gates a receipt (witness 18).

`tokensPerLanding` (SF-15): a `*int64`, window total over landing count,
rounded down, when the count is positive; at zero landings, an ordinary
window, JSON `null` and text `unknown` (`per-unit=unknown`), never numeric
zero, so nothing divides or fails to marshal (witnesses 13, 14).

### 2.7 The verbs (U5c, SSTB-205, 115, SF-4, SF-10)

One family, `spend`, in `families()` (cmd/metasystem/main.go:727-743 routes
by family and verb).

`spend window --root <checkout> (--from <RFC3339> --to <RFC3339> | --day
<date>) --zone <zone> --seat-roots <a,b,c> [--expect <tokens>] [--json]`.
`--root` supplies config and window-cache; each seat root is enrolled to
name its seat and slugged as the reader does. The window is `[from, to)`;
endpoints carry their own offsets; the zone labels days and expands `--day`
to that day's midnights. Output per seat: total, by kind, by model with the
five classes, by cause with turns, calls and tokens, `out-of-scope`,
`unknown`, `kind-missing`, `cause-unclassified`, landings and tokens per
landing; then one fleet line, the in-scope sum. `--expect` prints the
deviation and exits 1 over 2 percent.

`spend compare <the window flags> --provider <file> [--total-only]`. The
provider file, records/misc/provider-usage-<date>.json, holds `provider`,
`from`, `to`, `zone` and `models`, a map from the provider's model name to
`input`, `cacheCreation`, `cacheRead`, `output`; or `total` with the same
four fields when the page gives no split. The verb refuses a file whose
`from`, `to` or `zone` differ from its flags. Projection, both sides:
`input`, `cacheCreation`, `cacheRead` as they are, `output` as local
`output + reasoning`; model names pass through `config.CanonicalModel` on
both sides; the key set is the union, an absent model is zero on its side.
Per model and for the grand total the compared figure is the four-class
sum; `abs(local - provider) <= 0.10 * provider` passes, so exactly ten
percent passes; provider zero passes only with local zero. Classes print
per model for diagnosis and do not gate. A `total`-only file compares only
the total and exits 1 with `per-model: not proven` unless `--total-only` is
passed; the DONE proof never passes that flag, so a per-model provider file
is a precondition of the proof (section 9).

`spend reads --root <checkout> --last 5`: the last five receipts carrying
`read_tokens` and `read_calls` (receipt.go:227-232), each against 2M tokens
and 40 calls, pass or fail per row.

## 3. Settlements by id

- SSTB-114: 2.6.
- SSTB-115: 2.7.
- SSTB-116: 2.4, 6.
- SSTB-204: 2.1, 2.2.
- SSTB-205: 2.7, 8.
- SSTB-206: 2.6.
- SSTB-207: 2.3.

## 4. Runtime independence

The verbs, the ledger, the receipt field and the health text carry a runtime
name only as a field value: `runtime` copied from a record, or the scope
label composed from registrations. The registry is the one place code
names a runtime; only a reader parses a transcript.
`internal/usage` serves the context gate and stays separate; `claudeSlug`
may be shared, nothing else. A runtime without per-call usage reports
`unknown` with a count, never a derived number.

## 5. The R-115-m1e check

Answers near R-115:

- Withdrawing `build` (SF-3) removes a proposed value, not a landed one;
  implementer rounds stay visible as `delegate:other` and by model.
- Leaving status and `endedAt` out of `jobDigest` (SF-7) hides nothing;
  ownership and the stamp use only digest fields; witness 7.
- The scope filter (SF-13) touches only `attribution`; the legacy stream
  and its crossings are today's code; witness 3.
- `seat=` (SF-14) is optional; a resolution failure never fails `receipt
  add`; witness 18.
- Nine units (SF-11) keep every witness and floor of revision 1.

No floor is lowered, no witness or gate removed, no DONE narrowed;
total-only `spend compare` is an added diagnostic, not a relaxation.

## 6. Units in dependency order (SF-11)

Changed lines are `git diff --stat -M` on the landing, renames detected,
tests included. Each cap is hard; a builder who cannot fit stops and reports
rather than widening. Every unit runs its witnesses and the fast gate. No
witness needs a later unit: witness 1 (record index, continuation rule)
runs at U5a-3 and U5a-1 ends on witness 17; witness 7 grows (U5a-2
identity, classes, digest, rebuild; U5a-3 kinds; U5a-4 causes);
`spend.zone` lands in U5a-3 for witness 11.

| unit | cap | contents | ends on |
|---|---|---|---|
| U5a-1 | 300 | reader.go registry, capability, scope, typed results; transcript.go renamed; recursive discovery; delegate flag and parent from the path | `seat.files` counts subagent files; witnesses 17, 6 |
| U5a-2 | 300 | identity by `message.id`; cursor schema 2; five raw classes; `jobDigest`; schema-1 rebuild; job-record index | warm equals cold; witnesses 7 (part), 8 |
| U5a-3 | 300 | ownership by latest start; scope filter and counts; continuation rule; `spend.zone`; zone-day window; by kind and model | by-kind and by-model; witnesses 1, 2, 3, 11, 7 (kinds) |
| U5a-4 | 300 | turn starters, details, `causeUnclassified`, `Kind:`, `roleKinds`, `kind-missing`; by-cause rows | by-cause rows; witnesses 4, 5, 9, 7 (causes) |
| U5b-1 | 250 | per-model ceilings, crossings, episodes; health additions | health prints the split; witness 12 |
| U5b-2 | 300 | `seat=` with the fallback write; landings.go; claimant fallback; `tokensPerLanding`; landings in ledger and health | health prints landings; witnesses 13, 18 |
| U5c-1 | 300 | `spend` family and router line; `spend window` with `--day`, `--expect`, `--json`, window-cache | section 8 baselines run; witnesses 14, 16 |
| U5c-2 | 250 | `spend compare`, provider file reader | witness 15 |
| U5c-3 | 150 | `spend reads --last 5` | witness 10 |

## 7. Witnesses (SSTB-311)

Every witness is a Go test on inline fixtures (a temporary home, transcript
lines, job records, a temporary git repository); none needs the bed or a
live session.

1. `TestKindsFollowPathsAndJobRecords`: a top-level file, a `subagents/`
   file two levels down, a file owned by an implementer record and one by a
   `steward-continuation` record give main, delegate, engine, main.
2. `TestSharedSessionCallsHaveOneOwnerEach` (SF-1): two records with one
   `sessionId`, `startedAt` 23:30 and 00:30 Europe/Amsterdam, calls at
   23:45, 00:30 exactly and 00:45; the first window holds the first call,
   the second the other two; the sum equals a cold total; no call twice; a
   `resumedSessionId` alone owns nothing.
3. `TestScopeIsDeclaredNotInferred` (SF-2, SF-13): a second per-call reader
   registered out of scope leaves its records `out-of-scope` and its files
   unread; an in-scope reader with capability `none` gives zero tokens and
   `unknown=1`; the label stays `claude`; a `codex` record over the day
   ceiling is `out-of-scope=1`, adds nothing to `attribution`, and still
   sits in `Rows` and `DayScope` with its `day` crossing.
4. `TestCauseFollowsTheTurnStarterNotAQueuedDelivery` (SF-12): each rule of
   2.3 on a sample; a `queued_command` attachment and an `isMeta` expansion
   leave the cause; an unmatched entry gives `human`, `unclassified`, the
   counter; `unstarted` before any starter.
5. `TestKindDomainIsFourValuesBeforeAndAfterU4Rows` (SF-3): the `Kind:`
   line's four values, a missing line counted, a later message ignored;
   `roleKinds` on the five shipped role strings and an unmapped one; the
   output set equals the four values before and after two appended rows
   yielding `design` and `build-read`.
6. `TestSeamKeepsEveryVisibleGap` (SF-6): an unlistable slug directory, an
   aged file, a foreign file, a malformed usage line and a read-only cache
   directory reach the same counters and unmeasured entries as today.
7. `TestCursorV2MatchesAFullParseAndRebuildsV1` (SF-7): extends
   measure_test.go:413 across appends, with kinds from U5a-3 and causes
   from U5a-4; a schema-1 file forces a reparse; a changed `sessionId`,
   `resumedSessionId`, `startedAt` or role each forces one, a changed
   status does not; measure_test.go:702 (no transcript bytes on a warm
   read) stays green.
8. `TestCallIdentityReproducesThePrototype` (SF-8): repeated entries for one
   `message.id` in one file count once at the maximum output; the same id in
   two files counts once with the earliest entry's fields; no `message.id`
   keys by `requestId`, neither by line; `<synthetic>` is unmeasured; the
   total equals the hand-computed fixture total.
9. `TestByCauseRowsCarryTurnsAndCalls` (SF-12): two turns of three calls
   in the window plus a starter at `from - 1h` with two calls inside give
   turns 2, calls 8; that cause's row shows turns 0, calls 2.
10. `TestSpendReadsLastFive`: five rows, pass and fail.
11. `TestHealthAttributionUsesTheZoneDay` (SF-5): calls at 21:59Z and 22:01Z
    on a summer day with `spend.zone=Europe/Amsterdam`; the attribution
    holds only the second, `day` is the next local date, the ledger file key
    and `Seat.DayTokens` are the UTC values they are today.
12. `TestModelCeilingCrossingAlerts`: a model over its line yields
    `model-<m>.tokensx1` and an episode; no ceiling, no crossing, and the
    line prints `/-`.
13. `TestLandingsUseTheHalfOpenWindow` (SF-9, SF-15): commits at `from - 1s`,
    `from`, inside, `to`, `to + 1s` in a non-UTC zone count 2; two receipts
    in one commit count one; a revival counts zero; `seat=` wins over the
    fallback; one sha in two roots counts once; zero landings give
    `tokensPerLanding` `null` without error.
14. `TestSpendWindowIsHalfOpenInTheZone` (SF-15): the flags, `--day`
    expansion, endpoint offsets, the fleet line, `--expect` pass and fail;
    `--json` with zero landings marshals `null` and the text prints
    `unknown`; `metasystem spend` without a verb prints the family usage.
15. `TestSpendCompareProjectionAndTolerance` (SF-4): cache creation and
    reasoning placed as specified; a model on one side only; provider zero
    with local zero and nonzero; exactly ten percent passes, one token more
    fails; a window or zone mismatch refuses; a total-only file exits 1
    without the flag and passes the total with it.
16. `TestBaselineCommandShapesRun` (SF-10): the exact flag shapes of
    section 8 against an inline fixture home and root, `--expect` with a
    fixture total.
17. `TestDiscoveryFlagsSubagentFilesFromPaths` (SF-11): with no job record,
    a `subagents/` file two levels down carries the flag and parent session
    from its path, a top-level file neither; `seat.files` counts both.
18. `TestReceiptSeatFieldNeverGates` (SF-14): enrolled writes
    `seat=<name>`; `--seat other` wins; unenrolled writes today's line
    without `seat=`, exit 0.

## 8. Proof of DONE and the commands (SF-10)

`M1B`, `M1C`, `M1E` stand for the checkouts
`/Users/wido/LocalStorage/GitHub/agentic-tools-m1{b,c,e}/metasystem`.

Baseline, the seat's first act after U5c-1 lands; figures from
token-diagnosis-2026-09-15.md table 1 for 00:00 to 12:51 Europe/Amsterdam
(revision 2's m1e 316.3M was UTC-day main 264.4M plus delegates 51.9M,
another window):

```
metasystem spend window --root $M1C --from 2026-09-15T00:00:00+02:00 \
  --to 2026-09-15T12:51:00+02:00 --zone Europe/Amsterdam \
  --seat-roots $M1B,$M1C,$M1E --expect 871700000 --json \
  > records/misc/spend-baseline-2026-09-15-fleet.json
metasystem spend window --root $M1C --from 2026-09-15T00:00:00+02:00 \
  --to 2026-09-15T12:51:00+02:00 --zone Europe/Amsterdam \
  --seat-roots $M1E --expect 389200000 --json \
  > records/misc/spend-baseline-2026-09-15-m1e.json
```

Both exit 0 or the build has a defect. Proof day `D`, after Wido sets one
`spend.ceiling.model.<m>.tokens` and writes
records/misc/provider-usage-D.json with per-model figures:

```
metasystem health                       # on each seat: the split and the ceiling
metasystem spend window --root $M1C --day D --zone Europe/Amsterdam \
  --seat-roots $M1B,$M1C,$M1E --json > records/misc/spend-proof-D-window.json
metasystem spend compare --root $M1C --day D --zone Europe/Amsterdam \
  --seat-roots $M1B,$M1C,$M1E --provider records/misc/provider-usage-D.json
metasystem spend reads --root $M1C --last 5
```

The compare exits 0; commands and output go to
records/misc/spend-proof-D.md. The program's proof row (P7) reuses these
verbs.

## 9. What Wido must approve

The goal's budget (elapsedLimit=4h, attemptLimit=6, reviewRoundLimit=2)
covers design and critique. The build is nine units, each a Codex round with
an Opus read, at most 2,450 changed lines by cap, about 2,000 by estimate.
Tuple to approve: elapsedLimit=20h, attemptLimit=14, reviewRoundLimit 2
unchanged; both raises exceed the tier-2 box. Proof preconditions: a
per-model provider file for day D (2.7) and one per-model ceiling. One
correction: P4's m1e row should carry 389.2M or name its own window.

## Unchecked

Not checked: a goal History `claim` row's shape; that a Claude engine
job's transcript always lands under the seat's slug; the internal/gittree
API; the provider export's shape and per-model classes; the shipped
`<synthetic>` filter; the cmd/metasystem caller of `receipt.Add`; whether
a model-crossing episode should carry the zone day rather than
`ledger.Day`. The diagnosis confirms a delegate file's first user entry is
the brief.
