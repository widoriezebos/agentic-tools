package proofrun

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestNamedCommandGroupReportsLiveProgress(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	group := testpolicy.Group{ID: "command", Adapter: "command", CWD: ".", Argv: []string{"/bin/sh", "-c", "touch ran"}, Format: "exit-status"}
	var events []string
	results, err := RunNamedGroups(context.Background(), root, testpolicy.Contract{SchemaVersion: 2, Groups: []testpolicy.Group{group}}, []string{"command"}, []string{"PATH=/usr/bin:/bin"}, func(id string, planned int, completed []PackageExecution) {
		_, fileErr := os.Stat(filepath.Join(root, "ran"))
		if planned > 0 {
			events = append(events, "planned")
			if planned != 1 || !os.IsNotExist(fileErr) {
				t.Errorf("plan was not live: planned=%d file=%v", planned, fileErr)
			}
		}
		for _, execution := range completed {
			events = append(events, "completed")
			if id != "command" || execution.Package != "command" || execution.Status != "ok" || execution.ElapsedMS == nil || fileErr != nil {
				t.Errorf("invalid completion: id=%s %+v file=%v", id, execution, fileErr)
			}
		}
	})
	if err != nil || len(results) != 1 || results[0].Status != "green" || !reflect.DeepEqual(events, []string{"planned", "completed"}) {
		t.Fatalf("results=%+v events=%v err=%v", results, events, err)
	}
}
