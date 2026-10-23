package branch

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReaderRecordChangedBeforeFinalStage(t *testing.T) {
	t.Parallel()
	f := newAttestationPolicyFixture(t, "metasystem/code.go", false)
	original := f.writeReaderRecord()
	edited := append(append([]byte(nil), original...), []byte("Correction: this unit remains unread.\n")...)
	f.expectStart(f.root)
	f.expectAfterGate(f.root, false)
	// A changed record must refuse before staging or installing the read.
	f.expected = f.expected[:len(f.expected)-2]
	f.expect("RestoreIndex", f.root, f.index)
	effects := f.effects()
	build := effects.Build
	effects.Build = func(req CommitRequest, state commitBranchState, subject, trailer string, patch []byte) (string, error) {
		tip, err := build(req, state, subject, trailer, patch)
		f.write(f.record, edited)
		return tip, err
	}
	tip, _, err := commitRead(f.request(false), f, effects)
	want := "the reader record changed while it was being recorded; nothing was recorded; run the command again\nrun: metasystem work review --commit " + f.unit + " --goal goal-a --reader-record " + f.record
	var refusal *OpError
	if !errors.As(err, &refusal) || refusal.Code != ReadInvalidCode || refusal.Message != want || tip != "" {
		t.Fatalf("changed record accepted: tip=%q err=%v", tip, err)
	}
	data, readErr := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(f.record)))
	if readErr != nil || !bytes.Equal(data, edited) {
		t.Fatalf("edited record lost: %q %v", data, readErr)
	}
	if len(f.snapshots) != 0 {
		t.Fatalf("refusal installed a read: %+v", f.snapshots)
	}
	for path := range f.generated {
		if _, err := os.Stat(filepath.Join(f.root, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("refusal left generated file %s: %v", path, err)
		}
	}
}

// The record check resolves the repository root through the Git adapter.
func TestReaderRecordInputGitAdapter(t *testing.T) {
	t.Parallel()
	subject := AttestationSubject{Commit: policyID("a"), UnitDigest: policyHash([]byte("change"))}
	for _, mode := range []string{"valid", "mixed-case", "empty", "unsafe", "missing", "no-commit", "no-digest", "correct", "trailing-correction", "not-a-repository"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if mode != "not-a-repository" {
				if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
					t.Fatalf("git init: %s %v", out, err)
				}
			}
			path := "metasystem/records/misc/read.md"
			text := subject.Commit + "\n" + subject.UnitDigest + "\nVerdict: LAND\n\n"
			switch mode {
			case "mixed-case":
				text = subject.Commit + "\n" + subject.UnitDigest + "\n  vErDiCt: LaNd \n \n"
			case "empty":
				text = "\n \n"
			case "unsafe":
				path = "../outside.md"
			case "no-commit":
				text = subject.UnitDigest + "\nVerdict: LAND\n"
			case "no-digest":
				text = subject.Commit + "\nVerdict: LAND\n"
			case "correct":
				text = subject.Commit + "\n" + subject.UnitDigest + "\nVerdict: CORRECT\n"
			case "trailing-correction":
				text += "Correction: remains unread.\n"
			}
			if mode != "missing" && mode != "unsafe" {
				absolute := filepath.Join(root, filepath.FromSlash(path))
				if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(absolute, []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			err := checkReaderRecord(BranchReadRequest{Repo: root, GoalID: "goal-a", ReaderRecord: path}, subject)
			if mode == "valid" || mode == "mixed-case" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if mode == "not-a-repository" {
				if err == nil {
					t.Fatal("non-repository accepted")
				}
				return
			}
			var refusal *OpError
			if !errors.As(err, &refusal) || refusal.Code != ReadInvalidCode || !strings.Contains(refusal.Message, "run: metasystem work review --commit "+subject.Commit) {
				t.Fatalf("record input refusal: %v", err)
			}
		})
	}
}
