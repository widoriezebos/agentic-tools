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
		if cause.Commands < 1 || rendered.Commands != cause.Commands || commands != cause.Commands {
			t.Errorf("cause outcome token=%q kind=%q declares %d commands, rendered %d in %q", cause.Token, cause.Kind, cause.Commands, commands, rendered.Remedy)
		}
		if strings.Contains(rendered.Remedy, "reset --hard") {
			t.Errorf("cause outcome token=%q kind=%q destroys checkout state: %q", cause.Token, cause.Kind, rendered.Remedy)
		}
	}
}

func TestRefusalQuotesPathFacts(t *testing.T) {
	err := Refuse("not-enrolled", []Fact{Path("linked-worktree", "/tmp/a'b")}, "enrollment absent")
	if err == nil || !strings.Contains(err.Error(), `linked-worktree='/tmp/a'\''b'`) || !strings.Contains(err.Error(), `cd '/tmp/a'\''b'`) {
		t.Fatalf("path fact was not shell-quoted in the refusal and remedy: %v", err)
	}
}
