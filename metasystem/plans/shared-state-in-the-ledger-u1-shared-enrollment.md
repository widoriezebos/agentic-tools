# Unit U1 (step 1) of shared-state-in-the-ledger: one person for every install on the machine

Working Mode: Implement. Design: `plans/designs/shared-state-in-the-ledger.md`, decision D2 and the "Step 1" section (read both). Size: at most 200 production lines. Smallest change.

## Measured defect (lane test findings 84, 61, 72)

The seat (agentic-tools-m1e) and the lane (agentic-tools-landing) are two installs of one repository on one machine. `system enroll --name Wido` enrols ONE terminal per machine ("every machine of the fleet is told"), yet each install checks the person against its own enrollment record: after enrolling the lane pane, `goal done` from the seat pane was refused ("Wido enrolled another one"); `landing start` from the seat pane with `--repo <lane>` was refused the same way; `goal done` run in the lane install against the seat repo was refused as "concluding another's work is a human act" (the lane install lent its agent identity). A person had to re-enrol the other pane for every switch between lane acts and seat acts (findings 83, 84).

## Build

1. The enrollment proof is read from ONE machine-level location (where `system enroll` already records the fleet-wide fact; find it in internal/humanauthority: `enrollmentPath(root)` is per install today) so that every install on the machine resolves the same enrolled terminal; an install's own record stays as a cache and is refreshed from the machine-level proof when they differ.
2. The person verbs resolve WHO from that machine-level proof whatever install they act on: `goal done`, `goal resume`, `goal budget`, `goal approve`, `goal claim`, `work land`, `landing start`, `landing stop`, `landing return`, `landing resolve`, `question withdraw`; the install still records its own custody (what happened where). An agent session (lineage set, no enrolled terminal lineage) still never passes as the person.
3. The refusal texts: "Wido enrolled another one" disappears between installs on one machine; the "concluding another's work is a human act" refusal says who the caller resolved to and why, when it still applies (a real agent).

## Tests

- Two installs of one repository in a test bed, one enrolled terminal (the shell lineage of the test): `goal done` on a landed goal and `landing start` both succeed from the same lineage against either install (mutation: read the install-local record only -> the second install refuses, test fails).
- An agent lineage (no enrolled terminal) is still refused on every listed verb.
- Existing enrollment/authority tests green: `go test -count=1 -timeout 30m ./internal/humanauthority ./internal/goal`; cmd tests by name (`TestEnroll|TestHumanAuthority|TestGoalDone|TestGoalResume|TestLandingStart|TestLandingStop|TestLandingReturn|TestWorkLand|TestQuestion`); `go run ./cmd/devgate static`; `./internal/audit ./internal/layering`; `./cmd/metasystem -run TestAudit`. Any measure or policy here is tested against the real ledger shapes once.

Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat, each exit, the refusal texts before/after.
