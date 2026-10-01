package batch

import (
	"fmt"
	"strings"
)

func BranchLandingMessage(goalID string, build BranchBuild, goalLast bool) string {
	var message strings.Builder
	fmt.Fprintf(&message, "land %s/%s\n\nGoal-Unit: %s/%s\nGoal-Digest: %s\n", goalID, strings.Join(build.Units, "+"), goalID, strings.Join(build.Units, "+"), build.Digest)
	for _, fold := range build.Folds {
		fmt.Fprintf(&message, "Goal-Source: %s\n", fold.ID)
	}
	for _, path := range build.FoldPaths {
		fmt.Fprintf(&message, "Goal-Fold: %s\n", path)
	}
	fmt.Fprintf(&message, "Goal-Source: %s\n", build.Commit)
	for _, coauthor := range build.CoAuthors {
		fmt.Fprintf(&message, "Co-Authored-By: %s\n", coauthor)
	}
	if goalLast {
		fmt.Fprintf(&message, "Goal-Last: %s\n", goalID)
	}
	return message.String()
}
