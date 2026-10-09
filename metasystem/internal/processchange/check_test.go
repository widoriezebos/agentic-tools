package processchange

import (
	"slices"
	"testing"
	"time"
)

func TestAdmitCheckChangedAgentKeepsCallerArgv(t *testing.T) {
	t.Parallel()
	c := Check{Root: t.TempDir(), ProcessAct: ProcessAct{Goal: "goal", Unit: "unit", Operation: ".inputs/unit", Checkout: "checkout", AfterArgv: []string{"make", "d"}}, Observation: true, Now: time.Unix(1, 0), Remedy: func(string) string { return "person check" }}
	if _, err := AdmitCheck(c); err != nil {
		t.Fatal(err)
	}
	c.Observation, c.Person, c.Actor, c.AfterArgv = false, true, "direct-person", []string{"go", "test", "./x"}
	applied, err := AdmitCheck(c)
	if err != nil || applied.Status != "applied" {
		t.Fatalf("person check: %+v %v", applied, err)
	}
	c.Person, c.Actor, c.AfterArgv = false, "agent", []string{"rm", "-rf", "/tmp/zzz"}
	held, err := AdmitCheck(c)
	if err != nil || held.Status != "proposed" || held.ID == applied.ID || !slices.Equal(held.AfterArgv, []string{"rm", "-rf", "/tmp/zzz"}) || !slices.Equal(c.AfterArgv, []string{"rm", "-rf", "/tmp/zzz"}) {
		t.Fatalf("changed agent check must be held without rewriting caller argv: %+v %v; caller %q", held, err, c.AfterArgv)
	}
	// Naming an applied act cannot authorize different arguments either.
	c.Act = applied.ID
	if replay, err := AdmitCheck(c); err == nil && replay.Status == "applied" || !slices.Equal(c.AfterArgv, []string{"rm", "-rf", "/tmp/zzz"}) {
		t.Fatalf("applied identity rewrote or admitted different argv: %+v %v; caller %q", replay, err, c.AfterArgv)
	}
}

func TestAdmitCheckLongerPersonArgvIsNewAct(t *testing.T) {
	t.Parallel()
	c := Check{Root: t.TempDir(), ProcessAct: ProcessAct{Goal: "goal", Unit: "unit", Operation: ".inputs/unit", Checkout: "checkout", AfterArgv: []string{"make", "d"}}, Observation: true, Now: time.Unix(1, 0)}
	if _, err := AdmitCheck(c); err != nil {
		t.Fatal(err)
	}
	c.Observation, c.Person, c.Actor, c.AfterArgv = false, true, "direct-person", []string{"go", "test", "./x"}
	first, err := AdmitCheck(c)
	if err != nil || first.Status != "applied" {
		t.Fatalf("first person check: %+v %v", first, err)
	}
	c.AfterArgv = []string{"go", "test", "./x", "-count=1"}
	second, err := AdmitCheck(c)
	if err != nil || second.Status != "applied" || second.ID == first.ID || second.Actor != c.Actor || !slices.Equal(second.BeforeArgv, first.AfterArgv) || !slices.Equal(second.AfterArgv, c.AfterArgv) {
		t.Fatalf("longer person check must be a new admitted act: %+v %v", second, err)
	}
	acts, unknown, err := ReadActs(c.Root, c.Goal)
	if err != nil || len(unknown) != 0 || len(acts) != 2 || !slices.ContainsFunc(acts, func(act ProcessAct) bool { return act.ID == first.ID && slices.Equal(act.AfterArgv, first.AfterArgv) }) {
		t.Fatalf("person replacement lost prior evidence: %+v %v %v", acts, unknown, err)
	}
	c.Person, c.Actor, c.Act, c.AfterArgv = false, "agent", first.ID, slices.Clone(first.AfterArgv)
	if stale, err := AdmitCheck(c); err != nil || stale.Status != "superseded" {
		t.Fatalf("older applied identity overrode the person's latest selection: %+v %v", stale, err)
	}
}
