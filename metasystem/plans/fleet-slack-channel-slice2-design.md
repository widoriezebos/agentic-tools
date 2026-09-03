# Fleet conversation channel, slice 2: the provider switch and Telegram

Goal: fleet-slack-channel. Author: m0b (Fable lane, design). Revision 3, 2026-09-03
(revision 1 reviewed by fsc2-design-crit1, eight findings folded; revision 2 closed by
fsc2-design-crit2, four findings folded as build law, §9).
Builds on `plans/fleet-slack-channel-design.md` (revision 4, "the base design"): §2 of
the base design is the fixed contract and this slice changes no line of it. Under
R-54-m1 tier three; R-60-m1 governs the review budget; R-31 no benchmarks; R-35 no
sleeps for ordering.

## 1. What this slice is for

Slice 1 landed (66dfaf77) with one Slack adapter, one fake, and two hand-written
adapter switches: `load` in `internal/channel/phase/phase.go` and `loadChannel` in
`cmd/metasystem/channel_verbs.go` each read the same seven keys and build the same
`DestinationConfig`, while the registry in `internal/channel/channel.go`
(`NewRegistry`, `Register`, `Resolve`) is defined and used by nothing. The base design
names `telegram` (getUpdates, one owned offset as cursor) as the next adapter. This
slice does exactly two things: (a) one provider switch, used by both callers, driven by
one table; (b) a Telegram adapter behind the unchanged §2 contract, proven against the
same directory-backed fake wearing a Telegram face. Nothing about questions, TOTP,
consumption, the four-phase answer machine or the tick changes.

## 2. The provider switch

**One loader.** `phase.Load(root string, withHuman bool) (Loaded, error)`, exported,
with `Loaded{Provider, Destination DestinationConfig, Adapter, Face, HumanUserID,
TOTPSecret string}`; the human keys are read only when `withHuman` is true (`poll` and
`phase.Run`), exactly as today's `loadChannel(root, withHuman)`, so `status --post`,
`ask` and `close` never refuse for a missing user id or TOTP secret (FSC2-R1-003). Its
callers are exactly today's: `status --post`, `ask`, `poll`, `close`, `phase.Run`;
`telegram peek` has its own token-only path (§6); `show`, `wait`, `fake serve` and
`fake code` read no configuration, as today (FSC2-R1-004). Absent adapter → `Loaded{}`
with `Provider == nil` and nil error; every caller checks `Provider == nil` before any
call and keeps today's behaviour (status prints locally, ask records undelivered, poll
and the tick return quietly, `close` closes the durable record locally without posting,
exactly as `runChannelClose` does today, FSC2-R2-003); no nil provider is ever called. `loadChannel`
in `cmd/metasystem/channel_verbs.go` and the private `load` in phase are deleted, and
the duplicated `conf`/`secret` helpers in the command file go with them (the phase
package's `get`/`secret` stay, unchanged in behaviour: environment, then
`metasystem.conf.local`, a committed secret-named key reported and ignored).

**One table.** Inside phase:

```
// name → resolver; face names the human user-id key (slack, telegram): the adapter's
// own name for slack and telegram, the configured face for fake (FSC2-R1-005)
type adapterLoad func(root string) (p channel.Provider, d channel.DestinationConfig, face string, err error)
var adapters = map[string]adapterLoad{ "slack": …, "telegram": …, "fake": … }
```

`Load` reads `channel.destination.fleet.adapter` (absent → `Loaded{}` and nil error:
the unconfigured case, silent, exactly as today); an unknown name is
`unknown channel adapter %q`. The resolver reads that adapter's own keys (§5) and
returns the provider, destination and face. The `fake` resolver reads
`channel.destination.fleet.fake.face` (default `slack`) and binds the adapter of that
face to the fake's base URL (§4), returning that face; a value that is neither `slack`
nor `telegram` is refused as `unknown fake face %q` (FSC2-R2-004). The human keys, when asked for,
are read by face: `channel.human.<face>.user-id` and `channel.human.totp-secret`. So
the existing fixture, which sets `channel.human.slack.user-id` with `adapter=fake`,
runs unchanged. The registry type in `channel.go` is replaced by this table (it never
had a caller; a second indirection buys nothing) — one deletion, no behaviour.

**Provider name on the record.** `PollConfig.ProviderName`, the consumed row's
`Provider`, `AnswerProof.Provider` and the four channel keys on the goal ledger keep
carrying the ADAPTER name (`slack`, `telegram`, `fake`), as today. The cursor record
gains `Provider`: `cursorRecord{Provider, Cursor}`; `Poll` treats a cursor written under
another provider name as empty. Switching a destination from Slack to Telegram must not
feed a Slack thread-map into Telegram's offset or the reverse.

**Every posted ref goes back to the provider.** Base §2 says `threads` = the caller's
open thread roots. Slack pages by root; Telegram has no threads, only "this message is a
reply to that message", and Wido may press Reply on the bot's rejection receipt rather
than on the question root. So `Poll` passes, for each open question, the root AND every
receipt it posted in that thread, each `MessageRef` carrying the root in `ThreadID`
(the root carries its own id there too, as today). To do that the receipt's ref is
recorded: `Rejection` gains `PostRef *MessageRef`, and the order becomes RECORD, then
POST, then record the ref (FSC2-R1-002): the rejection row is written with a nil
`PostRef` before the receipt is posted (the disposition is durable first, so a replay
of the same ref never posts a second receipt: receipts are at-most-once), then the post,
then the row is rewritten with the ref. A crash after the post and before the rewrite
leaves an orphan receipt: Wido's reply to it resolves to no root and lands in
`unmatched.jsonl`, while the question stays open with its root still answerable; a
crash after the record and before the post leaves a rejection with no receipt. Both
are the declared outcomes; the question's `Undelivered` count carries the failed post
as today. This NARROWS base §5's "each invalid inbound ref is answered once" to "at
most once" for every provider (FSC2-R2-001): neither Slack nor Telegram offers an
idempotency key for a post, so exactly-once across a crash between record and post is
not available, and a duplicate receipt (which would invite a duplicate reply) is the
worse failure. Wido's remedy for a missing receipt is unchanged: his next reply to the
root is disposed on its own merits. The Slack adapter dedupes the list by root before paging — its bytes on the
wire do not change. This is a clarification of §2's comment, not a change of
signature; base §2 stays the contract.

## 3. The Telegram adapter (`internal/channel/telegram`)

`New(client *http.Client) *Adapter`, stateless, `var _ channel.Provider = (*Adapter)(nil)`.
Bot API over `dest.APIBase + "/bot" + dest.Token + "/" + method`, JSON body, 15-second
context from the caller as today. `dest.ChannelID` is the chat id (Wido's private chat
with the bot, or a group), `dest.Token` the bot token, `dest.Secrets = [token]`.

**Post** = `sendMessage` with `chat_id`, `text`, no `parse_mode` (the strict token must
survive verbatim; Telegram's plain text does), and `reply_parameters.message_id` =
`thread.ID` when a thread is given. Returned ref: `{ID: message_id, ThreadID: root}`
where root = `thread.ThreadID`, else `thread.ID`, else the new message's own id (a root
carries its own id, as Slack's does). Telegram caps a message at 4096 characters: text
over 4000 runes is split at line boundaries into chunks of at most 4000 (a single line
longer than 4000 runes is hard-split at 4000, FSC2-R1-008), the first chunk posted as
described, each later chunk as a reply to the previous, and the FIRST
chunk's ref returned (the status report is the only long text; questions are short).
A failed later chunk is `ErrSendFailed` naming the chunk index; the first is already
posted and the caller counts one undelivered, as it does for any post failure.

**Receive** = one `getUpdates` with `offset` = the cursor (omitted when empty),
`limit` 100, `timeout` 0 (never long-poll inside the tick's context),
`allowed_updates` = `["message"]` (edits never arrive: a code appended by editing is not
a reply). For each update whose `message.chat.id` equals `dest.ChannelID` and whose
`from.id` is not the bot's own id (`Credential`): `Inbound{Ref: {ID: message_id,
ThreadID: parent}, ThreadID: root, UserID: from.id as decimal, Text: message.text (empty
for media), At: message.date}`, where `parent` is `reply_to_message.message_id` or ""
— the REF is built only from the update's own fields, so it is identical on every
re-read of the same update whatever the caller's list holds; `Poll`'s matched-ref,
rejection and unmatched dedupe therefore hold across a crash after a durable answer
whose question is no longer open (FSC2-R1-001). `root` is resolved from the caller's
list (§2): if `parent` equals the `ID` of any given ref, root is that ref's `ThreadID`;
otherwise root is "" and the caller records the envelope as unmatched, as today. A non-reply message while questions are open is therefore unmatched, never
guessed onto a question (D6 in §8). Other chats' updates are skipped but still advance
the offset. The returned cursor is `max(update_id) + 1` as a decimal string, or the
input cursor when nothing arrived. Envelopes are in update order.

**Why one owned offset is the acknowledgment.** Telegram confirms every update below
the `offset` it is handed. The adapter only ever sends the cursor the caller persisted,
and `Poll` persists it only after every envelope's disposition is durable (base §5), so
nothing is confirmed before it is recorded. When more than the per-pass budget arrives
(five dispositions), the cursor stays put, the next pass re-reads the same page and
`Poll`'s per-ref dedupe (matched refs, recorded rejections, `unmatched.jsonl`) skips
what is already disposed: bounded progress, at most 100 updates per page, never a loss.
Telegram keeps unconfirmed updates for 24 hours; a longer outage loses Wido's replies on
Telegram's side, and the question stays open and is re-asked by hand (base §7 `channel
show`). A webhook set on the bot makes `getUpdates` answer 409: typed
`ErrReceiveFailed("a webhook is set on this bot; delete it")`.

**Credential** = `getMe`; identity is the bot's `id` as decimal. `ErrReceiveFailed`
when `ok` is false or the id is zero.

**Errors.** Every error string, including Go's transport errors which embed the URL and
therefore the token, passes through `channel.Scrub(problem, dest.Secrets...)` before it
leaves the adapter; the API's `description` is included after scrubbing; the HTTP
status is named. `ErrUnconfigured` when token or chat id is empty.

## 4. The fake's Telegram face (`internal/channel/fake`)

The same server, the same directory D, the same monotonic counter, the same
`journal.jsonl` and `replies.jsonl`. Requests whose path begins with `/bot` take the
Telegram branch; everything else stays the Slack branch, unchanged. Telegram methods
served: `sendMessage` (assigns `message_id` from the counter; returns Bot API shape
`{ok, result:{message_id, chat:{id}, date, text}}`), `getUpdates` (reads new
`replies.jsonl` lines on every call, assigns each an `update_id` AND a `message_id` from
the counter, keeps them in memory, answers those with `update_id >= offset`, honours
`limit`), `getMe` (`{ok, result:{id: 424242, is_bot: true, username: "fakebot"}}`). A
scripted line for the Telegram face is `{"face":"telegram", "reply_to": <message_id or
0>, "user": <from.id>, "chat": <chat id, default 1000>, "text": …}`; the Slack branch
takes only lines without `face`, the Telegram branch only lines with `face":"telegram`,
and neither counts the other's lines (FSC2-R1-006). Each emitted update is
`{update_id, message:{message_id, date, text, chat:{id}, from:{id, is_bot:false},
reply_to_message:{message_id}}}` (the last only when `reply_to` is non-zero), every
field the adapter reads. The Slack face's `{"thread_ts","user","text"}` lines are
untouched. The
journal line is `{method, form}` as today, `form` holding the decoded JSON body for the
Telegram methods, so the fixture asserts `sendMessage` exactly as it asserts
`chat.postMessage`. The `fake` adapter with `face=telegram` binds
`telegram.New(nil)` to `APIBase` = `D/base-url`, token `fake-telegram-token`, chat id
`1000`, `Secrets` = [that token]. The fake still invents no operation: the Telegram
adapter's own bytes are what it proves.

## 5. Configuration (adds to base §6)

```
channel.destination.fleet.adapter=slack|telegram|fake
channel.destination.fleet.fake.face=slack|telegram            # fake only, default slack
channel.destination.fleet.telegram.chat-id=<integer>
channel.destination.fleet.telegram.bot-token=<SECRET, conf.local or env only>
channel.destination.fleet.telegram.api-base=https://api.telegram.org
channel.human.telegram.user-id=<integer>
```

Wido's acts for Telegram, after the build: create the bot with BotFather (the token),
send it `/start` from his phone (a bot cannot message a person first), and give the
two integers. `metasystem channel telegram peek` (§6) prints them for him.

## 6. Command surface

One addition: `metasystem channel telegram peek` — does not call `phase.Load`
(FSC2-R2-002): it reads exactly two keys through the phase package's exported
`phase.Secret(root, key)` and `phase.Get(root, key, def)`,
`channel.destination.fleet.telegram.bot-token` (environment or conf.local; absent →
typed `ErrUnconfigured` naming the key) and `channel.destination.fleet.telegram.api-base`
(default), builds a `DestinationConfig{Provider: "telegram", Token, APIBase, Secrets:
[token]}` with an empty chat id, whether or not the adapter key exists, and calls
`Adapter.Peek(ctx, dest) ([]Update, error)`, a fourth method on the Telegram
adapter only (not on the contract) that issues `getUpdates` WITHOUT an offset through
the same request function and scrub as the three contract methods (FSC2-R1-007), then
prints one line per pending update: `chat=<id> user=<id> text=<first 40 runes>`; never
confirms, never posts, never reads the cursor; its stderr is the scrubbed error. It
exists so Wido can find his chat id and user id from the bot's own view in one command.
The verbs named in §2 go through `phase.Load`; the tick is otherwise unchanged.

## 7. Proof plan (tests by name)

internal/channel/telegram, against the fake in-process: `TestPostRootAndReplyChain`
(root ref carries its own id; a reply carries the root; the journal shows
`reply_parameters.message_id`), `TestPostSplitsLongTextOnLines` (9000 runes → three
`sendMessage` calls, later ones replying to the previous, first ref returned, no line
cut), `TestReceiveResolvesRootsFromEveryPostedRef` (a reply to a receipt resolves to the
question root; a reply to nothing is root ""; the Ref is identical with and without the
list), `TestPostHandlesSingleLineOverChunkLimit`, `TestReceiveFiltersOtherChatsAndOwnBot`
(both skipped, offset still advances past them), `TestReceiveAdvancesOffsetOnlyOnUpdates`
(empty page returns the input cursor unchanged), `TestReceiveHonoursLimitAndOffset`,
`TestCredentialIsBotIdentity`, `TestUnconfiguredIsTyped`,
`TestWebhookConflictIsTyped` (409 → the named message),
`TestTokenNeverAppearsInErrors` (a closed port, a redirect, a 401 with the token echoed
in `description`, a malformed body: each error string free of the token and of the
URL's `/bot…/` segment). internal/channel: `TestPollPassesEveryPostedRefWithItsRoot`,
`TestRejectionRecordsItsPostRef`,
`TestRejectionReceiptPostRecordCrashIsDispositionSafe` (crash injected after the
record, after the post: one receipt at most, one rejection row, the replay posts
nothing), `TestEveryInvalidInboundRefIsAnsweredAtMostOnceAcrossRecordBeforePostCrash`
(the closing review's obligation under its decided reading: zero or one receipt per
invalid ref over any crash point, never two, the row always present), `TestTelegramCrashAfterMatchedDoesNotRedisposeReplyAsUnmatched` (a provider
returning Telegram-shaped refs; crash after the answer is durable; the re-poll with the
question closed appends nothing to `unmatched.jsonl`),
`TestCursorFromAnotherProviderIsIgnored`. internal/channel/phase:
`TestLoadResolvesAdaptersThroughOneTable` (fake with each face returns that face and
picks the matching human key, asserted exactly; telegram with a missing token is the
typed refusal; a committed token is reported and ignored; unknown adapter named; absent
adapter is silent), `TestOutboundVerbsDoNotRequireHumanAuthentication` (status, ask,
close with no user id and no TOTP secret succeed against the fake),
`TestAbsentAdapterNeverCallsNilProvider` (status --post, ask, poll, close with no
adapter: today's messages, no panic), `TestCloseWithoutAdapterClosesLocallyWithoutCallingProvider`,
`TestLoadRejectsUnknownFakeFace`, `TestLoadIsTheOnlyLoader` (a grep-style test
that `cmd/metasystem/channel_verbs.go` no longer reads `channel.destination.` keys).
cmd/metasystem: `TestConfigurationIndependentChannelVerbs` (show, wait, fake code run
with no `metasystem.conf` at all), `TestTelegramPeekWorksWithoutConfiguredAdapterOrChatID`
(token in the environment only, no adapter key, no chat id: the pending lines print),
`TestTelegramPeekTokenNeverAppearsInErrors`
(transport, redirect, echoed 401, malformed JSON: stderr free of the token).
internal/channel/fake: `TestTelegramFaceSharesTheCounter` (a post then a scripted reply
sort in order across faces), `TestTelegramFaceSeparatesScriptRowsAndOtherChats` (a
Slack row is not a Telegram update and the reverse; a row with `chat` 2000 is emitted
with that chat id). Fixture `scripts/agents/channel-fixtures.sh` gains a second
pass with `fake.face=telegram` and `channel.human.telegram.user-id=7001`: status post
(assert `sendMessage` in the journal), a budget question, a scripted reply from 7001
without a code (assert the receipt), a reply to that RECEIPT's message id ending in
`channel fake code` (assert `answer actor=human:wido`, the close post, a claim with
`--approved-ref` succeeding), `channel wait` returns the answer, and `channel telegram
peek` lists the pending scripted line before the poll. Coverage floors are the file's;
internal/channel packages have none and this slice adds none (a floor is governance).
Live: one `channel status --post` by hand when Wido's Telegram token arrives; never in
a suite (R-31).

## 8. Slices and decisions Wido may still change

This is slice 2 of the goal; slice 3 is WhatsApp (Cloud API, a webhook is mandatory
there, so it needs its own inbound design and is not attempted here). D6 a Telegram
message that is not a Reply to the question or one of its receipts is unmatched, never
attached to the single open question; Wido may prefer "one open question takes any
message from me". D7 chunk size 4000 on lines. D8 the fake keeps one server with two
faces rather than two binaries.

## 9. Self-grade and review dispositions

Weakest: the receipt's record-post-record order (§2) trades a possible missing receipt
for never a duplicate; the orphan-receipt case is declared, not solved. Second: `peek`
handles a token outside the tick; it shares the adapter's request path and scrub.
Round 1 (fsc2-design-crit1, revision 1): FSC2-R1-001 folded §3 (ref from the update's
own fields); 002 folded §2 (record first, at-most-once receipts, outcomes declared);
003 folded §2 (`withHuman`); 004 folded §2 (callers restricted, nil provider never
called); 005 folded §2 (the resolver returns the face); 006 folded §4 (face
discriminator, `chat`, emitted fields enumerated); 007 folded §6 (`Adapter.Peek` on the
shared path); 008 folded §3 (hard split). Notes 001–004 confirm the offset budget, the
Slack wire bytes, the 3500-byte report cap under the split threshold, and the
registry's retirement. Closing review (fsc2-design-crit2, revision 2), folded as build
law: FSC2-R2-001 §2 (base §5 narrowed to at-most-once receipts, reason given; the
obligation renamed for the decided reading); R2-002 §6 (peek's token-only path, no
`phase.Load`); R2-003 §2 (close stays local with a nil provider); R2-004 §2 (unknown
fake face refused). Notes 001–006 confirm the ref identity, the envelope-scoped TOTP
row, at-most-once attempts, the loader boundaries, the fake's fields and the unchanged
signature and Slack bytes. Every obligation both reviews named is in §7 by name. The
design is closed; the build brief follows.
