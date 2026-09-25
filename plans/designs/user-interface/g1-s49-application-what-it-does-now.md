# g1-s49: Application — what it does now

- Kind: design
- Id: 01M3CPKG2AMTDPEB7P206QZBBD
- Status: accepted
- Goals: browser-interface

Wido, 2026-09-25: "do a full UX design (come from human wants, needs,
musts etc, jobs to be done) for the application page. then implement the
first version, smallest thing that works so I can look at it". Author
Fable. Step 1 of the Application section; every cite re-read at
`6d9529d1d`; the counts read from this checkout's ledger the same day.

## 1. What exists and binds

1. **The section is a placeholder.** `routes.ts` lists `application` at
   `/application`, icon Package, sentence "What has been released, with
   the evidence for each claim and where that evidence was produced",
   `projected: false` (src/routes.ts:105-112); no pane exists. The
   master's row: "Implemented and released behavior, observations, and
   links to supporting evidence — What does the application actually do?"
   (user-interface-design.md:36); Overview's slice reserved "examination
   findings and production problems as entries, when the Application
   section exists" (g1-s27:51).
2. **The subject and the machinery never blur.** The workspace is one
   checkout with one subject: the MetaSystem building itself, or an
   adopted application; the header says "MetaSystem · self-hosted" or the
   application's name "built with MetaSystem"; in the self-hosted case
   Application shows the MetaSystem as the product under development while
   Fleet and Settings show the instance doing the work, and "the interface
   shows the running engine build beside the source at HEAD, and says
   when they differ" (user-interface-design.md, Workspace identity;
   internal/ui/workspace/workspace.go:26-98, whose comment at 83 says the
   source at HEAD arrives with the slice that reads the checkout).
3. **The truthful account of what landed is the ledger's.** Every one of
   the 429 done goals carries a conclusion, one sentence written when it
   concluded, and a `doneAt` (internal/backlog/project.go:90, 118-122,
   259); the backlog payload's `closed` rows carry `intent`, `concluded`,
   `doneAt`, `labels`, `arc` (src/backlog/api.ts:98-146). On this ledger:
   320 concluded this month, 104 last month, 5 with no date; 49 carry the
   label `robustness`, most carry none; arcs are rare. Most conclusions name what landed, the commit and the machine; some
   record an administrative end, "Obsolete: self-declared duplicate
   holding no work" (records/goals/human-carried-landing-verb.md:11) or a
   requirement absorbed into another goal, and `doneAt` dates the
   conclusion, not a deployment or a capability that still stands.
4. **The record of what is wrong is a register.** `memory/known-issues.md`
   is a table, one row per defect or limitation, 44 rows here: `| Id |
   Date | Symptom and evidence | Cost when it bites | Fix direction or
   lever | Status |`, the status column prose beginning FIXED, ACCEPTED, RETIRED, RESOLVED, OPEN
   or a date, five OPEN rows carrying only three or four cells
   (known-issues.md:27, 42) and one cell holding an escaped pipe (line 26), and an open question (Q-…KGS7) records that
   adoption ships a second column set, `| Id | Date | Issue | Consequence
   | Reopen when | Status |` (scripts/adopt.sh:310); both are six columns
   in the same order of meaning. No Go reader exists for the page's use
   (internal/mission/guardrails.go and internal/audit read it for their
   own purposes). The flake registry (docs/flake-registry.md) is a
   protocol with its own register; deferred.
5. **The engine build is a fact this seat publishes.** The presence record
   carries `engine` and `generation` (internal/seat/record.go:64), shown
   on Fleet as "5b9d958 · generation 4" (seat/fleet.go:165); this seat's
   own record is read locally by the Fleet page.
6. **New since your last visit is per page** since g1-s48:
   `overview.VisitPage(root, page, human, now)` gives a page its own entry
   with the thirty-minute rule and the first-visit day.
7. **The idioms.** The Project pane's blocks and document links, the
   Decisions inbox's collapsed groups and one-line rows with an open row,
   Find and label chips (src/decisions/groups.ts, Inbox.tsx, InboxRow.tsx,
   QueueBlock.tsx), the Tabs component, the help register.

## 2. The human, and the jobs

The human is the one who owns the application: they direct the agents
that build it, they answer for it to whoever uses it, and they are the
release authority. What they want from this page is to be able to say,
without opening a terminal, what the application does today. What they
need is that every sentence on it is a recorded fact, because the page
will be quoted. What it must do: tell the subject from the machinery, so
that in the self-hosted case nobody mistakes the tool doing the work for
the product being built; say when a thing is not known rather than guess;
link every claim to the record it comes from.

Jobs, in the order of a visit:

- **J1. What does the application do today?** In product terms, not
  ledger terms, current not planned.
- **J2. What has changed since I last looked?** Landed work, with the
  words of its conclusion, and how much of it.
- **J3. Is it working? What is wrong with it?** The known problems, open
  ones first, with what each costs and what would fix it.
- **J4. Show me the proof.** For any claim, where the evidence is.
- **J5. What is it, and how do I read about it?** The application's own
  description and documents.
- **J6. Which application is this?** The subject and the mode, always.

What the kit can truthfully answer today: J2 and J6 in full; J1 only
as the record of concluded work, because the engine keeps no record of
implemented behaviour as such, and a conclusion dates an end, not a
capability that still stands, so the page names it work history and
says that step 1 does not establish current capabilities; J3 from the
register; J4
as the goal's own record and history; J5 as links to the documents the
Project pane already reads. Step 1 answers those. Examination findings,
production observations, evidence bundles and a capability map are what
would answer the rest, and none of them exists as a record yet.

Decisions:

- D1. **The header names the subject and the mode**, as the shell does,
  and in the self-hosted case says it in words: "The MetaSystem as the
  product under development; the instance doing the work is on Fleet".
  Beside it, "last published MetaSystem engine 5b9d958 · generation 4"
  from this seat's own presence record, which is a record at one tick
  and may predate what runs now, or "no engine build available"; the
  source at HEAD and whether they differ is step 2, named.
- D2. **"What concluded" is the answer to J2, and the only answer step 1
  has for J1.** The done goals with a conclusion, newest first, grouped by week under a line that says the
  week and its count; the latest four weeks open, the rest behind "Show
  earlier"; a row is one line: the intent, then the conclusion's first
  sentence muted, the date at the right, and a dot where it landed since
  the last visit here; an open row shows the whole conclusion, the
  labels, the arc, and "Open the goal", which is where its history and
  proof are read (J4). Find over id, intent and conclusion; label chips
  from the rows shown. The header's count line: "429 goals concluded · 320 this month · 7
  since your last visit"; the block's help term says this is the record
  of concluded work, that a conclusion may record an administrative end,
  and that the page does not establish what the application can do now.
- D3. **"Known problems" is the answer to J3.** A reader for the register,
  in the manner of the rulings reader: it takes the header row as the
  names of its six columns, kept as supplied because the fifth differs
  between the two sets, and reads either set by position; a row whose status begins with FIXED, RESOLVED, RETIRED, CLOSED or
  ACCEPTED is concluded, with the status word kept visible because an
  accepted limitation still exists; any other is open; cells are split
  on unescaped pipes only; a row with the wrong column count is listed
  in one line, "n issues could not be read as rows", with "Open the
  register" through the Project reader, because five of this register's
  open rows are such rows today. The block lists open rows first, one
  line each: the id, the symptom's first sentence, the date; an open row
  shows the cost, the lever and the status whole. Concluded rows behind
  "Show concluded (n)".
- D4. **"What it is" is the answer to J5**, one line of links: the
  application's README, and the concepts and glossary documents where the
  layout has them, opened in the Project pane's document reader; nothing
  is rendered here.
- D5. **New is by the page's own visit entry**, as Decisions keeps it: a
  goal is new when its `doneAt` is after the window's start, an undated
  one never; the help term says so.
- D6. **Nothing here acts.** The page reads; the goal page acts.
- D7. **Phone width**: the header stacks, the groups and rows stack.

## 3. The payload: `GET /api/application`, schema 1

```
{ "schemaVersion": 1, "readAt": "…",
  "subject": "MetaSystem", "mode": "self-hosted",
  "engine": { "build": "5b9d958", "generation": 4, "publishedAt": "…" } | null,
  "counts": { "landed": 429, "thisMonth": 320, "new": 7 },
  "landed": [ { "id": "…", "intent": "…", "concluded": "…", "doneAt": "…",
                "labels": [], "arc": "", "new": true } ],
  "problems": { "columns": ["Id","Date","Symptom and evidence",…], "open": [ {row} ], "concluded": [ {row} ],
                "unread": 5, "defects": [ "row=12: wrong column count: got 5, want 6" ] },
  "docs": [ { "title": "README", "path": "README.md" }, … ],
  "visit": { "since": "…", "first": false } }
```

`landed` is every closed row in lane done, newest `doneAt` first, the
undated last; `problems` rows carry `id, date, what, consequence, lever,
status, open`. Composed in a new `internal/ui/application` from the
board observation the other pages share, the register read per request
like the rulings register, this seat's presence record as Fleet reads it,
and the visit window. The route follows the read routes' policy.

## 4. The page

`ApplicationPane` at `/application`, `projected: true`, the route's
sentence rewritten to what it shows. Header, then three blocks in the order of the jobs: What concluded,
Known problems, What it is. Rows and open
rows in the inbox's idiom; Find and chips as the queue's. Help terms:
`application-section` rewritten, `what-concluded`, `known-problems`,
`concluded-new`, `last-engine`. The Partner's page capture carries the block open, the
narrowing and the row open. The page reads on mount and Refresh; no
timer; one EventSource.

## 5. Not here, step 2 and later

The source at HEAD beside the engine build, and the words when they
differ. Evidence bundles from the evidence root and receipts beside a
landed goal. Examination findings and production observations as records
of their own. The flake registry. A capability map, or the intent and
design documents summarised as "what it does". Releases, versions and
tags, which the kit does not record. The documents rendered here rather
than linked.

## 6. Verification and box

Go: the known-issues reader over a fixture with both column sets, an
open and a concluded row of each status word, an escaped pipe, a
three-cell OPEN row counted as unread; the
composition's ordering, week grouping input, counts and `new` against an
injected window; the route. Frontend: the header for both modes and for
no engine build; the weeks with "Show earlier"; a row opening; Find and a
chip; the problems block with open first and "Show concluded"; the links;
the guards stay green. The walkthrough fixture gains thirty concluded
goals across six weeks, a register with both column sets, and a visit
window; screenshots at 1280 and 400: the page at rest, a landed row open,
a problem open, phone. Budgets as always. Box: one build lane (Claude on
Opus), one code read (Codex on Sol) with one fix round under R-124, after
Astra's read; two attempts, 120 to 180 job-minutes.

## 7. Self-grade

High on D2, D5, D6: the rows are the ledger's own words through the
board observation every page shares, named as the work history they are. Medium on D3: a new reader over a
prose register with two known shapes; the defects line is its honesty.
Medium on D1: the engine build is this seat's last published record
and may be absent or older than what runs. Weakest: J1 answered as a changelog rather than a capability map,
because the kit records no such map; the page says so in its help term,
and the first sitting will show whether the conclusions read as "what it
does" or only as "what happened".

## Dispositions (Astra read, 2026-09-25, under R-124)

Four material findings, three deferred; every code claim checked.

| id | finding | fold |
|---|---|---|
| F1 | a done goal's conclusion records an end, sometimes an administrative one, and dates the conclusion, so "429 landed" and "what it does" overstate | the block is "What concluded", the count "goals concluded", the help term says it is work history and does not establish current capabilities |
| F2 | RETIRED and RESOLVED statuses exist and would read as open | both concluded; the status word stays visible; no prose interpreter |
| F3 | five OPEN rows have three or four cells and one cell holds an escaped pipe; the copied reader would skip them | unescaped pipes only; the unread count in one line with "Open the register" |
| F4 | presence is a record at one tick; an unarmed seat shows an old build as the engine it runs | "last published MetaSystem engine", with its tick, or "no engine build available" |

Deferred, step 1 works without them: four expanded weeks may push Known
problems far down, to be observed in a sitting; the fifth column's title
differs between the two sets and is shown as supplied; the register holds
44 rows, not 46.
