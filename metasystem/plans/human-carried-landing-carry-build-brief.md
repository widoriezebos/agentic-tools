Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal human-carried-landing-carry)
Date: 2026-09-11

# Goal

Goal human-carried-landing-carry (approved by Wido 2026-09-06, tier 3,
his ruling of 2026-09-04 "All approved"). Its record,
metasystem/plans/goals/human-carried-landing-carry.md, is the contract.
In one sentence: a verified human can carry one landing past one named
refusal or one named testing group, with the review deferred into an
obligation on the goal, every use counted and spoken, a cap on open
carries and no stacking on unpaid debt. Today 57 rows of the refusal
register name `land.sh --carried` as the human verb and 48 of them are
marked Pending because the verb does not exist; last night this seat
landed its own chain only by a hand-made publication under a delegated
word. The verb you build is the lawful form of that.

# The design is the specification

metasystem/plans/human-carried-landing-carry-design.md, revision 6
(sha256 45b204f9b5d9a037edb06dc3a054ca053c7dedae4f0cf4edc251acc16faac7a5, landed
ab513438b), is in your worktree. Build exactly what it says;
where it names a verb, a flag, a field, a code, a message, a trailer, a
row or a fixture, use that name. It was read three times on the Sol
lane (metasystem/records/misc/human-carried-landing-carry-critique-r1.md,
-r2.md, -r3.md) and folded three times; the decisions are settled. The
third read's four findings (HCL-C-33, -25, -03, -34) were folded into
revision 6 without a fourth read: the closing read of your chain
verifies them as named checks, so build them exactly as the page's
revision 6 record states. One inconsistency in the page predates that
fold: line 1853 says fourteen ask codes enter the register where
section 05 and the revision 5 record say fifteen; fifteen is right. If the tree contradicts the page at a cited line,
build what the tree requires, say so in your return under `deviations`,
and do not stop.

Read these sections in this order: the one-paragraph shape at the top;
HCL-WORD-04 (the word, the workspace tree, supersede, transfer, the
format raise); HCL-LANDING-05 (the carried classification, the match
rule, the two-step judge, the trailers, the lease, main-only,
carry-forward and recovery); HCL-TRANSACTION-06 (the six writes,
`goal carrying`, `goal carried`, consumption by ancestry, replay,
`landing carry-status`); HCL-IDENTITY-02 (the authority outcome, the
generation, the format fence); HCL-OBLIGATION-07; HCL-COUNTER-08 (the
counter, the counselor lines, the cap, the debt); HCL-CRITIC-09 (the four
seams); the channel section (HCL-C-12); the register flip; contract
ownership (HCL-C-31); the fixtures section; "Files this design touches";
"One chain, and its budget".

# Decisions (the orchestrator's; decided, not open)

1. One chain, one round if it fits the cap, corrections as follow-ups on
   this chain. Build in this order, each step green before the next:
   internal/governance and internal/goal (the outcome, the generation,
   the format fence, `Carry`, `Carrying`, `Carried`, the journal entry,
   recovery, `done`'s refusal, the accepted-risk discharge) with their
   tests; internal/humanauthority; internal/channel; internal/landing
   (carried.go, observe, carry-status) with tests; internal/config,
   internal/counselor, internal/steward; internal/dispatch (the four
   seams); the goal command layer and the landing and channel verbs in
   cmd/metasystem; scripts/agents/commit.sh and land.sh (carried mode,
   the lease re-exec, the judge, the trailers, carry-forward, recovery);
   dispatch.sh (`--reviews commit:`); testing.json (the ownership of
   HCL-C-31, before you run the contract check); the register flip and
   `TestHCL03NoPendingAfterSlice2` LAST, in the same round as the
   wrapper, never before it; then the bed legs and scenarios; the docs.
2. Keep every existing contract the page lists under "what must not
   change": the identity gate is `goal set-budget`'s; a record failure is
   never carried; the temporary word is not a proof; nothing from the
   shared testing contract or the workspace receipt gets weaker.
3. Every exit-3 ask prints `Observation.Refusal` or the page's exact ask
   text; every new refusal code is a Question row in the register with
   no Override; the nine (or more) ask codes are named in the page.
4. The fixtures are the page's, by name and oracle; a fixture the page
   names that you cannot make pass is a `deviations` line with the
   reason, never a silently weakened assertion.
5. `metasystem test check --root .` must pass on your candidate before
   you run any bed: the contract gains the surface and groups of
   HCL-C-31 and the selector declares any new section.

# Facts

- The lease: commit.sh re-executes itself under `lease run-held`
  (metasystem/scripts/agents/commit.sh:31-54) and the human branch is
  taken only without a claim epoch; the page moves the re-exec to land.sh
  for carried mode and has commit.sh enter through `__lease-held`.
- The landing engine at HEAD keys receipts by the selected groups'
  execution identities with the workspace projection as the floor
  (`internal/landing/registers.go`, `receipt.go`, landed 612b1e578);
  the word's `workspace=` is `landing.ProjectWorkspaceTree`.
- The sandbox denies process enumeration and object-store writes in
  linked worktrees for the public testing owner; report what it blocks
  and the coordinator reruns outside. Do not work around it.
- Fixture clones armed with `steward arm --repo` are ended with
  `steward disarm --repo`, never `stop --repo` (the land bed's
  `stop_receipt_runner` does this).
- Build the bed engine with `bash scripts/agents/go-build.sh` (default
  stamp) before running any bed.

# Proof before you return

- `go build ./... && go vet ./...`; `go test ./internal/... ./cmd/metasystem/...`
  (name each sandbox-bound test if red).
- `metasystem test check --root .` green.
- `bash scripts/agents/go-gate.sh --fast` green (it re-proves the
  refusal register).
- `bash scripts/agents/goal-cli-fixtures.sh`, `bash scripts/agents/land-fixtures.sh`,
  `bash scripts/agents/dispatch-fixtures.sh` green with the page's new
  scenarios and legs (name each leg's outcome).
- `metasystem test plan --root . --mode auto --purpose delivery --goal human-carried-landing-carry`
  on your staged candidate, and the selected groups in your return (it
  may stop at the sandbox's object-store denial; say where).

# Return

`diffBoundary` and `files` are repository-root paths (`metasystem/...`).
List under `evidence` every command above with its observed result, and
under `deviations` every departure from the page with the line.
Wall-clock expectation: the cap (120 minutes); at the cap return what is
green, in the build order above, and say what is red and why.
