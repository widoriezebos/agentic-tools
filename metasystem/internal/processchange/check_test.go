package processchange

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
)

func TestAdmitCheckChangedAgentKeepsCallerArgv(t *testing.T) {
	t.Parallel()
	c := Check{Root: t.TempDir(), Installation: roots.Installation(t.TempDir()), ProcessAct: ProcessAct{Goal: "goal", Unit: "unit", Operation: ".inputs/unit", Checkout: "checkout", AfterArgv: []string{"make", "d"}}, Observation: true, Now: time.Unix(1, 0), Remedy: func(string) string { return "person check" }}
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
	c := Check{Root: t.TempDir(), Installation: roots.Installation(t.TempDir()), ProcessAct: ProcessAct{Goal: "goal", Unit: "unit", Operation: ".inputs/unit", Checkout: "checkout", AfterArgv: []string{"make", "d"}}, Observation: true, Now: time.Unix(1, 0)}
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

func TestCheckAdmissionKeepsProjectHistoryAndInstallationQuestions(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	installation := roots.Installation(filepath.Join(t.TempDir(), "installation"))
	if err := os.WriteFile(installation.Path(), []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	c := Check{Root: project, Installation: installation, Observation: true, Now: time.Unix(1, 0),
		ProcessAct: ProcessAct{Goal: "goal", Unit: "unit", Operation: "unit-check", Checkout: project, AfterArgv: []string{"make", "check"}},
		Remedy:     func(string) string { return "person check" }}
	if _, err := AdmitCheck(c); err != nil {
		t.Fatal(err)
	}
	c.Observation, c.AfterArgv = false, []string{"make", "audit"}
	if _, err := AdmitCheck(c); err == nil {
		t.Fatal("an unavailable installation admitted the question")
	}
	acts, unknown, err := ReadActs(project, c.Goal)
	if err != nil || len(unknown) != 0 || len(acts) != 1 || acts[0].Question != "" || acts[0].Status != "proposed" {
		t.Fatalf("failed question lost the proposal: %+v, %v, %v", acts, unknown, err)
	}
	if err := os.Remove(installation.Path()); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(installation.Path(), 0700); err != nil {
		t.Fatal(err)
	}
	proposed, err := AdmitCheck(c)
	if err != nil || proposed.ID != acts[0].ID || proposed.Question == "" {
		t.Fatalf("retry replaced the proposal or lost its question: %+v, %v", proposed, err)
	}
	question, err := channel.ReadQuestion(installation.Path(), proposed.Question)
	if err != nil || question.State != "open" || question.ProcessAct != proposed.ID {
		t.Fatalf("proposal did not ask through the installation: %+v, %v", question, err)
	}
	c.Person, c.Act = true, proposed.ID
	if applied, err := AdmitCheck(c); err != nil || applied.Status != "applied" {
		t.Fatalf("person could not apply the retained check: %+v, %v", applied, err)
	}
	question, err = channel.ReadQuestion(installation.Path(), proposed.Question)
	if err != nil || question.State != "closed" {
		t.Fatalf("application did not close the installation question: %+v, %v", question, err)
	}
	for _, path := range []string{filepath.Join(project, "artifacts"), installation.Path("process")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("process history and question storage crossed roots at %s: %v", path, err)
		}
	}
}
