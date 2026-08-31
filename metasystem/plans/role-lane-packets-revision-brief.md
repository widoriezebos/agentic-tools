Working Mode: design
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal role-lane-packets)
Date: 2026-08-31

# Goal

Revise plans/role-lane-packets-design.md to answer the Sol design
critique below (job design-critic-38f304b59d2bb62c9e1711ab, one round,
all five threat-model lines FAILED, two critical findings). The
critique is relayed whole and verbatim per R-25b-m1 — the orchestrator
has changed nothing in your design and proposes nothing; every fold or
refusal is yours.

Verified by the orchestrator before this dispatch: rulings R-31-m2 and
R-34-m2 exist in memory/rulings.md exactly as the critique cites them
(the Sol implementation-lane model pin; no model substitution on any
lane without Wido's explicit per-case approval). Finding RLP-R1-001's
authority premise is therefore confirmed, not merely claimed.

# Workspace

Your prior worktree. Revise exactly one file:
plans/role-lane-packets-design.md. A finding you reject needs a
recorded refutation with evidence in the design's own text; a finding
you accept changes the design. If the 20-minute budget cannot carry a
sound fold of everything, fold in severity order (critical first) and
say plainly in the design's status line what remains unfolded — an
honest partial fold beats a rushed complete one.

# Constraints

- Wall-clock budget: 20 minutes (the goal's remaining reserved pool).
- The revised design keeps its R-24-m1 self-grade current, including a
  fresh weakest-claim and reject condition.
- Update the design's status line: revised against critique round 1,
  awaiting re-critique.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/role-lane-packets-design.md; whatWasDone names each
finding folded or refuted.

# Gap Rule

stop and report a gap; never fill it silently.

## Verdicts (five threat-model lines)

### Threat-model line 1 verdict — correctness against the rulings
FAIL. The four principal R-25-m1 family assignments are represented, and effort remains outside the lane object, but the design still permits any same-runtime model even though the later R-31-m2 and R-34-m2 rulings narrowed the earlier R-28-m1 delegation. It also fails to bind examiner type to reviewed work mode.

### Threat-model line 2 verdict — enforcement reality
FAIL. Fresh correction follow-ups have no derivable mode, code-critic and warden launches are not bound to the reviewed implementer's mode, and the proposed typed equality accepts forbidden or unknown zero-value JSON members. Canonical model aliasing alone is harmless only because the proposed family check ignores models entirely.

### Threat-model line 3 verdict — the warden claim
FAIL as a universal fixed-family claim. Claude is justified when the warden examines implementation-mode work, but current admission lets the warden examine any implementer job, including Claude-authored design-mode work. The proposed fallback is closure-sound under today's one-runtime-per-family premise because it rejects equal runtimes, but it is not launch-sound: the wrong-family examination can spend its budget before closure refuses.

### Threat-model line 4 verdict — configuration versus packet precedence
FAIL. Existing verifier, investigator, design-critic, implementer, and code-critic keys have determinate behavior, but day-one warden behavior is knowingly non-operational: it selects claude-opus-5 while the maximal-effort gate accepts only claude-fable-5, and the required machine-local correction is deferred outside slice 1.

### Threat-model line 5 verdict — sliceability
FAIL. Independent deployment is disproved by the broken follow-up path, the open examiner-to-work-mode channel, and the deliberately deferred warden configuration needed for governed launches. The claimed four-hour duration is also unsupported by a task-level estimate despite spanning the packet schema, engine mirror, roster and command wiring, composition provenance, refusal taxonomy, and six test groups.

### Self-grade under human ruling R-24-m1
Confidence is high on the follow-up, examiner-binding, configuration, and JSON-presence findings because each follows directly from a named deterministic boundary. Confidence is medium-high on the model-pin finding because it depends on reading the later R-34-m2 wording as narrowing R-28-m1 for every lane. The weakest claim is that model-pin scope. Reject that finding if a later human authority explicitly limits R-34-m2 to outage retries or evidence proves a separate pre-launch boundary already requires Wido's per-case approval for every model override.

## Findings, verbatim

### RLP-R1-001 (critical, material=True)

CLAIM: RLP-R1-001 — The design implements a model-substitution permission that current human authority revoked. Refuted text: “What never refuses: a lawful-family launch with any model” and “Benign variation follows for free: a new model within the lawful family ... cannot refuse on lane grounds.” An implementer following this design would leave a same-runtime model override admissible even though human ruling R-31-m2 pins the Sol implementation lane to gpt-5.6-sol and the later R-34-m2 ruling says no model substitution on any lane is allowed without Wido's explicit per-case approval. Treating R-31-m2 merely as an example configuration entry makes the design's authority premise stale.

EVIDENCE: metasystem/plans/role-lane-packets-design.md:94-109 and 270-273 deliberately exclude models from enforcement. metasystem/internal/dispatch/roster.go:182-231 currently accepts a model override and classifies it through the cost-escalation ladder rather than requiring Wido's authority. metasystem/memory/rulings.md:60 pins the Sol implementation lane, and line 64 later prohibits substitutions on every lane without explicit per-case approval. R-34-m2 was already present in the parent of the reviewed design commit.

### RLP-R1-002 (critical, material=True)

CLAIM: RLP-R1-002 — The design does not encode or enforce which examiner may review which implementer mode, so its claimed cross-family disjointness is false. Refuted text: “for every hazard class with IndependentCritiqueRequired, the builder lane's family differs from each examining lane's family” and “builder and examiner families are disjoint by construction for every dispatched path.” The proposed comparisons cover design-mode implementer versus design-critic and default implementer versus code-critic and warden, but admission never proves that a code-critic or warden is reviewing default-mode implementation. Either may review a design-mode implementer; both then lawfully launch on Claude against a Claude-authored design, spending budget on the same-family examination R-25-m1 forbids.

EVIDENCE: metasystem/plans/role-lane-packets-design.md:210-217 lists only selected static pairs, while lines 328-346 claim coverage of every dispatched path. metasystem/scripts/agents/dispatch.sh:1222-1227 checks only that a code-critic or warden's reviewed record has role implementer, not its Working Mode. metasystem/internal/dispatch/hazard.go:216-225 removes all critic roles from final-work selection, and lines 273-275 accept code-critic, design-critic, or warden without a target-mode relationship. The slice-2 equal-runtime closure check would detect this only after the examination has run; slice 1 permits it through closure as well.

### RLP-R1-003 (high, material=True)

CLAIM: RLP-R1-003 — The specified composition-time mode derivation makes every existing follow-up refuse. Refuted text: composition “re-derives the mode ... from p.Brief ... exactly one filled Working Mode header, refusing on zero or many,” while the design also says it does not touch brief templates. Fresh dispatch briefs carry that header, but follow-up composition receives the correction message, and the shipped follow-up template intentionally has no Working Mode header. Implementing D4.2 literally therefore disables correction rounds across all fixed-lane roles.

EVIDENCE: metasystem/plans/role-lane-packets-design.md:232-242 requires the header on every composition, line 353 excludes template changes, and lines 393-403 put this behavior in slice 1. metasystem/scripts/agents/dispatch.sh:1875 and 1888 pass the follow-up message as the composition brief. metasystem/scripts/agents/templates/follow-up.md:1-16 has no Working Mode header. Running the engine's exact parser against that template returned exit status 1 with no output.

### RLP-R1-004 (high, material=True)

CLAIM: RLP-R1-004 — Slice 1 knowingly changes the warden to a model that cannot satisfy the warden's governed effort gate, so the claimed day-one migration and independent deployment are false. Refuted text: “Zero conflicts, zero refusals” and “This alone discharges requirements 1–5.” Under packet precedence P3, the warden changes from the generic Codex runtime to Claude and selects the existing default Claude model, claude-opus-5. The maximal-model list contains only claude-fable-5, so every Ruling-O-gated warden composition refuses. The design calls a role-specific machine-local model line the remedy but expressly defers it outside the implementation slice.

EVIDENCE: metasystem/plans/role-lane-packets-design.md:303-323 both claims zero migration refusals and admits the governed warden refusal; lines 393-410 claim slice-1 discharge while putting the needed configuration in waits. The merged values read from metasystem/metasystem.conf and metasystem/metasystem.conf.local are role.default.model.claude=claude-opus-5 and runtime.claude.maximal-models=claude-fable-5. metasystem/internal/dispatch/hazard.go:91-105 refuses maximal work when that membership proof fails, and metasystem/internal/dispatch/composition.go:141-143 applies the refusal before composition.

### RLP-R1-005 (high, material=True)

CLAIM: RLP-R1-005 — The proposed typed mirror cannot enforce the design's own exact-equality and forbidden-field rules. Refuted text: for roster-authority rows, “family and modes are absent (and refused if present),” and any packet file the engine does not expect must refuse. With the proposed string and map fields decoded by ordinary encoding/json, family omitted and family set to an empty string produce the same Go value; modes omitted and modes set to null likewise produce the same nil map. reflect.DeepEqual therefore accepts forbidden present fields, while unknown members are silently discarded. The listed tamper tests do not cover these accepted mismatches.

EVIDENCE: metasystem/plans/role-lane-packets-design.md:154-159 requires presence itself to refuse; lines 172-188 define RoleLane with non-presence-tracking string and map fields; lines 202-205 rely on reflect.DeepEqual; and lines 372-376 omit present-empty, present-null, and unknown-member fixtures. The shipped reader at metasystem/internal/dispatch/composition.go:280-293 uses json.Unmarshal into typed structs without strict unknown-field rejection. The design itself acknowledges this decoder behavior at metasystem/plans/role-lane-packets-design.md:197-199 when justifying the version bump.