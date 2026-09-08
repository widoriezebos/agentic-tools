package validate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
)

// FullBatteryCommand is the existing landing full-width command, exported
// here so recertification selection and landing consumption bind the same
// bytes. internal/landing retains its compatibility alias.
const FullBatteryCommand = "scripts/agents/go-gate.sh --fast && scripts/agents/dispatch-fixtures.sh && scripts/agents/goal-cli-fixtures.sh"

const recertificationPreparationLimit = 5 * time.Minute

var recertificationTreeOID = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

// RecertificationRecord is the immutable proof joining the original reviewed
// output to one frozen integration target and one mechanically merged tree.
type RecertificationRecord struct {
	SchemaVersion           int                         `json:"schemaVersion"`
	ProofKind               string                      `json:"proofKind"`
	GitVersion              string                      `json:"gitVersion"`
	HunkManifest            []gittree.HunkManifestEntry `json:"hunkManifest"`
	HunkManifestDigest      string                      `json:"hunkManifestDigest"`
	RootChain               string                      `json:"rootChain"`
	CertifiedImplementerJob string                      `json:"certifiedImplementerJob"`
	ReviewArtifact          string                      `json:"reviewArtifact"`
	ReviewDigest            string                      `json:"reviewDigest"`
	PatchArtifact           string                      `json:"patchArtifact"`
	PatchDigest             string                      `json:"patchDigest"`
	CriticRoot              string                      `json:"criticRoot"`
	CriticTerminalRound     int                         `json:"criticTerminalRound"`
	CriticReturnArtifact    string                      `json:"criticReturnArtifact"`
	CriticReturnDigest      string                      `json:"criticReturnDigest"`
	SourceHead              string                      `json:"sourceHead"`
	BaseCommit              string                      `json:"baseCommit"`
	BaseTree                string                      `json:"baseTree"`
	ReviewedTree            string                      `json:"reviewedTree"`
	TargetCommit            string                      `json:"targetCommit"`
	TargetTree              string                      `json:"targetTree"`
	MergedTree              string                      `json:"mergedTree"`
	SourceWholeTree         string                      `json:"sourceWholeTree"`
	MergedWholeTree         string                      `json:"mergedWholeTree"`
	SourceAnchorRef         string                      `json:"sourceAnchorRef"`
	MergedAnchorRef         string                      `json:"mergedAnchorRef"`
	CertifiedPaths          []string                    `json:"certifiedPaths"`
	DiffArtifact            string                      `json:"diffArtifact"`
	MergedPatchDigest       string                      `json:"mergedPatchDigest"`
	GateWidth               string                      `json:"gateWidth"`
	TestCommand             string                      `json:"testCommand"`
	InputDigest             string                      `json:"inputDigest"`
	RecordDigest            string                      `json:"recordDigest"`
}

// RecertificationFailure is the stable refusal contract for producer and
// verifier. Reason is one of the design's operator-facing refusal codes.
type RecertificationFailure struct {
	Reason string
	Detail string
	Err    error
}

func (e *RecertificationFailure) Error() string {
	message := e.Reason
	if e.Detail != "" {
		message += " detail=" + e.Detail
	}
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return message
}

func (e *RecertificationFailure) Unwrap() error { return e.Err }

// VerifiedRecertification is the read-only verifier's admitted projection.
type VerifiedRecertification struct {
	Record     RecertificationRecord
	RecordPath string
	Patch      []byte
}

type originalCertification struct {
	reviewPath  string
	reviewBytes []byte
	patchPath   string
	patch       []byte
	reviewed    string
	criticRoot  string
	criticRound int
	criticPath  string
	criticBytes []byte
}

func sha256Bytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func repoRelative(root, path string) (string, error) {
	top, err := (gittree.Workspace{Dir: root}).TopLevel()
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		path = resolved
	}
	if resolved, resolveErr := filepath.EvalSymlinks(top); resolveErr == nil {
		top = resolved
	}
	relative, err := filepath.Rel(top, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %s is outside repository %s", path, top)
	}
	return filepath.ToSlash(relative), nil
}

func artifactAbsolute(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) || filepath.Clean(relative) != filepath.FromSlash(relative) ||
		relative == ".." || strings.HasPrefix(relative, "../") || strings.ContainsRune(relative, '\x00') {
		return "", fmt.Errorf("artifact path %q is not canonical repository-relative", relative)
	}
	top, err := (gittree.Workspace{Dir: root}).TopLevel()
	if err != nil {
		return "", err
	}
	return filepath.Join(top, filepath.FromSlash(relative)), nil
}

// artifactRegularNoFollow rejects a symlink in any repository-relative
// artifact component before bytes are trusted. Recertification evidence is a
// path-bound protocol; following a link would let a valid-looking record name
// mutable bytes outside that protocol location.
func artifactRegularNoFollow(root, relative string) (string, error) {
	absolute, err := artifactAbsolute(root, relative)
	if err != nil {
		return "", err
	}
	top, err := (gittree.Workspace{Dir: root}).TopLevel()
	if err != nil {
		return "", err
	}
	current := filepath.Clean(top)
	for _, component := range strings.Split(filepath.FromSlash(relative), string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("artifact path contains symlink %s", current)
		}
	}
	info, err := os.Lstat(absolute)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("artifact is not a regular file: %w", err)
	}
	return absolute, nil
}

func refuseSymlinkedDirectory(root, directory string) error {
	top, err := (gittree.Workspace{Dir: root}).TopLevel()
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(top, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("artifact directory escapes the repository")
	}
	current := filepath.Clean(top)
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("artifact directory contains symlink %s", current)
		}
	}
	return nil
}

func fileDigest(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	return data, sha256Bytes(data), nil
}

func recertificationInput(record RecertificationRecord) map[string]any {
	return map[string]any{
		"proofKind": record.ProofKind,
		"rootChain": record.RootChain, "certifiedImplementerJob": record.CertifiedImplementerJob,
		"reviewArtifact": record.ReviewArtifact, "reviewDigest": record.ReviewDigest,
		"patchArtifact": record.PatchArtifact, "patchDigest": record.PatchDigest,
		"criticRoot": record.CriticRoot, "criticTerminalRound": record.CriticTerminalRound,
		"criticReturnArtifact": record.CriticReturnArtifact, "criticReturnDigest": record.CriticReturnDigest,
		"sourceHead": record.SourceHead, "baseCommit": record.BaseCommit, "baseTree": record.BaseTree,
		"reviewedTree": record.ReviewedTree, "targetCommit": record.TargetCommit, "targetTree": record.TargetTree,
		"gateWidth": record.GateWidth, "testCommand": record.TestCommand,
	}
}

// RecertificationInputDigest recomputes the canonical immutable-input hash.
func RecertificationInputDigest(record RecertificationRecord) (string, error) {
	return canonicalDigest(recertificationInput(record))
}

// RecertificationRecordDigest recomputes the canonical record hash with its
// self-field omitted.
func RecertificationRecordDigest(record RecertificationRecord) (string, error) {
	record.RecordDigest = ""
	value := map[string]any{}
	encoded, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(encoded, &value); err != nil {
		return "", err
	}
	delete(value, "recordDigest")
	return canonicalDigest(value)
}

func recertificationRelative(root, chain, inputDigest string) (string, error) {
	prefix, err := projectInstallPrefix(root)
	if err != nil {
		return "", err
	}
	parts := []string{}
	if prefix != "" {
		parts = append(parts, prefix)
	}
	parts = append(parts, "artifacts", "agents", "landing", "recertifications", chain, inputDigest, "record.json")
	return filepath.ToSlash(filepath.Join(parts...)), nil
}

func recertificationDir(root, chain, inputDigest string) (string, error) {
	relative, err := recertificationRelative(root, chain, inputDigest)
	if err != nil {
		return "", err
	}
	absolute, err := artifactAbsolute(root, relative)
	if err != nil {
		return "", err
	}
	return filepath.Dir(absolute), nil
}

func decodeStrictRecord(data []byte) (RecertificationRecord, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record RecertificationRecord
	if err := decoder.Decode(&record); err != nil {
		return record, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return record, fmt.Errorf("trailing JSON")
		}
		return record, err
	}
	return record, nil
}

func (r *conformanceRun) rootRecord() (map[string]any, error) {
	data, err := os.ReadFile(filepath.Join(r.root, "artifacts", "agents", "jobs", r.rootJob+".json"))
	if err != nil {
		return nil, err
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	return record, nil
}

func terminalMember(records map[string]map[string]any, root string) (string, map[string]any, int, error) {
	bestID, bestRound := "", 0
	var best map[string]any
	for id, record := range records {
		chain, ok := chainRootIn(records, id)
		if !ok || chain != root {
			continue
		}
		roundValue, ok := jsonInteger(record["round"])
		if !ok || roundValue < 1 || roundValue > int64(^uint(0)>>1) {
			return "", nil, 0, fmt.Errorf("chain member %s has malformed round", id)
		}
		round := int(roundValue)
		if !dispatch.TerminalStatus(fmt.Sprint(record["status"])) {
			return "", nil, 0, fmt.Errorf("chain member %s is active", id)
		}
		if round > bestRound {
			bestID, best, bestRound = id, record, round
		}
	}
	if best == nil {
		return "", nil, 0, fmt.Errorf("chain %s has no members", root)
	}
	return bestID, best, bestRound, nil
}

func (r *conformanceRun) selectOriginalCertification() (originalCertification, error) {
	records := r.loadConformanceRecords()
	rootRecord, ok := records[r.rootJob]
	if !ok || rootRecord["chainClosed"] != true {
		return originalCertification{}, fmt.Errorf("implementation chain is not closed")
	}
	for id, record := range records {
		root, valid := chainRootIn(records, id)
		if valid && root == r.rootJob && !dispatch.TerminalStatus(fmt.Sprint(record["status"])) {
			return originalCertification{}, fmt.Errorf("implementation member %s is active", id)
		}
	}
	criticRoot, ok := rootRecord["independentCritiqueJobRef"].(string)
	if !ok || !conformanceJobID.MatchString(criticRoot) {
		return originalCertification{}, fmt.Errorf("root chain has no independent code-critic reference")
	}
	criticRecord, ok := records[criticRoot]
	if !ok || criticRecord["role"] != "code-critic" || criticRecord["parentJob"] != nil || criticRecord["chainClosed"] != true {
		return originalCertification{}, fmt.Errorf("referenced code-critic chain is not closed")
	}
	expectedImplementer, _ := criticRecord["reviews"].(string)
	if expectedImplementer == "" || expectedImplementer != r.job {
		return originalCertification{}, fmt.Errorf("requested implementer job %s differs from the critic's expected implementer job %s", r.job, expectedImplementer)
	}
	_, terminal, round, err := terminalMember(records, criticRoot)
	if err != nil {
		return originalCertification{}, fmt.Errorf("referenced code-critic terminal round is not completed: %w", err)
	}
	if terminal["status"] != "completed" {
		return originalCertification{}, fmt.Errorf("referenced code-critic terminal round is not completed")
	}
	criticPath := filepath.Join(r.root, "artifacts", "agents", criticRoot, "rounds", strconv.Itoa(round), "return.json")
	criticBytes, _, err := fileDigest(criticPath)
	if err != nil {
		return originalCertification{}, fmt.Errorf("critic return is unreadable: %w", err)
	}
	var criticReturn map[string]any
	if err := json.Unmarshal(criticBytes, &criticReturn); err != nil {
		return originalCertification{}, fmt.Errorf("critic return is malformed: %w", err)
	}
	reviewed, _ := criticReturn["reviewedTree"].(string)
	if !recertificationTreeOID.MatchString(reviewed) {
		return originalCertification{}, fmt.Errorf("critic return has no reviewed tree")
	}

	roundsRoot := filepath.Join(r.root, "artifacts", "agents", r.rootJob, "rounds")
	entries, err := os.ReadDir(roundsRoot)
	if err != nil {
		return originalCertification{}, err
	}
	type candidate struct {
		round       int
		reviewPath  string
		reviewBytes []byte
		patchPath   string
		patch       []byte
	}
	var candidates []candidate
	for _, entry := range entries {
		round, err := strconv.Atoi(entry.Name())
		if err != nil || round < 1 || !entry.IsDir() {
			continue
		}
		reviewPath := filepath.Join(roundsRoot, entry.Name(), "review.json")
		reviewBytes, err := os.ReadFile(reviewPath)
		if err != nil {
			continue
		}
		var review struct {
			DiffArtifact   string `json:"diffArtifact"`
			ImplementerJob string `json:"implementerJob"`
			ReviewedTree   string `json:"reviewedTree"`
		}
		if json.Unmarshal(reviewBytes, &review) != nil || review.DiffArtifact != "diff.patch" ||
			review.ImplementerJob != r.job || review.ReviewedTree != reviewed {
			continue
		}
		patchPath := filepath.Join(filepath.Dir(reviewPath), review.DiffArtifact)
		patch, err := os.ReadFile(patchPath)
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{round, reviewPath, reviewBytes, patchPath, patch})
	}
	if len(candidates) == 0 {
		return originalCertification{}, fmt.Errorf("requested implementer job %s has no review joined to critic tree %s", r.job, reviewed)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].round < candidates[j].round })
	chosen := candidates[0]
	for _, candidate := range candidates[1:] {
		if !bytes.Equal(candidate.reviewBytes, chosen.reviewBytes) || !bytes.Equal(candidate.patch, chosen.patch) {
			return originalCertification{}, fmt.Errorf("conflicting original review copies")
		}
	}
	return originalCertification{reviewPath: chosen.reviewPath, reviewBytes: chosen.reviewBytes,
		patchPath: chosen.patchPath, patch: chosen.patch, reviewed: reviewed,
		criticRoot: criticRoot, criticRound: round, criticPath: criticPath, criticBytes: criticBytes}, nil
}

func commandForWidth(rootRecord map[string]any, supplied string) (string, string, error) {
	width := "area"
	if value, present := rootRecord["gateWidth"]; present && value != nil {
		var ok bool
		width, ok = value.(string)
		if !ok || (width != "area" && width != "full") {
			return "", "", fmt.Errorf("root chain gate width is malformed")
		}
	}
	if width == "area" {
		if strings.TrimSpace(supplied) == "" {
			return "", "", fmt.Errorf("area-width recertification requires an explicit non-whitespace --test-command")
		}
		return width, supplied, nil
	}
	if supplied != "" && supplied != FullBatteryCommand {
		return "", "", fmt.Errorf("full-width --test-command must byte-equal the full battery command")
	}
	return width, FullBatteryCommand, nil
}

func filterRecertificationRefs(refs map[string]string, inputDigest string) map[string]string {
	prefix := "refs/metasystem/landing/recertifications/" + inputDigest + "/"
	copyMap := map[string]string{}
	for name, oid := range refs {
		if !strings.HasPrefix(name, prefix) {
			copyMap[name] = oid
		}
	}
	return copyMap
}

func stagedEntriesAdmitted(workspace gittree.Workspace, headTree, admittedWhole string, posture gittree.StagedPosture) error {
	if len(posture.Unmerged) > 0 {
		return fmt.Errorf("the real index has unresolved entries")
	}
	paths, err := workspace.ChangedPaths(headTree, posture.Tree)
	if err != nil {
		return err
	}
	staged, err := workspace.Entries(posture.Tree, paths)
	if err != nil {
		return err
	}
	head, err := workspace.Entries(headTree, paths)
	if err != nil {
		return err
	}
	admitted, err := workspace.Entries(admittedWhole, paths)
	if err != nil {
		return err
	}
	for _, path := range paths {
		entry := staged[path]
		if entry != head[path] && entry != admitted[path] {
			return fmt.Errorf("the real index carries a third version of %s", path)
		}
	}
	return nil
}

func activeOperation(workspace gittree.Workspace) error {
	pseudorefs, err := workspace.PseudorefCensus()
	if err != nil {
		return err
	}
	for _, ref := range pseudorefs {
		switch ref.Name {
		case "MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_HEAD", "AUTO_MERGE":
			return fmt.Errorf("active Git operation carries %s", ref.Name)
		}
	}
	for _, name := range []string{"rebase-apply", "rebase-merge", "sequencer", "BISECT_LOG"} {
		path, err := workspace.GitPath(name)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("active Git operation carries %s", name)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func recertificationLock(root, chain string, deadline time.Time) (func(), error) {
	lockDir := filepath.Join(root, "artifacts", "agents", "landing", "locks", chain+".d")
	pid, tag := int64(os.Getpid()), "metasystem"
	for {
		err := dispatch.OwnerLockClaim(lockDir, pid, tag)
		if err == nil {
			return func() { _ = dispatch.OwnerLockRelease(lockDir, pid, tag) }, nil
		}
		if !errors.Is(err, dispatch.ErrOwnerLockBusy) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, &RecertificationFailure{Reason: "chain-recertification-timeout", Detail: "lock-wait", Err: err}
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func verifyOriginalCritic(r *conformanceRun, certification originalCertification) error {
	copyRun := *r
	copyRun.out, copyRun.errs = nil, nil
	copyRun.criticRoot = certification.criticRoot
	configuredRuntime := copyRun.configGet("role.code-critic.runtime", "__missing__")
	independence := copyRun.configGet("independence", "")
	_, errs, code := copyRun.mergeCritique(filepath.Join(r.root, "artifacts", "agents", "jobs", r.job+".json"),
		certification.reviewed, configuredRuntime, independence)
	if code != 0 {
		return fmt.Errorf("original critic closure failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

func publishRecertification(r *conformanceRun, record RecertificationRecord, patch []byte, deadline time.Time) (string, error) {
	dir, err := recertificationDir(r.root, record.RootChain, record.InputDigest)
	if err != nil {
		return "", err
	}
	relative, err := recertificationRelative(r.root, record.RootChain, record.InputDigest)
	if err != nil {
		return "", err
	}
	release, err := recertificationLock(r.root, record.RootChain, deadline)
	if err != nil {
		return "", err
	}
	defer release()
	if time.Now().After(deadline) {
		return "", &RecertificationFailure{Reason: "chain-recertification-timeout", Detail: "publication"}
	}
	recordBytes, err := wiredoc.RenderValue(record)
	if err != nil {
		return "", err
	}
	if err := refuseSymlinkedDirectory(r.root, dir); err != nil {
		return "", err
	}
	if existing, err := os.ReadFile(filepath.Join(dir, "record.json")); err == nil {
		existingPatch, patchErr := os.ReadFile(filepath.Join(dir, "diff.patch"))
		if patchErr == nil && bytes.Equal(existing, recordBytes) && bytes.Equal(existingPatch, patch) {
			return relative, nil
		}
		return "", fmt.Errorf("recertification inputs already name different published bytes")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	temporary, err := os.MkdirTemp(parent, ".recertification.")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temporary)
	writeSync := func(path string, data []byte) error {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		if _, err := file.Write(data); err != nil {
			file.Close()
			return err
		}
		if err := file.Sync(); err != nil {
			file.Close()
			return err
		}
		return file.Close()
	}
	if err := writeSync(filepath.Join(temporary, "diff.patch"), patch); err != nil {
		return "", err
	}
	if err := writeSync(filepath.Join(temporary, "record.json"), recordBytes); err != nil {
		return "", err
	}
	temporaryHandle, err := os.Open(temporary)
	if err != nil {
		return "", err
	}
	if err := temporaryHandle.Sync(); err != nil {
		temporaryHandle.Close()
		return "", err
	}
	if err := temporaryHandle.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temporary, dir); err != nil {
		return "", err
	}
	readback, err := os.ReadFile(filepath.Join(dir, "record.json"))
	if err != nil || !bytes.Equal(readback, recordBytes) {
		return "", fmt.Errorf("recertification record read-back failed")
	}
	patchReadback, err := os.ReadFile(filepath.Join(dir, "diff.patch"))
	if err != nil || !bytes.Equal(patchReadback, patch) {
		return "", fmt.Errorf("recertification patch read-back failed")
	}
	if parentHandle, err := os.Open(parent); err == nil {
		if syncErr := parentHandle.Sync(); syncErr != nil {
			parentHandle.Close()
			return "", syncErr
		}
		if closeErr := parentHandle.Close(); closeErr != nil {
			return "", closeErr
		}
	} else {
		return "", err
	}
	return relative, nil
}

func (r *conformanceRun) recertify(testCommand string) ([]string, []string, int) {
	deadline := time.Now().Add(recertificationPreparationLimit)
	fail := func(reason, detail string, err error) ([]string, []string, int) {
		if time.Now().After(deadline) {
			reason = "chain-recertification-timeout"
			detail = "five-minute-preparation"
		}
		return r.fail((&RecertificationFailure{Reason: reason, Detail: detail, Err: err}).Error())
	}
	checkDeadline := func(detail string) ([]string, []string, int, bool) {
		if time.Now().After(deadline) {
			out, errs, code := fail("chain-recertification-timeout", detail, fmt.Errorf("five-minute preparation ceiling reached"))
			return out, errs, code, true
		}
		return nil, nil, 0, false
	}
	rootRecord, err := r.rootRecord()
	if err != nil {
		return fail("chain-recertification-source-changed", "root-record", err)
	}
	width, command, err := commandForWidth(rootRecord, testCommand)
	if err != nil {
		return fail("chain-recertification-test-command-refused", "selection", err)
	}
	certification, err := r.selectOriginalCertification()
	if err != nil {
		return fail("chain-recertification-source-changed", "review-selection", err)
	}
	if err := verifyOriginalCritic(r, certification); err != nil {
		return fail("chain-recertification-unproven", "critic-closure", err)
	}
	if out, errs, code, expired := checkDeadline("original-certification"); expired {
		return out, errs, code
	}
	project := r.projectWorkspace()
	topSource := gittree.Workspace{Dir: r.workspace}
	integration := gittree.Workspace{Dir: r.root}
	h, unborn, err := topSource.HeadCommit()
	if err != nil || unborn {
		return fail("chain-recertification-source-changed", "source-head", err)
	}
	t, unborn, err := integration.HeadCommit()
	if err != nil || unborn {
		return fail("chain-recertification-target-moved", "target-head", err)
	}
	bases, err := project.MergeBases(h, t)
	if err != nil || len(bases) != 1 {
		return fail("chain-recertification-base-unproven", "merge-base", fmt.Errorf("expected one merge base, got %d: %w", len(bases), err))
	}
	a := bases[0]
	b, err := project.TreeOf(a)
	if err != nil {
		return fail("chain-recertification-base-unproven", "base-tree", err)
	}
	treeT, err := integration.TreeOf(t)
	if err != nil {
		return fail("chain-recertification-base-unproven", "target-tree", err)
	}
	applied, err := project.Apply(b, certification.patch)
	if err != nil || applied != certification.reviewed {
		return fail("chain-recertification-base-unproven", "patch-equation", fmt.Errorf("patch application from B produced %s, not R %s: %w", applied, certification.reviewed, err))
	}
	certifiedPaths, err := project.ChangedPaths(b, certification.reviewed)
	if err != nil || len(certifiedPaths) == 0 {
		return fail("chain-recertification-base-unproven", "empty-change", err)
	}
	mergeResult, err := integration.DisjointMerge(b, certification.reviewed, treeT)
	if err != nil {
		var mergeFailure *gittree.DisjointMergeError
		if errors.As(err, &mergeFailure) && mergeFailure.Kind == "overlap" {
			return fail("chain-recertification-overlap", fmt.Sprintf("path=%s chain=%d,%d main=%d,%d", mergeFailure.Path,
				mergeFailure.ChainRange.Start, mergeFailure.ChainRange.Count, mergeFailure.MainRange.Start, mergeFailure.MainRange.Count), err)
		}
		detail := "merge-proof"
		if errors.As(err, &mergeFailure) && mergeFailure.Detail != "" {
			detail = mergeFailure.Detail
		}
		return fail("chain-recertification-unproven", detail, err)
	}
	if out, errs, code, expired := checkDeadline("merge-proof"); expired {
		return out, errs, code
	}
	m := mergeResult.MergedTree
	actualPaths, err := integration.ChangedPaths(treeT, m)
	if err != nil || !reflect.DeepEqual(actualPaths, certifiedPaths) {
		return fail("chain-recertification-unproven", "certified-paths", fmt.Errorf("target-to-merged paths differ from B-to-R paths"))
	}
	q, err := integration.Diff(treeT, m)
	if err != nil {
		return fail("chain-recertification-unproven", "merged-patch", err)
	}
	if replay, applyErr := integration.Apply(treeT, q); applyErr != nil || replay != m {
		return fail("chain-recertification-unproven", "merged-patch-equation", applyErr)
	}
	if violations := r.cumulativeBoundaryViolations(certifiedPaths); len(violations) > 0 {
		return fail("chain-recertification-unproven", "conformance-boundary", fmt.Errorf("%s", strings.Join(violations, "; ")))
	}

	projectSnapshot, err := project.Snapshot("HEAD")
	if err != nil {
		return fail("chain-recertification-source-changed", "project-snapshot", err)
	}
	originalWhole, err := project.GraftProjectTree(a, certification.reviewed)
	if err != nil {
		return fail("chain-recertification-unproven", "source-graft", err)
	}
	mergedWhole, err := project.GraftProjectTree(t, m)
	if err != nil {
		return fail("chain-recertification-unproven", "merged-graft", err)
	}
	observedWhole, err := topSource.Snapshot("HEAD")
	if err != nil {
		return fail("chain-recertification-source-changed", "whole-snapshot", err)
	}
	admittedWhole := ""
	switch {
	case projectSnapshot == certification.reviewed && observedWhole == originalWhole:
		admittedWhole = originalWhole
	case projectSnapshot == m && observedWhole == mergedWhole:
		admittedWhole = mergedWhole
	default:
		return fail("chain-recertification-source-changed", "worktree-posture", fmt.Errorf("source project/whole snapshot is neither original R/A nor retry M/T"))
	}
	if err := activeOperation(topSource); err != nil {
		return fail("chain-recertification-source-changed", "active-operation", err)
	}
	indexBefore, err := topSource.TopStagedPosture()
	if err != nil {
		return fail("chain-recertification-source-changed", "index", err)
	}
	headWhole, err := topSource.TreeOf(h)
	if err != nil {
		return fail("chain-recertification-source-changed", "index", err)
	}
	if err := stagedEntriesAdmitted(topSource, headWhole, admittedWhole, indexBefore); err != nil {
		return fail("chain-recertification-source-changed", "index", err)
	}
	refsBefore, err := topSource.RefMap()
	if err != nil {
		return fail("chain-recertification-source-changed", "refs", err)
	}

	reviewRelative, err := repoRelative(r.root, certification.reviewPath)
	if err != nil {
		return fail("chain-recertification-unproven", "review-path", err)
	}
	patchRelative, err := repoRelative(r.root, certification.patchPath)
	if err != nil {
		return fail("chain-recertification-unproven", "patch-path", err)
	}
	criticRelative, err := repoRelative(r.root, certification.criticPath)
	if err != nil {
		return fail("chain-recertification-unproven", "critic-path", err)
	}
	for label, relative := range map[string]string{
		"review-path": reviewRelative, "patch-path": patchRelative, "critic-path": criticRelative,
	} {
		if _, err := artifactRegularNoFollow(r.root, relative); err != nil {
			return fail("chain-recertification-unproven", label, err)
		}
	}
	record := RecertificationRecord{
		SchemaVersion: 1, ProofKind: gittree.DisjointMergeProofKind, GitVersion: mergeResult.GitVersion,
		HunkManifest: mergeResult.Manifest, HunkManifestDigest: gittree.ManifestDigest(mergeResult.Manifest),
		RootChain: r.rootJob, CertifiedImplementerJob: r.job,
		ReviewArtifact: reviewRelative, ReviewDigest: sha256Bytes(certification.reviewBytes),
		PatchArtifact: patchRelative, PatchDigest: sha256Bytes(certification.patch),
		CriticRoot: certification.criticRoot, CriticTerminalRound: certification.criticRound,
		CriticReturnArtifact: criticRelative, CriticReturnDigest: sha256Bytes(certification.criticBytes),
		SourceHead: h, BaseCommit: a, BaseTree: b, ReviewedTree: certification.reviewed,
		TargetCommit: t, TargetTree: treeT, MergedTree: m,
		SourceWholeTree: originalWhole, MergedWholeTree: mergedWhole,
		CertifiedPaths: append([]string(nil), certifiedPaths...),
		GateWidth:      width, TestCommand: command,
		MergedPatchDigest: sha256Bytes(q),
	}
	record.InputDigest, err = RecertificationInputDigest(record)
	if err != nil {
		return fail("chain-recertification-unproven", "input-digest", err)
	}
	record.SourceAnchorRef = "refs/metasystem/landing/recertifications/" + record.InputDigest + "/source"
	record.MergedAnchorRef = "refs/metasystem/landing/recertifications/" + record.InputDigest + "/merged"
	recordRelative, err := recertificationRelative(r.root, r.rootJob, record.InputDigest)
	if err != nil {
		return fail("chain-recertification-unproven", "record-path", err)
	}
	record.DiffArtifact = filepath.ToSlash(filepath.Join(filepath.Dir(recordRelative), "diff.patch"))
	if err := topSource.AnchorRef(record.SourceAnchorRef, originalWhole, "tree"); err != nil {
		return fail("chain-recertification-unproven", "source-anchor", err)
	}
	if err := topSource.AnchorRef(record.MergedAnchorRef, mergedWhole, "tree"); err != nil {
		return fail("chain-recertification-unproven", "merged-anchor", err)
	}
	wholePaths, err := topSource.ChangedPaths(observedWhole, mergedWhole)
	if err != nil {
		return fail("chain-recertification-unproven", "materialization-paths", err)
	}
	if out, errs, code, expired := checkDeadline("materialization-preflight"); expired {
		return out, errs, code
	}
	if err := topSource.PreflightMaterialize(observedWhole, mergedWhole, wholePaths); err != nil {
		return fail("chain-recertification-source-changed", "materialization-preflight", err)
	}
	if err := topSource.MaterializePaths(mergedWhole, wholePaths); err != nil {
		return fail("chain-recertification-worktree-incomplete", "materialization", err)
	}
	postWhole, err := topSource.SnapshotSeeded(t, mergedWhole, nil)
	if err != nil || postWhole != mergedWhole {
		return fail("chain-recertification-worktree-incomplete", "post-snapshot", fmt.Errorf("materialized snapshot %s differs from %s: %w", postWhole, mergedWhole, err))
	}
	postProject, err := project.SnapshotSeeded(t, m, nil)
	if err != nil || postProject != m {
		return fail("chain-recertification-worktree-incomplete", "project-post-snapshot", err)
	}
	indexAfter, err := topSource.TopStagedPosture()
	if err != nil || !indexAfter.Equal(indexBefore) {
		return fail("chain-recertification-worktree-incomplete", "index-moved", err)
	}
	headAfter, unborn, err := topSource.HeadCommit()
	if err != nil || unborn || headAfter != h {
		return fail("chain-recertification-source-changed", "source-head-moved", err)
	}
	targetAfter, unborn, err := integration.HeadCommit()
	if err != nil || unborn || targetAfter != t {
		return fail("chain-recertification-target-moved", "target-head-moved", err)
	}
	refsAfter, err := topSource.RefMap()
	if err != nil || !reflect.DeepEqual(filterRecertificationRefs(refsBefore, record.InputDigest), filterRecertificationRefs(refsAfter, record.InputDigest)) {
		return fail("chain-recertification-source-changed", "refs-moved", err)
	}
	if _, digest, err := fileDigest(certification.reviewPath); err != nil || digest != record.ReviewDigest {
		return fail("chain-recertification-source-changed", "review-moved", err)
	}
	if _, digest, err := fileDigest(certification.patchPath); err != nil || digest != record.PatchDigest {
		return fail("chain-recertification-source-changed", "patch-moved", err)
	}
	if _, digest, err := fileDigest(certification.criticPath); err != nil || digest != record.CriticReturnDigest {
		return fail("chain-recertification-source-changed", "critic-return-moved", err)
	}
	if out, errs, code, expired := checkDeadline("publication"); expired {
		return out, errs, code
	}
	record.RecordDigest, err = RecertificationRecordDigest(record)
	if err != nil {
		return fail("chain-recertification-unproven", "record-digest", err)
	}
	path, err := publishRecertification(r, record, q, deadline)
	if err != nil {
		var typed *RecertificationFailure
		if errors.As(err, &typed) {
			return r.fail(typed.Error())
		}
		return fail("chain-recertification-unproven", "publication", err)
	}
	r.out = append(r.out, "recertification="+path, "certifiedTree="+m, "diffArtifact="+record.DiffArtifact)
	return r.out, r.errs, 0
}

// VerifyRecertification loads and independently replays the selected proof.
// It never trusts hashes as a substitute for the deterministic merge.
func VerifyRecertification(root, chain, suppliedPath string) (VerifiedRecertification, error) {
	fail := func(detail string, err error) (VerifiedRecertification, error) {
		return VerifiedRecertification{}, &RecertificationFailure{Reason: "chain-recertification-unproven", Detail: detail, Err: err}
	}
	if !conformanceJobID.MatchString(chain) {
		return fail("chain-id", fmt.Errorf("malformed chain"))
	}
	if filepath.IsAbs(suppliedPath) || filepath.ToSlash(filepath.Clean(suppliedPath)) != suppliedPath || strings.Contains(suppliedPath, "..") {
		return fail("record-path", fmt.Errorf("recertification path is not canonical repository-relative"))
	}
	absolute, err := artifactRegularNoFollow(root, suppliedPath)
	if err != nil {
		return fail("record-path", err)
	}
	data, err := os.ReadFile(absolute)
	if err != nil {
		return fail("record-read", err)
	}
	record, err := decodeStrictRecord(data)
	if err != nil || record.SchemaVersion != 1 || record.ProofKind != gittree.DisjointMergeProofKind || record.RootChain != chain || record.GitVersion == "" {
		return fail("record-schema", err)
	}
	expectedPath, err := recertificationRelative(root, chain, record.InputDigest)
	if err != nil || expectedPath != suppliedPath {
		return fail("record-path", fmt.Errorf("record path is %q, expected %q", suppliedPath, expectedPath))
	}
	wantInput, err := RecertificationInputDigest(record)
	if err != nil || wantInput != record.InputDigest {
		return fail("input-digest", err)
	}
	wantRecord, err := RecertificationRecordDigest(record)
	if err != nil || wantRecord != record.RecordDigest {
		return fail("record-digest", err)
	}
	if gittree.ManifestDigest(record.HunkManifest) != record.HunkManifestDigest {
		return fail("manifest-digest", fmt.Errorf("hunk manifest digest differs"))
	}
	if record.GateWidth != "area" && record.GateWidth != "full" {
		return fail("gate-width", fmt.Errorf("recorded gate width is invalid"))
	}
	if strings.TrimSpace(record.TestCommand) == "" || (record.GateWidth == "full" && record.TestCommand != FullBatteryCommand) {
		return fail("test-command", fmt.Errorf("recorded test command does not satisfy its gate width"))
	}
	if !sort.StringsAreSorted(record.CertifiedPaths) || len(record.CertifiedPaths) == 0 {
		return fail("certified-paths", fmt.Errorf("certified paths are empty or unsorted"))
	}
	for index, path := range record.CertifiedPaths {
		if path == "" || filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, "../") ||
			(index > 0 && record.CertifiedPaths[index-1] == path) {
			return fail("certified-paths", fmt.Errorf("certified path %q is unsafe or duplicated", path))
		}
	}
	wantSourceRef := "refs/metasystem/landing/recertifications/" + record.InputDigest + "/source"
	wantMergedRef := "refs/metasystem/landing/recertifications/" + record.InputDigest + "/merged"
	if record.SourceAnchorRef != wantSourceRef || record.MergedAnchorRef != wantMergedRef {
		return fail("anchor-ref", fmt.Errorf("recorded anchor refs are noncanonical"))
	}
	reviewPath, err := artifactRegularNoFollow(root, record.ReviewArtifact)
	if err != nil {
		return fail("review-path", err)
	}
	reviewBytes, reviewDigest, err := fileDigest(reviewPath)
	if err != nil || reviewDigest != record.ReviewDigest {
		return fail("review-digest", err)
	}
	var review struct {
		DiffArtifact   string `json:"diffArtifact"`
		ImplementerJob string `json:"implementerJob"`
		ReviewedTree   string `json:"reviewedTree"`
	}
	if json.Unmarshal(reviewBytes, &review) != nil || review.DiffArtifact != "diff.patch" ||
		review.ImplementerJob != record.CertifiedImplementerJob || review.ReviewedTree != record.ReviewedTree {
		return fail("review-binding", fmt.Errorf("review artifact does not bind the recorded implementer and tree"))
	}
	patchPath, err := artifactRegularNoFollow(root, record.PatchArtifact)
	if err != nil {
		return fail("patch-path", err)
	}
	patch, patchDigest, err := fileDigest(patchPath)
	if err != nil || patchDigest != record.PatchDigest {
		return fail("patch-digest", err)
	}
	criticPath, err := artifactRegularNoFollow(root, record.CriticReturnArtifact)
	if err != nil {
		return fail("critic-path", err)
	}
	criticBytes, criticDigest, err := fileDigest(criticPath)
	if err != nil || criticDigest != record.CriticReturnDigest {
		return fail("critic-digest", err)
	}
	var criticReturn map[string]any
	if json.Unmarshal(criticBytes, &criticReturn) != nil || criticReturn["reviewedTree"] != record.ReviewedTree {
		return fail("critic-binding", fmt.Errorf("critic return does not name reviewed tree"))
	}
	workspace := gittree.Workspace{Dir: root}
	sourceHead, err := workspace.ResolveCommit(record.SourceHead)
	if err != nil || sourceHead != record.SourceHead {
		return fail("source-head", err)
	}
	targetCommit, err := workspace.ResolveCommit(record.TargetCommit)
	if err != nil || targetCommit != record.TargetCommit {
		return fail("target-commit", err)
	}
	baseCommit, err := workspace.ResolveCommit(record.BaseCommit)
	if err != nil || baseCommit != record.BaseCommit {
		return fail("base-commit", err)
	}
	bases, err := workspace.MergeBases(record.SourceHead, record.TargetCommit)
	if err != nil || len(bases) != 1 || bases[0] != record.BaseCommit {
		return fail("merge-base", fmt.Errorf("historical merge base is not the sole recorded base"))
	}
	if tree, treeErr := workspace.TreeOf(record.BaseCommit); treeErr != nil || tree != record.BaseTree {
		return fail("base-tree", treeErr)
	}
	if tree, treeErr := workspace.TreeOf(record.TargetCommit); treeErr != nil || tree != record.TargetTree {
		return fail("target-tree", treeErr)
	}
	applied, err := workspace.Apply(record.BaseTree, patch)
	if err != nil || applied != record.ReviewedTree {
		return fail("base-patch-equation", err)
	}
	paths, err := workspace.ChangedPaths(record.BaseTree, record.ReviewedTree)
	if err != nil || len(paths) == 0 || !reflect.DeepEqual(paths, record.CertifiedPaths) {
		return fail("certified-paths", err)
	}
	mergeResult, err := workspace.DisjointMerge(record.BaseTree, record.ReviewedTree, record.TargetTree)
	if err != nil {
		var typed *gittree.DisjointMergeError
		detail := "merge-proof"
		if errors.As(err, &typed) && typed.Detail != "" {
			detail = typed.Detail
		}
		return fail(detail, err)
	}
	if mergeResult.MergedTree != record.MergedTree {
		return fail("merged-tree-mismatch", fmt.Errorf("recomputed merged tree %s differs from recorded %s", mergeResult.MergedTree, record.MergedTree))
	}
	if !reflect.DeepEqual(mergeResult.Manifest, record.HunkManifest) {
		return fail("hunk-manifest-mismatch", fmt.Errorf("canonical hunk manifest differs"))
	}
	diffPath, err := artifactRegularNoFollow(root, record.DiffArtifact)
	if err != nil || filepath.Dir(diffPath) != filepath.Dir(absolute) || filepath.Base(diffPath) != "diff.patch" {
		return fail("diff-path", fmt.Errorf("diff artifact is not beside record: %w", err))
	}
	q, qDigest, err := fileDigest(diffPath)
	if err != nil || qDigest != record.MergedPatchDigest {
		return fail("merged-patch-digest", err)
	}
	if replay, applyErr := workspace.Apply(record.TargetTree, q); applyErr != nil || replay != record.MergedTree {
		return fail("merged-patch-equation", applyErr)
	}
	if changed, changeErr := workspace.ChangedPaths(record.TargetTree, record.MergedTree); changeErr != nil || !reflect.DeepEqual(changed, record.CertifiedPaths) {
		return fail("merged-boundary", changeErr)
	}
	if oid, refErr := workspace.ResolveTree(record.SourceAnchorRef); refErr != nil || oid != record.SourceWholeTree {
		return fail("source-anchor", refErr)
	}
	if oid, refErr := workspace.ResolveTree(record.MergedAnchorRef); refErr != nil || oid != record.MergedWholeTree {
		return fail("merged-anchor", refErr)
	}
	project := workspace
	sourceOriginal, graftErr := project.GraftProjectTree(record.BaseCommit, record.ReviewedTree)
	if graftErr != nil {
		return fail("source-whole-tree", graftErr)
	}
	mergedWhole, graftErr := project.GraftProjectTree(record.TargetCommit, record.MergedTree)
	if graftErr != nil || mergedWhole != record.MergedWholeTree {
		return fail("merged-whole-tree", graftErr)
	}
	if record.SourceWholeTree != sourceOriginal {
		return fail("source-whole-tree", fmt.Errorf("source anchor does not name the original reviewed whole-repository snapshot"))
	}
	// Re-run the original critic's normal closure, independence, exhaustion,
	// and cumulative boundary checks against R rather than M.
	jobData, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "jobs", record.CertifiedImplementerJob+".json"))
	if err != nil {
		return fail("implementer-record", err)
	}
	var jobRecord map[string]any
	if json.Unmarshal(jobData, &jobRecord) != nil {
		return fail("implementer-record", fmt.Errorf("malformed implementer record"))
	}
	run := &conformanceRun{root: root, job: record.CertifiedImplementerJob, record: jobRecord,
		rootJob: record.RootChain, workspace: fmt.Sprint(jobRecord["workspaceRoot"]), criticRoot: record.CriticRoot}
	prefix, err := projectInstallPrefix(root)
	if err != nil {
		return fail("project-prefix", err)
	}
	run.installPrefix = prefix
	selected, err := run.selectOriginalCertification()
	if err != nil {
		return fail("review-selection", err)
	}
	selectedReview, reviewPathErr := repoRelative(root, selected.reviewPath)
	selectedPatch, patchPathErr := repoRelative(root, selected.patchPath)
	selectedCritic, criticPathErr := repoRelative(root, selected.criticPath)
	if reviewPathErr != nil || patchPathErr != nil || criticPathErr != nil ||
		selectedReview != record.ReviewArtifact || selectedPatch != record.PatchArtifact || selectedCritic != record.CriticReturnArtifact ||
		selected.reviewed != record.ReviewedTree || selected.criticRoot != record.CriticRoot || selected.criticRound != record.CriticTerminalRound ||
		!bytes.Equal(selected.reviewBytes, reviewBytes) || !bytes.Equal(selected.patch, patch) || !bytes.Equal(selected.criticBytes, criticBytes) {
		return fail("review-selection", fmt.Errorf("record does not name the exact currently selected original certification"))
	}
	if violations := run.cumulativeBoundaryViolations(record.CertifiedPaths); len(violations) > 0 {
		return fail("conformance-boundary", fmt.Errorf("%s", strings.Join(violations, "; ")))
	}
	configuredRuntime := run.configGet("role.code-critic.runtime", "__missing__")
	independence := run.configGet("independence", "")
	_, failures, code := run.mergeCritique(filepath.Join(root, "artifacts", "agents", "jobs", record.CertifiedImplementerJob+".json"),
		record.ReviewedTree, configuredRuntime, independence)
	if code != 0 {
		return fail("critic-closure", fmt.Errorf("%s", strings.Join(failures, "; ")))
	}
	return VerifiedRecertification{Record: record, RecordPath: suppliedPath, Patch: q}, nil
}
