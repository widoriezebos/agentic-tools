# g1-s11 Goal detail

- Gate 1, state `designed`, **revision 1**, author Claude on Fable as a D23 delegate, 2026-09-22. Full pipeline: a data contract (one route, one payload the brain's tools will read later) and the first nested route of the shell.
- Refines the [master](../user-interface-design.md) at `ui-development` commit `c2b7ef4b7`: Backlog item detail (`:549` to `:562`), Execution evidence (`:621` to `:639`, the capture-gap labels at `:637`), Rules for an intuitive structure (`:433` to `:449`, the two kinds of emptiness at `:437`), the rule that constraints are not parsed conventions hidden inside Intent (`:507`), `DefinitionRefs` (`:509` to `:511`), the slice-plan owner (`:602`), the split that refuses once slicing has started (`:615`), allowed actions as an owner addition (`:261`), a historical result's revision (`:346`), the terminal-command rule (`:733`), the gate 1 and gate 6 rows (`:809`, `:814`). Plan: the `g1-s11` row (`:196`), D6 (`:132`, settled by `g1-s21`), D32 (`:51`), the conventions (`:114` to `:123`), the must-not-touch list (`:110`), M5 and M7 (`:144`, `:148`), the `g1-s6` and `g1-s16` rows (`:190`, `:204`).
- Depends on `g1-s10` revision 4 **as built** on branch `ui/g1-s10` at `46dab9a5d` (six commits, the Go half and the list, not yet merged: the design's `internal/backlog`, `internal/ui/snapshot`, the `/api/backlog` route, and the `_app/src/backlog/**` pane all exist there); `g1-s9` revision 3 (merged); `g1-s21` revision 2 (designed, not built: branch `ui/g1-s21` is behind `ui-development` and carries nothing) for the `document` reference kind only; D1, D2, D11, D20, D21, D32. Not on `g1-s7`, `g1-s5`, or `g1-s6`.
- Discharges nothing in full. Contributes to scenarios 15 (the resolver gains the `goal` kind), 21 (a blocker is directly inspectable from the goal), and 34 (an active item shows its authoritative status and names what cannot be reconstructed). Scenario 33, a completed item linked to its delegate rounds, checks, findings, and landing evidence, is gate 6's and is **not** claimed here.

Paths are relative to `metasystem/` unless they start with `plans/`; `_app/` is `internal/ui/web/_app/`. Every code claim was read whole in source at `c2b7ef4b7`; `internal/goal` is byte-identical between `94181f05a`, `f5768f525`, and `c2b7ef4b7` (`git diff --stat` on the package is empty across all three), so `g1-s10`'s citations into it stand. Claims about `g1-s10`'s build were read on `ui/g1-s10` at `46dab9a5d` and are marked *branch*. Nothing was executed but read-only `git`, `grep`, `awk`, and `wc` over the checkout; no goal verb, no server, no `metasystem.conf.local`.

## Outcome

A human opens a goal from the backlog list and sees one workspace with six tabs, Summary, Definition, Plan, Execution, Evidence, and History, each showing what the accepted ledger records about that goal and naming, in the master's two kinds, what it does not: what the record itself has no field for, and what exists elsewhere but this build does not read, each with where it lives and which slice or gate reads it. The claimed goal `tests-parallel-and-deterministic` shows its claim by machine `m1e`, its budget, its proven approval, one accepted read item, and nine History lines each labelled with the revision it produced; a queued goal whose History is mostly other goals' rank shifts shows those shifts folded behind a count rather than as its own events; a concluded goal from the archive opens the same way with its conclusion. The page has a stable address, `/backlog/goals/<id>`, and the list's rows become links to it. Nothing is mutable, nothing is inferred from prose, no execution phase is invented, and no job, receipt, or landing artefact is counted that this node does not hold.

## Scope and non-goals

In scope: a detail projection in `internal/backlog` beside the lane projection, so the terminal can adopt it in `g1-s16`; one prefix route `GET /api/goals/<id>`; the `goal` kind in the resolver and the first nested route beneath `/backlog`; six tabs; the rows of the list becoming links; the cut guard's third entry. Not: any write; the goal editor (gate 4); `DefinitionRefs` and the linked document editors (gate 5); the split editor, slice plan, and proposal comparison (gate 5); job records, chains, delegates, and spend (`g1-s6`, `g1-s13`, gate 6); landing receipts, task receipts, the journey, the evidence root, and the flight recorder (gate 6, and one open question below); questions and rulings as records of their own (`g1-s12`); reading an older revision than the tip's (gate 4, result cards); a board, outline, or dependency graph; the event stream (`g1-s7`); rendering the three prose fields as a Markdown tree (a pane-only change once `g1-s21` lands; the API carries source text either way).

## Existing code this builds on

| Fact | Where |
| --- | --- |
| `GoalFile`, the whole record: `Id`, `State`, `Priority`, `Sequence`, `Tier` (zero tolerated), `Risk`, `Intent`, `Origin`, `StopSurfaceMoves`, `NextStep`, `Conclude`, `OpenedAt`, `Revision`, `Blocked`, `Labels`, `Arc`, `Pinned`, `Budget`, `BudgetExtension`, `BudgetExceptions`, `NormApproval`, `Approved`, `Sliced`, `Ratified`, `Claimed`, `Obligation`, `ReviewObligations`, `AcceptedRisks`, `ReadItems`, `StopCapability`, `StopFence`, `Landing`, `Episode`, `Parked`, `Abandoned`, `Legacy`, `History`; two unexported legacy flags. **No execution phase, no attempt count, no delegate, no seat session, no spend, no definition reference, no next transition** | `internal/goal/file.go:23` to `:91` |
| The parser refuses a file without Intent, Origin, OpenedAt, a non-zero Revision, or any History line, so every projected goal has all five | `file.go:596` to `:598`, `:611` to `:624` |
| The sub-records: `RiskRecord` with `DerivedTier()` and `GateWidth()`; `ReviewObligation`; `AcceptedRiskRecord`; `ReadItem` with JSON tags; `GoalNormApprovalClaim`; `ApprovalRecord` (`By`, `At`, `Revision`, `EpisodeRevision`, `Opid`, `Authority`, `Digest`, `ReviewBy`); the four authority words; `ApprovalExpired`; `BudgetExtensionRecord`; `SlicedRecord`; `HandedOver`; `ClaimRecord`; `LandingRecord`; `EpisodeRecord`; `StopCapability`; `StopFence`; `ParkRecord`; `AbandonRecord` | `file.go:107` to `:140`, `:175` to `:208`, `:234` to `:254`, `:290` to `:306`, `:310` to `:428` |
| `Budget` is `goalbudget.Budget`: `ElapsedLimit`, `AttemptLimit`, `ReservedJobMinutesLimit`, `ActiveJobLimit`, `ReviewRoundLimit`; `GovernedObligation` is `governance.GovernedObligation` with seventeen named fields | `internal/goal/budget.go:28`; `internal/goalbudget/budget.go:82` to `:88`; `internal/goal/obligation.go:60`; `internal/governance/types.go:71` to `:89` |
| `HistoryLine`: `At`, `Opid`, `Verb`, `Actor` (`machine+lineage` or `human:<name>`), `Targets`, `Displaced`, `StopID`, `Resumed`, `Carried`, `Ack`, `Keep`, the authority fields, the five channel fields, `ApprovedRef`, `Question`, `Reason`; `reason=` is always last and consumes the remainder | `file.go:438` to `:463`, `:1951` to `:1955` |
| Every verb write appends one History line and bumps Revision by one, together | `internal/goal/verbs.go:367` to `:373` |
| The validator indexes History by revision: `f.History[a.Revision-1]` for the approval event, the abandon event, and the claim event | `file.go:839`, `:919`, `:1000` |
| The engine's own idiom for reading a `key=value` out of a `reason=` tail | `file.go:2163` to `:2166` (`reasonField`) |
| Rank shifts fan out: a `set-priority` touches every goal whose rank moved, with `reason=priority-order subject=<id> from=… to=… requested-sequence=…`; a `done` or `abandon` that compacts a priority merges `priority-order from=… to=…` into the affected goals' lines | `internal/goal/order.go:138` to `:149`, `:226` to `:240`; `internal/goal/abandon.go:415` to `:418` |
| `RootRecord`: `Decomposed []DecomposedEntry` (`Id`, `Opid`, `At`, `OldArc`), `PowerOfAttorney`, `History` | `internal/goal/root.go:21` to `:41`, `:234` to `:239` |
| `TreeGoals`: `Root`, `Live`, `Done`, `DonePaths`, `Abandoned`, `AbandonedPaths`; `Archived(id)`; `Exists(id)` | `internal/goal/validate.go:27` to `:64` |
| At-rest rules this projection may rely on: a done goal has a conclusion; a decomposed parent is never live; every `BlockedBy` names a goal that exists | `validate.go:245` to `:247`, `:296` to `:301`, `:319` to `:323` |
| `OrderedOpenGoalIDs`, `SortedGoalIds`; `arcMembers` is unexported and covers live goals only | `order.go:31` to `:55`; `validate.go:705`; `verbs.go:3301` to `:3316` |
| `OpenReadItemBlocks`, the read-item states | `internal/goal/readitems.go:11` to `:48` |
| `MarkSliced` writes `Sliced` with the claim's revision; "there is deliberately no command-line mount" | `internal/goal/sliced.go:5` to `:36` |
| `SplitRatification` (`Tier`, `By`, `MainID`, `ClaimEpoch`, `DraftSHA256`) | `internal/goal/split.go:33` to `:39` |
| `ResolveMachine(root)` is `git config --get metasystem.goal.machine` through the engine's git contract; an unenrolled machine is an error naming the fix | `internal/goal/actor.go:22` to `:29` |
| `validId`: lowercase kebab, at most 100 characters; ids are portable path components | `internal/goal/goal.go:409` to `:420` |
| Job records: one JSON file per job under `<root>/artifacts/agents/jobs/<jobId>.json`; the typed lens reads `status`, `phase`, `role`, `parentJob`, `goalId`, `goalRevision`, `capMin`, `endedAt`, `error`, `round`; the status graph and its terminal set; spend is projected by scanning that directory; no exported lister | `internal/dispatch/admission.go:21`; `jobrecord.go:17` to `:120`; `record.go:56` to `:65`; `budget.go:369` to `:370`; `chain.go:53` to `:58` |
| Landing receipts: `<root>/artifacts/agents/landing/receipts/<tree>.json`, volatile; task receipts: `memory/receipts.log`, one `|`-separated line per record with a `goal=` field, whose package exports `Add`, `Correct`, `Retro`, `Stats`, `Check` and no reader; the flight recorder: `<root>/artifacts/agents/events.jsonl`; the evidence root mirrors chain artefacts out of `artifacts/` | `internal/landing/receipt.go:74` to `:76`; `internal/receipt/receipt.go:1` to `:26`, `:155` to `:561`; `internal/events/emit.go:133`; `internal/evidence/gc.go:1` to `:11` |
| The law: a queued goal carries no budget; split turns a large parent into an arc of members before slicing; `land-ready` writes `Landing`; the kept `Episode`; drop versus abandon; a concluded goal's story goes to `docs/journey.md` with no identifiers in the prose | `docs/backlog-mechanism.md:7` to `:13`, `:103` to `:111`, `:192` to `:223`, `:225` to `:240`, `:304` to `:326` |
| *Branch*: `Info.Observe func() snapshot.Observation`, per request, nil is a 500; `backlogPath` matched exactly beside `workspacePath`; `writeError`; `backlogOf`; `stamp`; `problemLines` | `ui/httpd/httpd.go:32` to `:36`, `:136` to `:143`; `ui/httpd/backlog.go:17`, `:77` to `:91`, `:93` to `:130`, `:148` to `:161` |
| *Branch*: `Observation` (`State`, `Tip`, `CommittedAt`, `Tree`, `Horizon`, `Admission`, `Fetch`); the six states | `ui/snapshot/snapshot.go:20` to `:69` |
| *Branch*: `Ref`, `Row`, `Board`, `Project`, `LaneOf`, `rowOf` (which takes the last History line as `LastVerb`), `openBlockers`, `parkedFrom`, `Admission`, `Admit`, `DraftStatement`, `PhaseNotRecorded` | `internal/backlog/project.go:11`, `:16`, `:27` to `:31`, `:78` to `:121`, `:126` to `:152`, `:159` to `:202`, `:204` to `:266`, `:271` to `:300`; `admit.go:14` to `:35` |
| *Branch*: `ui.go` wires `Observe: ledger.Observe` inside `NewHandler` | `cmd/metasystem/ui.go:158` to `:168` |
| *Branch*: `cuts.test.ts` holds `CALL_SITES` as `[file, calls, resources[]]` with two entries; the scan is token-aware and tells a field named `fetch` from the global | `_app/src/cuts.test.ts:61` to `:63`, `:389` to `:395`, `:412` to `:426` |
| *Branch*: `Shell.tsx` routes `/backlog` to `BacklogPane`; `GoalRow` renders the id as copyable text "because this build has no goal view to open yet"; `api.ts` types `Row`; `state.ts` is the abort-controller pattern; `routes.ts` is untouched; the `backlog` empties row is gone | `_app/src/shell/Shell.tsx:21`, `:112`; `backlog/GoalRow.tsx:10` to `:18`; `backlog/api.ts:20` to `:68`; `backlog/state.ts:20` to `:46`; `git diff ui-development 46dab9a5d -- _app/src/routes.ts` empty |
| HEAD: `sectionFor` lights a section beneath its prefix; `activeSection` matches a section exactly and its comment says a nested route changes it; `routeFor` answers `section` only; the test already uses `/backlog/goals/g1` and asserts `activeSection` of it is null | `_app/src/routes.ts:56` to `:64`, `:66` to `:78`, `:84` to `:91`; `routes.test.ts:56`, `:73` to `:78` |
| HEAD: `Pane`, `EmptyState`; the identity pattern; `titleFor`; the shell's classes, among them `ms-pane-stack`, `ms-card`, `ms-card-title`, `ms-facts`, `ms-fact-name`, `ms-fact-value`, `ms-chip`, `ms-mono`, `ms-skeleton` | `_app/src/panes/Pane.tsx:17` to `:47`; `shell/identity.tsx:27` to `:54`; `title.ts:14` to `:28`; `panes/Settings.tsx:25` to `:61`; `shell/shell.css` |
| The `ui` family row and the package-map rows | `docs/architecture.md:145` to `:148`, `:162` |

### What is on disk, read-only, 2026-09-22

155 live goals (`plans/goals/*.md` less `backlog.md`), 430 archived (`records/goals/`: 429 done, 1 abandoned). On every one of the 585, the number of History lines equals `Revision`. History depth: live median 48 lines, longest 214 (`token-spend-fence`, parked, of which 197 are rank shifts); archived median 8, longest 237. Verbs over all lines: `done` 7,127, `set-priority` 6,627, `edit` 1,508, `claim` 607, `open` 540, `approve` 451, `release` 365, `set-budget` 253, `unapprove` 214, `slice-start` 155, `ack` 108, `park` 92, `set-pin` 76, `unpark` 48, `migrate` 46, `abandon` 34, `ask` 12, `set-obligation` 11, `set-arc` 11, `breach-stop` 10, `resume` 9, `answer` 6, `reopen` 5, `accept-risk` 5, `grant` 4, `steal` 2, one each of `read-items-add`, `read-items-close`, `land-ready`, `extend-budget`, `enroll-terminal`, `engine-floor`. 7,127 `done` lines against 429 done goals, and 34 `abandon` lines against one abandoned goal, are the fan-out: **47 live goals (36 queued, 10 parked, 1 approved) have `done` as the verb of their last History line**, because another goal's conclusion compacted their priority. Fields present: `Sliced` 21 live and 134 archived; `Approved` 14 and 196; `Budget` 58 and 272; `Risk` 108 and 237; `Tier` 109 and 262, so **46 live goals have no tier**; `Pinned` 25 and 36; `Labels` 33 and 84; `BlockedBy` 10 and 5; `Arc` 5 and 10 (two arcs, `headless-fleet` and `app-guardrail-program`); `Parked` 33, none with `blocker=`; `Claimed` 1; `StopCapability` 1; `ReadItem` 1 (accepted); `AcceptedRisk` 4 archived; `NormApproval` 7 archived; `BudgetExtension` 1 archived; `Ratified`, `Landing`, `Episode`, `Obligation`, `ReviewObligation`, `StopFence`, `LegacyNotes`: none. The root record has `FormatVersion` 1, four `PowerOfAttorney` entries, **zero `Decomposed` entries**, and 108 `ack` lines naming goals. On this node: `artifacts/agents/jobs/` **does not exist**, `artifacts/agents/mains/` holds only `worktree-lease.lock`, `artifacts/agents/events.jsonl` does not exist; `memory/receipts.log` has 962 lines, 227 naming a goal; `records/counselor/` holds `accepted-risk-register.jsonl` (6 lines), `misclassification-register.jsonl` (4), `carried-landings.jsonl` (0); `docs/journey.md` has 42 chapters. The machine nickname this node is enrolled under was not read (it is `git config metasystem.goal.machine`); the walkthrough records it.

## Contracts

### `internal/backlog`: the detail projection

New files `detail.go`, `history.go`, and their tests in the package `g1-s10` created; imports `internal/goal` and nothing under `internal/ui`; pure over a tree already in memory. It reuses `Row`, `LaneOf`, `openBlockers`, and `parkedFrom` unchanged and adds:

```go
// Detail is one goal as a reader opens it: the row the list shows, then what
// each tab can say from the record, and what it cannot.
type Detail struct {
    Row        `json:"row"`
    Node       Node       `json:"node"`
    Definition Definition `json:"definition"`
    Plan       Plan       `json:"plan"`
    Execution  Execution  `json:"execution"`
    Evidence   Evidence   `json:"evidence"`
    History    History    `json:"history"`
}

// Absent is the first kind of emptiness: the record has no field for this,
// or the field is empty. Reason is one sentence about the record.
type Absent struct{ What, Reason string }
// NotProjected is the second kind: the records may exist, elsewhere, and this
// build does not read them. Where names the home; Reach says whether it is at
// the accepted tip, on the claiming node, on this node, or outside the
// repository; Arrives names the slice or gate.
type NotProjected struct{ What, Where, Reach, Arrives string }

type Node struct{ Machine, Message string }   // this node's nickname, or why it is not known

type Definition struct {
    Intent, Origin, NextStep string
    Tier                     uint8
    Risk                     *Risk        // Severity, Novelty, Exposure, Accumulation, Basis, DerivedTier, GateWidth
    Priority                 uint8
    Sequence                 uint64
    Labels                   []string
    Arc, Pinned              string
    BlockedBy                []Edge       // ID, State ("" when absent), Where, Open
    StopSurfaceMoves         bool
    Budget                   *goal.Budget
    NormApproval             *NormApproval
    Approved                 *ApprovalDetail // Approval plus Revision, EpisodeRevision, Opid, Digest
    BudgetExceptions         uint16
    Absent                   []Absent
    NotProjected             []NotProjected
}

type Plan struct {
    Arc        string
    Members    []Member          // every goal in the arc, live and archived, by id: ID, State, Where, Lane
    Dependents []Edge            // goals whose BlockedBy names this one, live and archived
    Sliced     *goal.SlicedRecord
    Ratified   *Ratified         // Tier, By, MainID, ClaimEpoch, DraftSHA256
    Decomposed *Decomposed       // the root's entry for this id: Opid, At, OldArc
    Absent     []Absent
    NotProjected []NotProjected
}

type Execution struct {
    Claim          *ClaimDetail     // Claim plus Revision, AccountingRevision, EpisodeAt, EpisodeRevision, EpisodeObligationRevision, IdleSeconds, HandedOver
    Landing        *goal.LandingRecord
    Episode        *goal.EpisodeRecord
    StopCapability *goal.StopCapability
    StopFence      *goal.StopFence
    Budget         *goal.Budget
    BudgetExtension *goal.BudgetExtensionRecord
    BudgetExceptions uint16
    Obligation     *goal.GovernedObligation
    Absent         []Absent
    NotProjected   []NotProjected
}

type Evidence struct {
    Approved          *ApprovalDetail
    ReadItems         []goal.ReadItem
    AcceptedRisks     []goal.AcceptedRiskRecord
    ReviewObligations []goal.ReviewObligation
    BudgetExtension   *goal.BudgetExtensionRecord
    Concluded         string
    Abandoned         *AbandonDetail // Abandoned plus Revision, Opid, Displaced, StopID, Carried
    Decisions         []Line         // the History lines whose verb is ask, answer, accept-risk, grant, breach-stop, resume, land-ready
    Absent            []Absent
    NotProjected      []NotProjected
}

type History struct {
    Lines      []Line   // every line of the goal's History, in record order
    Root       []Line   // lines of the root record whose Targets name this goal
    Legacy     []string
    Revisioned bool     // true when len(Lines) == Revision, so each line's Revision is meaningful
    Absent     []Absent
    NotProjected []NotProjected
}

// Line is one History line as the grammar has it, with two derived facts.
type Line struct {
    Revision   uint64   // index + 1 when Revisioned, else 0
    Class      string   // "verb" | "rank"
    At, Opid, Verb, Actor string
    Targets    []string
    NamesThis  bool     // Targets contains the goal
    Displaced, StopID, Resumed, Carried string
    Ack        bool
    Keep       int
    Authority  *Authority // Outcome, Generation, ReviewBy, Ruling, Word
    Channel    *Channel   // Provider, User, Ref, Context, Step
    ApprovedRef, Question, Reason string
}

func DetailOf(tree *goal.TreeGoals, horizon goal.ApprovalHorizon, admission Admission, id string, node Node) (Detail, bool)
func Classify(line goal.HistoryLine, id string) Line
```

**Summary is the row.** `Detail.Row` is `rowOf` on the same file, so the list and the page cannot disagree about lane, phase, gaps, or badges (O1). Two additions the row does not have, both on `Detail`: `Row.LastVerb` and `LastChangeAt` keep `rowOf`'s meaning, the last line whatever its class; the Summary tab shows beside them "last verb on this goal", the last line whose `Class` is `verb`, and "last rank change", the last whose class is `rank`, because on 47 live goals the last line is another goal's `done` and a Summary that read "last done" on a queued goal would be false. The next expected transition is `Absent{"next transition", "the record has no such field; the allowed actions for a subject arrive with g2-s6 (master :261)"}`; the master's "latest durable result" is the conclusion for a done goal, the abandon reason for an abandoned one, and otherwise `Absent{"latest result", "the record holds no result; job returns live under artifacts/agents/jobs on the claiming node"}`.

**Definition.** Every field named above is copied from the file; `Risk.DerivedTier` and `GateWidth` are the engine's methods (`file.go:138`, `:142`), never recomputed. `Tier` 0 carries `Absent{"tier", "not classified; 46 live goals carry no Tier and Risk"}`. A nil `Budget` on a live goal carries `Absent{"budget", "a queued or draft goal carries no budget; the tuple arrives with the claim (docs/backlog-mechanism.md:7)"}`. `Approved` nil carries `Absent{"approval", "no human has approved this revision"}`; present, `ApprovalDetail.Expired` and `ExpiredWhy` come from `f.ApprovalExpired(horizon)` as the row's do. `BlockedBy` edges carry each blocker's state from `tree.Live`, `tree.Done`, `tree.Abandoned`, `Where` accordingly, and `Open` by `openBlockers`' rule; an id in none of the three is `State ""` with the row's existing "unknown goal" gap, though the at-rest rule at `validate.go:319` makes that unreachable on a validated tree. Fixed `NotProjected` entries: `{"definition references", "no field; typed DefinitionRefs arrive with gate 5 (master :509)", "not recorded", "gate 5"}`, `{"constraints and success criteria", "owned intent and design documents (master :507)", "at the accepted tip", "gate 5"}`, `{"the goal editor", "a write", "", "gate 4"}`. Intent, NextStep, and Conclude are carried as the one-line source text they are; nothing is parsed out of them (master `:507`), so a commit hash or a `plans/` path inside them is text, not a link.

**Plan.** `Members` is every goal in `tree.Live`, `tree.Done`, and `tree.Abandoned` whose `Arc` equals this goal's non-empty `Arc`, by `SortedGoalIds` of each map, live first; a goal with no arc has `Members` empty and `Absent{"arc", "not in an arc"}`. This is a different question from `arcMembers` (`verbs.go:3301`), which serves arc joins over live goals only, so no export is taken and no law is duplicated: membership here is the data fact `Arc == x`, and if `g1-s16` wants one owner for the live half, a one-line `ArcMembers` export is a D32 case then. `Dependents` is every goal in the three maps whose `Blocked` names this id, with `State` and `Where`. `Decomposed` is the root's entry for this id (`root.go:234`); on this ledger there are none. `Sliced` and `Ratified` are copied. Fixed `NotProjected`: `{"slices", "no slice-plan owner exists (master :602); the first-slicing fact is Sliced", "not recorded", "gate 5"}`, `{"split editor and proposal comparison", "a write", "", "gate 5"}`; and one `Absent` when `Sliced` is set: `{"split", "goal split refuses once slicing has started (master :615); a successor is the path"}`.

**Execution.** Everything the claim binding carries is copied: `ClaimDetail` is the row's `Claim` plus the revision fields, `HandedOver` when present (`file.go:326`), `Landing`, the kept `Episode`, `StopCapability`, `StopFence`, the budget tuple, its extension, the exception count, and the obligation. Fixed `Absent`: `{"execution phase", "the claim record proves ownership and landing and nothing finer (M5)"}`. Fixed `NotProjected`, each with its reach: `{"attempts, rounds, delegates, roles, time, and returns", "job records under <state root>/artifacts/agents/jobs/<jobId>.json keyed by goalId and goalRevision (dispatch/jobrecord.go:65, :99)", reach, "g1-s6 and g1-s13 for this node; gate 6 for other nodes"}` where `reach` is `"on this node"` when `Node.Machine` equals `Claim.Machine`, `"on node <machine>, not this one"` when both are known and differ, and `"on the claiming node; this node's nickname is not known: <Node.Message>"` otherwise; `{"spend against the budget", "projected from those job records (dispatch/budget.go:369)", the same reach, "g1-s6"}`; `{"the seat's session", "announcements under artifacts/agents/mains/ on the claiming node", the same reach, "g1-s6 and g1-s13"}`. An unclaimed goal carries the same entries with reach `"no claim; nothing is executing"`.

**Evidence.** The record's evidence-shaped fields are copied: the approval with its digest, read items, accepted risks, review obligations, the extension's evidence coordinate, the conclusion, the abandon record with its `Carried` successor, and `Decisions`, the History lines whose verb is one of `ask`, `answer`, `accept-risk`, `grant`, `breach-stop`, `resume`, `land-ready`, classified as under History. Fixed `NotProjected`, each with where and reach, in this order: landing receipts (`artifacts/agents/landing/receipts/<tree>.json`, `landing/receipt.go:76`, volatile, on the landing node; gate 6); task receipts (`memory/receipts.log`, at the accepted tip but outside the goal subtree the validated read covers, `validate.go:599`, and with no exported reader; open question 2); the accepted-risk register (`records/counselor/accepted-risk-register.jsonl`, at the tip; `g1-s12`); briefs, patches, critiques, and returns (chain artefacts under `artifacts/agents/` on the claiming node, mirrored to the evidence root by `internal/evidence`; gate 6); the flight recorder (`artifacts/agents/events.jsonl`, `events/emit.go:133`, local, absent here; gate 6); the journey chapter (`docs/journey.md`, at the tip, written without identifiers by law (`docs/backlog-mechanism.md:322`), so no chapter can be found by goal id; offered as a `document` reference to the whole file, which `routeFor` resolves once `g1-s21` lands and shows as text until then). Fixed `Absent` for a live goal: `{"landing", "no landing is recorded; Landing is written by land-ready"}` when nil. The master's capture-gap labels (`:637`) map as: "Not recorded" is `Absent`; "Not yet captured", "Unavailable on this node", and "Removed under retention policy" are the `Reach` strings of `NotProjected`; "Integrity check failed" cannot arise, because a torn file refuses the whole tree before projection (`g1-s10`).

**History.** `Lines` is every line of `f.History` in record order through `Classify`; `Root` is every line of `tree.Root.History` whose `Targets` contains the id, in record order, classified the same way (the 108 `ack` lines: "displacement acknowledgments write here", `root.go:7` to `:8`). `Revisioned` is `len(f.History) == f.Revision`; when true, `Line.Revision` is index plus one, the number the validator itself uses to name the approval, abandon, and claim events (`file.go:839`, `:919`, `:1000`) and the number `touch` advances with each line (`verbs.go:368`); when false, every `Revision` is 0 and `Absent{"revision labels", "the record is at revision N but carries M History lines; which line produced which revision is not recorded"}` is carried, a case no record on this ledger reaches and which is kept for one that does. `Class` is `"rank"` when `Reason` begins with `priority-order`, the exact prefix the three writers emit (`order.go:146`, `:234`; `abandon.go:417`) and which nothing in the engine reads, so this is the first reader and not a second owner; every other line is `"verb"`. Nothing else is parsed out of `Reason`: the `subject=` and `from=`/`to=` words stay in the verbatim reason, where a human reads them, so no rank is inferred and no goal is named from prose. `NamesThis` is `Targets` containing the id, so a `rank` line that was this goal's own `set-priority` still shows as the record has it. What Git carries and why it is not used: every publish is one commit whose message is the verb (`sliced.go:14`), so `git log` over the ledger subtree would give the same lines keyed by commit, plus the commit ids; reading it needs the engine's git contract, which exports no log; a clone's log reaches only what it fetched; and the record's History is complete by the ledger's law (`AGENTS.md:35`: the ledger mutates only through `goal` verbs, and a hand edit goes through `goal reconcile`, which also writes History). The archived file keeps the whole History, so a concluded goal's past survives its leaving the live set. Fixed `NotProjected`: `{"commit ids per line", "the ledger commits", "at the accepted tip", "not planned; the opid is the operation's identity"}`.

### The route

`GET /api/goals/<id>` in `internal/ui/httpd`, a prefix route beside the two exact ones, in a new file `goal.go` with `goalsPrefix = "/api/goals/"`; `/api/goals` and `/api/goals/` are 404 by the existing reserved rule (branch `httpd.go:156` to `:159`, HEAD `:252`); an `<id>` containing `/`, or empty, is 404 before the tree is consulted, so `/api/goals/x/y` and `/api/goals/../` never reach a lookup, and the lookup itself is a map read, never a path. The chain gains one `if strings.HasPrefix(r.URL.Path, goalsPrefix)` after the `backlogPath` branch (branch `httpd.go:140` to `:143`); no mux is introduced, and `g1-s21`'s two branches sit in the same chain whichever lands second (Reconciliation). The route answers from `h.info.Observe()`, the closure `g1-s10` added, called once per request; a nil `Observe` is the same 500 `{"error":"this engine was built without a ledger reader"}` (branch `backlog.go:79` to `:82`). No new `Info` field is needed for the goal itself, because the detail is a projection of the same observation the list is; one is added for the node: `Info.Machine func() (string, error)`, wired in `ui.go` as `func() (string, error) { return goal.ResolveMachine(roots.StateRoot) }`, called per request, whose error becomes `node.message` and whose nil is `node.message` `"this engine was built without a machine resolver"`, **200 either way**, because the goal is answerable without it; this is a stated deviation from the nil-is-500 rule, which applies to a route's primary closure. `ResolveMachine` runs one `git config --get` through the engine's scrubbed contract (`actor.go:23`), the same class of per-request read as `ResolveEndpoint` in `Observe` (`g1-s10` M4, M5).

```json
{"schemaVersion":1,"observedAt":"2026-09-22T09:12:03Z",
 "ledger":{"state":"read","tip":"c5d517f4…","committedAt":"2026-09-21T18:39:42Z","stale":true,"staleAfterSeconds":1800,
           "syncMode":"remote","stateRoot":"/Users/wido/…/metasystem","message":"","problems":[],"fetch":{"…":"…"}},
 "goal":{"row":{"ref":{"kind":"goal","id":"tests-parallel-and-deterministic","revision":9},"where":"live","lane":"in-progress",
                "phase":"not recorded","state":"claimed","gaps":["phase not recorded"],"…":"…"},
         "node":{"machine":"m1","message":""},
         "definition":{"intent":"Make every test safe…","origin":"human","tier":3,
                       "risk":{"severity":3,"novelty":2,"exposure":3,"accumulation":3,"basis":"Test scheduling…","derivedTier":3,"gateWidth":"full"},
                       "priority":0,"sequence":0,"labels":[],"arc":"","pinned":"","blockedBy":[],"stopSurfaceMoves":false,
                       "budget":{"elapsedLimit":"2d6h","attemptLimit":12,"reservedJobMinutesLimit":720,"activeJobLimit":8,"reviewRoundLimit":3},
                       "normApproval":null,
                       "approved":{"by":"human:Wido","at":"2026-09-21T18:39:41Z","authority":"proven","reviewBy":"","expired":false,"expiredWhy":"",
                                   "revision":9,"episodeRevision":9,"opid":"9FXDRPSD…","digest":"20eab20c…"},
                       "budgetExceptions":3,
                       "absent":[{"what":"rank","reason":"unranked"}],
                       "notProjected":[{"what":"definition references","where":"no field; typed DefinitionRefs arrive with gate 5 (master :509)","reach":"not recorded","arrives":"gate 5"},"…"]},
         "plan":{"arc":"","members":[],"dependents":[],"sliced":null,"ratified":null,"decomposed":null,"absent":[{"what":"arc","reason":"not in an arc"}],"notProjected":["…"]},
         "execution":{"claim":{"machine":"m1e","lineage":"main-1789893000-16595-1a7a6a","at":"2026-09-21T18:39:41Z","landingAt":"",
                               "revision":9,"accountingRevision":9,"episodeAt":"2026-09-21T03:36:57Z","episodeRevision":3,"episodeObligationRevision":0,"idleSeconds":0,"handedOver":null},
                      "landing":null,"episode":null,
                      "stopCapability":{"generation":9,"revision":9,"machine":"m1e","claimEpoch":3,"fenceEpoch":0},"stopFence":null,
                      "budget":{"…":"…"},"budgetExtension":null,"budgetExceptions":3,"obligation":null,
                      "absent":[{"what":"execution phase","reason":"the claim record proves ownership and landing and nothing finer (M5)"}],
                      "notProjected":[{"what":"attempts, rounds, delegates, roles, time, and returns","where":"job records under <state root>/artifacts/agents/jobs/<jobId>.json keyed by goalId and goalRevision (dispatch/jobrecord.go:65, :99)","reach":"on node m1e, not this one","arrives":"g1-s6 and g1-s13 for this node; gate 6 for other nodes"},"…"]},
         "evidence":{"approved":{"…":"…"},"readItems":[{"id":"c1-native-delegate-budget-read-1-1","read":"c1-native-delegate-budget-read-1","text":"N-1: …","state":"accepted","closingReference":"A one-line…","addedAt":"2026-09-21T12:11:17Z","changedAt":"2026-09-21T12:15:57Z"}],
                     "acceptedRisks":[],"reviewObligations":[],"budgetExtension":null,"concluded":"","abandoned":null,"decisions":[],
                     "absent":[{"what":"landing","reason":"no landing is recorded; Landing is written by land-ready"}],"notProjected":["…"]},
         "history":{"revisioned":true,"legacy":[],
                    "lines":[{"revision":1,"class":"verb","at":"2026-09-21T03:36:50Z","opid":"E8NGT9NQ…","verb":"open","actor":"human:Wido","targets":["tests-parallel-and-deterministic"],"namesThis":true,
                              "displaced":"","stopId":"","resumed":"","carried":"","ack":false,"keep":-1,"authority":null,"channel":null,"approvedRef":"","question":"","reason":""},"…"],
                    "root":[],"absent":[],"notProjected":["…"]}}}
```

`ledger` is `backlogOf`'s `ledgerPayload` reused verbatim, so the detail page shows the same tip, age, and fetch clause as the list. In every ledger state but `read`, `goal` is `null` and the status is **200**, as the list's rule has it: the workspace answered and the answer is why the ledger cannot be read. In state `read`, an id in none of the three maps is **404** `{"error":"no goal \"<id>\" on the accepted tree (tip <tip>)"}`, the words `goal show` uses (`cmd/metasystem/goal.go:482`). Field names are lower camel case, `omitempty` on nothing, so a reader can tell "absent" (`null`, `[]`, `""`) from "not sent"; a Go test asserts every name verbatim (O3). Times are RFC 3339 strings as the record has them; `keep` is `-1` when absent, as the parser has it (`file.go:1937`). HEAD on the route does the GET's work and `net/http` drops the body; no route branch (as `g1-s21` O9).

### Frontend

New under `_app/src/goal/`: `api.ts` (`loadGoal(id, signal)`: one `fetch("/api/goals/" + encodeURIComponent(id), {headers: {Accept: "application/json"}})`, the third call site, and the types mirroring the payload), `state.ts` (`useGoal(id)`: `loading | failed | notFound | known`, the `state.ts` pattern of `g1-s10` with an `AbortController`, refetched when `id` changes and on Refresh, **never on a tab change**), `tabs.ts` (the six tab ids, titles, and one-sentence meanings, in the master's order), `GoalPane.tsx`, `GoalHeader.tsx`, `Tabs.tsx`, `Summary.tsx`, `Definition.tsx`, `Plan.tsx`, `Execution.tsx`, `Evidence.tsx`, `History.tsx`, `Facts.tsx` (a `dl.ms-facts` from a list of name and value), `Gaps.tsx` (renders `absent` and `notProjected` under the two headings "Not recorded" and "Not read by this build, and where it lives"), `goal.css`, tests. Edited: `routes.ts`, `routes.test.ts`, `shell/Shell.tsx` (two routes), `backlog/GoalRow.tsx` (the id becomes a `NavLink` when `routeFor(row.ref)` is non-null; the comment at `:10` to `:11` goes), `cuts.test.ts` (one entry appended), `title.ts` unchanged, `panes/empties.ts` unchanged.

**Routes.** `/backlog/goals/:id` is the Summary; `/backlog/goals/:id/:tab` with `tab` one of `summary`, `definition`, `plan`, `execution`, `evidence`, `history`; any other tab is the not-found pane inside the shell. `routeFor({kind: "goal", id})` returns `/backlog/goals/` plus `encodeURIComponent(id)`; the revision, when a reference carries one, is not put in the route (the page is the object's, and older revisions are unreadable in this build; the pane says so when it matters, below). `sectionFor` already lights Backlog beneath `/backlog/` (`routes.ts:62`). `activeSection` changes: it gains a table `nested` of `[prefix, sectionId]` pairs, this slice's entry `["/backlog/goals/", "backlog"]`, and answers the section for a path beneath a listed prefix; `routes.test.ts:76` flips from null to `backlog`; `g1-s21` adds `["/project/doc/", "project"]` to the same table, whichever lands second (Reconciliation). The tab title is `titleFor(`${id} · Backlog`, identity)`.

**The page**, top to bottom in the work area: a `NavLink` "Backlog" back, 13px/500 `accent`; the id in mono `xl`; the same chip row `GoalRow` shows (state, tier, rank, pinned, labels, arc, sliced, split into goals, rev), from the same `Row`; the intent, `md` 16/24; the ledger line `g1-s10`'s `LedgerLine` renders, unchanged, with its Refresh; then `<nav aria-label="Goal sections">` of six `NavLink`s styled as tabs, 32 high, 13px/500, `text-2`, the current one `text` with a 2px `accent` underline and `aria-current="page"`; then the tab's content in one `.ms-pane-stack`. Tabs are links, not ARIA tabs, because each has an address; Tab moves through them, Enter follows. Switching tabs reads from the state already held: one request per goal page view, none per tab (O7).

**Summary**: `.ms-facts` of outcome (the intent), state, lane, phase, where (live or archived), ownership (`claimed by <machine> (<lineage>) since <at>`, `landing since <at>`, or "unclaimed"), blockers (each an id linked through `routeFor`, with its state, and "(open)"), dependents (linked), last verb on this goal, last rank change, then "Latest result" (the conclusion, the abandon reason, or the `Absent` line), "Next step" (the seat's own prose, verbatim, labelled so), "Next expected transition" (the `Absent` line), then `Gaps`. **Definition**: `.ms-facts` of every field, the risk as four scores with the basis and "derived tier N (the engine's rule), gate width W", the budget as five facts, the approval as by, at, authority, review by, expired verdict, revision, digest in mono (first twelve characters, the whole on hover and in a `title`), then `Gaps`, then a `text-3` line "Governing designs: not recorded until typed references arrive at gate 5". **Plan**: the arc and its members as rows (id linked, state chip, lane chip), dependents, the sliced record as facts, the decomposed entry when present, then `Gaps`. **Execution**: the claim as facts, landing, episode, stop capability, stop fence, budget, extension, exceptions, obligation (each field name and value; `effects` and `authorizedEffects` as their string forms), then `Gaps`; a goal with no claim shows "unclaimed" and the same `Gaps`, whose reach says nothing is executing. **Evidence**: the approval digest, read items as a table (id, read, state, added, changed, text, closing reference), accepted risks, review obligations, the extension's evidence, the conclusion or abandon record, then "Decisions recorded on this goal" as History rows, then `Gaps`, in which the journey entry renders as `routeFor({kind: "document", id: "metasystem/docs/journey.md"})` when non-null and as mono text otherwise. **History**: a `text-3` line "N lines, the record is at revision N" (or the `Absent` line when not revisioned); a `Button` with `aria-pressed`, "Show rank changes (k)", off by default, React state only; then the lines in record order as rows: `rev N` in mono, the time (local, `dateAndTime` from `g1-s10`'s `format.ts`), the verb as a `Chip`, the actor in mono, then the facts that are present, one per line: targets as "this goal" when the only target, else "n goals, including this one" or "n goals" with a `<details>` listing them each linked through `routeFor`; displaced, stop id, resumed, carried, ack, keep, authority (outcome, generation, review by, ruling, the quoted word), channel (provider, user, ref, step, context), approved ref, question, and the reason verbatim in `text-2`. Rank lines, when shown, sit in the same order with a `Chip` "rank"; the human reads `subject=` in the reason. Then "In the root record" with the `Root` lines the same way, and "Legacy notes" when any. Nothing is truncated; a 214-line History is 214 rows, and the walkthrough measures.

Wording keeps the shell's conventions: nothing asserts the absence of records; every `NotProjected` row names the home, the reach, and the gate; the only terminal step on any page is none.

**Older revisions.** The page shows the record at the tip. When a reference reaches the page with a revision (none does in this build; the list's rows carry the tip's), the pane compares nothing, because it has nothing to compare against: reading an older revision needs a git read this build does not do, and the master's stale-result notice (`:346`) arrives with result cards at gate 4. Stated here so nobody expects it.

## Behaviour

Main flow: a row's id in the list is a link; the page mounts, shows a `Skeleton` in the header and the ledger line, requests `/api/goals/<id>` once; the server observes (the tree held after the first read at a tip; gates and admission fresh; the machine resolved), projects, answers; the pane renders the Summary or the tab in the address.

| Case | Required result |
| --- | --- |
| The id is live | `where` live; the lane, phase, and gaps the list shows for the same row |
| The id is archived (done or abandoned) | `where` archived; Summary's latest result is the conclusion or the abandon reason; Plan and Execution show what the archived record kept (the `Sliced` record on 134 of them); the same six tabs |
| The id is unknown at a `read` tip | 404; the page shows "No goal `<id>` at tip `<tip>`", the id in mono, a link to Backlog; the rail works |
| The ledger is `absent`, `no-ledger`, `broken`, `unreadable`, or `refused` | 200 with `goal` null; the page shows the same statement the list shows for that state (`g1-s10`'s six panes), with its fetch clause; no goal is claimed absent |
| `/api/goals`, `/api/goals/`, `/api/goals/x/y`, `/api/goals/../backlog` | 404 before any lookup; the page for `/backlog/goals/` is the not-found pane |
| An id with characters outside `validId`'s set (`goal.go:409`) | Looked up and not found; 404. The route does not re-implement `validId`, because a map read is safe for any string |
| `Info.Machine` errors ("no machine nickname is enrolled…") or is nil | 200; `node.machine` `""`, `node.message` the words; Execution's reach lines read "on the claiming node; this node's nickname is not known: …" |
| The goal is claimed by this node | Execution's reach reads "on this node"; the `NotProjected` rows still say this build does not read the job records and name `g1-s6` and `g1-s13` |
| A queued goal whose last History line is another goal's `done` (36 of them) | Summary shows "last verb on this goal: open …" and "last rank change: done … (priority-order …)"; the row's `lastVerb` is unchanged and the list's wording is `g1-s10`'s (Reconciliation) |
| A goal with 197 rank lines out of 214 (`token-spend-fence`) | History shows 17 rows and "Show rank changes (197)"; on, 214 rows in record order |
| A record whose History length differs from its revision | `revisioned` false, no `rev` labels, the `Absent` line; unreachable on this ledger, tested with a hand-built file |
| `Tier` 0 (46 live goals) | Definition shows "tier: not classified" from `Absent`; the chip row shows no tier chip, as the list has it |
| A parked goal (`answer-archive`) | Summary's ownership reads "unclaimed"; the row's `waiting` renders as the list has it; Execution shows the kept `Episode` when the record has one and "unclaimed" otherwise |
| A goal in an arc (`counselor`, arc `app-guardrail-program`) | Plan lists every member, live and archived, linked; a member that is done shows its lane chip `done` |
| A goal another goal depends on | Summary and Plan list the dependent with its state, linked |
| A `Decomposed` entry for the id | Plan shows opid, at, old arc, and the chip "split into goals"; none on this ledger, tested with a hand-built root |
| `Obligation` present | Execution shows its seventeen fields by name; none on this ledger, tested with a hand-built record |
| Tab switched | No request; the address changes; the header, chips, and ledger line stay |
| Refresh | One request; the same tab stays |
| The pane is left before the response | Aborted; no state written |
| The id changes while a response is in flight (a link to another goal) | The first request is aborted; one request for the new id |
| The tip moves between the list and the page | The page shows the record at the new tip with its new revision; nothing compares to what the list showed |
| 404 (older server), 500, unparsable, or the fetch fails | "The goal could not be read", the message, Retry; rail and header work |
| HEAD on the route | The GET's status and headers, an empty body, the GET's work; proven through `httptest.Server` (`g1-s21` F2) |

## Change boundary

New: `internal/backlog/detail.go`, `history.go`, `detail_test.go`, `history_test.go` (the package's `testmain_test.go` exists on the branch); `internal/ui/httpd/goal.go`, `goal_test.go`; `_app/src/goal/**`. Edited, and nothing else in them: `internal/ui/httpd/httpd.go` (the `Machine` field on `Info`; one prefix branch in `ServeHTTP` after the `backlogPath` branch); `cmd/metasystem/ui.go` (one closure, `Machine`, beside `Observe`); `docs/architecture.md` (the `backlog` and `ui/httpd` rows gain the detail; the `ui` family row is the branch's); `_app/src/routes.ts`, `routes.test.ts` (the `goal` kind, the nested table, the flipped assertion), `shell/Shell.tsx` (two routes), `backlog/GoalRow.tsx` (the id as a link), `cuts.test.ts` (one entry appended: `["goal/api.ts", 1, ["/api/goals/"]]`; the array then holds three, or four once `g1-s21` appends its own, and is never replaced). `httpd_test.go` may gain the route in an existing route table only. `package.json` and the lockfile do not change. Regenerated as the last commit after rebasing: `bundle/**` (D7).

Must not be touched: **every file under `internal/goal/**`**, the four `g1-s10` added included: this slice takes no D32 export, because every field it reads is exported and the one law it could have duplicated, arc membership, is a different question here (Contracts, Plan); `internal/backlog/lanes.go`, `admit.go`, `project.go` (`rowOf`'s `LastVerb` is not changed here; see Reconciliation); `internal/ui/snapshot/**`; `internal/ui/httpd/backlog.go` (its `ledgerPayload`, `writeError`, `stamp`, and `problemLines` are reused, not moved); `cmd/metasystem/goal.go`, `goal_list.go` (`g1-s16`'s); `main.go`; the plan's list at `:110`; `go.mod`, `go.sum`, `gopackages`, `stateroot`, `config`, `dispatch`, `landing`, `receipt`, `evidence`, `events`, `ui/lifecycle`, `ui/web/*.go`, `ui/workspace`, `scripts/**`, `metasystem.conf`, `metasystem.conf.local` (never read); `g1-s8`'s `vite.config.ts`, `tsconfig.json`, `scripts/*.mjs`, its policy and page rule; `_app/src/backlog/**` except `GoalRow.tsx`; `_app/src/panes/**`, `shell/**` except `Shell.tsx`, `title.ts`, `tokens.css`; any other existing `*_test.go` or `testmain_test.go`. The request path never calls `Project`, `FetchAdvance`, `FetchAdvanceBounded`, `AdvanceAccepted`, or `RepairAcceptRemote` (`g1-s10` O1 holds); `internal/backlog` and `internal/ui/httpd` import no `os/exec`; nothing in this slice opens `artifacts/`, `memory/receipts.log`, `records/counselor/`, `docs/journey.md`, or `plans/goals-drafts/`; nothing parses `Intent`, `NextStep`, or `Conclude`; nothing reads `Reason` but the one prefix test.

Conventions, repeated because the builder sees only this document: every new Go package has `testmain_test.go` with `func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }` (both packages here already have one on the branch); tests never call `os.Setenv`; every top-level test and independent subtest calls `t.Parallel()`, the ratchet allowing an unlisted package zero serial tests; no sleeps, timers, polling, or fixed ports; `httptest`, `t.TempDir()`, `testutil.Expect` and `Require`; hand-built `goal.GoalFile` and `goal.TreeGoals` values as `internal/ui/httpd/backlog_test.go:22` to `:43` builds them; `cmd/metasystem/ui.go` wires and prints only. Frontend: exact versions, `npm ci --ignore-scripts`, the Node pin (D27), Vitest in `node` with no DOM library, no colour literal outside `tokens.css`, no `style` attribute, no `setTimeout`, no dependency this design does not name (none), the bundle rebuilt last. Commands, from `metasystem/`: `go build ./...`, `go vet` and `go test` over `./internal/backlog ./internal/ui/...`, the gate's Git-fed gofmt, `bin/metasystem audit parallel-ratchet` without `--update`; from `_app/`: `npm run typecheck`, `npm test`, `npm run bundle` twice with a clean `git status --porcelain` between.

## Verification

**Go, `internal/backlog`, `detail_test.go`.** A hand-built tree with a root (`FormatVersion` 2, one `Decomposed` entry for `parent`, one `PowerOfAttorney` entry, a root History with one `ack` line targeting `waiting` and one targeting `other`) and these files: `running` (claimed by `m1e`, `StopCapability`, budget, proven approval at revision 9, one accepted `ReadItem`, nine History lines), `waiting` (queued, `Arc` `a`, `BlockedBy` `running`, ten History lines of which nine carry `reason=priority-order …` and the last is a `done` naming another goal), `parked` (parked, an `Episode` released at the park's stamp), `member` (done, `Arc` `a`, `Sliced`, a conclusion, `AcceptedRisks` one, `NormApproval`), `dropped` (abandoned with `Carried` `member`), `obliged` (claimed, an `Obligation` with every field set, a `Landing`, a `StopFence`), `torn` (queued, `Revision` 5, three History lines), `untiered` (queued, `Tier` 0, no `Risk`, no `Budget`). Assertions, one subtest per case, all `t.Parallel()`: `DetailOf` of an unknown id is `false`; `Row` equals `rowOf` of the same file field for field; `Definition` copies every field and `Risk.DerivedTier` equals `f.Risk.DerivedTier()`; `untiered` carries the tier, budget, and approval `Absent` lines and the three fixed `NotProjected` lines verbatim; `waiting`'s `BlockedBy` edge names `running` with state `claimed`, where `live`, open true; `member`'s edge from `waiting` is not open; `Plan.Members` of `waiting` is `[waiting, member]` with `member` archived and lane `done`; `Plan.Dependents` of `running` is `[waiting]`; `Plan.Decomposed` of `parent` (not in the tree) is nil but `DetailOf("parent")` is false, and a live goal listed in `Decomposed` cannot exist on a validated tree, so the entry is tested on `member` by adding it to the root; `Execution` of `running` copies the claim's six revision fields and the capability, and its job-records `NotProjected` reach is "on this node" with `Node{Machine: "m1e"}`, "on node m1e, not this one" with `"m1"`, and the not-known form with `Node{Message: "x"}`; `Execution` of `obliged` copies all seventeen obligation fields, the landing, and the fence; `Execution` of `parked` shows "unclaimed" reach and the `Episode`; `Evidence` of `member` carries the accepted risk, the norm approval on Definition, the conclusion, and an empty `Decisions`; `Evidence` of `dropped` carries `Carried` `member`; `Evidence.Decisions` of a file with `ask`, `answer`, `accept-risk`, `grant`, `breach-stop`, `resume`, `land-ready`, and `edit` lines holds the first seven in order and not the eighth; `History` of `running` has nine lines, `Revisioned` true, `Revision` 1 to 9, all class `verb`, all `NamesThis`; `History` of `waiting` has one `verb` and nine `rank` lines, and its `Root` has the one `ack` line targeting it and not the other; `History` of `torn` has `Revisioned` false, every `Revision` 0, and the `Absent` line naming 5 and 3; a JSON round trip of `Detail` asserts every field name verbatim, `keep` `-1`, and that no `omitempty` drops an empty slice or an empty string. `history_test.go`: `Classify` over a table: a `set-priority` line with `reason=priority-order subject=x …` is `rank`; a `done` line with `reason=priority-order from=2:8 to=2:7` is `rank`; a `done` line with no reason is `verb`; a line whose reason merely contains `priority-order` later in the text is `verb`; a line with `targets=a,b,x` and id `x` has `NamesThis` true and with `targets=a,b` false; the authority and channel groups are nil when every field is empty and populated field for field otherwise; `Keep` `-1` passes through.

**Go, `internal/ui/httpd`, `goal_test.go`.** From `readObservation()` (branch `backlog_test.go:45`) extended with the tree above, and a faked `Machine` returning `"m1"`: `GET /api/goals/running` is 200 with `goal.row.ref` `{goal, running, 9}`, `goal.node.machine` `m1`, `ledger` byte-equal to `/api/backlog`'s `ledger` for the same observation, and every top-level field name verbatim; `/api/goals/nope` is 404 with `{"error":"no goal nope at tip c5d517f4…"}`; `/api/goals`, `/api/goals/`, `/api/goals/running/history`, `/api/goals/../backlog` are 404 and `Observe` is not called for the last two (a counting fake); an `absent` observation answers 200 with `goal` `null` and its `ledger`; nil `Observe` is 500 with the error shape; nil `Machine` and an erroring `Machine` answer 200 with the two `node.message` forms; `Observe` runs once per request; the headers and policy are on the response; HEAD through `httptest.Server` gives the GET's status and headers, an empty body, and the counting fake shows the projection ran.

**Frontend, Vitest in `node`.** `routes.test.ts`: `routeFor({kind: "goal", id: "a/b c"})` is `/backlog/goals/a%2Fb%20c`; `routeFor({kind: "goal"})` without an id is null; `routeFor({kind: "seat", id: "x"})` is null; `activeSection("/backlog/goals/g1")` and `("/backlog/goals/g1/history")` are `backlog`; `activeSection("/backlog/other")` is null; `sectionFor` unchanged; no route starts with a reserved prefix. `tabs.test.ts`: six ids in the master's order, titles and meanings verbatim, `summary` first. `goal/api.test.ts` (with `globalThis.fetch` replaced): the id is encoded into the path; a 404 yields `notFound`; a non-2xx otherwise throws naming the status; a 200 returns the payload. `cuts.test.ts`: the allowlist holds the appended entry; every other rule stands. `empties.test.ts`, `lanes.test.ts`, `format.test.ts`, `literals.test.ts`, `contrast.test.ts` pass unchanged.

**Walkthrough**, in order, in a real browser (Playwright allowed, D24), by Claude then the human, on `ui-development` after `ui/g1-s10` has merged, from `metasystem/`: `go build ./...`; from `_app/`: `npm ci --ignore-scripts`, `npm run typecheck`, `npm test`, `npm run bundle`; `git config --get metasystem.goal.machine` recorded (the node's nickname, or nothing); `bin/metasystem ui restart`; `curl -s -H 'Accept: application/json' http://127.0.0.1:7878/api/goals/tests-parallel-and-deterministic | head -c 1200` shows `"state":"read"`, `"revision":9`, `"machine":"m1e"`, and `"revisioned":true`; `curl -sI http://127.0.0.1:7878/api/goals/nope` shows 404; `/api/goals/` and `/api/goals/x/y` 404. Open `/backlog`, the list; the id of `tests-parallel-and-deterministic` is now a link; follow it: the header with chips `claimed`, `tier 3`, `unranked`, `rev 9`; Summary: ownership "claimed by m1e (main-1789893000-16595-1a7a6a) since 2026-09-21 20:39" local, no blockers, last verb on this goal `set-budget`, no rank change, Next step the seat's prose, "Next expected transition: not recorded", the gap lines; Definition: risk 3/2/3/3, derived tier 3, gate width full, budget `2d6h / 12 / 720 / 8 / 3`, approval by `human:Wido` proven at revision 9, digest `20eab20c1bd9`, three exceptions, the three not-read lines with gate 4 and gate 5; Plan: not in an arc, no dependents, not sliced, the two not-read lines; Execution: the claim's revision 9, accounting 9, episode revision 3 at 05:36 local, stop capability generation 9 claim epoch 3, the reach line reading "on node m1e, not this one" or "on this node" as the recorded nickname decides, the three not-read lines; Evidence: the digest, one read item `accepted`, no decisions, the six not-read lines each with a home and a reach, the journey as mono text; History: "9 lines, the record is at revision 9", nine rows `rev 1` `open` to `rev 9` `set-budget`, the toggle reading "Show rank changes (0)", nothing in the root record. Then `/backlog/goals/adoption-inventory-from-install-set`: `queued`, no tier chip, Summary "last verb on this goal: open 2026-09-11…" and "last rank change: done 2026-09-11…", Definition "tier: not classified", History "10 lines", one row, the toggle "(9)", on: ten rows, the `done` rows with a `rank` chip and a `priority-order from=2:8 to=2:7` reason, a `<details>` of thirty-odd targets each a link. Then `/backlog/goals/token-spend-fence`: parked, 214 lines, toggle "(197)", the paint time of the full 214 rows recorded in the evidence note, and the response size. Then `/backlog/goals/counselor`: Plan lists the arc `app-guardrail-program`'s members. Then `/backlog/goals/account-provenance`: archived, `done`, `sliced`, rev 36, Summary's latest result the conclusion, Plan's sliced record `m0b` at revision 3, History 36 rows of which the rank rows are folded. Then `/backlog/goals/ledger-attention-clears-after-journaled-verbs`: abandoned, the reason, rev 26, and `budget-raises-preserve-elapsed-origin` for an accepted risk, `deep-battery-under-ten-minutes` for a norm approval, `stop-infrastructure-allows-the-seat-to-stop` for a budget extension with its evidence coordinate. Then `/backlog/goals/nope`: "No goal `nope` at tip …" and the link back; `/backlog/goals/`: not found; `/backlog/goals/token-spend-fence/nope`: not found; `/backlog/goals/token-spend-fence/history`: the History tab directly, the title `token-spend-fence · Backlog · MetaSystem · self-hosted`. Network panel: one `/api/goals/…` per page view, none per tab, one per Refresh, none on refocus, on an offline and online toggle, or over two idle minutes; one `/api/workspace` on load. On the human's word, `git update-ref -d refs/metasystem/goals/accepted`, Refresh: the absent statement with the fetch clause, no goal claimed absent; wait past the due time, Refresh: the goal again. Widths 1,280, 800, 599, 400: no horizontal scroll, the tabs wrap. Keyboard: Backlog, Refresh, the six tabs, then the tab's links and toggle in DOM order. VoiceOver reads the tab nav and its current tab. Light and dark; zero policy violations; no request off the origin. `go vet` and `go test` on `./internal/backlog ./internal/ui/...`; `bin/metasystem audit parallel-ratchet` without `--update`; the Git-fed gofmt.

Obligations the code critique checks by name:

- **O1, the row is the row.** `Detail.Row` is `rowOf` on the same file; the Go test asserts field-for-field equality with `Project`'s row for the same id; the page's chip row is `GoalRow`'s markup or its shared helper, not a copy.
- **O2, nothing inferred.** No code in the slice reads `Intent`, `NextStep`, or `Conclude` except to copy them; `Reason` is read only by the one `priority-order` prefix test in `Classify`; no rank, phase, transition, or link is derived from prose; `Risk.DerivedTier()` and `GateWidth()` are the engine's methods.
- **O3, the payload's names.** The JSON round-trip test asserts every field name verbatim, no `omitempty` on `Detail`, `keep` `-1`, times as strings; `ledger` is `ledgerPayload` reused.
- **O4, honest gaps in two kinds.** Every fixed `Absent` and `NotProjected` line in Contracts appears verbatim in the Go test; every `NotProjected` names a home, a reach, and a slice or gate; the reach for job records answers the three node cases; the page renders them under the two headings; no rendered text contains "no jobs", "no evidence", "nothing happened", or "no history".
- **O5, revisions are the validator's.** `Revision` labels appear only when `len(History) == Revision`; the `torn` fixture carries none and its `Absent` line; the numbers on the claimed goal read 1 to 9.
- **O6, rank shifts fold, never vanish.** Every History line reaches the page; `rank` lines are hidden by a toggle that shows their count; `adoption-inventory-from-install-set` shows one row and "(9)"; the Summary's "last verb on this goal" is the last `verb`-class line.
- **O7, one request per page.** `cuts.test.ts` passes with the appended entry; `useGoal` refetches on id change and Refresh only; the browser shows no request on a tab change.
- **O8, the route.** Prefix match after the two exact routes; `/api/goals`, `/api/goals/`, an id with a slash, and `..` are 404 without a lookup; unknown id 404 with `goal show`'s words; non-`read` states 200 with `goal` null; nil `Observe` 500; `Observe` once per request; HEAD proven through `httptest.Server` with no route branch.
- **O9, the node is optional.** `Info.Machine` nil or erroring answers 200 with `node.message`; `ResolveMachine` is called through the closure `ui.go` wires and nowhere else; no git subprocess is run by `internal/backlog` or `internal/ui/httpd`.
- **O10, the resolver and the links.** `routeFor({kind: "goal", id})` encodes the id; `activeSection` answers `backlog` beneath `/backlog/goals/`; the list's ids are `NavLink`s to the same path; blockers, dependents, members, and targets link through `routeFor` and render as text when it answers null; `routes.test.ts` passes with the flipped assertion.
- **O11, additive only.** `git diff --stat` on `internal/goal` is empty; on `internal/backlog` shows only the four new files; on `internal/ui/httpd` shows `goal.go`, `goal_test.go`, and the two edits to `httpd.go`; on `internal/ui/snapshot` is empty; `package.json` and `go.mod` unchanged.
- **O12, layout.** The tabs are `NavLink`s with `aria-current`; the numbers under The page hold in both themes; nothing is truncated; the 214-row History paints, and the time is recorded.
- **O13, conventions.** `testmain_test.go`, `t.Parallel()` everywhere, no serial test, no timer, no `os.Setenv`; `ui.go` wires only.

## Reconciliation with `g1-s10` and `g1-s21`

1. **`g1-s10` is built, not merged.** The plan's row (`:194`) says `designed`; branch `ui/g1-s10` at `46dab9a5d` carries the whole slice, Go and list. This design cites the branch and assumes it merges first; the builder rebases `ui/g1-s11` onto that merge. If the planner merges this slice first instead, `Detail.Row` has no `rowOf` to reuse and the change boundary is wrong, so that order is refused here.
2. **`routeFor({kind: "goal"})`.** `g1-s10` revision 4 states the goal route is this slice's (`:233`, `:349`) and its O8 asserts `routeFor({kind:"goal"})` is null and no row links; both flip when this slice lands. The branch's `routes.ts` is untouched (its diff against `ui-development` is empty), so the only edits are this slice's. The planner amends `g1-s10`'s O8 on that row as it amended `g1-s9`'s O14.
3. **`activeSection`.** `g1-s21` changes it for `/project/doc/*` and `routes.test.ts:73`; this slice changes it for `/backlog/goals/*` and `:76`. Both become one table of nested prefixes; whichever lands second adds its pair and its test line, and neither replaces the other's.
4. **`cuts.test.ts`.** The branch's `CALL_SITES` holds two entries; this slice appends `["goal/api.ts", 1, ["/api/goals/"]]`; `g1-s21` appends `["project/api.ts", 1, ["/api/project", "/api/documents/"]]`; the final array holds four, and whichever lands second appends to the array it finds.
5. **The chain in `ServeHTTP`.** Two exact routes, then this prefix route, then `g1-s21`'s exact `/api/project` and prefix `/api/documents/`; order among the `/api/…` branches does not matter, since no path matches two of them, and no mux is introduced by anyone.
6. **The identity triple.** `{"kind":"goal","id":<id>,"revision":<integer>}` as `g1-s10` fixed it (`:346`); the `document` kind's `blob:` revision is `g1-s21`'s; the journey reference this slice emits is a `document` reference and switches on `kind` before reading `revision`, as `g1-s10` said a consumer must.
7. **`LastVerb` on the list.** Found here, not settled here: `rowOf` takes the last History line whatever its class (branch `project.go:261` to `:264`), so 47 live rows will read "last done <date>" on goals never concluded. This slice keeps the row's field as it is (O1 needs the row unchanged) and adds the two class-aware facts on `Detail`. The planner rules whether `g1-s10`'s `rowOf` takes the last `verb`-class line instead, which is a one-line change in the branch plus a fixture, and the recommendation is yes, before the human reads "last done" on a queued goal; `Classify` would then move to `project.go`'s side of the package, which this design allows.
8. **The three prose fields and `g1-s21`'s Markdown tree.** Its reconciliation item (4) recommends the goal view carry Intent as the tree; this design carries source text at gate 1 and adds `intentBlocks` beside it only after `g1-s21` lands, as a pane change with no field removed. Stated so the API's shape does not move under the brain's tools later.
9. **`g1-s6`'s reach.** The plan's `g1-s6` row (`:190`) adds exported listers for this node's jobs; when it lands, Execution's job-records `NotProjected` row becomes a projected list for the case `reach == "on this node"` and stays a `NotProjected` row for another node; the payload gains a field then, and loses none.

## Open questions

1. **Who owns the History classification, the row or the detail.** Options: (a) as written, `Classify` in `history.go` used by `DetailOf`, `rowOf` untouched, the list keeps "last done" on 47 rows until the planner rules on item 7; (b) `rowOf` adopts `Classify` for `LastVerb` now, inside this slice, which edits `g1-s10`'s `project.go` on its branch and one fixture, and O1 still holds because both readers share the change. Recommendation: (b), in the same slice, because the false "last done" reaches the human's screen on the first list they open and the fix is one line; the cost is one edit to a file this design otherwise fences.
2. **Task receipts as gate 1 evidence.** `memory/receipts.log` has 227 lines naming a goal, with type, outcome, verify, built-by, and a note: the one durable per-task account that exists today. Reading it at the accepted tip needs a git read of one blob outside the goal subtree, which is a D32 export in `internal/goal` on M5's reasoning (the interface must not run its own git), and a line reader, which does not exist in `internal/receipt` and would be a second owner of the receipt grammar if written here, so it would be an exported reader in a new file there, the `g1-s5`/`g1-s6` precedent. Options: (a) defer, as this design does, naming the log as a `NotProjected` row; (b) a follow-on slice `g1-s11b`, two additive exports, one route field; (c) fold into gate 6's evidence joins. Recommendation: (a) now and (b) soon after, because it is the cheapest real evidence there is and needs no node access; but not in this slice, which already crosses two packages and the resolver.
3. **`Info.Machine` and the nil-is-500 rule.** As written, a nil or erroring machine resolver answers 200 with `node.message`, because the goal is answerable without it. The alternative is to fold the nickname into `snapshot.Observation` on the `g1-s10` branch, one git read per observation beside `ResolveEndpoint`, which keeps `Info` at two closures and the rule intact but edits `snapshot.go`. Recommendation: as written; the rule is about a route's primary answer, and the reach line degrading to "not known" is the honest behaviour on a machine with no nickname.
4. **Whether Evidence should show `Decisions` at all before `g1-s12`.** The seven verbs are recorded on the goal and are decision-shaped, but the Decisions slice owns questions and rulings as records of their own, and a `question=` on an `answer` line names a record this build cannot open. Options: (a) as written, the lines shown with `question` as text; (b) omit `Decisions` until `g1-s12` can link them. Recommendation: (a); the lines are the goal's own record, and an unlinked identifier is text, which is the resolver's rule everywhere else.

## Planner's rulings, 2026-09-22

Recorded before Sol's critique returns; a finding there may reopen any of them.

**Open question 1 is settled by fact, and more widely than the design proposed.** The
`LastVerb` defect was fixed on `ui-development` itself at `4b4afc5df`, not inside this
slice: `internal/backlog/project.go` gained `lastVerbOn`, which walks back past history
lines whose `Reason` begins `priority-order`. The first browser walkthrough of the merged
list confirmed the design's count exactly — 47 of 155 live rows read "last done" while
queued or parked before the fix and 0 after — and the fix is mutation-checked both ways.
So this slice inherits a correct `rowOf` and fences `project.go` as it wished. The
classification belongs to `internal/backlog` rather than to the detail, because the list
and the page must not disagree about a record, which is the same reason `Detail.Row` is
`rowOf`.

**The Revision-to-History premise is true but is not an invariant, and the design is
right to defend against it.** Verified on this ledger: all 155 live records under
`plans/goals/` and all 430 archived under `records/goals/` have `Revision` exactly equal
to their History line count, 585 of 585, zero mismatches. But the validator does not
enforce it. `internal/goal/file.go:611` requires only that History be non-empty, and the
revision checks at `:835`, `:841`, `:916` and `:997` are one-directional — a *referenced*
revision may not exceed `len(History)`. A record whose `Revision` exceeded its History
would validate, and one whose History exceeded its `Revision` would validate too. The
equality is maintained by the writers, one appended line per bump, not by a rule.

So the design's fallback — a record where they differ gets no revision labels and an
`Absent` line — is load-bearing rather than defensive decoration, and the builder must
not simplify it away on the grounds that the ledger satisfies the premise today. The
labelling is a convenience derived from a convention; the History itself is the record.

## Planner's rulings on Sol's critique, 2026-09-22

The critique is at [g1-s11-goal-detail-design-sol-review.md](g1-s11-goal-detail-design-sol-review.md).
Verdict: four design errors to fix before building, two builder obligations, five
false alarms. The slice stays `designed`; it does not go to a builder until a revision 2
folds these in.

**Finding 4 is already half-settled, and it settled it against me.** Sol reached
independently the conclusion the code review reached about `lastVerbOn`: a `HasPrefix`
test on `priority-order` is not a sound classifier, because an abandonment copies the
human's reason verbatim and a human may write one that begins with those words. On
`ui-development` at `67de893fc` the row no longer skips such a line; it keeps the line's
date, which is exact, and withholds the verb, which it cannot attribute. So the residual
harm of the prefix test is now a withheld verb rather than a misattributed one — a
degradation, not a lie — and the detail page shows the whole History either way. Open
question 1 is closed. The design's own `Classify` must adopt the same posture: it may
label a line's class as unknown, and must never present a rank clause as this goal's verb.

**Findings 1, 2 and 3 stand and are the revision's work.** The gap taxonomy contradicts
itself where `NotProjected` is used for things that are simply absent; "no claim, nothing
is executing" is unsupported, because release clears a claim without consulting job
activity and the engine's own activity reader reports live jobs on an unclaimed goal; and
the journey link hardcodes `metasystem/docs/journey.md`, which is wrong in an adopted
installation where state lives beneath the application repository. The third is the kind
of error this interface exists to avoid: a link that is confidently wrong about where a
record lives is worse than a link that is absent.

**Finding 6 corrects me, not just the design.** My ruling on open question 2 accepted the
design's premise that reading `memory/receipts.log` at the accepted tip would need a new
`internal/goal` export under D32. It would not: production code already reads that blob at
the tip through the exported `gittree.Workspace.FileAt`, and parses receipts and
corrections by goal, type and outcome. So the follow-on is cheaper than either of us
thought, and D32 is not engaged. That is worth stating plainly, because D32 exists to keep
exports rare and I nearly spent one on a path that was already open.

**The false alarms confirm two things worth keeping.** No field the six tabs need requires
an `internal/goal` export, so the slice really does take none. And the Revision-to-History
premise is not enforced but the design's divergence handling is adequate — which is what
the ruling above already said, now checked by someone else against the same census.
