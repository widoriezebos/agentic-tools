# Budgets belong to the binding: per-model delegate caps

- Goal and current status: a delegate job's time budget is declared where
  its runtime and model are declared — keyed on the (runtime × model)
  pair, optionally sharpened per role. Revised against critique round 1
  (14 material findings, all folded below); the revision's spine is ONE
  rule the draft lacked: THE SIGNED CONTRACT IS THE ONLY AUTHORITY THAT
  CAN RAISE A MISSION JOB'S BUDGET.
- Next step: critique round 2 with sol
- In flight right now: nothing
- Waiting on the human: nothing

## Why, with the evidence

bm-2's uniform 15-minute cap killed a Devin/swe-1-7 implementer holding
1,322 compiling lines; the mission shipped a skeleton (acceptance 1/53,
both repetitions). Not the model, not the CLI, not a bug: the
configuration. The human ruled: cap keyed on (runtime × model), declared
where bindings are declared, and Devin's number DISCOVERED by one
instrumented run, not guessed.

## D-1. Two regimes, one authority rule (folds CAPS-R1-001/003/004)

- MISSION JOBS: `fence.job-cap-min` in the signed contract remains the
  SOVEREIGN UNIVERSAL CEILING. Per-pair caps for a mission exist ONLY as
  contract keys in the ```mission block —
  `cap.min.<runtime>.<canonical-model>=N` — sealed, signed, and
  preflight-verified like every other mission key. A contract pair cap
  may exceed fence.job-cap-min for its pair (the human signed exactly
  that); nothing UNSIGNED ever can: dispatch REFUSES a `--cap-min` above
  the contract-resolved value for a mission job, and refuses
  `.local`/environment cap overrides for mission-fenced dispatches
  outright. Authority, not precedence.
- NON-MISSION JOBS (ordinary harness work, no contract exists):
  `cap.min.<runtime>.<canonical-model>` and the role-sharpened
  `cap.min.<role>.<runtime>.<canonical-model>` resolve through the normal
  config chain (base, .local, env), most specific wins, explicit
  `--cap-min` wins over all — today's trust model for uncontracted work,
  unchanged in spirit.

## D-1a. Canonical key encoding (folds CAPS-R1-014)

Cap keys use the CANONICAL MODEL FORM, computed by a pre-dispatch helper
(`scripts/agents/canonical-model.py`, extracted from the devin adapter's
existing canonicalisation so there is exactly one implementation):
lowercase, every run of characters outside [a-z0-9] collapsed to one
hyphen (`SWE-1.7` → `swe-1-7`, `gpt-5.6-sol` → `gpt-5-6-sol`). Dispatch
canonicalises BEFORE resolution. Because the encoding is lossy, dispatch
also REFUSES a configuration that binds two distinct requested model
strings to the same canonical key with different cap values — collision
is a loud config error, never a silent winner.

## D-2. Transport: provisioning writes what the host will read (folds CAPS-R1-002)

The mission runner does not dispatch delegates; the HOST does, inside the
target. So the roster's caps travel as configuration, not prompts: the
spec manifest gains a sibling map `"delegateCaps": {"devin:swe-1-7": 90}`
(the roster string map itself is unchanged — no schema migration), and
PROVISIONING (a) writes the corresponding `cap.min.*` keys into the
target's metasystem.conf during its existing tailoring step and (b)
writes the same keys into the generated mission contract's ```mission
block, so the sealed bytes and the target config agree and preflight can
compare them. The host's dispatches then resolve caps from the target
config exactly like any dispatch, with the contract as the mission-job
authority per D-1.

## D-3. Deadlines, not just durations (folds CAPS-R1-005/006/008/009)

- At LAUNCH, dispatch computes the job's absolute `capDeadline` =
  startedAt + resolved cap, TRUNCATED to the mission's wall-clock end
  when a mission fence applies. Sub-minute or negative remainder refuses
  the dispatch ("mission has N seconds of wall clock; refusing to start
  a job that cannot run"). The reaper enforces `capDeadline` when
  present (fallback: today's startedAt + capMin arithmetic).
- FOLLOW-UPS RE-RESOLVE: a follow-up job resolves its cap fresh at its
  own dispatch and truncates against the wall clock as of THEN — never a
  copy of the parent's stamp.
- VERDICT ATTRIBUTION: a job ended at a wall-clock-truncated deadline is
  attributed to the WALL CLOCK — the fence refusal names
  wall-clock-hours, not job-cap-min, so the human is never asked to
  amend the wrong contract term.
- POST-CAS CONSEQUENCES ONLY: the reaper files its mission fence refusal
  ONLY when its terminal CAS won. A completed verdict that beat the
  timeout write means no cap-exhaustion ask — the committed winner owns
  the consequences (the flight recorder's D-3a rule, applied here).

## D-4. Provenance that survives questions (folds CAPS-R1-007)

The job record gains one object:
`"capResolution": {"requestedMin": int, "rule":
"argument|contract-pair|config-role-pair|config-pair|fence-default|
built-in", "origin": "contract|conf|conf-local|env|argument|default",
"truncatedBy": "wall-clock"|null, "deadline": ISO-8601}`.
Nothing overwrites the rule when truncation applies — truncation is its
own field. A scorecard can reconstruct why every job had the budget it
had, from the record, not the witness stream.

## D-5. The watcher ceiling is derived, not discovered (folds CAPS-R1-010)

Dispatch's rule that every cap stays below the watcher's inactivity
ceiling stands. Therefore ARMING derives the watcher ceiling from the
maximum cap resolvable in that checkout (max of config pair caps,
contract pair caps, fence.job-cap-min) plus a fixed margin, instead of a
constant 180. The experiment does not disable anything: raising the
contract's pair cap automatically raises the derived ceiling, visibly.

## D-6. The discovery experiment, honestly framed (folds CAPS-R1-011/012)

`bm-2c`: bm-2's spec with ONE repetition, contract pair cap
`cap.min.devin.swe-1-7=150`, `fence.wall-clock-hours` raised to 4, same
gate and grader. Measurement is DRIVER-SIDE (the witness stream stays a
witness): the experiment runs under a named watch script that samples the
implementer worktree's newest-mtime and file count every 60s into the
cohort directory (`cap-experiment-samples.jsonl`), with start = the job
record's startedAt and end = its endedAt. PREDECLARED CRITERIA: a job
with no worktree change for 45 consecutive minutes is classified STALLED
(practical noncompletion); a job ended by cap or fence is CENSORED and
reported as "exceeds T", never as a completion time; only natural
terminations produce completion times. A censored 150-minute result does
NOT conclude "cannot complete" — it concludes "needs more than 150
minutes or a different structure", and the comparison cohorts inform
which.

## D-7. What does not change (folds CAPS-R1-013)

- Host turn caps remain CONTRACT-ONLY (`host.turn-cap-min`). The draft's
  "config form for symmetry" is DROPPED: it was decorative at best and an
  unsigned override at worst.
- `capMin` on the record, the fence metering interfaces, timeout-vs-
  process-lost priority, and every non-cap dispatch path.

## Proof

- Authority: a mission dispatch with `--cap-min` above the
  contract-resolved value REFUSES; a `.local` cap override on a
  mission-fenced dispatch REFUSES; the same override on a non-mission
  dispatch applies and its provenance says conf-local.
- Precedence and provenance: one fixture per D-1 rule; each asserts the
  full capResolution object including origin.
- Canonicalisation: `SWE-1.7` and `swe-1-7` resolve to one binding; two
  distinct models colliding on one canonical key with different caps
  refuse loudly at dispatch.
- Deadlines: a job launched with 90s of mission wall clock left refuses;
  one truncated mid-cap carries truncatedBy=wall-clock and its reap
  attributes wall-clock-hours; a follow-up re-resolves and re-truncates.
- Post-CAS: a completed verdict that wins against the timeout write
  produces NO fence refusal (staged as the dispatcher-vs-reaper shape).
- Watcher ceiling: arming over a 150-minute contract cap yields a ceiling
  above it; dispatch still refuses a cap above the derived ceiling.
- Transport: a manifest delegateCaps entry lands byte-identically in the
  target conf AND the sealed contract, and preflight fails on a mismatch.
- Experiment: the sampler produces monotone timestamps; a synthetic
  stalled worktree classifies STALLED at exactly 45 minutes; a
  cap-ended job reports censored, not completed.
