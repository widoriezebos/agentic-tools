# witness-law-extension — design brief (round 0)

Intent (goal ledger): the witness law — proven bytes need not be
re-proven — extends beyond the delivery-contract run type, so battery
and gate runs stop paying full price for content already witnessed.
This is the efficiency half of the battery program (~1h47 full run
today; the target is a materially shorter re-run when little changed).

## The law today
A witness records surface digests (ENGINE/PAYLOAD projections +
toolchain identity) for a proven tree. go-gate.sh's witness fast path
consults the behavior-surface policy (witness-engine-gate skip family,
policy-owned since the freeze landing) and rebuilds-without-re-proving
on digest equality. Only delivery-contract runs honor it; the isolated
battery's clone validate re-proves everything, always.

## Proposed extensions (design space for critique)
1. BATTERY WITNESS: a green battery records a witness for its subject
   (projection digests + policy version + toolchain). A later battery
   whose subject carries EQUAL projections may skip the sections the
   policy names as witnessable, running only the delta-relevant ones.
   The enumeration mode's section registry is the natural unit.
2. SECTION-LEVEL WITNESS: witnesses per suite section (section name +
   input-closure digest), so a subject differing only in docs/ skips
   engine-heavy sections lawfully. The policy file owns which sections
   may witness-skip (S4 discipline: enumerated by name, versioned).
3. INVALIDATION: any policy-version change, toolchain change, or
   landing that touches a section's input closure invalidates that
   section's witness. Witness records live in the evidence root
   (machine-local, never in the repo).
4. HONESTY BOUND: a witness-skipped section is REPORTED as skipped-by-
   witness in the envelope (never silently green); the acceptance
   battery after landings that touch engine paths always runs full.

## Constraints
- The behavior-surface policy is the single owner of skippability
  (no new skip authority outside it; version law per S4).
- Witness records are runtime accelerators: correctness must survive
  their deletion (worst case = full run, never a wrong skip).
- Isolated-clone semantics unchanged; evidence envelopes complete.
- Comments/laws in repo voice; no python; fixtures prove the skip
  discriminates (a touched closure MUST re-run — fail-on-broken).

## Appetite
Design: 3 critique rounds max (severity discipline: BOUNDED class —
efficiency work, no proof-integrity change without policy version).
Build: 1 day.

## Round 1 verdict (needs-rework) — adopted direction
SLICE ONE (adopted as the build target, 4-6h): split the witness
decision into two predicates. ENGINE WITNESS — existing permissions,
containment, run-id, refusal, policy, ENGINE digest, Go-toolchain
equality — honored in ANY descendant validation under the same
controller's full outer gate (not only --delivery-contract), skipping
only the duplicate Go gate; bin/metasystem still rebuilt on the fast
path. DELIVERY REUSE — retains the additional PAYLOAD equality and
rebuilt-binary stamp before any delivery-contract family skips. No
persistent witness store, no section closures, no envelope changes,
no cadence changes. Fixtures (fail-on-broken): matching ENGINE +
changed PAYLOAD skips only the duplicate Go gate; changed ENGINE runs
full; foreign run-id/root refuses; deleted witness falls back full.
LAW BOUNDS from the round: witness-assisted runs never reset battery
weight and never serve as conclusion acceptance (standing full-battery
ruling); cross-run/persistent witnessing is OUT until a
behavior-complete closure, complete invalidation identity, and
isolation/evidence-compatible store are designed (rounds 2-3 scope if
pursued at all). RECORD REPAIR done this round: the freeze brief and
obligation table now persist at plans/gate-run-freeze-{brief,
obligations}.md (they had lived only in a session scratchpad — the
critic could not verify EVD/INT compliance; now it can).

## Round 2 amendments (adopted; round 3 decides)
A1 CONTROLLER IDENTITY: the same-controller predicate is proven by
live process ancestry against the controller's recorded exact
identity (pid + start, the fence.go machinery) — run-id stays
correlation only; borrowed env never qualifies.
A2 PRODUCER PLACEMENT: the isolated clone's ROOT validation is the
witness producer; savings apply only to its NESTED validations.
Controller-produced witnesses are out; RUN-08/RUN-09/SUR-17 bind
(no copy-back, producer inside the detached clone).
A3 PROSPECTIVE POLICY AUTHORITY: skip authorization comes from an
engine built from the consuming bytes (or the prospective source
engine), never the pre-existing binary (SUR-17/20/21).
A4 RUN CLASSIFICATION: a FULL battery = root-owned full engine proof
+ descendant-only deduplication; a root that imports proof is
WITNESS-ASSISTED. The classification is recorded structurally and
carried to weight reset; conclusion acceptance binds to FULL
mechanically (or the conclusion rule is explicitly procedural —
round 3 decides which).
A5 FIXTURES: every refusal an isolated leg (ancestry, explicit
consume scope, policy version, stale-binary policy, toolchain,
foreign run-id, foreign root, deleted witness, broken-proof full
fallback, reset provenance); each fallback leg must fail on a defect
only the skipped proof could catch.

## Conclusion and FULL acceptance rule

`goal done` remains procedurally bound rather than mechanically coupled to a
battery certificate. A conclusion that relies on milestone-battery acceptance
must cite a FULL run. Human-authorized and non-battery conclusions keep their
existing authority and blocker checks; adding a goal-to-battery association or
certificate protocol is outside this witness extension.

A FULL battery is one whose isolated validation root performs the full engine
proof itself. Witness deduplication used only by descendants under that same
controller does not change the root's FULL class. A root that imports engine
proof is WITNESS-ASSISTED. Both classes publish their root-owned classification
in run evidence, but only FULL may consume the battery-weight checkpoint.
WITNESS-ASSISTED terminalizes by abandonment and subtracts no accumulated
weight.
