Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Goal

The third and last correction of slice 2b (the material stop and the
close), on chain str-p2-build-2d after its closing review
(metasystem/records/misc/severity-tiered-rigor-p2-2b-critique-cc3.md).
One material finding and one cosmetic one; everything else in your
tree stays as it is. Your round-1 tree is the base; change only what
the two items below name.

# Item 1: STR2P2B-01, the fold tolerates a root without demotions

metasystem/internal/dispatch/finding_register.go, the fold in
`CritiqueRegisterAdvance` (about line 130): when the round demotes a
finding, the code reads `root["demotions"].([]any)` and refuses with
"has malformed demotions" when the assertion fails. A critic root
written before this slice lands has no `demotions` member at all
(main's engine never writes one; only this slice's BuildRecord adds
`record["demotions"] = []any{}`), so the first post-landing fold of
such a chain that demotes anything is refused.

Make the reader distinguish absent from malformed exactly as
`exhaustions` in metasystem/internal/dispatch/critique.go does for
`critiqueExhaustions`: an absent member reads as an empty list and the
fold writes the member; a present member that is not a list is the
"malformed demotions" refusal, unchanged. No other behaviour changes.

Test, in internal/dispatch: a critic root record without the
`demotions` member (the pre-slice shape) whose round carries a
material finding with no canonical artifact folds without error and
afterwards carries a `demotions` list with that one demotion; and a
root whose `demotions` is a string still refuses with the malformed
text. Use the register fixtures of finding_register_test.go.

# Item 2: STR2P2B-04, indentation

metasystem/scripts/agents/dispatch.sh: the design-critic argument
checks and the workspace resolution this slice added in `dispatch_job`
are tab-indented inside a two-space-indented function, and one `fi`
moved onto a tab-indented line. Re-indent those lines with spaces to
match their neighbours. Bytes only; no logic change.

# Recorded, do not build

STR2P2B-02 (a path containing the token " b/"; none exists) and
STR2P2B-03 (the review-round seam returning the constant three; the
next slice rebinds it to the tier) are recorded in the critique
record and are not yours.

# Gate

`cd metasystem && gofmt -l internal/dispatch` (empty) `&& go vet
./internal/dispatch/ && go test -count=1 ./internal/dispatch/` green;
`bash scripts/agents/coverage-delta.sh ./internal/dispatch
./internal/validate` still at or above both floors; `bash -n
scripts/agents/dispatch.sh`. Stage nothing, no commit wrapper, no
plans or records. diffBoundary: every file that differs from main.
Name the new tests in your return.

# Constraints

Wall-clock budget: 25 minutes; return by minute 20. Version-2
implementer JSON.

# Gap Rule

stop and report a gap; never fill it silently. Anything beyond the
two items is a gap to report, not to build.
