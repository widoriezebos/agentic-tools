package laneprobe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }

// The lane's leases are those taken for its installation, plus any whose
// record can't be read; another installation's leases and an older
// engine's are not the lane's.
func TestLaneLeasesAreTheInstallationsAndTheUnreadable(t *testing.T) {
	t.Parallel()
	installation := filepath.Join(t.TempDir(), "lane", "metasystem")
	reports := []proofrun.HostLeaseReport{
		{Lease: "lease-heavy-lane", Owner: proofrun.ProcessIdentity{Pid: 5}, Conf: filepath.Join(installation, "metasystem.conf"), State: proofrun.HostLeaseLive},
		{Lease: "lease-heavy-seat", Owner: proofrun.ProcessIdentity{Pid: 6}, Conf: "/seat/metasystem/metasystem.conf", State: proofrun.HostLeaseLive},
		{Lease: "lease-heavy-older", Owner: proofrun.ProcessIdentity{Pid: 7}, State: proofrun.HostLeaseLive},
		{Lease: "lease-heavy-torn", State: proofrun.HostLeaseUnknown, Reason: "unreadable"},
	}
	leases, err := laneLeases(installation, func(string) ([]proofrun.HostLeaseReport, error) { return reports, nil })
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, lease := range leases {
		names = append(names, lease.Name)
	}
	if strings.Join(names, ",") != "lease-heavy-lane,lease-heavy-torn" {
		t.Fatalf("lane leases = %q", names)
	}
}

// A lane whose installation is not known reads its leases as an error:
// the filter fails closed instead of reading every lease as another's.
func TestUnknownInstallationFailsClosed(t *testing.T) {
	t.Parallel()
	called := false
	inspect := func(string) ([]proofrun.HostLeaseReport, error) { called = true; return nil, nil }
	for _, installation := range []string{"", "relative/metasystem"} {
		if _, err := laneLeases(installation, inspect); err == nil {
			t.Fatalf("installation %q read its leases", installation)
		}
	}
	if called {
		t.Fatal("an unknown installation's leases were inspected")
	}
}
