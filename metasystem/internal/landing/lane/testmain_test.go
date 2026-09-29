package lane

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// provingHolderEnv makes this test binary a child that takes the proving
// flock under the named home, says so, and holds it until it is killed.
const provingHolderEnv = "LANE_TEST_PROVING_HOLDER_HOME"

func TestMain(m *testing.M) {
	if home := os.Getenv(provingHolderEnv); home != "" {
		os.Exit(runProvingHolder(home))
	}
	os.Exit(testenv.Main(m))
}

func runProvingHolder(home string) int {
	release, holder, err := TryProving(home)
	if err != nil || release == nil {
		fmt.Println("busy", holder, err)
		return 1
	}
	fmt.Println("held")
	select {}
}

func itoa(n int) string { return strconv.Itoa(n) }
