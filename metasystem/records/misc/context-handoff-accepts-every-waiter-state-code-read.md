# Code read: context-handoff-accepts-every-waiter-state (closing read)

Reader: Claude Opus 5, a fresh bounded delegate (25-call budget, 12 calls used), 2026-09-15 about 16:05 CEST, on worktree chaws-build (base 06b3d37c). Read-only. Brief: artifacts/reports/codex-chaws-brief.md; builder's result: artifacts/reports/codex-chaws-result.md. The reader's reply, verbatim:

VERDICT: LAND, no material findings.

1. Boundary and ceiling: exactly the five briefed paths changed, nothing else; numstat 157+/3- = 160 changed lines excluding the testdata copy, at the ceiling, not over. Writer internal/run/waiter.go untouched; schema, refusal codes, messages, the SchemaVersion==0 and MainId!=mainID skips and the "schema %d" error are unchanged. internal/steward/testdata/handoff-waiter-deadline-m1b.json is present at sha ff82ed99..., the hash the builder reports before and after; byte-identity with m1b's original rests on the seat's placement.
2. State list complete: the writer's only State writes are waiter.go:890 (State: "registering"), :967 and :1339 (current.State = "pending"), :1277 (current.State, current.Delivery = "registering", ""), :641 (current.State = state, the finishV2 parameter), and the 32 s.finishV2 (argument 5) and 43 waitResult (argument 2) call sites, every one a string literal, all six states, classed as the brief says. No other package writes a Waiter State. The two `state` identifiers the AST test forwards are exactly waiter.go:638 and :641, both inside finishV2, whose callers are literal-checked; `forwarded != 2` pins that hatch shut, and any non-literal, non-constant elsewhere hits the t.Fatalf at waiter_states_test.go:44.
3. Reader correctness: waiterInFlight in handoff_capture.go:415-427 is the only waiter-State reader in internal/steward (grep-confirmed); in-flight still returns HANDOFF_WAIT_IN_FLIGHT, ended passes, unlisted still yields "unknown state %q".
4. Witnesses: M1 and M3 fail on handoff_capture_test.go:320 (the hardcoded registering/pending oracle, independent of the list's own Class field) and :305; M2, M4 and M6 fail on waiter_states_test.go:80 via maps.Equal; M5 fails on :331. TestHandoffAcceptsPreservedDeadlineWaiter re-reads the written bytes, asserts bytes.Equal with the fixture, and calls waiterInFlight(root, waiter.MainId), the record's own mainId. Judged from the test code, all six hold.
5. Defect fixed: the fixture is exactly m1b's record (state deadline, mainId main-1789191336-90295-e4b24b, schemaVersion 2) and now classes ended, so the handoff no longer refuses.

Carry-ins (non-material): internal/run/wait_hint.go:151 and internal/goal/turnverdict.go:669 still hardcode "registering" and "pending" outside the brief's scope; WaiterStateClass's zero value is Ended, so a future list entry with an omitted Class would silently class as ended (the AST test checks membership only, and the steward oracle hardcodes in-flight to today's two names).

## Seat additions after the read (m1e)

- The seat confirmed the testdata copy byte-identical to m1b's record (cmp) at launch and after the build.
- go-gate --fast found two items outside the builder's Boundary, fixed by the seat: seven HANDOFF_* refusal sites re-pinned in internal/refusal/register.go to the shifted emission lines (the brief should have included the register in the Boundary), and the AST test's deprecated parser.ParseDir replaced by per-file parser.ParseFile over os.ReadDir (staticcheck SA1019). Unit total 190 changed lines.
- Seat verification: gofmt, go build ./..., go vet, go test -race -count=1 ./internal/run (1.8 s) and ./internal/steward (146 s), go-gate --fast green after the fixes.
