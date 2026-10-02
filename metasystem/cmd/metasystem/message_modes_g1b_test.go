package main

// Group G1b of the traced rewrite (round 2): machine and landing. The
// landing path, the landing lane and its batches, the batch owner, the
// steward and the git plumbing they share speak in two lines wherever the
// traced reading follows their words.
var _ = enforceTracedMessages(
	"cmd/metasystem/hold.go",
	"cmd/metasystem/intent_alert.go",
	"cmd/metasystem/intent_landing.go",
	"cmd/metasystem/intent_landing_prove.go",
	"cmd/metasystem/intent_landing_push.go",
	"cmd/metasystem/intent_landing_return.go",
	"cmd/metasystem/intent_machine.go",
	"cmd/metasystem/landing_verbs.go",
	"cmd/metasystem/steward_verbs.go",
	"cmd/metasystem/supervise_component.go",
	"cmd/metasystem/supervise_owner.go",
	"internal/enginecause",
	"internal/gittree",
	"internal/landing",
	"internal/lease",
	"internal/pattern",
	"internal/steward",
)
