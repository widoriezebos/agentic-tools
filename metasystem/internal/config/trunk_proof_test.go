package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProofTrunkIntervalDefaultAndValidation(t *testing.T) {
	t.Parallel()
	noEnv := func(string) (string, bool) { return "", false }
	value, _, err := Get(GetParams{Key: "proof.trunk-every", LookupEnv: noEnv})
	if err != nil || value != "4h" {
		t.Fatalf("default interval=%q err=%v", value, err)
	}
	for _, value := range []string{"30m", "4h", "0h", "-1h", "bad"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			conf := filepath.Join(root, "metasystem.conf")
			if err := os.WriteFile(conf, []byte("proof.full=true\nproof.cheap=true\nproof.trunk-every="+value+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, problems, err := validateWithRunner(conf, root, func(gitRequest) ([]byte, error) { return nil, fmt.Errorf("synthetic checkout") })
			if err != nil {
				t.Fatal(err)
			}
			invalid := value == "0h" || value == "-1h" || value == "bad"
			found := false
			for _, problem := range problems {
				found = found || strings.Contains(problem, "proof.trunk-every must be a positive duration")
			}
			if found != invalid {
				t.Fatalf("interval=%q problems=%v", value, problems)
			}
		})
	}
}
