package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnitDeclarationPreservesLegacyJSON(t *testing.T) {
	t.Parallel()
	legacy := `{"Values":["cheap","audits","15","full"],"Content":"","Commit":"","Tree":"","Branch":"","Directory":"","EnvironmentSHA256":"","SourceSHA256":"","FullArgv":null}`
	var declaration UnitDeclaration
	if err := json.Unmarshal([]byte(legacy), &declaration); err != nil {
		t.Fatal(err)
	}
	for _, explicit := range []bool{false, true} {
		if explicit {
			declaration.Values = declaration.EffectiveValues()
		}
		data, err := json.Marshal(declaration)
		if err != nil || string(data) != legacy {
			t.Fatal("default legacy declaration changed the bytes used by carried gate digests")
		}
	}
	declaration.Values[4], declaration.Values[5] = "bounded", "9"
	data, err := json.Marshal(declaration)
	var retained UnitDeclaration
	if err != nil || json.Unmarshal(data, &retained) != nil || retained.Values != declaration.Values {
		t.Fatal("bounded declaration lost its settings in serialization")
	}
}

func TestUnitDeclarationOldResultAndUnsupportedKind(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "check-old")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	// This is the on-disk shape written before kind and budget were declared.
	body := `{"executionId":"check-old","check":{"declaration":{"Values":["true","true","15","full"]}},"exits":[{"name":"cheap","exit":0},{"name":"audits","exit":0}]}`
	if err := os.WriteFile(filepath.Join(directory, "result.json"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := ReadCheckResult(directory)
	if err != nil || result.RequirePassed() != nil {
		t.Fatal("old four-value result no longer passes")
	}
	if values := result.Check.Declaration.EffectiveValues(); values[4] != "legacy" || values[5] != "6" {
		t.Fatal("old declaration lost its kind or budget default")
	}
	for _, kind := range []string{"bounded", "unknown"} {
		result.Check.Declaration.Values[4] = kind
		if result.RequirePassed() == nil {
			t.Fatal("unsupported declaration authorized complete coverage")
		}
		check := result.Check
		check.Cheap, check.Audits, check.Minutes = "printf legacy", "true", 15
		check.Directory, check.Environment = t.TempDir(), []string{}
		records := filepath.Join(t.TempDir(), "executions")
		execution, exits, err := check.Run(check.Directory, records, CheckContext{})
		if err == nil || !strings.Contains(err.Error(), "check") || execution != "" || len(exits) != 0 {
			t.Fatal("unsupported check fell through to legacy execution")
		}
		if _, err := os.Stat(records); !os.IsNotExist(err) {
			t.Fatal("unsupported check wrote execution evidence")
		}
	}
}
