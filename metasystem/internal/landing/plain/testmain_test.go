package plain

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// proveChildEnv makes this test binary the detached proof child Start
// launches: it runs Run for the attempt in its argv, as the engine's
// landing prove --wait --attempt ID does.
const (
	proveChildEnv     = "PLAIN_TEST_PROVE_CHILD_INSTALL"
	proveChildCommand = "PLAIN_TEST_PROVE_CHILD_COMMAND"
	// proveChildExited names a FIFO the child holds open for writing from
	// its start to its exit (waitForChildExit).
	proveChildExited = "PLAIN_TEST_PROVE_CHILD_EXITED"
)

func TestMain(m *testing.M) {
	if install := os.Getenv(proveChildEnv); install != "" {
		os.Exit(runProveChild(install, os.Getenv(proveChildCommand), os.Args[1:]))
	}
	os.Exit(testenv.Main(m))
}

func runProveChild(install, command string, args []string) int {
	if fifo := os.Getenv(proveChildExited); fifo != "" {
		// Kept open until this process exits; the test's reader sees the
		// end of the FIFO then.
		held, err := os.OpenFile(fifo, os.O_WRONLY, 0)
		if err != nil {
			fmt.Println(err)
			return 1
		}
		defer held.Close()
	}
	attempt := ""
	for index, arg := range args {
		if arg == "--attempt" && index+1 < len(args) {
			attempt = args[index+1]
		}
	}
	checkout, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if _, err := Run(install, checkout, command, attempt, os.Stdout, ProveSeams{}); err != nil {
		fmt.Println(err)
		return 1
	}
	return 0
}

// waitForChildExit opens the exit FIFO, which lets a child blocked opening
// it run, and reads it to its end: the child's exit, the only writer.
func waitForChildExit(t *testing.T, fifo string) {
	t.Helper()
	reader, err := os.Open(fifo)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatal(err)
	}
}
