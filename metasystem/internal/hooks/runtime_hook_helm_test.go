package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
)

// helmInstallation is a hook installation inside a seat at the helm (a fake
// .git, no Git): taken says whether the signature is written.
func helmInstallation(t *testing.T, taken bool) (hookInstallation, *fakeOps) {
	t.Helper()
	installation := newHookInstallation(t)
	if err := os.Mkdir(filepath.Join(installation.root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if taken {
		at := time.Date(2026, 9, 28, 19, 14, 3, 0, time.UTC).Format(time.RFC3339)
		if _, err := helm.Write(installation.root, helm.Record{By: "Wido", At: at, Reason: "coordinating by hand"}); err != nil {
			t.Fatal(err)
		}
	}
	return installation, newFakeOps(t, installation)
}

func helmYields(t *testing.T, installation hookInstallation) []string {
	data, _ := os.ReadFile(filepath.Join(installation.root, ".git", "metasystem", "helm-yields.log"))
	return strings.FieldsFunc(string(data), func(r rune) bool { return r == '\n' })
}

func wantHelmStop(t *testing.T, installation hookInstallation, ops *fakeOps, env map[string]string) {
	t.Helper()
	run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"s-1"}`, env: env})
	line := strings.TrimSuffix(run.stdout, "\n")
	if run.status != 0 || strings.Contains(line, "\n") || !validStopOutput(line, true) ||
		!strings.Contains(line, "helm: HUMAN AT THE HELM: Wido since") || !strings.Contains(line, "coordinating by hand") || !strings.Contains(line, "Stop allowed.") {
		t.Fatalf("stop under the helm: status %d stdout %q stderr %q", run.status, run.stdout, run.stderr)
	}
	if len(ops.calls) != 0 {
		t.Fatalf("stop under the helm reached the owners:\n%s", ops.trace())
	}
}

func TestStopUnderHelmEmitsValidAllowForm(t *testing.T) {
	t.Parallel()
	t.Run("HM-6", func(t *testing.T) {
		installation, ops := helmInstallation(t, true)
		wantHelmStop(t, installation, ops, nil)
		if yields := helmYields(t, installation); len(yields) != 1 || !strings.Contains(yields[0], `"boundary":"stop-hook"`) || !strings.Contains(yields[0], `"would":"not`) {
			t.Fatalf("stop yield records = %q", yields)
		}
	})
}

func TestStopUnderHelmCallsNoOps(t *testing.T) {
	t.Parallel()
	t.Run("HM-6", func(t *testing.T) {
		installation, ops := helmInstallation(t, true)
		wantHelmStop(t, installation, ops, nil)
		if len(ops.turnVerdicts) != 0 || len(ops.upRequests) != 0 {
			t.Fatalf("stop under the helm decided or armed: %+v %+v", ops.turnVerdicts, ops.upRequests)
		}
	})
}

func TestStopUnderHelmWithBrokenDelegateHintsAllows(t *testing.T) {
	t.Parallel()
	t.Run("HM-13", func(t *testing.T) {
		installation, ops := helmInstallation(t, true)
		wantHelmStop(t, installation, ops, map[string]string{"METASYSTEM_HOOK_DELEGATE_JOB": "job-without-roots"})
	})
}

func TestStopUnderHelmUnwritableYieldLogAllows(t *testing.T) {
	t.Parallel()
	t.Run("HM-12", func(t *testing.T) {
		installation, ops := helmInstallation(t, true)
		if err := os.Mkdir(filepath.Join(installation.root, ".git", "metasystem", "helm-yields.log"), 0o700); err != nil {
			t.Fatal(err)
		}
		wantHelmStop(t, installation, ops, nil)
	})
}

func TestStartUnderHelmArmsNothing(t *testing.T) {
	t.Parallel()
	t.Run("HM-6", func(t *testing.T) {
		installation, ops := helmInstallation(t, true)
		run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", payload: `{"session_id":"s-1","source":"startup"}`,
			env: map[string]string{"METASYSTEM_HOOK_DELEGATE_JOB": "job-without-roots"}})
		if run.status != 0 || !strings.Contains(run.stdout, `{"systemMessage":"helm: HUMAN AT THE HELM: Wido`) || len(ops.calls) != 0 || len(helmYields(t, installation)) != 1 {
			t.Fatalf("start under the helm: status %d stdout %q calls:\n%s", run.status, run.stdout, ops.trace())
		}
	})
}

func TestEndAndReceiptUnderHelmRecordOnly(t *testing.T) {
	t.Parallel()
	t.Run("HM-6", func(t *testing.T) {
		installation, ops := helmInstallation(t, true)
		for _, event := range []string{"end", "receipt"} {
			run := runHook(t, installation, ops, hookCall{runtime: "claude", event: event, payload: `{"session_id":"s-1"}`})
			if run.status != 0 || run.stdout != "" || !strings.HasPrefix(run.stderr, "helm: HUMAN AT THE HELM") {
				t.Fatalf("%s under the helm: status %d stdout %q stderr %q", event, run.status, run.stdout, run.stderr)
			}
		}
		if yields := helmYields(t, installation); len(yields) != 2 || len(ops.calls) != 0 {
			t.Fatalf("end and receipt: yields %q, calls:\n%s", yields, ops.trace())
		}
	})
}

func TestStopWithoutHelmUnchanged(t *testing.T) {
	t.Parallel()
	t.Run("HM-6", func(t *testing.T) {
		installation, ops := helmInstallation(t, false)
		run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"s-1"}`})
		if run.status != 0 || strings.Contains(run.stdout, "HELM") || len(ops.turnVerdicts) != 1 {
			t.Fatalf("stop without the helm: status %d stdout %q verdicts %d", run.status, run.stdout, len(ops.turnVerdicts))
		}
	})
}
