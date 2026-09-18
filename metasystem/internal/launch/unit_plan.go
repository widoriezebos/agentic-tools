package launch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type UnitPlan struct {
	Unit     string         `json:"unit"`
	Goal     string         `json:"goal"`
	Worktree string         `json:"worktree"`
	Base     string         `json:"base"`
	Build    UnitBuildPlan  `json:"build"`
	Read     UnitReadPlan   `json:"read"`
	Proof    []ProofCommand `json:"proof"`
	Path     string         `json:"-"`
}

type UnitBuildPlan struct {
	Brief     string   `json:"brief"`
	Inputs    []string `json:"inputs"`
	Outputs   []string `json:"outputs"`
	UnitsPage string   `json:"unitsPage"`
	Units     []string `json:"units"`
}

type UnitReadPlan struct {
	Brief   string   `json:"brief"`
	Inputs  []string `json:"inputs"`
	Outputs []string `json:"outputs"`
	Model   string   `json:"model"`
}

type ProofCommand struct {
	Name string   `json:"name"`
	Dir  string   `json:"dir"`
	Argv []string `json:"argv"`
	Env  []string `json:"env"`
}

type rawUnitPlan struct {
	Unit     *string       `json:"unit"`
	Goal     *string       `json:"goal"`
	Worktree *string       `json:"worktree"`
	Base     *string       `json:"base"`
	Build    *rawBuildPlan `json:"build"`
	Read     *rawReadPlan  `json:"read"`
	Proof    *[]rawProof   `json:"proof"`
}
type rawBuildPlan struct {
	Brief     *string   `json:"brief"`
	Inputs    *[]string `json:"inputs"`
	Outputs   *[]string `json:"outputs"`
	UnitsPage *string   `json:"unitsPage"`
	Units     *[]string `json:"units"`
}
type rawReadPlan struct {
	Brief   *string   `json:"brief"`
	Inputs  *[]string `json:"inputs"`
	Outputs *[]string `json:"outputs"`
	Model   *string   `json:"model"`
}
type rawProof struct {
	Name *string   `json:"name"`
	Dir  *string   `json:"dir"`
	Argv *[]string `json:"argv"`
	Env  *[]string `json:"env"`
}

var unknownPlanField = regexp.MustCompile(`unknown field "([^"]+)"`)

func ReadUnitPlan(path string) (UnitPlan, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return UnitPlan{}, planInvalid("plan", err)
	}
	return readUnitPlan(abs, filepath.Dir(abs))
}

func readUnitPlan(path, relativeRoot string) (UnitPlan, error) {
	abs := path
	data, err := os.ReadFile(abs)
	if err != nil {
		return UnitPlan{}, planInvalid("plan", err)
	}
	var raw rawUnitPlan
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		field := "json"
		if match := unknownPlanField.FindStringSubmatch(err.Error()); len(match) == 2 {
			field = match[1]
		}
		return UnitPlan{}, planInvalid(field, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("more than one JSON value")
		}
		return UnitPlan{}, planInvalid("json", err)
	}
	required := []struct {
		name    string
		missing bool
	}{
		{"unit", raw.Unit == nil}, {"goal", raw.Goal == nil}, {"worktree", raw.Worktree == nil}, {"base", raw.Base == nil},
		{"build", raw.Build == nil}, {"read", raw.Read == nil}, {"proof", raw.Proof == nil},
	}
	for _, item := range required {
		if item.missing {
			return UnitPlan{}, planInvalid(item.name, nil)
		}
	}
	build, read := raw.Build, raw.Read
	for _, item := range []struct {
		name    string
		missing bool
	}{
		{"build.brief", build.Brief == nil}, {"build.inputs", build.Inputs == nil}, {"build.outputs", build.Outputs == nil},
		{"build.unitsPage", build.UnitsPage == nil}, {"build.units", build.Units == nil}, {"read.brief", read.Brief == nil},
		{"read.inputs", read.Inputs == nil}, {"read.outputs", read.Outputs == nil}, {"read.model", read.Model == nil},
	} {
		if item.missing {
			return UnitPlan{}, planInvalid(item.name, nil)
		}
	}
	plan := UnitPlan{Unit: *raw.Unit, Goal: *raw.Goal, Worktree: *raw.Worktree, Base: *raw.Base,
		Build: UnitBuildPlan{*build.Brief, *build.Inputs, *build.Outputs, *build.UnitsPage, *build.Units},
		Read:  UnitReadPlan{*read.Brief, *read.Inputs, *read.Outputs, *read.Model}, Path: abs}
	for index, proof := range *raw.Proof {
		prefix := fmt.Sprintf("proof[%d]", index)
		for _, item := range []struct {
			name    string
			missing bool
		}{{prefix + ".name", proof.Name == nil}, {prefix + ".dir", proof.Dir == nil}, {prefix + ".argv", proof.Argv == nil}, {prefix + ".env", proof.Env == nil}} {
			if item.missing {
				return UnitPlan{}, planInvalid(item.name, nil)
			}
		}
		plan.Proof = append(plan.Proof, ProofCommand{*proof.Name, *proof.Dir, *proof.Argv, *proof.Env})
	}
	if err := plan.resolveAndValidate(relativeRoot); err != nil {
		return UnitPlan{}, err
	}
	return plan, nil
}

func (plan *UnitPlan) resolveAndValidate(root string) error {
	resolve := func(path string) string {
		if filepath.IsAbs(path) {
			return filepath.Clean(path)
		}
		return filepath.Join(root, path)
	}
	plan.Worktree, plan.Build.Brief, plan.Build.UnitsPage, plan.Read.Brief = resolve(plan.Worktree), resolve(plan.Build.Brief), resolve(plan.Build.UnitsPage), resolve(plan.Read.Brief)
	for index := range plan.Build.Inputs {
		plan.Build.Inputs[index] = resolve(plan.Build.Inputs[index])
	}
	for index := range plan.Read.Inputs {
		plan.Read.Inputs[index] = resolve(plan.Read.Inputs[index])
	}
	for index := range plan.Build.Outputs {
		plan.Build.Outputs[index] = resolve(plan.Build.Outputs[index])
	}
	for index := range plan.Read.Outputs {
		plan.Read.Outputs[index] = resolve(plan.Read.Outputs[index])
	}
	for index := range plan.Proof {
		plan.Proof[index].Dir = resolve(plan.Proof[index].Dir)
	}
	for _, item := range []struct{ name, value string }{{"unit", plan.Unit}, {"goal", plan.Goal}, {"worktree", plan.Worktree}, {"base", plan.Base}, {"build.brief", plan.Build.Brief}, {"build.unitsPage", plan.Build.UnitsPage}, {"read.brief", plan.Read.Brief}} {
		if strings.TrimSpace(item.value) == "" {
			return planInvalid(item.name, nil)
		}
	}
	if len(plan.Build.Units) == 0 {
		return planInvalid("build.units", nil)
	}
	if len(plan.Proof) == 0 {
		return planInvalid("proof", nil)
	}
	files := []struct{ name, path string }{{"worktree", plan.Worktree}, {"build.brief", plan.Build.Brief}, {"build.unitsPage", plan.Build.UnitsPage}, {"read.brief", plan.Read.Brief}}
	for index, path := range plan.Build.Inputs {
		files = append(files, struct{ name, path string }{fmt.Sprintf("build.inputs[%d]", index), path})
	}
	for index, path := range plan.Read.Inputs {
		files = append(files, struct{ name, path string }{fmt.Sprintf("read.inputs[%d]", index), path})
	}
	for _, item := range files {
		info, err := os.Stat(item.path)
		if err != nil {
			return planInvalid(item.name, err)
		}
		if item.name == "worktree" && !info.IsDir() || item.name != "worktree" && info.IsDir() {
			return planInvalid(item.name, nil)
		}
	}
	seen := map[string]bool{}
	for index, proof := range plan.Proof {
		if !idPattern.MatchString(proof.Name) || seen[proof.Name] {
			return planInvalid("proof.name", nil)
		}
		seen[proof.Name] = true
		if len(proof.Argv) == 0 || proof.Argv[0] == "" {
			return planInvalid(fmt.Sprintf("proof[%d].argv", index), nil)
		}
		if info, err := os.Stat(proof.Dir); err != nil || !info.IsDir() {
			return planInvalid(fmt.Sprintf("proof[%d].dir", index), err)
		}
		for _, entry := range proof.Env {
			if !strings.Contains(entry, "=") {
				return planInvalid(fmt.Sprintf("proof[%d].env", index), nil)
			}
		}
	}
	return nil
}

func planInvalid(field string, cause error) error {
	if cause == nil {
		return fmt.Errorf("UNIT_PLAN_INVALID field=%s", field)
	}
	return fmt.Errorf("UNIT_PLAN_INVALID field=%s: %w", field, cause)
}

func proofCommands(plan UnitPlan) []ProofCommand { return append([]ProofCommand(nil), plan.Proof...) }
