# Code read: code-reads-and-critiques-run-bounded

Verdict: land (0 material findings)

Reader: Opus 5 code critic, not the author. Read only: no edits to the copy.

## Prepared copy and diff

- Copy: `scratchpad/g12/wt28`, HEAD `f9b27a40aa855acf9e0e12cdecd87c958bfa4220` (goal claim), 5 files changed, +115/-3, uncommitted.
- `bounded-reads-code.diff` matches `git diff` in the copy byte for byte (`cmp`).
- Contract: the DONE clause of `metasystem/plans/goals/code-reads-and-critiques-run-bounded.md`.
- Materiality test used (from the brief): a finding is material only if it breaks the DONE clause, breaks existing receipts, or rewrites text in `docs/orchestration.md`.

## Checklist results

1. `scripts/agents/templates/review-brief.md`: pass.
   - It has a "Prepared copy" section (path, plus a commit SHA or base and candidate), a "Checklist" section whose items name `<file>:<start>-<end>` or fixture rows, and a "Tool-call budget" section (a number, then stop and list what was not checked).
   - The "Findings artifact and return shape" section asks for two lines only: the findings path and `VERDICT: land` or `VERDICT: fix first (<N> material findings)`.
   - The "Scoped confirmation read after a fold" section makes the folded findings the whole checklist, with its own budget and the same return shape.
   - The removed "Return format" text (most severe first; file, rule, and failure; AGREE) now lives in the findings-file section.
   - No script or Go code reads the template: grepping `review-brief` and `Return format` under scripts, internal, and cmd found nothing.
   - The wording is plain; see O-5 for one term.
2. `docs/orchestration.md`: pass.
   - The change only adds text: `git diff --numstat` gives `2 0`, and `git diff -U0` has zero removed lines. It is one new paragraph between the Delegation Contract paragraph and `## The Collaboration Loop`.
   - The paragraph covers all four points: prepare the copy and checklist before dispatch, a fresh reader for each round that is never resumed, a numeric tool-call budget, and a confirmation read limited to the folded findings.
   - The strings the existing gates look for are unchanged: the bounded-read sentence and `metasystem context handoff --root` (conformance-fixtures.sh:8-24).
   - `metasystem validate preamble-quotes --root <copy> --roles-dir scripts/agents/roles` exited 0, so no role quote drifted.
3. `internal/receipt/receipt.go` and `cmd/metasystem/receipt_verbs.go`: pass.
   - Both fields are optional: an empty value skips the check and the output (receipt.go:181-186, 219-224).
   - I built the binary from the copy and ran it against a temp ledger. A receipt without the fields serializes as before: `...|delegate=none|built_by=delegate|critique_waived=none|waiver_stream=none|note=`. With the fields, the line gains `|read_tokens=16000000|read_calls=40|` between built_by and critique_waived.
   - `--read-tokens 16m` and `--read-calls 1.5` exit 2 with `invalid --read-tokens: 16m` and `invalid --read-calls: 1.5`. Neither creates the ledger file or its `memory/` directory, because the checks return before `MkdirAll`.
   - Correction: `receipt correct --field read_calls --now 16m` exits 2 with `invalid corrected read_calls value: 16m`. A valid `--was 40 --now 38` writes `CORRECTION|...|field=read_calls|was=40|now=38|reason=probe`.
   - The receipt package has no amend path; its only functions are Add, Correct, Retro, Stats, and Check.
   - Existing readers are unaffected. `receipt.Stats`, metrics `receiptFields` and `loadReceipts` (data.go:354-440), and landing `receiptline.go:120` all split lines into key=value pairs and ignore unknown keys. Metrics applies corrections to any field name. `scripts/receipt.sh` passes its arguments straight to the Go binary (`exec "$ms" receipt "$@" --root "$here"`), so the new flags get through.
   - Values must match `epochRe` (`^[0-9]+$`, ASCII digits only).
4. `internal/receipt/receipt_test.go`: pass.
   - `TestReadMetricsAreOptionalAndNumeric` covers both cases: a receipt without the fields keeps the old shape, and a receipt with them gets the new text.
   - `TestAddValidation` adds the `16m` and `many` cases. `TestReceiptProvenanceValidationFailsClosed` covers `many` on Correct for both fields.
   - `GOCACHE=/tmp/claude-501/gocache-bounded-read go test -count=1 ./internal/receipt` gave `ok ... 0.374s`.

## Findings

No material findings. All observations below are non-material and do not block landing.

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| O-1 | medium | no | Nothing in the five changed files tells the seat to pass `--read-tokens` and `--read-calls` when it writes a receipt. One receipt line holds one pair of values, and a unit has two or three reads. So "the receipt records tokens and calls per read" works only if the seat writes one receipt per read, or sums the reads, and nobody tells it to. The DONE clause is met at the field level, and checklist item 2 does not require this instruction, so I did not count it. The adjudicator may still want one sentence in the seat instructions. | The flags are new in this diff, so no older doc can mention them. The orchestration.md paragraph and the template never mention receipts. receipt.go:219-224 writes a single pair per line. |
| O-2 | low | no | `receipt correct` cannot add read metrics to a receipt that was written without them. It can only change a value that is already on the line. goal and built_by already behave this way. | receipt.go:299 requires `\|field=was\|` on the original line. When the field is absent, `--was ""` looks for `\|read_tokens=\|`, which never matches. |
| O-3 | low | no | The tests check the refusal message for invalid values but do not assert that no row was written. The code is correct; the tests just do not prove this part. | TestAddValidation compares only `result.Code` and `Err[0]`. My probe found neither the file nor its directory after the `16m` and `1.5` refusals. |
| O-4 | low | no | No test covers a valid correction of read_tokens or read_calls; only the invalid path is tested. | receipt_test.go covers only `NowValue = "many"`. The CLI probe showed that a valid correction (`was=40 now=38`) is recorded. |
| O-5 | low | no | The template heading "Scoped confirmation read after a fold" uses "fold" without defining it. The next line ("Folded findings ... folded correction to confirm") makes the meaning clear enough. | review-brief.md:58-67. |
| O-6 | low | no | After applying corrections, metrics re-checks goal and built_by but not read_tokens or read_calls. A non-numeric value in a hand-edited ledger would pass unnoticed. Nothing reads these fields yet. | data.go:432-444 checks only `ValidGoalValue` and `ValidBuiltByValue`. |

Material finding count: 0

## Budget and unchecked items

- Tool calls used: 23 of 40. Every checklist item was checked.
- Not checked, because they are outside the checklist: the `cmd/metasystem` package tests, the full `validate-metasystem.sh`, and any docs outside the five changed files.
- I deleted the probe binary, temp ledger, and build cache under `/tmp/claude-501` after the read.
