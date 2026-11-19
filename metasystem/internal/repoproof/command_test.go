package repoproof

import (
	"bytes"
	"os"
	"os/exec"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func TestExecuteKeepsDeclaredArgumentZero(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := Execute([]string{"sh", "-c", `printf '%s' "$0"`}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "sh" {
		t.Fatalf("declared argument zero: %q", output.String())
	}
}

func TestExecuteStripsNativeHistoryRoot(t *testing.T) {
	t.Parallel()
	if !slices.Contains(os.Args, "--history-child") {
		child := exec.Command(os.Args[0], "-test.run=^TestExecuteStripsNativeHistoryRoot$", "--", "--history-child")
		child.Env = append(gittree.ScrubbedEnviron(), proofrun.NativeHistoryRootEnvironment+"=/fixture-installation")
		if err := child.Run(); err != nil {
			t.Fatal("reporter environment probe failed")
		}
		return
	}
	if _, present := os.LookupEnv(proofrun.NativeHistoryRootEnvironment); !present {
		t.Fatal("probe did not receive history hint")
	}
	var output bytes.Buffer
	if err := Execute([]string{"sh", "-c", `test -z "${METASYSTEM_NATIVE_HISTORY_ROOT+x}"`}, &output, &output); err != nil {
		t.Fatal("reporter child inherited installation history hint")
	}
}
