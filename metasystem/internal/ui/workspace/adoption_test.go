package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

const recordedSHA = "0123456789abcdef0123456789abcdef01234567"

// TestReadAdoption covers every value the reader reports, including the two
// shapes that are not a SHA: the placeholder the adoption script writes, and a
// line this reader cannot make a SHA of.
func TestReadAdoption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// document is written to docs/project-rules.md unless it is empty and
		// directory is set, which writes a directory at that path instead.
		document  string
		omit      bool
		directory bool
		want      Adoption
	}{
		{name: "no document at all", omit: true, want: Adoption{Record: Absent}},
		{name: "a document with no adoption line", document: "# Project rules\n\n- Something else\n", want: Adoption{Record: Absent}},
		{name: "the placeholder the adoption script writes", document: "- Adopted from template SHA: `<template sha>`\n", want: Adoption{Record: Placeholder}},
		{name: "a recorded template SHA", document: "- Adopted from template SHA: `" + recordedSHA + "`\n", want: Adoption{Record: Recorded, SHA: recordedSHA}},
		{name: "the first adoption line wins", document: "- Adopted from template SHA: `" + recordedSHA + "`\n- Adopted from template SHA: `<template sha>`\n", want: Adoption{Record: Recorded, SHA: recordedSHA}},
		{name: "a line carrying no backticks", document: "- Adopted from template SHA: " + recordedSHA + "\n", want: Adoption{Record: Unreadable}},
		{name: "a line whose backtick never closes", document: "- Adopted from template SHA: `" + recordedSHA + "\n", want: Adoption{Record: Unreadable}},
		{name: "a value that is not hexadecimal", document: "- Adopted from template SHA: `zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz`\n", want: Adoption{Record: Unreadable}},
		{name: "a value that is too short", document: "- Adopted from template SHA: `0123456789abcdef`\n", want: Adoption{Record: Unreadable}},
		{name: "an upper-case value", document: "- Adopted from template SHA: `0123456789ABCDEF0123456789ABCDEF01234567`\n", want: Adoption{Record: Unreadable}},
		{name: "an empty value", document: "- Adopted from template SHA: ``\n", want: Adoption{Record: Unreadable}},
		{name: "carriage returns are not part of the line", document: "- Adopted from template SHA: `" + recordedSHA + "`\r\n", want: Adoption{Record: Recorded, SHA: recordedSHA}},
		{name: "a document that cannot be read", directory: true, want: Adoption{Record: Unreadable}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			installation := t.TempDir()
			document := filepath.Join(installation, "docs", "project-rules.md")
			testutil.Require(t, "create the docs directory", os.MkdirAll(filepath.Dir(document), 0o755), nil)
			switch {
			case tc.directory:
				testutil.Require(t, "create a directory where the document belongs", os.MkdirAll(document, 0o755), nil)
			case !tc.omit:
				testutil.Require(t, "write the document", os.WriteFile(document, []byte(tc.document), 0o644), nil)
			}

			testutil.Expect(t, "adoption", ReadAdoption(installation), tc.want)
		})
	}
}

// TestReadAdoptionOfAnAbsentInstallation proves the reader answers rather than
// failing when the directory it is pointed at does not exist at all.
func TestReadAdoptionOfAnAbsentInstallation(t *testing.T) {
	t.Parallel()

	absent := filepath.Join(t.TempDir(), "no-such-installation")

	testutil.Expect(t, "adoption", ReadAdoption(absent), Adoption{Record: Absent})
}
