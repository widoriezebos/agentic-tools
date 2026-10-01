package laneengine

import (
	"os"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

var fixedNow = time.Date(2026, 9, 30, 18, 0, 0, 0, time.UTC)

func TestMain(m *testing.M) {
	os.Exit(testenv.Main(m))
}
