package testenv

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

var gitBeforeNamespace, _ = exec.LookPath("git")

func TestResolveTestGit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	realGit := filepath.Join(root, "real-git")
	checkTestenv(t, testexec.WriteFile(realGit, nil, 0o700))
	nonExecutable := filepath.Join(root, "not-executable")
	checkTestenv(t, os.WriteFile(nonExecutable, nil, 0o600))
	failed := errors.New("tool unavailable")
	for _, tc := range []struct {
		name, goos, found, output string
		lookupErr, runErr         error
		want                      string
		calls                     int
	}{
		{name: "wrapper resolved", goos: "darwin", found: "/usr/bin/git", output: realGit + "\n", want: realGit, calls: 1},
		{name: "xcrun fails", goos: "darwin", found: "/usr/bin/git", runErr: failed, calls: 1},
		{name: "fake already selected", goos: "darwin", found: realGit},
		{name: "non darwin", goos: "linux", found: "/usr/bin/git"},
		{name: "no git", goos: "darwin", lookupErr: failed},
		{name: "wrapper returned", goos: "darwin", found: "/usr/bin/git", output: "/usr/bin/git\n", calls: 1},
		{name: "missing target", goos: "darwin", found: "/usr/bin/git", output: filepath.Join(root, "missing"), calls: 1},
		{name: "not executable", goos: "darwin", found: "/usr/bin/git", output: nonExecutable, calls: 1},
		{name: "directory", goos: "darwin", found: "/usr/bin/git", output: root, calls: 1},
		{name: "empty output", goos: "darwin", found: "/usr/bin/git", calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			got := resolveTestGit(tc.goos, func(name string) (string, error) {
				if name != "git" {
					t.Fatalf("looked up %q", name)
				}
				return tc.found, tc.lookupErr
			}, func() ([]byte, error) {
				calls++
				return []byte(tc.output), tc.runErr
			})
			if got != tc.want || calls != tc.calls {
				t.Fatalf("resolved %q with %d xcrun calls, want %q with %d", got, calls, tc.want, tc.calls)
			}
			const inherited = "/fixture/bin:/usr/bin"
			path := namespaceGitPath(t.TempDir(), got, inherited)
			if tc.want == "" && path != inherited {
				t.Fatalf("PATH changed to %q", path)
			}
		})
	}
}

func TestNamespaceGitFailureKeepsPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	checkTestenv(t, os.WriteFile(filepath.Join(root, "git-bin"), nil, 0o600))
	const inherited = "/fixture/bin:/usr/bin"
	if got := namespaceGitPath(root, "/developer/git", inherited); got != inherited {
		t.Fatalf("PATH changed after directory creation failed: %q", got)
	}
}

// This adapter integration test checks Apple's selected binary and its resource
// lookup through the actual namespace PATH; a fake cannot establish that identity.
func TestNamespaceGitMatchesAppleWrapper(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" || gitBeforeNamespace != "/usr/bin/git" {
		t.Skip("Apple's git wrapper was not selected before namespace setup")
	}
	git, err := exec.LookPath("git")
	checkTestenv(t, err)
	directory := filepath.Join(filepath.Dir(os.Getenv("GIT_CONFIG_GLOBAL")), "git-bin")
	if git != filepath.Join(directory, "git") {
		t.Fatalf("git = %q, want the namespace git in %q", git, directory)
	}
	entries, err := os.ReadDir(directory)
	checkTestenv(t, err)
	if len(entries) != 1 || entries[0].Name() != "git" || entries[0].Type()&os.ModeSymlink == 0 {
		t.Fatalf("namespace git directory must contain only the git symlink: %v", entries)
	}
	for _, argument := range []string{"--version", "--exec-path"} {
		wrapper, err := exec.Command("/usr/bin/git", argument).CombinedOutput()
		checkTestenv(t, err)
		direct, err := exec.Command("git", argument).CombinedOutput()
		checkTestenv(t, err)
		if !bytes.Equal(wrapper, direct) {
			t.Fatalf("git %s: wrapper = %q, namespace = %q", argument, wrapper, direct)
		}
		t.Logf("git %s: %s", argument, direct)
	}
}

func TestNamespaceGitLetsLaterFakeWin(t *testing.T) {
	t.Parallel()
	const helper = "TESTENV_EXPECT_SELECTED_GIT"
	if want := os.Getenv(helper); want != "" {
		git, err := exec.LookPath("git")
		checkTestenv(t, err)
		if git != want {
			t.Fatalf("selected git = %q, want %q", git, want)
		}
		return
	}
	root := t.TempDir()
	target := filepath.Join(root, "real-git")
	checkTestenv(t, testexec.WriteFile(target, nil, 0o700))
	path := namespaceGitPath(root, target, os.Getenv("PATH"))
	fake := t.TempDir()
	checkTestenv(t, testexec.WriteFile(filepath.Join(fake, "git"), nil, 0o700))
	for _, tc := range []struct{ name, path, want string }{
		{"namespace", path, filepath.Join(root, "git-bin", "git")},
		{"later fake", fake + string(os.PathListSeparator) + path, filepath.Join(fake, "git")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=^TestNamespaceGitLetsLaterFakeWin$", "-test.timeout=30m")
			command.Env = append(os.Environ(), "PATH="+tc.path, helper+"="+tc.want)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("git selection in child: %v\n%s", err, output)
			}
		})
	}
}
