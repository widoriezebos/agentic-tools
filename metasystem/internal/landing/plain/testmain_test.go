package plain

import (
	"fmt"
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
)

func TestMain(m *testing.M) {
	if install := os.Getenv(proveChildEnv); install != "" {
		os.Exit(runProveChild(install, os.Getenv(proveChildCommand), os.Args[1:]))
	}
	os.Exit(testenv.Main(m))
}

func runProveChild(install, command string, args []string) int {
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
