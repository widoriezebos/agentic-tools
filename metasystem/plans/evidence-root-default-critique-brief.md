# Critique brief: evidence-root-default-design, round 1

You are the design critic (Codex, Astra). Critique the design at
`metasystem/plans/evidence-root-default-design.md` (commit 3e1262633 on branch
ui-development, repository root `/Users/wido/LocalStorage/GitHub/agentic-tools-ui`).
This is a read-only task: do not edit files, commit, or run builds or tests. Do not
read `metasystem/metasystem.conf.local` in any checkout; those files hold secrets.

## What the design is for

`evidence.root` is the host-local directory where run evidence is mirrored so it
survives `git clean`. Today most checkouts carry the template's placeholder, nothing in
setup sets or checks the root, and eight readers each handle "unset" their own way,
mostly failing at the moment they need it, one of them silently. Wido approved this
shape: one owner (resolver); a compiled-in default of
`$HOME/metasystem-evidence/<checkout basename>`; setup reports the root and never
requires it; a structural test so no new reader bypasses the owner. KI-6, the silent
mirror failure, is out of scope.

## The stop rule you judge by (quoted verbatim)

R-121: "We should always build the smallest thing that works FIRST before we start
iterating. Even when asked for something big, that needs to be sliced so that we
(again) have a design for the smallest thing that works FIRST and have something of
value already, before we (decide to) iterate."

R-124: "remember: the smallest thing that works — We clearly need to refine the stop
criterion for the design critiques. We have a stop criterion, but it clearly does not
include the smallest thing that works criterion. We must add that." Its mechanism: a
finding is material only when step 1 does not WORK or is not SAFE without it; rigor,
corners and elegance beyond the slice's first use go to the deferred list, and a fold
never adds mechanism for them.

## Threat model

What must not happen:
- Evidence is written inside a checkout, or into another checkout's root, where it
  would mix.
- An explicit setting a human made is silently replaced.
- A seat that has a root today changes to a different one.
- A reader keeps a private fallback that disagrees with the owner.
- The launch writes a new machine's evidence somewhere unexpected.
- The build breaks line-anchored checks, the dependency ratchet, the parallel ratchet
  or the coverage floors.

Verify claims at whole-function depth in the code. Do not trust the design's line
numbers.

## Scope

The design and the code it cites under `metasystem/`. Branches `embed` and `u9b` of
another seat are not visible; judge the Sequencing section only for internal
consistency.

## Budget

At most 60 tool calls. This is round 1 of at most 2; round 2 is the failsafe, and there
is no round 3.

## Return shape

For each finding:
- `ERD-NN` (stable id)
- severity: material / non-material
- the claim, with the evidence (path:function and what you read)
- the smallest fix
- for material findings, why step 1 does not work, or is not safe, without it

Then a "Deferred" list for everything non-material. End with exactly one line:
`VERDICT: <n> material` and either `EXECUTABLE` or `NOT YET EXECUTABLE`.
