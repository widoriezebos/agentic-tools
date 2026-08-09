# Budgets belong to the binding: per-model delegate caps

- Goal and current status: a delegate job's time budget is declared where
  its runtime and model are declared — keyed on the (runtime × model)
  pair, optionally sharpened per role. Revised against critique round 1
  (14 material findings, all folded below); the revision's spine is ONE
  rule the draft lacked: THE SIGNED CONTRACT IS THE ONLY AUTHORITY THAT
  CAN RAISE A MISSION JOB'S BUDGET.
- Next step: fold round 2's 11 findings (recorded below), then round 3
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
  unchanged in spirit. The existing general `dispatch.cap-min` config key
  REMAINS the fallback below the pair keys (provenance `config-general`),
  and the built-in default closes the chain (provenance `built-in`).

## D-1a. Canonical key encoding (folds CAPS-R1-014)

Cap keys use the CANONICAL MODEL FORM, computed by a pre-dispatch helper
(`scripts/agents/canonical-model.py`, extracted from the devin adapter's
existing canonicalisation so there is exactly one implementation):
the ADAPTER'S EXACT ALGORITHM (lowercase, runs outside [a-z0-9] collapse
to one hyphen, EDGE HYPHENS STRIPPED — the extracted helper is the single
truth and this prose defers to it). Dispatch canonicalises BEFORE
resolution. Collisions are detected WHERE RAW SPELLINGS EXIST:
provisioning refuses a manifest whose distinct raw strings collapse to
one canonical key with different values; config validation refuses the
same across cap.min.* keys; dispatch needs no raw provenance.

## D-2. Transport: provisioning writes what the host will read (folds CAPS-R1-002)

The mission runner does not dispatch delegates; the HOST does, inside the
target. So the roster's caps travel as configuration, not prompts: the
spec manifest gains a TOP-LEVEL map `"delegateCaps"` whose keys are the
RAW `"runtime:model"` strings and MUST each match a roster delegate entry
(unmatched keys refuse at provisioning), whose values are positive
integer minutes, and whose absence for a pair is legal — that pair uses
`fence.job-cap-min` (the roster string map is unchanged — no schema
migration), and
PROVISIONING writes the keys into the generated mission contract's
```mission block ONLY — and each pair cap becomes a `sealed.exposure.cap.min.<runtime>.<model>` seal entry, part of the human-visible exposure they sign (R2-008) — no configuration copy exists, so nothing can
drift and nothing needs reconciling (round 2 killed the duplicate: two
readable sources is two authorities). For a MISSION job, dispatch
resolves the pair cap FROM THE SEALED CONTRACT via the mission fence
state; config cap keys are IGNORED for mission jobs, and the refusal
message says so when one is present.

## D-3. Deadlines, not just durations (folds CAPS-R1-005/006/008/009)

- At LAUNCH, dispatch computes the job's absolute `capDeadline` =
  startedAt + resolved cap, TRUNCATED to the mission's wall-clock end
  when a mission fence applies — the end derived from ONE clock, the
  persisted fence counters' startedAt (the signed mission's own start),
  never the lease's or runner's, which reset on resume and would
  silently extend the signed fence. A remainder below TWO minutes refuses
  the dispatch ("mission has N seconds of wall clock; refusing to start
  a job that cannot run") — ONE threshold, used verbatim by the proof
  (the draft's text and proof disagreed at 90 seconds). The reaper enforces `capDeadline` when
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
maximum of EVERY resolvable cap source (contract pair caps, config pair
and role-pair caps, dispatch.cap-min, fence.job-cap-min, built-in) plus
30 minutes, WRITES it into the supervision state file, and the watcher
reads it from there. Dispatch compares against THE STATE FILE'S recorded
ceiling — what the live watcher actually enforces — never a recomputed
value; raising a cap past it requires re-arming, and the refusal says so
(R2-002's lifecycle).

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
terminations that are COMPLETED AND CERTIFIED (the delegation-floor bar)
produce completion times; an uncertified natural return classifies
PARTIAL (R2-009). Sampling is per implementer CHAIN (root jobId keys the
series; rounds continue it), samples schema {ts, jobId, newestMtime,
fileCount} (R2-011). PREDECLARED DERIVATION (R2-010): standing cap =
ceil(max qualifying completion minutes x 1.5) rounded up to the nearest
5, recorded as a ruling; only-stalled/partial/censored outcomes set NO
cap and escalate to the human with the distributions. A censored 150-minute result does
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
- Deadlines: a job launched with less than two minutes of mission wall
  clock left refuses (the D-3 threshold, verbatim);
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

## Round 2 findings, recorded verbatim-in-substance for the fold (11 material)

- CAPS-R2-001 (high): Section D-2 still defines two competing runtime authorities for a mission pair cap. Section D-1 says the pair cap exists only in the signed contract, but D-2 says the host resolves it from the target's mutable metasystem.conf exactly like an ordinary dispatch. Equality is checked only during preflight; the design does not say what a later dispatch does if base configuration drifts, nor how a resealed contract amendment updates the duplicate configuration. An implementer could trust the contract, trust configuration, take the smaller value, or refuse every mismatch. Those choices produce [...]
- CAPS-R2-002 (high): Section D-5 does not provide an executable watcher-ceiling algorithm or lifecycle. The fixed margin has no value or rule, and the stated maximum omits the role-specific non-mission caps defined in D-1, the existing configured dispatch.cap-min fallback, the built-in fallback named in D-4, and a rule for explicit non-mission arguments. More importantly, the arm-once watcher reads its ceiling only when launched and arming merely joins an existing live watcher; nothing refreshes it when a contract or configuration changes, and dispatch has no authoritative record of the ceiling the live [...]
- CAPS-R2-003 (high): Section D-3 and its proof prescribe opposite outcomes for the same remaining wall time. D-3 refuses only a sub-minute or negative remainder, so ninety seconds remaining is sufficient to launch a job with an absolute deadline ninety seconds away. The proof instead requires a job launched with ninety seconds remaining to refuse. An implementer must choose which outcome and test assertion is authoritative.
- CAPS-R2-004 (high): Section D-3 does not name the authoritative timestamp from which the mission wall-clock end is calculated or the component that exposes that boundary to dispatch. The current system has two plausible mission start timestamps: persisted fence counters retain the original mission start, while a live lease receives a new startedAt value whenever a mission runner starts or resumes. An implementation that uses the live lease after a resume would silently extend the signed wall-clock fence and produce a later capDeadline.
- CAPS-R2-005 (medium): Sections D-1 and D-4 leave the existing general non-mission cap unresolved. Current dispatch reads dispatch.cap-min from the normal configuration chain before falling back to 120 minutes. D-1 introduces role-and-pair and pair-specific keys but does not state whether dispatch.cap-min remains the next fallback, while D-4 has no provenance rule capable of representing a configured general fallback. One implementer may preserve the existing behavior, another may remove it, and a third may falsely label it built-in or config-pair.
- CAPS-R2-006 (high): Section D-1a still does not define one canonical encoding or an enforceable collision boundary. Its written algorithm differs from the Devin implementation it says will be extracted because the adapter strips edge hyphens and the design does not. Valid model identifiers may end in punctuation, so the difference changes cap keys. In addition, D-1a assigns collision refusal to dispatch after D-2 has already transported only canonical cap.min keys; at that point the distinct raw model spellings and their separate values may already have been collapsed. The design must decide whether [...]
- CAPS-R2-007 (medium): Section D-2 does not finish the delegateCaps manifest schema. It does not give an exact JSON location, state whether keys are raw or canonical runtime-and-model pairs, require positive integer minute values, or say whether entries must correspond to roster delegates and whether missing entries are legal fallbacks. These decisions determine which manifests provisioning accepts and which cap keys become human-signed contract terms.
- CAPS-R2-008 (medium): Sections D-1 and D-2 do not specify how pair caps participate in the generated mission seal. The current contract has an authored mission block, a generated mission-seal block, and an approval hash over all authored bytes. The revised design says pair caps are sealed and signed but names only their authored key grammar; it does not say whether they also become sealed.exposure entries or remain absent from the generated exposure statement. Because a pair cap may exceed fence.job-cap-min, those alternatives produce materially different contract schemas and human-visible exposure records.
- CAPS-R2-009 (high): Section D-6 equates a natural delegate termination with a completion time, but the current delegate status does not mean the benchmark task completed. A job is marked completed when the runtime returns schema-valid JSON; it may return naturally with an incomplete implementation that later fails the gate or receives a poor held-out grade. The phrase 'same gate and grader' does not define which gate or grading outcome qualifies a natural termination for the completion-time sample. Counting every natural return would discover a cap for producing a reply, not a cap for completing the work.
- CAPS-R2-010 (high): Section D-6 never converts the experiment's result into the production pair cap that is the design's goal. It gives an experimental ceiling of 150 minutes and defines censoring, but it does not say how a qualifying completion time becomes an integer cap, what rounding and safety margin apply, or what decision follows a STALLED or CENSORED result. Different implementers could choose the observed duration, round it up, multiply it by a margin, retain 150, or decline to set a cap, so the promised measured discovery still does not determine what will be built.
- CAPS-R2-011 (medium): Section D-6 still lacks a complete measurement contract for chains and classifications. It names a JSON Lines artifact, cadence, and timestamps, but supplies no field schema, no rule for selecting the implementer record when there are multiple jobs or follow-up rounds, and no precedence when a job is declared STALLED after forty-five quiet minutes but later terminates naturally. It also does not say whether STALLED stops the observation or merely labels an interim state. The same run can therefore yield different artifacts and final classifications depending on the implementer's choices.

## Round 3 findings, recorded for the decision (10 material; 001 CRITICAL)

- CAPS-R3-001 (critical): CAPS-R3-001, the signed-byte freshness gap in Sections D-1 and D-2: the design does not say how a later dispatch proves that the pair cap it reads is still part of the sealed and approved contract. Preflight is a one-time check, while the proposed runtime path is the mission fence state. That state currently contains counters but no approved contract hash, and the fence reader parses authored values without verifying the seal or approval. Implementers could trust the [...]
- CAPS-R3-002 (high): CAPS-R3-002, the duplicate-authority proof contradiction in Section D-2 and Proof: D-2 says no configuration copy exists and provisioning writes the cap only into the mission contract. The Proof section instead requires the manifest entry to land in both target configuration and the sealed contract and requires mismatch preflight. Following the proof would restore the competing mutable authority that round-two finding CAPS-R2-001 removed; following D-2 makes the [...]
- CAPS-R3-003 (high): CAPS-R3-003, the incomplete watcher lifecycle in Section D-5: the derived maximum still excludes explicit non-mission cap arguments even though D-1 says those arguments win the normal trust chain. Re-arming cannot derive a value that is supplied only at dispatch. In addition, the design does not say whether re-arming must replace a live watcher or may join it unchanged. With the 120-minute built-in cap and the specified 30-minute margin, arming records 150 minutes; an [...]
- CAPS-R3-004 (high): CAPS-R3-004, the false completion predicate in Section D-6: completed-and-certified under the named delegation-floor rule does not establish that the benchmark task completed. The existing rule merely requires a completed delegate return that the host marked accepted for a stream. It does not join that job or chain to a passing mission gate or any held-out grading threshold. A quickly returned, useful but partial patch can therefore enter the completion-time maximum and [...]
- CAPS-R3-005 (high): CAPS-R3-005, the unresolved chain and STALLED semantics in Section D-6: the design says sampling starts and ends at one job record but also says one root job identifier keys a series continued across follow-up rounds. It does not say whether each sample's jobId is the root or current round, whether completion minutes cover only the qualifying round or the entire root-to-final chain interval, or how gaps between rounds count. It also still gives no precedence when a [...]
- CAPS-R3-006 (high): CAPS-R3-006, the missing timeout-to-contract-term mapping in Sections D-3 and D-4: only a wall-clock-truncated timeout has a defined fence refusal. A timeout at a contract pair cap should identify the pair term, while a timeout at a lower explicit argument has exhausted no signed contract term at all. The current reaper always files job-cap-min, which names the universal fallback rather than cap.min.<runtime>.<model>. Implementers can therefore file an ask against the [...]
- CAPS-R3-007 (medium): CAPS-R3-007, the incomplete provenance schema in Sections D-1 and D-4: D-1 explicitly names config-general as the rule for the preserved dispatch.cap-min fallback, but D-4's closed rule enumeration omits config-general. A conforming implementation must either emit a value rejected by the written schema or mislabel a configured general fallback as config-pair or built-in, and the promised full-object provenance fixture cannot be written consistently.
- CAPS-R3-008 (medium): CAPS-R3-008, the impossible collision proof in Sections D-1a and Proof: D-1a says collisions are refused where both raw spellings still exist—during manifest provisioning or configuration validation—and explicitly says dispatch retains no raw provenance. The Proof section nevertheless requires two colliding models with different values to refuse at dispatch. An implementation can satisfy that test only by retaining an additional raw-value authority that D-1a rejects, [...]
- CAPS-R3-009 (medium): CAPS-R3-009, the deadline field-location conflict between Sections D-3 and D-4: D-3 names capDeadline as the field the reaper reads when present, but D-4 says the record gains one capResolution object containing a differently named nested deadline field. An implementer can add a top-level capDeadline, add only capResolution.deadline, or duplicate both. Those schemas make the reaper read different locations and create different compatibility behavior for existing records.
- CAPS-R3-010 (medium): CAPS-R3-010, the incomplete unsigned-input matrix in Sections D-1 and D-2: the design refuses an explicit mission --cap-min only when it is above the contract value, but never states whether an equal or lower argument becomes the effective cap, is ignored, or is also refused. D-2 similarly says configuration cap keys are ignored and then refers to a refusal message when one is present, without defining whether matching base configuration, unrelated pair keys, or both [...]
