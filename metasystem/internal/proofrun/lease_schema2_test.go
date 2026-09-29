package proofrun

// U5d (engine-owns-disk-lifetimes Part B, 3.5 "Admission leases", DL2-01,
// DL3B-02): a lease record of schema 2 names its fixture owner and conf
// path, and every custodian started for it is recorded with its group
// beside it; a dirty schema-2 lease is reclaimed only when the owner and
// every recorded custodian are dead with empty groups and the owner-scoped
// fixture census finds no survivor. What cannot be read keeps the lease.

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const (
	custodianPid   = 56460
	custodianGroup = 56460
	custodianMicro = 1790530359000000
)

// writeDirtySchema2Lease writes the fixture lease as schema 2 with its
// custodians recorded beside it (nil: an empty custody record).
func writeDirtySchema2Lease(t *testing.T, directory string, custodians []leaseCustodian) string {
	t.Helper()
	owner := processIdentity(reclaimExact(reclaimOwnerPid, reclaimOwnerMicro), reclaimOwnerPgid)
	encoded, err := json.Marshal(hostLeaseRecord{Schema: 1, Owner: owner, Class: "heavy", Slot: "slot-00", Resources: []string{},
		Cleared: false, FixtureOwner: &owner, ConfPath: "/checkout/metasystem.conf"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, reclaimLeaseName)
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	var lines []byte
	for _, custodian := range custodians {
		line, err := json.Marshal(custodian)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(append(lines, line...), '\n')
	}
	if err := os.WriteFile(leaseCustodyPath(path), lines, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func recordedCustodian(t *testing.T) leaseCustodian {
	t.Helper()
	ref, err := identity.EncodeRef(reclaimExact(custodianPid, custodianMicro).Ref())
	if err != nil {
		t.Fatal(err)
	}
	return leaseCustodian{Ref: ref, Group: custodianGroup}
}

func TestASchema2LeaseIsReclaimedOnlyOnceItsCustodiansAreSettled(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		prober    reclaimProber
		liveGroup int64
		reclaimed bool
		reason    string
	}{
		"a live custodian keeps it": {prober: reclaimProber{custodianPid: {reclaimExact(custodianPid, custodianMicro), identity.Alive}},
			reason: "custodian"},
		"a live member of the custodian's group keeps it": {liveGroup: custodianGroup, reason: "custodian"},
		"a dead custodian with an empty group settles it": {reclaimed: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			directory := reclaimDirectory(t)
			path := writeDirtySchema2Lease(t, directory, []leaseCustodian{recordedCustodian(t)})
			seams := newReclaimSeams(c.prober)
			reclaimer := seams.reclaimer()
			reclaimer.groupLive = func(group int64, _ identity.Prober) (bool, error) { return group == c.liveGroup, nil }
			if _, _, err := hostLeaseStateWithReclaim(directory, true, reclaimer); err != nil {
				t.Fatal(err)
			}
			if !c.reclaimed {
				assertKept(t, directory, path)
				if len(*seams.lines) != 1 || !strings.Contains((*seams.lines)[0], c.reason) {
					t.Fatalf("the kept lease names its %s: %q", c.reason, *seams.lines)
				}
				return
			}
			for _, gone := range []string{path, leaseCustodyPath(path)} {
				if _, err := os.Stat(gone); !os.IsNotExist(err) {
					t.Fatalf("%s must go with the reclaimed lease: %v", gone, err)
				}
			}
			records := reclaimRecords(t, directory)
			if len(records) != 1 || len(records[0].Custodians) != 1 || records[0].Custodians[0].Group != custodianGroup {
				t.Fatalf("the reclaim record names the custodians it proved settled: %+v", records)
			}
			// A repeat finds nothing and writes nothing.
			if _, _, err := hostLeaseStateWithReclaim(directory, true, reclaimer); err != nil || len(reclaimRecords(t, directory)) != 1 {
				t.Fatalf("a repeat writes nothing: %v", err)
			}
		})
	}
}

// Rule 1: a custody record that cannot be read, or is missing, keeps the
// lease; the census of the fixture owner runs with the recorded owner.
func TestASchema2LeaseWhoseCustodyCannotBeReadIsKept(t *testing.T) {
	t.Parallel()
	for name, corrupt := range map[string]func(string) error{
		"torn":    func(path string) error { return os.WriteFile(leaseCustodyPath(path), []byte("{\"ref\":"), 0o600) },
		"missing": func(path string) error { return os.Remove(leaseCustodyPath(path)) },
		"unknown field": func(path string) error {
			return os.WriteFile(leaseCustodyPath(path), []byte(`{"ref":"pid=1;micro=1","group":1,"extra":true}`+"\n"), 0o600)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			directory := reclaimDirectory(t)
			path := writeDirtySchema2Lease(t, directory, nil)
			if err := corrupt(path); err != nil {
				t.Fatal(err)
			}
			seams := newReclaimSeams(reclaimProber{})
			if _, _, err := hostLeaseStateWithReclaim(directory, true, seams.reclaimer()); err != nil {
				t.Fatal(err)
			}
			assertKept(t, directory, path)
		})
	}
}

func TestAcquisitionWritesSchema2AndCustodyRecordsTheCustodian(t *testing.T) {
	t.Parallel()
	directory, conf := privateHostResources(t)
	lease, err := acquireHostResourcesIn(t.Context(), directory, directory, conf, "heavy", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	var marker *os.File
	for _, file := range lease.Files() {
		if strings.HasPrefix(filepath.Base(file.Name()), "lease-") {
			marker = file
		}
	}
	if marker == nil {
		t.Fatal("the lease has no marker")
	}
	record, _, err := readHostLeaseRecord(marker)
	if err != nil || record.Schema != 1 || record.FixtureOwner == nil || record.FixtureOwner.Pid != record.Owner.Pid || !filepath.IsAbs(record.ConfPath) {
		t.Fatalf("the lease stays schema 1 with its fixture owner and conf path: %+v %v", record, err)
	}
	if err := recordLeaseCustodian(lease.Files(), reclaimExact(custodianPid, custodianMicro).Ref(), custodianGroup); err != nil {
		t.Fatal(err)
	}
	custodians, err := readLeaseCustodians(marker.Name())
	if err != nil || len(custodians) != 1 || custodians[0].Group != custodianGroup {
		t.Fatalf("the custodian is recorded beside the lease: %+v %v", custodians, err)
	}
}

// Round D2 F-1: an engine older than this one shares the host admission
// directory and rejects any schema but 1, so a lease stays schema 1 and
// carries the fixture owner and conf path as extra fields the old reader
// ignores. baseLeaseRecord and baseValidHostLeaseRecord are the base
// engine's (fa3c41a16) reader, copied verbatim.
type baseLeaseRecord struct {
	Schema    int             `json:"schema"`
	Owner     ProcessIdentity `json:"owner"`
	Class     string          `json:"class"`
	Slot      string          `json:"slot,omitempty"`
	Resources []string        `json:"resources"`
	Cleared   bool            `json:"cleared"`
}

func baseValidHostLeaseRecord(path string, record baseLeaseRecord) error {
	name := filepath.Base(path)
	if record.Schema != 1 || record.Owner.Pid <= 0 || !record.Owner.Ref().NativeExact() ||
		(record.Class != "cheap" && record.Class != "heavy") ||
		!strings.HasPrefix(name, "lease-"+record.Class+"-") || len(strings.TrimPrefix(name, "lease-"+record.Class+"-")) != 32 {
		return fmt.Errorf("unreconciled proof resource marker %s has an invalid claim", name)
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(name, "lease-"+record.Class+"-")); err != nil {
		return fmt.Errorf("unreconciled proof resource marker %s has an invalid nonce", name)
	}
	if record.Class == "cheap" && record.Slot != "" || record.Slot != "" && !strings.HasPrefix(record.Slot, "slot-") {
		return fmt.Errorf("unreconciled proof resource marker %s has an invalid slot", name)
	}
	seen := map[string]bool{}
	for _, resource := range record.Resources {
		if seen[resource] || !strings.HasPrefix(resource, "resource-") || len(resource) != len("resource-")+64 {
			return fmt.Errorf("unreconciled proof resource marker %s has an invalid resource", name)
		}
		if _, err := hex.DecodeString(strings.TrimPrefix(resource, "resource-")); err != nil {
			return fmt.Errorf("unreconciled proof resource marker %s has an invalid resource", name)
		}
		seen[resource] = true
	}
	return nil
}

func TestAnOlderEngineReadsTheLeaseThisEngineWrites(t *testing.T) {
	t.Parallel()
	directory, conf := privateHostResources(t)
	lease, err := acquireHostResourcesIn(t.Context(), directory, directory, conf, "heavy", []string{"shared-resource"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	for _, file := range lease.Files() {
		if !strings.HasPrefix(filepath.Base(file.Name()), "lease-") {
			continue
		}
		data, err := os.ReadFile(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		var old baseLeaseRecord
		if err := json.Unmarshal(data, &old); err != nil {
			t.Fatalf("the base engine decodes the lease: %v", err)
		}
		if err := baseValidHostLeaseRecord(file.Name(), old); err != nil {
			t.Fatalf("the base engine accepts the lease this engine writes: %v", err)
		}
		// And this engine still recognizes it as the richer lease.
		record, _, err := readHostLeaseRecord(file)
		if err != nil || record.FixtureOwner == nil || record.ConfPath == "" {
			t.Fatalf("this engine reads the fixture owner and conf path: %+v %v", record, err)
		}
	}
}

// A richer lease must carry both extra fields, well formed: a fixture owner
// without a conf path, or a relative conf path, is unreadable (rule 1).
func TestAHalfRicherLeaseIsUnreadable(t *testing.T) {
	t.Parallel()
	owner := processIdentity(reclaimExact(reclaimOwnerPid, reclaimOwnerMicro), reclaimOwnerPgid)
	path := filepath.Join(t.TempDir(), reclaimLeaseName)
	for name, record := range map[string]hostLeaseRecord{
		"a fixture owner without a conf path": {Schema: 1, Owner: owner, Class: "heavy", FixtureOwner: &owner},
		"a relative conf path":                {Schema: 1, Owner: owner, Class: "heavy", FixtureOwner: &owner, ConfPath: "metasystem.conf"},
		"schema 2":                            {Schema: 2, Owner: owner, Class: "heavy", FixtureOwner: &owner, ConfPath: "/c/metasystem.conf"},
	} {
		if err := validHostLeaseRecord(path, record); err == nil {
			t.Fatalf("%s is refused", name)
		}
	}
}

// An older engine that reclaims a cleared richer lease removes the lease
// alone; this engine's next reclaiming scan, under admission.lock, drops the
// custody record no lease names any more, and never one whose lease exists.
func TestAnOrphanCustodyRecordGoesWithTheNextReclaimingScan(t *testing.T) {
	t.Parallel()
	directory := reclaimDirectory(t)
	live := writeDirtySchema2Lease(t, directory, []leaseCustodian{recordedCustodian(t)})
	orphan := filepath.Join(directory, leaseCustodyPrefix+"lease-heavy-11111111111111111111111111111111.jsonl")
	if err := os.WriteFile(orphan, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	seams := newReclaimSeams(reclaimProber{custodianPid: {reclaimExact(custodianPid, custodianMicro), identity.Alive}})
	if _, _, err := hostLeaseStateWithReclaim(directory, true, seams.reclaimer()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("the orphan custody record goes: %v", err)
	}
	if _, err := os.Stat(leaseCustodyPath(live)); err != nil {
		t.Fatalf("a live lease keeps its custody record: %v", err)
	}
}
