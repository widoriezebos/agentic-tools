package steward

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

func notifyIdentity(t *testing.T, root, enrollment string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
		RepoIdentity: canonicalPath(root), Generation: 1, InstallPath: "/bin/true",
		MintedAt: "2026-09-06T00:00:00Z", Enrollment: enrollment,
	}); err != nil {
		t.Fatal(err)
	}
}

type notifyRead struct {
	command string
	err     error
}

type notifyFixture struct {
	root       string
	sink       string
	deps       notificationDependencies
	invocation []string
}

func newNotifyFixture(t *testing.T, reads ...notifyRead) *notifyFixture {
	t.Helper()
	f := &notifyFixture{root: t.TempDir(), sink: filepath.Join(t.TempDir(), "delivered.log")}
	expected := make([]testgit.Expectation, 0, len(reads))
	for _, read := range reads {
		command := read.command
		if command == "default" {
			command = `printf '%s\n' "$STEWARD_MESSAGE" >> ` + f.sink
		}
		result := testgit.Result{Err: read.err}
		if read.err == nil {
			result.Stdout = []byte(command + "\n")
		}
		expected = append(expected, testgit.Expectation{
			Call:   testgit.Call{Dir: f.root, Args: []string{"config", "--get", "metasystem.steward.notify-command"}},
			Result: result,
		})
	}
	stub := testgit.New(t, expected...)
	f.deps = notificationDependencies{
		configuredCommand: func(root string) ([]byte, error) {
			result := stub.Run(testgit.Call{Dir: root, Args: []string{"config", "--get", "metasystem.steward.notify-command"}})
			return result.Stdout, result.Err
		},
		platform: "linux", commandContext: exec.CommandContext,
	}
	return f
}

func configuredNotifyFixture(t *testing.T, reads int, command string) *notifyFixture {
	t.Helper()
	answers := make([]notifyRead, reads)
	for i := range answers {
		answers[i].command = command
	}
	return newNotifyFixture(t, answers...)
}

func absentNotifyFixture(t *testing.T, reads int) *notifyFixture {
	t.Helper()
	answers := make([]notifyRead, reads)
	for i := range answers {
		answers[i].err = errors.New("configuration absent")
	}
	return newNotifyFixture(t, answers...)
}

func (f *notifyFixture) darwin() {
	f.deps.platform = "darwin"
	f.deps.commandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		f.invocation = append([]string{name}, args...)
		return exec.CommandContext(ctx, "/usr/bin/true")
	}
}

func (f *notifyFixture) deliver(root, message string) error {
	return deliverWithDependencies(root, message, f.deps)
}

func (f *notifyFixture) pending() (int, error) {
	return deliverPendingWith(f.root, f.deliver)
}

func (f *notifyFixture) alert(health HealthVerdict, message string, now time.Time) (AlertEpisode, error) {
	return updateAlertEpisodesWith(f.root, health, message, now, f.deliver)
}
