package snapshot

import (
	"strings"
	"testing"
)

func TestAGoalTitleStartsAfterTheLabelOfItsIntent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, intent string
		most         int
		want         string
	}{
		{"the label is dropped", "What: the header reads on a phone. Why: it is cut today.", 140, "the header reads on a phone."},
		{"the label on a line of its own", "What:\nthe header reads on a phone", 140, "the header reads on a phone"},
		{"the label in another case", "WHAT: one outcome", 140, "one outcome"},
		{"an intent with no label", "Finish these repairs. Then push.", 140, "Finish these repairs."},
		{"a word that only starts like the label", "Whatever lands first wins", 140, "Whatever lands first wins"},
		{"a label and nothing after it", "What: ", 140, ""},
		{"the caller's own length", "What: " + strings.Repeat("word ", 40) + "end", 24, "word word word word word…"},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			if got := GoalTitle(one.intent, one.most); got != one.want {
				t.Errorf("GoalTitle(%q, %d) = %q, want %q", one.intent, one.most, got, one.want)
			}
		})
	}
}

func TestIntentWordsKeepTheIntentAsWritten(t *testing.T) {
	t.Parallel()
	if got := IntentWords("  What: line one\nline two. Rest.  "); got != "line one\nline two. Rest." {
		t.Errorf("IntentWords kept %q, want the intent after its label, lines and all", got)
	}
}

func TestALedeIsOneSentenceOnOneLine(t *testing.T) {
	t.Parallel()
	if got := Lede("What: spread   over\nlines. Next.", 140); got != "What: spread over lines." {
		t.Errorf("Lede = %q, want the first sentence on one line, label and all", got)
	}
}
