package partner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Asking what happened (g1-s68).
//
// Every refusal, problem, failed turn, caught throw and failure notification on
// screen can be asked about with one press. The press sends one fixed sentence
// in the human's name, the way a sitting opens with OpeningRequest, and the
// trouble travels beside the page as what the page knew about it: the sentence
// it showed, the code, where, the act as words, when, the tip it had and whether
// the sign-in sheet was the remedy. It never carries the request's arguments —
// there is no field for them — and the page scrubs its secrets from the
// sentence before it is sent.

// TroubleRequest is the one question a press on "Ask what happened" asks.
const TroubleRequest = "What just happened here, why, and how do I recover?"

// MaxTroubleText is the bound on the sentence a trouble carries, in
// characters; maxTroubleField is every other field's.
const (
	MaxTroubleText  = 2000
	maxTroubleField = 200
)

// ErrTroubleTooLong is the refusal a trouble past its bounds gets. Nothing of
// the turn is started and nothing is written.
var ErrTroubleTooLong = errors.New("a trouble is at most 2,000 characters of sentence and 200 of every other field")

// Trouble is one thing that went wrong on screen, as the page knew it.
type Trouble struct {
	// Text is the sentence exactly as the screen showed it, scrubbed.
	Text  string       `json:"text"`
	Code  string       `json:"code,omitempty"`
	Where TroubleWhere `json:"where"`
	// Act is the act that was refused, as words, where there was one.
	Act *TroubleAct `json:"act,omitempty"`
	At  string      `json:"at"`
	Tip string      `json:"tip,omitempty"`
	// SignIn says the sign-in sheet was the remedy the page offered.
	SignIn bool `json:"signIn,omitempty"`
}

// TroubleWhere is where on the page the trouble happened.
type TroubleWhere struct {
	Section string `json:"section"`
	Path    string `json:"path"`
	Subject string `json:"subject,omitempty"`
	Kind    string `json:"kind,omitempty"`
}

// TroubleAct is the act a refusal refused: the verb as the button says it, the
// kind of thing it acts on, and which one.
type TroubleAct struct {
	Verb   string `json:"verb"`
	Object string `json:"object"`
	Target string `json:"target,omitempty"`
}

// Bound holds a trouble to what this server will carry and keep, or refuses it.
// It refuses rather than truncates: a sentence cut short is a different
// sentence, and the one thing a trouble must not do is misquote the screen.
func (t Trouble) Bound() (Trouble, error) {
	t.Text = strings.TrimSpace(t.Text)
	if t.Text == "" {
		return Trouble{}, errors.New("a trouble needs the sentence the screen showed")
	}
	if utf8.RuneCountInString(t.Text) > MaxTroubleText {
		return Trouble{}, ErrTroubleTooLong
	}
	fields := []*string{&t.Code, &t.Where.Section, &t.Where.Path, &t.Where.Subject, &t.Where.Kind, &t.At, &t.Tip}
	if t.Act != nil {
		act := *t.Act
		t.Act = &act
		fields = append(fields, &t.Act.Verb, &t.Act.Object, &t.Act.Target)
	}
	for _, field := range fields {
		*field = strings.TrimSpace(*field)
		if utf8.RuneCountInString(*field) > maxTroubleField {
			return Trouble{}, ErrTroubleTooLong
		}
	}
	return t, nil
}

// TroubleBlock is the trouble as the Partner is given it, one field a line.
func TroubleBlock(t Trouble) string {
	var built strings.Builder
	built.WriteString("The trouble:\n")
	built.WriteString("- What the screen said: " + oneLine(t.Text) + "\n")
	if t.Code != "" {
		built.WriteString("- Code: " + t.Code + "\n")
	}
	built.WriteString("- Where: " + troubleWhere(t.Where) + "\n")
	if t.Act != nil && t.Act.Verb != "" {
		act := t.Act.Verb
		if t.Act.Object != "" {
			act += " on " + t.Act.Object
		}
		if t.Act.Target != "" {
			act += " " + t.Act.Target
		}
		built.WriteString("- The act: " + act + "\n")
	}
	if t.At != "" {
		built.WriteString("- When: " + t.At + "\n")
	}
	if t.Tip != "" {
		built.WriteString("- The tip the page had: " + t.Tip + "\n")
	}
	if t.SignIn {
		built.WriteString("- The remedy the page offered: the sign-in sheet\n")
	}
	return built.String()
}

func troubleWhere(where TroubleWhere) string {
	said := where.Section
	if where.Path != "" {
		said = strings.TrimSpace(fmt.Sprintf("%s (%s)", said, where.Path))
	}
	if where.Subject != "" {
		subject := where.Subject
		if where.Kind != "" {
			subject = where.Kind + " " + subject
		}
		said += ", " + subject
	}
	if said == "" {
		return "the page did not say"
	}
	return said
}

// AskTrouble admits the turn one press on "Ask what happened" asks, in one
// conversation: a sitting's, named by its record, or the human's own. It is a
// turn like any other — the same key twice is the same turn once, a busy
// conversation refuses it, and it is recorded as every turn is — with the
// fixed sentence as the question and the trouble kept on the message for its
// chip.
func (s *Service) AskTrouble(ctx context.Context, human, sitting, key string, trouble Trouble, page Page) (string, error) {
	bounded, err := trouble.Bound()
	if err != nil {
		return "", err
	}
	return s.submitWith(ctx, human, sitting, key, TroubleRequest, page, true, "", "", &bounded)
}
