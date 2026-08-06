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
Change: `scripts/assert-mission.sh --sign "<name>"` is display-then-confirm
in one command: it validates, seals if needed, SHOWS the contract's English
sections plus the decisive numbers (exposure, fences, gate threshold), and
only then asks for typed confirmation before appending the approval line with
the computed hash; consent is the confirmation after the display, never the
invocation (HS-1-1). Non-interactive use requires an explicit
`--confirmed-after-review` flag carrying the same meaning. The git
transaction is defined, not implied (HS-1-2): the command refuses unless on
the origin-tracked default branch preflight will fetch, commits exactly the
contract file, pushes to origin's default branch, and on any git failure
states precisely what remains for the human. It executes nothing else — no
gates, no hooks beyond git's own.
Proof: fixtures for the confirmed happy path through preflight; refusal
without confirmation; refusal off the default branch; refusal on an
unsealable contract — each asserting the message names the fix.

**F-2: Prose-to-mission is the real flow but is documented nowhere.**
Evidence: both real missions started as human prose; the orchestrator drafted
the contract; the human consented. The docs instead present the contract
grammar first, as if a human writes key=value blocks.
Change: `docs/orchestration.md` mission section opens with the flow: the
human states intent, limits, and budget in prose; the orchestrating agent
drafts the contract; the human reads the English sections and signs (F-1's
one command). The grammar remains, labeled as the agent-side format.
Proof: doc change only; the audit's placeholder and reference checks pass.

**F-3: Cohorts must not multiply signatures.**
Evidence: a cohort of N repetitions currently implies N seal-and-sign
ceremonies; the human has already rejected artifact multiplication in
principle.
Change: cohort-level authorization as the default, made representable and
bounded rather than asserted. The contract grammar gains one optional line
(HS-1-3): `Cohort-Authorization: cohort-id=<id>; record-sha256=<hash>`, which
seal covers and preflight accepts as the approval when — and only when — the
named cohort record is itself signed (F-1 flow) and its hash matches. The
cohort record's signed bytes must bind everything one signature spends
(HS-1-4): the contract template hash, spec id and version, measuring-kit
version, fence vector, roster, repetition count, per-repetition exposure, and
the total exposure ceiling; a repetition whose generated contract deviates
from the bound template fails preflight because the template hash no longer
matches. Per-repetition signing remains available; the driver accepts either
and refuses a repetition with neither.
Proof: kit-gate fixtures for both paths, the refusal with neither, a
tampered-template repetition refused, and a total-exposure ceiling exceeded
refused.

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
absence (verify it already does; fix if not). An adopter who never uncomments
them loses only the cost-escalation guard's tier comparison, and validate
says so in one informational line rather than demanding tiers.
Proof: the suite passes with the demoted template; fixtures prove validate
accepts absence, still refuses malformed present keys, and the dispatcher's
escalation guard still works when tiers are configured.

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

**F-7: Answering a parked mission's ask is a raw-file ceremony (HS-1-7).**
Evidence: a mission that parks on a human question expects the answer as a
committed file in the exact shape the runner reads; nothing offers the human
a prose path.
Change: `scripts/assert-mission.sh --answer <ask-id> "<prose>"` writes the
answer artifact in the runner's shape, commits, and pushes, one command; the
runner's resume flow is unchanged.
Proof: fixture answering a parked fixture mission by the command and
resuming; refusal for an unknown ask id names the open asks.

## What is deliberately NOT simplified

The agent-side machinery — close-by-join, conformance, schemas, gates,
supervision, the receipts grammar — stays exactly as heavy as it is. It
caught five of the last six defects in its own author's work. The line this
design draws is not "less rigor"; it is "rigor never billed to the human."

## Implementation order

F-4 (template demotion, smallest risk), F-1, F-3 (builds on F-1's flow), F-7,
F-5, then F-2 and F-6 (docs last, describing what then exists). One implement
brief per finding or adjacent pair; each cites this design; the usual loop
applies.

## Completion

Complete when every finding's proof exists and is green in the gates, the
rule from the standard is in the shipped template, and a walkthrough of every
human touchpoint (adopt, daily prose, mission consent, cohort authorization,
upgrade checklist) shows nothing but prose and single commands on the human
side. Then this file is closed into `development/` as a finished report.
