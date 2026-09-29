# Build brief: public error messages outside U9b

- **Source (the spec):** the audit at
  `/Users/wido/LocalStorage/agentic-tools-evidence/error-messages-20260928/public-audit.md`.
  Read it whole.
- **Wido, verbatim:** "An error message that confuses you IS REALLY BAD. Also check the
  other error messages." Tonight he answered "fix and land those", meaning the rows
  outside U9b.
- **Workspace:** the worktree `/Users/wido/LocalStorage/GitHub/agentic-tools-emsg`, branch
  `public-error-messages`, based on `evidence-root-default`, which lands first. Run every
  Go command from `metasystem/`.
- **In scope:** every row whose last column says "outside U9b" and whose emission file is
  not changed on u9b. Before you edit a file, check it with
  `git -C /Users/wido/LocalStorage/GitHub/verbs-u9a diff --stat main...u9b -- <path>`
  and `git -C /Users/wido/LocalStorage/GitHub/verbs-u9a status --short -- <path>`.
  Skip a row that touches a file U9b changes, and list it as skipped. For rows that span
  both, fix only the half outside U9b, if that half stands on its own.
- **Out of scope:** `cmd/metasystem/main.go` dispatch, `cmd/metasystem/intent.go`, every
  internal family's flag parsing, and every row marked U9b-touched. Those belong to seat
  m1e-c4's U9b.
- **Rule for each fix:** the smallest change that makes the message true, says what went
  wrong in the reader's terms, and names a command that works. For exit codes, a
  refusal or an unknown target exits non-zero, and a status read that found its subject
  exits 0. Change behaviour only where the audit's finding is that it is false (for
  example exit 0 for a mission that doesn't exist). Add no new flags, verbs or
  mechanisms. Where the audit marks a row "inferred", reproduce it first; fix it only if
  it reproduces, otherwise list it.
- **Tests:** each fix is held by a test that failed before it. Behaviour tests run with
  real Git unavailable; use the existing seams. No `t.Setenv`. Existing tests that pin
  the old text are updated to the new one.
- **Pinned checks:** refusal-register rows (`internal/refusal/register.go`, ±2 lines,
  `TestHCL03EveryRowedSiteNamesAnEmission`) for any line that moves. Set each re-pinned
  row wrong once, watch it fail, then restore it.
- **Proof, before you return:** `go run ./cmd/devgate static` passes (about 10 minutes;
  run it once at the end), and `go test` passes for every package you touched plus
  `./internal/refusal/`. `METASYSTEM_TESTING_WORKERS=9`.
- **Return:** `metasystem/plans/public-error-messages-build-report.md`, committed. It
  holds one line per EM id: fixed, with the test name; skipped because of U9b; or not
  reproduced. Then the proof commands and their results. Print it as your answer, with
  the last commit.
