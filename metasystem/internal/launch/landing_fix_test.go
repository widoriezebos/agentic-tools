package launch

import "testing"

func TestLaneFixBuildAndReadPassLauncherAdmission(t *testing.T) {
	t.Parallel()
	checkout, module := nestedLane(t)
	manager, _ := laneManager(t, checkout, module)
	for _, kind := range []string{"build", "read"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			if err := manager.admitOnLane(StartSpec{Kind: kind, WorkingDirectory: checkout, FenceRoot: module, Goal: "goal-a"}); err != nil {
				t.Fatalf("lane %s refused: %v", kind, err)
			}
		})
	}
}
