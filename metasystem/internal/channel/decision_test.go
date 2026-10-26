package channel

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDecisionMatchesAcrossProse(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"same-decision-other-facts", "other-decision-same-facts", "closed-not-retired", "answered-not-retired", "retired", "persisted", "legacy-facts", "unreadable-record"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			provider := &testProvider{}
			decision := Decision{Kind: "degraded-tick", Subject: "checkout-a", State: "evidence-store"}
			request := AskRequest{RepoRoot: root, About: "machine", Kind: "other", Machine: "m", Facts: []string{"The evidence file is torn."}, Decision: &decision, Provider: provider, Now: time.Date(2031, 1, 1, 9, 0, 0, 0, time.UTC)}
			if name == "legacy-facts" {
				request.Decision = nil
			}
			first, existing, err := AskOrFind(request)
			if err != nil || existing {
				t.Fatalf("first ask: %+v existing=%t error=%v", first, existing, err)
			}
			if name == "persisted" {
				stored, err := ReadQuestion(root, first.ID)
				if err != nil || stored.Decision == nil || *stored.Decision != decision {
					t.Fatalf("persisted decision = %+v, error=%v", stored.Decision, err)
				}
				raw, err := os.ReadFile(questionPath(root, first.ID))
				if err != nil || bytes.Contains(raw, []byte(`"retiredAt"`)) {
					t.Fatalf("zero retirement must be omitted: %s, %v", raw, err)
				}
				return
			}
			switch name {
			case "same-decision-other-facts":
				request.Facts = []string{"The same store cannot be read: another explanation."}
			case "other-decision-same-facts":
				other := decision
				other.State = "progress-marks"
				request.Decision = &other
			case "closed-not-retired", "answered-not-retired", "retired":
				first.State = "closed"
				if name == "answered-not-retired" {
					first.State = "answered"
				}
				if name == "retired" {
					first.RetiredAt = request.Now.Add(time.Minute)
				}
				if err := writeJSON(questionPath(root, first.ID), first); err != nil {
					t.Fatal(err)
				}
			case "legacy-facts":
				request.Decision = &decision
			case "unreadable-record":
				if err := os.WriteFile(filepath.Join(channelRoot(root), "questions", "broken.json"), []byte("{torn"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(questionPath(root, first.ID))
			if err != nil {
				t.Fatal(err)
			}
			again, existing, err := AskOrFind(request)
			files, readErr := os.ReadDir(filepath.Join(channelRoot(root), "questions"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if name == "unreadable-record" {
				if err == nil || len(files) != 2 || len(provider.posts) != 1 {
					t.Fatalf("strict question read must refuse without another ask: files=%d posts=%d error=%v", len(files), len(provider.posts), err)
				}
				return
			}
			wantExisting := name != "other-decision-same-facts" && name != "retired" && name != "legacy-facts"
			wantCount := 2
			if wantExisting {
				wantCount = 1
			}
			if err != nil || existing != wantExisting || (again.ID == first.ID) != wantExisting || len(files) != wantCount || len(provider.posts) != wantCount {
				t.Fatalf("decision match: id=%s first=%s existing=%t files=%d posts=%d error=%v; want existing=%t count=%d", again.ID, first.ID, existing, len(files), len(provider.posts), err, wantExisting, wantCount)
			}
			after, err := os.ReadFile(questionPath(root, first.ID))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("matching must not rewrite the stored question: %v", err)
			}
		})
	}
	t.Run("refused", func(t *testing.T) {
		t.Parallel()
		for _, field := range []string{"lane", "kind", "subject", "state"} {
			t.Run(field, func(t *testing.T) {
				t.Parallel()
				decision := Decision{Kind: "health", Subject: "runner", State: "dead"}
				request := AskRequest{RepoRoot: t.TempDir(), About: "machine", Kind: "other", Facts: []string{"fact"}, Decision: &decision}
				switch field {
				case "lane":
					request.About = "lane"
				case "kind":
					decision.Kind = ""
				case "subject":
					decision.Subject = ""
				case "state":
					decision.State = ""
				}
				if _, _, err := AskOrFind(request); err == nil || !strings.Contains(err.Error(), "decision") {
					t.Fatalf("invalid decision must be refused: %v", err)
				}
				if questions, err := WalkQuestions(request.RepoRoot); err != nil || len(questions) != 0 {
					t.Fatalf("refusal wrote questions: %+v %v", questions, err)
				}
			})
		}
	})
}
