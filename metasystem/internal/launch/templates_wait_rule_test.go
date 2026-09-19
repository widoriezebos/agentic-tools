package launch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBriefsAndProfilesCarryTheWaitRule(t *testing.T) {
	t.Parallel()

	const profileSentence = "A subagent never waits longer than 240 seconds in one tool call; any longer wait runs as `metasystem wait` in a background task of the main session, which is woken when it ends."
	const templateSentence = "No single command may wait longer than 240 seconds; run a longer one in the background with its output to a file and poll the file. A wait that outlives the turn is registered with `metasystem wait register`."

	templateDirectory := filepath.Join("..", "..", "scripts", "agents", "templates")
	for _, name := range []string{"brief.md", "review-brief.md"} {
		path := filepath.Join(templateDirectory, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s does not carry the wait rule: %v", path, err)
		} else if !strings.Contains(string(data), templateSentence) {
			t.Errorf("%s does not carry the wait rule", path)
		}
	}

	profileDirectory := filepath.Join("..", "..", "..", ".claude", "agents")
	paths, err := filepath.Glob(filepath.Join(profileDirectory, "*.md"))
	if err != nil {
		t.Errorf("%s profiles cannot be listed: %v", profileDirectory, err)
	}
	if len(paths) == 0 {
		t.Errorf("%s contains no profiles", profileDirectory)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s does not carry the wait rule: %v", path, err)
		} else if !strings.Contains(string(data), profileSentence) {
			t.Errorf("%s does not carry the wait rule", path)
		}
	}
}
