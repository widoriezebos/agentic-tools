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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/returnschema"
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

// DesignEvidenceRequired distinguishes frozen whole-page examinations from
// legacy rounds whose collection uses the original structured findings.
func DesignEvidenceRequired(repoRoot, rootJob string, round int64) (bool, error) {
	subject, present, err := readsubject.ReadRoundSubject(filepath.Join(repoRoot, "artifacts", "agents"), rootJob, round)
	return present && subject.DesignPage != "", err
}

// CollectExamination reads the immutable evidence of the actual examination
// job, including its recorded engine and resolved dispatch model.
func CollectExamination(repoRoot, jobID string) (readsubject.Read, error) {
	state := loadCritiqueState(repoRoot)
	record, present := state.records[jobID]
	if !present || (asString(record["role"]) != "code-critic" && asString(record["role"]) != "design-critic") {
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
	if subject.Kind == readsubject.SubjectDesign {
		if violations := returnschema.ReturnCompleteRole(repoRoot, "design-critic", filepath.Join(dir, "return.json")); len(violations) > 0 {
			return readsubject.Read{}, fmt.Errorf("design evidence unknown: %s", strings.Join(violations, "; "))
		}
		page, problems, present := project.ParseRecord(subject.DesignPath, subject.DesignPage)
		if !present || len(problems) != 0 || page.Kind != project.KindDesign {
			return readsubject.Read{}, fmt.Errorf("frozen design record is unavailable or malformed")
		}
		read, err := readsubject.CollectDesignRead(jobID, root, page.ID, page.Goals, subject, engine, model, output, data, verdict)
		sectionOwner := record
		sectionRound := round
		if source := asString(record["examinationRetryOf"]); source != "" {
			sectionOwner = state.records[source]
			sectionRound, _ = numInt(sectionOwner["round"])
			original, present, problem := readsubject.ReadRoundSubject(state.agents, root, sectionRound)
			if problem != nil || !present || !original.Equal(subject) || original.DesignPage != subject.DesignPage {
				return read, fmt.Errorf("design retry does not bind its original frozen page: %v", problem)
			}
		}
		if err == nil && sectionRound > 1 {
			parent := state.records[asString(sectionOwner["parentJob"])]
			previousRound, ok := numInt(parent["round"])
			if !ok || previousRound < 1 || previousRound >= round {
				return read, fmt.Errorf("section history has no preceding examination")
			}
			decisions, problem := frozenDesignDecisions(state.agents, root, page.ID, asString(sectionOwner["jobId"]), asString(sectionOwner["operationId"]), previousRound)
			if problem != nil {
				return read, problem
			}
			sections, problem := DesignRevisionSections(repoRoot, root, previousRound, subject.DesignPage, decisions)
			if problem != nil {
				return read, problem
			}
			if sections != nil {
				read.Design.Sections = sections
			}
		}
		if err == nil {
			canonical, _ := read.Canonical()
			if retained, problem := os.ReadFile(filepath.Join(dir, "read.json")); problem == nil && string(retained) != string(canonical) {
				return read, fmt.Errorf("design examination's immutable section evidence changed")
			} else if problem != nil && !os.IsNotExist(problem) {
				return read, problem
			}
		}
		return read, err
	}
	return readsubject.Collect(jobID, subject, engine, model, output, data, verdict)
}
