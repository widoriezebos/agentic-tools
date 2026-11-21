package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up/testdata/startfixture"
)

func TestEachStartReportsTheChangedEnrollment(t *testing.T) {
	t.Parallel()
	for _, caller := range []string{"recovery", "system", "session", "hook", "helm"} {
		t.Run(caller, func(t *testing.T) {
			t.Parallel()
			b := newProcessBed(t)
			root, err := canonicalPath(b.root())
			if err != nil {
				t.Fatal(err)
			}
			binary, digest := startfixture.Stage(t, root)
			owners := b.owners()
			owners.processes.executable = func() (string, error) { return binary, nil }
			calls := 0
			owners.processes.up = func(o up.Options) up.Result {
				calls++
				if o.Root != root {
					t.Fatalf("wrong installation %s", o.Root)
				}
				self, live, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
				if err != nil || live != identity.Alive {
					t.Fatalf("fixture process: %v %v", live, err)
				}
				o.Pid, o.StartTime, o.CallerPid = self.Pid, self.StartedAt.Unix(), self.Pid
				o.Binary, o.Runtime = binary, "claude"
				o.Executions = startfixture.Current(digest)
				o.EnsureArmed = func(arming supervise.EnsureOptions) (supervise.EnsureResult, error) {
					path := steward.RepoIdentityPath(root)
					current, err := steward.VerifyIdentity(path, root)
					if err != nil {
						t.Fatal(err)
					}
					current.Generation++
					if err := steward.MintIdentity(path, current); err != nil {
						t.Fatal(err)
					}
					return supervise.EnsureArmed(arming)
				}
				return up.Run(o)
			}
			owners.processes.process.armSteps = (processArmEffects{
				seed: func(string) (stewardLandingRefSeed, error) { return stewardLandingRefSeed{}, nil },
				arm:  func(string, string, processArmAuthority) (string, error) { return "", nil },
				up:   owners.processes.up,
			}).steps
			scope := processScope{Checkout: root, Installation: stateroot.Installation(root)}
			var out string
			code := 0
			switch caller {
			case "hook":
				var report bytes.Buffer
				code = (hookOwners{processes: &owners.processes}).Up(hooks.UpRequest{MetasystemRoot: scope.Installation, Repo: root}, &report, &report)
				lines := strings.Split(strings.TrimSpace(report.String()), "\n")
				out = lines[len(lines)-1]
			case "helm":
				out = owners.processes.helmRecover(scope)
			default:
				args := []string{caller, "start"}
				if caller == "recovery" {
					args = []string{"system", "start", "--if-down"}
				}
				var stderr string
				code, out, stderr = b.run(owners, args...)
				out += stderr
			}
			if calls != 1 || caller != "helm" && code != 1 || !strings.Contains(out, "Enrollment changed to install 149") {
				t.Fatalf("%s: calls=%d code=%d output=%s", caller, calls, code, out)
			}
			if !strings.Contains(out, "metasystem system start --if-down") || !strings.Contains(out, "if supervision is not running") {
				t.Errorf("%s omitted the recovery command or its condition", caller)
			}
		})
	}
}
