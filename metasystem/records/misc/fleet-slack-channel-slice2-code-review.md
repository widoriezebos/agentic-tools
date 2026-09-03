# fleet-slack-channel slice 2: the one code review, by hand

Reviewer: m0b (Fable lane, R-25), 2026-09-03. Reviewed: the working tree of
job fsc2-build-1 (Sol) on 28b9fed8, against
`plans/fleet-slack-channel-slice2-design.md` revision 3 and the brief
`plans/fleet-slack-channel-slice2-code-critique-brief.md`.

Why by hand: four dispatches of the review under the claude runtime
(fsc2-code-crit1, 1b, 1c, 1d) each ended before their first turn on
`API Error: 529 Overloaded` (claude-result.json in each round). The lane
law is satisfied by the reviewer's model class, not by the dispatcher; the
brief's nine attack surfaces were read in that order. Finding standard
R-60-m1.

## Findings

None material.

## Read, by attack surface

1. Ref identity (telegram.go Receive L192–198): `Ref{ID: message_id,
   ThreadID: parent}` from the update alone; `Inbound.ThreadID` is the root
   resolved through the caller's list (`roots[ref.ID] = root`, receipts
   included). poll.go's `matchedRefs`, `alreadyRejected`, `unmatchedAlready`
   compare the full ref and so survive the question closing. Consumed row
   unchanged (destination, provider, Inbound.ThreadID, Ref, qid).
2. Receipt order (poll.go L187–216): row appended and written, failure
   point `rejection-recorded`, post, failure point `rejection-posted`,
   rewrite with `PostRef`; a failed post increments `q.Undelivered` and is
   written. Replay hits `alreadyRejected` before any network. Same
   `writeJSON` (temp + rename) as every question write.
3. Offset (telegram.go updates L203–229): `offset` only from the given
   cursor, `limit` 100, `timeout` 0, `allowed_updates ["message"]`; next =
   max(update_id)+1 or the input cursor; other chats and the bot's own
   messages skipped after the cursor is computed; 409 on getUpdates typed
   with the webhook text. Peek sends no offset.
4. Secrets (telegram.go request L61–97): one request function; every
   error path (marshal, request build, transport `*url.Error`, decode,
   status + description, result unmarshal) passes `channel.Scrub` with
   `dest.Secrets`; `Peek` uses it; the peek verb prints the typed error
   only. Test covers closed port, redirect, echoed 401, malformed body.
5. Loader (phase.go): one table, `withHuman`; status/ask/close false,
   poll and Run true; absent adapter → `Loaded{}`, nil error, every caller
   guards `Provider == nil` (close stays local, `l, _ :=` as before);
   unknown adapter and unknown fake face refused by name; human key by
   face; `loadChannel`/`conf`/`secret` removed from the command file and
   `TestLoadIsTheOnlyLoader` reads that file for `channel.destination.`
   and `loadChannel`. show, wait, fake serve, fake code untouched.
6. Slack (slack.go L104–113): `seenRoots` dedupe before paging; nothing
   else; slack_test.go unchanged. Poll passes root + each `PostRef` with
   the root in `ThreadID`.
7. Fake (fake.go): `/bot` prefix branch; `face` discriminator splits the
   rows, each face assigned ids from the one counter; emitted update
   carries message_id, date, text, chat.id, from.id, reply_to_message when
   `reply_to` ≠ 0; getMe 424242; unknown method answered `ok:false`, no
   invented operation.
8. Chunking (telegram.go splitText): line-bounded at 4000 runes, an
   overlong line hard-split, later chunks reply to the previous, first ref
   returned, later failure typed with its index; no `parse_mode`.
9. Tests: all 22 new names plus the six in channel_test.go present and
   asserting their names (the crash tests use `FailurePoint`, no sleeps);
   diff inside the brief's boundary (15 files); no refusal weakened; no
   benchmarks. Fixture's Telegram pass replies to the receipt's message id
   and asserts the peek line.

Notes (not material): `request` decodes the body before the 409 check, so
a non-JSON 409 would report the decode error rather than the webhook text
(Telegram answers JSON). `Credential` is called on every `Receive`, one
extra `getMe` per pass, mirroring the Slack adapter's `auth.test`.

Agreed parts land as written.
