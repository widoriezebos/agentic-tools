package batch

import (
	"fmt"
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

func TestMain(m *testing.M) {
	if os.Getenv("METASYSTEM_CONTRACT_DRIVER_HELPER") == "1" {
		if len(os.Args) != 6 || os.Args[1] != "testing" || os.Args[2] != "merge-driver" {
			os.Exit(2)
		}
		base, baseErr := os.ReadFile(os.Args[3])
		ours, oursErr := os.ReadFile(os.Args[4])
		theirs, theirsErr := os.ReadFile(os.Args[5])
		merged, err := contractmerge.MergeBytes(base, ours, theirs)
		if baseErr != nil || oursErr != nil || theirsErr != nil || err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := os.WriteFile(os.Args[4], merged, 0o644); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(testenv.Main(m))
}
