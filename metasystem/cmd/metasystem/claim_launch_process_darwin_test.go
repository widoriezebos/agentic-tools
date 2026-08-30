//go:build darwin

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestClaimLaunchCapabilitySurvivesRelativeExecutableProcessShape(t *testing.T) {
	const helperEnv = "METASYSTEM_RELATIVE_EXECUTABLE_HELPER"
	if os.Getenv(helperEnv) == "1" {
		kernelPath, ok := identity.ExecutablePath(int64(os.Getpid()))
		if !ok || filepath.IsAbs(kernelPath) {
			t.Fatalf("kernel executable path = %q, want the relative Darwin shape from the live defect", kernelPath)
		}
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		legacyKernelPath := kernelPath
		if resolved, resolveErr := filepath.EvalSymlinks(legacyKernelPath); resolveErr == nil {
			legacyKernelPath = resolved
		}
		if legacyKernelPath == resolvedDelegatePath(self) {
			t.Fatalf("legacy comparison unexpectedly joined relative %q to absolute %q", legacyKernelPath, resolvedDelegatePath(self))
		}
		binding := dispatchcore.DelegateClaimCapabilityBinding{
			JobID: "relative-executable-job", OperationID: "relative-executable-operation",
			DispatchMode: dispatchcore.DispatchModeFresh, AdapterVerb: "dispatch",
		}
		root := os.Getenv("METASYSTEM_RELATIVE_EXECUTABLE_ROOT")
		if !claimLaunchInternalAuthorized(root, binding, true) {
			t.Fatal("delegate capability did not authorize preflight across the relative executable process shape")
		}
		if !claimLaunchInternalAuthorized(root, binding, false) {
			t.Fatal("delegate capability did not authorize consumption across the relative executable process shape")
		}
		return
	}

	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(binDir, "metasystem.test")
	if err := os.WriteFile(copyPath, data, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := dispatchcore.MintDelegateClaimCapability(root, dispatchcore.DispatchModeFresh)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(filepath.Join("bin", "metasystem.test"), "-test.run=^TestClaimLaunchCapabilitySurvivesRelativeExecutableProcessShape$")
	command.Dir = root
	command.Env = append(os.Environ(),
		helperEnv+"=1",
		"METASYSTEM_RELATIVE_EXECUTABLE_ROOT="+root,
		"METASYSTEM_DELEGATE_INTERNAL=1",
		delegateClaimCapabilityEnv+"="+raw,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("relative executable helper failed: %v\n%s", err, output)
	}
}
