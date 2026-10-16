package project

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

// These Git adapter tests need actual detached checkouts to prove every home
// and the ledger come from the candidate, including trees with no installation.
func TestPublicationTreeHomesGitAdapter(t *testing.T) {
	t.Parallel()
	for _, layout := range []string{"root", "self-hosted", "adopted", "no-configuration"} {
		t.Run(layout, func(t *testing.T) {
			t.Parallel()
			installation, state := publicationTreeLayout(layout)
			homes := []string{"docs/intent", "docs/doctrine", "docs/decisions", "plans/designs", "plans/reviews"}
			paths := make([]string, 0, len(homes)+3)
			for _, home := range homes {
				paths = append(paths, filepath.Join(state, home, "change.md"))
			}
			if layout == "self-hosted" {
				paths = append(paths, "plans/designs/change.md", "metasystem/plans/change-design.md")
			}
			paths = append(paths, filepath.Join(state, "memory/questions.md"))
			for _, path := range paths {
				t.Run(filepath.ToSlash(path), func(t *testing.T) {
					t.Parallel()
					f := newPublicationTreeFixture(t, layout, installation, state)
					root := filepath.Join(f.Top, installation)
					for _, reference := range []string{"held", "missing"} {
						body := fmt.Sprintf("# Change\n\n- Kind: design\n- Id: change\n- Status: accepted\n- Goals: %s\n", reference)
						kind := "design"
						if strings.HasSuffix(path, "questions.md") {
							body = "| id | opened | question | goals | status |\n| --- | --- | --- | --- | --- |\n" +
								fmt.Sprintf("| Q-1 | 2026-10-15 | What changes? | %s | open |\n", reference)
							kind = "question"
						}
						f.Git("checkout", "-q", "--detach", f.Base)
						f.Write(path, body)
						candidate := f.Commit()
						f.Git("checkout", "-q", "--detach", "main")
						err := CheckCommit(root, candidate)
						if layout == "self-hosted" {
							fromCheckout := CheckCommit(f.Top, candidate)
							if fmt.Sprint(fromCheckout) != fmt.Sprint(err) {
								t.Fatalf("checkout root omitted nested homes: %v; installation root: %v", fromCheckout, err)
							}
						}
						if reference == "held" {
							if err != nil {
								t.Fatalf("candidate ledger's goal refused: %v", err)
							}
						} else {
							want := fmt.Sprintf("the %s page %s names goal missing, which is not in the ledger; the push waits — open the goal or fix the page", kind, filepath.Base(path))
							if err == nil || strings.Join(strings.Fields(err.Error()), " ") != want {
								t.Fatalf("candidate home %s: got %v; want %s", path, err, want)
							}
						}
					}
				})
			}
		})
	}
}

func publicationTreeLayout(layout string) (installation, state string) {
	switch layout {
	case "self-hosted":
		return "metasystem", "metasystem"
	case "adopted":
		return "vendor/metasystem", "."
	default:
		return ".", "."
	}
}

func newPublicationTreeFixture(t *testing.T, layout, installation, state string) *testgoal.PublicationFixture {
	t.Helper()
	top, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &testgoal.PublicationFixture{T: t, Top: top, Root: top}
	f.Git("init", "-q", "-b", "main")
	f.Git("config", "user.name", "fixture")
	f.Git("config", "user.email", "fixture@invalid")
	f.Git("config", "core.hooksPath", "/dev/null")
	if layout != "no-configuration" {
		conf := "metasystem.runtimes=fake\n"
		if layout == "self-hosted" {
			conf += "metasystem.template=true\n"
		}
		f.Write(filepath.Join(installation, "metasystem.conf"), conf)
	}
	f.Write(filepath.Join(state, "plans/goals/held.md"), string(goal.RenderFile(&goal.GoalFile{Id: "held", State: goal.StateApproved, Intent: "Fixture goal", Revision: 1})))
	// The caller has no held goal: only the candidate's ledger can admit it.
	f.Base = f.Commit()
	f.Git("rm", "-q", filepath.Join(state, "plans/goals/held.md"))
	f.Commit()
	f.Git("checkout", "-q", "--detach", f.Base)
	return f
}

func TestPublicationUnreadableCandidateGitAdapter(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ commit, cause string }{
		{"not-a-commit", `"not-a-commit" is not a commit id`},
		{strings.Repeat("a", 40), "worktree"},
	} {
		t.Run(c.commit, func(t *testing.T) {
			t.Parallel()
			f := newPublicationTreeFixture(t, "no-configuration", ".", ".")
			err := CheckCommit(f.Top, c.commit)
			if err == nil {
				t.Fatal("unreadable candidate passed")
			}
			message := err.Error()
			if !strings.Contains(message, "the records could not be read: ") || !strings.Contains(message, c.cause) ||
				!strings.Contains(message, "; the push waits — retry the publication") || strings.Contains(message, "fix the page") {
				t.Fatalf("environmental refusal lost its cause or remedy: %v", err)
			}
		})
	}
}
