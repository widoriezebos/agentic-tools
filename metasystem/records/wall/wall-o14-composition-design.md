# Review brief: wall-o14 sealed-dirty composition

Round budget: 3 focused rounds — agreed before round one; exhaustion
follows the critique skills' budget rules, never a silent round 4.

Threat model: one trusted human operator; no external adversaries.
IN SCOPE: a mission host laundering unauthorized product bytes by
mislabeling their origin — including a false "sealed-dirty
admission" claim that relabels an unauthorized base tree — and
staleness (a patch reviewed against a tree that is not the tree it
lands on). Accidents, crashes, and stale anchors are in scope.
OUT OF SCOPE: the unbuilt isolation tier (a host that can mutate
delegate worktrees can still forge apparent authorship — D100
ruling 2 accepted that for the detector tier), runtime compromise,
and hostile third parties.

Appetite: 4h for this design; findings whose fixes exceed it pause
and go to the human.

Scope: the composition rule and the provenance-backed diagnosis
below; `internal/validate/authorization.go` issuance binding,
dispatch worktree creation for mission implementers, admission
recording at preflight. OUT: any change to the authorization
record's schema fields beyond what is named here; the recovery
ladder (o19); interactive non-mission work.

Return format: numbered findings, most severe first, each with
file, rule, and the concrete failure it causes; or AGREE with
observations that do not gate.

---

# Design: sealed-dirty admission composes with the wall (revision 3)

Revision 3 abandons revision 2's central claim. Two rounds proved
"the carrier changes, not the wall" false from two directions:
merge-base semantics make any synthetic commit either a descendant
of HEAD or unrelated to it, and the diagnosis stayed unreachable
behind issuance's own anchor repair. The wall's boundary
computation changes DELIBERATELY, in one narrow, named way.

## The problem (unchanged)

A human may seal a mission contract whose admitted baseline is
DIRTY. Issuance binds authorization base trees to named
expected-tree sequence points; a delegate based on committed HEAD
produces an unnamed boundary and sealed-dirty missions cannot use
delegates at all.

## The composition rule

1. ADMISSION IS ONE WRITE, AT STATE BIRTH (unchanged from r2). The
   child runner's stable-observation admission writes the admission
   record in the same write that publishes the first state.
2. THE ADMISSION RECORD IS REPLAYABLE, NOT SELF-CONSISTENT. It
   records sealedProjection, pointZero, sealSha256
   (fences.approvedContractSha256 by name), the contract's git
   entry as a NAMED INPUT — path, blob oid, mode — the dirty flag
   (one projection domain), and admittedAt. STATE VALIDATION
   REPLAYS THE SEAL: it re-reads the approved contract bytes,
   verifies them against sealSha256, extracts the literal
   wall.sealed-baseline value, and refuses any admission whose
   sealedProjection differs from that literal or whose pointZero
   does not recompute from the recorded inputs including the
   contract entry. Self-consistent-but-false records are
   unrepresentable in a valid state.
3. THE BOUNDARY BINDS THE RECORDED JOB BASE. Dispatch of a mission
   implementer records the delegate's base as part of the job
   identity (baseSha already sits inside jobRecordDigest, which
   authorizations bind). Dispatch REFUSES to create the worktree
   unless that base's tree IS a named expected-tree point of the
   authenticated mission state at dispatch time — this is where
   the unknown-base refusal lives, and where it is REACHABLE.
   Conformance's boundary computation for mission chains then
   binds the RECORDED JOB BASE instead of merge-base(target HEAD,
   delegate HEAD): the base the wall proves against is the base
   the job was born with, which the authorization's jobRecordDigest
   already seals. Staleness is unchanged in kind: full-tree
   equality at issuance still compares the reviewed tree against a
   named point, and consumption still verifies the same point.
   For a clean-baseline mission the recorded base IS HEAD's tree,
   so behavior there is byte-identical.
4. ISSUANCE READS AN AUTHENTICATED SNAPSHOT. The state authority
   returns the verified byte image (or the decision path hashes the
   exact bytes it parsed and compares against the verified hash —
   same-bytes binding, stated as the contract). A replacement
   between verification and use is a fixture, not a hope.

The wall invariant sharpened: every authorized patch applies to a
tree a named authority vouched for, and the BASE a delegate was
born on is part of its sealed job identity — relabeling a base
after dispatch breaks jobRecordDigest and refuses at
certification, which existing fixtures already pin.

## The diagnosis (moved to where it is reachable)

- At DISPATCH: a mission implementer whose requested base tree is
  no named point of the authenticated state refuses with the
  sealed-composition message naming the current expected tree —
  reachable, because dispatch performs no anchor repair.
- At ISSUANCE: the generic refusal stays byte-for-byte for unnamed
  bases (the defense in depth); no sharpened issuance message
  exists, because two rounds proved every sharpened case there
  unreachable or repairable.

## Fixture obligations (the arbiter)

- F1: dispatch of a mission implementer on a sealed-dirty mission
  creates the worktree on the recorded job base whose tree is
  point zero; on a clean mission, byte-identical behavior to today.
- F2: conformance's mission boundary binds the recorded job base;
  issuance binds it to the named point and consumption verifies it.
- F3: dispatch refuses an implementer whose base tree is no named
  point, with the sealed-composition message; issuance keeps the
  generic message byte-for-byte for unnamed bases (both asserted).
- F4: a lawful mid-turn merge keeps the generic issuance refusal
  (regression, message verbatim).
- F5: issuance's decision path refuses a state failing authority
  verification AND a state replaced between verification and use
  (same-bytes binding fixture).
- F6: state validation replays the seal — a chain-valid admission
  whose sealedProjection differs from the contract's literal
  wall.sealed-baseline, or whose contract bytes fail sealSha256,
  or whose pointZero does not recompute from the named inputs
  (contract entry included), refuses validation.
- F7: dirty is computed in one projection domain (nested-project
  bed, unchanged from r2).
- F8: a job whose recorded base was relabeled after dispatch fails
  jobRecordDigest verification at certification (cite the existing
  pin if it covers this exactly; extend if the base field is not
  yet in the covered set — it is, via jobIdentityKeys.baseSha).

## Dispositions of round 2

- O14-R2-01 FOLDED: the synthetic-commit carrier is abandoned; the
  boundary binds the recorded job base (rule 3); the "carrier not
  wall" claim is retracted in the revision header.
- O14-R2-02 FOLDED: state validation replays the seal from
  contract bytes (rule 2); F6 reshaped to the replay.
- O14-R2-03 FOLDED: same-bytes binding stated as the contract
  (rule 4); F5 gains the replacement fixture.
- O14-R2-04 FOLDED: the contract's git entry (path, blob, mode) is
  a named recorded input (rule 2); F6 includes it.
- O14-R2-05 FOLDED: the diagnosis moves to dispatch where no
  anchor repair precedes it (diagnosis section); F3 reassigned to
  dispatch; issuance keeps only the generic message.

---

# ROUND 3 STATE: budget exhausted, appetite exceeded — RAISED TO WIDO

The three-round budget closed with five material findings still
open (O14-R3-01..05, archived under
artifacts/agents/critiques/wall-o14/). The chain remains OPEN per
the budget rules; a successor chain must enumerate all five and
carries a fresh three-round budget, and a second exhaustion stops
for the human regardless.

The appetite verdict, honestly: three rounds each overturned the
design's core (tree-vs-commit carrier; unreachable diagnosis;
merge-base semantics; now the dirty point-zero having NO lawful
worktree carrier at all without either a mission-owned branch of
synthetic commits or a tree-based base identity through
conformance). This composition touches conformance semantics, the
dispatch contract, and the state schema — that is not a 4h design,
and continuing to iterate would spend the wall's budget on exactly
the rabbit-hole shape the appetite law exists to stop.

## Decisions that are Wido's (the raise)

1. BASE SELECTION AFTER A MID-TURN MERGE (O14-R3-04): when a
   delegate dispatches mid-turn, should it base on HEAD (fresher,
   but introduces a new refusal window on clean missions) or on
   the current expected tree (never refuses, but withholds
   already-merged reviewed work)? User-visible either way.
2. STATE-SCHEMA CUTOVER FOR THE ADMISSION RECORD (O14-R3-03):
   sealed-only optional field vs universal admission — either
   needs a schema-version decision.
3. WORTH: sealed-dirty delegate support now visibly costs a
   deliberate conformance-semantics change plus a real carrier
   mechanism (O14-R3-01). Is that worth building, or do
   sealed-dirty missions stay host-solo (with the wall's existing
   solo-build park) until the need is proven on a real VM target?

Recommendation: option 3's "stay host-solo until proven needed" is
the cheapest true statement — the VM evidence row that motivated
O14 can be attempted with a clean baseline first. If the need
proves real, re-scope O14 with an appetite that matches what these
three rounds uncovered (a day, not four hours).
