package main

import (
	"os"
	"testing"
)

func TestGoalBranchMissingRefExitChild(t *testing.T) {
	t.Parallel()
	if os.Getenv("GOAL_BRANCH_MISSING_REF_CHILD") == "1" {
		os.Exit(1)
	}
}
