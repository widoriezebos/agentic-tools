package rootaudit

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestRootAuditFindsStateCrossingsAndSparesTheInstallationAndTestLocals(t *testing.T) {
	t.Parallel()
	release := testenv.HoldToolchain()
	sites, err := Scan(context.Background(), filepath.Join("testdata", "module"))
	release()
	if err != nil {
		t.Fatal(err)
	}
	got := map[Key]int{}
	for _, site := range sites {
		got[site.Key()]++
	}
	file := "crossing/crossing.go"
	want := map[Key]int{
		{file, "crossing.DirectState", "State state.Path", `Path("artifacts")`}:                                              1,
		{file, "crossing.ParamChain", "State.Path() of state.Path", "crossing.middle arg 0 (dir)"}:                           1,
		{file, "crossing.FieldFlow", "State.Path() of state.Path", "field example.test/fixture/crossing.store.root"}:         1,
		{file, "crossing.FuncField", "State.Path() of state.Path", "crossing.runDir arg 0 via field:crossing.owners.launch"}: 1,
		{file, "crossing.Named", "variable stateRoot", "crossing.runDir arg 0 (root)"}:                                       1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sites = %v, want %v", sites, want)
	}
}

func TestRootAuditRatchetRefusesANewSiteAndAFixedSiteStillListed(t *testing.T) {
	t.Parallel()
	open := Site{File: "x/x.go", Line: 7, Function: "x.F", Source: "variable stateRoot", Sink: `filepath.Join(_, "artifacts")`, Witness: `x/x.go:7 joins "artifacts"`}
	entry := Entry{Owner: "later-work", Count: 1, File: open.File, Function: open.Function, Source: open.Source, Sink: open.Sink}
	if refusals := Check([]Entry{entry}, []Site{open}); len(refusals) != 0 {
		t.Fatalf("agreeing scan and list refused: %q", refusals)
	}
	twice := open
	twice.Line = 9
	refusals := Check([]Entry{entry}, []Site{open, twice})
	if len(refusals) != 1 || !strings.Contains(refusals[0], "new crossing x/x.go x.F: variable stateRoot -> filepath.Join(_, \"artifacts\") (owner later-work; found 2, listed 1; at x/x.go:7, x/x.go:9") {
		t.Fatalf("a second crossing under one key: %q", refusals)
	}
	refusals = Check(nil, []Site{open})
	if len(refusals) != 1 || !strings.Contains(refusals[0], "new crossing x/x.go x.F") || !strings.Contains(refusals[0], "owner none; found 1, listed 0") {
		t.Fatalf("an unlisted crossing: %q", refusals)
	}
	refusals = Check([]Entry{entry}, nil)
	if len(refusals) != 1 || refusals[0] != "run-state audit: fixed site still listed x/x.go x.F: variable stateRoot -> filepath.Join(_, \"artifacts\") (owner later-work; listed 1, found 0): delete this entry from run-state-audit.json" {
		t.Fatalf("a fixed site still listed: %q", refusals)
	}
	entry.Count = 2
	if refusals := Check([]Entry{entry}, []Site{open}); len(refusals) != 1 || !strings.HasSuffix(refusals[0], "lower this entry's count to 1 in run-state-audit.json") {
		t.Fatalf("one of two sites fixed: %q", refusals)
	}
}

func TestRootAuditBaselineRefusesAnIncompleteOrRepeatedEntry(t *testing.T) {
	t.Parallel()
	entry := `{"owner":"later-work","count":1,"file":"x/x.go","function":"x.F","source":"variable stateRoot","sink":"field x.T.root"}`
	for name, test := range map[string]struct{ text, refusal string }{
		"valid":    {`{"sites":[` + entry + `]}`, ""},
		"no owner": {`{"sites":[` + strings.Replace(entry, "later-work", "", 1) + `]}`, "entry is incomplete"},
		"repeated": {`{"sites":[` + entry + "," + entry + `]}`, "names x/x.go x.F: variable stateRoot -> field x.T.root twice"},
		"unknown":  {`{"sites":[],"update":true}`, "unparsable"},
	} {
		path := filepath.Join(t.TempDir(), BaselineFile)
		if err := os.WriteFile(path, []byte(test.text), 0o644); err != nil {
			t.Fatal(err)
		}
		entries, err := ReadBaseline(path)
		if test.refusal == "" && (err != nil || len(entries) != 1) || test.refusal != "" && (err == nil || !strings.Contains(err.Error(), test.refusal)) {
			t.Fatalf("%s: %v, %v", name, entries, err)
		}
	}
}
