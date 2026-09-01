# Two-bars design addendum: the floor-distinct refusal code

Design-lane addendum to `records/two-bars/two-bars-design-r4-joint.md` §0
item 3, §2.3, and the bar (c) taxonomy, in that design's voice. The
implementer lawfully gap-stopped on one taxonomy question: the narrow
fail-closed promotion slice (re-sliced at Wido's intake order, recorded on
`plans/goals/two-bars-for-changes.md`) refuses a declared direct-fix class
that stages paths on the never-direct-fix floor, and explicitly not ordinary
class-rule misses off the floor — but the implemented evaluator folds both
causes into `register-carriage-path-refused` and `not-exact-revert`.

## Ruling: ADD one shared code, `direct-fix-floor-refused`, alongside the per-class codes

One new code, not a per-class split, for three reasons. First, the floor is
one class-independent predicate — `neverDirectFix` in
`internal/landing/observe.go` — so one cause gets one name; the class is
already recorded in the same commit's `Landing-Provenance` trailer
(`class=register-carriage` / `class=exact-revert`), and per-class floor codes
would duplicate that bit. Second, the promotion record must name the floor
alone: with one shared code it names exactly one string,
`would-refuse code=direct-fix-floor-refused`, and provably catches no
non-floor miss because the code fires only when a specific path satisfies
`neverDirectFix`. Third, splitting would retire two code names mid-window;
adding narrows the existing codes' extension while keeping every stamped
trailer's name comparable across the observation window.

## Exact trigger against the implemented evaluator

The ruling moves no pass/refuse boundary — every triggering landing already
refuses today — it renames the verdict of an already-refusing landing.

- **Register-carriage** (the `registerCarriage` path scan, reached from both
  the standalone bar (b) arm and bundled chain carriage in `observeChain`):
  the class evaluation scans the full staged path set for allowlist misses
  before applying the append-only disciplines. If any disallowed path
  satisfies `neverDirectFix`, the code is `direct-fix-floor-refused`; if all
  misses are off the floor, `register-carriage-path-refused` as today.
  Allowlisted paths never trigger: the allowlist is the recorded exact
  exception to the floor (§2.3), so an allowlisted path is off the floor for
  verdict purposes. Floor precedence is over the whole set, not first-miss,
  so the code never depends on path order.
- **Exact-revert** (the `exactRevert` failure mapped in `observeDirectFix`):
  when the class refuses, the floor scan covers the union of the declared
  inverse's target path set (when computable) and the candidate's changed
  path set against the evaluation base tree. Any floor path in that union
  yields `direct-fix-floor-refused`; a floor-clean union keeps
  `not-exact-revert`. The union matters: a candidate that declares a clean
  revert but stages an extra floor path is exactly the staging the order
  refuses, and today it hides inside the path-set-mismatch cause.

## Relation to the existing codes

- `register-carriage-path-refused` — kept, narrowed: a disallowed carriage
  path off the floor (adopted-project payload staged under carriage).
- `not-exact-revert` — kept, narrowed: any revert miss whose target and
  candidate paths are all off the floor.
- `register-carriage-not-append-only` — unchanged, outside the floor code:
  it names misuse of an explicitly allowlisted register, a distinct and
  already-unconflated cause the promotion record may name separately.
- `chain-has-uncarried-paths` — unchanged: no direct-fix class was declared
  for those paths, and the order's clause covers declared classes only.
- `register-carriage-policy-unreadable`, `direct-fix-policy-unreadable`,
  `malformed-revert-commit` — unchanged: undecidable or malformed input is
  not a floor claim; the fail-closed slice owns undecidability on its own
  terms.
- Grammar: §0 gains the one code in the bar (c) list; no trailer shape
  changes.
