package launch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/checkenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/commandenv"
)

type PlainExec struct{}

type PlainBrief struct {
	EngineDirectory string          `json:"engineDirectory,omitempty"`
	Input           *checkenv.Input `json:"approvedEnvironment,omitempty"`
	Overlays        []string        `json:"overlays,omitempty"`
	Argv            []string        `json:"argv"`
	Dir             string          `json:"dir"`
	Env             []string        `json:"env"`
}

func (PlainExec) Command(record Record, stateDir string) (Command, error) {
	data, err := os.ReadFile(readString(record.AdapterData, "brief"))
	if err != nil {
		return Command{}, err
	}
	var brief PlainBrief
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&brief); err != nil {
		return Command{}, err
	}
	if len(brief.Argv) == 0 || brief.Argv[0] == "" || brief.Dir == "" {
		return Command{}, fmt.Errorf("plain command requires argv and dir")
	}
	environment := brief.Env
	if brief.Input != nil {
		if err := brief.Input.Validate(); err != nil {
			return Command{}, err
		}
		environment = brief.Input.Environment()
	}
	environment = commandenv.EngineEnvironmentInDirectory(append([]string{}, environment...), brief.EngineDirectory)
	environment = append(environment, brief.Overlays...)
	return Command{ReplaceEnvironment: true, Program: brief.Argv[0], Args: brief.Argv[1:], Directory: brief.Dir,
		Environment: environment, LogPath: filepath.Join(stateDir, "exec.log")}, nil
}

func (PlainExec) Measure(Record, string) (Measurement, []Output, map[string]json.RawMessage, error) {
	return Measurement{}, nil, nil, nil
}

func (PlainExec) Strays() ([]string, error) { return nil, nil }
