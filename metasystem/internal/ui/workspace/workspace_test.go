package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

// record is the server's own record, the half of the resource that is not read
// from the filesystem.
func record() Record {
	return Record{
		StartedAt:        "2026-09-21T10:11:12Z",
		EngineBuild:      "dev-0123456789ab",
		ExecutableDigest: "sha256:fedcba9876543210",
	}
}

// rootsOf resolves the three roots the way every interface verb does, so the
// fixtures are the paths the server would actually carry.
func rootsOf(t *testing.T, installation string) Roots {
	t.Helper()

	resolved, err := lifecycle.ResolveRoots("", installation)
	testutil.Require(t, "resolve roots", err, nil)
	return Roots{Checkout: resolved.Checkout, Installation: resolved.Installation, StateRoot: resolved.StateRoot}
}

// TestDescribeAcrossLayoutsAndRecords walks every mode against every adoption
// record: the mode comes from the layout, the subject default from the mode,
// and the conflict from both. The self-hosted row with a recorded SHA is D20's
// copied-marker case, the one cell where conflict holds.
func TestDescribeAcrossLayoutsAndRecords(t *testing.T) {
	t.Parallel()

	layouts := []struct {
		name string
		// installation is relative to the application repository.
		installation string
		selfHosted   bool
		wantMode     string
		wantSubject  string
	}{
		{name: "self-hosted", installation: "metasystem", selfHosted: true, wantMode: ModeSelfHosted, wantSubject: SelfHostedSubject},
		{name: "adopted at the application root", installation: ".", wantMode: ModeAdopted, wantSubject: "application"},
		{name: "adopted beneath the application", installation: filepath.Join("vendor", "metasystem"), wantMode: ModeAdopted, wantSubject: "application"},
	}
	records := []struct {
		name            string
		document        string
		omit            bool
		wantRecord      AdoptionRecord
		wantAdoptedFrom string
	}{
		{name: "absent", omit: true, wantRecord: Absent},
		{name: "placeholder", document: "- Adopted from template SHA: `<template sha>`\n", wantRecord: Placeholder},
		{name: "recorded", document: "- Adopted from template SHA: `" + recordedSHA + "`\n", wantRecord: Recorded, wantAdoptedFrom: recordedSHA},
		{name: "unreadable", document: "- Adopted from template SHA: `not a sha`\n", wantRecord: Unreadable},
	}

	for _, layout := range layouts {
		for _, adoption := range records {
			t.Run(layout.name+", "+adoption.name, func(t *testing.T) {
				t.Parallel()

				repository := testRepository(t)
				installation := filepath.Clean(filepath.Join(repository, layout.installation))
				testutil.Require(t, "create the installation", os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755), nil)
				testutil.Require(t, "write the installation's configuration", os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.version=1\n"), 0o644), nil)
				if layout.selfHosted {
					design := filepath.Join(repository, "development", "metasystem-design.md")
					testutil.Require(t, "create the development directory", os.MkdirAll(filepath.Dir(design), 0o755), nil)
					testutil.Require(t, "write the design document", os.WriteFile(design, []byte("# design\n"), 0o644), nil)
				}
				if !adoption.omit {
					document := filepath.Join(installation, "docs", "project-rules.md")
					testutil.Require(t, "create the docs directory", os.MkdirAll(filepath.Dir(document), 0o755), nil)
					testutil.Require(t, "write the project rules", os.WriteFile(document, []byte(adoption.document), 0o644), nil)
				}
				roots := rootsOf(t, installation)

				described, err := Describe(roots, record(), "")

				testutil.Require(t, "describe the workspace", err, nil)
				testutil.Expect(t, "schema version", described.SchemaVersion, SchemaVersion)
				testutil.Expect(t, "mode", described.Mode, layout.wantMode)
				testutil.Expect(t, "subject", described.Subject, layout.wantSubject)
				testutil.Expect(t, "adoption record", described.AdoptionRecord, string(adoption.wantRecord))
				testutil.Expect(t, "adopted from", described.AdoptedFrom, adoption.wantAdoptedFrom)
				testutil.Expect(t, "conflict", described.Conflict, layout.selfHosted && adoption.wantRecord == Recorded)
				testutil.Expect(t, "checkout", described.Checkout, roots.Checkout)
				testutil.Expect(t, "installation", described.Installation, roots.Installation)
				testutil.Expect(t, "state root", described.StateRoot, roots.StateRoot)
				testutil.Expect(t, "engine build", described.EngineBuild, record().EngineBuild)
				testutil.Expect(t, "started at", described.StartedAt, record().StartedAt)
				testutil.Expect(t, "executable digest", described.ExecutableDigest, record().ExecutableDigest)
				testutil.Expect(t, "source at HEAD", described.SourceHead, "")
			})
		}
	}
}

// TestDescribeUsesTheConfiguredSubject proves the default applies only when
// nothing is configured, in both modes.
func TestDescribeUsesTheConfiguredSubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		selfHosted bool
		configured string
		want       string
	}{
		{name: "self-hosted, configured", selfHosted: true, configured: "The engine", want: "The engine"},
		{name: "self-hosted, blank", selfHosted: true, configured: "   ", want: SelfHostedSubject},
		{name: "adopted, configured", configured: "Ledger", want: "Ledger"},
		{name: "adopted, blank", configured: "", want: "application"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repository := testRepository(t)
			installation := repository
			if tc.selfHosted {
				installation = filepath.Join(repository, "metasystem")
				design := filepath.Join(repository, "development", "metasystem-design.md")
				testutil.Require(t, "create the development directory", os.MkdirAll(filepath.Dir(design), 0o755), nil)
				testutil.Require(t, "write the design document", os.WriteFile(design, []byte("# design\n"), 0o644), nil)
			}
			testutil.Require(t, "create the installation", os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755), nil)
			testutil.Require(t, "write the installation's configuration", os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.version=1\n"), 0o644), nil)
			roots := rootsOf(t, installation)

			described, err := Describe(roots, record(), tc.configured)

			testutil.Require(t, "describe the workspace", err, nil)
			testutil.Expect(t, "subject", described.Subject, tc.want)
		})
	}
}

// TestDescribeRefusesAnInstallationItCannotResolve proves the resource fails
// loudly rather than reporting a mode it did not resolve.
func TestDescribeRefusesAnInstallationItCannotResolve(t *testing.T) {
	t.Parallel()

	roots := Roots{Checkout: t.TempDir(), Installation: filepath.Join(t.TempDir(), "no-such-installation"), StateRoot: t.TempDir()}

	_, err := Describe(roots, record(), "")

	testutil.Require(t, "describe refuses", err != nil, true)
}

// testRepository is an initialised Git checkout named "application", so the
// adopted subject default is a name a reader recognises.
func testRepository(t *testing.T) string {
	t.Helper()

	repository := filepath.Join(t.TempDir(), "application")
	output, err := exec.Command("git", "init", "-q", repository).CombinedOutput()
	testutil.Require(t, "git init: "+string(output), err, nil)
	canonical, err := filepath.EvalSymlinks(repository)
	testutil.Require(t, "canonical repository", err, nil)
	return canonical
}
