# Account provenance design

Goal: account-provenance (goal file at revision 3, claimed by
m0b+main-1788250419-3170380-8a1fb3). Wido's word (decision-ask 2026-08-31,
recorded in the goal): the string m0 stamps in every landing message until
this lands is "Wido@M0". Design brief:
`plans/account-provenance-design-brief.md`. Revision 1, 2026-09-02.

## Verdict up front

The account identity enters the records at BOTH surfaces, captured each
time by the runtime's own identity surface through a new per-adapter
`identity` verb: the session announcement gets an optional `account` key
written by `metasystem up` (attributing the host session's own paid
turns), and the job record gets an `account` field written by dispatch at
record composition (attributing that job's rounds, fresh at every dispatch
and follow-up). Each record is authoritative for exactly the spend it
stamps; disagreement between them is signal (a login changed between
announcement and launch), never an error to reconcile. Proof is graded,
not asserted: `claude` attests at CLI-surface strength (`claude auth
status` returns a JSON identity — verified on Claude Code 2.1.257),
`codex` attests at credential-claims strength (its CLI status names no
account; the identity claims in `~/.codex/auth.json` do — verified on
codex 0.148.0), and a runtime that can prove only "logged in" records
`unattested` honestly. Per the R-24 discipline the record claims exactly
what the surface attests — the identity logged into this runtime's CLI
configuration on this host at capture time — and never claims a billing
guarantee, because no rostered surface offers a per-call billing receipt
and nothing refuses a re-login between capture and the paid calls.

## The object under judgment: where attribution lives today

- The session announcement
  (`artifacts/agents/mains/session-*.json`, written by
  `internal/lease/verbs.go` `AnnounceWithProofAt` at line 65, struct in
  `internal/lease/classify.go`, called from `internal/up/up.go` lines
  227–240) carries `sessionId`, `mainId`, pid identity, `runtime`,
  `instanceTag`, `commandHash`, `announcedAt`, and `identityProvenance` —
  process identity and runtime, no account.
- The job record (`artifacts/agents/jobs/<job-id>.json`, composed in
  `internal/dispatch/build.go`) carries `runtime`, `requestedModel`,
  `effectiveModel`, `canonicalModelKey`, `machineId`, `capabilitySnapshot`
  and forty-odd process and protocol fields — runtime and model, no
  account.
- The account lives only in prose: landing messages and goal `Next step`
  lines hand-write "account Wido@M0" (seven goal files carry it today,
  e.g. `plans/goals/watch-verb.md:6`), and commit trailers repeat it
  ("Landed by m0b; account Wido@M0", commit af7f438c). That is the
  memory-not-records failure this goal exists to kill: the fleet's two
  capacity pools (m0/m0b on one Claude account, m1/m2 on another) are
  distinguishable only by a conduct rule humans remember to type.

## Q1 — where the identity enters, and the authority rule

Both surfaces, because they attribute different spend:

1. **Session announcement** — `metasystem up` captures the host runtime's
   account at announcement time and writes it as the optional `account`
   key. This attributes the main session's own paid turns (interactive
   work, mission host turns), the spend the hand-written landing stamp was
   actually covering.
2. **Job record** — dispatch captures the launching runtime's account at
   record-composition time, per dispatch AND per follow-up, and writes it
   as the job record's `account` field. This attributes delegate spend,
   and it is fresher than any announcement because it is taken at the
   moment the paid work is bought.

**Authority rule:** each record is authoritative for the spend it stamps —
the announcement for the host session's turns, the job record for that
job's rounds. There is no cross-record reconciliation: a host announcement
and a job record that name the same runtime but different accounts are two
true observations of a mutable login state at two times. The difference
means the operator switched accounts mid-session; surfacing that is a
dashboard's business (goals reconciliation-guards / actionable-metrics),
not this record's.

**Capture never gates dispatch.** A failed or empty identity capture
records `attestation: "unattested"` and the launch proceeds: attribution
is an observation about paid work, and an accounting probe that can veto
the work it accounts for is a new failure mode this design refuses to
create. The honest floor is the graded record, not a refusal.

## The record shape

One uniform object, identical in both surfaces:

```json
"account": {
  "attestation": "cli-surface" | "credential-claims" | "unattested",
  "accountId": "<stable provider account or organization id, or absent>",
  "accountLabel": "<human label, normally the login email, or absent>",
  "surface": "<the command or file that attested it>",
  "capturedAt": "<RFC3339 UTC>"
}
```

`attestation` grades the proof; `accountId` is the stable join key for
cost attribution; `accountLabel` is what a human reads in place of
"Wido@M0". `unattested` carries `surface` and `capturedAt` only (plus an
optional `error` string when the capture itself failed), and never carries
an id or label it did not prove — the R-24 discipline applied to the field
level.

## Q2 — proof over self-declaration, per runtime

The proof mechanism is the runtime CLI's own identity surface, exercised
at capture time — never an operator-set config key. Verified surfaces (all
run on this machine, 2026-09-02):

| Runtime | Surface | What it proves | Grade |
| --- | --- | --- | --- |
| claude (2.1.257) | `claude auth status` — exit 0, JSON on stdout with `loggedIn`, `authMethod`, `email`, `orgId`, `orgName`, `subscriptionType` | The logged-in identity by the CLI's own report | `cli-surface`; `accountId` = `orgId`, `accountLabel` = `email` |
| codex (0.148.0) | `codex login status` proves logged-in but prints no identity and has no JSON flag. The identity lives in `~/.codex/auth.json`: `tokens.account_id`, and the id-token claims `email` and `https://api.openai.com/auth`.`chatgpt_account_id` / `chatgpt_plan_type` (claim keys verified) | The identity the stored credential was issued to | `credential-claims`; `accountId` = `chatgpt_account_id`, `accountLabel` = `email` claim. The adapter extracts ONLY identity claims; token material never enters any record. API-key mode (`auth_mode` = API key, no id token) names no account and records `unattested` with the surface noted |
| devin | `devin auth status` exists (`scripts/agents/adapters/devin.sh:72`); its output shape is UNVERIFIED — this design worktree's sandbox denies devin's network egress, so the surface could not be exercised | Whatever it proves when exercised: identity fields if present, otherwise logged-in only | Adapter maps what the surface names; a yes/no surface records `unattested`. Implementation verifies the real output on a network-capable terminal before mapping |
| fake | none (fixture protocol simulator) | n/a | Fixed deterministic identity (`cli-surface`, `accountId` = `fake-account`) so fixtures exercise the full path |

**The honest bound, stated plainly (R-24):** every grade above attests
"this identity was logged into this runtime's CLI configuration, on this
host, at `capturedAt`". It does NOT attest that the provider billed that
account for the tokens this session then consumed: no rostered CLI
exposes a per-call billing receipt, and no mechanism here refuses a
re-login between capture and the paid calls. The mechanism refuses
nothing, so the record claims observation, not guarantee — that is
exactly what the `attestation` field says, and why a pure operator-set
config key (self-declaration with zero probe) is rejected below.

## Q3 — retirement of the landing conduct rule

Wido's word is "until this lands". The mechanical retirement condition:

**A landing message may drop the hand-written account clause when the
session announcement of the seat writing that landing carries an
`account` key whose attestation is `cli-surface` or `credential-claims`.**
The writer checks its own live announcement — one file read — and the
record it would have hand-stamped is now discoverable from the
announcement and from every job record of the work being landed. Until
that seat's announcement carries the key (the engine rebuilt and
`metasystem up` re-run on that machine), the "Wido@M0" stamp continues
unchanged. The conduct rule retires fleet-wide when every fleet machine's
current announcement carries the key — which happens at each machine's
next engine rebuild plus re-arm, with no coordination step. An
`unattested` capture does NOT retire the stamp: an unproven record is not
better than the honest hand-written convention it replaces.

## Q4 — per-runtime coverage: the `identity` adapter verb

The registry pattern, exactly like `probe`: each adapter grows one verb,

```
scripts/agents/adapters/<runtime>.sh identity
```

which prints the `account` JSON object above to stdout and exits 0 — also
exiting 0 with `attestation: "unattested"` when it cannot attest, because
capture never gates work. The engine owns shape validation of the printed
object (one small `adapter` verb, mirroring how `probe` hands its facts to
`adapter capability-snapshot` via `write_capability_snapshot`,
`scripts/agents/adapters/runtime-common.sh:445`); the adapter owns the
runtime-specific surface, which is where auth surfaces already live
(`claude.sh:64`, `codex.sh:60`, `devin.sh:72`). Callers:

- `internal/up/up.go` invokes the host runtime's adapter `identity` at
  announcement time and threads the result through
  `AnnounceWithProofAt`.
- `internal/dispatch` record composition invokes the dispatched runtime's
  adapter `identity` and writes the result into the job record, on every
  dispatch and follow-up round.

A new runtime lands by writing its adapter script with the verb — no
engine edit, same as probe. A rostered-but-absent runtime never reaches
the verb, because dispatch already fails on the missing adapter and
capability snapshot before identity capture would run.

## Q5 — fixtures and blast

- **The closed announcement key contract.** `internal/census/announcement.go`
  is the ONE home of the announcement key set, and `ValidateAnnouncementKeys`
  (line 45) rejects unknown keys — the census once went CENSUS-FAILED in
  production over exactly this (the file's own header comment). `account`
  is added to `AnnouncementOptionalKeys` (line 35) — optional, so legacy
  announcements stay valid, same pattern as `pidStartTicks`/`bootId`. The
  struct in `internal/lease/classify.go` gains the matching
  `omitempty` field. Tests touching the key contract:
  `internal/lease/classify_test.go`, `internal/lease/runneredge_test.go`,
  `internal/census/run_test.go`, `internal/census/prune_pair_test.go` —
  plus one new case pinning that an announcement carrying `account`
  validates.
- **The job record is additive-safe.** No closed-key validator exists for
  job records (`DisallowUnknownFields` appears nowhere in
  `internal/dispatch` or `internal/run`), and Go's default `Unmarshal`
  ignores unknown fields, so watcher, reaper, conformance, mission-fence,
  and goal-admission readers tolerate the new field; implementation
  confirms the same for `internal/supervise`. The `account` field stays
  OUT of the claim fingerprint (`internal/dispatch/claim_fingerprint.go`
  lines 220, 257 hash dispatch decision inputs): account identity is an
  observation, not a decision input, and hashing it would break
  fingerprint equality across a re-login that changes nothing about the
  dispatch decision.
- **Adapter edits re-arm supervision.** The four adapter scripts are
  inside the census fingerprint; after landing, each machine stops and
  re-arms (`metasystem up`) so the fingerprint names the code in force —
  the standing arm-once rule, called out here because this change touches
  every adapter at once.
- **Fixtures.** `fake.sh` returns the fixed identity so the validation
  suite exercises capture, record shape, and the unattested branch
  deterministically; one engine-side test pins the identity-JSON shape
  validation (bad JSON from an adapter → `unattested` with `error`, never
  a scraped repair — the malformed-output discipline).
- **Consumers gained, none broken.** The landing writer gains the Q3
  check. Cost dashboards join on `accountId` later; per the scope
  governor, nothing here builds them.

## Rejected alternatives

- **Vehicle = capability snapshot.** The brief's "probed at announcement
  like the capability snapshots" analogy holds for the MECHANISM (the
  adapter probes its own CLI) but not the VEHICLE: snapshots are immutable
  artifacts keyed by CLI version and config hash
  (`claude-2.1.257-7e93ff97…`), and a re-login moves neither key — the
  observed snapshot's `configKeyHashes` covers settings keys, not the
  credential. An account stamped there goes stale invisibly for up to
  `capability.snapshot-max-age-days`, guarded only by the operator
  discipline "re-probe after an account change" — a discipline, not a
  refusal (R-24). Launch-time capture is strictly fresher and costs one
  sub-second CLI call per dispatch.
- **Operator-set config key** (e.g. `account.label=Wido@M0` in
  `metasystem.conf`). Honest but unproven self-declaration — the
  memory-not-records failure in configuration clothing, and it silently
  survives the very event (a re-login) it exists to record. The CLI
  surfaces exist and are cheap; there is no reason to settle for
  declaration.
- **Job-record only, no announcement.** Loses attribution of the host
  session's own paid turns — interactive work and mission host turns,
  which is precisely the spend the hand-written landing stamp covered.
  The conduct rule could never retire.

## Self-grade

- Confidence 0.85 that this is the simplest durable record: two additive
  fields, one new adapter verb following the existing probe registry
  pattern, no new artifact class, no dashboard.
- **Weakest claims:** (a) the devin identity surface is unverified — the
  sandbox denies its network egress; the design bounds this (map what the
  surface proves, `unattested` floor) but implementation must exercise the
  real output before mapping; (b) codex API-key mode was not observed on
  this machine (this host is ChatGPT-auth), so its unattested branch is
  specified from the credential file's `auth_mode` key, not from a live
  example; (c) `claude auth status` JSON is a CLI-version-observed shape
  with no contract that it stays stable — drift lands on the adapter,
  whose capture degrades to `unattested` rather than guessing, and the
  probe freshness rules already force re-examination on CLI upgrades.
- **Reject this design if:** implementing it requires touching anything
  beyond the announcement key contract and struct and writer
  (`internal/census/announcement.go`, `internal/lease/classify.go`,
  `internal/lease/verbs.go`, `internal/up/up.go`), dispatch record
  composition, the four adapter scripts plus one engine identity-shape
  verb, and their tests — or if any implementation turns identity capture
  into a dispatch gate, because accounting must never veto the work it
  accounts for.

Observed discrepancy, recorded not resolved: the brief states devin is
"rostered but uninstalled"; on this machine `devin --version` answers
3000.4.25 — installed but unable to reach its service from this sandbox.
Neither state changes the design: the registry shape covers present,
absent, and unattestable runtimes identically.
