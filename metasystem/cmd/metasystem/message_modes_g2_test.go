package main

// Round 2 of "Messages a Person Reads", group G2 (goals and grants): the
// traced reading enforces the goal ledger, its branch owner, the goal
// revision lock and the goal and grant verbs.
var _ = enforceTracedMessages(
	"internal/goal",
	"internal/goalbudget",
	"internal/goalrevision",
	"cmd/metasystem/goal.go",
	"cmd/metasystem/goal_branch.go",
	"cmd/metasystem/goal_list.go",
	"cmd/metasystem/goal_refusal.go",
	"cmd/metasystem/goalsync_mutations.go",
	"cmd/metasystem/goalsync_verbs.go",
	"cmd/metasystem/intent_goal_land_without_sitting.go",
	"cmd/metasystem/intent_goal_permission.go",
	"cmd/metasystem/intent_goal_review.go",
	"cmd/metasystem/intent_goals.go",
	"cmd/metasystem/intent_planning.go",
)
