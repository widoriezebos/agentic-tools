package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// The conformance gate. The review stage computes the implementer
// worktree's exact review object from an isolated index and checks changed
// paths against the cumulative union of immutable per-round declarations.
// The merge stage leaves review artifacts untouched and requires either a
// mechanically valid waiver or a closed, independent code-critic chain over
// the branch's final committed tree.

var conformanceJobID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// quoted formats a decoded JSON value for a refusal message: strings in
// single quotes, null as None, numbers plainly. The quoting style is part of
// the gate's message contract, which the conformance fixtures assert.
func quoted(value any) string {
	switch v := value.(type) {
	case nil:
		return "None"
	case bool:
		if v {
			return "True"
		}
		return "False"
	case string:
		return "'" + v + "'"
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'g', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func quotedList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = "'" + item + "'"
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

type conformanceRun struct {
	root          string
	job           string
	record        map[string]any
	workspace     string
	baseSha       string
	roundText     string
	rootJob       string
	installPrefix string
	boundaryBase  string
	targetSha     string

	out  []string
	errs []string
}

func (r *conformanceRun) git(dir string, env []string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	stdout, err := cmd.Output()
	return strings.TrimSpace(string(stdout)), err
}

func (r *conformanceRun) gitBytes(dir string, env []string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd.Output()
}

func (r *conformanceRun) fail(lines ...string) ([]string, []string, int) {
	r.errs = append(r.errs, lines...)
	return r.out, r.errs, 1
}

// installationPath strips the checkout's own prefix within its repository so
// a path can be compared against installation-relative rules.
func (r *conformanceRun) installationPath(path string) string {
	normalized := strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	prefix := strings.Trim(strings.ReplaceAll(r.installPrefix, "\\", "/"), "/")
	if prefix != "" && strings.HasPrefix(normalized, prefix+"/") {
		return normalized[len(prefix)+1:]
	}
	return normalized
}

// controlPlaneTampered reports whether the delegate worktree's normally
// ignored agent control plane contains any file.
func (r *conformanceRun) controlPlaneTampered() bool {
	control := filepath.Join(r.workspace, r.installPrefix, "artifacts", "agents")
	tampered := false
	filepath.Walk(control, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			tampered = true
		}
		return nil
	})
	return tampered
}

func nulSplitPaths(data []byte) []string {
	var paths []string
	for _, item := range strings.Split(string(data), "\x00") {
		if item != "" {
			paths = append(paths, item)
		}
	}
	return paths
}

func unprefixMetasystem(value string) string {
	return strings.TrimPrefix(value, "metasystem/")
}

// resolveFacts reads the implementer job record and walks its parent chain
// to the root job, reproducing the fact-resolution program's messages.
func (r *conformanceRun) resolveFacts() []string {
	jobsDir := filepath.Join(r.root, "artifacts", "agents", "jobs")
	data, err := os.ReadFile(filepath.Join(jobsDir, r.job+".json"))
	if err != nil {
		return []string{fmt.Sprintf("malformed job record: %v", err)}
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return []string{fmt.Sprintf("malformed job record: %v", err)}
	}
	record, ok := parsed.(map[string]any)
	if !ok {
		return []string{"conformance review is only defined for implementer records"}
	}
	if record["role"] != "implementer" {
		return []string{"conformance review is only defined for implementer records"}
	}
	r.record = record
	rootJob := r.job
	parent := record["parentJob"]
	seen := map[string]bool{}
	for parent != nil {
		name, ok := parent.(string)
		if !ok || seen[name] {
			return []string{"parent chain is malformed or contains a cycle"}
		}
		seen[name] = true
		parentData, err := os.ReadFile(filepath.Join(jobsDir, name+".json"))
		var parentRecord map[string]any
		if err == nil {
			err = json.Unmarshal(parentData, &parentRecord)
		}
		if err != nil {
			return []string{fmt.Sprintf("parent job record %s is unreadable: %v", quoted(name), err)}
		}
		rootJob = name
		parent = parentRecord["parentJob"]
	}
	workspace, ok := record["workspaceRoot"].(string)
	if !ok {
		return []string{"malformed job record: workspaceRoot is missing"}
	}
	baseSha, ok := record["baseSha"].(string)
	if !ok {
		return []string{"malformed job record: baseSha is missing"}
	}
	round, present := record["round"]
	if !present {
		return []string{"malformed job record: round is missing"}
	}
	r.workspace = workspace
	r.baseSha = baseSha
	r.roundText = scalarText(round)
	r.rootJob = rootJob
	return nil
}

// scalarText renders a decoded JSON scalar plainly: integers without a
// decimal point, strings as themselves.
func scalarText(value any) string {
	switch v := value.(type) {
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'g', -1, 64)
	case string:
		return v
	case nil:
		return "None"
	case bool:
		if v {
			return "True"
		}
		return "False"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Conformance implements `validate conformance`: stage review or merge for
// one implementer job. It returns stdout lines, stderr lines, and the exit
// code, matching the shell gate's contract exactly.
func Conformance(root, stage, job string) (out, errs []string, code int) {
	r := &conformanceRun{root: root, job: job}
	recordPath := filepath.Join(root, "artifacts", "agents", "jobs", job+".json")
	if _, err := os.Stat(recordPath); err != nil {
		return r.fail("conformance failure: unknown job: " + job)
	}
	if factErrors := r.resolveFacts(); factErrors != nil {
		return r.fail(append(factErrors, "conformance failure: could not resolve implementer job facts")...)
	}
	roundDir := filepath.Join(root, "artifacts", "agents", r.rootJob, "rounds", r.roundText)
	returnFile := filepath.Join(roundDir, "return.json")
	diffFile := filepath.Join(roundDir, "diff.patch")
	reviewFile := filepath.Join(roundDir, "review.json")
	if info, err := os.Stat(roundDir); err != nil || !info.IsDir() {
		return r.fail("conformance failure: implementer round return is missing")
	}
	if info, err := os.Stat(returnFile); err != nil || info.IsDir() {
		return r.fail("conformance failure: implementer round return is missing")
	}
	if _, err := r.git(r.workspace, nil, "cat-file", "-e", r.baseSha+"^{commit}"); err != nil {
		return r.fail("conformance failure: baseSha is not a commit in the implementer workspace")
	}
	rootTop, _ := r.git(root, nil, "rev-parse", "--show-toplevel")
	workspaceTop, _ := r.git(r.workspace, nil, "rev-parse", "--show-toplevel")
	if rootTop == workspaceTop {
		return r.fail("conformance failure: invoke this command from the merge-target checkout, not the implementer workspace")
	}
	targetSha, err := r.git(root, nil, "rev-parse", "HEAD^{commit}")
	if err != nil {
		return r.fail("conformance failure: current merge target is not a commit")
	}
	r.targetSha = targetSha
	boundaryBase, err := r.git(r.workspace, nil, "merge-base", targetSha, "HEAD")
	if err != nil {
		return r.fail("conformance failure: implementer branch has no merge-base with the current target")
	}
	r.boundaryBase = boundaryBase
	prefix, _ := r.git(root, nil, "rev-parse", "--show-prefix")
	r.installPrefix = strings.TrimSuffix(prefix, "/")

	if stage == "review" {
		return r.reviewStage(diffFile, reviewFile)
	}
	return r.mergeStage(recordPath)
}

// reviewStage snapshots the worktree in an isolated index, persists the
// diff artifact, and checks the changed paths against the cumulative
// declared boundary. review.json is written only when the boundary holds.
func (r *conformanceRun) reviewStage(diffFile, reviewFile string) ([]string, []string, int) {
	snapshotDir, err := os.MkdirTemp("", "metasystem-conformance.")
	if err != nil {
		return r.fail(fmt.Sprintf("conformance failure: %v", err))
	}
	defer os.RemoveAll(snapshotDir)
	index := []string{"GIT_INDEX_FILE=" + filepath.Join(snapshotDir, "index")}
	if _, err := r.git(r.workspace, index, "read-tree", "HEAD"); err != nil {
		return r.fail("conformance failure: could not snapshot the implementer worktree")
	}
	if _, err := r.git(r.workspace, index, "add", "-A", "--", "."); err != nil {
		return r.fail("conformance failure: could not snapshot the implementer worktree")
	}
	reviewedTree, err := r.git(r.workspace, index, "write-tree")
	if err != nil {
		return r.fail("conformance failure: could not snapshot the implementer worktree")
	}
	diff, err := r.gitBytes(r.workspace, index, "diff", "--cached", "--binary", "--no-renames", r.boundaryBase, "--")
	if err != nil {
		return r.fail("conformance failure: could not snapshot the implementer worktree")
	}
	if err := os.WriteFile(diffFile, diff, 0o644); err != nil {
		return r.fail(fmt.Sprintf("conformance failure: %v", err))
	}
	rawPaths, err := r.gitBytes(r.workspace, index, "diff", "--cached", "--name-only", "-z", "--no-renames", r.boundaryBase, "--")
	if err != nil {
		return r.fail("conformance failure: could not snapshot the implementer worktree")
	}
	paths := nulSplitPaths(rawPaths)

	var violations []string
	for _, path := range paths {
		normalized := r.installationPath(path)
		if normalized == "plans" || strings.HasPrefix(normalized, "plans/") {
			violations = append(violations, "trusted plans/ state changed: "+path)
		}
		if normalized == "artifacts/agents" || strings.HasPrefix(normalized, "artifacts/agents/") {
			violations = append(violations, "agent control plane changed: "+path)
		}
	}
	if r.controlPlaneTampered() {
		violations = append(violations, "agent control plane contains delegate-created files")
	}

	currentRound := 0
	if parsed, err := strconv.Atoi(r.roundText); err == nil {
		currentRound = parsed
	} else {
		violations = append(violations, fmt.Sprintf("implementer round is not an integer: %s", quoted(r.roundText)))
	}
	// The chain's true round is the MAX across its job records — the root
	// record stays at round 1 while follow-ups live in -rN records.
	jobsDir := filepath.Join(r.root, "artifacts", "agents", "jobs")
	if entries, err := os.ReadDir(jobsDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasPrefix(name, r.rootJob) || !strings.HasSuffix(name, ".json") {
				continue
			}
			record, ok := readJobRecord(jobsDir, strings.TrimSuffix(name, ".json"))
			if !ok {
				continue
			}
			if round, ok := record["round"].(float64); ok && int(round) > currentRound {
				currentRound = int(round)
			}
		}
	}
	declared := map[string]bool{}
	roundsRoot := filepath.Join(r.root, "artifacts", "agents", r.rootJob, "rounds")
	var roundNames []string
	if entries, err := os.ReadDir(roundsRoot); err == nil {
		for _, entry := range entries {
			roundNames = append(roundNames, entry.Name())
		}
	}
	sort.Strings(roundNames)
	for _, name := range roundNames {
		candidate := filepath.Join(roundsRoot, name, "return.json")
		if _, err := os.Stat(candidate); err != nil {
			continue
		}
		candidateRound, err := strconv.Atoi(name)
		if err != nil {
			continue
		}
		if candidateRound > currentRound {
			continue
		}
		data, err := os.ReadFile(candidate)
		var parsed any
		if err == nil {
			err = json.Unmarshal(data, &parsed)
		}
		if err != nil {
			violations = append(violations, fmt.Sprintf("round %d return.json is unreadable: %v", candidateRound, err))
			continue
		}
		result, _ := parsed.(map[string]any)
		claim, isList := result["diffBoundary"].([]any)
		items := make([]string, 0, len(claim))
		if isList {
			for _, item := range claim {
				text, ok := item.(string)
				if !ok {
					isList = false
					break
				}
				items = append(items, text)
			}
		}
		if !isList {
			violations = append(violations, fmt.Sprintf("round %d return diffBoundary is not an array of paths", candidateRound))
			continue
		}
		for _, item := range items {
			declared[unprefixMetasystem(item)] = true
		}
	}
	outsideSet := map[string]bool{}
	for _, path := range paths {
		normalized := unprefixMetasystem(path)
		if !declared[normalized] {
			outsideSet[normalized] = true
		}
	}
	outside := make([]string, 0, len(outsideSet))
	for path := range outsideSet {
		outside = append(outside, path)
	}
	sort.Strings(outside)
	if len(outside) > 0 {
		violations = append(violations, fmt.Sprintf(
			"changed paths fall outside the cumulative implementation boundary: %s; some implementation round must declare every changed path",
			quotedList(outside)))
	}
	if len(violations) > 0 {
		for _, violation := range violations {
			r.errs = append(r.errs, "conformance failure: "+violation)
		}
		return r.out, r.errs, 1
	}

	review := map[string]string{
		"diffArtifact":   filepath.Base(diffFile),
		"implementerJob": r.job,
		"reviewedTree":   reviewedTree,
	}
	encoded, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return r.fail(fmt.Sprintf("conformance failure: %v", err))
	}
	if err := os.WriteFile(reviewFile, append(encoded, '\n'), 0o644); err != nil {
		return r.fail(fmt.Sprintf("conformance failure: %v", err))
	}
	r.out = append(r.out, "reviewedTree="+reviewedTree, "diffArtifact="+diffFile)
	return r.out, r.errs, 0
}

func (r *conformanceRun) configGet(key, def string) string {
	value, _, err := config.Get(config.GetParams{
		Key: key, Default: def, DefaultSet: true,
		ConfPath: filepath.Join(r.root, "metasystem.conf"),
	})
	if err != nil {
		return def
	}
	return value
}

// mergeStage validates either a mechanically valid waiver or a closed,
// independent code-critic chain over the final committed tree.
func (r *conformanceRun) mergeStage(recordPath string) ([]string, []string, int) {
	finalTree, err := r.git(r.workspace, nil, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return r.fail("conformance failure: implementer branch has no final committed tree")
	}
	configuredRuntime := r.configGet("role.code-critic.runtime", "__missing__")
	independence := r.configGet("independence", "")

	instructionPathsFile := filepath.Join(r.root, "scripts", "agents", "instruction-bearing-paths.txt")
	instructionData, err := os.ReadFile(instructionPathsFile)
	if err != nil {
		return r.fail(fmt.Sprintf("conformance failure: instruction-bearing path list is unreadable: %v", err))
	}
	var instructionPaths []string
	seenInstruction := map[string]bool{}
	duplicate := false
	for _, line := range strings.Split(string(instructionData), "\n") {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}
		if seenInstruction[entry] {
			duplicate = true
		}
		seenInstruction[entry] = true
		instructionPaths = append(instructionPaths, entry)
	}
	if len(instructionPaths) == 0 || duplicate {
		return r.fail("conformance failure: instruction-bearing path list is empty or contains duplicates")
	}
	isInstructionBearing := func(path string) bool {
		normalized := r.installationPath(path)
		for _, entry := range instructionPaths {
			if strings.HasSuffix(entry, "/") {
				directory := entry[:len(entry)-1]
				if normalized == directory || strings.HasPrefix(normalized, entry) {
					return true
				}
			} else if normalized == entry {
				return true
			}
		}
		return false
	}

	waiver := r.record["critiqueWaived"]
	if waiver != nil {
		rawPaths, _ := r.gitBytes(r.workspace, nil, "diff", "--name-only", "-z", "--no-renames", r.boundaryBase, "HEAD", "--")
		numstat, _ := r.gitBytes(r.workspace, nil, "diff", "--numstat", "--no-renames", r.boundaryBase, "HEAD", "--")
		return r.mergeWaiver(waiver, nulSplitPaths(rawPaths), string(numstat), isInstructionBearing)
	}
	return r.mergeCritique(recordPath, finalTree, configuredRuntime, independence)
}

func (r *conformanceRun) mergeWaiver(waiver any, paths []string, numstat string, isInstructionBearing func(string) bool) ([]string, []string, int) {
	var violations []string
	for _, path := range paths {
		normalized := r.installationPath(path)
		if normalized == "plans" || strings.HasPrefix(normalized, "plans/") {
			violations = append(violations, "trusted plans/ state changed: "+path)
		}
		if normalized == "artifacts/agents" || strings.HasPrefix(normalized, "artifacts/agents/") {
			violations = append(violations, "agent control plane changed: "+path)
		}
	}
	if r.controlPlaneTampered() {
		violations = append(violations, "agent control plane contains delegate-created files")
	}
	var waiverClass any
	if claim, isObject := waiver.(map[string]any); isObject && exactlyClassKey(claim) {
		waiverClass = claim["class"]
	} else {
		violations = append(violations, "critiqueWaived must be an object containing exactly the claimed class")
	}
	if waiverClass != "prose-under-30" {
		violations = append(violations, fmt.Sprintf(
			"unsupported critique waiver class %s; the only class is prose-under-30", quoted(waiverClass)))
	}
	var nonMarkdown, instructionHits []string
	for _, path := range paths {
		if !strings.HasSuffix(r.installationPath(path), ".md") {
			nonMarkdown = append(nonMarkdown, path)
		}
		if isInstructionBearing(path) {
			instructionHits = append(instructionHits, path)
		}
	}
	if len(nonMarkdown) > 0 {
		violations = append(violations, fmt.Sprintf("prose-under-30 includes non-Markdown paths: %s", quotedList(nonMarkdown)))
	}
	if len(instructionHits) > 0 {
		violations = append(violations, fmt.Sprintf(
			"prose-under-30 touches instruction-bearing paths that are never waivable: %s", quotedList(instructionHits)))
	}
	changedLines := 0
	binary := false
	for _, line := range strings.Split(numstat, "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 2 || !allDigits(fields[0]) || !allDigits(fields[1]) {
			binary = true
			continue
		}
		added, _ := strconv.Atoi(fields[0])
		deleted, _ := strconv.Atoi(fields[1])
		changedLines += added + deleted
	}
	if binary {
		violations = append(violations, "prose-under-30 contains a binary or uncountable diff")
	}
	if changedLines > 30 {
		violations = append(violations, fmt.Sprintf(
			"prose-under-30 changes %d lines; the maximum is 30 additions plus deletions", changedLines))
	}
	if len(violations) > 0 {
		for _, violation := range violations {
			r.errs = append(r.errs, "conformance failure: critique waiver mismatch: "+violation)
		}
		return r.out, r.errs, 1
	}

	currentStream := r.streamFor(r.rootJob)
	count := 0
	jobsDir := filepath.Join(r.root, "artifacts", "agents", "jobs")
	if entries, err := os.ReadDir(jobsDir); err == nil {
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			candidate, ok := readJobRecord(jobsDir, strings.TrimSuffix(entry.Name(), ".json"))
			if !ok || candidate["role"] != "implementer" || candidate["parentJob"] != nil {
				continue
			}
			candidateClaim, ok := candidate["critiqueWaived"].(map[string]any)
			if !ok || candidateClaim["class"] != "prose-under-30" {
				continue
			}
			candidateJob, _ := candidate["jobId"].(string)
			if r.streamFor(candidateJob) == currentStream {
				count++
			}
		}
	}
	r.out = append(r.out, fmt.Sprintf(
		"critique waiver accepted and counted: class=prose-under-30 stream=%s count=%d changedLines=%d",
		quoted(currentStream), count, changedLines))
	return r.out, r.errs, 0
}

// exactlyClassKey reports whether a decoded waiver object has exactly the
// key set {class}; the value itself may be anything, including null.
func exactlyClassKey(claim map[string]any) bool {
	if len(claim) != 1 {
		return false
	}
	_, present := claim["class"]
	return present
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (r *conformanceRun) streamFor(rootJob string) string {
	brief := filepath.Join(r.root, "artifacts", "agents", rootJob, "brief.md")
	data, err := os.ReadFile(brief)
	if err != nil {
		return "standalone"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "Mission Stream:") {
			continue
		}
		value := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		if value == "" {
			return "standalone"
		}
		return value
	}
	return "standalone"
}

// loadConformanceRecords reads every job record whose jobId matches its
// file name, keyed by job id.
func (r *conformanceRun) loadConformanceRecords() map[string]map[string]any {
	records := map[string]map[string]any{}
	jobsDir := filepath.Join(r.root, "artifacts", "agents", "jobs")
	entries, err := os.ReadDir(jobsDir)
	if err != nil {
		return records
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		stem := strings.TrimSuffix(name, ".json")
		record, ok := readJobRecord(jobsDir, stem)
		if ok && record["jobId"] == stem {
			records[stem] = record
		}
	}
	return records
}

func chainRootIn(records map[string]map[string]any, jobID string) (string, bool) {
	seen := map[string]bool{}
	current := jobID
	for {
		if seen[current] {
			return "", false
		}
		record, present := records[current]
		if !present {
			return "", false
		}
		seen[current] = true
		parent := record["parentJob"]
		if parent == nil {
			return current, true
		}
		name, ok := parent.(string)
		if !ok {
			return "", false
		}
		current = name
	}
}

// enumerates reports whether text names the finding id as a whole word,
// never as a fragment of a longer identifier.
func enumerates(text, findingID string) bool {
	isWord := func(b byte) bool {
		return b == '-' || b == '_' ||
			(b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
	}
	for offset := 0; ; {
		position := strings.Index(text[offset:], findingID)
		if position < 0 {
			return false
		}
		start := offset + position
		end := start + len(findingID)
		beforeOK := start == 0 || !isWord(text[start-1])
		afterOK := end == len(text) || !isWord(text[end])
		if beforeOK && afterOK {
			return true
		}
		offset = start + 1
	}
}

func (r *conformanceRun) mergeCritique(recordPath, finalTree, configuredRuntime, independence string) ([]string, []string, int) {
	records := r.loadConformanceRecords()
	implementationIDs := map[string]bool{}
	for jobID := range records {
		if root, ok := chainRootIn(records, jobID); ok && root == r.rootJob {
			implementationIDs[jobID] = true
		}
	}
	var criticIDs []string
	for jobID, record := range records {
		if record["role"] != "code-critic" || record["parentJob"] != nil {
			continue
		}
		if reviews, ok := record["reviews"].(string); ok && implementationIDs[reviews] {
			criticIDs = append(criticIDs, jobID)
		}
	}
	sort.Strings(criticIDs)
	implementerJob := r.record["jobId"]
	if len(criticIDs) == 0 {
		r.errs = append(r.errs, fmt.Sprintf(
			"conformance failure: merge requires a code-critic chain whose reviews field names implementer job %s; dispatch that role with --reviews %s",
			quoted(implementerJob), scalarText(implementerJob)))
		if configuredRuntime == "__missing__" {
			r.errs = append(r.errs,
				"conformance failure: the code-critic role is unconfigured; set the exact key role.code-critic.runtime")
		}
		return r.out, r.errs, 1
	}

	successorText := func(successorJob string) (string, bool) {
		record, present := records[successorJob]
		if !present {
			return "", false
		}
		round, ok := record["round"].(float64)
		if !ok || round != float64(int64(round)) {
			return "", false
		}
		prompt := filepath.Join(r.root, "artifacts", "agents", r.rootJob,
			"rounds", strconv.FormatInt(int64(round), 10), "prompt.md")
		data, err := os.ReadFile(prompt)
		if err != nil {
			return "", false
		}
		return string(data), true
	}

	type diagnostic struct {
		criticID string
		failures []string
	}
	var diagnostics []diagnostic
	for _, criticID := range criticIDs {
		criticRoot := records[criticID]
		var failures []string
		var members []map[string]any
		for jobID, record := range records {
			if root, ok := chainRootIn(records, jobID); ok && root == criticID {
				members = append(members, record)
			}
		}
		if len(members) == 0 {
			diagnostics = append(diagnostics, diagnostic{criticID, []string{"has no readable rounds"}})
			continue
		}
		final := members[0]
		finalScore := func(record map[string]any) float64 {
			if round, ok := record["round"].(float64); ok {
				return round
			}
			return -1
		}
		for _, member := range members[1:] {
			if finalScore(member) > finalScore(final) {
				final = member
			}
		}
		finalRound := final["round"]
		if criticRoot["chainClosed"] != true {
			failures = append(failures, "is not closed")
		}
		if final["status"] != "completed" {
			failures = append(failures, fmt.Sprintf("final round status is %s, not completed", quoted(final["status"])))
		}
		returnPath := filepath.Join(r.root, "artifacts", "agents", criticID,
			"rounds", scalarText(finalRound), "return.json")
		result := map[string]any{}
		returnData, err := os.ReadFile(returnPath)
		var parsedReturn any
		if err == nil {
			err = json.Unmarshal(returnData, &parsedReturn)
		}
		if err != nil {
			failures = append(failures, fmt.Sprintf("code-critic chain %s final return is unreadable: %v", quoted(criticID), err))
		} else if object, ok := parsedReturn.(map[string]any); ok {
			result = object
		} else {
			failures = append(failures, fmt.Sprintf("code-critic chain %s final return is not a JSON object", quoted(criticID)))
		}

		var materialIDs []string
		findings, findingsIsList := result["findings"].([]any)
		if findingsIsList {
			for _, item := range findings {
				finding, ok := item.(map[string]any)
				if !ok || finding["material"] != true {
					continue
				}
				if id, ok := finding["id"].(string); ok {
					materialIDs = append(materialIDs, id)
				} else {
					materialIDs = append(materialIDs, "<unnamed>")
				}
			}
		} else {
			failures = append(failures, "final return has no findings array")
		}
		verdict := result["verdictMaterialCount"]
		verdictZero := verdict == float64(0)
		if len(materialIDs) > 0 || !verdictZero {
			detail := "reported count " + quoted(verdict)
			if len(materialIDs) > 0 {
				detail = strings.Join(materialIDs, ", ")
			}
			failures = append(failures, "final round still has material findings despite any dispositions: "+detail)
		}

		exhaustions, exhaustionsIsList := criticRoot["critiqueExhaustions"].([]any)
		if criticRoot["critiqueExhaustions"] != nil && !exhaustionsIsList {
			failures = append(failures, "critiqueExhaustions is not an array")
			exhaustions = nil
		}
		if len(exhaustions) > 1 {
			failures = append(failures,
				"a second critique exhaustion is refused outright; waiting on the human is the only remedy")
		}
		for index, item := range exhaustions {
			exhaustion, ok := item.(map[string]any)
			expected := map[string]bool{"round": true, "openFindingIds": true, "successorJobId": true}
			shapeOK := ok && len(exhaustion) == len(expected)
			if shapeOK {
				for key := range expected {
					if _, present := exhaustion[key]; !present {
						shapeOK = false
					}
				}
			}
			if !shapeOK {
				failures = append(failures, fmt.Sprintf(
					"critiqueExhaustions[%d] must contain exactly round, openFindingIds, and successorJobId", index))
				continue
			}
			exhaustedRound, roundIsNumber := exhaustion["round"].(float64)
			roundValid := roundIsNumber && exhaustedRound == float64(int64(exhaustedRound)) && exhaustedRound >= 1
			if !roundValid {
				failures = append(failures, fmt.Sprintf(
					"critiqueExhaustions[%d].round must be a positive round number", index))
			}
			rawIDs, idsIsList := exhaustion["openFindingIds"].([]any)
			var openIDs []string
			idsValid := idsIsList && len(rawIDs) > 0
			if idsValid {
				seenIDs := map[string]bool{}
				for _, rawID := range rawIDs {
					id, ok := rawID.(string)
					if !ok || id == "" || seenIDs[id] {
						idsValid = false
						break
					}
					seenIDs[id] = true
					openIDs = append(openIDs, id)
				}
			}
			if !idsValid {
				failures = append(failures, fmt.Sprintf(
					"critiqueExhaustions[%d].openFindingIds must be a nonempty list of unique finding identifiers", index))
				continue
			}
			successor, successorIsString := exhaustion["successorJobId"].(string)
			if !successorIsString || !conformanceJobID.MatchString(successor) {
				failures = append(failures, fmt.Sprintf(
					"critiqueExhaustions[%d].successorJobId is not a valid job identifier", index))
				continue
			}
			successorRecord, present := records[successor]
			successorRoot, successorRooted := chainRootIn(records, successor)
			if !present || successorRecord["role"] != "implementer" || successorRecord["parentJob"] == nil ||
				!successorRooted || successorRoot != r.rootJob {
				failures = append(failures, fmt.Sprintf(
					"successor job %s is not an implementer follow-up in the reviewed implementation chain", quoted(successor)))
				continue
			}
			text, textOK := successorText(successor)
			var missing []string
			for _, findingID := range openIDs {
				if !textOK || !enumerates(text, findingID) {
					missing = append(missing, findingID)
				}
			}
			if len(missing) > 0 {
				failures = append(failures, fmt.Sprintf(
					"successor job %s prompt does not enumerate open findings: %s", quoted(successor), strings.Join(missing, ", ")))
			}
			finalRoundNumber, finalRoundIsNumber := finalRound.(float64)
			finalRoundValid := finalRoundIsNumber && finalRoundNumber == float64(int64(finalRoundNumber))
			if !finalRoundValid || (roundValid && finalRoundNumber <= exhaustedRound) || len(materialIDs) > 0 || !verdictZero {
				failures = append(failures, fmt.Sprintf(
					"critique exhausted at round %s with open material findings: %s",
					scalarText(exhaustion["round"]), strings.Join(openIDs, ", ")))
			}
		}

		reviewedTree := result["reviewedTree"]
		if reviewedTree != finalTree {
			failures = append(failures, fmt.Sprintf(
				"reviewed tree %s is stale; the implementer branch final committed tree is %s",
				quoted(reviewedTree), quoted(finalTree)))
		}

		implementerModel, implementerModelOK := r.record["effectiveModel"].(string)
		criticModel, _ := final["effectiveModel"].(string)
		if !implementerModelOK || implementerModel == "" {
			failures = append(failures, fmt.Sprintf("implementer job %s has no effective model", quoted(implementerJob)))
		}
		if criticFinalModel, ok := final["effectiveModel"].(string); !ok || criticFinalModel == "" {
			failures = append(failures, fmt.Sprintf("code-critic chain %s final round has no effective model", quoted(criticID)))
		}
		if implementerModelOK && implementerModel != "" && implementerModel == criticModel && independence != "session-only" {
			failures = append(failures, fmt.Sprintf(
				"independence refused: implementer job %s uses effective model %s, and code-critic chain %s uses effective model %s; remedy one is to dispatch a critic on a different model; remedy two is to declare independence=session-only in configuration",
				quoted(implementerJob), quoted(implementerModel), quoted(criticID), quoted(criticModel)))
		}

		if len(failures) == 0 {
			if implementerModel == criticModel && independence == "session-only" {
				r.out = append(r.out, fmt.Sprintf(
					"merge critique accepted with independence=session-only recorded in gate evidence: implementer job %s and code-critic chain %s both use effective model %s; their sessions alone are independent; both agree on tree %s",
					scalarText(implementerJob), criticID, implementerModel, finalTree))
			} else {
				r.out = append(r.out, fmt.Sprintf(
					"merge critique accepted with model independence: implementer job %s uses effective model %s, code-critic chain %s uses effective model %s, and both agree on tree %s",
					scalarText(implementerJob), implementerModel, criticID, criticModel, finalTree))
			}
			return r.out, r.errs, 0
		}
		diagnostics = append(diagnostics, diagnostic{criticID, failures})
	}
	for _, entry := range diagnostics {
		for _, failure := range entry.failures {
			r.errs = append(r.errs, fmt.Sprintf("conformance failure: code-critic chain %s: %s", entry.criticID, failure))
		}
	}
	return r.out, r.errs, 1
}
