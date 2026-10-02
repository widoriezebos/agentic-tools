package testexec

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// TestStartReadyReturnsAfterTheChildsImageRuns: StartReady returns only once
// the child's own running image reported on descriptor 3, so its argv is
// the new image's when the caller reads it, and the descriptor is closed in
// the parent and in the child before anything the child starts later.
func TestStartReadyReturnsAfterTheChildsImageRuns(t *testing.T) {
	t.Parallel()
	tag := "testexec-start-ready-tag"
	command := exec.Command("/bin/sh", "-c", ReadyPrologue+`read -r _`, tag)
	hold, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := StartReady(command); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hold.Close(); _ = command.Wait() })
	argv, known := identity.KernelProber{}.ReadArgv(int64(command.Process.Pid))
	if !known || !strings.Contains(strings.Join(argv, " "), tag) {
		t.Fatalf("argv after StartReady = %q (known %t); want the shell's, carrying %q", argv, known, tag)
	}
	if len(command.ExtraFiles) != 1 {
		t.Fatalf("ExtraFiles = %d, want the one readiness descriptor", len(command.ExtraFiles))
	}
	if _, err := command.ExtraFiles[0].Stat(); err == nil {
		t.Fatal("the parent still holds the readiness writer after StartReady")
	}
}

// TestStartReadyRefusesAChildThatNeverReports: a child that exits without
// reporting is an error, and StartReady has reaped it.
func TestStartReadyRefusesAChildThatNeverReports(t *testing.T) {
	t.Parallel()
	command := exec.Command("/bin/sh", "-c", "exit 0")
	err := StartReady(command)
	if err == nil || !strings.Contains(err.Error(), "did not report ready") {
		t.Fatalf("StartReady(silent child) = %v, want a readiness error", err)
	}
	if command.ProcessState == nil {
		t.Fatal("StartReady left the silent child unreaped")
	}
}

// TestStartReadyRefusesInheritedFiles: descriptor 3 is the readiness
// descriptor, so a command that already passes inherited files is refused
// before it starts.
func TestStartReadyRefusesInheritedFiles(t *testing.T) {
	t.Parallel()
	command := exec.Command("/bin/sh", "-c", ReadyPrologue)
	command.ExtraFiles = []*os.File{os.Stdin}
	if err := StartReady(command); err == nil || command.Process != nil {
		t.Fatalf("StartReady(with ExtraFiles) = %v, started=%t; want a refusal before start", err, command.Process != nil)
	}
}

// TestReportReadyWritesTheReadinessLine: a Go helper main reports through
// ReportReady, which StartReady accepts.
func TestReportReadyWritesTheReadinessLine(t *testing.T) {
	t.Parallel()
	command := exec.Command(os.Args[0], "-test.run=^TestReportReadyHelper$")
	command.Env = append(os.Environ(), "TESTEXEC_REPORT_READY_HELPER=1")
	hold, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := StartReady(command); err != nil {
		t.Fatal(err)
	}
	_ = hold.Close()
	if err := command.Wait(); err != nil {
		t.Fatalf("helper: %v", err)
	}
}

func TestReportReadyHelper(t *testing.T) {
	t.Parallel()
	if os.Getenv("TESTEXEC_REPORT_READY_HELPER") != "1" {
		return
	}
	if err := ReportReady(); err != nil {
		t.Fatal(err)
	}
	var buffer [1]byte
	_, _ = os.Stdin.Read(buffer[:])
}
