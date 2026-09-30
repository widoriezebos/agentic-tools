package testenv

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The audit's filename check knows every GOOS and GOARCH name the toolchain
// knows: each port `go tool dist list` prints, and the historical names
// go/build still treats as constraints though no port ships them.
func TestBuildConstraintAuditKnowsEveryToolchainName(t *testing.T) {
	t.Parallel()
	output, err := exec.Command("go", "tool", "dist", "list").Output()
	if err != nil {
		t.Fatalf("go tool dist list: %v", err)
	}
	systems := map[string]bool{"hurd": true, "nacl": true, "zos": true}
	architectures := map[string]bool{
		"amd64p32": true, "armbe": true, "arm64be": true, "mips64p32": true, "mips64p32le": true,
		"ppc": true, "riscv": true, "s390": true, "sparc": true, "sparc64": true,
	}
	for _, line := range strings.Fields(string(output)) {
		system, architecture, found := strings.Cut(line, "/")
		if !found {
			t.Fatalf("go tool dist list line %q is not GOOS/GOARCH", line)
		}
		systems[system] = true
		architectures[architecture] = true
	}
	check := func(name, want string) {
		t.Helper()
		file := parseTestFile(t, name, sharedMainFixtureSource)
		if got := testMainConstraint(file); got != want {
			t.Errorf("testMainConstraint(%s) = %q, want %q", name, got, want)
		}
	}
	for system := range systems {
		check("testmain_"+system+"_test.go", "GOOS filename suffix")
	}
	for architecture := range architectures {
		check("testmain_"+architecture+"_test.go", "GOARCH filename suffix")
		check("testmain_linux_"+architecture+"_test.go", "GOARCH filename suffix")
	}
	for _, name := range []string{"testmain_test.go", "shared_main_test.go", "testmain_helper_test.go", "linux_test.go"} {
		check(name, "")
	}
}

// Only a build constraint line before the package clause constrains a file:
// a //go:build or // +build comment after it is plain text to the toolchain.
func TestBuildConstraintAuditReadsOnlyTheHeader(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct {
		label  string
		source string
		want   string
	}{
		{label: "go_build_header", source: "// Copyright.\n\n//go:build !windows\n\n" + sharedMainFixtureSource, want: "//go:build line"},
		{label: "plus_build_header", source: "// +build linux\n\n" + sharedMainFixtureSource, want: "// +build line"},
		{label: "go_build_after_package", source: sharedMainFixtureSource + "\n//go:build darwin\n", want: ""},
		{label: "go_build_in_function", source: strings.Replace(sharedMainFixtureSource, "{ os.Exit", "{\n//go:build darwin\nos.Exit", 1), want: ""},
		{label: "go_build_in_doc_comment", source: strings.Replace(sharedMainFixtureSource, "func TestMain", "// Keep the line\n// //go:build darwin\n// as text.\nfunc TestMain", 1), want: ""},
		{label: "go_build_in_block_comment", source: "/*\n//go:build darwin\n*/\n" + sharedMainFixtureSource, want: ""},
		{label: "go_build_prefix_word", source: "//go:buildx darwin\n\n" + sharedMainFixtureSource, want: ""},
	} {
		t.Run(fixture.label, func(t *testing.T) {
			t.Parallel()
			file := parseTestFile(t, "testmain_test.go", fixture.source)
			if got := testMainConstraint(file); got != fixture.want {
				t.Fatalf("testMainConstraint = %q, want %q", got, fixture.want)
			}
		})
	}
}

// writeStagingOwner makes a staging-shaped directory under root with an
// owner file holding contents, as createRegistryHomeUnder writes it before
// its publishing rename.
func writeStagingOwner(t *testing.T, root, name, contents string) string {
	t.Helper()
	staging := filepath.Join(root, name)
	checkTestenv(t, os.Mkdir(staging, 0o700))
	checkTestenv(t, os.WriteFile(filepath.Join(staging, registryOwnerFile), []byte(contents), 0o600))
	return staging
}

// A creator killed between MkdirTemp and the publishing rename leaves a
// dot-named staging directory; the next sweep removes it once its owner lock
// is free, and keeps every staging directory it cannot prove dead.
func TestSweepRemovesStaleStagingDirectoryOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	nonce := strings.Repeat("a", 2*registryNonceSize)
	stale := writeStagingOwner(t, root, "."+registryHomePrefix+"1234567", nonce)
	checkTestenv(t, os.WriteFile(filepath.Join(stale, "partial"), []byte("x"), 0o600))

	live := writeStagingOwner(t, root, "."+registryHomePrefix+"2345678", nonce)
	var liveLock *os.File
	checkTestenv(t, testexec.Locked(func() (err error) {
		if liveLock, err = os.Open(filepath.Join(live, registryOwnerFile)); err != nil {
			return err
		}
		return unix.Flock(int(liveLock.Fd()), unix.LOCK_SH|unix.LOCK_NB)
	}))
	t.Cleanup(func() { _ = unlockAndClose(liveLock) })

	// The creator locks the owner file before it writes the nonce, so an
	// incomplete nonce may be a live creator that has not locked yet.
	unwritten := writeStagingOwner(t, root, "."+registryHomePrefix+"3456789", "")
	partial := writeStagingOwner(t, root, "."+registryHomePrefix+"4567890", nonce[:10])
	ownerless := filepath.Join(root, "."+registryHomePrefix+"5678901")
	checkTestenv(t, os.Mkdir(ownerless, 0o700))
	notDigits := writeStagingOwner(t, root, "."+registryHomePrefix+"12ab", nonce)
	unrelated := writeStagingOwner(t, root, ".metasystem-other-1234", nonce)

	target := writeStagingOwner(t, root, "link-target", nonce)
	linked := filepath.Join(root, "."+registryHomePrefix+"6789012")
	checkTestenv(t, os.Symlink(target, linked))

	linkedOwner := filepath.Join(root, "."+registryHomePrefix+"7890123")
	checkTestenv(t, os.Mkdir(linkedOwner, 0o700))
	checkTestenv(t, os.Symlink(filepath.Join(target, registryOwnerFile), filepath.Join(linkedOwner, registryOwnerFile)))

	var report bytes.Buffer
	removeDeadRegistryHomes(root, &report)
	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale staging directory survived the sweep: %v; report %q", err, report.String())
	}
	for _, path := range []string{live, unwritten, partial, ownerless, notDigits, unrelated, target, linked, linkedOwner,
		filepath.Join(target, registryOwnerFile), filepath.Join(linkedOwner, registryOwnerFile)} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("sweep removed %s: %v", filepath.Base(path), err)
		}
	}
}

// A home an exited owner kept because a joined helper child still held it
// is orphaned once that child exits. The next Main sweeps its TMPDIR as well
// as /tmp, so a home made under the TMPDIR fallback is removed even when
// /tmp now works (primary stands in for /tmp, fallback for TMPDIR).
func TestCreateRegistryHomeSweepsFallbackRootForOrphanedJoinedHome(t *testing.T) {
	t.Parallel()
	primary := t.TempDir()
	fallback := t.TempDir()

	orphan, err := createRegistryHomeUnder(os.MkdirTemp, fallback)
	checkTestenv(t, err)
	joined, err := createRegistryHomeUnder(os.MkdirTemp, fallback)
	checkTestenv(t, err)
	var children [2]*os.File
	checkTestenv(t, testexec.Locked(func() error {
		for index, home := range []string{orphan.path, joined.path} {
			child, err := openRegistryOwner(home)
			if err != nil {
				return err
			}
			if err := unix.Flock(int(child.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
				_ = child.Close()
				return err
			}
			children[index] = child
		}
		return nil
	}))
	t.Cleanup(func() { _ = unlockAndClose(children[1]) })
	// Both owners exit while their joined children still hold the homes.
	checkTestenv(t, orphan.cleanupReporting(io.Discard))
	checkTestenv(t, joined.cleanupReporting(io.Discard))
	for _, home := range []string{orphan.path, joined.path} {
		if _, err := os.Stat(home); err != nil {
			t.Fatalf("owner removed a home a joined child holds: %v", err)
		}
	}
	// The first child exits; the second lives on.
	checkTestenv(t, unlockAndClose(children[0]))

	registry, err := createRegistryHomeIn(os.MkdirTemp, primary, fallback)
	checkTestenv(t, err)
	t.Cleanup(func() { _ = registry.cleanupReporting(io.Discard) })
	if filepath.Dir(registry.path) != primary {
		t.Fatalf("registry home = %q, want a child of the primary root %q", registry.path, primary)
	}
	if _, err := os.Lstat(orphan.path); !os.IsNotExist(err) {
		t.Fatalf("orphaned joined home under the TMPDIR fallback survived the next Main: %v", err)
	}
	if _, err := os.Stat(joined.path); err != nil {
		t.Fatalf("next Main removed a home a live joined child holds: %v", err)
	}
}
