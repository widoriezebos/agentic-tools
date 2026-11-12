package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func newCheckoutDeliveryFixture(t *testing.T) *buildFixture {
	t.Helper()
	f := newBuildFixture(t)
	f.fence = func(string, int64) []gaterun.Holder { return nil }
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(f.root)), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, contents := range map[string]string{
		steward.RepoIdentityPath(f.root):         "{}",
		filepath.Join(f.root, "metasystem.conf"): "metasystem.engine-delivery=central\n",
	} {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func TestBuildCheckoutCentralRefusesBeforeEffects(t *testing.T) {
	t.Parallel()
	f := newCheckoutDeliveryFixture(t)
	engine := f.installEngine()
	if code := f.run(); code != 1 {
		t.Fatalf("exit = %d, want 1; stderr: %s", code, f.stderr.String())
	}
	want := "go-build: this checkout takes its engine from the deploy (central delivery); nothing was built\n" +
		"the steward installs the deployed engine at its next tick while no launch runs, then repeat the command that sent you here; to deploy main's tip: metasystem deploy now\n" +
		"run: METASYSTEM_ALLOW_CHECKOUT_BUILD=1 go run ./cmd/devgate build  (only to build into this checkout anyway)\n"
	if f.stderr.String() != want || f.stdout.Len() != 0 {
		t.Fatalf("stdout = %q, stderr = %q; want only %q", f.stdout.String(), f.stderr.String(), want)
	}
	if len(f.goCalls) != 0 || len(f.gitCalls) != 0 || f.fenceHits != 0 {
		t.Fatalf("refusal ran go, git or fence: %+v, %+v, %d", f.goCalls, f.gitCalls, f.fenceHits)
	}
	if data, err := os.ReadFile(engine); err != nil || string(data) != "previous engine" {
		t.Fatalf("installed engine = %q, %v", data, err)
	}
	if entries, err := os.ReadDir(filepath.Dir(engine)); err != nil || len(entries) != 1 {
		t.Fatalf("bin entries = %v, %v; want only the installed engine", entries, err)
	}
}

func TestBuildCheckoutCentralAllowsOut(t *testing.T) {
	t.Parallel()
	f := newCheckoutDeliveryFixture(t)
	out := filepath.Join(f.root, "candidate")
	if code := f.run("--out", out); code != 0 {
		t.Fatalf("exit = %d; stderr: %s", code, f.stderr.String())
	}
	call := f.onlyGoCall()
	if index := slices.Index(call.args, "-o"); index < 0 || call.args[index+1] != out {
		t.Fatalf("go args = %q; want -o %q", call.args, out)
	}
}

func TestBuildCheckoutCentralAllowsPersonEscape(t *testing.T) {
	t.Parallel()
	f := newCheckoutDeliveryFixture(t)
	f.env["METASYSTEM_ALLOW_CHECKOUT_BUILD"] = "1"
	if code := f.run(); code != 0 {
		t.Fatalf("exit = %d; stderr: %s", code, f.stderr.String())
	}
	f.onlyGoCall()
}

func TestBuildCheckoutCentralAllowsUnarmed(t *testing.T) {
	t.Parallel()
	f := newCheckoutDeliveryFixture(t)
	if err := os.Remove(steward.RepoIdentityPath(f.root)); err != nil {
		t.Fatal(err)
	}
	if code := f.run(); code != 0 {
		t.Fatalf("exit = %d; stderr: %s", code, f.stderr.String())
	}
	f.onlyGoCall()
}

func TestBuildCheckoutAllowsSourceOrUnreadableConf(t *testing.T) {
	t.Parallel()
	for _, delivery := range []string{"", "metasystem.engine-delivery=source\n", "unreadable"} {
		t.Run(delivery, func(t *testing.T) {
			f := newCheckoutDeliveryFixture(t)
			conf := filepath.Join(f.root, "metasystem.conf")
			if err := os.Remove(conf); err != nil {
				t.Fatal(err)
			}
			var err error
			if delivery == "unreadable" {
				err = os.Mkdir(conf, 0o755)
			} else {
				err = os.WriteFile(conf, []byte(delivery), 0o644)
			}
			if err != nil {
				t.Fatal(err)
			}
			if code := f.run(); code != 0 {
				t.Fatalf("exit = %d; stderr: %s", code, f.stderr.String())
			}
			f.onlyGoCall()
		})
	}
}

func TestBuildCheckoutCentralDirectBuildRemainsAllowed(t *testing.T) {
	t.Parallel()
	f := newCheckoutDeliveryFixture(t)
	if code := runBuild(context.Background(), nil, f.root, f.deps()); code != 0 {
		t.Fatalf("exit = %d; stderr: %s", code, f.stderr.String())
	}
	f.onlyGoCall()
}
