package repoproof

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

// TestOutput refers to the exact ordered output of one completed test.
type TestOutput struct {
	Path    string `json:"path"`
	Digest  string `json:"digest"`
	Outcome string `json:"outcome"`
}

func packageUnit(pkg string) string {
	if relative, ok := strings.CutPrefix(pkg, "github.com/widoriezebos/agentic-tools/metasystem/"); ok {
		return "metasystem/" + relative
	}
	return pkg
}

// TestEvidence retains exact output for completed tests, including empty output, from a complete log.
func TestEvidence(log, unit string, tests []string, outcome string) (map[string]TestOutput, error) {
	data, err := os.ReadFile(log)
	if err != nil {
		return nil, err
	}
	outputs, ended := map[string][]byte{}, map[string]string{}
	complete := ""
	for len(data) > 0 {
		data = bytes.TrimLeft(data, " \t\r\n")
		if len(data) == 0 {
			break
		}
		if data[0] == '{' {
			decoder := json.NewDecoder(bytes.NewReader(data))
			var event struct{ Action, Package, Test, Output string }
			if err := decoder.Decode(&event); err != nil {
				return nil, err
			}
			data = data[decoder.InputOffset():]
			if packageUnit(event.Package) == unit {
				if event.Test == "" && complete != "fail" && (event.Action == "pass" || event.Action == "fail" || event.Action == "skip") {
					complete = event.Action
				}
				if event.Test != "" {
					if event.Action == "output" {
						outputs[event.Test] = append(outputs[event.Test], []byte(event.Output)...)
					}
					if ended[event.Test] != "fail" && (event.Action == "pass" || event.Action == "fail" || event.Action == "skip") {
						ended[event.Test] = event.Action
					}
				}
			}
		} else {
			_, rest, found := bytes.Cut(data, []byte("\n"))
			if !found {
				break
			}
			data = rest
		}
	}
	if complete == "" || outcome != "" && complete != outcome || len(tests) == 0 {
		return nil, fmt.Errorf("test output unavailable for %s: no complete package report", unit)
	}
	for name, result := range ended {
		if result == "fail" && !slices.Contains(tests, name) {
			return nil, fmt.Errorf("the failed-test report omits %s/%s", unit, name)
		}
	}
	evidence := map[string]TestOutput{}
	for _, test := range tests {
		if ended[test] == "" || outcome != "" && ended[test] != outcome {
			return nil, fmt.Errorf("test output unavailable for %s/%s: no matching completed test", unit, test)
		}
		key, _ := json.Marshal([2]string{unit, test})
		path := fmt.Sprintf("%s.%x.output", log, sha256.Sum256(key))
		data := outputs[test]
		artifact, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(err, os.ErrExist) {
			retained, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(retained, data) {
				return nil, fmt.Errorf("retained test output differs: %s", path)
			}
		} else if err != nil {
			return nil, err
		} else {
			_, writeErr := artifact.Write(data)
			if err := errors.Join(writeErr, artifact.Sync(), artifact.Close()); err != nil {
				return nil, errors.Join(err, os.Remove(path))
			}
		}
		evidence[test] = TestOutput{Path: path, Digest: fmt.Sprintf("%x", sha256.Sum256(data)), Outcome: ended[test]}
	}
	return evidence, nil
}
