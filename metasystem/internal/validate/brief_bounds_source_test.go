package validate

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

type briefSourceFixture struct {
	root, job string
	body      []byte
	prompt    []byte
	encoded   []byte
	record    dispatch.CompositionRecord
}

func composeBriefSource(t *testing.T, body []byte) briefSourceFixture {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	brief, prompt, composition := filepath.Join(temp, "brief.md"), filepath.Join(temp, "prompt.md"), filepath.Join(temp, "composition.json")
	if err := os.WriteFile(brief, body, 0o644); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(root, "artifacts", "agents", "test-brief-bounds-source-"+strings.ReplaceAll(t.Name(), "/", "-"), "staged")
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(stage)) })
	record, err := dispatch.ComposeRolePacket(dispatch.ComposeRolePacketParams{Root: root, Role: "verifier", Brief: brief, JobID: "bounds-r2", Runtime: "fake", Model: "fake-model", Round: 2, DestructiveReach: dispatch.HazardMechanical, ToolPolicy: "read-only", Output: prompt, CompositionOutput: composition, StageDir: stage, ReferenceDir: stage})
	if err != nil {
		t.Fatal(err)
	}
	promptBytes, err := os.ReadFile(prompt)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(composition)
	if err != nil {
		t.Fatal(err)
	}
	return briefSourceFixture{root: root, job: "bounds-r2", body: body, prompt: promptBytes, encoded: encoded, record: record}
}

func checkBriefSource(t *testing.T, f briefSourceFixture, record dispatch.CompositionRecord, prompt []byte) (dispatch.CompositionSource, *dispatch.CompositionReference, error) {
	t.Helper()
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return validateBriefBoundsSource(f.root, f.job, "2", encoded, prompt)
}

func requireBoundsUnreadable(t *testing.T, err error, detail string) {
	t.Helper()
	var typed *BriefBoundsUnreadable
	if !errors.As(err, &typed) || typed.Detail != detail || err.Error() != "BRIEF_BOUNDS_UNREADABLE: "+detail {
		t.Fatalf("error = %#v, want BRIEF_BOUNDS_UNREADABLE: %s", err, detail)
	}
}

func TestReviewBriefBoundsRoundIsolation(t *testing.T) {
	f := composeBriefSource(t, []byte("inline\n"))
	tests := []struct {
		name, detail string
		mutate       func(*dispatch.CompositionRecord)
		ok           bool
	}{
		{"job", `composition jobId "other" does not match job "bounds-r2"`, func(r *dispatch.CompositionRecord) { r.JobID = "other" }, false},
		{"round", "composition round 3 does not match round 2", func(r *dispatch.CompositionRecord) { r.Round = 3 }, false},
		{"missing-source", "composition has no task-direction source", func(r *dispatch.CompositionRecord) { r.Sources = r.Sources[1:] }, false},
		{"duplicate-source", "composition has multiple task-direction sources", func(r *dispatch.CompositionRecord) { r.Sources = append(r.Sources, r.Sources[0]) }, false},
		{"wrong-source", `task-direction source is "engine:brief", want caller:brief`, func(r *dispatch.CompositionRecord) { r.Sources[0].Source = "engine:brief" }, false},
		{"other-slot", "", func(r *dispatch.CompositionRecord) {
			other := r.Sources[0]
			other.Slot = "other"
			r.Sources = append(r.Sources, other)
		}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := f.record
			r.Sources = append([]dispatch.CompositionSource(nil), r.Sources...)
			test.mutate(&r)
			_, _, err := checkBriefSource(t, f, r, f.prompt)
			if test.ok && err != nil {
				t.Fatal(err)
			} else if !test.ok {
				requireBoundsUnreadable(t, err, test.detail)
			}
		})
	}
	for name, raw := range map[string][]byte{"decode-unknown": bytes.Replace(f.encoded, []byte("{"), []byte(`{"unknown":true,`), 1), "decode-multiple": append(append([]byte(nil), f.encoded...), []byte("{}")...)} {
		t.Run(name, func(t *testing.T) {
			_, _, err := validateBriefBoundsSource(f.root, f.job, "2", raw, f.prompt)
			var typed *BriefBoundsUnreadable
			if !errors.As(err, &typed) || !strings.HasPrefix(typed.Detail, "decode composition: ") {
				t.Fatalf("error = %#v", err)
			}
		})
	}
	t.Run("round-text", func(t *testing.T) {
		_, _, err := validateBriefBoundsSource(f.root, f.job, "0", f.encoded, f.prompt)
		requireBoundsUnreadable(t, err, `round "0" is not a positive integer`)
	})
}

func TestReviewBriefBoundsRejectsCorruptSource(t *testing.T) {
	f := composeBriefSource(t, []byte("inline\n"))
	for _, name := range []string{"range", "delivered-digest"} {
		t.Run(name, func(t *testing.T) {
			r := f.record
			r.Sources = append([]dispatch.CompositionSource(nil), r.Sources...)
			if name == "range" {
				r.Sources[0].EndByte = len(f.prompt) + 1
			} else {
				r.Sources[0].DeliveredDigest = strings.Repeat("0", 64)
			}
			_, _, err := checkBriefSource(t, f, r, f.prompt)
			if name == "range" {
				requireBoundsUnreadable(t, err, "task-direction range 0:"+strconv.Itoa(len(f.prompt)+1)+" is outside prompt length "+strconv.Itoa(len(f.prompt)))
			} else {
				requireBoundsUnreadable(t, err, "task-direction delivered digest is "+strings.Repeat("0", 64)+", want "+sourceDigest(f.prompt[r.Sources[0].StartByte:r.Sources[0].EndByte]))
			}
		})
	}
}

func TestReviewBriefBoundsInlineIdentity(t *testing.T) {
	f := composeBriefSource(t, []byte("inline\n"))
	for _, name := range []string{"bytes", "digest", "envelope"} {
		t.Run(name, func(t *testing.T) {
			r, prompt := f.record, append([]byte(nil), f.prompt...)
			r.Sources = append([]dispatch.CompositionSource(nil), r.Sources...)
			s := &r.Sources[0]
			switch name {
			case "bytes":
				s.SourceBytes = s.EndByte - s.StartByte
			case "digest":
				s.SourceDigest = strings.Repeat("0", 64)
			case "envelope":
				prompt[s.StartByte+2] = 'X'
				s.DeliveredDigest = sourceDigest(prompt[s.StartByte:s.EndByte])
			}
			_, _, err := checkBriefSource(t, f, r, prompt)
			want := map[string]string{"bytes": "task-direction inline sourceBytes does not match its body", "digest": "task-direction inline sourceDigest does not match its body", "envelope": "delivered bytes are not the task-direction envelope"}[name]
			requireBoundsUnreadable(t, err, want)
		})
	}
	t.Run("missing-reference", func(t *testing.T) {
		large := composeBriefSource(t, bytes.Repeat([]byte("z"), dispatch.MaxDirectiveBytes+1))
		large.record.References = nil
		_, _, err := checkBriefSource(t, large, large.record, large.prompt)
		requireBoundsUnreadable(t, err, "task-direction inline sourceBytes does not match its body")
	})
	t.Run("no-final-newline", func(t *testing.T) {
		noNewline := composeBriefSource(t, []byte("inline"))
		if _, _, err := checkBriefSource(t, noNewline, noNewline.record, noNewline.prompt); err != nil {
			t.Fatal(err)
		}
	})
}

func TestReviewBriefBoundsRejectsUnboundReference(t *testing.T) {
	body := []byte("source")
	source := dispatch.CompositionSource{Slot: "task-direction", Source: "caller:brief", SourceDigest: sourceDigest(body), SourceBytes: len(body)}
	base := dispatch.CompositionReference{Slot: "task-direction", Path: "artifacts/body.md", OpenPath: "/repo/artifacts/body.md", Digest: source.SourceDigest, Bytes: source.SourceBytes}
	wants := map[string]string{"duplicate": "composition has multiple task-direction references", "digest": "task-direction reference digest does not match sourceDigest", "bytes": "task-direction reference bytes do not match sourceBytes", "path-empty": "task-direction reference path is empty", "path-absolute": "task-direction reference path is absolute", "path-parent": "task-direction reference path has a parent component", "openpath-relative": "task-direction reference openPath is not absolute", "path-suffix": "task-direction reference openPath does not end with its path"}
	for _, name := range []string{"slot", "duplicate", "digest", "bytes", "path-empty", "path-absolute", "path-parent", "openpath-relative", "path-suffix"} {
		t.Run(name, func(t *testing.T) {
			r, refs := base, []dispatch.CompositionReference{base}
			switch name {
			case "slot":
				r.Slot = "other"
			case "duplicate":
				refs = append(refs, base)
			case "digest":
				r.Digest = sourceDigest([]byte("other!"))
			case "bytes":
				r.Bytes++
			case "path-empty":
				r.Path = ""
			case "path-absolute":
				r.Path = "/artifacts/body.md"
			case "path-parent":
				r.Path = "artifacts/../body.md"
			case "openpath-relative":
				r.OpenPath = "repo/artifacts/body.md"
			case "path-suffix":
				r.OpenPath = "/repo/other/body.md"
			}
			refs[0] = r
			got, err := validateBriefSourceReferenceBinding(source, refs)
			if name == "slot" {
				if err != nil || got != nil {
					t.Fatalf("reference = %+v, error = %v", got, err)
				}
				return
			}
			requireBoundsUnreadable(t, err, wants[name])
		})
	}
}

func TestReviewBriefBoundsRejectsCorruptReference(t *testing.T) {
	f := composeBriefSource(t, bytes.Repeat([]byte("z"), dispatch.MaxDirectiveBytes+1))
	for _, name := range []string{"escape", "nonregular", "bytes", "digest"} {
		t.Run(name, func(t *testing.T) {
			r, ref := f.record, f.record.References[0]
			r.References = append([]dispatch.CompositionReference(nil), r.References...)
			_ = os.Remove(ref.OpenPath)
			if err := os.WriteFile(ref.OpenPath, f.body, 0o644); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "escape":
				ref.Path = "outside/task-direction.md"
				ref.OpenPath = filepath.Join(filepath.Dir(f.root), "brief-bounds-outside", ref.Path)
			case "nonregular":
				target := filepath.Join(t.TempDir(), "body")
				if err := os.WriteFile(target, f.body, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(ref.OpenPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, ref.OpenPath); err != nil {
					t.Fatal(err)
				}
			case "bytes":
				if err := os.WriteFile(ref.OpenPath, append(append([]byte(nil), f.body...), 'x'), 0o644); err != nil {
					t.Fatal(err)
				}
			case "digest":
				changed := append([]byte(nil), f.body...)
				changed[0] = 'x'
				if err := os.WriteFile(ref.OpenPath, changed, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			r.References[0] = ref
			_, mismatch := dispatch.ReadVerifiedReference(f.root, ref)
			if name == "escape" && mismatch.Found != "escapes-root" {
				t.Fatalf("escape mismatch = %+v", mismatch)
			}
			_, _, err := checkBriefSource(t, f, r, f.prompt)
			requireBoundsUnreadable(t, err, mismatch.Line())
		})
	}
}

func TestReviewBriefBoundsReferencedSource(t *testing.T) {
	f := composeBriefSource(t, bytes.Repeat([]byte("z"), dispatch.MaxDirectiveBytes+1))
	source, reference, err := checkBriefSource(t, f, f.record, f.prompt)
	if err != nil || reference == nil || source.SourceDigest != reference.Digest {
		t.Fatalf("source = %+v, reference = %+v, error = %v", source, reference, err)
	}
}
