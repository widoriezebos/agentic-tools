package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

func TestBuildRetainsEngineAndHostAdmission(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, paths, refusal string
		load                 float64
	}{
		{name: "stale engine", paths: "internal/changed.go\n", refusal: "BUILD_ENGINE_STALE"},
		{name: "busy host", load: 9, refusal: "host.load-max"},
		{name: "fresh engine and idle host"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			if err := os.MkdirAll(filepath.Join(bed.root(), "cmd", "metasystem"), 0700); err != nil {
				t.Fatal(err)
			}
			bed.manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
				return hostload.Sample{At: at.Format(time.RFC3339Nano), Available: true, Load1m: row.load}
			}
			owners := bed.workOwners()
			owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent process")
			}
			owners.work.engineStamp = "0123456789abcdef0123456789abcdef01234567"
			git := owners.work.git
			owners.work.git = func(root string, args ...string) ([]byte, error) {
				if len(args) > 0 && args[0] == "log" {
					return []byte(row.paths), nil
				}
				if slices.Equal(args, []string{"rev-parse", "origin/main"}) {
					return []byte("main-tip\n"), nil
				}
				return git(root, args...)
			}
			brief := bed.brief("admission.md", "Build this unit.\n")
			code, result := fleetWorkJSON(t, bed, owners, "work", "build", bed.id, "admission", "--brief", brief, "--lines", "5")
			if row.refusal == "" {
				if code != 0 || len(bed.starter.launched()) == 0 {
					t.Fatalf("admitted build did not start: %d %+v", code, result)
				}
			} else if code != 1 || result.Outcome != intentRefused || !strings.Contains(jsonText(result), row.refusal) || len(bed.starter.launched()) != 0 {
				t.Fatalf("missing %s refusal before launch: %d %+v", row.refusal, code, result)
			}
		})
	}
}

func TestWorkBedRetainsClaimCapabilityAndClock(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	file, _ := bed.acceptedGoal()
	claim := file.Claimed
	capability := file.StopCapability
	if claim == nil || capability == nil || capability.Generation != claim.Revision || capability.Revision != claim.Revision ||
		capability.Machine != claim.Machine || capability.ClaimEpoch != 1 {
		t.Fatalf("work fixture lost its claimed execution authority: claim=%+v capability=%+v", claim, capability)
	}
	claimedAt, err := time.Parse(time.RFC3339, claim.At)
	if err != nil || !claimedAt.Equal(time.Date(2026, 9, 1, 9, 55, 0, 0, time.UTC)) {
		t.Fatalf("claim does not share the work clock: %q %v", claim.At, err)
	}
	for _, row := range []struct {
		name, at string
		offset   time.Duration
	}{
		{"opened", file.OpenedAt, -5 * time.Minute},
		{"approved", file.Approved.At, time.Minute},
		{"open history", file.History[0].At, -5 * time.Minute},
		{"claim history", file.History[1].At, 0},
		{"approval history", file.History[2].At, time.Minute},
	} {
		at, err := time.Parse(time.RFC3339, row.at)
		if err != nil || at.Sub(claimedAt) != row.offset {
			t.Fatalf("%s time lost its relation to the claim: %q %v", row.name, row.at, err)
		}
	}
}
