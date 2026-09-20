package main

import (
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGLEExplicitToolReadinessUsesExecutedEnvironment(t *testing.T) {
	t.Parallel()
	group := testpolicy.Group{ID: "portable", CWD: ".", EnvironmentMode: "explicit",
		Tools: []testpolicy.Tool{{ID: "shell", Executable: "sh"}}}
	contract := testpolicy.Contract{Groups: []testpolicy.Group{group}}
	if err := checkTestingTools(t.TempDir(), contract); err == nil || !strings.Contains(err.Error(), "portable:shell") {
		t.Fatalf("ambient PATH supplied an undeclared explicit tool: %v", err)
	}
	contract.Groups[0].Env = map[string]string{"PATH": os.Getenv("PATH")}
	if err := checkTestingTools(t.TempDir(), contract); err != nil {
		t.Fatalf("declared PATH did not resolve tool: %v", err)
	}
}
