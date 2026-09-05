Working Mode: implement
Orchestrator Identity: m1 (lineage main-1788594343-3833-fb64b9, dispatch delegate under goal channel-local-timestamps, approved 2026-09-05 under Wido's relayed word, box 1d/10/240m/1, review rounds 2)
Date: 2026-09-05

# Goal

The timestamps a human reads in the fleet channel are local time, not UTC.

Wido's words, 2026-09-05: the Telegram messages use a non-local timestamp,
and he wants actual local timestamps. Tier 2, MECHANICAL: build plus one
code review, no design round.

# The defect, against the code

metasystem/internal/channel/report.go renders the status headline as
"<machine> status 2006-01-02 15:04Z" from c.Now.UTC(), and
metasystem/internal/channel/report.go and
metasystem/internal/channel/question.go both set Now from time.Now().UTC().
So every message Wido reads is stamped in a timezone he is not in.

# The line that binds

Display is not record. Change what a human reads; do not change what
machines compare.

- DISPLAY, and in scope: the status headline in
  metasystem/internal/channel/report.go, and any other time a human reads in
  a posted message.
- RECORD, and out of scope: the inbox fields SentAt and ReceivedAt in
  metasystem/internal/channel/inbox.go, which other machines read and
  compare, and the git --since argument in
  metasystem/internal/channel/report.go, which git parses. These stay UTC.
  If you find another time value that is compared, sorted, deduplicated or
  handed to another program, it is a record: leave it and say so in your
  return.

# What to build

Render the human-facing timestamps in the local timezone of the machine
posting the message, and make the offset unambiguous to the reader rather
than dropping the Z and leaving a bare number that could be any zone. The
machine's own zone is the honest source for a per-machine status line.

Keep the rendering testable: a test must be able to fix the zone rather
than depending on where the suite happens to run, so take the location
from the config the renderer already carries rather than reading the
process environment deep inside a formatting helper.

# Tests

In metasystem/internal/channel/channel_test.go:

- A status report rendered with a fixed non-UTC location shows that
  location's wall-clock time and its offset, not the UTC time.
- The inbox record fields and the git --since argument are unchanged by
  this work: they remain UTC and parse as they did.
- The existing report tests keep passing with whatever zone the suite runs
  in, so pin the location in the test rather than assuming the runner's.

# Boundary

Touch metasystem/internal/channel/report.go,
metasystem/internal/channel/question.go if its Now needs the same treatment,
and metasystem/internal/channel/channel_test.go. Declare every touched path
in diffBoundary with the metasystem/ prefix. Do not change the ask's
rendering bound: that is a separate goal (channel-ask-fits-one-message)
landing in parallel, and folding the two together makes both unreviewable.

Report `go test ./internal/channel/...` green in your return. Any deviation
you find necessary is a GAP to report, never a silent choice.
