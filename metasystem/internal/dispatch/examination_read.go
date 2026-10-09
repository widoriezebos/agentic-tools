package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

// executingEngineIdentity reads this executable's own build provenance. It
// does not sample the checkout or treat a stamp as a sortable fleet version.
func executingEngineIdentity() string {
	if name, err := os.Executable(); err == nil {
		if file, err := os.Open(name); err == nil {
			stamp, err := enginebuild.ReadStamp(file)
			file.Close()
			if err == nil && stamp != "" {
				return stamp
			}
		}
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		return "go-build:" + digestJSON(info)
	}
	return ""
}

// CollectExamination reads the immutable evidence of the actual examination
// job, including its recorded engine and resolved dispatch model.
func CollectExamination(repoRoot, jobID string) (readsubject.Read, error) {
	state := loadCritiqueState(repoRoot)
	record, present := state.records[jobID]
	if !present || asString(record["role"]) != "code-critic" {
		return readsubject.Read{}, fmt.Errorf("code examination %s is unreadable", jobID)
	}
	round, ok := numInt(record["round"])
	if !ok || round < 1 {
		return readsubject.Read{}, fmt.Errorf("examination %s has no round", jobID)
	}
	root := state.chainRoot(jobID)
	subject, present, err := readsubject.ReadRoundSubject(state.agents, root, round)
	if err != nil || !present {
		return readsubject.Read{}, fmt.Errorf("examination %s has no readable subject: %v", jobID, err)
	}
	dir := filepath.Join(state.agents, root, "rounds", strconv.FormatInt(round, 10))
	data, err := os.ReadFile(filepath.Join(dir, "return.json"))
	if err != nil {
		return readsubject.Read{}, err
	}
	var returned map[string]any
	if err = json.Unmarshal(data, &returned); err != nil {
		return readsubject.Read{}, err
	}
	returnedRound, _ := numInt(returned["round"])
	if asString(returned["jobId"]) != jobID || returnedRound != round || !readsubject.ReturnBindsSubject(subject, returned) {
		return readsubject.Read{}, fmt.Errorf("examination %s return names another job or subject", jobID)
	}
	engine := asString(record["engineBuild"])
	model := asString(record["effectiveModel"])
	if model == "" {
		model = asString(record["requestedModel"])
	}
	if engine == "" || model == "" {
		return readsubject.Read{}, fmt.Errorf("examination %s has unavailable engine or model provenance", jobID)
	}
	output := filepath.Join(dir, "return.md")
	prose, err := os.ReadFile(output)
	if err != nil {
		return readsubject.Read{}, fmt.Errorf("examination %s prose evidence is unreadable: %w", jobID, err)
	}
	verdict := ""
	for _, line := range strings.Split(string(prose), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "VERDICT:") {
			if verdict != "" {
				return readsubject.Read{}, fmt.Errorf("examination %s has multiple prose verdicts", jobID)
			}
			verdict = strings.TrimSpace(line)
		}
	}
	return readsubject.Collect(jobID, subject, engine, model, output, data, verdict)
}
