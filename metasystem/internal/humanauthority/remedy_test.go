package humanauthority

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// A person refused a person's act reads why in plain words and the one
// command that resolves it, with the name the engine knows filled in (Wido,
// 2026-09-30: "this terminal isn't enrolled yet ... metasystem system enroll
// --name wido, then repeat this command").
func TestRemedyForNamesTheReasonAndTheOneCommand(t *testing.T) {
	t.Parallel()
	retry := []string{"metasystem", "grant", "add", "--acts", "everything", "--for", "24h"}
	enrolled := authorityRoot(t)
	enrollTestTerminal(t, enrolled, enrolledReader())
	enrollment, err := ReadEnrollment(enrolled)
	if err != nil {
		t.Fatal(err)
	}
	bare := authorityRoot(t)
	for _, test := range []struct {
		name, root, person string
		err                error
		want               Remedy
	}{
		{"not enrolled, the helm holder's name", bare, "wido", fmt.Errorf("%s: human authority has no readable terminal enrollment: nope", OutcomeNotEnrolled),
			Remedy{Kind: RemedyNotEnrolled, Reason: "this terminal isn't enrolled yet", Argv: []string{"metasystem", "system", "enroll", "--name", "wido"}, Then: "then repeat this command"}},
		{"not enrolled, in plain words, no one known", bare, "", errors.New("only a person may run this: no terminal is enrolled on this machine"),
			Remedy{Kind: RemedyNotEnrolled, Reason: "this terminal isn't enrolled yet", Argv: []string{"metasystem", "system", "enroll", "--name", "NAME"}, Then: "then repeat this command"}},
		{"another terminal is enrolled, its person's name", enrolled, "", errors.New(OutcomeTerminalMissing),
			Remedy{Kind: RemedyOtherTerminal, Reason: "this terminal isn't enrolled (" + enrollment.Human + " enrolled another one)", Argv: []string{"metasystem", "system", "enroll", "--name", enrollment.Human}, Then: "moves the enrollment here; then repeat this command"}},
		{"an agent started the shell", enrolled, "wido", fmt.Errorf("%s: claude", OutcomeAgent),
			Remedy{Kind: RemedyAgent, Reason: "an agent (claude) started this shell", Argv: retry, Then: "in a terminal you opened yourself"}},
		{"an agent, in plain words", enrolled, "wido", errors.New("this shell was started by an agent (codex)"),
			Remedy{Kind: RemedyAgent, Reason: "an agent (codex) started this shell", Argv: retry, Then: "in a terminal you opened yourself"}},
		{"no terminal above the shell, and none enrolled", bare, "wido", errors.New(OutcomeTerminalMissing),
			Remedy{Kind: RemedyNotEnrolled, Reason: "this terminal isn't enrolled yet", Argv: []string{"metasystem", "system", "enroll", "--name", "wido"}, Then: "then repeat this command"}},
		{"another cause names no command", enrolled, "wido", errors.New("goal budget fixture authority does not combine with a temporary human word"),
			Remedy{Reason: "goal budget fixture authority does not combine with a temporary human word"}},
		{"unreadable ancestry", enrolled, "wido", errors.New(OutcomeChanged),
			Remedy{Kind: RemedyUnreadable, Reason: "the processes behind this shell couldn't be read", Argv: retry, Then: "try again"}},
	} {
		if got := RemedyFor(test.root, test.err, test.person, retry); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s:\n got  %+v\n want %+v", test.name, got, test.want)
		}
	}
}

// An act the helm admitted keeps why the walk refused it, so a refusal that
// the helm does not cover names the real cause.
func TestHelmProofKeepsTheWalkRefusal(t *testing.T) {
	t.Parallel()
	root := authorityRoot(t)
	reader := agentShellReader()
	enrollTestTerminal(t, root, reader)
	proof, err := prove(root, 80, reader, time.Unix(1100, 0), func(string, int64) (HelmGrant, bool) { return HelmGrant{By: "wido"}, true })
	if err != nil || proof.Helm == nil {
		t.Fatalf("the helm did not admit: %v", err)
	}
	if refusal := proof.WalkRefusal(); refusal == nil || !strings.Contains(refusal.Error(), OutcomeAgent) {
		t.Fatalf("walk refusal = %v, want the agent in the chain", refusal)
	}
	direct, err := prove(root, 20, reader, time.Unix(1100, 0), nil)
	if err != nil || direct.WalkRefusal() != nil {
		t.Fatalf("a proof the walk made carries a refusal: %v %v", err, direct.WalkRefusal())
	}
}
