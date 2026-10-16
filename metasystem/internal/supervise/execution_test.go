package supervise

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin/testdata/pinfixture"
)

func TestSupervisionExecutionsReadEachOwningRecord(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	prober := pinfixture.Prober{}
	want := map[string]string{}
	document := stateDocument{Generation: 999, EngineBuild: "untrusted-record-source", Components: map[string]stateComponent{}}
	for i, role := range []string{"owner", "watcher", "reaper"} {
		path, digest := pinfixture.Pin(t, root, 147+i, role+"-source")
		exact := pinfixture.Process(int64(41+i), path)
		prober[exact.Pid], want[role] = exact, digest
		if role == "owner" {
			if err := os.MkdirAll(filepath.Dir(ownerPath(root)), 0o700); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(ArmingOwner{Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), InstanceTag: role})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ownerPath(root), data, 0o600); err != nil {
				t.Fatal(err)
			}
		} else {
			document.Components[role] = stateComponent{Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), InstanceTag: role}
		}
	}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(SupervisionDir(root), "state.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	got := ReadExecutions(root, prober)
	if len(got) != 3 {
		t.Fatalf("roles: %+v", got)
	}
	for i, role := range []string{"owner", "watcher", "reaper"} {
		if got[role].Digest != want[role] || got[role].Generation != 147+i || got[role].Source != role+"-source" {
			t.Fatalf("%s: %+v", role, got[role])
		}
	}
	delete(prober, 42)
	if got := ReadExecutions(root, prober); got["watcher"].Digest != "" || got["watcher"].Cause == "" || got["reaper"].Digest != want["reaper"] {
		t.Fatalf("one dead role: %+v", got)
	}
}

func TestSupervisionExecutionsKeepUnknownRoles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, malformed := range []bool{false, true} {
		if malformed {
			if err := os.MkdirAll(SupervisionDir(root), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(SupervisionDir(root), "state.json"), []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		got := ReadExecutions(root, pinfixture.Prober{})
		if len(got) != 3 {
			t.Fatalf("missing roles: %+v", got)
		}
		for role, execution := range got {
			if execution.Digest != "" || execution.Cause == "" {
				t.Fatalf("%s: %+v", role, execution)
			}
		}
	}
}
