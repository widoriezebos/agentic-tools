# Unit U1 correction 1: the reverse dependent passes; an existing install copy is promoted without a re-enrol; two verbs keep their audience

Working Mode: Implement. Correction of unit U1 of shared-state-in-the-ledger (brief `plans/shared-state-in-the-ledger-u1-shared-enrollment.md`, design D2 / Step 1) after its Opus read. The worktree holds U1 uncommitted on origin/main 5dfaf4fdc; change only what this brief names.

## F-1 (breaking): a reverse-dependent package turns red

`internal/ui/act/helm_test.go` ~43-53 writes only the install's copy and calls `humanauthority.Prove`; with no machine file Prove refuses (TERMINAL_NOT_REACHED) so `TestBootProofIgnoresTheHelm` fails. Fix: the ui/act package's TestMain sets the location override like the other packages (cmd, goal, steward, humanauthority), or its fixture enrols at the machine location. Run EVERY reverse dependent of internal/humanauthority (`go list -f '{{.ImportPath}}' ./... | xargs go list -deps ...` or grep the import) and name each in the report with its exit.

## F-2 (material): the move needs a re-enrol on every install

The design's Step 1 says "no data migration", but `Prove` (authority.go ~947-955) refuses when the machine file (`~/.metasystem/host/human-terminal.json`) is absent, so after the deploy every person act on every install (and the seat agent's power-of-attorney acts, which only AtHelm/AtAttorney after proveEnrolled) refuses until Wido runs `system enroll` once. Fix: when the machine file is absent and the acting install holds a valid copy (the legacy per-install proof), `ReadEnrollment` promotes that copy to the machine file once (same generation; the same promotion `Enroll` already does at authority.go ~890-912) and proceeds; a corrupt machine file still refuses (keep TestEnrollmentBrokenMachineProofRefusesStaleCache). Test: an install with only a legacy copy and no machine file -> the person act passes and the machine file exists afterwards (mutation: skip the promotion -> refused, test fails).

## F-3 / F-4 (decisions, recorded here)

- `landing resolve` and `question withdraw` keep their audience (agents use them; the landing agent's own verb; a channel act): NOT person-gated; the brief's list was too wide. Add an audit-style test that they still admit an agent lineage.
- `landing stop` returns to its previous audience: agents may pause the lane (the steward pauses on health; the UI pause is a session act), only a person resumes (`landing start`). Revert the new person gate on stop; the person resolution (WHO) still comes from the machine-level proof when a person stops.

## Checks

`go vet ./...`; `go test -count=1 -timeout 30m ./internal/humanauthority ./internal/testenv ./internal/goal ./internal/ui/act` and every other reverse dependent of humanauthority by package; cmd tests by name (`TestEnroll|TestHumanAuthority|TestGoalDone|TestGoalResume|TestLandingStart|TestLandingStop|TestLandingReturn|TestLandingResolve|TestWorkLand|TestQuestion|TestSharedEnrollment|TestAmbient|TestLanePerson|TestBootProof`); `go run ./cmd/devgate static` (ignore only the design-record ledger complaint); `./internal/audit ./internal/layering`; `./cmd/metasystem -run TestAudit`. Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat of the correction, each exit, the promotion scenario before/after.
