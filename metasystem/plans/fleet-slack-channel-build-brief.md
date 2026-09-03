Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fleet-slack-channel)
Date: 2026-09-03

# Goal

Build slice 1 of the fleet conversation channel exactly as
metasystem/plans/fleet-slack-channel-design.md revision 4 decides it (§2
to §8; §5 and §8 already carry the closing review's five obligations,
whose findings and dispositions are recorded in
metasystem/records/misc/fleet-slack-channel-design-critique-r2.md). The
first build (fsc-build-1b) wrote nothing and stopped at gap FSC-B1-001:
the fake's state had no contract across the separate command invocations
the fixture makes. Revision 4 decides that contract (§2 fake provider, §7
the `channel fake serve` and `channel fake code` verbs, §8 the fixture):
a directory D holds `base-url`, `journal.jsonl` and `replies.jsonl`; the
same code serves in-process for Go tests and as a background command for
the fixture. Build to it; that gap is closed. The alert-channel sections the design adopts
(metasystem/plans/alert-channel-design.md §2, §2a, §3, §3a, §4, §10) are
law where the design cites them; do not rebuild what they defer. The
standard is Wido's: hard deterministic machinery, no refusal weakened, no
guarantee narrowed to make a test pass, no benchmarks (R-31).

# What to build, by package

1. `internal/channel` (new): the `Provider` interface (§2: Post, Receive,
   Credential), `Inbound`, `Cursor`, `MessageRef` (reuse the alert design's
   shape), typed sanitized errors, the registry (`slack`, `fake`; unknown
   names refused by name); `report.go` composing the §3 status from the
   goal ledger projection, origin/main `Goal-Item:` trailers, the job
   records (the jobs directory under the agents control plane, as
   internal/report scans them) and internal/usage facts (units per
   runtime, never dollars), with the cadence/digest state file and the
   size caps; `question.go` with the §4 record, ULID ids, dedup on (goal,
   kind, facts digest), the next-step `ASKED` line; `totp.go` (RFC 6238,
   SHA-1, 30 s, ±1 step) with the durable consumption row; `poll.go` with
   the flock, `Receive` once per pass, the rejection posts capped at three,
   the four-phase answer machine (op id allocated at MATCHED, history op at
   RECORDED through the goal transaction engine, receipt, close), the
   destination cursor persisted only after every envelope's disposition is
   durable, the work budget (one Receive, five dispositions, one status
   post) and one 15-second context for the whole pass; secret redaction of
   every configured secret from every error string (§6, alert §10).
2. `internal/channel/slack` (new): `chat.postMessage`, `conversations.replies`
   paged with `oldest` per thread root and the root→last-ts cursor map,
   `auth.test` as Credential; base URL from `slack.api-base`.
3. `internal/channel/fake` (new): `fake.Serve(ctx, dir)` per §2 — the HTTP
   server speaking those three methods with Slack's JSON shapes, `base-url`
   written by rename once listening, `journal.jsonl` appended per request,
   `replies.jsonl` read on every `conversations.replies` call with ts from
   one monotonic counter shared with posts, `auth.test` = `UFAKEBOT`. The
   `fake` adapter name binds the Slack adapter to `fake.dir` (base URL from
   `base-url`, absent → `ErrUnconfigured`).
4. `internal/goal`: the `answer` history verb writing `actor=human:wido`
   with the answer text (and the `wants` token verbatim when present) in
   the reason field; `ParseHistoryLine` and the writer accept authority
   outcome `AUTHENTICATED_CHANNEL_WORD` with proof keys provider, userId,
   messageRef, step, next to `TEMPORARY_HUMAN_WORD`; round-trip grammar
   tests; a repeated op id is a no-op through the transaction engine.
5. `internal/humanauthority` and `internal/governance`: the new outcome
   and its recorded validator; consumers `goal resume --approved-ref
   <opid>` and `goal set-obligation --approved-ref <opid>` (in
   metasystem/cmd/metasystem/goalsync_mutations.go beside the existing
   `--temporary-human-word` flags) validate an AUTHENTICATED_CHANNEL_WORD
   operation on the same goal, independent of the R-32-m1 horizon;
   set-budget and enroll-terminal unchanged; `goal claim --approved-ref
   <opid>` needs no change beyond the grammar (verify with the named test).
6. `cmd/metasystem`: the `channel` verb family (§7: status [--post], ask,
   show, wait, poll, close, and the fixture-only `fake serve --dir`, `fake
   code --secret [--at]`) reading §6 keys through internal/config, with
   secrets only from environment or metasystem.conf.local.
7. The channel phase as the LAST duty in both tick drivers: the resident
   runner loop in metasystem/internal/steward/runner.go after
   `DeliverPending`, and the standalone tick in
   metasystem/cmd/metasystem/steward_verbs.go after its `DeliverPending`;
   never inside `RunTick`, never under the arbitration lock; a failure is
   an undelivered count and a stderr line, never an error return.
8. `scripts/agents/channel-fixtures.sh` (new): the §8 end-to-end fixture
   against the fake (background `channel fake serve`, trap-killed; bounded
   wait for `base-url`), runnable from a clean export like the other fixture
   scripts in metasystem/scripts/agents (read goal-cli-fixtures.sh there
   for the clone-and-run idiom).

# Tests

Every test named in the design's §8, plus the closing review's
obligations, by those exact names; each fails before its code and passes
after. Fixture-driven where the design says so; no sleeps for ordering
(a synthetic clock or injected failure points).

# Gate

gofmt, go vet, go build; go test -count=1 over internal/channel/...,
internal/goal, internal/humanauthority, internal/governance,
internal/steward and cmd/metasystem green; `bash -n` on the fixture
script and one run of it in your sandbox if network-free execution is
possible there (it should be: the fake is in-process). The repository-wide
run's known sandbox failure (TestHolderProbeUnreadableArgvIsNeverDead) is
not yours. Paste the final lines. Commit on your branch; the diffBoundary
is the packages and files named above and nothing else.

# Constraints

Wall-clock budget: 120 minutes. If the budget will not reach the fixture
script, land everything else green with the fixture's remaining steps
listed in your return and stop; a continuation build picks up your
commit. Version-2 implementer JSON.

# Gap Rule

stop and report a gap; never fill it silently.
