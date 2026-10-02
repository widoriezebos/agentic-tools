package launch

import (
	"os/exec"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The kernel scanner lists the processes of the table it is given and no
// other: a table of the test's own process yields exactly that process,
// and an empty one nothing, whatever runs on the host.
func TestKernelProcessScannerReadsOnlyItsTable(t *testing.T) {
	t.Parallel()
	child := exec.Command("/bin/sh", "-c", testexec.ReadyPrologue+"read -r _")
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := testexec.StartReady(child); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close(); _ = child.Wait() })
	pid := int64(child.Process.Pid)
	// StartReady returns once the child's own image runs, so its argv is
	// published: no wait for it to become readable.
	scanned, err := KernelProcessScanner{Prober: identity.KernelProber{}, Table: identity.ListedProcessTable{pid}}.Scan()
	if err != nil || len(scanned) != 1 || scanned[0].Ref.Pid != pid || len(scanned[0].Argv) == 0 {
		t.Fatalf("scan of a table of the child = %+v, %v; want exactly the child", scanned, err)
	}
	if scanned, err := (KernelProcessScanner{Prober: identity.KernelProber{}, Table: identity.ListedProcessTable{}}).Scan(); err != nil || len(scanned) != 0 {
		t.Fatalf("scan of an empty table = %+v, %v; want nothing", scanned, err)
	}
}
