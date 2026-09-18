package branch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

const (
	ReadUngatedCode      = "GOAL_READ_UNGATED"
	ReadTestsUnnamedCode = "GOAL_READ_TESTS_UNNAMED"
	ReadStaleCode        = "GOAL_READ_STALE"
	ReadInvalidCode      = "GOAL_READ_INVALID"
)

type AttestationSubject struct {
	Commit      string `json:"commit"`
	Parent      string `json:"parent"`
	Tree        string `json:"tree"`
	PatchDigest string `json:"patchDigest"`
	UnitDigest  string `json:"unitDigest"`
}

type AttestationSource struct {
	Kind          string `json:"kind"`
	RootJob       string `json:"rootJob,omitempty"`
	Round         int64  `json:"round,omitempty"`
	ClosureSHA256 string `json:"closureSha256,omitempty"`
	ReaderRecord  string `json:"readerRecord,omitempty"`
	RecordSHA256  string `json:"recordSha256,omitempty"`
}

type GateObservation struct {
	Kind  string `json:"kind"`
	Tree  string `json:"tree"`
	RunID string `json:"runId"`
}

type Fold struct {
	Kind   Kind   `json:"kind"`
	Commit string `json:"commit"`
	Digest string `json:"digest"`
}

type Carry struct {
	FromCommit string `json:"fromCommit"`
	FromTree   string `json:"fromTree"`
	ToCommit   string `json:"toCommit"`
	ToTree     string `json:"toTree"`
}

type TestChange struct {
	Path       string `json:"path"`
	ReaderWord string `json:"readerWord"`
}

type Attestation struct {
	SchemaVersion int                `json:"schemaVersion"`
	Goal          string             `json:"goal"`
	Unit          string             `json:"unit"`
	Subject       AttestationSubject `json:"subject"`
	Source        AttestationSource  `json:"source"`
	Verdict       string             `json:"verdict"`
	Gate          GateObservation    `json:"gate"`
	Folds         []Fold             `json:"folds"`
	Carry         *Carry             `json:"carry,omitempty"`
	TestsChanged  []TestChange       `json:"testsChanged"`
	SHA256        string             `json:"sha256"`
}

type closureBundle struct {
	SchemaVersion int               `json:"schemaVersion"`
	RootJob       string            `json:"rootJob"`
	Files         map[string]string `json:"files"`
}

type CommitReadRequest struct {
	Repo, Remote, EndpointTip, GoalID, Unit, OpID string
	Units                                         []string
	CheckClaim                                    func() error
	RootJob, ReaderRecord, Carry                  string
	GateRunID, GateTree                           string
	TestsChanged                                  []TestChange
	Transport                                     PushTransport
}

type LandedUnit struct {
	Goal, Unit, Digest, CriticRoot, GateRunID string
	Round                                     int64
	GoalRevision                              uint64
	FoldPaths, ChangedPaths                   []string
	HasPlan, Destructive                      bool
}

type LandedUnitError struct {
	Code string
	Err  error
}

func (e *LandedUnitError) Error() string                  { return e.Err.Error() }
func (e *LandedUnitError) Unwrap() error                  { return e.Err }
func (e *LandedUnitError) LandingAttestationCode() string { return e.Code }

func attestationPath(goalID, commit string) string {
	return "metasystem/records/reads/" + goalID + "/" + commit + ".json"
}

func closureBundlePath(goalID, commit string) string {
	return "metasystem/records/reads/" + goalID + "/" + commit + ".closure.json"
}

func computeSubject(repo, commit string) (AttestationSubject, readsubject.ReadSubject, error) {
	read, present, err := dispatch.ComputeReadSubject(dispatch.ReadSubjectRequest{
		RepoRoot: repo, Role: "code-critic", Reviews: "commit:" + commit,
	})
	if err != nil || !present {
		return AttestationSubject{}, readsubject.ReadSubject{}, fmt.Errorf("commit subject %s is unreadable: %v", commit, err)
	}
	digest, err := UnitDigest(repo, commit)
	if err != nil {
		return AttestationSubject{}, readsubject.ReadSubject{}, err
	}
	return AttestationSubject{
		Commit: read.Commit, Parent: read.Parent, Tree: read.Tree,
		PatchDigest: read.DiffDigest, UnitDigest: digest,
	}, read, nil
}

func foldRange(repo, endpointTip, unitCommit, goalID string) ([]Fold, error) {
	commits, err := ValidateRange(repo, endpointTip, unitCommit, goalID)
	if err != nil {
		return nil, err
	}
	for i := len(commits) - 1; i >= 0; i-- {
		if commits[i].ID != unitCommit {
			continue
		}
		var folds []Fold
		for _, commit := range commits[:i] {
			if commit.Kind == Unit {
				continue
			}
			digest, err := UnitDigest(repo, commit.ID)
			if err != nil {
				return nil, err
			}
			folds = append(folds, Fold{Kind: commit.Kind, Commit: commit.ID, Digest: digest})
		}
		return folds, nil
	}
	return nil, operationRefusal(ReadInvalidCode, "unit commit %s is outside the goal range", unitCommit)
}

func digestAttestation(att Attestation) (string, error) {
	att.SHA256 = ""
	data, err := json.Marshal(att)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func isExistingTestPath(path, sourceBlob string) bool {
	if strings.Trim(sourceBlob, "0") == "" {
		return false
	}
	clean := filepath.ToSlash(path)
	base := filepath.Base(clean)
	return strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, "_test.py") ||
		strings.HasSuffix(base, ".bats") || strings.HasSuffix(base, "-fixtures.sh") ||
		strings.Contains(clean, "/testdata/") || strings.HasPrefix(clean, "tests/") || strings.Contains(clean, "/tests/")
}

func requiredTestChanges(repo, commit string) ([]string, error) {
	entries, err := RawEntries(repo, commit)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if isExistingTestPath(entry.Path, entry.SrcBlob) {
			paths = append(paths, entry.Path)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func validateTestChanges(repo, commit string, supplied []TestChange) error {
	required, err := requiredTestChanges(repo, commit)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var got []string
	for _, item := range supplied {
		if strings.TrimSpace(item.ReaderWord) == "" || seen[item.Path] {
			return operationRefusal(ReadTestsUnnamedCode, "every changed test needs one nonempty reader word")
		}
		seen[item.Path] = true
		got = append(got, item.Path)
	}
	sort.Strings(got)
	if strings.Join(required, "\x00") != strings.Join(got, "\x00") {
		return operationRefusal(ReadTestsUnnamedCode, "changed tests are %v, but the attestation names %v", required, got)
	}
	return nil
}

func safeReaderRecord(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return clean == path && strings.HasPrefix(clean, "metasystem/records/misc/") && !strings.Contains(clean, "../")
}

func fileSHA256(path string) (string, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), data, nil
}

func fileAt(repo, snapshot, path string) ([]byte, error) {
	if snapshot == "" {
		return os.ReadFile(filepath.Join(repo, filepath.FromSlash(path)))
	}
	data, err := gitOutput(repo, "show", snapshot+":"+path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func fileSHA256At(repo, snapshot, path string) (string, []byte, error) {
	data, err := fileAt(repo, snapshot, path)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), data, nil
}

func directSource(req CommitReadRequest, subject AttestationSubject, read readsubject.ReadSubject) (AttestationSource, []byte, error) {
	if (req.RootJob == "") == (req.ReaderRecord == "") {
		return AttestationSource{}, nil, fmt.Errorf("read needs exactly one of --root-job or --reader-record")
	}
	if req.RootJob != "" {
		closure, files, err := dispatch.CommitCriticClosureFiles(filepath.Join(req.Repo, "artifacts", "agents"), req.RootJob, read)
		if err != nil {
			return AttestationSource{}, nil, err
		}
		bundle := closureBundle{SchemaVersion: 1, RootJob: req.RootJob, Files: make(map[string]string, len(files))}
		for path, data := range files {
			bundle.Files[path] = string(data)
		}
		data, err := json.MarshalIndent(bundle, "", "  ")
		if err != nil {
			return AttestationSource{}, nil, err
		}
		data = append(data, '\n')
		sum := sha256.Sum256(data)
		return AttestationSource{Kind: "critic-root", RootJob: req.RootJob, Round: closure.Round,
			ClosureSHA256: hex.EncodeToString(sum[:])}, data, nil
	}
	if !safeReaderRecord(req.ReaderRecord) {
		return AttestationSource{}, nil, fmt.Errorf("reader record must be a repository-relative path under metasystem/records/misc")
	}
	digest, data, err := fileSHA256(filepath.Join(req.Repo, filepath.FromSlash(req.ReaderRecord)))
	if err != nil {
		return AttestationSource{}, nil, err
	}
	text := string(data)
	if !strings.Contains(text, subject.Commit) || !strings.Contains(text, subject.UnitDigest) {
		return AttestationSource{}, nil, fmt.Errorf("reader record must name commit %s and unit digest %s", subject.Commit, subject.UnitDigest)
	}
	return AttestationSource{Kind: "reader-record", ReaderRecord: req.ReaderRecord, RecordSHA256: digest}, nil, nil
}

func sameFoldDigests(a, b []Fold) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || a[i].Digest != b[i].Digest {
			return false
		}
	}
	return true
}

func readAttestationAt(repo, snapshot, goalID, commit string) (Attestation, error) {
	rel := attestationPath(goalID, commit)
	data, err := fileAt(repo, snapshot, rel)
	if err != nil {
		return Attestation{}, err
	}
	var att Attestation
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&att); err != nil {
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation %s is malformed: %v", rel, err)
	}
	canonical, err := json.MarshalIndent(att, "", "  ")
	if err != nil || !bytes.Equal(data, append(canonical, '\n')) {
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation %s is not in its canonical form", rel)
	}
	return att, nil
}

func safeClosureBundlePath(path string) bool {
	windowsAbsolute := len(path) >= 3 && ((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z')) && path[1] == ':' && path[2] == '/'
	if path == "" || filepath.IsAbs(path) || filepath.VolumeName(path) != "" || windowsAbsolute || strings.Contains(path, `\`) {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	return clean == path && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

func withClosureBundle(repo, snapshot, goalID, commit string, source AttestationSource, use func(string) error) error {
	path := closureBundlePath(goalID, commit)
	data, err := fileAt(repo, snapshot, path)
	if err != nil {
		return operationRefusal(ReadInvalidCode, "critic closure bundle for %s is unreadable", commit)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != source.ClosureSHA256 {
		return operationRefusal(ReadInvalidCode, "critic closure bundle for %s fails its digest", commit)
	}
	var bundle closureBundle
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil || bundle.SchemaVersion != 1 || bundle.RootJob != source.RootJob {
		return operationRefusal(ReadInvalidCode, "critic closure bundle for %s is malformed", commit)
	}
	canonical, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil || !bytes.Equal(data, append(canonical, '\n')) {
		return operationRefusal(ReadInvalidCode, "critic closure bundle for %s is not in its canonical form", commit)
	}
	temporary, err := os.MkdirTemp("", "goal-read-closure-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	for path, content := range bundle.Files {
		if !safeClosureBundlePath(path) {
			return operationRefusal(ReadInvalidCode, "critic closure bundle for %s has unsafe path %q", commit, path)
		}
		target := filepath.Join(temporary, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return use(temporary)
}

func validateCriticSource(repo, snapshot, goalID, commit string, source AttestationSource, read readsubject.ReadSubject) (readsubject.Closure, error) {
	var closure readsubject.Closure
	var err error
	if source.ClosureSHA256 == "" {
		return dispatch.ValidateCommitCriticClosure(repo, source.RootJob, read)
	}
	err = withClosureBundle(repo, snapshot, goalID, commit, source, func(agentsRoot string) error {
		closure, err = dispatch.ValidateCommitCriticClosureAt(agentsRoot, source.RootJob, read)
		return err
	})
	return closure, err
}

func validateAttestation(repo, snapshot, endpointTip, goalID, unit, commit string, seen map[string]bool) (Attestation, error) {
	if seen[commit] {
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation carry cycle at %s", commit)
	}
	seen[commit] = true
	att, err := readAttestationAt(repo, snapshot, goalID, commit)
	if err != nil {
		return Attestation{}, err
	}
	if att.SchemaVersion != 1 || att.Goal != goalID || att.Unit != unit || att.Verdict != "LAND" || att.Subject.Commit != commit {
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation identity or verdict does not match %s/%s at %s", goalID, unit, commit)
	}
	digest, err := digestAttestation(att)
	if err != nil || digest != att.SHA256 {
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation %s fails its self-digest", commit)
	}
	subject, read, err := computeSubject(repo, commit)
	if err != nil || subject != att.Subject {
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation %s does not match its commit subject", commit)
	}
	folds, err := foldRange(repo, endpointTip, commit, goalID)
	if err != nil || !sameFoldDigests(folds, att.Folds) {
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation %s does not match its fold range", commit)
	}
	if att.Gate.Kind != "go-gate-fast" || att.Gate.RunID == "" || att.Gate.Tree != subject.Tree {
		return Attestation{}, operationRefusal(ReadUngatedCode, "attestation %s has no fast-gate observation on tree %s", commit, subject.Tree)
	}
	if err := validateTestChanges(repo, commit, att.TestsChanged); err != nil {
		return Attestation{}, err
	}
	if att.Carry != nil {
		prior, err := validateAttestation(repo, snapshot, endpointTip, goalID, unit, att.Carry.FromCommit, seen)
		if err != nil {
			return Attestation{}, err
		}
		if att.Carry.FromTree != prior.Subject.Tree || att.Carry.ToCommit != commit || att.Carry.ToTree != subject.Tree ||
			prior.Subject.UnitDigest != subject.UnitDigest || !sameFoldDigests(prior.Folds, folds) || prior.Source != att.Source {
			return Attestation{}, operationRefusal(ReadStaleCode, "carry from %s does not preserve the unit and fold bytes", att.Carry.FromCommit)
		}
		return att, nil
	}
	switch att.Source.Kind {
	case "critic-root":
		closure, err := validateCriticSource(repo, snapshot, goalID, commit, att.Source, read)
		if err != nil || closure.Round != att.Source.Round {
			return Attestation{}, operationRefusal(ReadInvalidCode, "critic source for %s is not a clean bound closure: %v", commit, err)
		}
	case "reader-record":
		if att.Source.ClosureSHA256 != "" || !safeReaderRecord(att.Source.ReaderRecord) {
			return Attestation{}, operationRefusal(ReadInvalidCode, "reader record path is outside records/misc")
		}
		digest, data, err := fileSHA256At(repo, snapshot, att.Source.ReaderRecord)
		if err != nil || digest != att.Source.RecordSHA256 || !strings.Contains(string(data), commit) || !strings.Contains(string(data), subject.UnitDigest) {
			return Attestation{}, operationRefusal(ReadInvalidCode, "reader record for %s is missing or changed", commit)
		}
	default:
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation %s has unknown source %q", commit, att.Source.Kind)
	}
	return att, nil
}

func ValidateAttestation(repo, endpointTip, goalID, unit, commit string) (Attestation, error) {
	return validateAttestation(repo, "", endpointTip, goalID, unit, commit, map[string]bool{})
}

// ValidateAttestationAt validates the evidence as it exists in one fetched
// branch snapshot while keeping local critic artifacts available at repo.
func ValidateAttestationAt(repo, snapshot, endpointTip, goalID, unit, commit string) (Attestation, error) {
	return validateAttestation(repo, snapshot, endpointTip, goalID, unit, commit, map[string]bool{})
}

func rawTransition(repo, before, after string) ([]byte, error) {
	return gitOutput(repo, "diff-tree", "-r", "-z", "--no-renames", "--full-index", before, after)
}

func filteredTransitionDigest(raw []byte, prefix string, excluded map[string]bool) (string, []string, bool, error) {
	parts := bytes.Split(raw, []byte{0})
	var kept bytes.Buffer
	var paths []string
	destructive := false
	for index := 0; index+1 < len(parts) && len(parts[index]) != 0; index += 2 {
		fields := strings.Fields(string(parts[index]))
		if len(fields) != 5 || !strings.HasPrefix(fields[0], ":") {
			return "", nil, false, fmt.Errorf("candidate transition is malformed")
		}
		path := string(parts[index+1])
		if excluded[path] {
			continue
		}
		paths = append(paths, path)
		kept.Write(parts[index])
		kept.WriteByte(0)
		kept.WriteString(prefix + path)
		kept.WriteByte(0)
		if fields[4] == "D" {
			destructive = true
		}
	}
	sum := sha256.Sum256(kept.Bytes())
	return hex.EncodeToString(sum[:]), paths, destructive, nil
}

func treeEntryAt(repo, tree, path string) (string, error) {
	out, err := gitOutput(repo, "ls-tree", "-z", tree, "--", path)
	if err != nil {
		return "", err
	}
	meta, _, _ := strings.Cut(strings.TrimSuffix(string(out), "\x00"), "\t")
	return meta, nil
}

func rootGoalRevisionAt(agentsRoot, job, goalID string) (uint64, error) {
	record, err := dispatch.ReadRecordObject(filepath.Join(agentsRoot, "jobs", job+".json"))
	if err != nil {
		return 0, err
	}
	lens := dispatch.JobRecordOf(record)
	revision, present := lens.GoalRevision()
	if !present || revision == 0 || lens.GoalID() != goalID {
		return 0, fmt.Errorf("critic root %s is not bound to goal %s at a revision", job, goalID)
	}
	return revision, nil
}

func attestedGoalRevision(repo, snapshot, goalID, commit string, source AttestationSource) (uint64, error) {
	if source.ClosureSHA256 == "" {
		return rootGoalRevisionAt(filepath.Join(repo, "artifacts", "agents"), source.RootJob, goalID)
	}
	var revision uint64
	err := withClosureBundle(repo, snapshot, goalID, commit, source, func(agentsRoot string) error {
		var err error
		revision, err = rootGoalRevisionAt(agentsRoot, source.RootJob, goalID)
		return err
	})
	return revision, err
}

// BindLandedUnit validates the persisted evidence and binds one prospective
// trunk transition to the exact branch contribution it certifies.
func BindLandedUnit(repo, snapshot, endpointTip, goalID, commit, beforeTree, afterTree string) (LandedUnit, error) {
	if _, err := gitOutput(repo, "cat-file", "-e", commit+"^{commit}"); err != nil {
		return LandedUnit{}, &LandedUnitError{Code: "unreadable", Err: err}
	}
	info, err := KindOf(repo, commit, goalID)
	if err != nil || info.Kind != Unit {
		if err == nil {
			err = fmt.Errorf("commit %s is not a Goal-Unit commit", commit)
		}
		return LandedUnit{}, &LandedUnitError{Code: "invalid", Err: err}
	}
	att, err := readAttestationAt(repo, snapshot, goalID, commit)
	if err != nil {
		return LandedUnit{}, &LandedUnitError{Code: "unreadable", Err: err}
	}
	if att.Goal != goalID {
		return LandedUnit{}, &LandedUnitError{Code: "goal-mismatch", Err: fmt.Errorf("attestation names goal %s", att.Goal)}
	}
	att, err = ValidateAttestationAt(repo, snapshot, endpointTip, goalID, info.Unit, commit)
	if err != nil {
		return LandedUnit{}, &LandedUnitError{Code: "invalid", Err: err}
	}
	result := LandedUnit{Goal: att.Goal, Unit: att.Unit, Digest: att.Subject.UnitDigest,
		CriticRoot: att.Source.RootJob, Round: att.Source.Round, GateRunID: att.Gate.RunID}
	if att.Source.Kind != "critic-root" {
		return result, nil
	}
	result.GoalRevision, err = attestedGoalRevision(repo, snapshot, goalID, commit, att.Source)
	if err != nil {
		return LandedUnit{}, &LandedUnitError{Code: "invalid", Err: err}
	}
	prefixBytes, err := gitOutput(repo, "rev-parse", "--show-prefix")
	if err != nil {
		return LandedUnit{}, &LandedUnitError{Code: "change-mismatch", Err: err}
	}
	prefix := strings.TrimSpace(string(prefixBytes))
	excluded := map[string]bool{
		"memory/receipts.log": true,
	}
	for _, readPrefix := range []string{"records/reads/" + goalID + "/"} {
		raw, rawErr := rawTransition(repo, beforeTree, afterTree)
		if rawErr != nil {
			return LandedUnit{}, &LandedUnitError{Code: "change-mismatch", Err: rawErr}
		}
		parts := bytes.Split(raw, []byte{0})
		for index := 1; index < len(parts); index += 2 {
			if strings.HasPrefix(string(parts[index]), readPrefix) {
				excluded[string(parts[index])] = true
			}
		}
	}
	lastFold := map[string]string{}
	for _, fold := range att.Folds {
		result.HasPlan = result.HasPlan || fold.Kind == Plan
		entries, entriesErr := RawEntries(repo, fold.Commit)
		if entriesErr != nil {
			return LandedUnit{}, &LandedUnitError{Code: "invalid", Err: entriesErr}
		}
		for _, entry := range entries {
			path := strings.TrimPrefix(entry.Path, prefix)
			if prefix != "" && path == entry.Path {
				return LandedUnit{}, &LandedUnitError{Code: "change-mismatch", Err: fmt.Errorf("folded path %s is outside the landing workspace", entry.Path)}
			}
			excluded[path] = true
			lastFold[path] = fold.Commit
		}
	}
	for path, foldCommit := range lastFold {
		_, beforeErr := treeEntryAt(repo, beforeTree, path)
		after, afterErr := treeEntryAt(repo, afterTree, path)
		want, wantErr := treeEntryAt(repo, foldCommit, prefix+path)
		if beforeErr != nil || afterErr != nil || wantErr != nil || after != want {
			return LandedUnit{}, &LandedUnitError{Code: "change-mismatch", Err: fmt.Errorf("folded path %s does not match %s", path, foldCommit)}
		}
		result.FoldPaths = append(result.FoldPaths, path)
	}
	raw, err := rawTransition(repo, beforeTree, afterTree)
	if err != nil {
		return LandedUnit{}, &LandedUnitError{Code: "change-mismatch", Err: err}
	}
	digest, paths, destructive, err := filteredTransitionDigest(raw, prefix, excluded)
	if err != nil || digest != att.Subject.UnitDigest {
		if err == nil {
			err = fmt.Errorf("candidate digest %s does not match attested digest %s", digest, att.Subject.UnitDigest)
		}
		return LandedUnit{}, &LandedUnitError{Code: "change-mismatch", Err: err}
	}
	result.ChangedPaths, result.Destructive = paths, destructive
	sort.Strings(result.FoldPaths)
	return result, nil
}

func unitCommitInRange(repo, endpointTip, tip, goalID string, units []string) (string, error) {
	commits, err := ValidateRange(repo, endpointTip, tip, goalID)
	if err != nil {
		return "", err
	}
	for i := len(commits) - 1; i >= 0; i-- {
		if commits[i].Kind == Unit && sameUnits(commits[i].Units, units) {
			return commits[i].ID, nil
		}
	}
	return "", operationRefusal(ReadInvalidCode, "goal branch has no build %s", unitList(units))
}

func prospectiveReadPatch(repo string, generated map[string][]byte, readerRecord string) ([]byte, error) {
	scratch, err := os.MkdirTemp("", "goal-read-index-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch)
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(scratch, "index")}
	tree, err := gitOutput(repo, "write-tree")
	if err != nil {
		return nil, err
	}
	if _, err := gitInputEnv(repo, env, nil, "read-tree", strings.TrimSpace(string(tree))); err != nil {
		return nil, err
	}
	for path, data := range generated {
		blob, err := gitInput(repo, data, "hash-object", "-w", "--path="+path, "--stdin")
		if err != nil {
			return nil, err
		}
		entry := "100644," + strings.TrimSpace(string(blob)) + "," + path
		if _, err := gitInputEnv(repo, env, nil, "update-index", "--add", "--cacheinfo", entry); err != nil {
			return nil, err
		}
	}
	if readerRecord != "" {
		if _, err := gitInputEnv(repo, env, nil, "add", "--", readerRecord); err != nil {
			return nil, err
		}
	}
	return gitInputEnv(repo, env, nil, "diff", "--cached", "--binary", "--full-index")
}

func CommitRead(req CommitReadRequest) (string, Attestation, error) {
	unitRequest := CommitRequest{Unit: req.Unit, Units: req.Units}
	units, err := requestUnits(unitRequest)
	if err != nil || len(units) == 0 {
		return "", Attestation{}, fmt.Errorf("read commits need one or more distinct unit names")
	}
	list := unitList(units)
	commitReq := CommitRequest{
		Repo: req.Repo, Remote: req.Remote, EndpointTip: req.EndpointTip, GoalID: req.GoalID,
		Units: units, OpID: req.OpID, Kind: Read, CheckClaim: req.CheckClaim, Transport: req.Transport,
	}
	if err := CheckCommitAccess(req.GoalID, req.CheckClaim); err != nil {
		return "", Attestation{}, err
	}
	state, err := inspectCommitBranch(commitReq)
	if err != nil {
		return "", Attestation{}, err
	}
	if req.GateRunID == "" || req.GateTree == "" {
		return "", Attestation{}, operationRefusal(ReadUngatedCode, "a fast-gate run id and tree are required")
	}
	unitCommit, err := unitCommitInRange(req.Repo, req.EndpointTip, state.baseTip, req.GoalID, units)
	if err != nil {
		return "", Attestation{}, err
	}
	subject, read, err := computeSubject(req.Repo, unitCommit)
	if err != nil {
		return "", Attestation{}, err
	}
	if req.GateTree != subject.Tree {
		return "", Attestation{}, operationRefusal(ReadUngatedCode, "fast gate observed tree %s, not unit tree %s", req.GateTree, subject.Tree)
	}
	if err := validateTestChanges(req.Repo, unitCommit, req.TestsChanged); err != nil {
		return "", Attestation{}, err
	}
	folds, err := foldRange(req.Repo, req.EndpointTip, unitCommit, req.GoalID)
	if err != nil {
		return "", Attestation{}, err
	}
	att := Attestation{
		SchemaVersion: 1, Goal: req.GoalID, Unit: list, Subject: subject,
		Verdict: "LAND", Gate: GateObservation{Kind: "go-gate-fast", Tree: req.GateTree, RunID: req.GateRunID},
		Folds: folds, TestsChanged: append([]TestChange(nil), req.TestsChanged...),
	}
	var bundleData []byte
	sort.Slice(att.TestsChanged, func(i, j int) bool { return att.TestsChanged[i].Path < att.TestsChanged[j].Path })
	if req.Carry != "" {
		prior, err := ValidateAttestation(req.Repo, req.EndpointTip, req.GoalID, list, req.Carry)
		if err != nil {
			return "", Attestation{}, err
		}
		if prior.Subject.UnitDigest != subject.UnitDigest || !sameFoldDigests(prior.Folds, folds) {
			return "", Attestation{}, operationRefusal(ReadStaleCode, "carry from %s does not preserve the unit and fold bytes", req.Carry)
		}
		att.Source = prior.Source
		if prior.Source.ClosureSHA256 != "" {
			bundleData, err = fileAt(req.Repo, "", closureBundlePath(req.GoalID, req.Carry))
			if err != nil {
				return "", Attestation{}, err
			}
		}
		att.Carry = &Carry{FromCommit: req.Carry, FromTree: prior.Subject.Tree, ToCommit: unitCommit, ToTree: subject.Tree}
	} else {
		att.Source, bundleData, err = directSource(req, subject, read)
		if err != nil {
			return "", Attestation{}, err
		}
	}
	att.SHA256, err = digestAttestation(att)
	if err != nil {
		return "", Attestation{}, err
	}
	data, err := json.MarshalIndent(att, "", "  ")
	if err != nil {
		return "", Attestation{}, err
	}
	data = append(data, '\n')
	rel := attestationPath(req.GoalID, unitCommit)
	bundleRel := ""
	generated := map[string][]byte{rel: data}
	if len(bundleData) != 0 {
		bundleRel = closureBundlePath(req.GoalID, unitCommit)
		generated[bundleRel] = bundleData
	}
	paths, err := stagedPaths(req.Repo)
	if err != nil {
		return "", Attestation{}, err
	}
	paths = append(paths, rel)
	if bundleRel != "" {
		paths = append(paths, bundleRel)
	}
	if att.Source.Kind == "reader-record" && att.Carry == nil {
		paths = append(paths, att.Source.ReaderRecord)
	}
	if err := validateCommitPaths(Read, paths, req.GoalID); err != nil {
		return "", Attestation{}, err
	}
	readerRecord := ""
	if att.Source.Kind == "reader-record" && att.Carry == nil {
		readerRecord = att.Source.ReaderRecord
	}
	if err := adoptionCheckoutClean(commitReq, state, []string{rel, bundleRel, readerRecord}); err != nil {
		return "", Attestation{}, err
	}
	patch, err := prospectiveReadPatch(req.Repo, generated, readerRecord)
	if err != nil {
		return "", Attestation{}, err
	}
	subjectLine, trailer, err := commitMessage(commitReq, unitCommit)
	if err != nil {
		return "", Attestation{}, err
	}
	preparedTip, err := buildCommitOnto(commitReq, state, subjectLine, trailer, patch)
	if err != nil {
		return "", Attestation{}, err
	}
	for path, contents := range generated {
		abs := filepath.Join(req.Repo, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return "", Attestation{}, err
		}
		if err := os.WriteFile(abs, contents, 0o644); err != nil {
			return "", Attestation{}, err
		}
	}
	paths = []string{rel}
	if bundleRel != "" {
		paths = append(paths, bundleRel)
	}
	if att.Source.Kind == "reader-record" && att.Carry == nil {
		paths = append(paths, att.Source.ReaderRecord)
	}
	args := append([]string{"add", "--"}, paths...)
	if _, err := gitOutput(req.Repo, args...); err != nil {
		return "", Attestation{}, err
	}
	if err := installCommitOnto(commitReq, state, preparedTip); err != nil {
		return "", Attestation{}, err
	}
	return preparedTip, att, nil
}
