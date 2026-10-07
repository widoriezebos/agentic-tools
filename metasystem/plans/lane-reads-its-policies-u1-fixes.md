# Brief: lane-reads-its-policies, unit U1-fixes (split from U1 under the stop rule): regression test only

Working Mode: Implement
m1e already changed internal/landing/plain/batch.go (uncommitted, `git diff`): an older entry is skipped only when its goal's newest entry is a selected, waiting member. Add exactly one regression test through the public verbs, no production change: with landing.batch=1, goal b hands in b1, then b1 is returned (case 1) or superseded (case 2); goal a's branch merges b1 and a hands in; b hands in again at b2 (waiting, not selected, not in the candidate); the selection is just a; the candidate is main plus a merge of a, carrying b1. Assert `landing prove --wait` refuses with LANE_BATCH_MEMBERSHIP and `landing push` pushes nothing (mutation: skip every older entry of every goal, red). Use the real local Git beds the batch tests use.

Check: go vet ./cmd/metasystem/ && go test -count=1 -timeout 30m -run 'TestLandingBatch' ./cmd/metasystem/
Never open any metasystem.conf.local; do not touch memory/ or records/. Leave uncommitted. Return the exits, the test name, and the mutation result.
