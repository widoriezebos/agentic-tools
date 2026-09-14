# Independent code read: ccf section 13, follow-up reads close terminal work

Reader: Opus 5 (1M), independent of the builder. Worktree read only, nothing edited or committed, no fixture bed run.

Worktree: `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccf-s13/metasystem`
HEAD: `338df580` ("coordinator-context slice 3 unit B1")
Spec: `artifacts/reports/ccf-9c-follow-up-reads-close.md` (section 13 of `plans/critique-closes-on-folded-proof-design.md`)
Builder report: `artifacts/reports/codex-ccf-section13-result.md`

Computed change surface (tracked plus untracked, nothing else):

- `internal/dispatch/review_reference.go` (+87)
- `internal/dispatch/read_admission.go` (+25/-10)
- `internal/dispatch/read_admission_test.go` (+48)
- `scripts/agents/dispatch-fixtures.sh` (+79)
- `internal/dispatch/review_reference_closure_test.go` (new, 479 lines)
- `cmd/metasystem/dispatch_followup_close_test.go` (new, 148 lines)

## What I ran

All runs used a scratch `GOCACHE`. The repository itself was never modified; the pre-change and post-change probes ran against copies under the scratchpad (`scratchpad/pre`, `scratchpad/post`) with the three production files reverted to `HEAD` in the `pre` copy.

- `go vet ./internal/dispatch ./cmd/metasystem`: clean.
- `go test ./internal/dispatch -count=1`: PASS (56.8 s), full package including every compatibility guard the decision names.
- `go test ./cmd/metasystem -run '^TestDispatchFollowUpReadClosesTerminalWork$'`: PASS post, FAIL pre.
- `gofmt -l`: clean. `bash -n scripts/agents/dispatch-fixtures.sh`: clean.
- Pre-change red check of the named tests: A-10, A-11, A-12 all red at the predicted line; B-12 red on its `validated closed live root` subtest only (the other two subtests are declared preservation checks); the two compatibility guards green pre and post, as the decision says they should be.
- Twelve adversarial probes written into the scratch copies only (later-work-round variants, tie, cancelled and failed terminal round, foreign implementation chain, in-flight and non-terminal critic round four, strict-path stamp target, clean subject-bearing fold without closure).

## Answers to the seven questions

**1. Can it accept a closure whose subject is not the terminal work round's?** Not in any construction I could build. Probes, all against the post tree:

- Later work round with different content: refused, `closure subject <a61d…> does not equal terminal work round work-r7 subject <9db2…>`.
- Later work round with no conformance artifacts: refused, `closure subject cannot bind terminal work round work-r7: terminal work subject has no review.json and diff.patch`.
- Two work members at the same terminal round: refused before the new code runs, `hazard-governed chain has more than one terminal work record at round 6` (`review_reference.go:140`).
- Any non-terminal implementation member: refused, `reviewed chain work-r4 is not terminal` (`review_reference.go:135`).
- Closure from a root that reviewed a different implementation chain: refused twice over, by the explicit `boundRoot != reviewedRoot` binding check (`review_reference.go:206`) and by `ImplementerRoot` being part of live-subject equality.
- A later work round whose tree and patch digest are byte-identical to the read one IS accepted. That is the decision's own no-op-terminal-member requirement (A-10) and the same rule the hazard gate already applies; the critic's read covers identical bytes.
- Cancelled or failed terminal work round: see F-7. Accepted when its conformance artifacts exist, but that is pre-existing hazard/close behavior, not something this diff introduces (proved on the pre tree).

**2. Same subject identity, still valid/clean/closed?** Yes. The comparison is `readsubject.ReadSubject.Equal` on kind `live` (`review_reference.go:221`), identical to `hazard.go:392` and `read_admission.go`, and the terminal subject comes from the same exact-member reader `liveReadSubject` the hazard gate uses, not the admission selector and not the workspace check. Validity is delegated to the shared `readsubject.ReadClosedClosure`, so `chainClosed`, clean register, folded round equal to closure round equal to the highest critic round, persisted-subject equality, folded subject digest, return binding and terminal membership all still hold. Probes confirm the reader still bites through reconciliation: an in-flight round four (`running`, `pending`) refuses with `closure member critic-read-r4 at round 4 is not terminal`; a `cancelled` or `failed` round four refuses with `closure last member … is cancelled, not completed`.

**3. Canonical root, and can it stamp the wrong one?** It stamps `state.chainRoot(evidenceJob)` (`review_reference.go:224`), and `ReadClosedClosure` independently requires that record to be round 1, parentless, and equal to `closure.criticRoot`, so the stamped job cannot be a follow-up round. `stampReviewReference` then re-derives the reviewed chain from `final.job` and refuses unless it equals the requested root and is distinct from the critic chain. The fixture Go assertions check the stamp is the root for both the root argument and the child argument.

**4. Redundant-read guard.** Untouched as a decision. `cleanReadCanClose` became `cleanReadCloseState` and returns exactly the same `closeable` boolean on every branch (`read_admission.go:521-550`); only the recovery sentence changed. Decision, exit 11, event id and wire fields are unchanged, and the full `internal/dispatch` suite including `TestReadAdmissionRefusesCleanSubject` is green.

**5. Do chains that should stay open stay open, are refusals accurate?** Yes to the first, with the single caveat in F-7. Refusal text is accurate in substance and names the supplied job, the resolved root, the closure round and the failed binding as the decision requires; see F-4 for one misleading prefix.

**6. Red before, tautology?** A-10, A-11, A-12 and B-12 are genuinely red on the pre-change tree for exactly the reasons the decision predicted, and A-11 keeps `reviews` equal to the terminal member so the old comparison cannot mask the new guards. A-13 is the weak one: see F-1.

**7. Scope.** Clean. Four tracked files plus two new test files, nothing under `plans/`, no `testing.json`, no coverage floor, no role text, no receipt row, no change to `scripts/agents/dispatch.sh` (the decision said production shell sequencing needs none). `artifacts/` is gitignored, so the builder's own report is not part of the change.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | medium | yes | The test that carries obligation A-13's name proves nothing about the repair in an ordinary run. Without `METASYSTEM_FOLLOWUP_CLOSE_FIXTURE_REPO` it only greps `dispatch-fixtures.sh` for eight literal strings that this same diff added, plus a `close_chain` ordering check over an unchanged `dispatch.sh` that is green before and after. Its pre-change redness is therefore the absence of its own fixture text, not a behavior difference. Every behavioral assertion lives behind the env branch and runs only inside the fixture bed, which was not run. A green `go test ./cmd/metasystem` must not be read as A-13 discharged. | `cmd/metasystem/dispatch_followup_close_test.go:22-49`; pre-tree run fails with `follow-up close fixture lacks "follow-up-read-closes-terminal-work"`; `artifacts/reports/codex-ccf-section13-result.md` ("No fixture bed was run") |
| F-2 | medium | yes | Root normalization was hoisted above the branch, so it also changes the lawful-absence strict path, which the decision told the build to preserve. Supplying a completed critic child on a chain with no closure now stamps the critic ROOT; pre-change it stamped the child. Probed: pre stamp `critic-read-r3`, post stamp `critic-read`, same input. That silently removes hazard's "is not a fresh-context chain" refusal for the child-argument form on that path. The outcome is not a new authority (naming the root reached the same result pre-change) and the new stamp is the more correct one, but it is an undeclared change to the close gate's input handling with zero test coverage. | `internal/dispatch/review_reference.go:123-129` (`criticReferenceJob = state.chainRoot(evidenceJob)` set before the `designCritic`/`liveCritic` split); decision build change 1 and 4 in `artifacts/reports/ccf-9c-follow-up-reads-close.md`; scratch probes `TestProbeStrictPathStampTarget` (post) vs `TestProbePreStrictPathStampTarget` (pre) |
| F-3 | low | no | A critic root with a clean, subject-bearing fold and no closure now refuses at reconciliation where it previously stamped. Net behavior is unchanged because close-check already refused that same shape with the same underlying reason, so no chain that could close now cannot. Conforms to decision build change 4 ("use the shared reader's absence result"). | Probe pre: reconcile returns nil and stamps, then `CloseCheck` refuses `REFUSED-R22-M1-RULING-O-CRITIQUE-STALE … completed clean subject-bearing fold at round 3 has no closure`. Probe post: reconcile refuses with the same sentence. `internal/readsubject/closure.go:309` |
| F-4 | low | no | Every refusal from the new path is labelled `with invalid terminal-subject binding`, including failures that are not subject bindings: unreadable critic chain membership, and the closure-reader's absence errors. The detail after the colon is accurate; the prefix is not. | `internal/dispatch/review_reference.go:181`, `:189`, `:242`; observed text `… closure round 3 with invalid terminal-subject binding: completed clean subject-bearing fold at round 3 has no closure` |
| F-5 | low | no | The new admission recovery line decides `closedLive` from the closure's subject kind but prints the implementer root from the clean-read entry's subject. If those two ever disagree the command names an empty or wrong job. Not reachable in practice: `closeable` forces read round equal to latest round equal to closure round, and `Equal` forces matching kinds. Using `closure.Subject.ImplementerRoot` would remove the coupling. | `internal/dispatch/read_admission.go:216-217` vs `:546-549` |
| F-6 | low | no | The fixture Go assertion does an unchecked type assertion on the mirror path, so a mirror record without a string `path` panics instead of failing with a message. | `cmd/metasystem/dispatch_followup_close_test.go:92` |
| F-7 | low | no | Reconciliation stamps, and close-check then closes, an implementation chain whose terminal work round is `cancelled` or `failed`, as long as that round's `review.json` and `diff.patch` exist: `liveReadSubject` and `finalHazardWorkState` both ignore member status. I flagged this because the brief asked about it, but it is pre-existing in hazard and close and is not widened here, so it is out of scope for section 13. | Probe post: reconcile nil, `CloseCheck` nil with a cancelled terminal round. Probe pre with the root stamped by hand: `CloseCheck` also nil. `internal/dispatch/hazard.go:284-318`, `internal/dispatch/read_subject_compute.go:120-133`, `internal/dispatch/close.go:23-27` |
| F-8 | low | no | The builder's report lists `memory/receipts.log` among the files changed; no such change exists in the delivered tree. The tree is correct (no stray receipt row); the report is not. | `git status` clean for `memory/`; `artifacts/reports/codex-ccf-section13-result.md` "Files changed" |

## Verdict

**Fit to land once F-1 and F-2 are dispositioned.** The widening itself is sound: I could not construct a subject that this repair accepts and should not. Every hostile case is refused (different-content later round, unreviewed later round, tie at the terminal round, non-terminal implementation member, in-flight or non-completed later critic round, foreign implementation chain, and all twelve named invalid-closure shapes), the equality is the system's own live-subject identity read through the same exact-member reader the hazard gate uses, the canonical critic root is what gets stamped, `reviews` is left untouched, the redundant-read decision is unchanged, and `CloseCheck` remains the authority for admission, freshness, session, effort, proving round and durability.

What must change before certification:

1. F-1: run the `follow-up-read-closes-terminal-work` leg in both instances (cluster c of the dispatcher bed) and retain the structured result. Until that runs, A-13 is undischarged, and the seat should not read a green `go test ./cmd/metasystem` as evidence for it. Optionally strengthen the non-fixture mode so it cannot pass by text presence alone.
2. F-2: either add a case to `internal/dispatch/review_reference_closure_test.go` pinning the strict-path stamp target for a child argument, or record on the decision page that normalizing live-critic arguments to their root applies to both paths.

F-3 through F-8 are recorded, not actionable, and must not block.

Material findings: 2.
