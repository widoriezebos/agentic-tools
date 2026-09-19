package testimpact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"
)

const SchemaVersion = 1
const (
	ModeRun               = "run"
	ModeList              = "list"
	StatusPassed          = "passed"
	StatusFailed          = "failed"
	StatusIncomplete      = "incomplete"
	StatusListed          = "listed"
	StatusNothingSelected = "nothing-selected"
	StatusNotRun          = "not-run"
	ScopeNamed            = "named"
	ScopePackage          = "package"
	ScopeModule           = "module"
	ScopeCheck            = "check"
)

type Request struct {
	SchemaVersion  int      `json:"schemaVersion"`
	Mode           string   `json:"mode"`
	Base           string   `json:"base"`
	RepositoryRoot string   `json:"repositoryRoot"`
	ChangedPaths   []string `json:"changedPaths"`
	Binding        string   `json:"binding"`
}
type Result struct {
	SchemaVersion int           `json:"schemaVersion"`
	Mode          string        `json:"mode"`
	Binding       string        `json:"binding"`
	Status        string        `json:"status"`
	Selections    []Selection   `json:"selections"`
	Uncertainty   []Uncertainty `json:"uncertainty"`
}
type Selection struct {
	Label   string   `json:"label"`
	CWD     string   `json:"cwd"`
	Argv    []string `json:"argv"`
	Scope   string   `json:"scope"`
	Tests   []string `json:"tests"`
	Reasons []Reason `json:"reasons"`
	Result  string   `json:"result"`
	Seconds float64  `json:"seconds"`
}
type Reason struct {
	Test    string `json:"test"`
	Because string `json:"because"`
}
type Uncertainty struct {
	Paths   []string `json:"paths"`
	Because string   `json:"because"`
	Action  string   `json:"action"`
}

func DecodeRequest(data []byte) (Request, error) {
	var value Request
	err := decodeStrict(data, &value)
	if err == nil {
		err = value.Validate()
	}
	if err != nil {
		return Request{}, fmt.Errorf("request: %w", err)
	}
	return value, nil
}
func DecodeResult(data []byte, request Request) (Result, error) {
	var value Result
	err := decodeStrict(data, &value)
	if err == nil {
		err = ValidateResult(request, value)
	}
	if err != nil {
		return Result{}, fmt.Errorf("result: %w", err)
	}
	return value, nil
}
func Encode(value any) ([]byte, error) { return json.Marshal(value) }
func (request Request) Validate() error {
	valid := request.SchemaVersion == SchemaVersion && (request.Mode == ModeRun || request.Mode == ModeList) &&
		isHexID(request.Base, 40, 64) && request.RepositoryRoot != "" && filepath.IsAbs(request.RepositoryRoot) &&
		filepath.Clean(request.RepositoryRoot) == request.RepositoryRoot && isHexID(request.Binding, 64) &&
		request.ChangedPaths != nil && sort.StringsAreSorted(request.ChangedPaths)
	for i, path := range request.ChangedPaths {
		valid = valid && relativePath(path) && (i == 0 || request.ChangedPaths[i-1] != path)
	}
	return protocolValidity(valid)
}
func ValidateResult(r Request, v Result) error { return validateResult(r, v, false) }
func validateResult(request Request, result Result, allowNotRun bool) error {
	valid := result.SchemaVersion == SchemaVersion && result.Mode == request.Mode && result.Binding == request.Binding && result.Selections != nil && result.Uncertainty != nil
	labels, counts := map[string]bool{}, map[string]int{}
	for _, selection := range result.Selections {
		valid = valid && validSelection(selection, request.Mode) && !labels[selection.Label]
		labels[selection.Label], counts[selection.Result] = true, counts[selection.Result]+1
	}
	for _, uncertainty := range result.Uncertainty {
		valid = valid && uncertainty.Paths != nil && uncertainty.Because != "" && uncertainty.Action != ""
		for _, path := range uncertainty.Paths {
			valid = valid && relativePath(path)
		}
	}
	return protocolValidity(valid && validOutcome(request.Mode, result.Status, len(result.Selections), counts, allowNotRun))
}
func validOutcome(mode, status string, selections int, counts map[string]int, allowNotRun bool) bool {
	switch status {
	case StatusPassed:
		return mode == ModeRun && selections > 0 && counts[StatusPassed] == selections
	case StatusFailed:
		return mode == ModeRun && counts[StatusFailed] > 0
	case StatusIncomplete:
		return mode == ModeRun && counts[StatusFailed] == 0 && counts[StatusIncomplete] > 0
	case StatusListed:
		return mode == ModeList
	case StatusNothingSelected:
		return mode == ModeRun && selections == 0
	case StatusNotRun:
		return allowNotRun && selections == 0
	}
	return false
}
func validSelection(selection Selection, mode string) bool {
	valid := selection.Label != "" && relativePath(selection.CWD) && len(selection.Argv) > 0 && selection.Argv[0] != "" &&
		selection.Tests != nil && selection.Reasons != nil && !math.IsNaN(selection.Seconds) && !math.IsInf(selection.Seconds, 0) && selection.Seconds >= 0 &&
		(selection.Scope == ScopeNamed || selection.Scope == ScopePackage || selection.Scope == ScopeModule || selection.Scope == ScopeCheck) &&
		(mode != ModeList || selection.Result == StatusNotRun && selection.Seconds == 0) &&
		(mode != ModeRun || selection.Result == StatusPassed || selection.Result == StatusFailed || selection.Result == StatusIncomplete || selection.Result == StatusNotRun)
	for _, arg := range selection.Argv {
		valid = valid && !strings.ContainsRune(arg, 0)
	}
	reasoned := map[string]bool{}
	for _, reason := range selection.Reasons {
		valid = valid && reason.Test != "" && reason.Because != ""
		reasoned[reason.Test] = true
	}
	if selection.Scope == ScopeNamed {
		valid = valid && len(selection.Tests) > 0
		for _, test := range selection.Tests {
			valid = valid && test != "" && reasoned[test]
		}
	} else {
		valid = valid && reasoned["*"]
	}
	return valid && (selection.Result != StatusNotRun || len(selection.Reasons) > 0)
}
func protocolValidity(valid bool) error {
	if valid {
		return nil
	}
	return fmt.Errorf("protocol fields or outcomes are invalid")
}
func decodeStrict(data []byte, target any) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("document is not UTF-8")
	}
	if err := rejectDuplicates(json.NewDecoder(bytes.NewReader(data))); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	canonical, _ := json.Marshal(target)
	var input, output any
	_ = json.Unmarshal(data, &input)
	_ = json.Unmarshal(canonical, &output)
	if !reflect.DeepEqual(input, output) {
		return fmt.Errorf("object has missing or inconsistent fields")
	}
	return nil
}
func rejectDuplicates(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token == nil {
		return fmt.Errorf("required field cannot be null")
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			field, err := decoder.Token()
			if err != nil {
				return err
			}
			name := field.(string)
			if seen[name] {
				return fmt.Errorf("duplicate field %q", name)
			}
			seen[name] = true
			if err := rejectDuplicates(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := rejectDuplicates(decoder); err != nil {
				return err
			}
		}
	}
	_, err = decoder.Token()
	return err
}
func relativePath(path string) bool {
	return path == "." || path != "" && !filepath.IsAbs(path) && filepath.ToSlash(filepath.Clean(path)) == path && !strings.ContainsRune(path, 0) && path != ".." && !strings.HasPrefix(path, "../")
}
func isHexID(value string, lengths ...int) bool {
	ok := false
	for _, length := range lengths {
		ok = ok || len(value) == length
	}
	if !ok {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
