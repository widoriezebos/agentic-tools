// Package startfixture supplies isolated enrollment and execution records.
package startfixture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin/testdata/pinfixture"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func JSON(t testing.TB, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func Register(t testing.TB, home, root string) {
	t.Helper()
	_, _, err := lane.Register(home, lane.Layout{Checkout: lane.CheckoutRoot(root), Install: lane.InstallRoot(root)}, "Wido", time.Unix(1800000000, 0))
	if err != nil {
		t.Fatal(err)
	}
}

func Stage(t testing.TB, root string) (string, string) {
	t.Helper()
	path, digest := pinfixture.Pin(t, root, 148, "current")
	if err := steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{RepoIdentity: root, Generation: 148, InstallPath: path, InstallDigest: digest, MintedAt: "2026-10-17T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=\nwatch.interval-sec=1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(binary), 0700); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(binary, data, 0700); err != nil {
		t.Fatal(err)
	}
	return path, digest
}

// Executions publishes small process records and reads real pin bytes with a fixture prober.
func Executions(t testing.TB, root, current string, changes map[string]string) func(string) map[string]enginepin.Execution {
	t.Helper()
	prober := pinfixture.Prober{}
	owner := supervise.ArmingOwner{}
	held := []supervise.Held{}
	for i, role := range []string{"owner", "watcher", "reaper", "runner"} {
		path := current
		if changes[role] == "obsolete" {
			path, _ = pinfixture.Pin(t, root, 147, "obsolete")
		}
		exact := pinfixture.Process(int64(41+i), path)
		if changes[role] == "unknown" {
			exact.ExeKnown = false
		}
		prober[exact.Pid] = exact
		switch role {
		case "owner":
			owner = supervise.ArmingOwner{Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), InstanceTag: "fixture"}
		case "runner":
			if changes[role] != "absent" {
				JSON(t, filepath.Join(root, "artifacts/agents/steward/runner.json"), steward.RunnerRecord{Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix()})
			}
		default:
			held = append(held, supervise.Held{Component: supervise.Component(role), Identity: exact.Ref(), Tag: role, Generation: 1})
		}
	}
	if err := os.MkdirAll(filepath.Join(supervise.SupervisionDir(root), "lock.d"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := supervise.WriteArmingOwner(root, owner); err != nil {
		t.Fatal(err)
	}
	checkout := supervise.DiskCheckout{Root: root, Self: identity.Ref{Pid: owner.Pid, StartedAtSec: owner.PidStartedAt}, SelfTag: owner.InstanceTag, IntervalSec: 1, Fingerprint: "fixture", WatcherCap: 330}
	if err := checkout.PublishState(held); err != nil {
		t.Fatal(err)
	}
	return func(root string) map[string]enginepin.Execution { return steward.ReadExecutions(root, prober) }
}

func Current(digest string) func(string) map[string]enginepin.Execution {
	return func(string) map[string]enginepin.Execution {
		return map[string]enginepin.Execution{"owner": {Digest: digest}, "watcher": {Digest: digest}, "reaper": {Digest: digest}, "runner": {Digest: digest}}
	}
}
