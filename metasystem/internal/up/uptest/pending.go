// Package uptest supplies real session arming with private process boundaries.
package uptest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

// Pending drives up.Run through a real cancelled ledger read. Only process
// discovery and the supervision launches are replaced; no child is launched.
func Pending(t testing.TB) up.Result {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0644); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(binary), 0755); err != nil {
		t.Fatal(err)
	}
	data := []byte("#!/bin/sh\nexit 99\n")
	if err := testexec.WriteFile(binary, data, 0755); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if err := steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{RepoIdentity: root, Generation: 1, InstallPath: binary, InstallDigest: fmt.Sprintf("sha256:%x", digest), MintedAt: "2026-08-23T09:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("read fixture process: %s %v", state, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := goal.Endpoint{Root: root, Remote: "origin", Repository: testgoal.New(nil, time.Now(), "seed")}
	result := up.Run(up.Options{Root: root, Scope: root, Binary: binary, Session: "pending-reader", OwnerLineage: "pending-reader", CallerPid: exact.Pid,
		FindSessionAncestor: func(string, int64, string) (census.AgentAncestor, error) {
			return census.AgentAncestor{Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), PidStartTicks: exact.StartTicks, BootID: exact.BootID, Runtime: "fake"}, nil
		},
		EnsureArmed: func(supervise.EnsureOptions) (supervise.EnsureResult, error) {
			return supervise.EnsureResult{Action: "joined", Generation: 1}, nil
		},
		EnsureStewardRunner: func(string, *steward.EnrolledBinary, int) (steward.EnsureRunnerResult, error) {
			return steward.EnsureRunnerResult{Action: "already-running", Generation: 1}, nil
		},
		RestampStopCapability: func(string, string, int64) (up.StopCapabilityRestampResult, error) {
			_, observation, err := goal.FreshProjection(ctx, e, func() (time.Time, error) { return time.Now(), nil })
			return up.StopCapabilityRestampResult{Observation: &observation}, err
		},
	})
	if result.Adoption == nil || result.Adoption.Status != "pending" || result.Outcome != "armed" || result.ExitCode() != 0 {
		t.Fatalf("up's real pending result: %+v", result)
	}
	return result
}
