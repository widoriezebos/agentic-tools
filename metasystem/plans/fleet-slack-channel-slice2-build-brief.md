Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fleet-slack-channel)
Date: 2026-09-03

# Goal

Build slice 2 of the fleet conversation channel exactly as
metasystem/plans/fleet-slack-channel-slice2-design.md revision 3 decides
it (§2 to §7; §9 records how both reviews' findings were folded, and §7
carries every obligation they named). The base design
metasystem/plans/fleet-slack-channel-design.md revision 4 stays law where
the slice cites it; its §2 Provider signature does not change by one
byte. Slice 1 is on main (66dfaf77 and after): read the landed packages
metasystem/internal/channel, its slack, fake and phase subpackages, and
metasystem/cmd/metasystem/channel_verbs.go before writing. The standard
is Wido's: hard deterministic machinery, no refusal weakened, no
guarantee narrowed beyond what §2 decides (the at-most-once receipt), no
benchmarks (R-31), no sleeps for ordering (R-35).

# What to build, by package

1. `internal/channel/phase`: `Load(root, withHuman) (Loaded, error)` and
   the adapter table (§2), exported `Get` and `Secret` (today's private
   helpers, behaviour unchanged); `Run` uses `Load`; the private `load`
   goes. The `fake` resolver reads `fake.face` (default slack, unknown
   refused by name) and returns the face; human keys by face.
2. `internal/channel`: delete the unused registry (`Factory`, `Registry`,
   `NewRegistry`, `Register`, `Resolve`); `cursorRecord` gains `Provider`
   and `Poll` ignores a cursor under another provider name; `Rejection`
   gains `PostRef *MessageRef` with the record, post, rewrite order of §2;
   `Poll` passes the root and every recorded `PostRef` of each open
   question to `Receive`, each carrying the root in `ThreadID`.
3. `internal/channel/slack`: `Receive` dedupes the given refs by root
   before paging; nothing else changes (the wire bytes are proven by the
   existing tests staying green unchanged).
4. `internal/channel/telegram` (new): `New`, `Post`, `Receive`,
   `Credential` and the adapter-only `Peek` per §3 and §6; one request
   function for all four; `channel.Scrub` with `dest.Secrets` on every
   error string including transport errors; the ref from the update's
   own fields, the root resolved from the caller's list; chunking with
   the hard split; `offset`, `limit` 100, `timeout` 0, `allowed_updates`
   `["message"]`; 409 named as the webhook conflict.
5. `internal/channel/fake`: the Telegram face on the same server, same
   counter, same files, per §4: paths under `/bot…/` take the Telegram
   branch; `replies.jsonl` rows with `"face":"telegram"` (and only those)
   feed `getUpdates`; the emitted update carries every field §4 lists;
   `getMe` answers 424242. The `fake` adapter with `face=telegram` binds
   `telegram.New(nil)` to `base-url` with the fixed token and chat 1000.
6. `cmd/metasystem`: `loadChannel` and the duplicated `conf`/`secret`
   helpers go; `status --post`, `ask`, `poll`, `close` call `phase.Load`
   with today's `withHuman` values and today's nil-provider behaviour
   (close stays local); `show`, `wait`, `fake serve`, `fake code` read no
   configuration; new `channel telegram peek` on the token-only path of
   §6, output format exact.
7. `scripts/agents/channel-fixtures.sh`: the second pass with
   `fake.face=telegram` and `channel.human.telegram.user-id=7001` per §7,
   including the reply to the RECEIPT's message id and the `peek` line.
8. Configuration keys of §5 read through internal/config; secrets only
   from the environment or metasystem.conf.local; a committed secret-named
   key reported and ignored (unchanged helper).

# Tests

Every test named in the design's §7, by those exact names, each failing
before its code and passing after; the slice-1 tests stay green
unchanged (in particular the slack package's
TestReceivePagesAndFiltersByCursor and every test in internal/channel).
Crash points by injected failure, never by sleep.

# Gate

gofmt, go vet, go build; `GOFLAGS=-buildvcs=false go run
honnef.co/go/tools/cmd/staticcheck@2025.1 ./...` reporting nothing; go
test -count=1 over internal/channel/..., internal/goal,
internal/humanauthority, internal/governance, internal/steward and
cmd/metasystem green; `bash -n` on the fixture script and one run of it
in your sandbox (the fake is in-process; no network). Coverage floors in
metasystem/scripts/agents/coverage-ratchet-linux.json apply to every
staged Go package (internal/goal 80.0, internal/governance 100.0): if you
touch those packages, keep their in-package coverage at or above the
floor; a floor is governance and not yours to lower. The repository-wide
run's known sandbox failure (TestHolderProbeUnreadableArgvIsNeverDead) is
not yours. Paste the final lines. Leave the work in your working tree,
stage nothing and do not run the commit wrapper (the delegate's commit
wrapper has no lease; the orchestrator reads the working tree and carries
the bytes through the chain). The diffBoundary is the packages and files
named above and nothing else.

# Constraints

Wall-clock budget: 120 minutes. If the budget will not reach the fixture
pass, leave everything else green with the remaining steps listed in your
return and stop; a follow-up round in the same worktree picks up.
Version-2 implementer JSON.

# Gap Rule

stop and report a gap; never fill it silently.
