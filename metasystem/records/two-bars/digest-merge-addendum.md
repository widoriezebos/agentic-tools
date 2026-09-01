# Two-bars design addendum: merge and rewrite discipline for `records/narrator-digest.log`

Design-lane addendum to `plans/two-bars-for-changes-design.md` §5, in that
design's voice, via R-25b's deviation path. The digest-union-merge goal
wants `merge=union` on the narrator digest so fleet landings stop
hand-resolving append/append conflicts; union is only safe if rewrites are
either refused somewhere or ruled harmless. Two implementer gap-stops
isolated the question this addendum answers: what merge and rewrite
discipline governs the digest?

## Ruling: union is IN, and digest carriage becomes APPEND-ONLY

The digest rides carriage APPEND-ONLY, by exactly the mechanism the
receipts addendum minted for `memory/receipts.log`: the engine refuses
carriage when the staged diff for `records/narrator-digest.log` deletes or
modifies any existing line; only trailing additions qualify, re-checked at
the push boundary against each outgoing commit's own parent-to-commit diff
(§8). With that refusal standing, `merge=union` is safe and the
digest-union-merge implementation proceeds.

1. **The engine never rewrites, so refusal costs no honest writer.**
   Both digest writers only append: `Append` and `AppendPayload` extend the
   existing bytes and deduplicate retries; neither deletes or edits a line
   (internal/narratordigest/digest.go:110-194). The only actor an
   append-only refusal can touch is a hand rewrite riding the carriage
   flag — the exact lane the rulings and receipts rules already close.
2. **Option (b) fails at fleet scope.** The emitted-prefix protection is a
   machine-local cursor (`artifacts/agents/steward/narrator-digest-cursor.json`,
   gitignored); on every machine but the emitter, a rewritten digest
   history is undetectable, and even locally the cursor detects after the
   fact rather than refusing. The seat-governance record names pending
   narration as rewriteable repository content — that is the hazard, not
   the guard. Ruling union safe on the strength of the cursor would
   document a residual the mechanism does not actually cover.
3. **Option (c)'s loud conflicts protect the wrong case.** A one-sided
   rewrite merges CLEANLY under the default driver — three-way merge takes
   the modification without a conflict. Conflicts fire on both-sides
   appends, which is precisely the honest case. Refusing union taxes every
   honest fleet landing to keep a protection that misses the dishonest
   one; the append-only refusal catches the rewrite at its origin commit
   instead, where its own parent-to-commit diff shows the modification.
4. **Union and append-only compose by construction.** The union driver
   only inserts lines from the other side; it deletes nothing, so a
   union-produced merge commit shows pure additions against each parent
   and passes the rule without exception paths.

## What this ruling does not touch

Digest content stays content-unverified — §5's deliberate bounded residue
is unchanged. Append-only is a shape rule on the staged diff, not content
verification, and it establishes no actor boundary: the narrator open item
(plans/seat-governance-record.md:113-148) remains conduct-bounded, its
remedies remain Wido's at the 2026-11-30 review, and nothing here
forecloses either candidate. A legitimate future rewrite of digest history
(rotation, redaction) is not carriage; it takes a bar, like any rulings or
receipts rewrite.

## Residual, documented openly

Git does not guarantee line order inside a union-merged hunk. A merge that
interleaves remote lines into a locally-emitted-but-unmerged region
changes the local prefix bytes and trips `Pending`'s hard error
("narrator digest changed before the last check-in cursor",
digest.go:241-243) on that machine. This residual predates union — any
merge or hand resolution could reorder those bytes, and hand resolution is
strictly worse — so union inherits it rather than creating it. The failure
is loud and machine-local; its remedy, if the fleet ever needs one, is
steward-side cursor recovery, out of this ruling's scope.

## The mechanical consequence for digest-union-merge, exactly

- Add the gitattributes entry `records/narrator-digest.log merge=union`.
- Extend the carriage check so `records/narrator-digest.log` gets the same
  APPEND-ONLY staged-diff refusal as `memory/receipts.log` — trailing
  additions only, deletion or modification of any existing line refuses
  carriage — re-checked at the push boundary per §8.
- The §5 seeded-list entry for the narrator digest log now reads
  APPEND-ONLY (as defined here), still content-unverified.
