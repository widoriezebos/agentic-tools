package delegation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

type toolPrimaryGit struct{ primary, linked string }

func (g toolPrimaryGit) Run(_ context.Context, dir string, args ...string) ([]byte, []byte, error) {
	if dir == filepath.Join(g.linked, "tools", "install") {
		switch strings.Join(args, " ") {
		case "rev-parse --path-format=absolute --git-common-dir":
			return []byte(filepath.Join(g.primary, ".git")), nil, nil
		case "rev-parse --show-toplevel":
			return []byte(g.linked), nil, nil
		}
	}
	return nil, nil, fmt.Errorf("unexpected Git query in %s: %v", dir, args)
}

func TestAnUnarmedGoalWorktreeLaunchesItsPrimarysEngine(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	resolvedBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	git := toolPrimaryGit{primary: filepath.Join(base, "primary"), linked: filepath.Join(base, "goal")}
	primary := filepath.Join(git.primary, "tools", "install")
	linked := filepath.Join(git.linked, "tools", "install")
	for _, dir := range []string{filepath.Join(primary, "bin"), linked} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	engine := filepath.Join(primary, "bin", "metasystem")
	witness := filepath.Join(base, "engine-args")
	source := filepath.Join(base, "engine.go")
	if err := os.WriteFile(source, []byte(`package main
import ("os"; "strings")
func main() { if e := os.WriteFile(os.Getenv("ENGINE_WITNESS"), []byte(strings.Join(os.Args, "\n")), 0600); e != nil { panic(e) } }
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := testenv.Go("build", "-o", engine, source).CombinedOutput(); err != nil {
		t.Fatalf("build witness engine: %v: %s", err, out)
	}
	ports, err := newOwnerPorts(OwnerConfig{Root: linked, Host: stubHost{}, ConfigEnv: []string{"ENGINE_WITNESS=" + witness}}, git, func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	var pid int64
	testenv.ReapFixtureProcessGroups(t, []testenv.FixtureProcessGroup{{Verb: "primary engine supervisor", Resolve: func() (int, bool, error) { return int(pid), pid > 1, nil }}})
	pid, err = ports.Adapter.Launch(context.Background(), AdapterLaunch{Runtime: "recorder", Verb: "dispatch", Job: "job-1", StartGate: "/gate", InstanceTag: "tag", LaunchCapability: "cap"})
	if err != nil {
		t.Fatal(err)
	}
	if err := testenv.AwaitProcessTargetGone(-int(pid)); err != nil {
		t.Fatal(err)
	}
	args, err := os.ReadFile(witness)
	wantEngine := filepath.Join(resolvedBase, "primary", "tools", "install", "bin", "metasystem")
	if want := wantEngine + "\ndelegate-supervisor\nrecorder\ndispatch\n--root\n" + linked + "\n"; err != nil || !strings.HasPrefix(string(args), want) {
		t.Fatalf("launched program = %q, %v; want prefix %q", args, err, want)
	}
}
