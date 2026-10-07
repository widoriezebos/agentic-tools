package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type freshHookOperations struct {
	hookOwners
	b                          *goalCLIBed
	prepared, armed, recovered int
}

func (o *freshHookOperations) Git(args ...string) (string, error) {
	if strings.HasSuffix(strings.Join(args, " "), "rev-parse --path-format=absolute --git-dir --git-common-dir") {
		return o.b.root + "/.git\n" + o.b.root + "/.git\n", nil
	}
	if strings.Contains(strings.Join(args, " "), "rev-parse --show-toplevel") {
		return o.b.root + "\n", nil
	}
	o.b.t.Fatalf("hook used unexpected Git: %v", args)
	return "", errors.New("unexpected Git")
}
func (o *freshHookOperations) EngineBehind(stateroot.Installation, string) (bool, error) {
	return false, nil
}
func (o *freshHookOperations) RuntimeNames() (string, int)                    { return "claude\nfake\n", 0 }
func (o *freshHookOperations) StateRoot(stateroot.Installation) (string, int) { return o.b.root, 0 }
func (o *freshHookOperations) FindAncestor(stateroot.Installation, int, string, bool) (string, int) {
	exact, _, _ := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	return fmt.Sprintf(`{"runtime":"claude","pid":%d,"pidStartedAt":%d}`, exact.Pid, exact.StartedAt.Unix()), 0
}
func (o *freshHookOperations) HookDelegate(string, stateroot.Installation, string, int) (string, int) {
	return "", 3
}
func (o *freshHookOperations) Classify(string, stateroot.Installation, int) (string, int) {
	return "", 7
}
func (o *freshHookOperations) StartContext(string) (string, int) {
	return "field=hookSpecificOutput.additionalContext event=SessionStart bytes=2048 sources=startup,compact\n", 0
}
func (o *freshHookOperations) PeerClaims(string) (string, int) {
	return `{"live":{},"concluded":{}}`, 0
}
func (o *freshHookOperations) BrainBoot(context.Context, string, string, int, int) (string, string, int) {
	o.prepared++
	return `{"declared":true,"state":"declared","payload":"role packet","bytes":11,"sections":{"asks":"complete","held":"complete","fleet":"complete","digest":"complete"},"digestEmitted":false,"digestCursor":0,"digestPrefixSha256":"","declarationSha256":"` + strings.Repeat("a", 64) + `"}`, "", 0
}
func (o *freshHookOperations) BrainStartDelivered(string, string, string, string, string) int {
	return 0
}
func (o *freshHookOperations) Up(request hooks.UpRequest, stdout, stderr io.Writer) int {
	o.armed++
	return o.hookOwners.Up(request, stdout, stderr)
}
func (o *freshHookOperations) SessionStart(root, session string, stdout, stderr io.Writer) int {
	o.recovered++
	return o.hookOwners.SessionStart(root, session, stdout, stderr)
}

func TestLedgerFreshClaudeHookKeepsOfflinePreparationAndRecovery(t *testing.T) {
	t.Parallel()
	b := newGoalCLIBed(t, goalCLISeed{})
	attempts := 0
	r := freshCommandRepository{Repository: b.repo, attempts: &attempts, capture: func(context.Context, string) (string, error) { return "", errors.New("remote transport unavailable") }}
	inputs := freshUpFixture(t, b, freshDependencies(b, r))
	var output, diagnostic bytes.Buffer
	ops := &freshHookOperations{hookOwners: hookOwners{diagnostics: &diagnostic, upInputs: inputs, repositoryTop: fakeTop(b.root)}, b: b}
	invocation := hooks.Invocation{Installation: b.root, Stdin: strings.NewReader(`{"session_id":"goal-cli-fixture","source":"startup","cwd":"` + b.root + `"}`),
		Pid: os.Getpid(), Ppid: os.Getppid(), Now: b.clock, Monotonic: func() time.Duration { return 0 }, After: func(time.Duration) <-chan time.Time { return make(chan time.Time) }, Lookup: func(string) (string, bool) { return "", false }, IsExecutable: func(string) bool { return true }, Environ: func() []string { return nil }}
	code := runHookEntryWithInputs([]string{"claude", "start"}, &output, &diagnostic, func(string, string, chan os.Signal, io.Writer) hookEntryInputs {
		inputs := defaultHookEntryInputs(b.root, "start", nil, &diagnostic)
		inputs.invocation, inputs.operations = invocation, ops
		return inputs
	})
	if code != 0 || attempts != 0 || ops.prepared != 1 || ops.armed != 1 || ops.recovered != 1 || !strings.Contains(output.String(), "role packet") || !strings.Contains(output.String(), "adoption pending") || !strings.Contains(output.String(), "metasystem session start") {
		t.Fatalf("hook lost offline preparation, adoption cause or wait recovery: exit=%d attempts=%d prepared=%d armed=%d recovered=%d out=%s err=%s", code, attempts, ops.prepared, ops.armed, ops.recovered, &output, &diagnostic)
	}
}
