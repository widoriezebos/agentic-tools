# Budgets belong to the binding: per-model delegate caps

- Goal and current status: a delegate job's time budget is declared where
  its runtime and model are declared — keyed on the (runtime × model)
  pair, optionally sharpened per role. Load-bearing rule, established
  across the critique chain: THE SIGNED CONTRACT IS THE ONLY AUTHORITY
  THAT CAN RAISE A MISSION JOB'S BUDGET, and the sovereign MISSION FENCE
  (not dispatch) enforces it.
- Next step: none
- In flight right now: nothing
- Waiting on the human: nothing

The authority-core implementation is DELEGATED TO CODEX (by the human,
2026-08-09) and running as an external coding agent — not a
metasystem-tracked job, so it is deliberately not claimed as an in-flight
job above. Codex builds against the resolutions in this plan and leaves
the work in the tree for review; the orchestrator does not touch the same
files while it runs, and reviews the result when it returns.

CHAIN HISTORY. The main caps chain ran 14 → 11 → 10 material and, on a
critical authority finding, the human ruled a SPLIT (authority core
first). The authority-core chain then ran 6 → 9 — not converging in
prose, but with findings dropping to interface/implementation grain. The
human authorized a fixtures-as-arbiter CLOSE: build the authority core
with ADVERSARIAL fixtures that attack the invariant, plus mandatory
code-critique, to settle the two remaining criticals — a deliberate,
recorded deviation from the usual no-invariant-grade-findings condition.
The nine round-2 findings' resolutions are fixed in the resolutions
section below; each becomes a named fixture. The experiment and
measurement legs (R3-004..R3-010) remain for their own later chain.

## Why, with the evidence



bm-2's uniform 15-minute cap killed a Devin/swe-1-7 implementer holding
1,322 compiling lines; the mission shipped a skeleton (acceptance 1/53,
both repetitions). Not the model, not the CLI, not a bug: the
configuration. The human ruled: cap keyed on (runtime × model), declared
where bindings are declared, and Devin's number DISCOVERED by one
instrumented run, not guessed.

## D-1. The mission fence owns pair-cap authority (folds AUTH-001/002, R1-001/003/004)

Round 1 of the authority chain corrected the draft's core mistake: caps
were treated as a DISPATCH-resolution concern, but the mission FENCE is
the sovereign, so the fence must be the component that knows and enforces
pair caps. The revised model:

- The mission fence (`mission-fence.py`) is extended to accept the job's
  (runtime, canonical-model) and to authorize a per-pair cap that MAY
  exceed the general `fence.job-cap-min`. It authorizes this because it,
  the sovereign, has verified the signed contract (D-2) — dispatch never
  becomes a competing authority. The fence's existing "reject any cap
  above job-cap-min" check is replaced by "reject any cap above the
  SIGNED pair cap for this runtime+model, or above job-cap-min when the
  pair has none".
- Dispatch ASKS the fence for the authorized cap; it does not resolve
  mission caps itself. For a mission job the answer comes only from the
  fence's verified contract state; unsigned inputs (`--cap-min` above the
  authorized value, `.local`/env cap keys) are refused, and the refusal
  names the fence as the authority.
- NON-MISSION JOBS keep today's config trust chain unchanged
  (`cap.min.<role>.<runtime>.<model>` > `cap.min.<runtime>.<model>` >
  `dispatch.cap-min` > built-in; explicit `--cap-min` wins), because no
  contract and no sovereign fence exist to defer to.

## D-1a. Canonical key encoding (folds R1-014, AUTH-006)

Cap keys use the CANONICAL MODEL FORM from one extracted helper
(`scripts/agents/canonical-model.py`, lifted verbatim from the devin
adapter: lowercase, non-[a-z0-9] runs to one hyphen, edge hyphens
stripped — the helper is the single truth, this prose defers to it).
Config VALIDATION refuses ANY `cap.min.*` key that is not already
canonical (AUTH-006: a solitary non-canonical key like
`cap.min.devin.swe-1.7` is a loud error, never normalized and never
silently ignored into a fallback), which also subsumes collision refusal:
two raw spellings cannot both be present if neither non-canonical form is
accepted. Provisioning refuses a manifest whose raw `runtime:model`
strings collapse to one canonical key with different values.

## D-2. Contract-only transport, with freshness on EVERY signed limit (folds AUTH-002/003, R1-002, R2-001/008)

- The HOST dispatches inside the target, so caps travel as a signed
  CONTRACT, never configuration. The manifest gains a top-level
  `delegateCaps` map (raw `"runtime:model"` keys matching roster
  delegates, positive-integer minutes, absence legal → job-cap-min);
  provisioning writes them into the generated contract's ```mission block
  AND a `sealed.exposure.cap.min.<runtime>.<model>` seal entry, and
  NOWHERE ELSE. No configuration copy exists (R2-001).
- THE PIN GOVERNS EVERY SIGNED LIMIT, not just pair caps (AUTH-002): the
  fence state carries `approvedContractSha256`, and the fence verifies
  the live contract file against it on EVERY metering call before reading
  ANY signed value — pair cap, the fallback `fence.job-cap-min`, and the
  `fence.wall-clock-hours` D-3 consumes. An unsigned post-preflight edit
  cannot raise a pair cap, the fallback cap, OR extend the wall clock.
- THE PIN'S LIFECYCLE is explicit (AUTH-003): preflight verifies the
  signed+approved bytes and, in the SAME step that consumes its result,
  hashes THOSE verified bytes into `approvedContractSha256` — never a
  later re-hash of a file that could have drifted between preflight and
  fence-state creation. The only legal way to change the pin is the
  existing amend → reseal → sign → preflight → resume flow, whose resume
  re-pins from its own fresh preflight; any other change to the contract
  bytes makes the fence refuse until re-approval.

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
value; raising a cap past it requires re-arming, and the refusal says so.
The lifecycle, completed against AUTH-004/005: the watcher, AFTER loading
its ceiling at startup, ATTESTS it by writing the loaded value into its
own heartbeat; dispatch compares a job against the ATTESTED ceiling the
live watcher actually enforces, not a freshly derived number, so there is
no compute-new-enforce-old split. Re-arming is a PUBLIC establishing
operation (`arm-supervision.sh --rearm`) that replaces the live watcher
and re-derives; a join never changes the ceiling. Arming accepts
`--max-cap <min>` so an operator raising a cap beyond every configured
source has a declared input, and an explicit `--cap-min` is bound by the
attested ceiling exactly like a configured one — the trust chain lets an
argument win among CONFIGURED values, never against the live watcher.

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
- Transport: a manifest delegateCaps entry lands in the sealed contract's
  mission block and its sealed.exposure entry, and NOWHERE else — the
  round-2/round-3 rule: the contract is the only transport, so the proof
  asserts the ABSENCE of any cap key in the target configuration.
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

## Authority-core round 1 findings (6 material, 3 CRITICAL) — the next fold

- CAPS-AUTH-R1-001 (critical): CAPS-AUTH-R1-001, Sections D-1 and D-2 do not give the sovereign mission fence a way to authorize a signed runtime-and-model-specific cap above the general fence.job-cap-min value. The current fence independently rejects every cap above that general value and receives neither the runtime and model nor proof of the approved resolution. An implementer must therefore either retain the check, making the design's raised pair caps unusable, or weaken/remove it and make dispatch the sole authority, contrary to the [...]
- CAPS-AUTH-R1-002 (critical): CAPS-AUTH-R1-002, Section D-2 limits approved-contract freshness checking to dispatches that honor a pair-specific cap. A mission with no pair entry instead uses the signed fence.job-cap-min fallback, and Section D-3 also consumes the signed wall-clock duration. Because the current mission fence rereads both values from the live authored block without seal or approval verification, an unsigned edit after preflight could still raise the fallback cap or extend the wall-clock end. The standing hash must govern [...]
- CAPS-AUTH-R1-003 (critical): CAPS-AUTH-R1-003, Section D-2 does not define an atomic handoff from successful preflight to approvedContractSha256, or the legal transition for replacing that digest after a human-approved contract amendment. Under the current lifecycle, preflight finishes before a separate child creates or reads fence state, so hashing the file during initialization can pin bytes changed after preflight. Conversely, retaining the initial digest makes the existing amend, reseal, sign, and resume flow reject a legitimately [...]
- CAPS-AUTH-R1-004 (high): CAPS-AUTH-R1-004, Section D-5's replace-on-re-arm lifecycle is not callable as designed. It distinguishes an establishing re-arm from a join but supplies no public operation or rule that tells arming which transition the caller requested, including whether providing --max-cap forces replacement or whether the operator must perform shutdown followed by a fresh arm. Implementers could make every arm replace live supervision, make --max-cap replace it, or leave ordinary calls joining unchanged; those choices [...]
- CAPS-AUTH-R1-005 (high): CAPS-AUTH-R1-005, Section D-5 and its Proof do not join the recorded watcher ceiling to the value the live watcher actually loaded. The design says dispatch trusts the state file because it represents live enforcement, but the current watcher reads configuration at process startup and supervision writes state only after launching it. An implementation could update the state and dispatch checks while leaving the watcher on its old ceiling, yet still satisfy the stated proof by inspecting the derived state [...]
- CAPS-AUTH-R1-006 (medium): CAPS-AUTH-R1-006, Section D-1a defines collision refusal but leaves a solitary noncanonical configuration key without an outcome. A valid raw model such as SWE-1.7 can be written as cap.min.devin.swe-1.7; current validation accepts that key, while canonical dispatch lookup asks for cap.min.devin.swe-1-7. With no second spelling there is no collision to refuse. One implementer may reject the noncanonical key, another may normalize it, and another may silently ignore it and use a fallback, producing different [...]

The shape these force: the mission FENCE, not dispatch, must be the
component that knows and enforces pair caps — it receives the runtime,
model, and the pinned approved hash, verifies ALL signed limits it
consumes against that hash on every metering call, and the pin's
lifecycle (atomic handoff at preflight, legal replacement on an
approved amendment) becomes part of the fence state contract.

## Authority-core round 2 findings (9 material, 2 critical) — DECISION POINT

- CAPS-AUTH-R2-001 (critical): CAPS-AUTH-R2-001, the unresolved fence authorization interface in Sections D-1 and D-3: the design says both that the fence rejects a caller-supplied cap above the signed limit and that dispatch asks the fence for the resolved cap. It defines no request and response contract for an omitted cap, an equal or lower explicit cap, the signed fallback, or the verified wall-clock end. A minimal implementation could [...]
- CAPS-AUTH-R2-002 (critical): CAPS-AUTH-R2-002, the non-atomic verified-byte transaction in Section D-2: verifying a path before reading its signed values does not prove that the parsed values are the bytes whose hash matched the pin. The design also does not require reading the pin, hashing and parsing one immutable snapshot, and reserving against it under the same fence lock. An implementer can therefore follow the written order by [...]
- CAPS-AUTH-R2-003 (medium): CAPS-AUTH-R2-003, the undefined approved-contract hash domain in Section D-2: the design alternates between a hash of the contract file, the signed and approved bytes, and the bytes already verified by preflight. It never states whether approvedContractSha256 is the raw-file Secure Hash Algorithm 256-bit digest or the existing canonical signed-content digest. Those domains differ because the existing digest [...]
- CAPS-AUTH-R2-004 (high): CAPS-AUTH-R2-004, the unenforceable re-pin transition in Section D-2: the text says preflight writes approvedContractSha256, but also says only resume may re-pin after amendment. It does not identify the pin-writing command, required mission state, lock, or caller authority that distinguishes a resume-owned preflight from the public preflight used elsewhere. One implementation can make every successful [...]
- CAPS-AUTH-R2-005 (high): CAPS-AUTH-R2-005, the contradictory watcher bootstrap and authority record in Section D-5: the first half says dispatch compares the ceiling recorded in supervision state and that the watcher reads that file at startup; the second half says dispatch compares the value attested in the watcher heartbeat. These records are distinct, and the current state schema cannot be the watcher's startup input because it is [...]
- CAPS-AUTH-R2-006 (high): CAPS-AUTH-R2-006, the unsafe downward re-arm in Section D-5: re-arming derives a new ceiling only from current configuration, contracts, defaults, and an optional maximum-cap argument; active delegate-job records are not included. A 300-minute explicit job can legally start after arming with a matching maximum-cap input, then a later re-arm without that input can load a 150-minute ceiling while the 300-minute [...]
- CAPS-AUTH-R2-007 (medium): CAPS-AUTH-R2-007, the undefined contract population for watcher derivation in Section D-5: supervision is repository-wide and arming receives no mission identifier, yet its maximum includes contract pair caps and fence.job-cap-min. A repository can contain multiple active, dormant, unsigned, stale, or amended mission contracts. The design does not say which contracts qualify, nor whether --max-cap is itself a [...]
- CAPS-AUTH-R2-008 (medium): CAPS-AUTH-R2-008, the incomplete noncanonical-key refusal in Section D-1a: saying configuration validation rejects every noncanonical cap.min key does not define whether this includes metasystem.conf.local and environment-backed configuration, both of which Section D-1 retains for non-mission cap resolution. Updating only the existing validator leaves a solitary noncanonical local key silently ignored into a [...]
- CAPS-AUTH-R2-009 (high): CAPS-AUTH-R2-009, the stale authority proof in the Proof section: none of the new load-bearing claims has a corresponding assertion that the fence accepts a signed pair cap above fence.job-cap-min, all fence-read signed limits fail after byte drift, a generic preflight cannot re-pin a live mission, resume can legally re-pin an approved amendment, or dispatch compares a fresh identity-matched heartbeat [...]

Trajectory: authority-core round 1 = 6 material, round 2 = 9 — NOT
falling. Per IL-23 (two rounds without a falling count) and the fact
that the caps chain was ALREADY split once by human ruling, this is a
recorded escalation, not a unilateral round 3. The findings have shifted
from architecture (round 1: 'the fence must own caps' — accepted and
folded) to INTERFACE/IMPLEMENTATION grain (round 2: the exact
request/response of the fence call, the exact read-hash-parse-reserve
locking transaction, the exact hash domain raw-vs-canonical). That grain
is where code plus fixtures judge better than prose — EXCEPT two findings
are still critical, so the fixtures-as-arbiter exit is not automatically
available. The human's call.

## Authority-core resolutions (folds AUTH-R2-001..009; each is a fixture)

- R2-001 FENCE INTERFACE: dispatch calls `mission-fence.py authorize-cap
  --runtime R --model M [--requested N]` and receives `{capMin,
  capDeadline, source}` or a refusal. The FENCE selects the signed pair
  cap even with no `--requested` (an omitted cap does not fall back to
  job-cap-min when a pair cap is signed); `--requested` at or below the
  authorized value is honored as-is; above it refuses. Dispatch never
  resolves a mission cap itself. Fixture: signed 150 pair cap is selected
  with no argument; requested 200 refuses; requested 90 honored.
- R2-002 ATOMIC TRANSACTION: under the fence state lock, in this order and
  on ONE in-memory snapshot — read `approvedContractSha256`; read the
  contract file bytes ONCE into a buffer; hash the buffer; compare to the
  pin; parse the SAME buffer; reserve. No reopen between hash and parse.
  Fixture: a contract file swapped AFTER the buffer read but before
  reserve changes nothing (the buffer governs); a swap that changes the
  pinned bytes on the next call refuses.
- R2-003 HASH DOMAIN: `approvedContractSha256` is the RAW-FILE sha256 of
  the exact on-disk bytes, NOT the canonical signed-content digest —
  drift detection needs exact bytes, including the Approval line and
  trailing whitespace. Stated in the field's definition. Fixture: a
  trailing-whitespace-only edit refuses.
- R2-004 RE-PIN: the pin is written ONCE, by the mission runner at start
  and at resume, from the bytes preflight just verified, in the same
  invocation (no independent re-hash). Amendment flows through amend ->
  reseal -> sign -> preflight -> resume, and resume re-pins. Fixture: an
  amended+resigned+resumed contract re-pins and is honored; an amended
  but UNsigned contract refuses at resume.
- R2-005 WATCHER AUTHORITY RECORD: the ATTESTED ceiling in the watcher
  heartbeat is the sole authority dispatch reads; the derived value in
  supervision state is an input to the watcher, not the dispatch
  reference. Fixture: dispatch refuses against the attested value even
  when state carries a higher derived number.
- R2-006 DOWNWARD RE-ARM: `--rearm` refuses to lower the ceiling below
  any currently-reserved cap (a live job's budget cannot be
  retroactively invalidated); it names the blocking job. Fixture: a
  re-arm below a reserved cap refuses.
- R2-007 WATCHER DERIVATION SCOPE: supervision is repo-wide and arming
  does not enumerate contracts, so the derived ceiling comes from the
  CONFIG cap sources plus `--max-cap`, NOT from contracts; a mission
  needing more raises it explicitly at its arming via `--max-cap`.
  Fixture: arming with no --max-cap derives from config only; --max-cap
  raises it.
- R2-008 NON-CANONICAL REFUSAL: config validation refuses a non-canonical
  `cap.min.*` key, the message naming the offending key AND its canonical
  form. Fixture: `cap.min.devin.swe-1.7` refuses naming `swe-1-7`.
- R2-009 PROOF: every resolution above ships with the named fixture; the
  suite gains a delegate-caps fixture file, and the authority fixtures are
  ADVERSARIAL (they attempt the bypass and assert refusal), per the close
  rule.
