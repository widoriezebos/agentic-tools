# Independent code critique, round 3 (final confirmation): complete tasks through intent

Critic: Fable 5.1 (claude-fable-5-1), same context as rounds 1 and 2. Subject: review-subject.json.
reviewedTree f0a58991da7a91986ec33c3c254c2d2658d8abb2; candidate bcf1679ca371cfdded4f80a400827f71c49898c4;
base 9e8f98108c20cf4c6479883b0317d89e3659f178. Verified: full.patch SHA256
1e4e6e43023b6a837ac07a1625bc72759a0ca95211c2997b324c7597a13e2729 matches the manifest; `git rev-parse HEAD^{tree}`
equals reviewedTree; the only status lines are ignored metasystem/artifacts/ and metasystem/bin/.
corrections-from-r3.patch SHA256 fed0692aa552f263bdaa709fc707f6c544af33aa89e7eec3c068abed070c6f9b (27 files,
1192 lines) read in full. incoming-main.patch SHA256 bdd4af5840be06f142bbb53dd72f52f2f260f12c0db435827e84781de71c87c7
(sitting/decisions UI, goal verbs, refusal anchor, testing surfaces) was listed, not reviewed: it is main's work,
not this implementation. Scope check: the R4 full patch's file list equals R3's plus internal/launch/unit_test.go.
No agent, test, build, product edit, commit or private configuration read.

Round 3 provenance: started 2026-09-26T16:47:50Z, ended 2026-09-26T16:52:06Z; 9 new tool calls (Bash 4, Read 5).

## Materiality test applied (verbatim)

> Would the change ship a defect, violate its brief, or damage what certifies it?

## Verdict

VERDICT: fix first (1 material). Every accepted behavioral correction (ICR-9, ICR-10, the ICR-11 sweep's named
strings, the carried-adoption error propagation, the protocol-failure risk remedy) is implemented and carried by
real-owner fixtures. One residual of the accepted ICR-11 contract sweep remains: four ordinary-help strings still
name internal owners. It is descriptor-only; no behavior, authority, schema or fixture is affected.

## Findings

| Id | Severity | Material | Claim | WORKS without fix | SAFE without fix | Artifact to change |
| --- | --- | --- | --- | --- | --- | --- |
| ICR-14 | low | yes (residual of accepted ICR-11) | Four ordinary help strings still narrate internal owners: goals details "fetches and validates the canonical ledger through the goal owner" (intent.go:107); resume details "through the mission runner" (intent.go:197); split details "each ratified by the owner's own rules" (intent_planning.go:310); repair details "repair review replays the whole close of the work's finished examination" (intent_planning.go:429). Root's ICR-11 acceptance treats this class as a direct violation of the human's hidden-internals contract; the sweep left these. | Yes | Yes | the four descriptor `details` strings; existing help assertions cover format only |

Non-material notes retained from round 2 and unchanged in disposition: ICR-12 (explicit ambiguity between an
unread build and a collected-unpublished build) and ICR-13 (a completed round with corrupt return has no retry;
the completed-round invariant is preserved). No new behavioral defect found.

## Confirmation of the round 2 corrections

- ICR-9 (manual retry continuation): commitReview's in-progress and unpublished branches now print
  canonicalReviewArgv (intent_unit_review.go:450-457), which carries no `--retry`, `--model` or `--dispositions`;
  the build path's stripping at intent_selection.go:578-579 is unchanged. TestManualReviewFailedExaminationRetries
  now marks round 1 as capped with group-death proof, advances the register, follows the printed retry, asserts
  the retried request's continuation is exactly `review G --work alpha`, completes round 2 through finishRound
  (subject and return under the root, register advance), follows the continuation to the decision, closes through
  the real close owner, collects and publishes, and proves a repeat is unchanged (no critic, round, read commit or
  push). The live-round refusal remains. Confirmed.
- ICR-10 (reserved goal names): intent_review_binding.go:97 builds `again` with reviewGoalWords;
  TestIntentGoalReviewEmptyJoinRealClose now uses goal "design" and asserts the decisions continuation reads
  `review goal design --work main --dispositions`. A grep of the checkout finds no remaining `publicArgv("review", goal…)`
  spelling. Confirmed.
- ICR-11 (plain help): the named strings are replaced (review dispatch identity, accept-risk/decide registers,
  edit set-obligation owner, recover/repair owners, wait launch/dispatch owners, stop launch/dispatch, answer
  mission owner, ask poll lock, start `up` alias and mission runner, check/doctor machinery, build summary);
  `status unit U` is removed from status usage while its parser remains; TestIntentPublicDiscovery asserts
  `metasystem status unit` is absent from help work/status/all and TestIntentPublicCoverage's status grammar
  uses `status work`. Residual: ICR-14 above.
- Carried adoption shadowed error: intent_exception.go:181-189 now stops on either RetainCarriedSubject or
  BindCarriedSubject failure before staging. TestIntentCarriedChannelWordAdoptionStopsWhenBindingFails obstructs
  the subject store with a regular file at the adoption's own composition (filesystem, permission-independent),
  asserts no staging, no binding file, no carry act and exactly one composition. Confirmed; ratchet entry added.
- Protocol-failure risk remedy (main 21): riskRemedy (intent_review_binding.go:338-365) reads the chain's open
  finding ids and the typed decision finding per id, keeps only Severe/Unproven classes, and refuses with
  `accept-risk G --finding F --review ROOT --repo <checkout holding the review> --reason TEXT` followed by the
  canonical continuation; it grants nothing. It is called from closeWorkReview's ordinary-failure branch
  (:255-257) and both commitReview close-failure branches (:398-399, :418). closeChain sets `data["owner"]` only
  for the fence case (intent_delivery.go:1279), so the fence branch does not swallow this refusal on the build
  path. finding_register.go:446-465 supplies title/claim/evidence for exactly the two canonical synthetic ids
  (protocol failure, unbound return) with critic identity and evidence digest, preserving unproven class and goal
  binding (test covers both kinds and the other-goal refusal). TestManualReviewProtocolFailureNeedsAcceptedRisk
  drives the real register (CritiqueRegisterAdvance of the failed round), the retry, round 2 completion, the
  refused decided review naming the synthetic id and the exact public command with no internal command, the
  agent's authority refusal on that command with the register unchanged, and the declared human fixture's
  acceptance that discharges exactly that finding. Confirmed as composed evidence; see boundaries.
- Staticcheck: writeIntentSummaryLine and designReviewPlan.dispatch removed; two dead test assignments removed.
- Stale smoke assertion: TestIntentReviewCommitClosesThenPublishes keeps the initial `--model gpt-critic`
  forwarding assertion on reads[0] and now expects the decision and lost-publication continuations without the
  model and follows the printed continuation. TestUnitRunRefusesMainBranch expects unit_run.go:258, matching the
  register row already present since round 1. carry-remote-required anchor follows main's verbs.go move.
- Registrations: command-interface-smoke lists the two new tests; the ratchet adds their serial reasons;
  interface-standard adds internal/ui/project for main's inventory.

## Evidence boundaries (not findings)

- The protocol-failure fixture stops after the human acceptance clears the register; the continuation that
  then closes, collects and publishes is the unchanged close owner path proven by the other fixtures, not
  re-driven after the acceptance. Root states this boundary; it is consistent with the code.
- The retry round-cap remains composed evidence (branch retry to dispatch.sh follow_up to the register and
  exhaustion owners), as accepted in round 2.
- The fixture's `--repo <goal worktree>` for accept-risk relies on the bed adjusting its authority facts to the
  selected checkout; production classification of that checkout is the existing owner's.

## Unexamined in this round (recorded; no AGREE claimed over it)

incoming-main.patch contents (main's sitting/decisions UI and goal block/unblock endpoint changes; only its file
list and the corresponding testing.json/refusal-register deltas inside the corrections patch were read); the
builder returns; the external evidence logs beyond their exit codes (public-risk-repository: complete-human-risk,
reader and audits exits 0); no test executed. Final race/coverage and the named native/section runs are root's
pending evidence.
