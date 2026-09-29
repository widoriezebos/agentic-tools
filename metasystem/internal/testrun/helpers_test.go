package testrun

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }

// streamBuffer is one stream of a run, safe for concurrent writes.
type streamBuffer struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *streamBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(p)
}

func (b *streamBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.String()
}

// runOnOwnStreams runs run on writers of its own and returns its status and
// what it printed on each.
func runOnOwnStreams(run func(stdout, stderr io.Writer) int) (int, string, string) {
	var stdout, stderr streamBuffer
	code := run(&stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func writeTestingFixtureFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func testingFixtureGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
