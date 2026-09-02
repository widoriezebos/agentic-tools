# Job record birth token: design

Goal: `plans/goals/job-record-birth-token.md` (claimed 2026-09-02, revision 3,
4-hour box). Revision 1 of this note, 2026-09-02, authored under job
birth-token-design-r1. Every path below is relative to `metasystem/`.

## 0. The decision in one paragraph

Every job record gets a field named `birthToken`. The three create paths in
`internal/dispatch` mint it under the record lock from the clock plus 16
random bytes, overwrite whatever the caller's source carried, and persist it
in the record's first write. The field joins `immutableFields`, so the
compare-and-swap refuses any patch that names it; the setup handshake carries
the husk's value forward and refuses a source that carries a different one;
no other writer touches it. Records born before this lands never gain the
field: readers treat absence as "pre-contract", never as an error, and no
verb back-fills it. The token is opaque bytes to every reader. Nothing parses
its timestamp. Two landed designs consume it by this name: the alert channel's
retention pin and the failed-job-attention dedup digest.

## 1. Facts traced in the tree

- The spike's verdict (`records/misc/alert-channel-spike-verdicts.md:12`,
  evidence at lines 17 and 25): against the real writers, `createdAt` is
  neither mandatory nor immutable, `startedAt` and `claimEpoch` are immutable
  but optional and caller-supplied, inode and file birth change on every
  atomic rewrite, and a second-precision timestamp mint still collides on
  same-second reuse while a timestamp-plus-nonce mint passes with no clock
  assumption.
- `immutableFields` is the map at `internal/dispatch/record.go:60-75`.
  `RecordCAS` refuses any patch naming a member with the typed refusal at
  `record.go:522-525` (`*OpError`, code 1, message "record patch attempts to
  change immutable identity"). Patches merge field by field at
  `record.go:537-539`, so a field a patch does not name survives untouched.
- There are THREE create paths, not one. The legacy path is `RecordCreate`
  at `record.go:245-272` (writes at 271). The indexed path, taken when the
  source carries a `sessionKey`, is `recordCreateLocked` at
  `record.go:278-312` (writes at 310). The claim-launch path builds the
  reservation with `claimReservationRecord` (`internal/dispatch/claim.go:669-720`)
  and writes it directly at `claim.go:522`, inside `withRecordLock` taken at
  `claim.go:450`. `docs/design/dispatch-sequence.md:119-128` names that
  third path as the production reservation. Both `RecordCreate` variants
  also run under `withRecordLock` (`record.go:139-174`).
- `RecordSetup` (`record.go:318-371`) REPLACES the husk with the caller's
  setup source. It refuses a mismatch on the identity fields at
  `record.go:328-342`, then copies a fixed list of reservation-minted fields
  from the husk into the source at `record.go:346-362` with the rule that
  "the reservation's truth wins unconditionally". A field not on that list
  is dropped by setup. So immutability alone is not enough: the token must
  join the carry list or setup would erase it.
- The setup and follow-up record builders (`internal/dispatch/build.go:148-168`,
  `365-437`, and the follow-up builder ending at `build.go:648`) write staging
  files that create and setup consume; they are not record writers.
- The source-file builder stamps `createdAt` at `build.go:163`; the claim
  path stamps it at `claim.go:705`. Neither value is what this design mints.
- `RecordProtocolError` (`record.go:420-465`) and `RepairClaim`
  (`record.go:726-753`) read the record under the lock, set named fields,
  and write it back. Several dedicated-metadata writers do the same:
  `internal/dispatch/critique.go:328-351`, `review_reference.go:132-139`,
  `finding_register.go:123-131`, `custody.go:69-93`, `adoption.go:86-148`,
  `claim.go:740-755`, and `launch_capability.go:242-271`.
- One writer rewrites a record outside the lock and outside the wire
  renderer: `internal/steward/reap.go:143-169` decodes with `encoding/json`,
  sets `chainClosed`, and renames a temp file into place. A string field
  survives that round trip unchanged.
- The fake host simulator writes a complete fake job record directly at
  `internal/host/fake.go:96-99`. It is fixture-only (the runtime table in
  the orchestration skill calls it a protocol simulator).
- No shell script writes a job record directly. A search of `scripts/` for
  redirections into `$jobs/` finds only `.log` appends
  (`scripts/agents/dispatch.sh:2366-2416`). The shell reaches records only
  through `job record-create`, `record-setup`, `record-cas`,
  `record-protocol-error`, and `repair-claim` (`dispatch.sh:2450-2477`,
  verbs at `cmd/metasystem/main.go:104-108`, flag parsing at
  `cmd/metasystem/dispatch_verbs.go:150-162`, `585-621`, `623-657`).
- Existing nonce precedent: `claimNonce` at `claim.go:385-391` mints 16
  bytes from `crypto/rand` as 32 lowercase hex characters;
  `mintDelegateClaimCapability` at `launch_capability.go:39` takes the clock
  and the entropy reader as parameters so tests inject both.
- The record's timestamp grammar is `nowISO` at `record.go:709-711`:
  `2006-01-02T15:04:05Z`, UTC, whole seconds.
- The renderer (`record.go:625-636`, via `internal/wiredoc`) writes whatever
  keys the map holds; the corpus test at
  `internal/dispatch/corpus_capture_test.go:42-48` pins that unknown keys
  render. No job-record JSON schema exists under `scripts/agents/schemas/`
  (the only `startedAt` there is `mission-state.schema.json:116-123`, a
  mission-state field).

## 2. The field

**Name.** `birthToken`. Stated once here; the two waiting designs read it by
this name. The failed-job-attention design calls the fact it carries `birth`
inside its own episode file (`plans/failed-job-attention-design.md:329-331`);
that is its fact key, not the record field, and stays as it is.

**Value.** One string of exactly 53 bytes:

```
<UTC second timestamp, 20 bytes>-<32 lowercase hex, 16 random bytes>
```

Grammar, as the build's regular expression:

```
^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z-[0-9a-f]{32}$
```

The timestamp is the record's own format from `nowISO`
(`record.go:709-711`). The nonce is 16 bytes from `crypto/rand`, hex
encoded, the same width as `claimNonce` (`claim.go:385-391`) so the package
has one nonce width. The separator is a hyphen; the layout is fixed width,
so the nonce is always the final 32 bytes. Pinned vector: clock
`2026-09-02T17:20:05Z` and entropy bytes `00 11 22 33 44 55 66 77 88 99 aa
bb cc dd ee ff` mint exactly

```
2026-09-02T17:20:05Z-00112233445566778899aabbccddeeff
```

The token is LF-free, whitespace-free, and slash-free, which is what the
alert channel's digest joining requires
(`plans/alert-channel-design.md:1961-1967`). Why keep the timestamp when the
nonce alone is unique: the goal record asks for timestamp plus nonce, the
spike's passing probe used that shape, and a human reading a record sees
when the incarnation was born without opening anything else. Why the token is
still opaque: no reader parses the timestamp out of it (section 5 keeps every
age computation on the lifecycle fields it uses today), so a clock step never
changes any reader's answer.

**Mint helper.** One unexported function in `record.go`:

```go
func mintBirthToken() (string, error)
```

It reads two package variables, `birthTokenClock func() time.Time`
(default `time.Now`) and `birthTokenEntropy io.Reader` (default
`rand.Reader`), formats `birthTokenClock().UTC()` with the `nowISO` layout,
reads 16 bytes, and returns the joined string. An entropy read error returns
an error; each create path wraps it as `refuse(1, "cannot mint birth token
for %s: %v", job, err)`, a harness failure with exit 1, never a delegate
outcome. The variables exist only so tests inject a fixed clock and fixed
bytes; production never sets them.

**Where it is minted.** At each of the three create paths, after every
existing check passes and immediately before the first write, one line:
`record["birthToken"] = token`. The assignment is unconditional, which is
the "ignore any caller-supplied value" rule: a source that carries
`birthToken` with any value, including null, is overwritten.

| Create path | Mint site | Existing write |
| --- | --- | --- |
| `RecordCreate`, legacy | between `record.go:270` and `271` | `record.go:271` |
| `recordCreateLocked`, indexed | between `record.go:309` and `310` | `record.go:310` |
| `claimLaunchAttemptLocked`, reservation | between `claim.go:521` and `522` | `claim.go:522` |

All three sit inside the job's record lock (`record.go:245`, `record.go:279`,
`claim.go:450`). The source builders (`build.go:148-168`,
`claimReservationRecord` at `claim.go:669-720`) do not set the field; the
mint belongs to the writer. `scripts/agents/dispatch.sh` needs no change:
`__record-create` (`dispatch.sh:2450-2456`) forwards the source and the
engine mints.

**Why not reuse `instanceTag`.** It is the nearest existing per-reservation
minted value (`claim.go:488-499`, immutable at `record.go:72`). It does not
qualify: the legacy create path mints nothing (`record.go:245-272`), so the
field is optional; and its shape is bound to the process census, which
matches it in argv. Coupling the incarnation identity to a process tag would
make a census change a record-identity change. The spike's rule names a
distinct field, and this design keeps it distinct.

## 3. Immutability: every writer, and what each does with the field

`"birthToken": true` joins `immutableFields` at `record.go:60-75`. The
table lists every path that writes a job record, with its disposition. "No
change" means the build touches nothing there and section 6 proves the
behavior.

| Writer | Where | What it does with `birthToken` | Change |
| --- | --- | --- | --- |
| `RecordCreate` legacy | `record.go:245-272` | Mints, overwriting any source value | Mint (section 2) |
| `recordCreateLocked` indexed | `record.go:278-312` | Same | Mint (section 2) |
| Claim-launch reservation | `claim.go:450-524` | Same | Mint (section 2) |
| `RecordSetup` | `record.go:318-371` | See the setup rule below | Refusal plus carry |
| `RecordCAS` | `record.go:475-553` | A patch naming the field, with any value including the current one or null, is refused at `record.go:522-525` with the typed `*OpError` (code 1, "record patch attempts to change immutable identity"); the record is not written. A patch not naming it leaves it intact (`record.go:537-539`) | Map entry only |
| `RecordProtocolError` | `record.go:420-465` | Sets status, error, phase, protocolError, endedAt; never the token | No change |
| `RepairClaim` | `record.go:726-753` | Sets returnRepairs only | No change |
| Critique exhaustion writer | `critique.go:328-351` | Sets critiqueExhaustions only | No change |
| Review reference stamp | `review_reference.go:132-139` | Sets one pointer field on the root | No change |
| Finding register fold | `finding_register.go:123-131` | Sets the register fields on the root | No change |
| Custody registration | `custody.go:69-93` | Sets custodyProcesses | No change |
| Reconciliation publish | `adoption.go:86-148` | Sets reconciliation, identity, status fields; never the token | No change |
| Reconciliation hand-off | `claim.go:740-755` | Sets reconciliationHandoff | No change |
| Adapter launch-capability consume | `launch_capability.go:242-271` | Rewrites the nested launchCapability object | No change |
| Steward chain close | `steward/reap.go:143-169` | Round-trips the whole record through `encoding/json` and sets chainClosed; a string field survives byte-identical | No change; its bypass of the record lock and the wire renderer predates this design and is out of scope |
| Fake host simulator | `host/fake.go:96-99` | Writes a fake completed record with no token | No change; its records are pre-contract by construction |
| Go test helpers writing records directly | for example `decisions_test.go:114`, `exact_identity_test.go:72-75` | No token | No change; pre-contract by construction |

Callers of `RecordCAS` supply patches and never name the field; none needs a
change, and the map entry protects against any future one: the CLI verb
(`dispatch_verbs.go:600-621`); the shell relays (`dispatch.sh:157`, `159`,
`181`, `215`, `1619`, `2112`, `2464-2467`); the adapter patch producers
(`internal/adapter/patch.go:15-59`: effectiveModel, transport,
returnRepairs, error/phase/usage) and the ownership patch
(`internal/dispatch/ownership.go:81`); the adapter shells
(`scripts/agents/adapters/runtime-common.sh:167`, `180`, `191`, `429`;
`devin.sh:262`; `fake.sh:110`, `176`); the steward reaper
(`steward/reap.go:109-138`); the supervision reaper's applier
(`cmd/metasystem/supervise_component.go:338-369`, serving
`internal/supervise/reaper.go:115`); and the mission drain
(`internal/missionrunner/drain.go:296-319`).

Files that call `writeRecord` on paths that are not job records need no
change and are listed so the enumeration is checkable: `build.go:168`,
`436`, `648` (staging sources), `envelope.go:101`, `ownership.go:81`,
`prefork.go:67`, `mirror.go:243`, `claim_occupancy.go:427`, and
`launch_capability.go:69`, `135` (the capability store).

**The setup rule** (`RecordSetup`, `record.go:318-371`). Setup replaces the
husk with the source, so the source's value can never be honored and the
husk's value must survive. Two mechanical additions:

1. Refusal, added to the identity block at `record.go:328-342`: if the
   source has the key `birthToken` and its value is not `sameValue` to the
   husk's (`record.go:680-690`; an absent husk value counts as different
   from any present non-null source value), refuse with
   `refuse(1, "invalid setup transition for %s: birthToken is minted at create and cannot be supplied", job)`.
   A source without the key, or with the husk's exact value, passes.
2. Carry: delete `birthToken` from the source map, then add `"birthToken"`
   to the carry list at `record.go:346-354`, so the husk's value (when
   present) is copied in under the existing rule at `record.go:359-361`.

Consequences, stated so nobody has to infer them: a husk with a token always
sets up with that token; a pre-contract husk (created before the build,
set up after) sets up with no token and stays pre-contract; setup never
mints; the persisted value is never null.

**Fields that stay out.** `jobIdentityKeys` in
`internal/validate/authorization.go:35-40` is the immutable subset the
host-implementer wall digests into an integration authorization. It is
already a strict subset of `immutableFields` (it lacks `operationId`,
`goalId`, `sessionKey`, and more from `record.go:66-74`), and it binds
provenance, not incarnation. Adding the token there would change the digest
of every future authorization, a wall-contract decision this goal does not
own. Unchanged.

## 4. Pre-contract records

The rule, word for word the one the failed-job-attention design already
states (`plans/failed-job-attention-design.md:314-319`): "a record without
the minted field is a pre-contract record; its birth element is EMPTY,
forever. No fallback bytes are ever hashed, so no record's digest ever
changes: a record either carries the mint from its creation (the field is
immutable and minted only at create) or never carries it."

This design binds that rule on the writer side:

- No verb, sweep, migration, or setup path ever adds `birthToken` to an
  existing record. Minting happens at the three create sites and nowhere
  else.
- The one reader is `func BirthToken(record map[string]any) string` exported
  from `record.go`, returning `asString(record["birthToken"])`. Absent, null,
  and non-string all return the empty string, and the empty string means
  pre-contract. It never returns an error and never falls back to
  `createdAt`, `startedAt`, `claimEpoch`, or anything else. A consumer that
  cannot import `internal/dispatch` applies the same three-line rule inline;
  whether `internal/evidence` can import dispatch is the alert channel
  slice's question, not this one's.
- The alert channel design still carries a fallback clause ("fallback
  `createdAt` then `startedAt`, else empty") at
  `plans/alert-channel-design.md:1380`, `1472-1473`, and `2094-2096`. That
  clause contradicts the failed-job-attention rule and this contract, and
  its own round-4 text already concedes why (the fallback bytes can repeat
  across a reuse, `plans/alert-channel-design.md:2113`, last sentence). Its
  next revision replaces the fallback with "empty for a pre-contract
  record". This note cannot edit that file; the coordinator carries the
  amendment. The alert channel's empty-birth vector at
  `plans/alert-channel-design.md:1991-1993` and the failed-job-attention
  empty-birth vector at `plans/failed-job-attention-design.md:289-291` are
  both exactly what this rule produces for a pre-contract record.

Why absence is safe rather than a hole: identifier reuse needs the old
record gone (`RecordCreate` refuses only while the file exists,
`record.go:246-247`), evidence GC prunes terminal records only
(`internal/evidence/gc.go:384-386`), and every record created after landing
carries a token. So at most one pre-contract incarnation per id can ever
exist on disk, and every later incarnation of that id differs from it by the
token. The failed-job-attention design argues the same at lines 322-328.

## 5. Every incarnation-comparison caller

The goal record says whoever builds this "runs every reader of the record
identity". This section reads "runs" as: find every place that decides
whether two job records, or a record and a remembered key, are the same
incarnation, and state its disposition. Dispositions are switch, stay, or
out of scope, each with the reason.

| Caller | Where | Disposition | Reason |
| --- | --- | --- | --- |
| Failed-job-attention dedup digest | `plans/failed-job-attention-design.md:280-292`, `314-328` (design only, not in the tree; goal is `BlockedBy` this one) | Switches, by its own design | The digest's third element is the minted token verbatim, empty for pre-contract. Section 4 verified the rule matches word for word. |
| Alert channel retention pin and producer digest | `plans/alert-channel-design.md:1468-1497` (digest), `2085-2113` (pin); the pin will sit in `internal/evidence/gc.go:375-404` where GC already reads the whole record at line 380 | Switches, by its own design, with one amendment | Keys on the minted token. The fallback clause at lines 1380, 1472-1473, 2094-2096 is replaced by the section 4 rule at its next revision. |
| Follow-up chain resolution | `dispatch.sh:1703` (`root_job_id`, `dispatch.sh:787-789`, into `internal/usage/usage.go:47-69`), `dispatch.sh:1724-1757` (the gate), `dispatch.sh:1725` (`latest_chain_record`, `dispatch.sh:791-793`, into `internal/dispatch/chain.go:95-115`) | Stays | These resolve lineage by NAME: `parentJob` is a string id, and a child binds to its parent by that string alone. Binding lineage to incarnations needs a second field (a parent birth token stamped at follow-up create and checked on the walk), which is a second record-contract change outside this 4-hour item. Residual, disclosed: after evidence GC collects a completed root while a pinned failed child survives (the spike's finding 4, `records/misc/alert-channel-spike-verdicts.md:12`), a fresh dispatch reusing the root's id makes the old child's `parentJob` resolve to the new root. The coordinator decides whether to backlog "parent birth token chain binding". |
| Mission chain incarnation check | `internal/dispatch/build.go:19-37`, called at `build.go:495-500` and `dispatch.sh:1812-1813` | Out of scope | "Incarnation" there is the MISSION's approved-contract digest (`build.go:41-53`), a different noun. |
| Reap facts | `internal/dispatch/reapfacts.go:63-77` (`createdAt` age), `87-101` (`startedAt` handshake age), `138-152` (`CapExpired`); the supervision reaper `internal/supervise/reaper.go:110-121`; the mission drain `missionrunner/drain.go:168`, `364-374` | Stays | Every one compares a record's lifecycle time to the clock. None asks whether two records are the same incarnation. The token is opaque (section 2), so it is not a substitute for any of these timestamps. |
| Steward failed-job candidacy | `internal/steward/delivery.go:170-228` (per-goal job facts), `246-264` (`reservedAt` from the newest of `createdAt` and `startedAt`), `282` (a failed job counts when its `endedAt` is after the claim, after the receipt, and newest) | Stays | It orders records by time against the goal's claim and receipt times. A reused id is a new record with a new `endedAt`, so it already sees a new incarnation as new. |
| Steward dead-delegate check | `internal/steward/health.go:847-890` | Out of scope | Proves process liveness by pid identity; no record-to-record comparison. |
| Unwatched-work block-once key | `internal/goal/turnverdict.go:225-233` builds `job:<id>@<startedAt>`; fed by `internal/report/scan.go:156-190`; the test at `internal/goal/turnverdict_test.go:455-462` names the reuse case | Stays | The key exists so a NEW in-flight incarnation re-arms the block. Reuse of an id needs the old record collected, which needs it terminal past the grace window; the digest set the key feeds is per session. A same-second `startedAt` across a collect-and-reuse cycle is unreachable. Switching would touch three packages (`goal`, `report`, `run`) for no reachable defect. |
| Job waiter target | `internal/dispatch/watch.go:18-28`, `internal/report/scan.go:185-186`, `internal/run/waiter.go:38-43`, `195` | Stays | Same argument: a waiter is pinned to an in-flight lifecycle by `startedAt`, and in-flight records are never collected, so the id cannot be reused underneath a live waiter. |
| Session occupancy index and reservation tag | `claim.go:488-499` (fresh nonce per reservation, immutable `instanceTag` at `record.go:72`), `claim_occupancy.go:449-461` | Out of scope | Already per-reservation by its own nonce; section 2 says why the two identities stay separate. |
| Integration authorization digest | `internal/validate/authorization.go:35-40` | Out of scope | Section 3, "Fields that stay out". |
| Goal incarnations | `internal/goal/file.go:164` | Out of scope | Goal identifiers, not job records. |

## 6. Fixtures

All deterministic: temp roots, injected clock and entropy through the two
package variables, no sleeps. Unless named otherwise they live in
`internal/dispatch/record_test.go`, using its `createPending` and
`setupPending` helpers (`record_test.go:52-76`) and `wantCode`
(`record_test.go:78-87`).

1. `TestBirthTokenMintGrammar`: with the clock fixed at
   `2026-09-02T17:20:05Z` and the entropy reader yielding the 16 bytes
   `00 11 ... ff`, `mintBirthToken` returns exactly
   `2026-09-02T17:20:05Z-00112233445566778899aabbccddeeff`, 53 bytes, and
   the grammar regular expression matches it. An entropy reader that errors
   makes `RecordCreate` return an `*OpError` with code 1.
2. `TestRecordCreateMintsBirthTokenIgnoringCallerValue`: the create source
   carries `"birthToken": "caller-supplied"`; the persisted record's value
   matches the grammar and is not the caller's; a second source carrying
   `"birthToken": null` for a different id persists a grammar-valid token.
3. The indexed path: the existing indexed record-verbs test at
   `internal/dispatch/occupancy_index_test.go:156-192` gains one assertion
   after its `RecordCreate`: the reservation carries a grammar-valid token,
   and it survives the test's `RecordSetup` at line 183 unchanged.
4. The claim-launch path: `TestClaimLaunchWONReservationCompletesRecordSetup`
   (`internal/dispatch/claim_test.go:205`) gains the same two assertions
   around its `RecordSetup` at line 230.
5. `TestBirthTokenSameSecondReuseIsDistinct`: clock fixed; create `job-a`,
   read token T1; remove the record file as GC would; create `job-a` again
   under the same fixed clock; T2 differs from T1, both share the same
   20-byte prefix, and their nonces differ. Real `crypto/rand` is fine here
   because the assertion is inequality; no clock movement is needed.
6. `TestRecordCASRefusesBirthTokenPatch`: after create and setup, a patch
   `{"birthToken": <current value>}` and a patch with a different value are
   each refused with code 1 and the immutable-identity message; the record
   bytes on disk are unchanged; a patch naming only `phase` succeeds and the
   token is still present and equal.
7. `TestRecordSetupCarriesBirthTokenAndRefusesADifferentOne`: a setup
   source without the key persists the husk's token; a source with the
   husk's exact value persists it; a source with a different value is
   refused with code 1 and the husk is unchanged.
8. `TestPreContractRecordNeverGainsBirthToken`: a pending-setup record is
   written directly without the field (the idiom at
   `decisions_test.go:114`), then driven through `RecordSetup`, a
   pending-to-running `RecordCAS`, `RecordProtocolError`, and `RepairClaim`;
   after each step the key is absent and `BirthToken` returns the empty
   string. A record written with `"birthToken": null` also reads as empty.
   A setup source carrying a token against that pre-contract husk is
   refused (section 3's setup rule).
9. `scripts/agents/record-protocol-fixtures.sh`: updated by name. After the
   existing grep at lines 47-48, one more grep proves the persisted smoke
   record carries `"birthToken": "` followed by a grammar-valid value. This
   is the one shell path Go cannot prove (`__record-create` forwarding,
   `record-protocol-fixtures.sh:41`).
10. `scripts/agents/return-schema-fixtures.sh`: passes unchanged. Its record
    inputs (`return-schema-fixtures.sh:107`, `275-278`) are hand-written
    pre-contract files fed to `adapter fake-return`, which never reads the
    token.
11. `TestJobRecordCorpus` (`corpus_capture_test.go:64-80`) passes unchanged:
    it renders fixed maps, never a created record.
12. Any existing test that asserts whole-record equality after a create is
    updated to expect the field; the build's return names each one. The
    tests read so far assert fields individually
    (`record_test.go:89-100`, `dispatch_verbs_test.go:87-122`).

The validation suite's process-owning fixtures (`dispatch-fixtures.sh`)
cannot run in a delegate sandbox (KI-15); the orchestrator runs them
outside it. The design expects them to pass unchanged because no fixture
there names the field.

## 7. Size and diff boundary

Precedent: the spike's throwaway package proved the mint and the
immutability refusal in one job with six passing probes
(`records/misc/alert-channel-spike-verdicts.md:16-17`). The product change
here is smaller than the spike: one helper and one regular expression, two
package variables, three one-line mint sites, one map entry, one refusal
clause and one carry-list entry in setup, one exported reader. About 40
product lines and about 150 test lines. Estimate against the 4-hour box:
build 90 minutes, implementation critique 60 minutes, gate and landing 30
minutes. One slice.

Diff boundary of that slice, exhaustive:

- `internal/dispatch/record.go` (helper, variables, map entry, setup rule,
  mint sites, exported reader)
- `internal/dispatch/claim.go` (one mint line before `claim.go:522`)
- `internal/dispatch/record_test.go` (fixtures 1, 2, 5, 6, 7, 8)
- `internal/dispatch/occupancy_index_test.go` (fixture 3)
- `internal/dispatch/claim_test.go` (fixture 4)
- `scripts/agents/record-protocol-fixtures.sh` (fixture 9)
- `docs/design/dispatch-sequence.md` (one sentence in section 9, lines
  119-128: the reservation also mints the birth token)
- `docs/glossary.md` (one entry: birth token)

Nothing else. No shell dispatcher change, no adapter change, no steward
change, no GC change, no consumer change.

## 8. Decided against, recorded

- A second-precision timestamp alone: refuted by the spike (same-second
  reuse collides).
- Reusing `createdAt` by making it mandatory and immutable: it is
  caller-supplied by two builders (`build.go:163`, `claim.go:705`) and read
  as a lifecycle time by five readers (section 5); changing its meaning
  changes theirs.
- Reusing `instanceTag`: section 2.
- Refusing, rather than ignoring, a caller-supplied value at create: the
  goal record says ignore, and ignoring keeps every existing source builder
  and fixture valid without edits.
- Back-filling pre-contract records on first touch: it would change the
  digest of any episode already keyed on the empty birth, exactly the
  silent-suppression failure the consumers exist to prevent.
- A parent birth token for chain binding: a real gap, disclosed in section
  5, but a second contract change; the coordinator decides its fate.

## 9. Self-grade

**Confidence: high on the mechanism, medium on the enumeration.** The
mechanism is the spike's own passing rule, and every product line lands in
two files whose lock discipline is already the package's spine.

**Weakest claim:** that section 3's writer enumeration is complete. It was
built from a search for every `writeRecord`, `RecordCAS`, `RecordSetup`,
`RecordCreate`, `RecordProtocolError`, and `RepairClaim` call in Go, a
search for every `"agents", "jobs"` path construction in non-test Go, and a
search for every shell redirection into `$jobs/`. The steward chain-close
writer (`steward/reap.go:143-169`) was found only by reading, because it uses
`os.Rename` rather than any of those names. Another writer of that shape,
in a package the searches did not open, would have been missed.

**Reject condition:** the critic finds a shipped writer that replaces a job
record wholesale from caller-supplied bytes on a path other than
`RecordSetup`. That writer would drop the token, the immutability claim in
section 3 would be false, and the design must add a carry rule there before
the build starts.

## 10. For the coordinator

- Amend `plans/alert-channel-design.md` at its next revision: replace the
  three fallback clauses (lines 1380, 1472-1473, 2094-2096) with "empty for
  a pre-contract record" so the two consumers and this contract agree.
- Decide whether "parent birth token chain binding" (section 5, follow-up
  row) is backlogged as its own goal.
- The generic implementer preamble says "never touch `plans/`"; this brief
  named one new file in that directory as the deliverable and the
  dispatcher's sandbox allowed exactly that write. The specific instruction
  was followed.
