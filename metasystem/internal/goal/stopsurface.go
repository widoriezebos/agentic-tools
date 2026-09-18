package goal

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
)

// StopSurfaceGoalReader validates a goal file with the canonical parser and
// projects only the fields used to authorize Stop-surface moves.
func StopSurfaceGoalReader(goalID string, content []byte) (audit.StopSurfaceGoalRecord, []string) {
	file, parsedProblems := ParseFile(content)
	problems := make([]string, 0, len(parsedProblems)+1)
	for _, problem := range parsedProblems {
		problems = append(problems, string(problem))
	}
	if file.Id != goalID {
		problems = append(problems, fmt.Sprintf("heading %q does not match its file", file.Id))
	}
	return audit.StopSurfaceGoalRecord{
		State:            file.State,
		StopSurfaceMoves: file.StopSurfaceMoves,
	}, problems
}
