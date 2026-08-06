# Human Surface Design: recovering from over-engineering

- Goal and current status: every human touchpoint of the metasystem reduced to English prose or a single command, with complexity permitted only on the agent side; over-engineered pieces named by evidence and removed. Status: CLOSED by join at round 12 (2026-08-06), 48 findings adjudicated; ready for implementation in the stated order.
- Next step: implement per the order below, one brief per finding or adjacent pair, each citing this design.
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

**F-3: Cohorts must not multiply ceremonies — SIMPLIFIED at round 5, by the
scope fence.** Rounds 2 through 5 grew a cohort-authorization protocol
(signed records, template hashes, consumed-index ledgers — since removed —
and expiry clocks) that the critique correctly kept finding holes in — HS-5-2 through
HS-5-5 showed the surface cannot bottom out, because a driver-owned mutable
record and a human-signed immutable authorization are different artifacts,
and any agent-invokable attestation is self-attested. That is this design's
own fence: the finding grew complexity, so the finding was wrong.
Change, minimal and inheriting the existing trust model wholesale: F-1's
command accepts multiple files — `--sign "<name>" --file a.md --file b.md …`.
Equality is defined on the regions the human is authorizing (HS-6-2): the
English sections and the ```mission block must be byte-identical across the
batch; seal blocks may differ, since sealing stamps per-repetition values
(sealed.at, baselines); identity lives in the filename, and duplicate paths
refuse (HS-6-4). The combined display lists every contract's path and
resolved mission id, then the shared English sections and bounds, then
per-repetition and TOTAL exposure (HS-6-4). One typed confirmation appends
each contract's ORDINARY approval line with that contract's own hash. The
git transaction is per contract repository (HS-6-3): each contract commits
and pushes in ITS OWN repo under F-1's single-contract rules — the batch is
one consent, N ordinary transactions, and a failure in repo K stops there
and names the signed-but-unpushed remainder precisely. No new grammar, no
records, no ledgers: every repetition carries individually signed bytes the
existing preflight already verifies, and the driver refuses a repetition
without its approval line. Non-identical contracts refuse with the first
differing line named.
Proof, matched one-to-one to the semantics (HS-7-2): the batch happy path
through preflight on every repetition; contracts differing by one byte
INSIDE the mission block refused naming the first differing line; contracts
differing only in seal blocks accepted; a duplicate --file path refused; the
combined display asserting every path, every mission id, and the TOTAL
exposure line; an induced push failure in repository K stopping there and
naming the signed-but-unpushed remainder; and the driver refusing an
unsigned repetition.

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
cited — none exists today (HS-4-4): the dispatcher gains an
escalation confirmation with the same trust anchor as signing (HS-5-5: a
bare flag is self-attested, since the agent is dispatch's normal caller):
`--approve-escalation` is interactive-only — it displays the roster
resolution, the requested model, and the cost direction, then requires typed
confirmation, recording name, timestamp, and the displayed facts in the job
record. The ONE non-interactive path that remains is the one that already
carries a signature (HS-6-5), and it is enforceable with or without tiers
(HS-7-3): the envelope names the exact
runtime-and-model PAIRS it permits — one key, `envelope.dispatch-allow=
<runtime>:<model>[,<runtime>:<model>…]` — and the guard checks the RESOLVED
pair for membership, never tier arithmetic. One exact key replaces the two
imprecise ones an earlier fold proposed (HS-9-2: authorizing a runtime
without binding the model that runtime's roster then selects is not a
spending bound). The key JOINS the authoritative contract grammar where
envelopes live (HS-8-2): the orchestration contract's envelope section and
the shipped project-rules envelope table both gain `dispatch-allow` with its
bound, and both RETIRE the `tier-move` category in the same change — an
unenforceable-without-tiers authorization must not remain grantable. Both
override flags (`--model`, `--runtime`) are guarded by the same membership
check on the resolved pair (HS-8-3), because switching provider crosses cost
lines tier arithmetic never sees. Interactive-only governs everything OUTSIDE a
signed envelope. Without tiers, any differing
`--model` override refuses, naming the remedies (configure tiers, sign an
envelope that authorizes it, or re-run interactively).
The guard never silently accepts what it cannot rank. Validate says in one
informational line that tiers are absent and overrides therefore always
escalate.
Proof (HS-6-6, enumerated): the suite passes with the demoted template;
validate accepts absence and still refuses malformed present keys; the guard
ranks when tiers are configured; an override without tiers refuses; the
interactive path has fixtures for non-TTY refusal, declined confirmation,
and a completed confirmation whose job record carries the recorded display
facts; an envelope-allowlisted resolved pair proceeds unattended while the
same request outside the allowlist refuses — covering a model override, a
runtime override, and a runtime override whose implied model is not listed
(the HS-9-2 case), all WITHOUT a tier table; and the grammar fixtures
proving `tier-move` refuses while `dispatch-allow` parses.

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

## Retired terms

RETIRED: standing authorization -- per-repetition batch signing (F-3): one confirmation, N ordinary approval lines

RETIRED: standing benchmark authorization -- the same retirement, under its long name, because the checker matches exact terms

RETIRED: D-B5 -- nothing: the amendment is gone, not renamed; batch signing is a signing-tool feature outside mission machinery

RETIRED: cohort-authorization line -- ordinary per-contract approval lines written by batch signing

RETIRED: consumed-index ledger -- individually signed repetitions; no shared authorization exists to consume

RETIRED: tier-move envelope -- envelope.dispatch-allow with exact runtime:model pairs
