package humanauthority

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// W3: when the walk refuses and the helm does not admit, a general power of
// attorney admits the caller as the person, with a helm-shaped proof naming
// the grant; without it the refusal is today's, and the helm answers first.
func TestProveAdmitsTheCallerAGrantAdmits(t *testing.T) {
	t.Parallel()
	root := authorityRoot(t)
	reader := agentShellReader()
	enrollTestTerminal(t, root, reader)
	enrollment, err := ReadEnrollment(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1100, 0)
	grant := HelmGrant{By: "wido", Class: "MAIN", Grant: "01M-grant", Until: "2026-10-01T07:14:00Z"}
	var askedAt []time.Time
	atAttorney := func(gotRoot string, pid int64, at time.Time) (HelmGrant, bool) {
		if gotRoot != root || pid != 80 {
			t.Errorf("the grant was asked about %s/%d", gotRoot, pid)
		}
		askedAt = append(askedAt, at)
		return grant, true
	}
	reader.reads = map[int64]int{}
	proof, err := proveEnrolled(root, 80, reader, now, enrollment, nil, atAttorney)
	if err != nil {
		t.Fatalf("the grant did not admit the agent: %v", err)
	}
	if !reflect.DeepEqual(askedAt, []time.Time{now}) {
		t.Fatalf("the grant was asked at %v, want once at the proof's time", askedAt)
	}
	if proof.Helm == nil || *proof.Helm != grant || !proof.ValidFor(root) || !proof.EnrolledTerminalFor(root) {
		t.Fatalf("grant proof = %+v", proof)
	}
	if err := RecordProof(root, "op-grant", "goal approve", proof); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "authority", "proofs", "op-grant.json"))
	if err != nil || !strings.Contains(string(data), `"grant": "01M-grant"`) || !strings.Contains(string(data), `"until": "2026-10-01T07:14:00Z"`) {
		t.Fatalf("the proof record lacks the grant: %v\n%s", err, data)
	}

	// The helm answers first.
	helm := HelmGrant{By: "wido", Class: "DELEGATE"}
	reader.reads = map[int64]int{}
	askedAt = nil
	proof, err = proveEnrolled(root, 80, reader, now, enrollment, func(string, int64) (HelmGrant, bool) { return helm, true }, atAttorney)
	if err != nil || proof.Helm == nil || *proof.Helm != helm || len(askedAt) != 0 {
		t.Fatalf("the helm did not answer first: %+v %v %v", proof, err, askedAt)
	}

	// Declined or absent: today's refusal byte for byte.
	reader.reads = map[int64]int{}
	today, todayErr := proveEnrolled(root, 80, reader, now, enrollment, nil, nil)
	reader.reads = map[int64]int{}
	declined, declinedErr := proveEnrolled(root, 80, reader, now, enrollment, nil, func(string, int64, time.Time) (HelmGrant, bool) { return grant, false })
	if todayErr == nil || declinedErr == nil || !reflect.DeepEqual(today, declined) || todayErr.Error() != declinedErr.Error() {
		t.Fatalf("a declining grant changed the refusal: %+v %v / %+v %v", today, todayErr, declined, declinedErr)
	}
	reader.reads = map[int64]int{}
	if exported, err := Prove(root, 80, reader, now); AtAttorney != nil || err == nil || !reflect.DeepEqual(exported, today) {
		t.Fatalf("Prove with the library's nil seam is not today's refusal: %+v %v", exported, err)
	}
}
