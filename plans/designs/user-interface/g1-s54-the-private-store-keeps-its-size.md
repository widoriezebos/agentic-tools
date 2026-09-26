# g1-s54: the private store keeps its size

- Kind: design
- Id: 01M3EHSKGNBHDJEW5HM2PQVYMW
- Status: draft
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
2. **Nothing bounds the transcript or the wire journal.** Both are
   append-only; on this host after three days the wire journal is 121 KB
   and one conversation 49 KB; no reader trims either. The stickies store
   is bounded at 500 per human (g1-s50).
3. **What is memory and what is not.** The paper: records are the only
   memory that survives; the sitting has none of its own (docs/paper/15-the-sitting.md).
   The master: raw transcripts and streamed tool detail are "disposable"
   sitting material in a protected server-local store (user-interface-design.md:381);
   a fresh session is given the last few messages, not the file
   (partner/service.go, `freshLine`).

## 2. Decisions

- D1. **Three bounds, one owner.** `uihome` gains housekeeping the
  interface runs at start and once a day while it serves:
  - the wire journal rotates at 8 MB: the current file becomes
    `wire.1.jsonl`, one previous is kept, older ones removed;
  - a conversation is trimmed from its head when it passes 2 MB or its
    oldest message is older than 90 days, keeping at least the last 200
    messages and never cutting inside a turn; what is cut is gone, and
    the conversation's first remaining message says "earlier messages
    were trimmed on <date>";
  - a workspace directory whose newest file is older than 180 days is
    removed whole, so a checkout that was deleted does not keep its
    private store forever.
- D2. **Never the records.** Housekeeping touches only the private store
  under the account's home; nothing in a checkout, nothing under the
  state root.
- D3. **Said where the human reads.** Settings shows the private store's
  path, its size per workspace and the bounds in words; the Partner's
  help term says transcripts are trimmed and records are the memory.
- D4. **Numbers are configuration with defaults**, `ui.store.wire-mb`,
  `ui.store.conversation-mb`, `ui.store.conversation-days`,
  `ui.store.workspace-days` in `metasystem.conf`, so a project can
  keep more or less; zero disables that bound.

## 3. Not here, later

Archiving trimmed transcripts elsewhere. Per-sitting export. Bounds on
the checkout's own artifacts, which are the engine's.

## 4. Verification and box

Go: housekeeping over a fixture home with an oversized wire journal, an
oversized and an old conversation, an unused workspace, each bound
crossed and not crossed, the turn boundary respected, zero disabling a
bound, nothing outside the home touched; the daily run scheduled without
a wall-clock wait in tests. Frontend: the Settings lines. Budgets as
always. Box: one build lane (Claude on Opus), one code read (Codex on
Sol) with one fix round under R-124, after Astra's read; two attempts,
60 to 120 job-minutes.

## 5. Self-grade

High: three bounds in one owner over files it already owns. Medium on
the conversation trim: cutting inside a turn would leave a human message
without its answer; the boundary is the turn key. Weakest: the numbers
are guesses until a month of use says otherwise.
