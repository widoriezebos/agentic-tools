# Goal integration fixes 2: the `none` gate state renders from the requested HEAD; the status layout goldens follow

Working Mode: Implement. Second integration step of goal lane-proves-at-the-batch-risk (the worktree holds U1-U3 and the first integration fix committed; change only what this brief names).

Six `TestAudit*` layout tests are red on the goal tree because unit U3 changed `internal/landing/plain/status.go` ~315-327 (`readStatus`): when gates.jsonl can be read but no gate was requested for HEAD, `last_gate` is now `{Result:"none", Requested: HEAD}` instead of nil (design D3: `none` for this HEAD).

1. Rendering defect (fix in code): `cmd/metasystem/intent_landing.go` ~510 renders `gate.Result + " for " + provedWords(gate.Commit, gate.Tree)`; for the `none` state Commit and Tree are empty, so a person reads `gate   none for  (tree )`. Render the `none` state from the requested HEAD: `gate   none for <short requested id>` (reuse the existing short-id helper), and for `skipped` show the reason after the ids as the other states do.
2. JSON shape: drop the zero `classification-policy` from the `none` record (omitempty) so no golden freezes a year-0001 timestamp; keep `requested` and `result`.
3. Goldens: update `cmd/metasystem/testdata/layout/landing-status{,-stopped,-verbose}.json` and `.txt` to the new shape (TestAuditOutputLayoutJSONUnchanged and TestAuditOutputLayout; the stopped-lane golden also gains the Last section, which D3 permits). Regenerate them with the tests' own update mechanism if one exists (look for an UPDATE env or flag in the audit tests); never hand-edit a golden beyond what the renderer prints.

Checks: `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestAudit'` (every audit green), `-run 'TestLandingStatus|TestReadStatus|TestLandingGate|TestSkillLandingAgent'`, `./internal/landing/plain`, `go run ./cmd/devgate static`. Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat, each exit, the rendered Last section for the three states (none, skipped, red).
