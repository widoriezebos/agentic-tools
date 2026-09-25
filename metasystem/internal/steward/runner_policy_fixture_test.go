package steward

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

type runnerPolicyNotify struct {
	output string
	err    error
}

// runnerPolicyDeps supplies only raw repository answers. The runner and
// notification policies still decide whether the checkout may be armed.
func runnerPolicyDeps(t *testing.T, root string, notify *runnerPolicyNotify) rearmResolverDeps {
	t.Helper()
	expected := []testgit.Expectation{
		{
			Call:   testgit.Call{Dir: root, Args: []string{"rev-parse", "--git-common-dir"}},
			Result: testgit.Result{Stdout: []byte(".git\n")},
		},
		{
			Call:   testgit.Call{Dir: root, Args: []string{"rev-parse", "--git-dir"}},
			Result: testgit.Result{Stdout: []byte(".git\n")},
		},
	}
	if notify != nil {
		expected = append(expected, testgit.Expectation{
			Call:   testgit.Call{Dir: root, Args: []string{"config", "--get", "metasystem.steward.notify-command"}},
			Result: testgit.Result{Stdout: []byte(notify.output), Err: notify.err},
		})
	}
	stub := testgit.New(t, expected...)
	rawGit := func(dir string, args ...string) ([]byte, error) {
		result := stub.Run(testgit.Call{Dir: dir, Args: args})
		return result.Stdout, result.Err
	}
	return rearmResolverDeps{
		runnerExcluded: func(dir string, allowFixture bool) (string, bool) {
			return runnerExclusionWithGit(dir, allowFixture, rawGit)
		},
		notifyAvailable: func(dir string) bool {
			if notify == nil {
				t.Error("fixture exclusion must precede notification lookup")
				return false
			}
			_, ok := notifyCommandWithDependencies(dir, notificationDependencies{
				configuredCommand: func(root string) ([]byte, error) {
					return rawGit(root, "config", "--get", "metasystem.steward.notify-command")
				},
				platform: "linux",
			})
			return ok
		},
	}
}
