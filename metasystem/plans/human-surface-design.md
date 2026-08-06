# Human Surface Design: recovering from over-engineering

- Goal and current status: every human touchpoint of the metasystem reduced to English prose or a single command, with complexity permitted only on the agent side; over-engineered pieces named by evidence and removed. Status: DRAFT, awaiting critique.
- Next step: design critique by Codex until closed by join; then implement per the order below.
- In flight right now: nothing
- Waiting on the human: ratification comes through accepting this design after critique

## The standard (the human's words, made binding)

The framework must be robust, practical, and easy for humans; it must never
hinder a human trying to achieve something — it helps, or protects from harm,
or gets out of the way. A human never has to produce many artifacts, or any
complex artifact: **English prose should be enough to get things done or
started.**

Made mechanical as one rule, to be added to the shipped
`docs/project-rules.md` template (anonymous form) through this design's
change-gate passage:

> Every artifact names whose hands touch it. Anything a human must PRODUCE
> for the machine to consume is English prose or a single command; complexity
> may only live on the agent side of that line. Review duties — reading
> contracts, diffs, evidence, retro proposals — are unchanged and are not
> "touch": the rule governs inputs, never oversight (HS-1-6). A refusal a
> human can hit must state, in its message, exactly what to change.

## Scope fence, against over-engineering the fix

No new frameworks, no new file formats, no new required inputs. Every change
below either deletes something, collapses steps into one command, or improves
an error message. If implementing any finding grows the human's surface, the
finding is wrong and comes back here.

## Findings, each with evidence and proof

**F-1: The signing ceremony transcribes hashes by hand.**
Evidence: every mission signed so far took four to five steps (seal, copy a
64-character hash, paste an approval line, commit, push); the protection —
review and explicit consent — lives in none of them.
Change: `scripts/assert-mission.sh --sign "<name>" --file <contract>` is display-then-confirm (the file is explicit: with several contracts present, guessing is worse than typing a path — HS-4-6)
in one command, and it REQUIRES an already-sealed contract: sealing runs the
gate to record the baseline, which is code execution, so sealing stays an
agent-side step and --sign refuses an unsealed contract by naming the seal
command (HS-3-1). The mandatory display shows the contract's English
sections plus every enforceable bound: exposure, fences, gate threshold, and
the permission envelopes, since envelopes are reserved-decision boundaries
the signer is granting (HS-3-2). Only then does it ask for typed
confirmation; consent is the confirmation after the display, never the
invocation (HS-1-1). Non-interactive use requires an explicit
`--confirmed-after-review` flag carrying the same meaning. The git
transaction is defined AND proven (HS-1-2, HS-3-3): refuse unless on the
origin-tracked default branch preflight will fetch; refuse when anything
besides the contract is staged; commit exactly the contract file; push to
origin's default branch; on any git failure state precisely what remains.
Proof: fixtures for the confirmed happy path through preflight; refusal
without confirmation; refusal off the default branch; refusal on an unsealed
contract; refusal with unrelated staged changes; induced commit failure and
induced push failure, each asserting the exact residual state named in the
message.

**F-2: Prose-to-mission is the real flow but is documented nowhere.**
Evidence: both real missions started as human prose; the orchestrator drafted
the contract; the human consented. The docs instead present the contract
grammar first, as if a human writes key=value blocks.
Change: `docs/orchestration.md` mission section opens with the flow: the
human states intent, limits, and budget in prose; the orchestrating agent
drafts the contract; the human reads the English sections and signs (F-1's
one command). The grammar remains, labeled as the agent-side format. The
same section documents the EXISTING one-command prose answer for parked
missions (`mission-runner.sh answer`, absorbed from withdrawn F-7) as THE
answer flow (HS-4-5).
Proof: the audit's checks pass AND a fixture asserts the mission section
names both the prose-first flow and the answer command verbatim.

**F-3: Cohorts must not multiply signatures.**
Evidence: a cohort of N repetitions currently implies N seal-and-sign
ceremonies; the human has already rejected artifact multiplication in
principle.
Change: cohort-level authorization as the default, made representable and
bounded rather than asserted. The contract grammar gains one optional line
(HS-1-3): `Cohort-Authorization: cohort-id=<id>; record-sha256=<hash>`, which
seal covers and preflight accepts as the approval when — and only when — the
named cohort record is itself signed and its hash matches. The cohort
record's signed bytes bind everything one signature spends (HS-1-4): the
contract template hash, spec id and version, measuring-kit version, fence
vector, roster, repetition count, per-repetition exposure, and the total
exposure ceiling. The hash protocol is normative and complete (HS-3-4,
HS-4-1): the template hash is sha256 over the contract file's bytes from its
first byte through the closing fence of the ```mission block ONLY — the seal
block, approval lines, and Cohort-Authorization line all sit outside the
hashed region by construction, which also excludes every seal-time variable
(timestamp, gate-ref sha) — with exactly one substitution: the `mission.id`
value replaced by the literal token `@COHORT-MISSION-ID@`. Preflight
recomputes exactly this; a deviating repetition fails on the hash. Replay
bounds have grammar, owner, and atomicity (HS-3-5, HS-4-2): the record
carries `expiresAt` (ISO-8601 UTC; preflight refuses at or after expiry);
each repetition's mission id is `<spec-id>-<cohort-id>-r<index>` with index
in [1, count]; the consumed-index ledger is the driver-owned directory
`benchmark/results/cohorts/<cohort-id>.consumed/`, one file per index created
with O_EXCL BEFORE provisioning — creation is the atomic reservation,
reservations are never released, and a failed repetition consumes its index:
rerunning it is a new cohort decision, never a silent retry. The cohort
record signs without sealing (HS-4-3): records run no gate, so F-1's command
gains a `--record <path>` mode whose display shows the bound facts (spec id
and version, template hash, repetition count, per-repetition and total
exposure, expiry) and whose approval line hashes the record's canonical
bytes; no seal step exists for records. Per-repetition signing remains
available; the driver accepts either and refuses a repetition with neither.
Proof: kit-gate fixtures for both paths, the refusal with neither, a
tampered-template repetition refused, a total-exposure ceiling exceeded
refused, a reused repetition index refused, and an expired record refused.

**F-4: Configuration generality billed to every human up front.** (Amended
per HS-1-5: the original deletion claim was WRONG — both key families have
shipped readers, and `model.tier.*` powers the dispatcher's cost-escalation
guard. The critic checking rather than trusting saved a protection.)
Evidence: the tier table and mode-scoped keys confront every adopter as
placeholders to fill although most projects never need them, and the tier
table contributed to the roster confusion that cancelled three healthy jobs
on 2026-08-05.
Change: the readers and their guards stay untouched. The shipped template
demotes both families to commented-out examples with one line each saying
what configuring them buys; `metasystem-config.sh validate` accepts their
absence (verify it already does; fix if not). The cost-escalation guard
fails CLOSED without tiers (HS-3-6), and the approval path is CREATED, not
cited — none exists today (HS-4-4): the dispatcher gains
`--approve-escalation "<name>"`, which records name and timestamp in the job
record as the escalation's approval evidence; without tiers, any `--model`
override differing from the roster's resolution refuses with a message
naming both remedies (configure tiers, or re-run with the approval flag).
The guard never silently accepts what it cannot rank. Validate says in one
informational line that tiers are absent and overrides therefore always
escalate.
Proof: the suite passes with the demoted template; fixtures prove validate
accepts absence, still refuses malformed present keys, the escalation guard
still ranks when tiers are configured, and — decisive for HS-3-6 — an
override without tiers is refused pending approval rather than accepted.

**F-5: Refusals that do not hand the human the fix.**
Evidence: a contract metric named with an underscore cost an hour; the
refusal named the key as unknown instead of stating the allowed charset and
the fixed spelling.
Change: sweep the refusal messages a HUMAN can plausibly hit — contract
validation, seal, sign, preflight, adopt, provision, and the mission
ask-and-answer path (HS-1-7) — and make each one state what to change, in the
message itself. Agent-only assertion messages are out of scope.
Proof: the implementation enumerates every refusal on those surfaces into a
checklist in its return (HS-1-8: the sweep is auditable, not sampled), and
fixtures assert the message for at least: underscore metric name, missing
approval, dirty template, existing target, unanswered-ask park, and answer
to an unknown ask.

**F-6: Adoption asks the human to fill files a conversation should fill.**
Evidence: after `adopt.sh`, the human faces two files of placeholders;
nothing says the intended flow is an agent interviewing them in prose and
filling both, with the human reviewing the diff.
Change: document that flow as the default in `docs/project-adaptation.md`
step 2 (prose interview, agent fills, human reviews); adopt.sh's closing
message points at it. No new tooling: the agent is the interviewer.
Proof: doc change only; adopt fixture asserts the closing message names the
flow.

**F-7: WITHDRAWN (HS-3-7) — the one-command prose answer already exists.**
The critic showed `mission-runner.sh` already accepts a prose answer in one
command with reason-specific validation, atomic recording, and anchoring; the
finding was built on a false premise and would have created a second,
incomplete owner for answers. What remains of it folds into F-2 (document
the existing command as THE flow) and F-5 (its refusals join the sweep).
The withdrawal stays in this design as evidence that the scope fence works.

## What is deliberately NOT simplified

The agent-side machinery — close-by-join, conformance, schemas, gates,
supervision, the receipts grammar — stays exactly as heavy as it is. It
caught five of the last six defects in its own author's work. The line this
design draws is not "less rigor"; it is "rigor never billed to the human."

## Implementation order

F-4 (template demotion, fail-closed guard), F-1, F-3 (builds on F-1's flow),
F-5, then F-2 and F-6 (docs last, describing what then exists; F-2 absorbs
withdrawn F-7's documentation duty). One implement brief per finding or
adjacent pair; each cites this design; the usual loop applies.

## Completion

Complete when every finding's proof exists and is green in the gates, the
rule from the standard is in the shipped template, and a walkthrough of every
human touchpoint (adopt, daily prose, mission consent, cohort authorization,
upgrade checklist) shows nothing but prose and single commands on the human
side. Then this file is closed into `development/` as a finished report.
