package enginecause

import (
	"regexp"
	"strings"
	"testing"
)

func TestCauseOutcomesRenderDeclaredCommandCount(t *testing.T) {
	token := regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	seen := map[string]bool{}
	for _, cause := range Table {
		key := cause.Token + "\x00" + cause.Kind
		if !token.MatchString(cause.Token) || seen[key] {
			t.Errorf("invalid or duplicate cause outcome token=%q kind=%q", cause.Token, cause.Kind)
		}
		seen[key] = true
		facts := append([]Fact(nil), cause.sample...)
		rendered, err := Render(cause.Token, facts)
		if err != nil || strings.TrimSpace(rendered.Remedy) == "" {
			t.Errorf("cause outcome token=%q kind=%q has no remedy: %q %v", cause.Token, cause.Kind, rendered.Remedy, err)
			continue
		}
		commands := strings.Count(rendered.Remedy, " && ") + 1
		if cause.Commands < 1 || rendered.Commands != commands || commands > cause.Commands {
			t.Errorf("cause outcome token=%q kind=%q declares at most %d commands, rendered %d in %q", cause.Token, cause.Kind, cause.Commands, commands, rendered.Remedy)
		}
		if strings.Contains(rendered.Remedy, "reset --hard") {
			t.Errorf("cause outcome token=%q kind=%q destroys checkout state: %q", cause.Token, cause.Kind, rendered.Remedy)
		}
	}
}

func TestFastForwardBlockerRemediesPreserveCheckout(t *testing.T) {
	facts := []Fact{
		Path("tracked-path", "docs/a file.txt"),
		Path("untracked-path", "generated/input"),
		Path("ledger-path", "memory/receipts.log"),
		Path("ledger-path", "records/narrator-digest.log"),
	}
	rendered, err := Render("fast-forward-blocked", facts)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"$(git rev-parse --short HEAD)", "while test -e", "cp -p -n --",
		"git restore --staged --worktree --source=HEAD --",
		"git stash push --include-untracked --", "git stash pop",
		"after the fast-forward, append only the missing saved lines",
	} {
		if !strings.Contains(rendered.Remedy, want) {
			t.Errorf("blocker remedy does not contain %q: %s", want, rendered.Remedy)
		}
	}
	for _, forbidden := range []string{"reset --hard", "git checkout --", "memory/receipts.log.local &&"} {
		if strings.Contains(rendered.Remedy, forbidden) {
			t.Errorf("blocker remedy contains unsafe or fixed operation %q: %s", forbidden, rendered.Remedy)
		}
	}
}

func TestUnprintableBlockerPathsAreCountedNotRendered(t *testing.T) {
	err := Refuse("fast-forward-blocked", []Fact{Path("untracked-path", "dir/line\nbreak")}, "blocked")
	if err == nil || strings.Contains(err.Error(), "line\nbreak") || !strings.Contains(err.Error(), "unprintable=1 see=git-status--porcelain-z") {
		t.Fatalf("unprintable blocker path was not counted and hidden: %v", err)
	}
}

func TestRefusalQuotesPathFacts(t *testing.T) {
	err := Refuse("not-enrolled", []Fact{Path("linked-worktree", "/tmp/a'b")}, "enrollment absent")
	if err == nil || !strings.Contains(err.Error(), `linked-worktree='/tmp/a'\''b'`) || !strings.Contains(err.Error(), `cd '/tmp/a'\''b'`) {
		t.Fatalf("path fact was not shell-quoted in the refusal and remedy: %v", err)
	}
}
