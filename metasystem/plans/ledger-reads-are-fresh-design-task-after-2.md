# Task: fold critique round 2 into plans/designs/ledger-reads-are-fresh.md and accept it

Working Mode: Design
Round 2 found 2 material (round 1: 3). The design is one unit, so a split leaves nothing buildable: under the plan's design stop rule both fold as acceptance items (a line "Acceptance item (round 2): ..." under U1) and the page is accepted. Set Status: accepted; add under the header lines "- Critique: closed at round 2 on 2 material findings folded as acceptance items (rounds 3, 2)". Never open any metasystem.conf.local. Edit only this page.

1. Decided by m1e for Wido (record it): adoption pending is a typed adoption field; up's Outcome stays `armed`, exit 0, the session intent maps it to partial (intent.go:787). Name the machine callers that read up's outcome: internal/missionrunner/launch.go:228 (requireArmed :544), internal/testrun/rearm.go:428, cmd/metasystem/session_isolate.go:82, and say they are unaffected; test 2 asserts through requireArmed and the rearm envelope with a mutation that flips Outcome.
2. Fix the citations: brain preparation's eight seconds is internal/hooks/runtime_hook_start.go:599-605 (brainDeadlineMS 5000, brainWait 5 s + 3 s); supervision arming is internal/up/up.go:833-845 (not cmd/metasystem/up.go), in Decision 2 and in the Decided section.
Notes, one sentence each: why mechanism 3's environment retry does not apply (it governs launched steps keyed by launch id; an inline command read is not one); the scheduler's up --recover-only --if-down never reaches the restamp and is unaffected.
Return the final units table.
