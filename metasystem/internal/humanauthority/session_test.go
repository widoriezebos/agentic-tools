package humanauthority

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSignedInSessionProofIsBoundToItsRootAndCarriesNoSecret(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Unix(1700, 0)
	proof, err := SignedInSessionProof(root, "Wido", "sess-abc123", "browser", now)
	if err != nil {
		t.Fatalf("mint a signed-in session proof: %v", err)
	}
	if OutcomeSession != "SIGNED_IN_SESSION" || proof.Outcome != OutcomeSession {
		t.Fatalf("outcome = %q, want SIGNED_IN_SESSION", proof.Outcome)
	}
	if proof.ChannelProvider != "browser" || proof.ChannelUser != "Wido" || proof.ChannelRef != "sess-abc123" {
		t.Fatalf("proof does not name its issuer, human, and session: %+v", proof)
	}
	if proof.ChannelContext != "" || proof.ChannelStep != 0 || proof.ReviewBy != "" ||
		proof.TemporaryHumanWord != "" || proof.Grade != "" || len(proof.Nodes) != 0 {
		t.Fatalf("a session proof carried terminal or channel-thread facts: %+v", proof)
	}
	if !proof.SessionValidFor(root) {
		t.Fatal("the minted proof is not valid for the root it was bound to")
	}
	if proof.SessionValidFor(filepath.Join(root, "elsewhere")) {
		t.Fatal("a session proof authorized a different root")
	}
}

func TestSignedInSessionProofIsNotEnrolledTerminalAuthority(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	proof, err := SignedInSessionProof(root, "Wido", "sess-abc123", "browser", time.Unix(1700, 0))
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]bool{
		"Valid":                   proof.Valid(),
		"ValidFor":                proof.ValidFor(root),
		"TerminalValidFor":        proof.TerminalValidFor(root),
		"EnrolledTerminalFor":     proof.EnrolledTerminalFor(root),
		"ChannelWordFor":          proof.ChannelWordFor(root),
		"AuthorizesResume":        proof.AuthorizesResume(root),
		"AuthorizesSetObligation": proof.AuthorizesSetObligation(root),
	} {
		if got {
			t.Fatalf("a signed-in session passed %s", name)
		}
	}
}

func TestParsedSignedInSessionProofHasNoAuthority(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	proof, err := SignedInSessionProof(root, "Wido", "sess-abc123", "browser", time.Unix(1700, 0))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	var parsed Proof
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "observedRoot") {
		t.Fatalf("the observation escaped into the proof document: %s", encoded)
	}
	if parsed.SessionValidFor(root) {
		t.Fatal("a parsed proof document acted as a signed-in session")
	}
}

func TestSignedInSessionProofRefusesIncompleteRecords(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Unix(1700, 0)
	for _, test := range []struct {
		name, user, ref, issuer, want string
		now                           time.Time
	}{
		{name: "zero time", user: "Wido", ref: "sess", issuer: "browser", now: time.Time{}, want: "non-zero observation time"},
		{name: "no issuer", user: "Wido", ref: "sess", issuer: "", now: now, want: "issuer"},
		{name: "no user", user: "", ref: "sess", issuer: "browser", now: now, want: "user"},
		{name: "no session reference", user: "Wido", ref: "", issuer: "browser", now: now, want: "session reference"},
		{name: "spaced user", user: "Wido Riezebos", ref: "sess", issuer: "browser", now: now, want: "user"},
		{name: "spaced session reference", user: "Wido", ref: "sess abc", issuer: "browser", now: now, want: "session reference"},
		{name: "newline in issuer", user: "Wido", ref: "sess", issuer: "brow\nser", now: now, want: "issuer"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			proof, err := SignedInSessionProof(root, test.user, test.ref, test.issuer, test.now)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("SignedInSessionProof = %+v, %v; want a refusal naming %q", proof, err, test.want)
			}
			if proof.SessionValidFor(root) {
				t.Fatal("a refused construction still produced authority")
			}
		})
	}
}
