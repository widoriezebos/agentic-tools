package testenv

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const mainWithSetupFixtureEnv = "TESTENV_MAIN_WITH_SETUP_FIXTURE"

// mainWithSetupFixture is the setup of this binary's TestMain in fixture
// mode: it reports the custodian and the temp root it sees, makes a directory
// the way the moved package TestMains do, and refuses in "fail" mode.
func mainWithSetupFixture() error {
	custodian, started := FixtureCustodian()
	_, state, err := identity.KernelProber{}.Probe(custodian.Pid)
	made, makeErr := os.MkdirTemp("", "metasystem-proofrun-admission.")
	fmt.Printf("custodian=%t alive=%t\ntempdir=%s\nmade=%s\n", started, err == nil && state == identity.Alive, os.TempDir(), made)
	if makeErr != nil {
		return makeErr
	}
	if os.Getenv(mainWithSetupFixtureEnv) == "fail" {
		return errors.New("fixture setup refused")
	}
	return nil
}

func TestMainWithSetupFixtureProcess(t *testing.T) {
	t.Parallel()
	if os.Getenv(mainWithSetupFixtureEnv) != "" {
		fmt.Println("ran=true")
	}
}

func TestMainWithSetupRunsInsideNamespaceAfterCustodian(t *testing.T) {
	t.Parallel()
	registry := os.Getenv(supervisionRegistryHome)
	if registry == "" {
		t.Fatal("parent TestMain did not pin a registry home")
	}
	for _, mode := range []string{"pass", "fail"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			scratch := t.TempDir()
			command := exec.Command(os.Args[0], "-test.run=^TestMainWithSetupFixtureProcess$", "-test.count=1", "-test.v")
			for _, entry := range os.Environ() {
				name, _, _ := strings.Cut(entry, "=")
				if name != "TMPDIR" && name != mainWithSetupFixtureEnv {
					command.Env = append(command.Env, entry)
				}
			}
			command.Env = append(command.Env, "TMPDIR="+scratch, mainWithSetupFixtureEnv+"="+mode)
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			runErr := command.Run()
			report := map[string]string{}
			for _, line := range strings.Split(stdout.String(), "\n") {
				if name, value, ok := strings.Cut(line, "="); ok {
					report[name] = value
				}
			}
			if report["custodian"] != "true alive=true" {
				t.Fatalf("setup ran before a live fixture custodian: %q\nstdout=%s\nstderr=%s", report["custodian"], stdout.String(), stderr.String())
			}
			tempdir, made := report["tempdir"], report["made"]
			namespace := filepath.Dir(tempdir)
			if filepath.Base(tempdir) != "tmp" || !strings.HasPrefix(filepath.Base(namespace), processNamespacePrefix) ||
				filepath.Dir(namespace) != registry || filepath.Dir(made) != tempdir {
				t.Fatalf("setup saw tempdir=%q made=%q, want a directory in its own namespace below %q", tempdir, made, registry)
			}
			if entries, err := os.ReadDir(scratch); err != nil || len(entries) != 0 {
				t.Fatalf("setup wrote into the host temp root: entries=%v err=%v", entries, err)
			}
			if _, err := os.Stat(namespace); !os.IsNotExist(err) {
				t.Fatalf("namespace %s survived the package exit: %v", namespace, err)
			}
			var exit *exec.ExitError
			switch mode {
			case "pass":
				if runErr != nil || report["ran"] != "true" {
					t.Fatalf("passing setup: err=%v ran=%q\nstderr=%s", runErr, report["ran"], stderr.String())
				}
			case "fail":
				if !errors.As(runErr, &exit) || exit.ExitCode() != 2 || report["ran"] != "" ||
					!strings.Contains(stderr.String(), "set up test package: fixture setup refused") {
					t.Fatalf("refused setup: err=%v ran=%q\nstderr=%s", runErr, report["ran"], stderr.String())
				}
			}
		})
	}
}
