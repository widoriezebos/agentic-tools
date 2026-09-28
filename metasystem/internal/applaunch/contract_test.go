package applaunch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeContract(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "launch.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A contract of start alone validates and leaves every default in place:
// no stop means TERM then KILL, no ready means alive is ready, no log means
// the engine's capture, no prepare means shared data, no check and no tools.
func TestStartAloneIsAWorkingContract(t *testing.T) {
	t.Parallel()
	contract, err := Decode([]byte(`{"schemaVersion":1,"start":{"argv":["./app"]}}`))
	if err != nil {
		t.Fatalf("a contract of start alone must validate: %v", err)
	}
	if contract.ReadyKind() != ReadyNone {
		t.Errorf("no ready means alive is ready, got %q", contract.ReadyKind())
	}
	if contract.Probed() {
		t.Error("the none form has nothing to probe")
	}
	if !contract.Stop.Empty() || !contract.Prepare.Empty() || !contract.Build.Empty() {
		t.Error("a contract of start alone declares no other command")
	}
	if contract.DataWord() != DataShared {
		t.Errorf("no prepare means data shared with the standing run, got %q", contract.DataWord())
	}
	if contract.ReadyWaitMS() != DefaultReadyMS || contract.StopWaitMS() != DefaultStopMS {
		t.Errorf("the stated defaults must apply, got readyMs=%d stopMs=%d", contract.ReadyWaitMS(), contract.StopWaitMS())
	}
	if contract.Check != "" {
		t.Error("no check declared")
	}
	if contract.Digest() == "" {
		t.Error("a loaded contract carries the digest of the bytes it was read from")
	}
}

func TestValidationNamesEveryFault(t *testing.T) {
	t.Parallel()
	for _, specimen := range []struct{ name, body, want string }{
		{"missing start", `{"schemaVersion":1}`, "start is required"},
		{"http without an address", `{"schemaVersion":1,"start":{"argv":["./app"]},"ready":{"kind":"http","url":"http://${address}/health"}}`,
			"an http readiness probe needs the application's address"},
		{"tcp without an address", `{"schemaVersion":1,"start":{"argv":["./app"]},"ready":{"kind":"tcp","address":"${address}"}}`,
			"a tcp readiness probe needs the application's address"},
		{"range overlapping the standing address",
			`{"schemaVersion":1,"start":{"argv":["./app"]},"address":"127.0.0.1:8085","portRange":"8080-8090"}`,
			"overlaps the standing address"},
		{"unknown placeholder in argv",
			`{"schemaVersion":1,"start":{"argv":["./app","--at","${hostname}"]}}`,
			"unknown placeholder ${hostname}"},
		{"unknown placeholder in the probe",
			`{"schemaVersion":1,"start":{"argv":["./app"]},"address":"127.0.0.1:8080","ready":{"kind":"http","url":"http://${endpoint}/health"}}`,
			"unknown placeholder ${endpoint}"},
		{"own data with no prepare",
			`{"schemaVersion":1,"start":{"argv":["./app"]},"data":"own"}`,
			"data: own is declared with no prepare to make it"},
		{"an unreadable schema", `{"schemaVersion":2,"start":{"argv":["./app"]}}`, "schemaVersion must be 1"},
		{"a ready kind the engine has no form for",
			`{"schemaVersion":1,"start":{"argv":["./app"]},"ready":{"kind":"ping"}}`, "is not one of http, tcp, log or none"},
		{"a log form with no pattern",
			`{"schemaVersion":1,"start":{"argv":["./app"]},"ready":{"kind":"log"}}`, "a log readiness form needs a pattern"},
		{"a tool with no executable",
			`{"schemaVersion":1,"start":{"argv":["./app"]},"tools":[{"id":"go"}]}`, "tool go declares no executable"},
		{"a start command escaping the project",
			`{"schemaVersion":1,"start":{"argv":["./app"],"cwd":"../elsewhere"}}`, "must be a relative normalized path"},
	} {
		t.Run(specimen.name, func(t *testing.T) {
			t.Parallel()
			_, err := Decode([]byte(specimen.body))
			if err == nil {
				t.Fatalf("%s must be refused", specimen.name)
			}
			if !strings.Contains(err.Error(), specimen.want) {
				t.Fatalf("the refusal must name the fault %q, got %q", specimen.want, err.Error())
			}
		})
	}
}

// data: own with a prepare is the declaration the refusal exists to protect,
// so it must validate.
func TestOwnDataWithPrepareValidates(t *testing.T) {
	t.Parallel()
	contract, err := Decode([]byte(`{"schemaVersion":1,"start":{"argv":["./app"]},"prepare":{"argv":["./seed"]},"data":"own"}`))
	if err != nil {
		t.Fatalf("own data with a prepare must validate: %v", err)
	}
	if !contract.OwnData() || contract.DataWord() != DataOwn {
		t.Error("a contract with prepare gives every run its own data")
	}
}

func TestLoadReadsTheFileAndItsDigest(t *testing.T) {
	t.Parallel()
	path := writeContract(t, `{"schemaVersion":1,"name":"demo","start":{"argv":["./app"]}}`)
	contract, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if contract.Name != "demo" || !strings.HasPrefix(contract.Digest(), "sha256:") {
		t.Fatalf("unexpected contract %+v", contract)
	}
	if _, err := Load(filepath.Join(filepath.Dir(path), "absent.json")); err == nil {
		t.Fatal("a contract that is not there is not a contract")
	}
}

// The three facts reach the argv and the probe by substitution, and the same
// three, with the state root and the log, reach the command as environment.
func TestPlaceholdersAndEnvironment(t *testing.T) {
	t.Parallel()
	contract, err := Decode([]byte(`{"schemaVersion":1,"address":"127.0.0.1:9412",
		"start":{"argv":["./app","--host","${host}","--port","${port}","--bind","${address}"]},
		"ready":{"kind":"http","url":"http://${address}/-/health"}}`))
	if err != nil {
		t.Fatal(err)
	}
	facts := FactsFor("127.0.0.1:9412")
	argv := facts.Argv(contract.Start)
	want := []string{"./app", "--host", "127.0.0.1", "--port", "9412", "--bind", "127.0.0.1:9412"}
	if strings.Join(argv, " ") != strings.Join(want, " ") {
		t.Fatalf("argv substitution: got %v want %v", argv, want)
	}
	if got := facts.Substitute(contract.Ready.URL); got != "http://127.0.0.1:9412/-/health" {
		t.Fatalf("probe substitution: %q", got)
	}
	environment := facts.Environment([]string{"PATH=/bin", EnvAddress + "=stale"}, "/state", "/state/app.log")
	joined := strings.Join(environment, "\n")
	for _, want := range []string{"PATH=/bin", EnvAddress + "=127.0.0.1:9412", EnvHost + "=127.0.0.1",
		EnvPort + "=9412", EnvStateRoot + "=/state", EnvLog + "=/state/app.log"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the run's environment must carry %s; got\n%s", want, joined)
		}
	}
	if strings.Contains(joined, EnvAddress+"=stale") {
		t.Error("the run's own facts replace an inherited one")
	}
}

func TestKeyForNamesOneRunPerRef(t *testing.T) {
	t.Parallel()
	if KeyFor("") != StandingKey {
		t.Error("no ref is the standing run")
	}
	first, second := KeyFor("main"), KeyFor("refs/heads/main")
	if first != KeyFor(" main ") {
		t.Error("one run per ref at a time: the same ref is the same key")
	}
	if first == second {
		t.Error("two spellings of two refs are two runs")
	}
	if KeyFor("main") == KeyFor("goal/g1-s70") {
		t.Error("two refs are two runs")
	}
	if strings.ContainsAny(KeyFor("goal/g1-s70"), "/ ") {
		t.Errorf("a key is a file name: %q", KeyFor("goal/g1-s70"))
	}
}
