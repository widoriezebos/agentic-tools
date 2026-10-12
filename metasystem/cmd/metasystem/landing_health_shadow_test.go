package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingHealthShadowDriftCLI(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"text", "json"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			b := newPlainVerbBed(t)
			if _, err := plain.RefreshShadow(b.home, laneTestNow, "fixture", b.owners.landing.plainProve); err != nil {
				t.Fatal(err)
			}
			if _, err := lane.SetPause(b.home, "the person", laneTestNow.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			words := "lane running, agent idle since 20:33"
			if err := plain.RecordIntervention(b.installation, plain.Intervention{At: laneTestNow.Format(time.RFC3339Nano), Row: "silent-agent", Shadow: words}); err != nil {
				t.Fatal(err)
			}
			engine := os.Getenv("METASYSTEM_WAIT_BINARY")
			if engine == "" {
				t.Fatal("the package did not build the CLI")
			}
			args := []string{"landing", "health"}
			if format == "json" {
				args = append(args, "--json")
			}
			cmd := exec.CommandContext(t.Context(), engine, args...)
			cmd.Dir = b.checkout
			cmd.Env = fixtureCommandEnvironment(t, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("health: %v %s", err, output)
			}
			for _, part := range []string{"drift", "shadow: lane running", "records: paused since " + laneTestNow.Add(time.Minute).Local().Format("15:04") + " by the person"} {
				if !strings.Contains(string(output), part) {
					t.Fatalf("missing %q: %s", part, output)
				}
			}
			if format == "json" {
				var result struct{ Data plain.Health }
				if err := json.Unmarshal(output, &result); err != nil {
					t.Fatal(err)
				}
				if len(result.Data.Drift) != 1 || len(result.Data.Interventions) != 1 || result.Data.Interventions[0].Shadow != words {
					t.Fatalf("JSON lost rows or intervention evidence: %+v", result.Data)
				}
			}
		})
	}
}
