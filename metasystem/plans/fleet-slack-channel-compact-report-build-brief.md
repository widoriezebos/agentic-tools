Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fleet-slack-channel)
Date: 2026-09-03

# Goal

Reshape the per-machine status report the fleet channel posts, in
metasystem/internal/channel/report.go (`ComposeReport` and its helpers),
to the compact form Wido approved after the first live Telegram post on
2026-09-03. His finding, verbatim: "I got a wall of text. For a test that
is ok; for something I can easily read; not really." The shape below is
his word ("much better yes"); it is the law of this change. Nothing else
in the channel changes: posting, cadence, digest, question threads,
`ShouldPost`, `StatusState` and every adapter stay byte-identical.

# The shape (exact)

```
m0b · 03 Sep 14:45Z
Landed today: fleet slack channel (10 landings)
Under way:    fleet slack channel — slice 2 landed, awaiting live proof
Planned next: breach clock and budget honesty; boundary declaration repair; +94 queued
Spend today:  16 jobs · claude 47k · codex 66M units
```

Rules, each one a test:

1. Header: `<machine> · <DD Mon HH:MM>Z` (Go layout `02 Jan 15:04`), UTC.
2. A goal is named by its id with hyphens replaced by spaces and NOTHING
   else: never the intent, never a first sentence of it. The
   `landingFeatures` map value becomes the bare name; `firstSentence` is
   no longer applied to `Intent` anywhere in this file.
3. `Landed today:` one line, the goals this machine claimed that landed
   in the window, comma-separated `name (N landings)`, sorted by name; at
   most 3 names, then `; +K more`. The window stays `WindowStart` (default
   now minus 4 h); the label is `Landed today:` regardless. Line omitted
   when nothing landed. The landing subject (commit subject) is dropped.
4. `Under way:` one line per claimed goal, at most 3 lines, then
   `; +K more` on the last. Content: `name — <first sentence of NextStep,
   cut at 80 runes>`; append `; job <role> running <N> min` when a job is
   running (unchanged source, `runningJobs`). Lines sorted by name.
5. `Planned next:` ONE line: the first 3 queued goals eligible for this
   machine (today's `Pinned` rule), by name, `; `-separated, then `; +K
   queued` when more exist. Readiness ("needs budget", "blocked by …") is
   no longer shown. Ordering: goals with a budget before goals without,
   then by id. Line omitted when nothing is queued.
6. `Spend today:` `N jobs · <runtime> <units> · …`, runtimes in today's
   order, units compact: below 1000 as is, thousands as `47k`, millions
   as `66M` (one decimal only when the leading digit is a single digit:
   `1.2M`, `9.8k`; `47k`, `66M` otherwise). The source of the numbers is
   unchanged (`spendLine` keeps its data path, changes its formatting).
7. `Undelivered:` line unchanged in content, kept last.
8. Labels are padded so the values align (`Landed today: `, `Under way:
   `, `Planned next: `, `Spend today:  `; four spaces after
   `Under way:` and two after `Spend today:` as shown). The total text
   must not exceed 1200 runes; the existing 3500-byte guard stays as a
   second guard.

# Tests (a new report test file beside metasystem/internal/channel/channel_test.go, which holds today's ComposeReport tests; add, remove none)

- TestReportNamesGoalsNeverIntents: a goal with a long intent yields its
  name only; the intent text is absent from the report.
- TestReportPlannedIsOneLineWithCount: 96 queued goals → one
  `Planned next:` line with 3 names and `+93 queued`.
- TestReportUnderWayCutsNextStepAt80Runes: a 300-rune next step is cut
  at 80 runes; multi-byte runes are not split.
- TestReportLandedTodayCountsPerGoal: two goals with 10 and 1 landings
  render `name (10 landings), name (1 landing)` (singular for 1).
- TestReportSpendUnitsCompact: 47125 → `47k`, 66115426 → `66M`, 950 →
  `950`, 1234567 → `1.2M`.
- TestReportHeaderFormat: `m0b · 03 Sep 14:45Z` for the given Now.
- TestReportStaysUnder1200Runes: 96 queued, 12 claimed, 20 landed goals
  → total under 1200 runes.
- Existing tests: keep them green; where one asserted the old shape, its
  assertion follows the new shape and its name and intent stay.
- The channel fixture (metasystem/scripts/agents/channel-fixtures.sh)
  must still pass; if it greps the old status lines, update the grep to
  the new shape and nothing else in it.

# Gate

gofmt, go vet, go build; `GOFLAGS=-buildvcs=false go run
honnef.co/go/tools/cmd/staticcheck@2025.1 ./...` silent; go test
-count=1 over metasystem/internal/channel/... and the metasystem command package green; one run of
channel-fixtures.sh in your sandbox. No benchmarks (R-31), no sleeps
(R-35). Leave the work in your working tree, stage nothing, do not run
the commit wrapper. diffBoundary: metasystem/internal/channel/report.go, the new report
test file in that package, metasystem/internal/channel/channel_test.go
(assertions on the old shape only) and
metasystem/scripts/agents/channel-fixtures.sh. Paste the final gate
lines and one rendered sample report in your return.

# Constraints

Wall-clock budget: 45 minutes. Version-2 implementer JSON.

# Gap Rule

stop and report a gap; never fill it silently.
