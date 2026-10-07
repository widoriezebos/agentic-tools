# Brief: lane-policies-and-helm U4, correction 1

Working Mode: Implement
The unit is uncommitted in this worktree. One Opus read found these defects; fix exactly these.

1. internal/helm/helm.go:169 refuses a write whose record.Checkout differs from the located checkout, and the repeat path cmd/metasystem/intent_helm.go:215-223 writes standing.Record back unchanged, so a stale checkout field (repo moved, field edited) makes every repeat take fail with a retry that cannot succeed. In the repeat path set record.Checkout to the located checkout. Test: a held signature with a stale checkout; a repeat take by the same person succeeds and the field is current (mutation: write the standing record unchanged, red).
2. cmd/metasystem/intent_landing.go:792-794: when the lane is held, landing run's refusal names `helm return --repo <lane>` to whoever runs it, including the landing agent, who is now refused. Use humanauthority.PersonActRemedy (as intent_policy.go:184 does) so the step says it is the person's act at their enrolled terminal. Also the same wording at helmLine (intent_helm.go:356) and the malformed status line (:493). Message-only; no guard changes. Run the message audit.
3. (promoted) The repeat take no longer refreshes Leader, LeaderRef, Enrollment and EnrolledAs (record = standing.Record; the comparison at :219 is always true), so a person re-taking at a newly enrolled terminal stays reported as not enrolled. On a repeat by the same person keep the original take's identity and snapshots but refresh the terminal fields from this take. Test: take at one terminal, enroll another, re-take there: helm status reports the new terminal as enrolled (mutation: keep the standing terminal fields, red).

Check: go build ./... && go vet ./internal/helm/ ./cmd/metasystem/ && go test -count=1 -timeout 30m ./internal/helm/ ./internal/hooks/ && go test -count=1 -timeout 30m -run 'TestHelm|TestPolicy|TestLandingRun|TestAudit|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static
t.Parallel(); never open any metasystem.conf.local; do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat of the correction, each test with its mutation.
