# Brief: lane-reads-its-policies U4, correction 1

Working Mode: Implement
U4 is uncommitted in this worktree. One Opus read found three material defects; fix exactly these.

1. internal/landing/plain/stop.go:178-182 closeGoalStopsLocked now matches only Loop == "lane-return"; its callers (hand-in queue.go:321, return queue.go:376, push push.go:170) no longer close lane-proof and lane-gate stops naming the goal, while Stop.Command() (stop.go:57-66) still prints `metasystem landing return GOAL --cause <kind> --reason TEXT` for unclassified or own proof stops; closeProofAdmissionStopsLocked (proof_admission.go:163) closes only `landing prove` commands. So a followed return leaves the stop open, NewestStop/status keep "the lane stopped", questionHold (landing_agent.go:213-216) waits forever. A return, hand-in or push closes the proof or gate stop whose printed command it is, matched on that stop's goal and tree. Restore TestLandingStopQuestionClosesOnReturnOrHandInAndNotAnAnswer (landing_stop_record_test.go) to expect the lane-proof stop closed after the return (mutation: lane-return only, red).
2. prove.go checkBound calls redContinuationLocked for any newest red when Person == nil without checking ClassificationPerson/ClassificationOf; red.go:35 dedupes only against open stops and red.go:177 closes on the person's classification, so each later agent prove appends a fresh lane-classify stop and question, before NoRepeat. Skip the classify stop when the newest red already carries a non-pending person classification; NoRepeat and the return stop apply. Test: person classifies, agent proves the same tree twice, no new stop or question (mutation: no classification check, red).
3. resolve.go:293-300 (regeneration hold under on-red=person) and :240-244 (source-conflict hold when checkReturnLocked refuses) return without resolveWaitingLocked(..., held=true) (the baseline path :303 does), so Pending (queue.go:458) selects the entry again and every turn re-merges and regenerates without a person. Record the waiting line held with its cause on both paths. Extend TestLandingRegenerationPersonStopsBeforeReplay: Held is set and a second resolve runs nothing (mutation: no held mark, red).


The five questions are answered in Decision 1's table; implement those answers, and say in the return where each lives in code.

# Defect classes the reads keep finding (avoid each; the read checks them)
1. A refusal remedy that cannot succeed when followed, or that undoes the gate.
2. An agent given a person's power, or a person treated as an agent. A person's act is never refused except to prevent damage.
3. An older or records entry hiding current state.
4. A test seam hiding production behavior: every new function has a production caller and a test through the public verb; no stub returns an error shape production does not.

# Check (impacted tests only; the full gate runs per batch on the goal branch)
Build, vet, the packages you changed with -count=1 -timeout 30m, the cmd tests you added or changed by name, their mutations, then the cmd tests that drive the code you changed (by name or a narrow -run of the area; NOT the whole package and NOT a broad selection: the whole suite runs once per goal on the integration tree, per Wido's rule "expensive test once per batch"), then `go run ./cmd/devgate static`. Report done only when all of these exit 0. If you change a message or skill text, also run `go test -count=1 -timeout 30m -run 'TestAudit|TestInstruction' ./cmd/metasystem/`. Every new test calls t.Parallel(); no wall-clock waits (inject clocks); test executables via testexec. Never open any metasystem.conf.local (synthetic settings only). Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat, and each test with the mutation that turns it red.
