# Unit U2 (shared enrollment) correction 2: the enrollment refresh never writes outside an install; the unit is named as the design names it

Working Mode: Implement. Correction of the shared-enrollment unit of shared-state-in-the-ledger after its machinery read (code-critic-7b8b0386b004625073cd7ad9). Worktree at commit d41c68669; make the change uncommitted on top; never commit, rebase or push; never touch memory/receipts.log or plans/.

## F-1 (material): ReadEnrollment writes a per-install copy with an empty root
`internal/humanauthority/authority.go` ~711-714: every successful read refreshes the install copy at enrollmentPath(root); with root "" (a person act refused from a directory that is not an install: lanePerson, session stop) the copy lands at the relative path artifacts/agents/authority/human-terminal.json under the current directory. Fix: refresh the copy only when root is a resolved install root (non-empty and the install layout is present); otherwise read only. Test: a read with an empty root leaves the current directory untouched (mutation: drop the guard → a file appears).

## N-1 (noted, small guard if cheap): promotion of a legacy copy never lowers a generation already recorded for that install.

## N-2: the unit is the design's U2 (D2 machine-level enrollment); the steward renames the commit trailer at the amend (nothing for the builder).

## Checks
`go vet ./internal/humanauthority`; `go test -count=1 -timeout 30m ./internal/humanauthority ./internal/testenv ./internal/goal ./internal/ui/act ./cmd/metasystem -run 'TestEnroll|TestHumanAuthority|TestSharedEnrollment|TestAmbient|TestLanePerson|TestBootProof|TestAudit'`; `./internal/layering`; `go run ./cmd/devgate static`. Report: diff --stat, every exit, the mutation.
