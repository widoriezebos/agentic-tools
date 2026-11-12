package launch

import (
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestUnitCheckPassesTheSelectionPath(t *testing.T) {
	t.Parallel()
	check := UnitCheck{Directory: t.TempDir(), Cheap: `printf '%s' "$METASYSTEM_CHECK_SELECTION"`, Audits: `printf '%s' "$METASYSTEM_CHECK_SELECTION"`, Minutes: 1,
		Environment: []string{"METASYSTEM_CHECK_SELECTION=inherited"}}
	var attached []string
	execution, exits, err := check.Run(check.Directory, t.TempDir(), CheckContext{
		Now:    func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
		After:  func(time.Duration) <-chan time.Time { return nil },
		Attach: func(process *exec.Cmd) { attached = append(attached, strings.Join(process.Env, "\n")) },
	})
	want := filepath.Join(execution, "selection.json")
	if err != nil || len(exits) != 2 || exits[0].Exit != 0 || exits[0].Output != want || exits[1].Output != "inherited" {
		t.Fatalf("selection path beside result.json: execution=%s exits=%+v error=%v", execution, exits, err)
	}
	if len(attached) != 2 || !strings.Contains(attached[0], "METASYSTEM_CHECK_SELECTION="+want) || !slices.Equal(check.Environment, []string{"METASYSTEM_CHECK_SELECTION=inherited"}) {
		t.Fatalf("attached environments=%v frozen environment=%v", attached, check.Environment)
	}
}
