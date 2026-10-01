package main

// Group 3 of the "Messages a Person Reads" rewrite: landing, exceptions and
// manual reviews. The landing path, the landing lane and its batch owner,
// the ledger fence and the exception, evidence and manual-review verbs speak
// in two lines; codes, verdicts and step logs are details.
var _ = enforceMessages(
	"cmd/metasystem/hold.go",
	"cmd/metasystem/holder_step.go",
	"cmd/metasystem/intent_evidence.go",
	"cmd/metasystem/intent_exception.go",
	"cmd/metasystem/intent_exception_release.go",
	"cmd/metasystem/intent_land_release.go",
	"cmd/metasystem/intent_land_staged.go",
	"cmd/metasystem/intent_landing.go",
	"cmd/metasystem/intent_manual_review.go",
	"cmd/metasystem/intent_manual_submit.go",
	"cmd/metasystem/landing_gate.go",
	"cmd/metasystem/landing_path.go",
	"cmd/metasystem/landing_stop.go",
	"cmd/metasystem/landing_verbs.go",
	"internal/landing",
	"internal/ledgerfence",
	"internal/gittree",
)
