package lane

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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
	release, err := HoldProving(home)
	if err != nil {
		fmt.Println("not held", err)
		return 1
	}
	// The release stays referenced: a collected lock file would close and
	// drop the flock.
	defer release()
	fmt.Println("held")
	// It waits for a signal, never on the wall clock: a registered signal
	// channel keeps the runtime from reading the wait as a deadlock, and the
	// tests end the holder with SIGKILL, which no process can catch.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop
	return 0
}

func itoa(n int) string { return strconv.Itoa(n) }
