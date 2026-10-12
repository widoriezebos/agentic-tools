package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingUnselectableRedStillRecordsMainIncident(t *testing.T) {
	t.Parallel()
	for _, trunk := range []bool{true, false} {
		for _, names := range []string{"TestP(did not report)", "TestP/subtest", "TestMain", "TestValid TestP(did not report)"} {
			t.Run(fmt.Sprintf("trunk=%t/%s", trunk, names), func(t *testing.T) {
				t.Parallel()
				b, register, _ := incidentProveFixture(t, false)
				before := register.repo.canonical
				register.repo.commits["main"] = register.repo.commit(before)
				unit := "metasystem/internal/p"
				whole := 0
				b.fail = func(cmd *exec.Cmd, only string) (string, error) {
					if freshProofEnv(cmd, "LANDING_ONLY_TESTS") != "" {
						// The reporter drops an identity absent from the tree and reports green.
						return "LANDING-CHECKED\t0\n", nil
					}
					if only != "" && only != unit {
						t.Fatalf("wrong whole-unit selector: %q", only)
					}
					whole++
					// A package can end red without any completed failing test event.
					return mainRepeatEvents(unit, nil, "fail") + fmt.Sprintf("LANDING-FAILED\t%s\t%s\nLANDING-CHECKED\t1\n", unit, names), errors.New("persistent package red")
				}
				args := []string{"prove", "--wait", "--json"}
				if trunk {
					args = append(args, "--trunk")
				}
				code, out := b.run(t, b.root, args...)
				var response struct{ Data plain.Result }
				helmMust(t, json.Unmarshal([]byte(out), &response))
				check := response.Data
				entries, problems := goal.ParseTrunkRed(register.repo.commit(register.repo.canonical).files["plans/goals/trunk-red.json"])
				wantWhole := 2
				if !trunk {
					wantWhole = 3 // Original batch, isolated main, and main's repeat.
				}
				if code != 1 || check.Result != plain.Red || check.Cause == nil || check.Cause.Kind != "main" || whole != wantWhole || len(b.runs) != wantWhole || len(entries) != len(strings.Fields(names)) || len(problems) != 0 || register.repo.canonical == before {
					t.Fatalf("red lost its incident: code=%d cause=%+v reason=%q whole=%d runs=%v incidents=%d problems=%v", code, check.Cause, check.Reason, whole, b.runs, len(entries), problems)
				}
				for _, entry := range entries {
					if entry.Class == goal.TrunkRedClassFlake || !strings.HasPrefix(entry.Identity, "red:"+unit+":") {
						t.Fatalf("incident lost its failing identity: %+v", entry)
					}
				}
			})
		}
	}
}
