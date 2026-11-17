package testenv

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
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
			path := namespaceGitPath(t.TempDir(), got, inherited, func(string, string) (gitResources, error) { return gitResources{}, nil })
			if tc.want == "" && path != inherited {
				t.Fatalf("PATH changed to %q", path)
			}
		})
	}
}

func TestNamespaceGitFailureKeepsPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	checkTestenv(t, os.WriteFile(filepath.Join(root, "git-prefix"), nil, 0o600))
	const inherited = "/fixture/bin:/usr/bin"
	if got := namespaceGitPath(root, "/developer/git", inherited, func(string, string) (gitResources, error) {
		t.Fatal("probe after mkdir failure")
		return gitResources{}, nil
	}); got != inherited {
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
	directory := filepath.Join(filepath.Dir(os.Getenv("GIT_CONFIG_GLOBAL")), "git-prefix", "bin")
	if git == "/usr/bin/git" {
		t.Log("namespace prefix guard retained Apple's wrapper")
	} else {
		if git != filepath.Join(directory, "git") {
			t.Fatalf("git = %q, want the namespace git in %q or Apple's wrapper", git, directory)
		}
		entries, err := os.ReadDir(directory)
		checkTestenv(t, err)
		if len(entries) != 1 || entries[0].Name() != "git" || entries[0].Type()&os.ModeSymlink == 0 {
			t.Fatalf("namespace git directory must contain only the git symlink: %v", entries)
		}
	}
	t.Logf("selected git: %s; DEVELOPER_DIR=%s", git, os.Getenv("DEVELOPER_DIR"))
	root := t.TempDir()
	environment := gitProbeEnvironment(os.Environ(), root)
	// Compare the real system-config lookup too; the namespace normally
	// disables it for fixtures, but that must not conceal a prefix difference.
	environment = slices.DeleteFunc(environment, func(entry string) bool {
		return strings.HasPrefix(entry, "GIT_CONFIG_NOSYSTEM=")
	})
	command := func(git string, args ...string) *exec.Cmd {
		cmd := exec.Command(git, args...)
		cmd.Dir = root
		cmd.Env = environment
		return cmd
	}
	for _, argument := range []string{"--version", "--exec-path", "--html-path", "--info-path"} {
		wrapper, err := command("/usr/bin/git", argument).CombinedOutput()
		checkTestenv(t, err)
		direct, err := command(git, argument).CombinedOutput()
		checkTestenv(t, err)
		equal := bytes.Equal(wrapper, direct)
		if argument != "--version" {
			equal = canonicalGitResource(t, string(wrapper)) == canonicalGitResource(t, string(direct))
		}
		if !equal {
			t.Fatalf("git %s: wrapper = %q, namespace = %q", argument, wrapper, direct)
		}
		t.Logf("git %s: wrapper=%s namespace=%s", argument, bytes.TrimSpace(wrapper), bytes.TrimSpace(direct))
	}
	for _, args := range [][]string{{"config", "--system", "--list", "--show-origin"}, {"var", "GIT_EXEC_PATH"}} {
		wrapper := command("/usr/bin/git", args...)
		direct := command(git, args...)
		wrapperOutput, wrapperErr := wrapper.CombinedOutput()
		directOutput, directErr := direct.CombinedOutput()
		if wrapper.ProcessState == nil || direct.ProcessState == nil {
			t.Fatalf("git %v did not start: wrapper=%v namespace=%v", args, wrapperErr, directErr)
		}
		equal := bytes.Equal(wrapperOutput, directOutput)
		if args[0] == "var" && wrapperErr == nil && directErr == nil {
			equal = canonicalGitResource(t, string(wrapperOutput)) == canonicalGitResource(t, string(directOutput))
		}
		if wrapper.ProcessState.ExitCode() != direct.ProcessState.ExitCode() || !equal {
			t.Fatalf("git %v differs: wrapper=(%v, %q) namespace=(%v, %q)", args, wrapperErr, wrapperOutput, directErr, directOutput)
		}
		t.Logf("git %v: exit=%d output=%s", args, direct.ProcessState.ExitCode(), directOutput)
	}
	hooks := func(git string) map[string]string {
		t.Helper()
		if output, err := command(git, "submodule", "-h").CombinedOutput(); err != nil || !bytes.Contains(output, []byte("usage: git submodule")) {
			t.Fatalf("%s submodule -h: %v\n%s", git, err, output)
		}
		repo := t.TempDir()
		if output, err := command(git, "init", "-q", repo).CombinedOutput(); err != nil {
			t.Fatalf("%s init: %v\n%s", git, err, output)
		}
		directory := filepath.Join(repo, ".git", "hooks")
		entries, err := os.ReadDir(directory)
		checkTestenv(t, err)
		if len(entries) == 0 {
			t.Fatal("git init created no hook templates")
		}
		contents := make(map[string]string)
		for _, entry := range entries {
			data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
			checkTestenv(t, err)
			contents[entry.Name()] = string(data)
		}
		return contents
	}
	wrapper, selected := hooks("/usr/bin/git"), hooks(git)
	if !reflect.DeepEqual(wrapper, selected) {
		t.Fatal("git init hook templates differ from Apple's wrapper")
	}
	t.Logf("submodule found; git init produced %d matching hook templates", len(selected))
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
	target, resources := fixtureGitInstallation(t)
	path := namespaceGitPath(root, target, os.Getenv("PATH"), func(string, string) (gitResources, error) { return resources, nil })
	fake := t.TempDir()
	checkTestenv(t, testexec.WriteFile(filepath.Join(fake, "git"), nil, 0o700))
	for _, tc := range []struct{ name, path, want string }{
		{"namespace", path, filepath.Join(root, "git-prefix", "bin", "git")},
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

func fixtureGitInstallation(t *testing.T) (string, gitResources) {
	t.Helper()
	prefix := filepath.Join(t.TempDir(), "developer", "usr")
	for _, name := range []string{"bin", "libexec/git-core", "share", "etc"} {
		checkTestenv(t, os.MkdirAll(filepath.Join(prefix, name), 0o700))
	}
	git := filepath.Join(prefix, "bin", "git")
	checkTestenv(t, testexec.WriteFile(git, nil, 0o700))
	return git, gitResources{execPath: filepath.Join(prefix, "libexec", "git-core"), hooks: map[string]string{"hook.sample": "template\n"}}
}

func TestNamespaceGitResourceGuard(t *testing.T) {
	t.Parallel()
	target, resources := fixtureGitInstallation(t)
	missing := filepath.Join(t.TempDir(), "missing")
	file := filepath.Join(t.TempDir(), "file")
	checkTestenv(t, os.WriteFile(file, nil, 0o600))
	failed := errors.New("git unavailable")
	for _, tc := range []struct {
		name                     string
		execPath                 string
		hooks                    map[string]string
		wrapperErr, namespaceErr error
		sameInvalidPath          bool
		used                     bool
	}{
		{name: "equal", execPath: resources.execPath + "\n", hooks: resources.hooks, used: true},
		{name: "mirrored exec path", execPath: "mirror", hooks: resources.hooks, used: true},
		{name: "different exec path", execPath: t.TempDir(), hooks: resources.hooks},
		{name: "missing directory", execPath: missing, hooks: resources.hooks, sameInvalidPath: true},
		{name: "not a directory", execPath: file, hooks: resources.hooks, sameInvalidPath: true},
		{name: "empty exec path", hooks: resources.hooks, sameInvalidPath: true},
		{name: "relative exec path", execPath: ".", hooks: resources.hooks, sameInvalidPath: true},
		{name: "missing hooks", execPath: resources.execPath},
		{name: "different hook names", execPath: resources.execPath, hooks: map[string]string{"other.sample": "template\n"}},
		{name: "different hook contents", execPath: resources.execPath, hooks: map[string]string{"hook.sample": "other"}},
		{name: "wrapper fails", execPath: resources.execPath, hooks: resources.hooks, wrapperErr: failed},
		{name: "namespace fails", execPath: resources.execPath, hooks: resources.hooks, namespaceErr: failed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			prefix := filepath.Join(root, "git-prefix")
			link := filepath.Join(prefix, "bin", "git")
			calls := map[string]int{}
			const inherited = "/fixture/bin:/usr/bin"
			path := namespaceGitPath(root, target, inherited, func(git, probeRoot string) (gitResources, error) {
				if probeRoot != root {
					t.Fatalf("probe root = %q, want %q", probeRoot, root)
				}
				calls[git]++
				if git == "/usr/bin/git" {
					wrapper := resources
					if tc.sameInvalidPath {
						wrapper.execPath = tc.execPath
					}
					return wrapper, tc.wrapperErr
				}
				if git != link {
					t.Fatalf("probed %q instead of the namespace git", git)
				}
				actual, err := os.Readlink(link)
				checkTestenv(t, err)
				if actual != target {
					t.Fatalf("symlink target = %q, want %q", actual, target)
				}
				for _, name := range []string{"libexec", "share", "etc"} {
					actual, err := os.Readlink(filepath.Join(prefix, name))
					checkTestenv(t, err)
					if want := filepath.Join(filepath.Dir(filepath.Dir(target)), name); actual != want {
						t.Fatalf("%s target = %q, want %q", name, actual, want)
					}
				}
				execPath := tc.execPath
				if execPath == "mirror" {
					execPath = filepath.Join(prefix, "libexec", "git-core")
				}
				return gitResources{execPath: execPath, hooks: tc.hooks}, tc.namespaceErr
			})
			want := inherited
			if tc.used {
				want = filepath.Dir(link) + string(os.PathListSeparator) + inherited
				entries, err := os.ReadDir(filepath.Dir(link))
				checkTestenv(t, err)
				if len(entries) != 1 || entries[0].Name() != "git" {
					t.Fatalf("PATH exposes other tools: %v", entries)
				}
			} else if _, err := os.Stat(prefix); !os.IsNotExist(err) {
				t.Fatalf("rejected prefix was not removed: %v", err)
			}
			if path != want {
				t.Fatalf("PATH = %q, want %q", path, want)
			}
			if calls["/usr/bin/git"] != 1 || calls[link] != 1 {
				t.Fatalf("resource probes = %v, want each git once", calls)
			}
		})
	}
}

func TestNamespaceGitResourceDirectories(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"libexec", "share", "etc"} {
		t.Run(name, func(t *testing.T) {
			target, resources := fixtureGitInstallation(t)
			checkTestenv(t, os.RemoveAll(filepath.Join(filepath.Dir(filepath.Dir(target)), name)))
			root := t.TempDir()
			calls := 0
			got := namespaceGitPath(root, target, "/usr/bin", func(string, string) (gitResources, error) {
				calls++
				return resources, nil
			})
			if name == "etc" {
				if got == "/usr/bin" || calls != 2 {
					t.Fatalf("optional etc prevented namespace: PATH=%q probes=%d", got, calls)
				}
			} else if got != "/usr/bin" || calls != 0 {
				t.Fatalf("missing %s did not fall back before probes: PATH=%q probes=%d", name, got, calls)
			}
		})
	}
}

func TestInspectGitResources(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"", "--exec-path", "submodule", "help", "init", "missing hooks", "empty hooks", "unreadable hook"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			got, err := inspectGitResources(root, func(args ...string) ([]byte, error) {
				if args[0] == failure {
					return nil, errors.New("probe command failed")
				}
				switch args[0] {
				case "--exec-path":
					return []byte("/developer/usr/libexec/git-core\n"), nil
				case "submodule":
					if failure == "help" {
						return []byte("different command"), nil
					}
					return []byte("usage: git submodule"), nil
				case "init":
					if failure == "missing hooks" {
						return nil, nil
					}
					hooks := filepath.Join(args[2], ".git", "hooks")
					checkTestenv(t, os.MkdirAll(hooks, 0o700))
					if failure == "empty hooks" {
						return nil, nil
					}
					if failure == "unreadable hook" {
						checkTestenv(t, os.Mkdir(filepath.Join(hooks, "hook.sample"), 0o700))
					} else {
						checkTestenv(t, testexec.WriteFile(filepath.Join(hooks, "hook.sample"), []byte("template"), 0o700))
					}
					return nil, nil
				default:
					t.Fatalf("unexpected command: %v", args)
					return nil, nil
				}
			})
			if failure != "" {
				if err == nil {
					t.Fatalf("%s was accepted: %+v", failure, got)
				}
			} else if err != nil || got.execPath != "/developer/usr/libexec/git-core\n" || !reflect.DeepEqual(got.hooks, map[string]string{"hook.sample": "template"}) {
				t.Fatalf("resources = %+v, error = %v", got, err)
			}
		})
	}
}

func TestGitProbeEnvironment(t *testing.T) {
	t.Parallel()
	got := gitProbeEnvironment([]string{
		"PATH=/usr/bin", "DEVELOPER_DIR=/Library/Developer/CommandLineTools",
		"HOME=/person", "XDG_CONFIG_HOME=/person/config", "GIT_EXEC_PATH=/override",
		"GIT_TEMPLATE_DIR=/override", "GIT_CONFIG_SYSTEM=/override", "GIT_CONFIG_GLOBAL=/override",
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=init.templateDir", "GIT_CONFIG_VALUE_0=/override",
	}, "/probe")
	want := []string{"PATH=/usr/bin", "DEVELOPER_DIR=/Library/Developer/CommandLineTools",
		"HOME=/probe", "XDG_CONFIG_HOME=/probe", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("probe environment = %v, want %v", got, want)
	}
}

// Documentation directories may be absent in Apple's installation. Resolve
// the existing ancestor so both entries must still name the same missing path.
func canonicalGitResource(t *testing.T, path string) string {
	t.Helper()
	path = strings.TrimSpace(path)
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved
		}
		if !os.IsNotExist(err) || filepath.Dir(path) == path {
			t.Fatalf("resolve resource %q: %v", path, err)
		}
		suffix = append(suffix, filepath.Base(path))
		path = filepath.Dir(path)
	}
}
