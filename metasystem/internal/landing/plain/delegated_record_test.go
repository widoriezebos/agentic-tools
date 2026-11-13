package plain

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestADelegatedBatchKeepsItsFixedSelection(t *testing.T) {
	t.Parallel()
	provenance := func(batch *Batch) *DelegatedProvenance {
		return &DelegatedProvenance{
			Kind: "selection", Grant: "G1", Decision: "D1", Session: "main-1",
			Class: "lane-recovery", Delegate: "Wido", CheckedAt: restartNow,
			Destination: batch.Lane, Subject: slices.Clone(batch.Members),
			Hold:         DelegatedHold{BarrenAt: restartNow.Format(time.RFC3339Nano), Fingerprint: "same", Launch: "last"},
			BarrenStopAt: "recorded-stop",
		}
	}
	t.Run("record", func(t *testing.T) {
		t.Parallel()
		install, batch, _, seams, _ := restartBed(t, 12, false)
		if _, err := RecordedDelegatedBatch(install, batch.Lane, ""); err == nil {
			t.Fatal("automatic selection was accepted as a delegate's")
		}
		batch.Delegated = provenance(batch)
		if err := writeBatch(install, batch, seams); err != nil {
			t.Fatal(err)
		}
		saved, err := RecordedDelegatedBatch(install, batch.Lane, batch.ID)
		if err != nil || saved == nil || saved.Person != nil || !reflect.DeepEqual(saved.Delegated, batch.Delegated) {
			t.Fatalf("delegated selection did not round-trip: %+v %v", saved, err)
		}
		if _, err := RecordedPersonBatch(install, batch.Lane, batch.ID); err == nil {
			t.Fatal("delegated selection was accepted as a person's")
		}
		other := batch.Lane
		other.CustodyEpoch++
		if _, err := RecordedDelegatedBatch(install, other, batch.ID); err == nil {
			t.Fatal("delegated selection continued under another registration")
		}
		if _, err := RecordedDelegatedBatch(install, batch.Lane, "another"); err == nil {
			t.Fatal("delegated selection continued under another batch id")
		}
		if automaticBatch(*saved) || !admissionFacts(*saved, Entry{}).PersonSeal {
			t.Fatal("delegated selection allowed automatic membership changes")
		}
		batch.State = BatchClosed
		if err := writeBatch(install, batch, seams); err != nil {
			t.Fatal(err)
		}
		if _, err := RecordedDelegatedBatch(install, batch.Lane, ""); err == nil {
			t.Fatal("closed delegated selection continued")
		}
		batch.State = BatchRunning
		if err := writeBatch(install, batch, seams); err != nil {
			t.Fatal(err)
		}
		if _, _, err := HandInChecked(install, Line{Goal: batch.Members[0].Goal, SHA: "newer", At: restartNow.Format(time.RFC3339Nano)}, "", seams); err != nil {
			t.Fatal(err)
		}
		if _, err := RecordedDelegatedBatch(install, batch.Lane, ""); err == nil || !strings.Contains(err.Error(), "newer hand-in") {
			t.Fatalf("superseded delegated selection continued: %v", err)
		}
	})
	for _, tc := range []struct {
		name   string
		change func(*Batch)
	}{
		{"both provenances", func(b *Batch) {
			b.Person = &ActProvenance{Kind: "selection", Person: "Wido", Root: b.Lane.Root, CheckedAt: restartNow, TerminalGeneration: 1, Destination: b.Lane, Subject: b.Members}
			b.Person.TerminalRef.PID = 1
		}},
		{"no grant", func(b *Batch) { b.Delegated.Grant = "" }},
		{"no decision", func(b *Batch) { b.Delegated.Decision = "" }},
		{"no session", func(b *Batch) { b.Delegated.Session = "" }},
		{"no hold", func(b *Batch) { b.Delegated.Hold.BarrenAt = "" }},
		{"another class", func(b *Batch) { b.Delegated.Class = "budget" }},
		{"another destination", func(b *Batch) { b.Delegated.Destination.CustodyEpoch++ }},
		{"another subject", func(b *Batch) { b.Delegated.Subject = []GoalSHA{{Goal: "other", SHA: "other"}} }},
		{"empty selection", func(b *Batch) { b.Members, b.Delegated.Subject = nil, nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			install, batch, _, seams, _ := restartBed(t, 12, false)
			batch.Delegated = provenance(batch)
			tc.change(batch)
			if err := writeBatch(install, batch, seams); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadBatch(install); err == nil {
				t.Fatal("incomplete or conflicting delegated selection was accepted")
			}
			if _, err := RecordedDelegatedBatch(install, batch.Lane, ""); err == nil {
				t.Fatal("invalid delegated selection continued")
			}
		})
	}
	for _, tc := range []struct {
		name          string
		red, prepared bool
	}{{name: "ordinary main"}, {name: "red main", red: true}, {name: "prepared selection", prepared: true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			install, batch, _, seams, ended := restartBed(t, 12, tc.red)
			batch.Delegated = provenance(batch)
			if tc.prepared {
				batch.State, batch.SealedAt = BatchPrepared, ""
				seams.Alive = func(Running) bool { return false }
				seams.Policy = func(string) (PolicyValue, error) { return PolicyValue{Value: "person"}, nil }
			}
			if err := writeBatch(install, batch, seams); err != nil {
				t.Fatal(err)
			}
			selected, err := SelectBatch(install, install, batch.Lane, seams)
			if err != nil || selected == nil || *ended != 0 || !slices.Equal(selected.Members, batch.Members) || selected.Open || len(selected.Restarts) != 0 {
				t.Fatalf("arrival changed a delegated selection: %+v ended=%d error=%v", selected, *ended, err)
			}
			if !tc.prepared && (len(selected.Waiting) != 1 || !strings.Contains(selected.Waiting[0], "this batch was selected under decision D1")) {
				t.Fatalf("missing delegated wait reason: %v", selected.Waiting)
			}
			saved, err := RecordedDelegatedBatch(install, batch.Lane, batch.ID)
			if err != nil || saved == nil || !slices.Equal(saved.Members, batch.Members) || saved.Open || len(saved.Restarts) != 0 {
				t.Fatalf("saved delegated selection changed: %+v %v", saved, err)
			}
			if tc.prepared {
				person := ActProvenance{Person: "Wido", Root: install, CheckedAt: restartNow, TerminalGeneration: 1}
				person.TerminalRef.PID = 1
				if _, err := PersonSelection(install, install, batch.Lane, []string{batch.Members[0].Goal}, person, seams); err != nil {
					t.Fatal(err)
				}
				replaced, err := RecordedPersonBatch(install, batch.Lane, batch.ID)
				if err != nil || replaced == nil || replaced.Delegated != nil {
					t.Fatalf("person's replacement did not clear the delegation: %+v %v", replaced, err)
				}
			}
		})
	}
}
