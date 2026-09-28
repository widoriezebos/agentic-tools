package steward

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func proofAdmissionReports(reports ...proofrun.HostLeaseReport) func(string) ([]proofrun.HostLeaseReport, error) {
	return func(string) ([]proofrun.HostLeaseReport, error) { return reports, nil }
}

func TestCheckProofAdmissionTurnsRedOnlyForOldDeadOrUnknownLeases(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoRoot, "metasystem.conf"), []byte("metasystem.version=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 28, 15, 0, 0, 0, time.UTC)
	lease := func(state string, age time.Duration, remedy string) proofrun.HostLeaseReport {
		return proofrun.HostLeaseReport{
			Lease: "lease-heavy-0a9b107b6b73a7cf070269f147e8a26d", Owner: proofrun.ProcessIdentity{Pid: 56449, Pgid: 56440},
			Since: now.Add(-age), State: state, Reason: "why", Remedy: remedy,
		}
	}
	cases := []struct {
		name   string
		report []proofrun.HostLeaseReport
		status HealthStatus
		reason string
		remedy string
	}{
		{"none", nil, HealthAlive, "no dirty heavy proof lease", ""},
		{"live for an hour", []proofrun.HostLeaseReport{lease(proofrun.HostLeaseLive, time.Hour, "")}, HealthAlive, "owner pid 56449 group 56440 age 1h0m0s live", ""},
		{"reclaimed", []proofrun.HostLeaseReport{lease(proofrun.HostLeaseReclaimed, time.Hour, "")}, HealthAlive, "reclaimed", ""},
		{"dead inside the bound", []proofrun.HostLeaseReport{lease(proofrun.HostLeaseDead, 9*time.Minute, "")}, HealthAlive, "reclaimable-dead", ""},
		{"dead past the bound", []proofrun.HostLeaseReport{lease(proofrun.HostLeaseDead, 11*time.Minute, "")}, HealthDead, "over 10 min", "next admission pass"},
		{"unknown inside the bound", []proofrun.HostLeaseReport{lease(proofrun.HostLeaseUnknown, time.Minute, "run the settling command")}, HealthAlive, "unknown", "run the settling command"},
		{"unknown past the bound", []proofrun.HostLeaseReport{lease(proofrun.HostLeaseUnknown, 11*time.Minute, "run the settling command")}, HealthDead, "lease-heavy-0a9b107b6b73a7cf070269f147e8a26d", "run the settling command"},
	}
	for _, test := range cases {
		role := checkProofAdmission(repoRoot, now, proofAdmissionReports(test.report...))
		if role.Role != RoleProofAdmission || role.Status != test.status || !strings.Contains(role.Reason, test.reason) || !strings.Contains(role.Remedy, test.remedy) {
			t.Fatalf("%s: %+v", test.name, role)
		}
	}
	if role := checkProofAdmission(repoRoot, now, func(string) ([]proofrun.HostLeaseReport, error) { return nil, errors.New("denied") }); role.Status != HealthUnknown {
		t.Fatalf("unreadable leases = %+v", role)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "metasystem.conf"), []byte(proofAdmissionRedKey+"=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if role := checkProofAdmission(repoRoot, now, proofAdmissionReports(lease(proofrun.HostLeaseDead, 2*time.Minute, ""))); role.Status != HealthDead {
		t.Fatalf("configured one-minute bound = %+v", role)
	}
}
