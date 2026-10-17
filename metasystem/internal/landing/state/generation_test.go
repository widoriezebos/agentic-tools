package state_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func TestGenerationEventKeepsFourRoleEntries(t *testing.T) {
	t.Parallel()
	root, home := t.TempDir(), t.TempDir()
	now := time.Unix(1800000000, 0)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.template=true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := lane.Register(home, lane.Layout{Checkout: lane.CheckoutRoot(root), Install: lane.InstallRoot(root)}, "fixture", now); err != nil {
		t.Fatal(err)
	}
	dir := plain.Dir(root)
	payload := state.Generation{Target: "target", Verdict: "mismatch", Roles: []state.GenerationRole{
		{Role: "owner", Digest: "target", Generation: 148, Source: "current"},
		{Role: "watcher", Digest: "target", Generation: 148, Source: "current"},
		{Role: "reaper", Digest: "old", Generation: 147, Source: "obsolete"},
		{Role: "runner", Digest: "target", Generation: 148, Source: "current"},
	}}
	transition := func(from string, value state.Generation) (state.Event, error) {
		return state.Transition(dir, now, "generation", from, value.Verdict, "fixture", "", func(r *state.Record) { r.Generation = &value })
	}
	assertSaved := func(want state.Generation) {
		t.Helper()
		record, found, err := state.Read(dir)
		if err != nil || !found || !reflect.DeepEqual(record.Generation, &want) {
			t.Fatalf("generation record: %+v found=%t error=%v; want %+v", record.Generation, found, err, want)
		}
		events, skipped, err := state.ReadEvents(dir)
		if err != nil || skipped != 0 {
			t.Fatalf("events: skipped=%d error=%v", skipped, err)
		}
		var last *state.Generation
		for _, event := range events {
			if event.Kind == "generation" {
				last = event.Generation
			}
		}
		if !reflect.DeepEqual(last, &want) {
			t.Fatalf("generation event: %+v; want %+v", last, want)
		}
	}
	if _, err := transition("", payload); err != nil {
		t.Fatal(err)
	}
	assertSaved(payload)
	seams := plain.ProveSeams{
		Machine: func(string) (string, error) { return "fixture", nil },
		Git: func(_ string, args ...string) (string, error) {
			if args[0] == "merge-base" {
				return "", errors.New("no ancestor")
			}
			return "main", nil
		},
		Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
	}
	changed, err := plain.RefreshShadow(home, now, "fixture", seams)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range changed {
		if strings.Contains(change, "generation") {
			t.Fatalf("refresh reported a generation change: %v", changed)
		}
	}
	assertSaved(payload)
	payload.Verdict = "one generation"
	payload.Roles = append([]state.GenerationRole(nil), payload.Roles...)
	payload.Roles[2] = state.GenerationRole{Role: "reaper", Digest: "target", Generation: 148, Source: "current"}
	event, err := transition("mismatch", payload)
	if err != nil || event.From != "mismatch" {
		t.Fatalf("second transition: %+v %v", event, err)
	}
	assertSaved(payload)
	_, err = transition("mismatch", payload)
	var refused *state.Refused
	if !errors.As(err, &refused) || refused.Actual != "one generation" || refused.Expected != "mismatch" {
		t.Fatalf("stale from was not refused against the last verdict: %v", err)
	}
	assertSaved(payload)
	var legacy state.Event
	if err := json.Unmarshal([]byte(`{"kind":"lane","from":"running","to":"paused"}`), &legacy); err != nil || legacy.Generation != nil {
		t.Fatalf("legacy event: %+v %v", legacy, err)
	}
	data, err := json.Marshal(legacy)
	if err != nil || strings.Contains(string(data), "generation") {
		t.Fatalf("legacy encoding changed: %s %v", data, err)
	}
}
