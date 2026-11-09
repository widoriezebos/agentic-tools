package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProofTrunkIntervalDefaultAndValidation(t *testing.T) {
	t.Parallel()
	noEnv := func(string) (string, bool) { return "", false }
	value, _, err := Get(GetParams{Key: "proof.trunk-every", LookupEnv: noEnv})
	if err != nil || value != "off" {
		t.Fatalf("default interval=%q err=%v", value, err)
	}
	for _, value := range []string{"off", "24h", "48h", "30m", "4h", "23h59m", "0h", "-1h", "bad"} {
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
			invalid := value != "off" && value != "24h" && value != "48h"
			found := false
			for _, problem := range problems {
				found = found || strings.Contains(problem, "proof.trunk-every must be off or a duration of at least 24h")
			}
			if found != invalid {
				t.Fatalf("interval=%q problems=%v", value, problems)
			}
		})
	}
}

func TestTrunkEvery(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw   string
		every time.Duration
		valid bool
	}{
		{"off", 0, true}, {"24h", 24 * time.Hour, true}, {"48h", 48 * time.Hour, true},
		{"", 0, false}, {"30m", 0, false}, {"4h", 0, false}, {"23h59m", 0, false}, {"0h", 0, false}, {"-1h", 0, false}, {"bad", 0, false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			every, valid := TrunkEvery(tc.raw)
			if every != tc.every || valid != tc.valid {
				t.Fatalf("TrunkEvery(%q)=(%s,%v), want (%s,%v)", tc.raw, every, valid, tc.every, tc.valid)
			}
		})
	}
}
