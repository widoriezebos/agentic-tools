package goal

import "testing"

func TestIsLedgerFile(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		path string
		want bool
	}{
		{"plans/goals/backlog.md", true},
		{"plans/goals/goal-a.md", true},
		{"records/goals/goal-a.md", true},
		{"records/goals/archive/goal-a.md", true},
		{"plans/goals.md", false},
		{"plans/goals-other/goal-a.md", false},
		{"records/goals-other/goal-a.md", false},
		{"plans/designs/goal-a.md", false},
		{"records/reads/goal-a/read.json", false},
		{"metasystem/plans/goals/goal-a.md", false},
		{"code.go", false},
		{"", false},
	} {
		if got := IsLedgerFile(test.path); got != test.want {
			t.Errorf("IsLedgerFile(%q) = %v, want %v", test.path, got, test.want)
		}
	}
}
