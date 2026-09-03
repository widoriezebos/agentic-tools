# The tiering machinery is split across two seats (m3 to m2, 2026-09-03 22:05 local)

Wido's order to m3, verbatim: "1. I want this done in 16 hours MAX 2. I
want this done in parallel between the two of you if that makes this
possible 3. I want the second part to include the risk scoring 'Risk is
four separate questions, never the shape of the change' because without
it is is a stupid deterministic rue at best. 4. I need you to coordinate
the delivery of this and guard progress and intervene if things go off
track." Deadline: 2026-09-04 13:37 local. Ruling row R-73-m3.

## The split

- m2 keeps goal severity-tiered-rigor: finish part one (chain
  str-build1) -> one Fable code review -> at most one correction -> land
  with --chain; then part three (the tier-1 landing class). Do not start
  part three before part one is on main.
- m3 holds goal severity-tiered-rigor-p2: the risk basis (design
  revision 4, plans/severity-tiered-rigor-p2-design.md: the tier derived
  from four recorded answers; one Fable design review running as
  str-p2-design-cc1), then revision 3's part two (material stop and
  close), then the docs. Part two starts only on part one's LANDED tree;
  both touch internal/goal/file.go, verbs.go, dispatch.sh,
  dispatch_verbs.go.
- Ceremony per part: build, one review, one correction, land. A second
  correction goes to Wido with evidence. Part three is deferred first if
  the slack runs out.

## Observation for m2 on the part-one build

Round 4 (str-build1-r4) was not hung: Sol re-ran `go test
./internal/goal/...` from ~20:20, the package is slow (git-backed, about
25 minutes), and the 120-minute cap fires at 22:10. If the cap ends it
before the return, take the worktree (artifacts/agents/worktrees/str-build1,
base 12ce45c8, predating the gap-3 landing 87858405) seat-side, run
go-gate.sh --fast and `go test ./... -timeout 25m`, and go to the Fable
code review, not a round 5.

## Paper factors for part one's review brief

(a) ch. 6/11: the four questions set depth and spend, never size or
kind, so the tier is derived, a bare --tier is an override with a
record (revision 4); (b) ch. 12: a new enforced rule is born marking,
with owner, review date, known-bad case and appeal route; (c) ch. 6:
misclassification is a defect with a record in both directions;
(d) ch. 11: repeated budget exceptions are a defect signal.

## Asked of m2

Reply on the session bridge (m3's session: "M3 Metasystem") or by a
record: ETA for part one on main; the exact names part two depends on
(the Tier field, the token quadruple, goalTier on roots, the tier-box
keys); anything in the part-one tree that already touches the close or
transition table. m3 checks m2's landings every half hour.
