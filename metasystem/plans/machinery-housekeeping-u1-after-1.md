# Brief: machinery-housekeeping U1, correction 1

Working Mode: Implement
U1 is uncommitted in this worktree. One Opus read found two material defects in the launcher-death fix (the rebase and findings-store fixes hold); fix exactly these, in internal/identity/fixture_custodian_witness_test.go.

F-1. :386-390: the new cleanup's stop stage kills only the launcher process; the old cleanup killed its process group (HEAD :260 syscall.Kill(-pid, SIGKILL)). Children are joined only once own() captures them (:425 on); a waitWitnessRef failure during setup (a checkWitness fatal before :425/:433) leaves uncaptured descendants alive while cleanup.directory removes the directory, the race the design requires closed "including setup failure". The stop stage kills the group: syscall.Kill(-command.Process.Pid, SIGKILL).
F-2. :303-361 and :433: assertLauncherWitnessCleanupOrder tests a hand-built launcherWitnessCleanup, not the wiring in runLauncherDeathWitness; deleting own(custodian, true) at :433 (the design's mutation "remove the custodian-exit join") stays green. Assert on the real run: before release, cleanup.writers holds the custodian ref (or a barrier on the real custodian-join registration), so that mutation turns the test red deterministically.

Check: go build ./... && go vet ./internal/identity/ && go test -count=3 -parallel 64 -run '^(TestLauncherDeathKillsTheTest|<the launcher witness tests you touched>)$' ./internal/identity/, plus each mutation (remove own(custodian, true); kill only the launcher) red. Every new test calls t.Parallel(); no wall-clock waits. Never open any metasystem.conf.local; do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits and each mutation result.
