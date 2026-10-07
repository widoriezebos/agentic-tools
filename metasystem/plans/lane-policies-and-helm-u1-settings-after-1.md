# Brief: lane-policies-and-helm U1, correction 1

Working Mode: Implement
The unit is uncommitted in this worktree. One Opus read found two material defects; fix exactly these.

1. cmd/metasystem/intent_policy.go:157-168: every settings show read error prints `metasystem settings set KEY auto --repo <checkout>`, which cures only a bad value in the config file. Pick the remedy by cause: a bad environment value (internal/config/policy.go:128,157) names unsetting METASYSTEM_<KEY>; a corrupt coordinator declaration names `metasystem settings coordinator` declare/withdraw; a corrupt lane record names `metasystem landing set PATH`; a malformed helm signature (policy.go:221-222) names `metasystem helm return`; a bad file value keeps settings set. Test through settings show for each cause: following the printed remedy makes the next read succeed (mutation: one remedy for all, red).
2. intent_policy.go:199-201: settingsPerson finds the calling checkout with helm.Locate(inv.cwd), which returns the primary checkout, but enrollment lives in each worktree's own state root (artifacts/agents/authority/human-terminal.json). Try the cwd's own layout root first, then the primary checkout, then the destination. Test: a person enrolled only in the linked worktree they run from sets a lane key and set-by records them (mutation: primary first only, red).
Also (not material, small): call humanauthority.RecordAttorneyRefusal on a helm or grant refusal, as directPersonProof does.

Check: go build ./... && go vet ./cmd/metasystem/ ./internal/config/ && go test -count=1 -timeout 30m ./internal/config/ && go test -count=1 -timeout 30m -run 'TestPolicy|TestSettings|TestIntentSettings|TestAudit|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static
t.Parallel(); synthetic settings only; never open any metasystem.conf.local; do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat of the correction, each test with its mutation.
