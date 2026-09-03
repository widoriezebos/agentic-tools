Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Goal

The one correction round of slice 2b (chain str-p2-build-2c). Fable's
code review of your round-1 tree (reviewed tree a1ce1f30; dispositions
in metasystem/records/misc/severity-tiered-rigor-p2-2b-critique-cc1.md,
the full findings in the critic's return under job str-p2-build-2c-cc1)
accepted eight material findings and two notes. Fix all ten below in
your working tree, gate, return. Contracts unchanged: the three 2b
briefs and metasystem/plans/severity-tiered-rigor-p2-build-2c-brief.md.
A second correction goes to Wido; make this one complete.

# Fixes, in order

1. F-1 (critical), the subject set is always empty. `critiqueSubjectForRound`
   (metasystem/internal/dispatch/finding_register.go) reads diff.patch
   keeping only paths `ParseArtifactRef` accepts, which must begin
   `metasystem/`; conformance writes diff.patch PROJECT-relative
   (this chain's own round-1 diff.patch: `diff --git
   a/cmd/metasystem/delegate.go b/cmd/metasystem/delegate.go`), so
   every code-critic and warden fold refuses with "no changed paths".
   Fix: prefix each diff.patch path with the install prefix before
   parsing (the prefix `projectInstallPrefix` in
   metasystem/internal/validate/conformance.go derives; expose or
   duplicate that one rule in dispatch so both sides use the same
   derivation, empty at the toplevel, `metasystem` nested). Test:
   feed a diff.patch with project-relative headers, as conformance
   writes it, and observe the subject set non-empty; feed the
   `rename from`/`rename to` form too.
2. F-2, roots from before this slice. `criticRoundsConsumed` errors
   when `reviewRoundLimit` or `criticRoundsConsumed` is missing, and
   it sits on the fold, the exhaustion advance and the close. Fix: a
   missing `criticRoundsConsumed` reads as the count of completed and
   failed rounds under the root (what it would have been), a missing
   `reviewRoundLimit` reads as `goalReviewRoundLimit`'s value; both are
   written on the next advance; `CritiqueBudgetRebind` writes both on
   a root that lacks them. Test: a root record with a register and
   neither counter folds, advances and closes.
3. F-7, the third reader. `CritiqueRegisterClose` decodes the register
   without a presence guard, so a register-less chain can never be
   closed by `dispatch.sh close` and never reaches the compatible merge
   path. Fix: absent register is the pre-2b path in `CritiqueRegisterClose`
   exactly as in `CloseCheck` and `CritiqueRegisterAdvance` (one helper
   the three call). Extend TestMergeCritiqueKeepsRegisterlessChainCompatibility
   or add a sibling that runs the close verb on a register-less chain.
4. F-3, the fixture bed. Every design-critic launch in
   metasystem/scripts/agents/dispatch-fixtures.sh passes `--design
   metasystem/scripts/agents/roles/design-critic.md`, but the bed
   repository has `scripts/agents` at its top level and no `metasystem/`
   directory; dispatch.sh dies "design-critic --design file does not
   exist in the reviewed workspace" before any assertion. Fix in the
   fixture, not the rule: the bed gets the design file where the rule
   expects it (a `metasystem/scripts/agents/roles/design-critic.md`
   copy inside the bed), or `designBlobSource` and the dispatch.sh
   check derive the prefix the same way as fix 1 so a toplevel bed
   passes a bed-relative path. Choose the one consistent with fix 1
   and record it under `decisions`.
5. F-4, STR3-GAP05 proves less than it demands. The test drives the
   register step directly with a literal opid and never calls the verb.
   Fix: drive `AcceptedRiskDecision` through the injected prover the
   sibling human-only verbs use (metasystem/cmd/metasystem/goalsync_mutations_test.go,
   `fixedTemporaryGoalAuthority`): the pair is refused; the human writes
   the goal line, the counselor register line and the register entry
   in that order; close then succeeds; a bounded finding is refused.
6. F-5, the union's two named fixtures. In
   metasystem/internal/validate/conformance_test.go add the two
   STR2-CRITIC-UNION-11 cases with TWO critic roots: clean plus material
   on the same tree refuses; differing class refuses.
7. F-6, three unproven halves. One BuildRecord test on role
   design-critic with a canonical outputs file asserting
   `declaredOutputs`, `declaredOutputsDigest`, `declaredOutputsSource`
   and `reviewRoundLimit: 3` on the root; one delegate front-door test
   that design dispatch without `--outputs` is refused with the exact
   text.
8. F-8, print every blocking entry. `CritiqueRegisterClose` and
   `CloseCheck` return at the first severe or unproven entry; the close
   table's first row says "print each with artifact". Fix: collect all
   blocking entries, print each with its artifact, then refuse; the
   test asserts two entries are both named.
9. F-9 (note), next steps in refusals: the severe-blocks-close message
   ends with "next: goal accept-risk --finding <id> --chain <root> --by
   <human> --why, or raise the goal budget and run job
   critique-budget-rebind"; the malformed-round-accounting message
   ends with "next: job critique-budget-rebind --root-job <root>".
10. F-10 (note), leave as is.

# Gate

As the 2c brief: `scripts/agents/go-gate.sh --fast` silent; `go test
-count=1 -timeout 30m $(go list ./... | grep -v /internal/goal$)`
green but for the named pre-existing failures; for internal/goal run
`go test -count=1 -run 'STR|Accept|Obligation|Discharge' ./internal/goal/`;
return-schema-fixtures.sh; dispatch-fixtures.sh and goal-cli-fixtures.sh
if your sandbox can run them (the seat reruns them). Stage nothing,
no commit wrapper, no plans or records. diffBoundary as the 2b brief
plus metasystem/cmd/metasystem/goalsync_mutations_test.go and
metasystem/internal/validate/conformance_test.go.

# Constraints

Wall-clock budget: 45 minutes; return by minute 40 whatever the
state, listing what is fixed and what is not. Version-2 implementer
JSON with the test names and the gate lines.

# Gap Rule

stop and report a gap; never fill it silently. The grain: mechanical
choices are recorded under `decisions` and built.
