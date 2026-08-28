# Devin, from written-down to proven

- Goal and current status: make Devin a full citizen of the metasystem —
  delegate adapter proven against the real CLI, a host adapter so Devin can run
  mission turns, and a benchmark run with Devin delegates. The delegate adapter
  was written from documentation and had never executed a single verb. It has
  now, and the live runs contradict several of its assumptions — including one
  that matters more than the rest: on this account Devin cannot confine a
  delegate at all.
- Next step: none
  implemented, reviewed, gate-green, and pushed (origin bc21214). The bm-2
  cohort is provisioned and stopped at its seal/sign boundary.
- Waiting on the human: nothing — the bm-2 cohort was signed, ran, and closed at 2/2 valid (D70)
  bm-2-20260808t170805z-35043 is staged; seal and sign its contract, then
  `benchmark/run-cohort.sh --resume bm-2-20260808t170805z-35043` runs the
  repetition. The kit is designed to stop here; nothing routes around it.
- Design status: closed. The successor chain returned 3 material findings after
  the round-3 rescope (9, 9, 12, then 3); all fixed. The implementation review
  ran four rounds (11, 7, 4, 2) plus three confirming rounds; every finding was
  real and is fixed or recorded (KI-26..KI-29).
- In flight right now: nothing
- Waiting on the human: nothing. The containment question was decided on
  2026-08-08 — see "The ruling" below.

## The ruling, and the correction that produced it

Three critique rounds returned 9, 9, and 12 material findings. A count that does
not fall across two rounds is this system's own signal to split or rescope
rather than buy a fourth round (IL-23), and the findings agreed on where the
seam was: a read-only Devin delegate was specified and provable, while a
write-capable one rested on containment that did not exist.

DEVIN-R3-004 forced the correction, and it was right. An earlier version of D-9
said that a benchmark target being a disposable checkout supplies the
containment Devin lacks. That is FALSE. The escape in O-9 is a shell command,
and nothing about a target being disposable stops it reaching sibling
repositories, the evidence store, temporary files, or credentials. The sentence
was reassuring and untrue, and it is deleted rather than softened.

Put to the human with that correction, the decision on 2026-08-08 was to accept
the uncontained runtime, for a reason the metasystem cannot verify and does not
claim to: "I will accept this since I aim to run any of this inside a VM
anyways." A virtual machine IS the boundary a shell escape cannot cross, which
is exactly what a disposable directory is not. The containment is therefore
real, it is the human's, and it lives outside this system — so this design
declares the runtime uncontained and records who accepted that, instead of
declaring a boundary it does not enforce.

This is not a Devin exception. Part 9 of `plans/agent-orchestration-design.md`
already rules, from 2026-08-04, that a same-user agent cannot be locally
contained and that the operator supplies the privilege domain — a container, a
VM, or a separate user — which the metasystem neither builds nor verifies nor
substitutes for. Devin does not break that rule; it makes it visible. Claude and
codex gate tools more finely, which reads like containment and is not: none of
the three is contained without the operator's domain. What is specific to Devin
is only that its declaration must say so, because its scopes look like
boundaries and are not.

What follows from the ruling: `writeRoots` is waived for devin GLOBALLY rather
than per purpose, because the acceptance is global. That also settles
DEVIN-R3-002, which found the waiver mechanism unable to express a
benchmark-specific, human-signed exception: no such expression is needed for a
global ruling. The snapshot still declares every envelope member unenforced, so
a reader of any Devin job record sees what was true.

## What the live CLI actually does

Every claim below was observed on this machine against `devin 3000.3.27`,
model `swe-1-7`, in a scratch git repository. They are facts, not readings of
the documentation, and several contradict what the adapter assumes.

O-1. A turn runs as `devin -p --prompt-file F --respect-workspace-trust false
--model M --permission-mode MODE --export T`: exit 0, the reply on stdout, the
conversation exported to `T`.

O-2. `--sandbox` FAILS on this account: `session/set_mode failed: Invalid
params: "Mode 'autonomous' is restricted by your organization's policy"`. It
fails alone and with `--config`. The adapter passes it on every dispatch, so no
Devin dispatch could ever have run here.

O-3. `--permission-mode autonomous`, which the adapter passes, is not a mode the
CLI offers. The modes are `auto`, `accept-edits`, `smart`, `dangerous`.

O-4. `--config FILE` REPLACES the user configuration rather than layering on it,
and each new config path re-runs onboarding, printing a "Welcome to Devin CLI!"
banner onto the turn's stdout. The real user file holds five keys: `version`,
`devin.org_id`, `shell.setup_complete`, `theme_mode`, `agent.model`.

O-5. The exported transcript carries `session_id`, `agent.model_name` (the model
that answered, e.g. "SWE-1.7"), `agent.version`, and `final_metrics` with
`total_prompt_tokens`, `total_completion_tokens`, `total_cached_tokens`,
`total_steps`.

O-6. Resume works and preserves identity: `-r <session-id>` continues the
session and the next export carries the same `session_id`.

O-7. `devin list --format json` reports sessions for the current directory as
objects keyed `id`, `short_id`, `working_directory`, `last_activity_at`,
`title`. There is no `session_id` or `sessionId` member — the adapter's parser
looks for exactly those and would find nothing. It works while a turn is still
running, which the export does not.

O-8. Writes happen under `--permission-mode accept-edits` when the config allows
`edit`, `exec`, and `Write(<root>/**)`. With those denied, a turn asked to write
ends with exit 0 and an empty reply rather than an error.

O-9. **`exec` is not bounded by `Write(...)` scopes.** With the config allowing
`exec` and `Write(<scratch>/**)` only, Devin ran `sh -c "echo escaped >
/tmp/claude-501/escape.txt"`, reported exit status 0, and the file exists
outside the declared write root.

O-10. **`Read(...)` scopes do not bound reads.** With `Read(<scratch>/**)` the
only read grant and `edit`, `exec`, and `Write(**)` denied, Devin read
`/tmp/claude-501/outside-secret.txt` and returned its contents verbatim.

O-11. `final_metrics` is CUMULATIVE for the session, not per turn: the first
turn reported 12833 prompt and 44 completion tokens; the resumed second turn
reported 25799 and 81 — the session totals, not that turn's.

O-12. A turn that cannot do what it was asked ends with exit 0 and no reply.
Observed three times: writes denied (O-8), asked to write outside the granted
scope, and — the case that matters most for the self-test — a read-only turn
asked to ATTEMPT a forbidden write and then return JSON produced no reply at
all. On this runtime a denial ends the turn; it does not become a report.

O-17. Devin is handed no schema. The other two adapters pass one to their CLI
(`--json-schema`, `--output-schema`); this CLI has no such flag, and the
dispatcher's prompt describes the return in prose. A real design-critic turn
came back with `description` where the schema names `command`, `observed`, and
`level`, and the protocol-error mechanism caught it. Prose is not a schema for
a runtime that was never shown one.

O-16. Reported by the human, 2026-08-08, from their own environment: an
ENTERPRISE Devin does not report token usage at all. It reports ACU, which is a
different unit — not a token count, not a currency. The metasystem must not
break on it. This account is not enterprise, so the exact key is not observable
here; the design handles the shape rather than guessing the name.

O-15. Denying `exec` makes a Devin delegate unusable, not merely read-only. A
design-critic turn with `exec` denied read files, reached for a shell command
to find a skill, was denied, and the turn ENDED with no reply — the O-12
pattern, reached mid-work rather than on the first instruction. A trivial
prompt with the same config answers fine, so the failure is not deterministic
from the config alone: it fires whenever the agent chooses a shell, which for
real work is most turns.

O-13. A config built by COPYING the user file and adding a permissions block,
written to a new path, prints no banner: the turn's entire stdout was its
reply. The banner in O-4 comes from a config that lacks the onboarding marker,
not from the path being new.

O-14. A project-level `.devin/config.json` granting `edit`, `exec`, and
`Write(**)` did NOT widen an envelope supplied by `--config`: the turn was
asked to create a file, created nothing, and produced no reply. `--config`
replaces the configuration rather than being layered over by project files.

## D-1. The envelope is declared as what it is, and the gate is made to act on it

Without `--sandbox` — which this organisation refuses (O-2) — Devin's
permission config gates which TOOLS exist, not which paths they may touch.
O-9 and O-10 prove both directions of that: a shell command escapes the write
root, and a read escapes the read root.

So the capability snapshot declares `writeRoots: notEnforced`, `readRoots:
notEnforced`, `network: notEnforced`, and lists both escapes under
`permissions.unverified` with the commands that demonstrated them. This is the
opposite of the current declaration, which claims all three are mapped.

Permission modes are still mapped, because they still decide what the agent may
attempt: a role with no write roots runs `auto`, a role with write roots runs
`accept-edits`, and `dangerous` is never used.

`exec` is granted to every Devin delegate, including the ones with no write
roots. That is not a preference: with `exec` denied, a delegate that reaches
for a shell mid-task is denied and its turn ends with no reply at all (O-15),
which makes the runtime unusable for real work rather than safely read-only.
`edit` and the `Write` scopes still follow the requested write roots, so the
distinction the envelope draws is preserved where it can be — but a delegate
with `exec` can write through a shell (O-9), so a "read-only" Devin delegate is
read-only by intent and not by enforcement. That is the same residual the human
accepted globally, and it is why the snapshot declares every envelope member
unenforced rather than distinguishing the two cases it cannot actually keep
apart.

The consequence is stated rather than hidden: a Devin delegate is confined by
what it is allowed to attempt, not by where it may act. A role that needs a
bounded workspace does not get one from this runtime on this account, and the
snapshot says so to anyone who reads it.

Declaring it is not enough, because nothing acts on the declaration today. The
capability selector refuses an unverified field only when the requested
envelope sets it to the literal `deny`, and the two root fields are ARRAYS, so
listing them changes nothing.

Two things are fixed. The list the gate reads is `permissions.unverified`, and
it names all three fields — `readRoots`, `writeRoots`, `network` — not just the
two filesystem escapes; a `network: notEnforced` declaration that no list
mentions is decorative, and the fixtures now include a restrictive-network case
that proves it is not. And the predicate is defined per field rather than by one
literal: `network`, `approvals`, and `tools` are restricted by any value other
than their most permissive; a non-empty `readRoots` or `writeRoots` array is a
bounded grant and therefore restrictive. An empty `writeRoots` normally means
"no writes", which for most runtimes is enforced by denying the write tools —
but NOT for this one. Devin is granted `exec` on every delegate (denying it
makes the runtime unusable, O-15), and a shell can write outside any root
(O-9). So when `writeRoots` is declared `notEnforced`, an empty `writeRoots` is
ALSO restrictive: "no writes" cannot be guaranteed, and the recorded waiver is
required. A runtime whose `writeRoots` is `mapped` (claude, codex) keeps the
empty-means-no-writes exemption, because it can actually keep that promise.

What survives the rule is decided per role, in the open, through the waiver
mechanism the selector already has. `<role>.requirements.json` carries
`waivers`, and a waiver is a recorded human acceptance of a named residual for
a named runtime:

- EVERY role that may run on devin waives `readRoots` for it — all six, not a
  "read-only" subset. Every role requests a non-empty read-root array, so a
  waiver list with a hole in it refuses that role outright; an implementer role
  refused for its read roots would never reach the write-root question at all,
  and the benchmark's core delegate would be unusable. What makes a delegate's
  output trustworthy is the evidence it cites, not the reach of its reads, and
  the snapshot records that the boundary is nominal.
- `writeRoots` is waived for devin on every role, by the human's ruling above,
  BECAUSE even an empty write scope is restrictive on this runtime (exec can
  write through a shell). A role without the waiver is refused rather than
  quietly dispatching an uncontained write-capable delegate. The reason lives
  in this plan, not a comment: the containment is the human's virtual machine,
  not anything this system enforces.

That keeps the refusal loud where it matters and makes every exception a line
someone signed.

## D-2. The per-turn config layers on the user's, with one precedence rule

The generated config starts as a copy of the user's `config.json`, so the
organisation id and the onboarding marker survive and no banner lands in a
turn's stdout (O-4). The metasystem's `permissions` block REPLACES any
`permissions` or `sandbox` member the user file carries; every other member is
preserved untouched. Replacing is the only safe direction: merging can only
widen what the job may attempt, and the job envelope is the narrower claim.

The adapter records which user members it replaced in the round directory, so a
turn's evidence shows the difference between the user's configuration and the
one the turn ran under.

That the copy suppresses the banner is measured, not assumed (O-13): a config
built by copying the user file and adding a permissions block, written to a new
path, produced a turn whose entire stdout was the reply.

Project-level configuration is named as an input by the adapter's identity hash,
so whether it can widen what `--config` grants was measured — for
`.devin/config.json` only (O-14): a project file granting `edit`, `exec`, and
`Write(**)` changed nothing under a `--config` envelope that denied them.
`--config` replaces; it is not layered over.

That result covers `config.json` and nothing else. `config.local.json` and
`hooks.v1.json` were not tested, and hooks in particular are a mechanism the
adapter already treats as live, so the design claims nothing about them: they
stay in the identity hash, and the adapter records only what it measured rather
than declaring untested files inert.

## D-3. Session identity: correlated live, settled by the transcript

During the turn the adapter correlates by `devin list --format json`, reading
the `id` member — not `session_id` or `sessionId`, which that output does not
have (O-7). A session counts as this turn's only if its `working_directory`
resolves to this turn's workspace and it was absent from the pre-launch
listing. If two such sessions appear, the adapter refuses with a named protocol
error instead of choosing by timestamp: two concurrent launches in one
directory cannot be told apart, and guessing records a peer's session as this
job's identity.

A RESUMED turn correlates nothing: its session id is the one being resumed, it
is already in the baseline listing by definition (O-6), and looking for a new
session would find none. The adapter uses the requested id directly for a
follow-up and verifies it against the transcript at the end.

A baseline listing that fails to produce JSON is a refusal, not an empty
baseline: with no baseline every pre-existing session looks new, which is how a
peer's session becomes this job's recorded identity.

Correlation polls from launch, not from first output. Stdout is the final reply
on this runtime (O-1), so waiting for output before correlating means the
handshake can only ever complete as the turn ends, and a turn that produces no
reply (O-12) never correlates at all.

When the turn ends, the exported `session_id` is authoritative. If it disagrees
with the correlated one, that is a protocol error, not a preference.

## D-4. Usage is native, per round, and never estimated

`final_metrics` is cumulative for the session (O-11), and the chain contract
sums per-round records, so recording the raw totals would count every earlier
turn again in every later round. Each round records the DELTA: the exported
totals minus the totals the previous round of this chain recorded, with the
first round recording them as-is. The cumulative totals are kept beside the
round's usage so the next round can subtract them.

The cumulative totals are persisted as `session-usage.json` in the round
directory, holding the four transcript figures exactly as exported. It is a
file rather than an extra member of the typed usage object, because the usage
object is a published schema and this is adapter bookkeeping. A follow-up reads
the previous round's file. If it is absent — an older chain, or a round whose
directory was collected — the round records usage as UNAVAILABLE rather than
publishing the session totals as if they were this round's. Chain, mission, and
benchmark consumers add round records, so publishing cumulative totals there
does not merely mislabel one round, it double-counts every earlier turn in
every aggregate. A note in a job log does not reach an aggregator; an
unavailable usage record does.

`availability` is `native` when tokens are reported. `cost` stays null — no
per-session cost is reported, and nothing is estimated from token counts.
`providerUnits` carries `total_steps` as a delta on the same rule.

An enterprise account reports no tokens and reports ACU instead (O-16), so
usage there is `unavailable` for tokens and the ACU count rides in
`providerUnits`, delta'd like everything else. It is never written into a token
field and never converted into cost: it is a provider unit, and `providerUnits`
is exactly the field the mission fence already meters by name. A tokenless
environment is therefore metered rather than silently unmetered, which is the
whole point — the fence must be able to stop a runaway mission on an account
that never mentions a token.

Because the exact key is not observable from a non-enterprise account, the
adapter matches any `final_metrics` member whose name contains "acu" and
records the key it matched beside the value. If none matches, usage is
unavailable and nothing is invented. The self-test asserts the property that
holds in BOTH worlds: a turn is measured by something the fence can meter,
never by nothing.

## D-5. The effective model is observed, never assumed

`agent.model_name` names the model that answered (O-5), and the adapter records
it as the effective model instead of echoing the requested one.

It is a display name — "SWE-1.7" for the requested `swe-1-7` — and benchmark
validity requires the requested and effective identifiers to be EQUAL, so a raw
display name would make every Devin benchmark run invalid. The adapter
canonicalises: lowercase, and every run of characters that is not a letter or a
digit becomes a single hyphen. "SWE-1.7" canonicalises to `swe-1-7`, which is
the requested identifier. If the canonical form does NOT equal the requested
one, that is the real disagreement the effective-model field exists to catch,
and it is recorded as-is. The raw display name is kept in the round's evidence
either way, so the canonicalisation can be audited rather than trusted.

## D-6. A turn with no reply is a failed turn

Exit 0 with no reply is Devin's shape for "could not do it" (O-12). The adapter
records `error: "empty_reply"` with `phase: "delivery"` — a distinct, testable
value, not a generic runtime error.

Ordering matters and the current one makes this unreachable. Correlation is
driven by the poll from launch (D-3), not by first output, so a turn that never
writes a reply still has a session and still reaches completion; completion
then sees an empty reply and records `empty_reply`. Under the shipped
first-output predicate the same turn would have failed earlier as a missing
session id, which names the wrong cause.

This also decides the shape of the no-write self-test leg, which asks a
delegate to attempt a forbidden write AND a forbidden network fetch and then
return JSON: on Devin that turn ends with no reply (O-12), so those questions
cannot share a turn. The leg runs as THREE turns for this runtime — one that
attempts the forbidden write, one that attempts the forbidden fetch, and one
that returns the schema-valid critique. Each denial turn is asserted to fail
with `empty_reply`, and the fetch turn is asserted separately because a denial
that ends the turn proves nothing about an action that came after it: absence
of an HTTP request in a turn that stopped at the write says only that the turn
stopped. Merging them would quietly drop the network coverage the other
runtimes' legs have.

## D-6a. One repair turn, in the same session, recorded

A reply can be perfect work in the wrong shape. Burning a whole session over
the shape is the wrong price, and so is the harness renaming fields to make a
return validate: the fields it would guess at are the evidence a critique is
judged on, and inventing them is exactly what the untrusted-return rule exists
to prevent.

The syntactic half of recovery already exists and is shared — `normalize_return`
already pulls the object out of surrounding prose, code fences, and wrappers.
What it cannot do is decide what a delegate observed.

So a return that does not validate costs ONE repair turn, resumed in the same
session, showing the delegate its own violations and the schema and asking for
the object alone. Everything it already did stays in context; nothing is
re-run. Three conditions:

- **Bounded.** One attempt. A delegate that cannot follow a schema it has just
  been handed twice does not get unbounded turns; the second failure is the
  protocol error it always was.
- **Recorded, never laundered.** The record carries `returnRepairs`, the first
  reply stays as evidence, and the job log names the repair. A chain that
  needed a repair must never read as one that got it right first time.
- **Shared, not Devin-specific.** The hook fires for any runtime whose adapter
  defines a repair turn, which in practice means the runtimes whose CLI takes
  no schema. Claude and codex have schema enforcement at the API and never
  reach it.

Human decision, 2026-08-08, on being shown the alternative of coercing fields:
"I will follow your recommendation."

## D-7. A host adapter, with its envelope stated

`scripts/agents/hosts/devin.sh` implements the sibling contract: `start-turn
--mission --turn-id --prompt --result [--resume-session] --instance-tag`,
honouring the start gate, writing one atomic result with `sessionId`,
`outcome`, `usage`, `rawPath`, `returnPath`.

Its envelope is stated here because the interface has no permission argument
and the siblings each choose one: the Devin host runs with the repository as
its workspace, `--permission-mode accept-edits`, and a config built from
`scripts/agents/permissions/workspace.json` the same way a write-capable
delegate's is. A mission host that could not edit the repository could not
advance a mission; that is why the mode is not `auto`. The same unenforced
boundary as D-1 applies and is declared the same way.

Resumed host turns carry the same cumulative-metrics problem as delegate rounds
(O-11), so they carry the same rule: each turn directory holds its own
`session-usage.json`, each turn publishes the delta, and a turn whose
predecessor artifact is missing publishes usage as unavailable. A host that
published session totals per turn would inflate every mission and benchmark
that sums them.

Because this runtime has no schema flag (O-17), the adapter appends the exact
schema to the prompt it sends, with an instruction to return one JSON object
and nothing else. The dispatcher's own prompt file is left untouched as
evidence; the augmented copy is what the CLI reads. The same applies to the
host below.

Devin has no native structured output, so the return arrives as the mission
prompt already asks: the host copies the turn's stdout to the return path when
it parses as the orchestrator return, and otherwise leaves the return absent so
the runner's validation reports it. Outcomes follow the existing rule: `failed`
on nonzero exit or an empty reply, `unresumable` when the session is missing or
differs from the one being resumed, `completed` otherwise.

## D-8. The self-test page carries the evidence that proves acceptance

`development/devin-selftest.md` was written for "the other laptop" because no
machine here had Devin. One does now. The page states that the acceptance run
happened and against which CLI version, and it carries evidence that a reader
can check WITHOUT trusting the adapter that produced it.

Job records and returns are not that evidence: they hold the adapter's own
derived claims. The acceptance bundle is the provider's own artifacts beside
the adapter's, so the two can be compared — the exported transcripts (session
id, `agent.model_name`, `final_metrics`), the `session-usage.json` files, the
generated per-turn configs, and the job records and returns. The capability
snapshot is not acceptance evidence at all: probe writes it before the
behavioural test runs and it survives a failed one.

STATUS 2026-08-08: this bundle is DESIGNED, not yet built (KI-28). The runbook
now carries the self-test's job records, returns, and transcripts as the
acceptance record and the capability snapshot travels; the redacted
external-store bundle with a tracked index is follow-up work. What IS built is
the correction of the false runbook — it no longer claims Devin has never run.

Where it goes follows the evidence contract rather than inventing a new home.
The bundle lands in the external durable evidence store beside every other
terminal chain's evidence, and the repository tracks only an index entry naming
it — the same division the rest of the system uses. `development/evidence/`
would have been worse than the `git add` it replaced: the repository ignores any
directory named `evidence`, so an explicit copy there is silently omitted too.

Two rules keep the bundle honest. Its name carries the self-test's own job id,
not just a CLI version and a date, so two runs of one version on one day cannot
collide, overwrite, or merge. And the generated per-turn configuration is
redacted before it travels: it is a copy of the human's own file and carries
their organisation identifier, so the bundle keeps the permissions block, which
is what a reader needs to check, and replaces every inherited member's value
with a hash of it, which proves sameness without disclosing content.

## D-9. The Devin benchmark is a separate spec, by the human's decision

The kit's only spec, bm-1, pins delegates to codex `gpt-5.6-luna` and carries
the human's cost ruling from 2026-08-05. Three paths were possible — amend
bm-1's roster, add a separate spec, or prove Devin on a mission first — and
they differ in what happens to that ruling and to benchmark identity, so the
choice was the human's. On 2026-08-08 they chose a SEPARATE SPEC: bm-1 and its
ruling stay untouched.

The new spec is bm-1's structure with one roster and one ruling of its own. Its
delegates are `devin:swe-1-7`. Its ruling records two things a reader needs:
that the roster was chosen deliberately rather than inherited, and that Devin
runs uncontained under the human's global acceptance of 2026-08-08. It does NOT
say the residual is accepted here because targets are disposable — that was the
false premise the ruling deleted, and it must not survive in a manifest either.
Nor does it say the residual is refused elsewhere: the acceptance is global, and
a manifest claiming otherwise would misdescribe the system it measures. Its
identity is its own, so its numbers are not comparable to bm-1's, and the
manifest says that in the same words the kit already uses.

The cohort runs TWO repetitions. The number is named here because
`run-cohort.sh` requires one and no one else supplies it: two is the smallest
count that shows whether a result repeats at all, and swe-1-7 is free, so the
usual reason to run fewer does not apply. A different number is a human's to
choose, and changing it changes how many targets, signatures, and scorecards
the run produces.

What remains the human's, unchanged, is the cohort signature the driver
requires per repetition. That is the kit's designed stopping point and this
design does not route around it.

## Proof

- The adapter's own self-test passes end to end against the real CLI: dispatch,
  follow-up on the same session, cancel, and usage.
- The permission leg proves the SNAPSHOT rather than a wish. A field declared
  `mapped` must actually stop the attempt — unchanged for the runtimes that
  claim it, and the check that would catch enforcement quietly disappearing. A
  field declared `notEnforced` asserts nothing about the turn in either
  direction: whether a given turn escapes depends on which tool the model
  reaches for, and the same Devin turn wrote outside its declared root through a
  shell once and declined the next time. Absence of enforcement is not
  observable from one turn's behaviour; only its presence is. The proof that the
  escape is real was made once, deliberately (O-9), and belongs in this
  document rather than in a per-run assertion that would flake both ways.
  If this organisation ever permits `--sandbox`, the declaration flips to
  `mapped` and the leg starts demanding denial again in the same run.
- The snapshot declares all three envelope members unenforced, and a fixture
  proves the gate ACTS on that: because Devin grants exec on every delegate,
  even an empty write scope is restrictive, so a devin role WITHOUT the recorded
  `writeRoots` waiver is refused, and the same role WITH the waiver on record
  runs. There is no "read-only Devin role runs unwaived" case — that was the
  bypass the corrected selector closes.
- Round two of a chain records usage that is the delta EXACTLY: for each of
  input, cached, output, and steps, the round-two figure equals the round-two
  transcript total minus the round-one total, and round one equals its own
  transcript total.
- The recorded effective model equals the requested identifier after
  canonicalisation, and the raw display name is present in the round evidence.
- A turn that produces no reply records `empty_reply` in phase `delivery`, not
  a missing session id.
- Session identity in the record equals the transcript's `session_id`, and a
  follow-up records the same one; two new sessions in one directory refuse.
- The Devin host passes the canonical host-cycle smoke, which is the sibling
  hosts' bar: a mission turn that dispatches at least one delegate on a toy
  change, plus the swapped-roles leg. A host that only echoes valid JSON does
  not pass.
- The full suite and the kit gate stay green.
- A benchmark cohort with Devin delegates on `swe-1-7` reaches the signing
  boundary, and completes once signed.
