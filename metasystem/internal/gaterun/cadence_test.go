package gaterun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCadencePackagesUseNoWallClock(t *testing.T) {
	paths, err := filepath.Glob("cadence*.go")
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths,
		filepath.Join("..", "proofrun", "reuse_policy.go"),
		filepath.Join("..", "cadence", "tick.go"),
	)
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"time.Now(", "time.Sleep(", "time.After("} {
			if strings.Contains(string(data), forbidden) {
				t.Errorf("%s uses %s", path, forbidden)
			}
		}
	}
}
