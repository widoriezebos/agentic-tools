package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProofCommandRunsAsAnOwnedLaunch(t *testing.T) {
	root := t.TempDir()
	brief := filepath.Join(root, "proof.json")
	data, _ := json.Marshal(PlainBrief{Argv: []string{"tool", "one"}, Dir: root, Env: []string{"KEY=value"}})
	os.WriteFile(brief, data, 0o600)
	adapterData := map[string]json.RawMessage{}
	setString(adapterData, "brief", brief)
	command, err := (PlainExec{}).Command(Record{Kind: "proof", AdapterData: adapterData}, root)
	measurement, strays, _, measureErr := (PlainExec{}).Measure(Record{}, root)
	if err != nil || measureErr != nil || command.Program != "tool" || command.Directory != root || !reflect.DeepEqual(command.Args, []string{"one"}) || !reflect.DeepEqual(command.Environment, []string{"KEY=value"}) || measurement != (Measurement{}) || len(strays) != 0 {
		t.Fatalf("command=%+v measurement=%+v strays=%v err=%v measureErr=%v", command, measurement, strays, err, measureErr)
	}
}
