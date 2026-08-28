package metrics

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type jobRecord struct {
	Path        string
	JobID       string
	Role        string
	Status      string
	GoalID      string
	ParentJob   string
	Runtime     string
	Round       int
	StartedAt   time.Time
	EndedAt     time.Time
	Usage       map[string]any
	RootJob     string
	Local       bool
	TimingError string
}

type receiptRecord struct {
	Line           int
	Epoch          string
	OriginalKey    string
	At             time.Time
	Fields         map[string]string
	InvalidGoal    bool
	InvalidBuiltBy bool
}

type landingCommit struct {
	SHA          string
	At           time.Time
	ChangedLines int
	Goals        map[string]bool
	ReceiptKeys  []string
	Shared       bool
}

type journalRecord struct {
	Path       string
	Verb       string
	Targets    []string
	Outcome    string
	Evidence   string
	Attempts   int
	TerminalAt time.Time
}

type proofRecord struct {
	Path      string
	Surface   string
	Verdict   string
	StartedAt time.Time
	At        time.Time
	Fallback  bool
}

type critiqueChain struct {
	Path   string
	Name   string
	GoalID string
	Rounds int
}

type goalRecord struct {
	Path string
	File *goal.GoalFile
}

type world struct {
	Identity sourceIdentity
	Machine  string

	Jobs             []jobRecord
	JobCoverage      Coverage
	Receipts         []*receiptRecord
	ReceiptCoverage  Coverage
	Landings         []landingCommit
	LandingCoverage  Coverage
	Journals         []journalRecord
	JournalCoverage  Coverage
	Proofs           []proofRecord
	ProofCoverage    Coverage
	Critiques        []critiqueChain
	CritiqueCoverage Coverage
	Goals            map[string]goalRecord
	GoalCoverage     Coverage
}

var goalToken = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func loadWorld(root string) (world, error) {
	w := world{Goals: map[string]goalRecord{}}
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		return w, err
	}
	w.Machine = machine

	gitFacts, err := loadGitFacts(root)
	if err != nil {
		return w, err
	}
	w.Identity.MainTip = gitFacts.mainTip
	w.Identity.ReceiptBlob = gitFacts.receiptBlob
	w.LandingCoverage = gitFacts.coverage

	w.Receipts, w.ReceiptCoverage = loadReceipts(root, gitFacts.mainTip, gitFacts.receiptPath, gitFacts.selfReceiptLines)
	w.Landings = attributeLandings(gitFacts.landings, w.Receipts)
	w.Jobs, w.JobCoverage = loadJobs(root)
	w.Journals, w.JournalCoverage = loadJournals(root)
	w.Proofs, w.ProofCoverage = loadProofs(root)
	w.Critiques, w.CritiqueCoverage = loadCritiques(root)
	w.Goals, w.GoalCoverage, w.Identity.AcceptedTip = loadGoals(root)
	return w, nil
}

func gitCommand(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = withoutGitSteering(os.Environ())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func withoutGitSteering(environ []string) []string {
	blocked := map[string]bool{
		"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_COMMON_DIR": true,
		"GIT_INDEX_FILE": true, "GIT_OBJECT_DIRECTORY": true,
		"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
	}
	result := make([]string, 0, len(environ))
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if !blocked[name] {
			result = append(result, entry)
		}
	}
	return result
}

type gitFacts struct {
	mainTip          string
	receiptBlob      string
	receiptPath      string
	landings         []landingCommit
	coverage         Coverage
	selfReceiptLines map[string]int
}

type rawCommit struct {
	sha         string
	parents     []string
	email       string
	at          time.Time
	paths       []string
	receiptPath string
	lines       int
	added       []string
}

func loadGitFacts(root string) (gitFacts, error) {
	facts := gitFacts{coverage: Coverage{Source: "landings"}, selfReceiptLines: map[string]int{}}
	gitRoot, err := gitCommand(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return facts, fmt.Errorf("metrics report: cannot resolve repository root: %w", err)
	}
	gitRoot = strings.TrimSpace(gitRoot)
	prefix, err := gitCommand(root, "rev-parse", "--show-prefix")
	if err != nil {
		return facts, fmt.Errorf("metrics report: cannot resolve checkout prefix: %w", err)
	}
	prefix = strings.TrimSpace(prefix)
	receiptRoot, err := stateroot.RelativeRoot(stateroot.Receipts)
	if err != nil {
		return facts, fmt.Errorf("metrics report: cannot resolve receipt state root: %w", err)
	}
	facts.receiptPath = filepath.ToSlash(filepath.Join(prefix, receiptRoot, "receipts.log"))
	legacyReceiptPath := filepath.ToSlash(filepath.Join(prefix, "plans", "receipts.log"))
	mainTip, err := gitCommand(gitRoot, "rev-parse", "--verify", "refs/heads/main")
	if err != nil {
		return facts, fmt.Errorf("metrics report: no readable main branch: %w", err)
	}
	facts.mainTip = strings.TrimSpace(mainTip)
	moveLog, err := gitCommand(gitRoot, "log", "--reverse", "--diff-filter=A", "--format=%H", facts.mainTip, "--", facts.receiptPath)
	if err != nil {
		return facts, fmt.Errorf("metrics report: cannot resolve receipt-home history: %w", err)
	}
	moveCommit := ""
	if fields := strings.Fields(moveLog); len(fields) > 0 {
		moveCommit = fields[0]
	}
	if blob, blobErr := gitCommand(gitRoot, "rev-parse", facts.mainTip+":"+facts.receiptPath); blobErr == nil {
		facts.receiptBlob = strings.TrimSpace(blob)
	}

	log, err := gitCommand(gitRoot, "log", "--topo-order", "--reverse", "--format=%x1e%H%x1f%P%x1f%ae%x1f%cI", "--numstat", facts.mainTip)
	if err != nil {
		return facts, fmt.Errorf("metrics report: cannot read main history: %w", err)
	}
	commits := map[string]*rawCommit{}
	var order []string
	for _, block := range strings.Split(log, "\x1e") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		header := strings.Split(lines[0], "\x1f")
		if len(header) != 4 {
			continue
		}
		stamp, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(header[3]))
		if parseErr != nil {
			facts.coverage.Rejected++
			facts.coverage.Details = append(facts.coverage.Details, "commit="+header[0]+" invalid committer timestamp")
			continue
		}
		commit := &rawCommit{
			sha: strings.TrimSpace(header[0]), parents: strings.Fields(header[1]),
			email: strings.TrimSpace(header[2]), at: stamp.UTC(), receiptPath: legacyReceiptPath,
		}
		for _, parent := range commit.parents {
			if ancestor := commits[parent]; ancestor != nil && ancestor.receiptPath == facts.receiptPath {
				commit.receiptPath = facts.receiptPath
				break
			}
		}
		if commit.sha == moveCommit {
			commit.receiptPath = facts.receiptPath
		}
		for _, line := range lines[1:] {
			fields := strings.Split(line, "\t")
			if len(fields) < 3 {
				continue
			}
			commit.paths = append(commit.paths, filepath.ToSlash(fields[len(fields)-1]))
			added, addErr := strconv.Atoi(fields[0])
			deleted, delErr := strconv.Atoi(fields[1])
			if addErr == nil && delErr == nil {
				commit.lines += added + deleted
			}
		}
		commits[commit.sha] = commit
		order = append(order, commit.sha)
	}

	if facts.receiptBlob != "" {
		for _, receiptPath := range []string{legacyReceiptPath, facts.receiptPath} {
			patches, patchErr := gitCommand(gitRoot, "log", "--reverse", "--format=%x1e%H", "-p", "--unified=0", facts.mainTip, "--", receiptPath)
			if patchErr == nil {
				for _, block := range strings.Split(patches, "\x1e") {
					block = strings.TrimLeft(block, "\r\n")
					if block == "" {
						continue
					}
					newline := strings.IndexByte(block, '\n')
					if newline < 0 {
						continue
					}
					sha := strings.TrimSpace(block[:newline])
					commit := commits[sha]
					if commit == nil || commit.receiptPath != receiptPath || sha == moveCommit {
						continue
					}
					for _, line := range strings.Split(block[newline+1:], "\n") {
						if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
							commit.added = append(commit.added, strings.TrimPrefix(line, "+"))
						}
					}
				}
			}
		}
	}

	metricsPrefix := filepath.ToSlash(filepath.Join(prefix, "plans", "metrics")) + "/"
	for _, sha := range order {
		commit := commits[sha]
		if commit == nil || commit.email == "goals@metasystem.invalid" {
			continue
		}
		onlyMetrics := len(commit.paths) > 0
		hasMetrics := false
		for _, path := range commit.paths {
			switch {
			case path == commit.receiptPath:
			case strings.HasPrefix(path, metricsPrefix):
				hasMetrics = true
			default:
				onlyMetrics = false
			}
		}
		for _, line := range commit.added {
			fields, ok := receiptFields(line)
			if !ok || fields["type"] != "metrics-report" {
				onlyMetrics = false
			}
		}
		if onlyMetrics && hasMetrics {
			for _, line := range commit.added {
				facts.selfReceiptLines[line]++
			}
			continue
		}
		var receiptKeys []string
		for _, line := range commit.added {
			if _, ok := receiptFields(line); ok {
				receiptKeys = append(receiptKeys, receiptOriginalKey(line))
			}
		}
		facts.landings = append(facts.landings, landingCommit{
			SHA: commit.sha, At: commit.at, ChangedLines: commit.lines,
			ReceiptKeys: receiptKeys,
		})
	}
	facts.coverage.Found = len(facts.landings)
	if facts.coverage.Found == 0 {
		facts.coverage.Missing = 1
	}
	return facts, nil
}

func receiptOriginalKey(line string) string {
	epoch, _, _ := strings.Cut(line, "|")
	return epoch + "|" + fmt.Sprintf("%x", sha1.Sum([]byte(line)))
}

func receiptFields(line string) (map[string]string, bool) {
	parts := strings.Split(line, "|")
	if len(parts) < 3 || parts[2] != "RECEIPT" {
		return nil, false
	}
	fields := map[string]string{}
	for _, field := range parts[3:] {
		key, value, found := strings.Cut(field, "=")
		if found && key != "" {
			fields[key] = value
		}
	}
	return fields, true
}

func loadReceipts(root, mainTip, receiptPath string, selfLines map[string]int) ([]*receiptRecord, Coverage) {
	coverage := Coverage{Source: "receipts"}
	if mainTip == "" || receiptPath == "" {
		coverage.Missing = 1
		return nil, coverage
	}
	text, err := gitCommand(root, "cat-file", "-p", mainTip+":"+receiptPath)
	if err != nil {
		coverage.Missing = 1
		coverage.Details = append(coverage.Details, "path="+receiptPath+" unreadable")
		return nil, coverage
	}
	originals := map[string]*receiptRecord{}
	var records []*receiptRecord
	for index, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if line == "" {
			continue
		}
		if selfLines[line] > 0 {
			selfLines[line]--
			continue
		}
		coverage.Found++
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			coverage.Rejected++
			coverage.Details = append(coverage.Details, fmt.Sprintf("line=%d malformed", index+1))
			continue
		}
		stamp, stampErr := time.Parse(time.RFC3339, parts[1])
		if stampErr != nil {
			coverage.Rejected++
			coverage.Details = append(coverage.Details, fmt.Sprintf("line=%d invalid timestamp", index+1))
			continue
		}
		fields := map[string]string{}
		for _, field := range parts[3:] {
			key, value, found := strings.Cut(field, "=")
			if found && key != "" {
				fields[key] = value
			}
		}
		switch parts[2] {
		case "RECEIPT":
			key := receiptOriginalKey(line)
			record := &receiptRecord{Line: index + 1, Epoch: parts[0], OriginalKey: key, At: stamp.UTC(), Fields: fields}
			records = append(records, record)
			originals[key] = record
		case "CORRECTION":
			key := fields["ref_epoch"] + "|" + fields["ref_sha1"]
			record := originals[key]
			if record == nil || fields["field"] == "" {
				coverage.Rejected++
				coverage.Details = append(coverage.Details, fmt.Sprintf("line=%d correction reference unresolved", index+1))
				continue
			}
			record.Fields[fields["field"]] = fields["now"]
		case "RETRO":
		default:
			coverage.Rejected++
			coverage.Details = append(coverage.Details, fmt.Sprintf("line=%d unknown record type", index+1))
		}
	}
	for _, record := range records {
		var invalidProvenance []string
		if !receipt.ValidGoalValue(record.Fields["goal"]) {
			record.InvalidGoal = true
			invalidProvenance = append(invalidProvenance, "goal")
		}
		if !receipt.ValidBuiltByValue(record.Fields["built_by"]) {
			record.InvalidBuiltBy = true
			invalidProvenance = append(invalidProvenance, "built_by")
		}
		if len(invalidProvenance) > 0 {
			coverage.Rejected++
			coverage.Details = append(coverage.Details, fmt.Sprintf("line=%d invalid effective %s", record.Line, strings.Join(invalidProvenance, " and ")))
		}
	}
	if coverage.Found == 0 {
		coverage.Missing = 1
	}
	return records, coverage
}

func attributeLandings(landings []landingCommit, receipts []*receiptRecord) []landingCommit {
	effectiveReceipts := make(map[string]*receiptRecord, len(receipts))
	for _, record := range receipts {
		effectiveReceipts[record.OriginalKey] = record
	}
	for index := range landings {
		goals := map[string]bool{}
		for _, key := range landings[index].ReceiptKeys {
			record := effectiveReceipts[key]
			if record == nil || record.InvalidGoal || record.InvalidBuiltBy {
				continue
			}
			if goalID := record.Fields["goal"]; goalID != "" {
				goals[goalID] = true
			}
		}
		landings[index].Goals = goals
		landings[index].Shared = len(goals) > 1
	}
	return landings
}

func evidenceRoot(root string) string {
	value, _, err := config.Get(config.GetParams{
		Key: "evidence.root", Default: "", DefaultSet: true,
		ConfPath: filepath.Join(root, "metasystem.conf"),
	})
	if err != nil || !filepath.IsAbs(value) {
		return ""
	}
	return filepath.Clean(value)
}

func loadJobs(root string) ([]jobRecord, Coverage) {
	coverage := Coverage{Source: "jobs"}
	localDir := filepath.Join(root, "artifacts", "agents", "jobs")
	var paths []string
	local, _ := filepath.Glob(filepath.Join(localDir, "*.json"))
	paths = append(paths, local...)
	if evidence := evidenceRoot(root); evidence != "" {
		current, _ := filepath.Glob(filepath.Join(evidence, "agents", "*", "*", "jobs", "*.json"))
		legacy, _ := filepath.Glob(filepath.Join(evidence, "agents", "*", "jobs", "*.json"))
		paths = append(paths, current...)
		paths = append(paths, legacy...)
	}
	sort.Strings(paths)
	coverage.Found = len(paths)
	type candidate struct {
		record jobRecord
	}
	byID := map[string][]candidate{}
	for _, path := range paths {
		record, err := parseJob(path, strings.HasPrefix(filepath.Clean(path), filepath.Clean(localDir)+string(filepath.Separator)))
		if err != nil {
			coverage.Rejected++
			coverage.Details = append(coverage.Details, "path="+path+" "+cleanLine(err.Error()))
			continue
		}
		if record.TimingError != "" {
			coverage.Rejected++
			coverage.Details = append(coverage.Details, "path="+path+" timing rejected: "+record.TimingError)
		}
		byID[record.JobID] = append(byID[record.JobID], candidate{record: record})
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var records []jobRecord
	for _, id := range ids {
		candidates := byID[id]
		winner := 0
		for index := range candidates {
			if candidates[index].record.Local {
				winner = index
				break
			}
		}
		chosen := candidates[winner].record
		for index, other := range candidates {
			if index == winner || other.record.Status == chosen.Status {
				continue
			}
			if dispatch.TerminalStatus(other.record.Status) || dispatch.TerminalStatus(chosen.Status) {
				coverage.Rejected++
				coverage.Details = append(coverage.Details, fmt.Sprintf("path=%s duplicate jobId=%s terminal status %s conflicts with %s", other.record.Path, id, other.record.Status, chosen.Status))
			}
		}
		records = append(records, chosen)
	}
	lookup := map[string]*jobRecord{}
	for index := range records {
		lookup[records[index].JobID] = &records[index]
	}
	for index := range records {
		seen := map[string]bool{}
		current := &records[index]
		for current.ParentJob != "" && !seen[current.JobID] {
			seen[current.JobID] = true
			parent := lookup[current.ParentJob]
			if parent == nil {
				break
			}
			current = parent
		}
		records[index].RootJob = current.JobID
	}
	if coverage.Found == 0 {
		coverage.Missing = 1
	}
	return records, coverage
}

func parseJob(path string, local bool) (jobRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return jobRecord{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		return jobRecord{}, fmt.Errorf("malformed JSON: %v", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return jobRecord{}, fmt.Errorf("malformed JSON: trailing value after job record")
		}
		return jobRecord{}, fmt.Errorf("malformed JSON: trailing content: %v", err)
	}
	text := func(key string, required bool) (string, error) {
		value, present := raw[key]
		if !present || value == nil {
			if required {
				return "", fmt.Errorf("missing %s", key)
			}
			return "", nil
		}
		result, ok := value.(string)
		if !ok || (required && result == "") {
			return "", fmt.Errorf("invalid %s", key)
		}
		return result, nil
	}
	jobID, err := text("jobId", true)
	if err != nil {
		return jobRecord{}, err
	}
	role, err := text("role", true)
	if err != nil {
		return jobRecord{}, err
	}
	status, err := text("status", true)
	if err != nil {
		return jobRecord{}, err
	}
	goalID, err := text("goalId", false)
	if err != nil {
		return jobRecord{}, err
	}
	parent, err := text("parentJob", false)
	if err != nil {
		return jobRecord{}, err
	}
	runtimeName, err := text("runtime", false)
	if err != nil {
		return jobRecord{}, err
	}
	startedText, err := text("startedAt", false)
	if err != nil {
		return jobRecord{}, err
	}
	endedText, err := text("endedAt", false)
	if err != nil {
		return jobRecord{}, err
	}
	terminal := dispatch.TerminalStatus(status)
	var started, ended time.Time
	timingError := ""
	if startedText != "" {
		started, err = time.Parse(time.RFC3339, startedText)
		if err != nil {
			return jobRecord{}, fmt.Errorf("invalid startedAt")
		}
		started = started.UTC()
	} else if terminal {
		timingError = "missing startedAt"
	}
	if endedText != "" {
		ended, err = time.Parse(time.RFC3339, endedText)
		if err != nil {
			return jobRecord{}, fmt.Errorf("invalid endedAt")
		}
		ended = ended.UTC()
	} else if terminal {
		if timingError != "" {
			timingError += " and endedAt"
		} else {
			timingError = "missing endedAt"
		}
	}
	if terminal && timingError == "" && ended.Before(started) {
		timingError = "endedAt precedes startedAt"
	}
	round := 0
	if number, ok := raw["round"].(json.Number); ok {
		parsed, parseErr := strconv.Atoi(number.String())
		if parseErr == nil {
			round = parsed
		}
	}
	usage, _ := raw["usage"].(map[string]any)
	return jobRecord{
		Path: path, JobID: jobID, Role: role, Status: status, GoalID: goalID,
		ParentJob: parent, Runtime: runtimeName, Round: round,
		StartedAt: started, EndedAt: ended, Usage: usage, Local: local, TimingError: timingError,
	}, nil
}

func loadJournals(root string) ([]journalRecord, Coverage) {
	coverage := Coverage{Source: "goal-journal"}
	paths, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "goal-transactions", "*.json"))
	sort.Strings(paths)
	coverage.Found = len(paths)
	var records []journalRecord
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			coverage.Rejected++
			coverage.Details = append(coverage.Details, "path="+path+" unreadable")
			continue
		}
		var raw struct {
			Intent struct {
				Verb    string   `json:"verb"`
				Targets []string `json:"targets"`
			} `json:"intent"`
			Phase      string `json:"phase"`
			Outcome    string `json:"outcome"`
			Evidence   string `json:"evidence"`
			Attempts   int    `json:"attempts"`
			TerminalAt string `json:"terminalAt"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			coverage.Rejected++
			coverage.Details = append(coverage.Details, "path="+path+" malformed JSON")
			continue
		}
		if raw.Phase != "terminal" {
			continue
		}
		stamp, err := time.Parse(time.RFC3339, raw.TerminalAt)
		if err != nil || raw.Intent.Verb == "" {
			coverage.Rejected++
			coverage.Details = append(coverage.Details, "path="+path+" invalid terminal record")
			continue
		}
		records = append(records, journalRecord{Path: path, Verb: raw.Intent.Verb, Targets: raw.Intent.Targets, Outcome: raw.Outcome, Evidence: raw.Evidence, Attempts: raw.Attempts, TerminalAt: stamp.UTC()})
	}
	if coverage.Found == 0 {
		coverage.Missing = 1
	}
	return records, coverage
}

func loadProofs(root string) ([]proofRecord, Coverage) {
	coverage := Coverage{Source: "proof-evidence"}
	var records []proofRecord
	if evidence := evidenceRoot(root); evidence != "" {
		directories, _ := filepath.Glob(filepath.Join(evidence, "suite-failures", "*"))
		sort.Strings(directories)
		for _, directory := range directories {
			info, statErr := os.Stat(directory)
			if statErr != nil || !info.IsDir() {
				continue
			}
			coverage.Found++
			outcomePath := filepath.Join(directory, "outcome.json")
			data, err := os.ReadFile(outcomePath)
			if err != nil {
				coverage.Rejected++
				coverage.Details = append(coverage.Details, "path="+outcomePath+" torn envelope without outcome.json")
				continue
			}
			var outcome struct {
				Verdict string `json:"verdict"`
			}
			if json.Unmarshal(data, &outcome) != nil || outcome.Verdict == "" {
				coverage.Rejected++
				coverage.Details = append(coverage.Details, "path="+outcomePath+" malformed verdict")
				continue
			}
			stamp := info.ModTime().UTC()
			fallback := false
			timingsPath := filepath.Join(directory, "timings.json")
			if timingData, timingErr := os.ReadFile(timingsPath); timingErr == nil {
				var timing struct {
					StartedAt string `json:"startedAt"`
					EndedAt   string `json:"endedAt"`
				}
				if json.Unmarshal(timingData, &timing) != nil {
					coverage.Rejected++
					coverage.Details = append(coverage.Details, "path="+timingsPath+" malformed timings")
					continue
				}
				parsed, parseErr := time.Parse(time.RFC3339, timing.EndedAt)
				if parseErr != nil {
					coverage.Rejected++
					coverage.Details = append(coverage.Details, "path="+timingsPath+" invalid endedAt")
					continue
				}
				stamp = parsed.UTC()
				if timing.StartedAt != "" {
					started, startErr := time.Parse(time.RFC3339, timing.StartedAt)
					if startErr != nil || started.After(stamp) {
						coverage.Rejected++
						coverage.Details = append(coverage.Details, "path="+timingsPath+" invalid startedAt")
						continue
					}
					records = append(records, proofRecord{Path: directory, Surface: "milestone-battery", Verdict: outcome.Verdict, StartedAt: started.UTC(), At: stamp})
					continue
				}
			} else if os.IsNotExist(timingErr) {
				fallback = true
			} else {
				coverage.Rejected++
				coverage.Details = append(coverage.Details, "path="+timingsPath+" unreadable")
				continue
			}
			records = append(records, proofRecord{Path: directory, Surface: "milestone-battery", Verdict: outcome.Verdict, At: stamp, Fallback: fallback})
		}
	}

	codes, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "battery", "*.codes"))
	sort.Strings(codes)
	for _, path := range codes {
		coverage.Found++
		data, err := os.ReadFile(path)
		info, statErr := os.Stat(path)
		if err != nil || statErr != nil {
			coverage.Rejected++
			coverage.Details = append(coverage.Details, "path="+path+" unreadable")
			continue
		}
		valid := false
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			name, code, found := strings.Cut(line, "=")
			if !found || name == "" {
				continue
			}
			valid = true
			verdict := "non-green"
			if code == "0" {
				verdict = "green"
			}
			records = append(records, proofRecord{Path: path, Surface: "local-battery:" + name, Verdict: verdict, At: info.ModTime().UTC()})
		}
		if !valid {
			coverage.Rejected++
			coverage.Details = append(coverage.Details, "path="+path+" malformed codes")
		}
	}

	enumeration := filepath.Join(root, "artifacts", "agents", "enumeration-report.txt")
	if data, err := os.ReadFile(enumeration); err == nil {
		coverage.Found++
		info, _ := os.Stat(enumeration)
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Split(line, "\t")
			if len(fields) < 5 || fields[0] != "section" {
				continue
			}
			verdict := fields[3]
			if verdict == "pass" {
				verdict = "green"
			}
			records = append(records, proofRecord{Path: enumeration, Surface: "suite:" + fields[1], Verdict: verdict, At: info.ModTime().UTC()})
		}
	}
	if coverage.Found == 0 {
		coverage.Missing = 1
	}
	return records, coverage
}

func loadCritiques(root string) ([]critiqueChain, Coverage) {
	coverage := Coverage{Source: "critique-chains"}
	directories, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "critiques", "*"))
	sort.Strings(directories)
	var chains []critiqueChain
	roundPattern := regexp.MustCompile(`^r([0-9]+)-output[.]md$`)
	for _, directory := range directories {
		info, err := os.Stat(directory)
		if err != nil || !info.IsDir() {
			continue
		}
		coverage.Found++
		entries, _ := os.ReadDir(directory)
		maxRound := 0
		for _, entry := range entries {
			match := roundPattern.FindStringSubmatch(entry.Name())
			if match == nil {
				continue
			}
			round, _ := strconv.Atoi(match[1])
			if round > maxRound {
				maxRound = round
			}
		}
		if maxRound == 0 {
			coverage.Rejected++
			coverage.Details = append(coverage.Details, "path="+directory+" no archived output rounds")
			continue
		}
		goalID := ""
		decisionPath := filepath.Join(directory, "attribution")
		if decisionInfo, statErr := os.Lstat(decisionPath); statErr == nil {
			if !decisionInfo.Mode().IsRegular() {
				coverage.Rejected++
				coverage.Details = append(coverage.Details, "path="+decisionPath+" unreadable: not a regular file")
				continue
			}
			decisionData, readErr := os.ReadFile(decisionPath)
			if readErr != nil {
				coverage.Rejected++
				coverage.Details = append(coverage.Details, "path="+decisionPath+" unreadable: "+cleanLine(readErr.Error()))
				continue
			}
			decision := strings.TrimSuffix(string(decisionData), "\n")
			switch {
			case decision == "unattributed":
			case strings.HasPrefix(decision, "goal "):
				goalID = strings.TrimPrefix(decision, "goal ")
				if !goalToken.MatchString(goalID) {
					coverage.Rejected++
					coverage.Details = append(coverage.Details, "path="+decisionPath+" invalid attribution decision")
					continue
				}
			default:
				coverage.Rejected++
				coverage.Details = append(coverage.Details, "path="+decisionPath+" invalid attribution decision")
				continue
			}
		} else if !os.IsNotExist(statErr) {
			coverage.Rejected++
			coverage.Details = append(coverage.Details, "path="+decisionPath+" unreadable: "+cleanLine(statErr.Error()))
			continue
		}
		chains = append(chains, critiqueChain{Path: directory, Name: filepath.Base(directory), GoalID: goalID, Rounds: maxRound})
	}
	if coverage.Found == 0 {
		coverage.Missing = 1
	}
	return chains, coverage
}

func loadGoals(root string) (map[string]goalRecord, Coverage, string) {
	coverage := Coverage{Source: "goals"}
	records := map[string]goalRecord{}
	tipText, err := gitCommand(root, "rev-parse", "--verify", goal.AcceptedRef)
	if err != nil {
		coverage.Missing = 1
		return records, coverage, ""
	}
	tip := strings.TrimSpace(tipText)
	files, err := goal.ReadCommitGoals(root, tip)
	if err != nil {
		coverage.Missing = 1
		coverage.Details = append(coverage.Details, cleanLine(err.Error()))
		return records, coverage, tip
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if filepath.Base(path) == "backlog.md" {
			continue
		}
		coverage.Found++
		parsed, problems := goal.ParseFile(files[path])
		if len(problems) > 0 {
			coverage.Rejected++
			coverage.Details = append(coverage.Details, "path="+path+" "+string(problems[0]))
			continue
		}
		records[parsed.Id] = goalRecord{Path: path, File: parsed}
	}
	if coverage.Found == 0 {
		coverage.Missing = 1
	}
	return records, coverage, tip
}

func number(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	}
	return 0, false
}

func containsTarget(targets []string, goalID string) bool {
	if goalID == "" {
		return true
	}
	for _, target := range targets {
		if target == goalID {
			return true
		}
	}
	return false
}
