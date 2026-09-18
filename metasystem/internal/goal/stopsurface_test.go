package goal

import (
	"strings"
	"testing"
)

func TestStopSurfaceGoalReaderReportsCanonicalProblems(t *testing.T) {
	t.Parallel()
	file := claimedGolden()
	file.StopSurfaceMoves = true
	canonical := RenderFile(file)

	record, problems := StopSurfaceGoalReader(file.Id, canonical)
	if record.State != StateClaimed || !record.StopSurfaceMoves || len(problems) != 0 {
		t.Fatalf("canonical record = %+v, problems = %v", record, problems)
	}

	withUnknown := strings.Replace(string(canonical), "\n\nHistory:", "\n- Unknown: malformed\n\nHistory:", 1)
	_, problems = StopSurfaceGoalReader(file.Id, []byte(withFreshIntegrity(withUnknown)))
	if !strings.Contains(strings.Join(problems, "\n"), `unknown field "Unknown"`) {
		t.Fatalf("unknown-field problems = %v", problems)
	}

	_, problems = StopSurfaceGoalReader("other-goal", canonical)
	if !strings.Contains(strings.Join(problems, "\n"), `heading "backlog-git-sync" does not match its file`) {
		t.Fatalf("heading-mismatch problems = %v", problems)
	}
}
