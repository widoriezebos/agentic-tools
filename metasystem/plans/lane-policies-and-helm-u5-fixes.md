# Brief: lane-policies-and-helm, unit U5-fixes (split from U5 under the stop rule)

Working Mode: Implement
U5 is committed on this branch. Its final read found one material defect; fix exactly this.

internal/goal/verbs.go:4456 (admitClaimAreas per arc member) with internal/goal/areas.go:173-185: the arc loop binds member 1 on the same in-memory tree (verbs.go:4471-4474), and ClaimAreas then counts member 1's fresh claim as a blocker for member 2, so `goal claim --arc` refuses itself whenever its members share declared areas (members of one design usually do), and `goal claim arc-two` refuses when this seat holds arc-one; the printed remedy can never finish the arc. Decided by m1e for Wido: members of ONE arc under ONE claimant are not sequenced against each other (as the quota counts them once, validate.go:399). In ClaimAreas skip any other goal claimed by the same claimant in the same arc. Test: an arc whose two members declare the same area is claimed whole (mutation: remove the skip, red); a different claimant's overlapping claim still blocks.

Check: go build ./... && go vet ./internal/goal/... ./cmd/metasystem/ && go test -count=1 -timeout 30m ./internal/goal/... && go test -count=1 -timeout 30m -run 'TestGoalClaim|TestClaim' ./cmd/metasystem/ && go run ./cmd/devgate static
t.Parallel(); never open any metasystem.conf.local; do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, the test with its mutation.
