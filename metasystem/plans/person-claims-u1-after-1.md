# Brief: person-claims U1, correction 1

Working Mode: Implement
U1 is uncommitted in this worktree. One Opus read found one material defect; fix exactly this.

internal/goal/verbs.go:1557 clears claim.By on any handover to a different pair, so PersonalReservation() is false and internal/goal/validate.go:368-380 reapplies the blocker, pin and approval-gate checks, and validate.go:459 counts it in the at-rest quota again: a person's reservation made over a pin or an unfinished blocker (warnings by design) is rejected at the lane handover ("pinned to machine mac-b but claimed by mac-a; ownership contradicts the pin") with no remedy. The same cause through goal resume of an adopted reservation (bindClaim at internal/goal/stop.go:498 drops By). Fix: keep the validation exemption for a claim whose ownership episode began as a person's reservation (judged from the claim's history or a carried provenance field), through the lane handover and a same-owner resume; a change to an ordinary agent pair that is not a handover or resume gets no person power. Test: reserve a pinned goal and separately a blocked one, approve, adopt, hand over to the lane, then breach-stop and resume: every publication confirmed (mutation: clear the provenance at handover, red).

Check: go build ./... && go vet ./... && go test -count=1 -timeout 30m ./internal/goal/... ./internal/steward/ ./internal/up/ && go test -count=1 -timeout 60m -run 'TestGoal|TestClaim|TestSession|TestUp|TestWork|TestPerson|TestIntent|TestAudit|TestInstruction|TestEvery' ./cmd/metasystem/ && go run ./cmd/devgate static
Never open any metasystem.conf.local; do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat of the correction, the test with its mutation.
