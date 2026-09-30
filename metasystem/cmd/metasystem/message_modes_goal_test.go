package main

// Group 1 of the "Messages a Person Reads" rewrite: the goal ledger and the
// goal verbs. Every path here is enforced by TestAuditMessagesAPersonReads.
var _ = enforceMessages(
	"internal/goal",
	"internal/goalbudget",
	"internal/goalrevision",
	"cmd/metasystem/goal.go",
	"cmd/metasystem/goal_branch.go",
	"cmd/metasystem/goal_list.go",
	"cmd/metasystem/goal_next_presence.go",
	"cmd/metasystem/goal_refusal.go",
	"cmd/metasystem/goalsync_mutations.go",
	"cmd/metasystem/goalsync_owner_report.go",
	"cmd/metasystem/goalsync_verbs.go",
	"cmd/metasystem/intent_goal_land_without_sitting.go",
	"cmd/metasystem/intent_goal_permission.go",
	"cmd/metasystem/intent_goal_review.go",
	"cmd/metasystem/intent_goals.go",
)
