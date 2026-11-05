package main

import (
	"os"
	"strings"
	"testing"
)

func TestSkillSeatStewardMandateInOrder(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../skills/seat-steward/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\nname: seat-steward\n") || !strings.Contains(text, "description:") || !strings.Contains(text, "ask the person only a value question") || len(strings.Fields(text)) > 3000 {
		t.Fatalf("steward mandate or word budget missing (words=%d)", len(strings.Fields(text)))
	}
	last := -1
	for _, command := range []string{
		"`metasystem system situation`",
		"`metasystem decision record --class C -- VERB`",
		"`metasystem agent reply`",
		"`metasystem decision record --row ROW --subject S --occurrence O --dispose act|wait|ask`",
		"`metasystem system situation --wait`",
		"`metasystem agent ask`",
		"`metasystem test run`",
		"`metasystem session handoff`",
	} {
		at := strings.Index(text, command)
		if strings.Count(text, command) != 1 || at <= last {
			t.Fatalf("command %s must occur once in mandate order (at=%d, previous=%d)", command, at, last)
		}
		last = at
	}
}
