Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fleet-slack-channel)
Date: 2026-09-03

# Goal

The one code review of the slice 2 build of the fleet conversation
channel (job fsc2-build-1, Sol), tier 3 under R-54-m1: after this the
code lands through the chain. The build sits UNCOMMITTED in the
worktree's working tree on top of 28b9fed8 (10 modified files, 509
insertions, 172 deletions, plus the new Telegram adapter package under
internal/channel and three new test files, all untracked), against
metasystem/plans/fleet-slack-channel-slice2-design.md revision 3 (§2–§7
are the law; §7 carries both reviews' obligations by test name) and the
build brief metasystem/plans/fleet-slack-channel-slice2-build-brief.md.
Read the diff with `git status --short` and `git diff` in the worktree,
and the untracked files whole. The orchestrator's host gate on an export
of that tree is recorded below; do not re-run it.

# The standard for a finding (R-60-m1, binding)

A finding is material only if it changes what gets built AND names the
artifact (file, function, test). At the end of this review every disputed
point is either a one-hunk fix the orchestrator folds through a fix round
or a named test obligation; never a raise for another review. Zero
material findings is a closing answer if the reading supports it.

# Attack surface, in priority order

1. Telegram ref identity and dedupe (internal/channel/telegram, poll.go):
   is `Inbound.Ref` built only from the update's own fields (message_id,
   reply_to_message.message_id) with the resolved root in
   `Inbound.ThreadID` alone? Do `matchedRefs`, `alreadyRejected` and
   `unmatchedAlready` therefore hold on a replay after the question closed
   (the named crash test must inject the crash, not simulate it by hand)?
   Does the consumed row still carry the envelope (destination, provider,
   Inbound.ThreadID, Ref, question id)?
2. The receipt order (poll.go): rejection row written with a nil
   `PostRef` BEFORE the post, then the post, then the rewrite; on replay
   `alreadyRejected` skips before any network; a crash after either step
   leaves exactly one row and at most one receipt. Is the row written by
   the same durable write (temp + rename) as the other question writes?
3. The offset as acknowledgment (telegram.go Receive): `offset` = cursor
   only (omitted when empty), `limit` 100, `timeout` 0, `allowed_updates`
   `["message"]`; other chats and the bot's own messages skipped but the
   returned cursor still `max(update_id)+1`; empty page returns the input
   cursor unchanged; 409 typed as the webhook conflict. Does any path
   send an offset the caller has not persisted?
4. Secrets (telegram.go, channel_verbs.go peek): the token is a URL path
   segment. Is EVERY error string — transport (`*url.Error` embeds the
   URL), redirect, HTTP status with echoed `description`, JSON decode —
   passed through `channel.Scrub(problem, dest.Secrets...)`? Does `peek`
   use the same request function, and is its stderr scrubbed? Does the
   token-only path read only the two keys of §6 through the exported
   phase helpers, with the committed-key refusal intact?
5. The loader (phase.go, channel_verbs.go): one table, `withHuman`
   honoured (status/ask/close false; poll and Run true); absent adapter →
   `Provider == nil`, nil error, and every caller guards it with today's
   behaviour (close local); unknown adapter and unknown fake face refused
   by name; human key by face; `loadChannel`/`conf`/`secret` gone from the
   command file; show, wait, fake serve, fake code read no configuration.
6. Slack bytes (slack.go): dedupe by root before paging and nothing
   else; the existing slack tests unchanged. Poll passes root + every
   recorded `PostRef` with the root in `ThreadID`.
7. The fake (fake.go): `/bot…/` branch; rows with `"face":"telegram"` only
   feed `getUpdates` and the Slack branch ignores them (and the reverse);
   the emitted update carries message_id, date, text, chat.id, from.id,
   reply_to_message.message_id when `reply_to` ≠ 0; one counter shared
   across faces; `getMe` 424242. Does the fake invent any operation the
   adapter does not make?
8. Chunking (telegram.go Post): split at line boundaries at 4000 runes,
   a single overlong line hard-split, later chunks replying to the
   previous, the FIRST ref returned, a failed later chunk typed with its
   index. No `parse_mode`.
9. Tests and boundary: every test named in design §7 exists by that
   exact name and asserts what its name says (a tautology is a finding;
   `TestLoadIsTheOnlyLoader` must actually read the command file); any
   hunk outside the brief's diffBoundary; any weakening of an existing
   refusal or assertion; any sleep for ordering (R-35); R-31 no
   benchmarks. The fixture's Telegram pass replies to the RECEIPT's
   message id, not the root, and asserts the `peek` line.

# Host gate (recorded by the orchestrator)

On an rsync export of the worktree's working tree: go-build.sh, gofmt -l
empty, go vet clean, staticcheck 2025.1 silent; go test -count=1 over
internal/channel/... (five packages), internal/goal,
internal/humanauthority, internal/governance, internal/steward,
cmd/metasystem: all ok. channel-fixtures.sh (both faces): PASSED.
goal-cli-fixtures.sh: PASSED. The build's return declares no gaps.

# Return

Code-critic schema. Findings first, each naming the file, the function or
test, and the one-line change. Then one line: "agreed parts land as
written" or the disputed points as fixes or test obligations by name.

# Constraints

Wall-clock budget: 25 minutes. Your sandbox is read-only; verify by
reading. No redesign; the design's decisions are closed.

# Gap Rule

stop and report a gap; never fill it silently.
