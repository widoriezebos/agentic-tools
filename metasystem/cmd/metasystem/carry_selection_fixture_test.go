package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// The child runs the impact producer and command groups; only Git's repository
// observations are supplied by the fixture.
func init() {
	testHelperCommands["fixture-carry-impact"] = func([]string) int {
		base := os.Getenv("LANDING_PROOF_BASE")
		sources := impactSources{environment: os.Environ(), getenv: os.Getenv, get: config.Get,
			git: func(_ string, args ...string) (string, error) {
				switch strings.Join(args, " ") {
				case "rev-parse --verify " + base + "^{commit}":
					return base, nil
				case "rev-parse " + base + "^{tree}":
					return "base-tree", nil
				case "merge-base --is-ancestor " + base + " HEAD":
					return "", nil
				case "show -s --format=%s " + base:
					return "carry base", nil
				}
				return "", fmt.Errorf("unexpected carry producer Git request: %v", args)
			},
			impact: func(string, string, bool) (goadapter.Impact, error) { return goadapter.Impact{}, nil },
		}
		return sources.run([]string{"--root", ".", "--base", base}, os.Stdout, os.Stderr, "")
	}
}

func installCarrySelectionProducer(t *testing.T, root string) {
	t.Helper()
	fixture := impactAdapterBed(t)
	for _, name := range []string{"testing.json", "go.mod"} {
		impactWrite(t, root, name, string(mustRead(t, filepath.Join(fixture, name))))
	}
	impactWrite(t, root, "metasystem.conf", "testing.contract=testing.json\n")
}

func carrySelectionProducerCommand(t *testing.T) string {
	t.Helper()
	return "GO_WANT_BATCH_E2E_COMMAND=1 " + shellCommand([]string{commandTestExecutable(t), "fixture-carry-impact"}) + " > /dev/null"
}

func assertCarrySelectionEvidence(t *testing.T, result string, complete, audited bool) {
	t.Helper()
	var execution struct{ Check launch.UnitCheck }
	helmMust(t, json.Unmarshal(mustRead(t, result), &execution))
	var evidence launch.CheckEvidence
	helmMust(t, json.Unmarshal(mustRead(t, filepath.Join(filepath.Dir(result), "evidence.json")), &evidence))
	if evidence.Strength.Complete != complete || evidence.Strength.Audited != audited {
		t.Fatalf("carry evidence strength: %+v", evidence)
	}
	if complete && (evidence.Selection.Base != execution.Check.Base || evidence.Selection.EnvironmentSHA256 != launch.CheckEnvironmentDigest(execution.Check.Environment, execution.Check.Base)) {
		t.Fatalf("carry selection lost its base or environment: %+v", evidence.Selection)
	}
}
