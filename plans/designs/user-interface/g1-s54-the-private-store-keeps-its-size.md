# g1-s54: the private store keeps its size

- Kind: design
- Id: 01M3EHSKGNBHDJEW5HM2PQVYMW
- Status: done
- Goals: browser-interface

Wido, 2026-09-26, on the conversation store's move to the account's
home: "what about automatic cleanup? I do not want this to grow and grow
and grow until it fills up the file system." Author Fable. Every cite
re-read at `1be2696e9`.

## 1. What exists and binds

1. **One owner, keyed per workspace and per human.** `uihome.Under(home,
   owner, checkout)` is `<registry home>/ui/<owner>/<name>-<digest of the
   resolved checkout path>` (internal/ui/uihome/uihome.go:60-90); inside
   it the Partner keeps `<human>.json`, `<human>.jsonl` and `wire.jsonl`
   (internal/ui/partner/conversation.go), the stickies owner its
   `stickies.json`. Two checkouts are two directories; two humans are two
   files; the registry's environment seam moves them all.
2. **Nothing bounds the transcript within a runtime, nor the wire
   journal within a runtime.** The transcript is loaded into memory and
   appended under the conversation's mutex, the file opened and closed
   per append (internal/ui/partner/conversation.go:431-505); the wire
   journal is opened once per runtime process with truncation and held
   as the connection's writer until that process ends (host.go:1147;
   internal/acp/conn.go:62), so it bounds itself per runtime but a long
   runtime grows it without limit. On this host after three days the
   journal is 121 KB and one conversation 49 KB. The stickies store
   is bounded at 500 per human (g1-s50).
3. **What is memory and what is not.** The paper: records are the only
   memory that survives; the sitting has none of its own (docs/paper/15-the-sitting.md).
   The master: raw transcripts and streamed tool detail live in a
   protected server-local store, apart from disposable execution
   artifacts (user-interface-design.md:381); a fresh session is given
   the last twenty messages within an eight-kilobyte block, not the
   file (conversation.go:616). Neither text calls the transcript
   disposable; both make the records the memory that must survive.

## 2. Decisions

- D1. **Two bounds, each by its owner,** requested by a housekeeping
  loop the interface runs at start and once a day while it serves, in
  the channel-driven pattern the fleet fetch loop uses with a synthetic
  tick in tests (internal/ui/fleet/fetch.go:373):
  - the wire journal rotates at 8 MB through its own writer: the
    Partner's journal writer closes, renames the current file to
    `wire.1.jsonl` and reopens a fresh sink between two writes, keeping
    one previous; housekeeping asks the writer, never renames the path
    under it;
  - a conversation is trimmed from its head when it passes 2 MB or its
    oldest message is older than 90 days, keeping at least the last 200
    messages: an operation of `Conversation` itself, under the mutex
    `Append` takes, replacing the file and the cached messages together,
    cutting only between distinct `Turn` values and keeping an unfinished
    trailing turn whole; what is cut is gone, and the first remaining
    message says "earlier messages were trimmed on <date>".
    A workspace directory is never removed: modification times prove
  neither disuse nor a deleted checkout, and the notepad in it is the
  human's durable reminders (g1-s50). Detecting a deleted checkout is a
  later decision of its own.
- D2. **Never the records.** Housekeeping touches only the private store
  under the account's home; nothing in a checkout, nothing under the
  state root.
- D3. **Said where the human reads.** Settings shows the private store's
  path, its size per workspace and the bounds in words; the Partner's
  help term says transcripts are trimmed and records are the memory.
- D4. **Numbers are configuration with defaults**, `ui.store.wire-mb`,
  `ui.store.conversation-mb` and `ui.store.conversation-days` in
  `metasystem.conf`, read through `config.Get` with its precedence as the
  other interface keys are (internal/config/ui.go:95); zero disables that
  bound. They are retention targets, not disk ceilings: two hundred kept
  messages can exceed two megabytes, and a day passes between sweeps.

## 3. Not here, later

Removing the private store of a checkout that no longer exists.
Archiving trimmed transcripts elsewhere. Per-sitting export. Bounds on
the checkout's own artifacts, which are the engine's. Aggregate quotas
across workspaces.

## 4. Verification and box

Go: the writer's rotation while a connection writes, the previous kept
and the older removed; the conversation's trim over an oversized and an
old transcript with an append racing it, the cut falling between turns
and an unfinished trailing turn kept, the cached messages matching the
file; each bound crossed and not crossed; zero disabling a bound;
nothing outside the home touched; the daily loop driven by a synthetic
tick in tests. Frontend: the Settings lines. Budgets as
always. Box: one build lane (Claude on Opus), one code read (Codex on
Sol) with one fix round under R-124, after Astra's read; two attempts,
60 to 120 job-minutes.

## 5. Self-grade

High: two bounds, each in the hands of the writer that owns the file.
Medium on the conversation trim: it shares the append's mutex and the
cut falls between turns. Weakest: the numbers
are guesses until a month of use says otherwise.

## Dispositions (Astra read, 2026-09-26, under R-124)

Three material findings, three deferred; every code claim checked.

| id | finding | fold |
|---|---|---|
| F1 | removing a workspace directory on modification time would erase a quiet checkout's stickies and transcript; reading a notepad refreshes nothing | dropped; no directory is removed; a deleted checkout is a later decision |
| F2 | a read-trim-replace outside the conversation can race an accepted append, and a disk-only trim leaves the served transcript stale; a trailing turn can be unfinished | the trim is the conversation's own operation under the append's mutex, replacing file and cache together, cutting between turns |
| F3 | renaming the wire journal under the runtime's open descriptor leaves it writing the renamed file; the journal is truncated at runtime spawn | rotation by the journal writer, close, rename, reopen; the fact corrected |

Folded because they cost nothing: the master and the paper do not call
transcripts disposable, and the design no longer says so; the numbers
are retention targets; the daily loop follows the fleet fetch loop's
synthetic-tick pattern; the workspace key hashes the cleaned absolute
path without resolving links.

## Built (2026-09-26)

Landed on ui-development and main; the bounds went live before the read
and the read's fixes landed as a second merge. Built by Claude on Opus,
read by Codex on Sol under R-124: three material findings, fixed in one
round. The first sweep ran before the server held the checkout's
ownership, so a second server attempt could trim a transcript the first
was appending to; every sweep now waits on the gate that Ready opens.
Ownership was released before housekeeping stopped, and an early return
from Serve could hang; housekeeping has its own context, cancelled and
joined inside Releasing and after an early return. An old runtime's
teardown could close the new runtime's journal, after which writes
reported success while recording nothing; the journal binds writes and
closes to the generation that opened it. Each fix was shown to fail
against the unfixed code.

Later, when it hurts: the size measurement ignores every metadata error
rather than only a vanished file; the trim notice's stderr line says only
what was saved to records survives.

Departures the read accepted: a trimmed mark on the first remaining
message so repeated sweeps do not stack notices; the trailing-turn clamp
reachable only past two hundred messages; the payload marker named
`current`; the store card on the Settings read the page already makes.
