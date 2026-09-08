package steward

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestIdentityMintVerifyRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	want := InstallIdentity{RepoIdentity: "repo-ulid", Generation: 2, InstallPath: "/opt/x", MintedAt: "2026-08-20T09:00:00Z", Enrollment: EnrollmentFixture}
	if err := MintIdentity(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := VerifyIdentity(path, "repo-ulid")
	if err != nil || got != want {
		t.Fatalf("round trip: %+v %v", got, err)
	}
}

func TestIdentityWithoutEnrollmentReadsAsHumanTerminal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	if err := os.WriteFile(path, []byte(`{"repoIdentity":"repo-ulid","generation":1,"installPath":"/opt/x","mintedAt":"2026-08-20T09:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := VerifyIdentity(path, "repo-ulid")
	if err != nil || got.Enrollment != EnrollmentHumanTerminal {
		t.Fatalf("an identity minted before enrollment provenance must remain a human installation: %+v %v", got, err)
	}
}

func TestForeignRepositoryRefusesByName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	if err := MintIdentity(path, InstallIdentity{RepoIdentity: "other", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIdentity(path, "repo-ulid"); err == nil || !strings.Contains(err.Error(), "serves repository") {
		t.Fatalf("foreign identity must refuse by name: %v", err)
	}
}

func TestGroupReadableIdentityRefuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	if err := MintIdentity(path, InstallIdentity{RepoIdentity: "repo-ulid", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIdentity(path, "repo-ulid"); err == nil || !strings.Contains(err.Error(), "owner-only") {
		t.Fatalf("loose permissions must refuse by name: %v", err)
	}
}

func TestAbsentAndMalformedIdentityRefuse(t *testing.T) {
	dir := t.TempDir()
	if _, err := VerifyIdentity(filepath.Join(dir, "missing.json"), "r"); err == nil {
		t.Fatal("absent identity must refuse")
	}
	torn := filepath.Join(dir, "torn.json")
	if err := os.WriteFile(torn, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIdentity(torn, "r"); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("malformed identity must refuse by name: %v", err)
	}
}

func TestZeroGenerationRefuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	if err := MintIdentity(path, InstallIdentity{RepoIdentity: "r", Generation: 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIdentity(path, "r"); err == nil || !strings.Contains(err.Error(), "generation") {
		t.Fatalf("generation zero must refuse: %v", err)
	}
}

func TestVerifyEnrolledBinaryDetectsByteDrift(t *testing.T) {
	root := canonicalPath(t.TempDir())
	bin := filepath.Join(root, "metasystem")
	if err := os.WriteFile(bin, []byte("accepted engine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	digest, err := installDigest(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
		RepoIdentity: canonicalPath(root), Generation: 3,
		InstallPath: bin, InstallDigest: digest, MintedAt: "2026-08-28T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if installed, err := VerifyEnrolledBinary(root); err != nil || installed.Generation != 3 {
		t.Fatalf("accepted engine did not verify: %+v %v", installed, err)
	}
	if err := os.WriteFile(bin, []byte("changed engine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyEnrolledBinary(root); !errors.Is(err, ErrEnrollmentDrift) {
		t.Fatalf("byte drift was not typed ENROLLMENT_DRIFT: %v", err)
	}
}

func TestPinnedEnrollmentExecutesTheVerifiedInodeAfterPathReplacement(t *testing.T) {
	root := canonicalPath(t.TempDir())
	bin := filepath.Join(root, "metasystem")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf 'accepted-engine\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	digest, err := installDigest(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
		RepoIdentity: root, Generation: 4, InstallPath: bin, InstallDigest: digest,
	}); err != nil {
		t.Fatal(err)
	}
	pinned, err := OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	if err := pinned.PrepareForExecution(); err != nil {
		t.Fatal(err)
	}
	temporary := bin + ".replacement"
	if err := os.WriteFile(temporary, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(temporary, bin); err != nil {
		t.Fatal(err)
	}
	command, err := pinned.Command()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.CombinedOutput()
	if err != nil || string(output) != "accepted-engine\n" {
		t.Fatalf("execution did not use the verified inode: output=%q err=%v", output, err)
	}
}

func TestPinnedEnrollmentRejectsChangedBytesInThePreparedInode(t *testing.T) {
	root := canonicalPath(t.TempDir())
	bin := filepath.Join(root, "metasystem")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	digest, err := installDigest(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
		RepoIdentity: root, Generation: 5, InstallPath: bin, InstallDigest: digest,
	}); err != nil {
		t.Fatal(err)
	}
	pinned, err := OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	if err := pinned.PrepareForExecution(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(pinned.execPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pinned.execPath, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := pinned.Command(); !errors.Is(err, ErrEnrollmentDrift) {
		t.Fatalf("changed prepared inode was not rejected as ENROLLMENT_DRIFT: %v", err)
	}
}

func makeEnrolledBinaryFixture(t *testing.T, generation int, contents []byte) (string, string) {
	t.Helper()
	root := canonicalPath(t.TempDir())
	bin := filepath.Join(root, "metasystem")
	if err := os.WriteFile(bin, contents, 0o755); err != nil {
		t.Fatal(err)
	}
	digest, err := installDigest(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(RepoIdentityPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(root), InstallIdentity{
		RepoIdentity: root, Generation: generation, InstallPath: bin, InstallDigest: digest,
	}); err != nil {
		t.Fatal(err)
	}
	return root, digest
}

func TestConcurrentPreparationKeepsEveryPreparedCommandValid(t *testing.T) {
	contents := append([]byte("#!/bin/sh\n"), bytes.Repeat([]byte("# fixture engine block\n"), 1<<18)...)
	root, _ := makeEnrolledBinaryFixture(t, 6, contents)

	const preparationCount = 16
	binaries := make([]*EnrolledBinary, preparationCount)
	for i := range binaries {
		pinned, err := OpenEnrolledBinary(root)
		if err != nil {
			t.Fatal(err)
		}
		binaries[i] = pinned
		defer pinned.Close()
	}

	type result struct {
		index int
		err   error
	}
	start := make(chan struct{})
	results := make(chan result, preparationCount)
	var preparations sync.WaitGroup
	for i, pinned := range binaries {
		preparations.Add(1)
		go func(index int, binary *EnrolledBinary) {
			defer preparations.Done()
			<-start
			results <- result{index: index, err: binary.PrepareForExecution()}
		}(i, pinned)
	}
	close(start)
	preparations.Wait()
	close(results)
	for result := range results {
		if result.err != nil {
			t.Fatalf("preparation %d failed: %v", result.index, result.err)
		}
	}
	for i, pinned := range binaries {
		if _, err := pinned.Command(); err != nil {
			t.Fatalf("prepared command %d became invalid after overlapping preparation: %v", i, err)
		}
	}
}

func TestConcurrentProcessPreparationKeepsEveryPreparedCommandValid(t *testing.T) {
	contents := append([]byte("#!/bin/sh\n"), bytes.Repeat([]byte("# fixture engine block\n"), 1<<18)...)
	root, _ := makeEnrolledBinaryFixture(t, 9, contents)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	type helperProcess struct {
		command  *exec.Cmd
		output   *bytes.Buffer
		start    *os.File
		ready    *os.File
		release  *os.File
		finished bool
	}
	const helperCount = 8
	helpers := make([]*helperProcess, 0, helperCount)
	t.Cleanup(func() {
		cancel()
		for _, helper := range helpers {
			_ = helper.start.Close()
			_ = helper.ready.Close()
			_ = helper.release.Close()
			if !helper.finished {
				_ = helper.command.Wait()
			}
		}
	})
	for i := 0; i < helperCount; i++ {
		startReader, startWriter, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		readyReader, readyWriter, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		releaseReader, releaseWriter, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		output := &bytes.Buffer{}
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPrepareForExecutionProcessHelper$", "--", root)
		command.Env = append(os.Environ(), "METASYSTEM_STEWARD_PREPARE_HELPER=1")
		command.ExtraFiles = []*os.File{startReader, readyWriter, releaseReader}
		command.Stdout = output
		command.Stderr = output
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		_ = startReader.Close()
		_ = readyWriter.Close()
		_ = releaseReader.Close()
		helpers = append(helpers, &helperProcess{
			command: command, output: output, start: startWriter, ready: readyReader, release: releaseWriter,
		})
	}
	for _, helper := range helpers {
		if err := helper.start.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for i, helper := range helpers {
		if _, err := io.ReadFull(helper.ready, make([]byte, 1)); err != nil {
			t.Fatalf("preparation helper %d did not become ready: %v", i, err)
		}
	}
	for _, helper := range helpers {
		if err := helper.release.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for i, helper := range helpers {
		err := helper.command.Wait()
		helper.finished = true
		if err != nil {
			t.Fatalf("preparation helper %d lost its prepared command: %v; output=%s", i, err, helper.output)
		}
	}
}

func TestPrepareForExecutionProcessHelper(t *testing.T) {
	if os.Getenv("METASYSTEM_STEWARD_PREPARE_HELPER") != "1" {
		return
	}
	if len(os.Args) == 0 {
		t.Fatal("helper root is absent")
	}
	root := os.Args[len(os.Args)-1]
	start := os.NewFile(3, "preparation-start")
	ready := os.NewFile(4, "preparation-ready")
	release := os.NewFile(5, "preparation-release")
	if start == nil || ready == nil || release == nil {
		t.Fatal("helper coordination descriptors are absent")
	}
	defer start.Close()
	defer ready.Close()
	defer release.Close()
	pinned, err := OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.Close()
	if _, err := io.ReadAll(start); err != nil {
		t.Fatal(err)
	}
	if err := pinned.PrepareForExecution(); err != nil {
		t.Fatal(err)
	}
	if _, err := ready.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(release); err != nil {
		t.Fatal(err)
	}
	if _, err := pinned.Command(); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareForExecutionReusesAValidPin(t *testing.T) {
	root, _ := makeEnrolledBinaryFixture(t, 7, []byte("#!/bin/sh\nexit 0\n"))
	first, err := OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if err := first.PrepareForExecution(); err != nil {
		t.Fatal(err)
	}
	second, err := OpenEnrolledBinary(root)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if err := second.PrepareForExecution(); err != nil {
		t.Fatal(err)
	}
	firstInfo, err := first.execFile.Stat()
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := second.execFile.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(firstInfo, secondInfo) {
		t.Fatal("a valid prepared pin was replaced instead of reused")
	}
	if _, err := first.Command(); err != nil {
		t.Fatalf("the original reused command became invalid: %v", err)
	}
	if _, err := second.Command(); err != nil {
		t.Fatalf("the later reused command is invalid: %v", err)
	}
}

func TestPrepareForExecutionRepairsInvalidPins(t *testing.T) {
	contents := []byte("#!/bin/sh\nexit 0\n")
	for _, fixture := range []struct {
		name     string
		contents []byte
		mode     os.FileMode
	}{
		{name: "changed bytes", contents: []byte("changed\n"), mode: 0o500},
		{name: "nonexecutable", contents: contents, mode: 0o400},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root, digest := makeEnrolledBinaryFixture(t, 8, contents)
			pinPath := EnrolledExecutionPath(root, InstallIdentity{Generation: 8, InstallDigest: digest})
			if err := os.MkdirAll(filepath.Dir(pinPath), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(pinPath, fixture.contents, fixture.mode); err != nil {
				t.Fatal(err)
			}
			pinned, err := OpenEnrolledBinary(root)
			if err != nil {
				t.Fatal(err)
			}
			defer pinned.Close()
			if err := pinned.PrepareForExecution(); err != nil {
				t.Fatal(err)
			}
			if _, err := pinned.Command(); err != nil {
				t.Fatalf("repaired pin cannot form a command: %v", err)
			}
			info, err := os.Stat(pinPath)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode()&0o111 == 0 {
				t.Fatal("repaired pin is not executable")
			}
			if repairedDigest, err := installDigest(pinPath); err != nil || repairedDigest != digest {
				t.Fatalf("repaired pin has the wrong digest: %q %v", repairedDigest, err)
			}
		})
	}
}
