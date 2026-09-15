# Build read: design-delegates-run-fresh-and-bounded (Codex build task-mu2loun6-g17xeg)

Prepared copy: scratchpad/g13/wt29 (base af22d53dc plus the uncommitted diff; git status shows the same five files as design-delegates-code.diff).

## Checklist

1. pass. metasystem/scripts/agents/templates/design-brief.md:1-71 has all the sections: Revision (first draft or revision N, with the reason), Context pack (read it first, open other files only to check a cited line; diagnosis or prior revision, critique findings, `<file>:<start>-<end>` excerpts, one example page), Tool-call budget (stop and list what went unchecked), Page-size ceiling (cut, or stop and propose a split), Fresh session (never resume), and the return shape (path plus `DESIGN: ready (<words> words)` or `DESIGN: blocked (<reason>)`). A grep for runtime and model names finds nothing. The file ends with a newline.
2. pass. metasystem/docs/orchestration.md:24-25 adds lines only. The new paragraph comes right after the one at line 23 ("Before dispatching a code read or critique"). It names the context pack, a numeric tool-call budget and a word-count page-size ceiling. It says every revision goes to a fresh delegate that is never resumed, and calls that an exception to the paragraph beginning "Corrections use" (still at line 185, unchanged). It records tokens and calls as design_tokens and design_calls on a type=design receipt for each revision, and it names no runtime or model. The paragraph that begins "The shared design-author lane" (line 174) is unchanged.
3. pass. receipt.go:45-46 adds the fields right after ReadCalls. receipt.go:189-194 validates them with epochRe, the same way as read_tokens and read_calls. receipt.go:233-238 appends them after read_calls and before critique_waived, only when set, so a receipt without them has the same line as before. receipt.go:283-284 adds them to the correction check. fieldRe (`^[a-z][a-z0-9_-]*$`) accepts both names. receipt_verbs.go:58-59 adds the flags right after read-calls. The struct's removed lines are layout only (gofmt realignment, same names and types). Note on the checklist count: there are 12 removed lines in 31-50, and 13 across the whole file (the 13th is the old correction check at 283, which the new two-line condition replaces).
4. pass. receipt_test.go:118-140 covers receipts without and with the fields and checks that the old line shape holds. receipt_test.go:176 adds both fields to the invalid-correction loop. receipt_test.go:197-198 refuses invalid values in add. receipt_test.go:267-286 covers a valid design_calls correction. The imports (crypto/sha1, fmt, time) and helpers (baseOptions, fixedNow) already exist. type=design is an accepted type (receipt.go:155). The expected correction text matches the format at receipt.go:318. I traced the tests by reading them and did not run them, because the brief forbids running tests.
5. pass. 151 insertions and 14 deletions make 165 changed lines, within the 250 allowed. Five files changed, all on the brief's list. None of the "Do not change" files is touched, and memory/receipts.log is not modified. No source comments were added to Go code (the only `#` lines are the template's markdown headings). A grep for read_tokens across the copy (excluding receipts.log and records) finds it documented only in the Go flag help text, where design-tokens and design-calls now sit beside it. No other document mentions read_tokens, so none needed the new fields. The receipt stats parser switches on key names (receipt.go:393) and does not know read_tokens either, so both old and new receipts still parse.

## Material findings

AGREE. No material findings.

## Polish (non-blocking)

- receipt_test.go:274 and :281 ignore os.ReadFile errors (`data, _ :=`). This copies TestCorrectLifecycle.
- The valid-correction test covers design_calls only. design_tokens is covered for invalid values but not for a valid correction. The code path is the same, so the risk is low.

## Unchecked within budget

None. Nothing was run (go test, gates and the engine are forbidden by the brief), so test pass status is not confirmed by this read.

## Tool calls used

10 of 25.
