package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// blockedRule is the one paragraph a blocked agent follows. Every command
// it spells must be one this engine declares.
const blockedRule = "**When you are blocked, ask.** If a refusal or failure stops your work and the one obvious next step (the command the refusal names, or one retry) did not clear it, do not loop, guess or stop silently. Run `metasystem question ask GOAL --question \"<the decision you need>\" --fact \"<refusal line 1>\" --fact \"<what you tried>\" --option \"<label>: <consequence>\"...` (with no goal: `--about lane` or `--about machine` in place of GOAL), then `metasystem question wait channel:Q` in the background and end your turn. The answer is a decision, the person having done the act, or a bypass; act on it. If the block clears meanwhile, `question withdraw Q`. Work on nothing the answer could invalidate."

// blockedDelegateLine is what a delegate does instead: its seat asks.
const blockedDelegateLine = "When blocked, stop and return `BLOCKED:` with the refusal's first line, what you tried and the decision needed; your seat asks."

// TestBlockedAgentRuleIsWhereItBelongs: the guide for working with agents
// carries the blocked-agent rule and AGENTS.md the short line pointing to
// it; every command the rule spells is declared with its flags; the landing agent's skill names
// case 8; both delegate brief templates carry the delegate's line.
func TestBlockedAgentRuleIsWhereItBelongs(t *testing.T) {
	t.Parallel()
	read := func(parts ...string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(append([]string{"..", ".."}, parts...)...))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if guide := read("docs", "working-with-agents.md"); !strings.Contains(guide, "\n"+blockedRule+"\n") {
		t.Error("docs/working-with-agents.md does not carry the blocked-agent rule as one paragraph")
	}
	if agents := read("AGENTS.md"); !strings.Contains(agents, "`metasystem question ask`") || !strings.Contains(agents, "docs/working-with-agents.md") {
		t.Error("AGENTS.md does not carry the short ask-on-block line pointing to the guide")
	}
	for _, spelled := range blockedRuleCommands(blockedRule) {
		words := strings.Fields(spelled)
		command, ok := findIntentAction(words[1], words[2])
		if !ok {
			t.Errorf("the rule spells %q, which this engine does not declare", spelled)
			continue
		}
		for _, word := range words[3:] {
			if name, flag := strings.CutPrefix(word, "--"); flag {
				if _, ok := command.lookupFlag(name); !ok {
					t.Errorf("the rule spells --%s, but %s takes none", name, command.name)
				}
			}
		}
	}
	if skill := read("skills", "landing-agent", "SKILL.md"); !strings.Contains(skill, "9. **Blocked outside cases 1-8:** ask with `--about lane`, then end your turn; the keeper holds you\n   until it is answered.") {
		t.Error("the landing-agent skill does not carry case 8")
	}
	for _, template := range []string{"brief.md", "design-brief.md"} {
		if !strings.Contains(read("internal", "protocol", "templates", template), blockedDelegateLine) {
			t.Errorf("%s does not carry the delegate's blocked line", template)
		}
	}
}

// blockedRuleCommands is each metasystem command the rule spells in
// backticks, with its flags, plus the --about flag the rule names apart.
func blockedRuleCommands(rule string) []string {
	var commands []string
	spans := strings.Split(rule, "`")
	for index := 1; index < len(spans); index += 2 {
		if strings.HasPrefix(spans[index], "metasystem ") {
			commands = append(commands, spans[index])
		}
	}
	if strings.Contains(rule, "`--about lane`") {
		commands = append(commands, "metasystem question ask --about")
	}
	return commands
}
