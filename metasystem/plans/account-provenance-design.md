# Account provenance design

Goal: account-provenance (goal file at revision 7, claimed by
m1+main-1788333680-2840-7f79f4). Wido's word (decision-ask 2026-08-31,
recorded in the goal): the string m0 stamps in every landing message until
this lands is "Wido@M0". Design brief:
`plans/account-provenance-design-brief.md`. Revision 2, 2026-09-02: folds
the eight material findings of
`records/misc/account-provenance-critique-r1.md` by id (fold record at the
end). Every tree reference below was read in this worktree today; every
CLI observation was run on this machine today.

## Verdict up front

The account identity enters the records at BOTH surfaces as a point
observation, captured each time by the runtime's own identity surface
through a new per-adapter verb named `account` (distinct from the existing
`identity` verb, which prints the CLI version and configuration hash and
keeps every caller it has today). The session announcement gets an optional
`account` key written by `metasystem up`; the job record gets an `account`
field written at record composition, fresh at every dispatch and follow-up
round. Each capture says exactly one thing: "surface S reported identity I
on this host at time T". No record claims an interval, a billing receipt,
or a cause for a difference between two captures; two captures that differ
are two observations.

Proof is graded by what the surface actually shows. `claude` attests at
`cli-surface` (its CLI prints a JSON identity: `claude auth status --json`,
verified on Claude Code 2.1.258). `codex` attests at `credential-claims`,
defined narrowly: the CLI reports a login, and the local credential file
holds an identity token whose decoded claims name an account; signature,
issuer trust, and the binding between the reported login and the file are
NOT verified, and the grade's definition says so (codex 0.148.0). `devin`
ships at the `unattested` floor; its surface cannot be observed from a
sandbox and the mapping above the floor is conditional on one recorded
live observation the build must report as a gap. The fake runtime returns a
fixed identity so fixtures walk the whole path.

Capture never gates work: it runs under a fixed twenty-second ceiling in
its own process group through the engine's bounded executor, expiry kills
the group, and the recorded outcome of a hang is `unattested` with the
cause `timeout`. The landing stamp retires per machine only when the
records carry usable provenance, checked mechanically at the landing
boundary and printed by the commit script.

## The object under judgment: where attribution lives today

- The session announcement (`artifacts/agents/mains/session-*.json`,
  written by `internal/lease/verbs.go` `AnnounceWithProofAt` at line 65,
  struct `Announcement` in `internal/lease/classify.go` lines 21 to 40,
  called from `internal/up/up.go` line 440) carries `sessionId`, `mainId`,
  pid identity, `runtime`, `instanceTag`, `commandHash`, `announcedAt`, and
  `identityProvenance`: process identity and runtime, no account.
- The job record (`artifacts/agents/jobs/<job-id>.json`, composed by
  `BuildRecord` in `internal/dispatch/build.go`, record map at line 365,
  and `BuildFollowRecord`, record map at line 578) carries `runtime`,
  `requestedModel`, `effectiveModel`, `mainId`, `machineId`,
  `capabilitySnapshot` and forty-odd process and protocol fields: runtime
  and model, no account.
- The account lives only in prose: landing messages and goal `Next step`
  lines hand-write "account Wido@M0" (for example
  `plans/goals/watch-verb.md:6`), and commit trailers repeat it. That is
  the memory-not-records failure this goal exists to kill: the fleet's two
  capacity pools (m0/m0b on one Claude account, m1/m2 on another) are
  distinguishable only by a conduct rule humans remember to type.

## Q1: where the identity enters, and what each capture claims

Both surfaces, because they sit nearest to different spend:

1. **Session announcement.** `metasystem up` captures the host runtime's
   account after it has resolved the session identity
   (`internal/up/up.go` line 428) and before it writes the announcement
   (line 440), and writes it as the optional `account` key. This is the
   observation nearest to the main session's own paid turns.
2. **Job record.** Record composition captures the dispatched runtime's
   account inside `BuildRecord` and `BuildFollowRecord`, once per round,
   and writes it as the record's top-level `account` field. Both verbs
   already receive `--root` from the dispatcher
   (`scripts/agents/dispatch.sh` lines 1537 and 2012), which is where the
   adapter script is found.

**What a capture claims, and nothing more.** An `account` object is a point
observation: the named surface reported the named identity on this host at
`capturedAt`. It is the best evidence available at the record's own moment,
not a receipt. The announcement's capture does not cover every later host
turn, and a job capture does not cover every later paid call of that
round, because nothing refuses a re-login between capture and the calls.
This design makes no interval claim and specifies no re-capture at paid
call boundaries: the paid calls inside a round (the turn and any return
repair turn, `runtime-common.sh` line 387) run in the adapter supervisor
after composition, and re-capturing there would put capture into the
custodian's own deadline path for a claim no consumer needs today.

**Disagreement is two observations.** An announcement and a job record
naming the same runtime but different accounts are two true observations
at two times. The records say neither "the operator switched accounts" nor
anything else about the interval between them; a consumer that wants to
interpret two observations is a reconciliation concern (goals
reconciliation-guards / actionable-metrics), out of scope here.

**Capture never gates dispatch or arming.** Every failure mode of capture
(adapter missing, CLI missing, nonzero exit, malformed output, hang) ends
in an `unattested` object with a fixed cause, and the announcement or job
record is written and the launch proceeds. The mechanism that makes this
true is specified under "Capture mechanics".

## The record shape: one closed contract

One object, identical in both surfaces, validated by ONE engine function
(`internal/account`, below) before it is written anywhere:

```json
"account": {
  "attestation": "cli-surface" | "credential-claims" | "unattested",
  "surface": "<fixed token naming the surface, see the grammar>",
  "capturedAt": "<RFC3339 UTC seconds, stamped by the engine>",
  "accountId": "<stable provider account or organization id>",
  "accountLabel": "<human label, normally the login email>",
  "error": "<fixed cause token, unattested only>"
}
```

The contract, each line a fixture (named in Q5):

- **Closed key set.** Exactly these six keys may appear; any other key
  refuses the whole object. This is the structural guarantee that
  credential material cannot ride into a record under a name the schema
  does not know.
- **Attested requires identity.** `attestation` of `cli-surface` or
  `credential-claims` requires BOTH `accountId` and `accountLabel`:
  non-empty, at most 256 bytes, printable, no control characters; and
  requires `error` absent. When a surface names a single identifier, the
  adapter puts it in both fields (stated per mapping in Q2).
- **Unattested carries no identity.** `attestation` of `unattested`
  requires `accountId` and `accountLabel` absent and `error` present.
- **`error` is a cause token, never raw text.** Exactly one of: `timeout`,
  `runtime-unknown`, `adapter-missing`, `adapter-failed`,
  `malformed-output`, `cli-missing`, `not-logged-in`, `api-key-mode`,
  `identity-incomplete`, `credential-unreadable`, `credential-invalid`,
  `credential-expired`, `surface-unmapped`. Adapter stderr is never stored
  anywhere; an operator reproduces a cause by running the adapter verb by
  hand.
- **`surface` is a fixed token**, grammar `^[a-z0-9+-]{1,64}$`, one per
  adapter path (Q2). Free text is refused.
- **`capturedAt` is stamped by the engine** from its own clock at capture
  completion, format `2006-01-02T15:04:05Z`. An adapter object carrying
  `capturedAt` is refused as an unknown key, so an invalid timestamp
  cannot enter a record from an adapter.
- **Refusal degrades, never repairs.** Any violation makes the engine
  substitute `{"attestation":"unattested","surface":"adapter-account-verb",
  "error":"malformed-output","capturedAt":…}`. Nothing is scraped from a
  refused object.

`attestation` grades the proof; `accountId` is the stable join key for
cost attribution; `accountLabel` is what a human reads in place of
"Wido@M0". The R-24 discipline applies at the field level: a record never
carries an id or label its surface did not name.

## Q2: proof over self-declaration, per runtime

The proof mechanism is the runtime CLI's own identity surface, exercised
at capture time, never an operator-set configuration key. Observed
surfaces, run on this machine on 2026-09-02:

| Runtime | Surface and observation | Mapping and grade |
| --- | --- | --- |
| claude 2.1.258 | `claude auth status --json`: exit 0, JSON on stdout with `loggedIn`, `authMethod`, `apiProvider`, `analyticsDisabled`, `projectsDirectory`, `email`, `orgId`, `orgName`, `subscriptionType`. `--json` and `--text` are the documented flags; JSON is the default, the adapter passes `--json` explicitly. | `cli-surface`, `surface` = `claude-auth-status`, `accountId` = `orgId`, `accountLabel` = `email`. Exit nonzero or `loggedIn` false: `not-logged-in`. Output not JSON: `malformed-output`. `loggedIn` true but `orgId` or `email` empty (API-key or third-party provider mode names no account): `identity-incomplete`. |
| codex 0.148.0 | `codex login status` prints "Logged in using ChatGPT" and nothing else; `--help` shows no output-format flag. The identity lives in `${CODEX_HOME:-~/.codex}/auth.json`: top-level keys `OPENAI_API_KEY`, `auth_mode` (observed `chatgpt`), `last_refresh`, `tokens`; `tokens` keys `access_token`, `account_id`, `id_token`, `refresh_token`. The `id_token` payload decodes to claims including `iss` (`https://auth.openai.com`), `aud`, `exp`, `iat`, `email`, `email_verified`, `sub`, and the object `https://api.openai.com/auth` with `chatgpt_account_id`, `chatgpt_plan_type`, `chatgpt_user_id`, `organizations`, `user_id`. | `credential-claims` as defined below, `surface` = `codex-login-status+auth-file`, `accountId` = `chatgpt_account_id`, `accountLabel` = `email`. Login status nonzero: `not-logged-in`. File unreadable: `credential-unreadable`. `auth_mode` other than `chatgpt`: `api-key-mode`. Payload not decodable, or `iss` not `https://auth.openai.com`, or `exp` missing: `credential-invalid`. `exp` before the engine's now: `credential-expired`. Claims present but either identity claim empty: `identity-incomplete`. |
| devin 3000.4.25 | `devin auth status` takes no options (`--help` verified; `--format json` is rejected). Run from this sandbox it opens a connection to `server.codeium.com:443` (denied by the sandbox) and panics before printing anything because it cannot create its rolling log file. Its success output is UNOBSERVED. | Floor: `unattested`, `surface` = `devin-auth-status`, `error` = `not-logged-in` on nonzero exit and `surface-unmapped` on exit 0. Conditional mapping above the floor is specified below and is NOT built until the named observation exists. |
| fake | none (fixture protocol simulator) | `cli-surface`, `surface` = `fake-fixed`, `accountId` = `fake-account`, `accountLabel` = `fake@example.invalid`; the environment knob `METASYSTEM_FAKE_ACCOUNT` selects `unattested` (prints `not-logged-in`), `hang` (spawns a sleeping child and never returns), `malformed` (prints an attested object with an extra `token` key), or `timestamp` (prints an object carrying `capturedAt`), for the fixtures in Q5. |

**The codex grade, stated honestly.** `credential-claims` means exactly:
the CLI reported a login (exit 0 of `codex login status`), AND the local
credential file's identity token decoded to claims naming this account,
with issuer string and expiry checked locally. It does NOT mean the token
signature was verified (no key fetch, no network), does NOT mean the token
is the one the CLI will present for the paid calls, and does NOT mean the
reported login is bound to that file (the status command names no account,
so equality cannot be checked). The file is mutable by anything with the
user's permissions. The verification that would earn `cli-surface` for
codex is a CLI surface that itself prints the account (a `codex login
status` that names the account, or a JSON flag); the adapter upgrades only
when that surface exists, never by verifying the file harder. Token
material (`access_token`, `refresh_token`, `OPENAI_API_KEY`, the raw
`id_token`) never leaves the engine helper that decodes the claims, and the
closed key set makes a leak into a record a refused object.

**The devin mapping, resolved conditionally with a floor.** Version 1
ships the floor above: the devin `account` verb runs `devin auth status`
under the engine ceiling and records `unattested` whatever it prints, with
the cause by exit code. The mapping above the floor is decided now and
enabled only after one live observation:

- Required observation, which the build reports as a gap in its return
  rather than performing: on a network-capable terminal where the CLI is
  logged in, run `devin auth status`, and record exit code, stdout, and
  stderr with the email redacted, in
  `records/misc/devin-auth-status-observation.md` (an ordinary record,
  no new class), together with the CLI version.
- Mapping once observed: if the output names an organization id and an
  email, `cli-surface`, `accountId` = organization id, `accountLabel` =
  email. If it names only an email, `cli-surface` with the email in both
  fields. If it names only "logged in", the floor stays and the cause
  becomes `surface-unmapped` permanently for that CLI version.
- Rejected as a source: `devin.org_id` in
  `~/.config/devin/config.json` (visible as a hashed key in the devin
  capability snapshot). It is an operator-writable configuration value,
  the self-declaration this design refuses.

**The honest bound, stated plainly (R-24):** every grade attests "this
identity was named by this surface, on this host, at `capturedAt`". No
grade attests that the provider billed that account for the tokens the
session then consumed: no rostered CLI exposes a per-call billing
receipt, and no mechanism here refuses a re-login between capture and the
paid calls. The mechanism refuses nothing, so the record claims
observation, not guarantee.

## Capture mechanics: the `account` adapter verb and the engine capture

**The adapter verb, and why it is not `identity`.** Every adapter already
has a verb named `identity` that prints `<cliVersion> <configHash>` on one
line (`claude.sh` lines 50 to 56 and 214, `codex.sh` 46 to 52 and 211,
`devin.sh` 58 to 64 and 808, `fake.sh` 337 to 342). Its callers, each
verified unchanged by this design:

- `internal/adapter/selftestrun.go` line 181 runs `identity` for its exit
  status only.
- `scripts/validate-metasystem.sh` line 1225 asserts every
  common-lifecycle adapter's usage text advertises `identity`.
- `scripts/agents/adapters/fake.sh` line 355 runs its own `identity` in
  `selftest`.
- `scripts/agents/dispatch.sh` `select_snapshot` (line 757) reads
  `config-identity`, the JSON sibling, and `metasystem config identity`
  (`cmd/metasystem/main.go` line 66) is the engine verb behind it. Both are
  a different namespace and are untouched.

The account verb is therefore named `account`:

```
scripts/agents/adapters/<runtime>.sh account
```

It prints the account object WITHOUT `capturedAt` to stdout and exits 0
whenever it produced an object, including an `unattested` one. Nonzero
exit is itself a recorded outcome (`adapter-failed`), never a gate. The
usage text of all four adapters gains the line, and
`scripts/validate-metasystem.sh` line 1225 gains `account` in its verb
list; the adapter-population loop at line 1242 gains the same usage check
so the fake adapter is covered too.

The runtime-specific work stays in the adapter and its Go seam file, where
auth surfaces already live (`claude.sh` 64, `codex.sh` 60, `devin.sh` 72):

- `claude.sh account`: runs `claude auth status --json`, reads fields with
  `$ms json get --value`, prints through the object builder below.
- `codex.sh account`: runs `codex login status`, then calls
  `metasystem adapter codex-account --auth-file <path>`, a seam verb in
  `internal/adapter/codex.go` (the file that already carries the codex
  seam) that reads the file, decodes the identity-token payload, applies
  the checks in Q2, and prints the object. Decoding lives in Go so the
  secrecy fixture can pin it.
- `devin.sh account`: runs `devin auth status`, prints the floor object.
- `fake.sh account`: prints by `METASYSTEM_FAKE_ACCOUNT`.
- All four print through `metasystem adapter account-object --attestation
  A --surface S [--account-id X] [--account-label Y] [--error E]`, a seam
  verb that validates against the contract and emits correct JSON, so no
  adapter assembles JSON by hand. Both verbs are members of the existing
  `adapter` family (the family `capability-snapshot`, `devin-config`, and
  `devin-session` belong to), not new top-level surface.

**The engine capture: one function, one ceiling, one kill path.** A new
dependency-leaf package `internal/account` (imports only
`internal/boundedexec`, which imports only `internal/config`; neither
`config` nor `boundedexec` imports `lease`, `up`, `dispatch`, or `landing`,
so every caller below stays acyclic) owns:

- the `Account` struct with `omitempty` on `accountId`, `accountLabel`,
  `error`;
- `Validate(raw []byte) (Account, cause)` implementing the closed contract;
- `Capture(metasystemRoot, runtime string, bound boundedexec.Bound, now
  func() time.Time) Account`, which never returns an error.

`Capture` does, in order: runtime empty or `unknown` (the value
`internal/up/up.go` line 257 substitutes when the runtime is unresolved)
gives `runtime-unknown`; adapter path
`<metasystemRoot>/scripts/agents/adapters/<runtime>.sh` missing or not
executable gives `adapter-missing`; otherwise it runs `<adapter> account`
through `boundedexec.Run` (`internal/boundedexec/boundedexec.go` line 84)
with `boundedexec.FixedBound(20*time.Second, "account-capture-ceiling")`.
`boundedexec.Run` places the child in its own process group (`Setpgid`,
line 92), sends `SIGKILL` to the whole group on expiry (line 107), and
bounds the reap by a five-second grace window (line 113): the same
group-kill primitive the engine already uses for git, gates, and the
supervision fingerprint, and the engine-side counterpart of the adapter
custodian's `sweep_kill_domain` (`runtime-common.sh` line 235). Stdout is
read into a 64 KiB bounded buffer (overflow is `malformed-output`); stderr
is discarded. Expiry gives `timeout`; nonzero exit gives `adapter-failed`;
otherwise the output goes through `Validate`. The engine then stamps
`capturedAt` from `now()`.

The ceiling is a hang bound, not a speed assertion: the claude and codex
surfaces answer locally in well under a second, and devin's is the one
network-dependent surface. Twenty seconds keeps a hung capture below one
census interval (`watch.interval-sec=60` in `metasystem.conf`). Stated
exposure, not hidden: the capture child carries a runtime signature
(`claude`, `codex`, or `devin` in argv), so a capture that overlaps a
census scan is reported `UNTRACKED` for at most one scan, the same
exposure the capability probe's `claude auth status` (`claude.sh` line 64)
already carries today. No configuration key is added; the ceiling is a
constant named in the failure text by its key string.

**Callers, exactly two:**

- `internal/up/up.go`: after `resolveSessionIdentity` (line 428), `acct :=
  account.Capture(installationRoot(options), session.Runtime, bound,
  time.Now)`; `lease.AnnounceWithProofAt` (`verbs.go` line 65) gains a
  trailing `*account.Account` parameter and writes it as the announcement's
  `account` key; `AnnounceWithProof` and `AnnounceWithPair` (lines 53 and
  59, direct lease callers) pass nil. `up` appends one component outcome
  `component=account outcome=<attestation> detail=label=<accountLabel>
  surface=<surface>` (or `detail=cause=<error>`), and this component is
  never `failed`.
- `internal/dispatch/build.go`: `BuildRecord` adds `"account":
  account.Capture(p.Root, p.Runtime, bound, time.Now)` to the record map
  at line 365; `BuildFollowRecord` adds the same at line 578 with the
  runtime read from the parent record. An empty `Root` yields
  `adapter-missing`, not an error. The `account` field stays OUT of the
  claim fingerprint's hashed inputs (`internal/dispatch/claim_fingerprint.go`):
  it is an observation, not a decision input, and hashing it would break
  fingerprint equality across a re-login that changes nothing about the
  dispatch decision.

## Q3: retirement of the landing conduct rule

Wido's word is "until this lands". The stamp retires only when the records
carry usable provenance, checked on the records themselves, not on one
enum:

**Condition.** For the seat writing the landing: (a) its current session
announcement carries `account` with `attestation` other than `unattested`
and both `accountId` and `accountLabel` present; and (b) every job record
this session has composed since it announced, meaning every record under
`artifacts/agents/jobs` whose `mainId` equals the announcement's `mainId`
(`BuildRecord` writes `mainId` at line 387, `BuildFollowRecord` at line
604), plus every member of the closed chain being landed when one is
declared, carries `account` satisfying the same test. A seat with an
attested announcement and zero jobs satisfies (b) vacuously.

**Who checks it, where.** The landing evaluator.
`internal/landing/observe.go` `Observation` (line 49) gains an additive
field `accountStamp` with `{"required": bool, "code": <token>, "record":
<job id, announcement basename, or empty>}`; `schemaVersion` is unchanged
because `commit.sh` reads named fields only (lines 307 to 310).
`Observe` computes it by resolving the calling main through the lease
classifier's ancestry walk (`lease.ClassifyAt`, `internal/lease/classify.go`
line 315, the route `up` uses at `up.go` line 449) from the root the
commit script already passes (`commit.sh` line 4 defines `root` as the
metasystem root, which holds `artifacts/agents`), reading the announcement
it names, enumerating job records by `mainId`, and walking chain membership
with `usage.RootJobID` (`internal/usage/usage.go` line 47; `usage` imports
only `atif`, `atomicfile`, and `wiredoc`, so the import is acyclic). Codes,
fixed: `no-main-announcement` (caller is not an announced main, including
human commits), `announcement-unattested`, `job-unattested` (with the
first offending job id in sorted order), `stamp-retired`. The values never
echo caller text, as the `Observation` contract requires.

`scripts/agents/commit.sh` prints one line after the observation (line
318): `account stamp: required (<code> <record>)` or `account stamp:
retired`. It never refuses on it: the stamp is prose, and strictness
guards invariants, not conventions.

**The conduct rule, mechanically.** On a machine, the hand-written
account clause is dropped from landing messages written after the first
landing whose commit output printed `account stamp: retired`, and returns
for as long as a landing prints `required`. Until that machine's engine is
rebuilt and `metasystem up` re-run so its announcement carries the key,
every landing prints `required` and "Wido@M0" continues unchanged. No
fleet-wide coordination step exists or is needed: each machine crosses at
its own first retired landing.

## Q4: per-runtime coverage, and what the registry actually requires

Two different acts, which revision 1 conflated:

- **Account capture for an already-registered runtime** (claude, codex,
  devin, fake) is one adapter-script change: the `account` verb, plus for
  codex the seam verb in `internal/adapter/codex.go`. The engine reaches
  the adapter by runtime name and never names a runtime in
  `internal/account`. No registry edit.
- **Registering a genuinely new runtime** is an engine edit by the
  registry's own contract: one `Declaration` in
  `internal/runtimes/runtimes.go` (the universe at lines 161 to 242, "the
  ONE declaration of the agent-runtime universe", header lines 1 to 12)
  plus that runtime's seam files, and `internal/config/validate.go` lines
  121 to 139 refuses any roster name outside that universe before a
  dispatch could reach an adapter. A runtime that is registered and
  rostered but whose adapter lacks the verb records `adapter-failed`
  (usage error, exit 2) and still launches; the validation suite's verb
  check (line 1225) is what catches the omission before it ships.

The registry declares no account capability: `ExpectedCapabilities` lists
Go seam-local tables (`delivery-recollection`, `usage-recovery`,
`selftest-probe`, `runtimes.go` lines 147 to 152), and account capture is
an adapter-script verb reached by name, so nothing in `runtimes.go`
changes for this design.

## Q5: fixtures and blast

- **The closed announcement key contract.** `internal/census/announcement.go`
  is the ONE home of the announcement key set, and
  `ValidateAnnouncementKeys` (line 45) rejects unknown keys; the census
  once went CENSUS-FAILED in production over exactly this (the file's own
  header). `account` is added to `AnnouncementOptionalKeys` (line 34),
  optional so legacy announcements stay valid, the `pidStartTicks`/`bootId`
  pattern. The `Announcement` struct (`internal/lease/classify.go` line 21)
  gains `Account *account.Account` with `omitempty`; `lease` importing
  `account` is acyclic (Q2 mechanics). Tests touching the key contract:
  `internal/lease/classify_test.go`, `internal/lease/runneredge_test.go`,
  `internal/census/run_test.go`, `internal/census/prune_pair_test.go`, plus
  one new case in `internal/lease/classify_test.go`
  (`TestAnnouncementCarriesAccount`): an announcement written through
  `AnnounceWithProofAt` with an attested account round-trips and validates.
- **The job record is additive-safe.** No closed-key validator exists for
  job records: `DisallowUnknownFields` appears in thirteen files (grep
  today), none in `internal/dispatch` or `internal/run`, and the one in
  `internal/supervise` (`acknowledged.go` line 79) decodes the
  acknowledged-processes record, not a job record. Go's default
  `Unmarshal` ignores unknown fields, so watcher, reaper, conformance,
  mission-fence, and goal-admission readers tolerate the new field.
- **Semantic and secrecy fixtures, Go, under the go gate,
  `internal/account/account_test.go`:**
  `TestValidateAttestedRequiresBothIdentifiers` (attested with either
  identifier missing becomes `identity-incomplete`);
  `TestValidateUnattestedRefusesIdentity` (unattested carrying an id is
  refused, `malformed-output`); `TestValidateRefusesUnknownKeys` (an extra
  `token` key is refused and its value is absent from the result);
  `TestValidateRefusesAdapterTimestamp` and `TestCaptureStampsCapturedAt`
  (adapter-supplied `capturedAt` refused; engine stamp matches
  `2006-01-02T15:04:05Z`); `TestValidateErrorMustBeCauseToken` (free text
  in `error` refused); `TestCaptureNeverStoresAdapterStderr` (a script
  adapter prints a marker on stderr and exits 1; the result is
  `adapter-failed` and the marker appears nowhere in the JSON);
  `TestCaptureTimeoutKillsAdapterGroup` (a script adapter spawns a
  sleeping child then hangs; with a one-second `FixedBound` the result is
  `timeout` within the grace window and `kill(-pgid, 0)` reports the
  group gone); `TestCaptureMissingAdapterIsUnattested`.
- **Codex claims fixture, Go, `internal/adapter/codexaccount_test.go`:**
  synthetic `auth.json` files with an unsigned identity token: chatgpt mode
  with valid claims gives `credential-claims`; `auth_mode` of `apikey`
  gives `api-key-mode`; expired gives `credential-expired`; wrong issuer
  gives `credential-invalid`; missing email gives `identity-incomplete`;
  and `TestCodexAccountNeverPrintsTokenMaterial` asserts the marker values
  planted in `access_token`, `refresh_token`, `OPENAI_API_KEY`, and the raw
  token do not appear in the verb's output.
- **Retirement fixture, Go, `internal/landing/observe_test.go`:**
  `TestAccountStampRetiresOnlyWithAttestedRecords` over a fixture mains and
  jobs directory: announcement unattested gives `required`
  `announcement-unattested`; one job with the announcement's `mainId`
  unattested gives `required` `job-unattested` naming it; all attested
  gives `retired`.
- **Protocol fixture, shell, `scripts/agents/dispatch-fixtures.sh` (run by
  `scripts/validate-metasystem.sh` line 2376):** a fake dispatch's job
  record carries `account` with `cli-surface` and `fake-account`; with
  `METASYSTEM_FAKE_ACCOUNT=unattested` the record carries `unattested`
  `not-logged-in` and the dispatch still launches and completes. The
  `hang` case stays in the Go test above, where the bound is injectable;
  the shell fixture does not wait out a twenty-second ceiling.
- **Live proof of the announcement wiring.** No shell fixture drives
  `metasystem up` end to end today (grep of `scripts/agents/*fixtures*.sh`
  for `up --repo` finds only remedy strings in `health-fixtures.sh`). The
  `up` wiring is proven at compile time by the changed
  `AnnounceWithProofAt` signature and by the orchestrator running
  `metasystem up` on this machine after the build and reading the
  announcement's `account` key; that run is recorded as evidence in the
  implementation return, not claimed from a delegate sandbox.
- **Adapter edits re-arm supervision.** The four adapter scripts are
  inside the census fingerprint; after landing, each machine stops and
  re-arms (`metasystem up`) so the fingerprint names the code in force,
  the standing arm-once rule.
- **Consumers gained, none broken.** `commit.sh` gains one printed line.
  Cost dashboards join on `accountId` later; per the scope governor,
  nothing here builds them.

## Rejected alternatives

- **Extending the `identity` verb's output.** Its output is one
  space-separated line, `<cliVersion> <configHash>`, and its callers treat
  it as exit status or as two tokens. Appending a JSON object to a
  line-oriented output, or replacing it, forces every caller into a
  compatibility decision the tree does not document. A distinct verb costs
  one usage line per adapter.
- **Vehicle = capability snapshot.** The brief's "probed at announcement
  like the capability snapshots" analogy holds for the MECHANISM (the
  adapter probes its own CLI) but not the VEHICLE: snapshots are immutable
  artifacts keyed by CLI version and config hash (`claude-2.1.258-d837f384…`,
  `internal/adapter/snapshot.go` line 62), and a re-login moves neither
  key: the snapshot's `configKeyHashes` cover settings keys, not the
  credential. An account stamped there goes stale invisibly for up to
  `capability.snapshot-max-age-days`, guarded only by operator discipline.
- **Operator-set config key** (`account.label=Wido@M0` in
  `metasystem.conf`), and `devin.org_id` in devin's config file. Honest but
  unproven self-declaration: the memory-not-records failure in
  configuration clothing, and it silently survives the very event (a
  re-login) it exists to record.
- **Verifying the codex token signature** to earn a higher grade. It needs
  a network key fetch inside a capture that must never gate work, and it
  would still not bind the file to the login the CLI reported. The honest
  move is the narrower grade definition, not a heavier check.
- **Job-record only, no announcement.** Loses the observation nearest to
  the host session's own paid turns, which is precisely the spend the
  hand-written landing stamp covered. The conduct rule could never retire.
- **Re-capture at every paid call boundary** to support an interval claim.
  It would push capture into the adapter custodian's deadline path for a
  claim no consumer needs; the point-observation record is enough for
  attribution and is honest.

## Fold record: finding id to fold

| Finding | Fold |
| --- | --- |
| account-provenance-r1-identity-verb-collision | The verb is named `account`; the four existing `identity` implementations and their callers (`selftestrun.go` 181, `validate-metasystem.sh` 1225, `fake.sh` 355, `dispatch.sh` 757 via `config-identity`, `metasystem config identity`) are enumerated and untouched. Usage text and the suite's verb list gain `account`. |
| account-provenance-r1-codex-proof-bound | `credential-claims` is defined as exactly "login reported by the CLI plus claims decoded from the local file with issuer and expiry checked locally"; signature, binding, and validity beyond expiry are stated as not proven; the surface that would earn `cli-surface` is named; token material is confined to the Go seam verb and pinned by fixture. |
| account-provenance-r1-authority-and-disagreement | "Authoritative for the spend it stamps" and "the operator switched accounts" are removed. Each capture is a point observation; a difference is two observations; no interval claim is made and the re-capture that would justify one is explicitly not built, with the reason. |
| account-provenance-r1-retirement-condition | The condition is on the records: an attested announcement with both identifiers and the same on every job record with that `mainId` plus the declared chain. Checked by the landing evaluator (`accountStamp` on the observation), printed by `commit.sh`; per-machine retirement rule stated. |
| account-provenance-r1-non-gating-time-bound | `internal/account.Capture` runs the adapter through `boundedexec.Run` with a fixed twenty-second bound: own process group, group `SIGKILL` on expiry, bounded reap; outcome `unattested`/`timeout`; stderr never stored; the census exposure of a hung child is stated. Applies to all four adapters including devin. |
| account-provenance-r1-runtime-registry-coverage | Capture for a registered runtime is adapter-only; a new runtime is a `runtimes.go` Declaration plus seam files with `config/validate.go` refusing others; the registry declares no account capability; the reject condition is restated accordingly. |
| account-provenance-r1-semantic-validation-fixtures | The object is a closed six-key contract with attested-requires-identity, unattested-carries-none, cause-token errors, fixed surface tokens, and engine-stamped timestamps; nine Go fixtures in `internal/account`, six in `internal/adapter`, one in `internal/landing`, one lease case, and one shell case are named with their gates. |
| account-provenance-r1-devin-unresolved-mapping | Devin ships at the `unattested` floor with fixed causes; the conditional mapping (org id and email, email only, logged-in only) is decided here; the one live observation and its record path are named as a gap the build reports; `devin.org_id` is rejected as a source. |

## Self-grade

- **Confidence 0.8** that this is the simplest durable record: two
  additive fields, one new adapter verb following the existing verb
  pattern, two seam verbs in the existing `adapter` family, one leaf
  package, one additive field on the landing observation, no new artifact
  class, no dashboard, no new configuration key.
- **Weakest claims:** (a) the devin surface is unobserved; the design
  bounds it at the floor and names the observation, but the conditional
  mapping may still not fit what the CLI prints, in which case the
  follow-up slice returns to design; (b) the retirement check relies on
  the landing evaluator resolving the calling main by ancestry from
  `commit.sh`; the same route already gates the commit
  (`lease require-holder --caller-pid`, `commit.sh` line 9), but if the
  policy engine is ever invoked outside the seat's process tree the check
  reports `no-main-announcement` and the stamp simply stays; (c) the
  twenty-second ceiling is a chosen constant; a slow but healthy surface
  records `timeout` rather than an identity, which is honest but costs an
  attested record; (d) `claude auth status --json` and the codex
  credential file are CLI-version-observed shapes with no stability
  contract; drift lands on the adapter, whose capture degrades to
  `unattested` rather than guessing.
- **Reject this design if:** implementing it requires touching anything
  beyond the announcement key contract, struct, and writer
  (`internal/census/announcement.go`, `internal/lease/classify.go`,
  `internal/lease/verbs.go`, `internal/up/up.go`), the two record
  composers in `internal/dispatch/build.go`, the new `internal/account`
  package, the two `adapter`-family seam verbs and `internal/adapter/codex.go`,
  the four adapter scripts, the landing observation and one `commit.sh`
  line, the suite's verb list, and their tests; or if any implementation
  turns capture into a dispatch or arming gate; or if a registered
  runtime's capture turns out to need a `runtimes.go` edit; or if a
  fixture has to store adapter stderr or token material to pass.

Observed discrepancy, recorded not resolved: the brief states devin is
"rostered but uninstalled"; on this machine `devin --version` answers
3000.4.25 (build 7e8e528a), installed but unable to reach its service or
create its log file from this sandbox. Neither state changes the design:
the verb covers present, absent, and unattestable runtimes identically.
