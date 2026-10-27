package channel

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// buttonProvider is a test provider that also posts buttons.
type buttonProvider struct {
	testProvider
	options [][]string
	texts   []string
	cleared []MessageRef
}

func (p *buttonProvider) PostOptions(ctx context.Context, d DestinationConfig, text string, labels []string, thread *MessageRef) (MessageRef, error) {
	p.options = append(p.options, slices.Clone(labels))
	p.texts = append(p.texts, text)
	return p.Post(ctx, d, text, thread)
}

func (p *buttonProvider) ClearOptions(_ context.Context, _ DestinationConfig, posted MessageRef) error {
	p.cleared = append(p.cleared, posted)
	return nil
}

func TestPostQuestionOffersButtonsOnlyForAContractText(t *testing.T) {
	t.Parallel()
	question := func(contract bool, last string) Question {
		q := Question{Contract: contract, Recommendation: "Continue", Options: []Option{{Label: "Stop here", Consequence: "nothing runs"}, {Label: "Continue", Consequence: "it goes on"}}}
		q.Text = "Decide.\n\nIf you do nothing: it waits.\n" + last
		return q
	}
	codeOffLine := "Reply: Continue / Stop here"
	codeOnLine := "Reply with the word and your code: Continue / Stop here"
	for _, row := range []struct {
		name     string
		q        Question
		codeOff  bool
		plain    bool
		wantTaps bool
	}{
		{"a contract text with the code off offers buttons", question(true, codeOffLine), true, false, true},
		{"a text made with the code on gets none", question(true, codeOnLine), true, false, false},
		{"a legacy question gets none", question(false, codeOffLine), true, false, false},
		{"the code on gets none", question(true, codeOffLine), false, false, false},
		{"a provider without buttons keeps the reply line", question(true, codeOffLine), true, true, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			buttons := &buttonProvider{}
			var p Provider = buttons
			if row.plain {
				p = &buttons.testProvider
			}
			if _, err := postQuestion(context.Background(), p, DestinationConfig{}, row.q, row.codeOff, nil); err != nil {
				t.Fatal(err)
			}
			if !row.wantTaps {
				if len(buttons.options) != 0 || !slices.Equal(buttons.posts, []string{row.q.Text}) {
					t.Fatalf("buttons %q, posts %q; want the stored text alone", buttons.options, buttons.posts)
				}
				return
			}
			want := strings.TrimSuffix(row.q.Text, codeOffLine) + "Tap a button, or reply: Continue / Stop here"
			if len(buttons.options) != 1 || !slices.Equal(buttons.options[0], []string{"Continue", "Stop here"}) || !slices.Equal(buttons.texts, []string{want}) {
				t.Fatalf("buttons %q, texts %q; want one keyboard, recommended first, under %q", buttons.options, buttons.texts, want)
			}
		})
	}
}
