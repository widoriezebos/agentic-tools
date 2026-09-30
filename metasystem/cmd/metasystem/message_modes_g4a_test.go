package main

// Group G4a of the round-2 message rewrite: delivery (design write, work
// brief/build/wait/workspace/review/revise/land/finish, test
// run/declare-moves/wait, settings show/keys/set/check) and the internal
// packages those verbs call, except internal/missionrunner and
// internal/delegation (G4b). Each path below speaks in the two lines of
// "Messages a Person Reads" in the traced reading too. The protocol files a
// parent reads by leading code (proof_run_protocol.go,
// internal/proofrun/protocol.go, internal/testrun/protocol.go) stay kept.
var g4aPaths = []string{
	"cmd/metasystem/delegate.go",
	"cmd/metasystem/delegate_drivers.go",
	"cmd/metasystem/delegate_host.go",
	"cmd/metasystem/delegate_supervisor.go",
	"cmd/metasystem/design_record_hint.go",
	"cmd/metasystem/dispatch_verbs.go",
	"cmd/metasystem/intent_delivery.go",
	"cmd/metasystem/intent_design.go",
	"cmd/metasystem/intent_design_dispositions.go",
	"cmd/metasystem/intent_design_review.go",
	"cmd/metasystem/intent_land_change.go",
	"cmd/metasystem/intent_land_release.go",
	"cmd/metasystem/intent_land_staged.go",
	"cmd/metasystem/intent_manual_review.go",
	"cmd/metasystem/intent_manual_submit.go",
	"cmd/metasystem/intent_references.go",
	"cmd/metasystem/intent_review_binding.go",
	"cmd/metasystem/intent_review_help.go",
	"cmd/metasystem/intent_selection.go",
	"cmd/metasystem/intent_sent_back.go",
	"cmd/metasystem/intent_unit_review.go",
	"cmd/metasystem/intent_work.go",
	"cmd/metasystem/intent_work_workspace.go",
	"cmd/metasystem/intent_worktree.go",
	"cmd/metasystem/mission.go",
	"cmd/metasystem/mission_contract_verbs.go",
	"cmd/metasystem/missionrunner_verbs.go",
	"cmd/metasystem/proof_run.go",
	"internal/candidateengine",
	"internal/dispatch",
	"internal/dispatchproc",
	"internal/enginecause",
	"internal/gaterun",
	"internal/proofrun",
	"internal/testenv",
	"internal/testpolicy",
	"internal/testrun",
}

var (
	_ = enforceTracedMessages(g4aPaths...)
	_ = enforceMessages("cmd/metasystem/delegate_drivers.go", "cmd/metasystem/delegate_supervisor.go")
)
