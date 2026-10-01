package launch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type PlainExec struct{}

type PlainBrief struct {
	Argv []string `json:"argv"`
	Dir  string   `json:"dir"`
	Env  []string `json:"env"`
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
	return Command{Program: brief.Argv[0], Args: brief.Argv[1:], Directory: brief.Dir,
		Environment: brief.Env, LogPath: filepath.Join(stateDir, "exec.log")}, nil
}

func (PlainExec) Measure(Record, string) (Measurement, []Output, map[string]json.RawMessage, error) {
	return Measurement{}, nil, nil, nil
}

func (PlainExec) Strays() ([]string, error) { return nil, nil }
