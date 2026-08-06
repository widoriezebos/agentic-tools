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

> Every artifact names whose hands touch it. Anything a human must touch is
> English prose or a single command; complexity may only live on the agent
> side of that line. A refusal a human can hit must state, in its message,
> exactly what to change.

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
Change: `scripts/assert-mission.sh --sign "<name>"` shows the contract's
English sections plus the decisive numbers (exposure, fences, gate threshold),
seals if not yet sealed, appends the approval line with the computed hash
itself, commits, and pushes where an origin exists. Consent is the act of
running the command with your name after the display; the display is
mandatory, not skippable.
Proof: a suite fixture signs a fixture contract with `--sign` and preflight
passes; a second fixture proves `--sign` on an unreviewable contract (invalid,
unsealed-and-unsealable) refuses with the reason.

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
Change: cohort-level authorization as the default. The human signs the cohort
record once (name plus the cohort record's hash, same one-command flow as
F-1); the driver then stamps each repetition's contract approval mechanically,
citing the cohort authorization. Per-repetition signing remains available;
the driver accepts either and refuses a repetition with neither.
Proof: kit-gate fixtures for both paths and for the refusal.

**F-4: Dead configuration generality that has already caused harm.**
Evidence: `model.tier.1..3` and `mode.<mode>.role.*` keys are read by no
shipped code path (verify by search before removal; abort this finding for
any key that is read), have never been exercised by real use, and the tier
table contributed to the roster confusion that cancelled three healthy jobs
on 2026-08-05.
Change: remove the keys from the shipped `metasystem.conf` template and from
`metasystem-config.sh validate`'s required set; delete dead resolution code
for them. Projects that need per-mode rosters can add keys when a real need
arrives, through the change gate, with a reader.
Proof: the suite passes with the pruned template; a fixture proves validate
accepts a conf without tiers and still refuses genuinely malformed rosters.

**F-5: Refusals that do not hand the human the fix.**
Evidence: a contract metric named with an underscore cost an hour; the
refusal named the key as unknown instead of stating the allowed charset and
the fixed spelling.
Change: sweep the refusal messages a HUMAN can plausibly hit — contract
validation, seal, preflight, adopt, provision — and make each one state what
to change, in the message itself. Agent-only assertion messages are out of
scope.
Proof: fixtures asserting the improved messages for the known-bad cases
(underscore metric name, missing approval, dirty template, existing target).

**F-6: Adoption asks the human to fill files a conversation should fill.**
Evidence: after `adopt.sh`, the human faces two files of placeholders;
nothing says the intended flow is an agent interviewing them in prose and
filling both, with the human reviewing the diff.
Change: document that flow as the default in `docs/project-adaptation.md`
step 2 (prose interview, agent fills, human reviews); adopt.sh's closing
message points at it. No new tooling: the agent is the interviewer.
Proof: doc change only; adopt fixture asserts the closing message names the
flow.

## What is deliberately NOT simplified

The agent-side machinery — close-by-join, conformance, schemas, gates,
supervision, the receipts grammar — stays exactly as heavy as it is. It
caught five of the last six defects in its own author's work. The line this
design draws is not "less rigor"; it is "rigor never billed to the human."

## Implementation order

F-4 (deletion first, smallest risk), F-1, F-3 (builds on F-1's flow), F-5,
F-2 and F-6 (docs last, describing what then exists). One implement brief per
finding or adjacent pair; each cites this design; the usual loop applies.

## Completion

Complete when every finding's proof exists and is green in the gates, the
rule from the standard is in the shipped template, and a walkthrough of every
human touchpoint (adopt, daily prose, mission consent, cohort authorization,
upgrade checklist) shows nothing but prose and single commands on the human
side. Then this file is closed into `development/` as a finished report.
