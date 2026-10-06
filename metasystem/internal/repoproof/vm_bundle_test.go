package repoproof

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// A command stub cannot prove that the transfer keeps both the repository prefix
// and Git metadata. This adapter integration uses a real fixture repository.
func TestFullVMBundleAdapterRunsReporterFromCompleteCheckout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	module := filepath.Join(root, "metasystem")
	proof := filepath.Join(module, "proof")
	if err := os.MkdirAll(proof, 0755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture Git %v: %s %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "--quiet")
	if err := os.WriteFile(filepath.Join(root, "root-marker"), []byte("repository root\n"), 0644); err != nil {
		t.Fatal(err)
	}
	script := "set -e\ntest -f ../root-marker\ntest -d ../.git\ntest \"$(git rev-parse HEAD)\" = \"$LANDING_COMMIT\"\ntest \"$1\" = --host\nprintf 'VM checkout proved\\nLANDING-CHECKED\\t0\\n'\n"
	if err := os.WriteFile(filepath.Join(proof, "full.sh"), []byte(script), 0644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "--quiet", "-m", "proof fixture")
	commit := git("rev-parse", "HEAD")
	tools, remote := t.TempDir(), t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	wrapper := "#!/bin/sh\nexec \"$PROOF_VM_TEST_BINARY\" -test.run '^TestFullVMRemoteProcess$' -- \"$@\"\n"
	if err := testexec.WriteFile(filepath.Join(tools, "limactl"), []byte(wrapper), 0755); err != nil {
		t.Fatal(err)
	}
	command := func(argv []string, stdout, stderr io.Writer) error {
		if argv[0] == "metasystem" {
			if argv[1] == "landing" {
				fmt.Fprint(stdout, `{"data":{"root":"/fixture-lane"}}`)
			} else {
				fmt.Fprint(stdout, `{"data":{"settings":[{"Key":"host.proof-vm","Value":"fixture-vm"}]}}`)
			}
			return nil
		}
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = module
		cmd.Env = append(os.Environ(), "PATH="+tools+":"+os.Getenv("PATH"), "PROOF_VM_TEST_BINARY="+binary, "PROOF_VM_TEST_ROOT="+remote)
		cmd.Stdout, cmd.Stderr = stdout, stderr
		return cmd.Run()
	}
	for range 2 {
		var out, stderr bytes.Buffer
		code := Run(&out, &stderr, func(key string) string {
			if key == "LANDING_COMMIT" {
				return commit
			}
			return ""
		}, command)
		if code != 0 || !strings.HasSuffix(out.String(), "VM checkout proved\nLANDING-CHECKED\t0\n") {
			t.Fatalf("VM checkout proof: %d %s %s", code, &out, &stderr)
		}
	}
	if refs := git("for-each-ref", "--format=%(refname)", "refs/metasystem/proof/"); refs != "" {
		t.Fatalf("temporary proof ref survived: %s", refs)
	}
	if err := os.Rename(filepath.Join(remote, commit, ".git"), filepath.Join(remote, commit, "saved-git")); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	code := Run(&out, &stderr, func(key string) string {
		if key == "LANDING_COMMIT" {
			return commit
		}
		return ""
	}, command)
	if code != 1 || !strings.HasSuffix(out.String(), "LANDING-NOT-RUN\tenvironment\n") || strings.Contains(out.String(), "LANDING-FAILED") {
		t.Fatalf("a missing checkout became a code red: %d %s %s", code, &out, &stderr)
	}
}

func TestFullVMRemoteProcess(t *testing.T) {
	t.Parallel()
	root := os.Getenv("PROOF_VM_TEST_ROOT")
	if root == "" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) != 7 || strings.Join(args[1:6], " ") != "shell fixture-vm -- bash -c" {
		fmt.Fprintln(os.Stderr, "unexpected VM invocation", args)
		os.Exit(2)
	}
	script := strings.ReplaceAll(args[6], "/tmp/metasystem-proof", root)
	cmd := exec.Command("/bin/bash", "-c", script)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}
