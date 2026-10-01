package lane

import (
	"os"
	"strconv"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) {
	os.Exit(testenv.Main(m))
}

func itoa(n int) string { return strconv.Itoa(n) }
