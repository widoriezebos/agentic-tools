package direct

import (
	"os"
	"testing"
)

// A test file is out of the audit's scope.
func TestSync(t *testing.T) {
	if err := os.Stdout.Sync(); err != nil {
		t.Log(err)
	}
}
