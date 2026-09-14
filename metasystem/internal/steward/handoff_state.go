package steward

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/output"
)

const HandoffSchemaVersion = 1

const HandoffDisposable = "what the harness holds only in memory is disposable; nothing in memory is needed to continue"

const (
	maxHandoffOpenJobs = 50
	maxHandoffScratch  = 20
	maxHandoffMessages = 50
	maxHandoffLandings = 5
)

var (
	handoffNoncePattern  = regexp.MustCompile(`^[0-9a-f]{16}$`)
	handoffDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	handoffReceiptSource = regexp.MustCompile(`^memory/receipts\.log:[1-9][0-9]*$`)
)

// HandoffReference is a verified immutable copy plus the live source it
// represents. Optional absent scratch is explicit and carries no readable
// copy, so absence cannot be confused with failed verification.
type HandoffReference struct {
	dispatch.CompositionReference
	Required   bool   `json:"required"`
	SourcePath string `json:"sourcePath,omitempty"`
	Status     string `json:"status,omitempty"`
}

type HandoffSeat struct {
	Machine           string       `json:"machine"`
	Runtime           string       `json:"runtime"`
	Session           string       `json:"session"`
	NormalizedSession string       `json:"normalizedSession"`
	MainID            string       `json:"mainId,omitempty"`
	Identity          identity.Ref `json:"identity"`
	Tag               string       `json:"tag,omitempty"`
	JobID             string       `json:"jobId,omitempty"`
}

type HandoffClaimant struct {
	Machine  string `json:"machine"`
	Lineage  string `json:"lineage"`
	At       string `json:"at"`
	Revision uint64 `json:"revision"`
}

type HandoffHeldGoal struct {
	ID               string          `json:"id"`
	State            string          `json:"state"`
	AcceptedRevision uint64          `json:"acceptedRevision"`
	Claimant         HandoffClaimant `json:"claimant"`
}

type HandoffNextStep struct {
	Text       string             `json:"text"`
	References []HandoffReference `json:"references,omitempty"`
}

type HandoffOpenJob struct {
	ID     string           `json:"id"`
	Role   string           `json:"role"`
	Status string           `json:"status"`
	Phase  string           `json:"phase"`
	Record HandoffReference `json:"record"`
}

type HandoffLanding struct {
	At    string `json:"at"`
	OpID  string `json:"opid"`
	Verb  string `json:"verb"`
	Actor string `json:"actor"`
}

type HandoffReceipt struct {
	Receipt string `json:"receipt"`
	Type    string `json:"type"`
	Outcome string `json:"outcome"`
	Note    string `json:"note"`
}

type HandoffLastLandings struct {
	History  []HandoffLanding `json:"history,omitempty"`
	Receipts []HandoffReceipt `json:"receipts,omitempty"`
}

type HandoffMessage struct {
	Nonce          string `json:"nonce"`
	Message        string `json:"message"`
	DeliveryStatus string `json:"deliveryStatus"`
}

// HandoffManifest holds list overflow without weakening the state file's
// byte bound. Its digest and exact owned path are pinned by HandoffState.
type HandoffManifest struct {
	SchemaVersion int                 `json:"schemaVersion"`
	OpenJobs      []HandoffOpenJob    `json:"openJobs,omitempty"`
	LastLandings  HandoffLastLandings `json:"lastLandings"`
	Scratch       []HandoffReference  `json:"scratch,omitempty"`
	MessagesOwed  []HandoffMessage    `json:"messagesOwed,omitempty"`
}

// HandoffState is the bounded orientation snapshot. The live goal ledger and
// job records remain authoritative; references preserve the bytes captured at
// handoff time so later lawful progress cannot rewrite this record.
type HandoffState struct {
	SchemaVersion int                            `json:"schemaVersion"`
	WrittenAt     time.Time                      `json:"writtenAt"`
	Seat          HandoffSeat                    `json:"seat"`
	HeldGoal      HandoffHeldGoal                `json:"heldGoal"`
	NextStep      HandoffNextStep                `json:"nextStep"`
	OpenJobs      []HandoffOpenJob               `json:"openJobs,omitempty"`
	LastLandings  HandoffLastLandings            `json:"lastLandings"`
	Scratch       []HandoffReference             `json:"scratch,omitempty"`
	MessagesOwed  []HandoffMessage               `json:"messagesOwed,omitempty"`
	Manifest      *dispatch.CompositionReference `json:"manifest,omitempty"`
	Disposable    string                         `json:"disposable"`
}

func decodeStrictHandoffJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func canonicalExistingPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func relativePathInside(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func validateSourcePath(path string) error {
	if path == "" {
		return nil
	}
	converted := filepath.FromSlash(path)
	if filepath.IsAbs(converted) || filepath.Clean(converted) == "." || filepath.Clean(converted) != converted ||
		converted == ".." || strings.HasPrefix(converted, ".."+string(filepath.Separator)) {
		return fmt.Errorf("source path %q is not a clean state-root-relative path", path)
	}
	return nil
}

func emptyCompositionReferenceExceptPurpose(reference dispatch.CompositionReference) bool {
	return reference.Slot == "" && reference.Path == "" && reference.OpenPath == "" && reference.Digest == "" &&
		reference.Bytes == 0 && reference.Lifetime == ""
}

func validateHandoffReference(root, ownedDir string, reference HandoffReference) error {
	if err := validateSourcePath(reference.SourcePath); err != nil {
		return err
	}
	if reference.Status == "missing" {
		if reference.Required {
			return fmt.Errorf("required reference %q cannot be missing", reference.SourcePath)
		}
		if reference.SourcePath == "" || reference.Purpose == "" || !emptyCompositionReferenceExceptPurpose(reference.CompositionReference) {
			return fmt.Errorf("optional missing reference must name only its purpose and source path")
		}
		return nil
	}
	if reference.Status != "" {
		return fmt.Errorf("reference %q has invalid status %q", reference.SourcePath, reference.Status)
	}
	if reference.SourcePath == "" {
		return fmt.Errorf("verified reference must name its live source")
	}
	if reference.Slot == "" || reference.Purpose == "" || reference.Path == "" || reference.OpenPath == "" ||
		!handoffDigestPattern.MatchString(reference.Digest) || reference.Bytes < 0 || reference.Lifetime != "immutable" {
		return fmt.Errorf("reference %q is incomplete", reference.SourcePath)
	}
	if filepath.IsAbs(filepath.FromSlash(reference.Path)) {
		return fmt.Errorf("reference path %q must be state-root-relative", reference.Path)
	}
	openPath, err := canonicalExistingPath(reference.OpenPath)
	if err != nil || openPath != reference.OpenPath {
		return fmt.Errorf("reference open path %q is not a canonical regular path", reference.OpenPath)
	}
	rel, inside := relativePathInside(root, openPath)
	if !inside || filepath.ToSlash(rel) != reference.Path {
		return fmt.Errorf("reference path %q does not name its open path", reference.Path)
	}
	if _, inside := relativePathInside(ownedDir, openPath); !inside || openPath == ownedDir {
		return fmt.Errorf("reference open path %q is outside the immutable handoff directory", reference.OpenPath)
	}
	if _, mismatch := dispatch.ReadVerifiedReference(root, reference.CompositionReference); mismatch != nil {
		return fmt.Errorf("handoff reference does not verify: %s", mismatch.Line())
	}
	return nil
}

func validateManifestReference(root, ownedDir string, reference dispatch.CompositionReference) ([]byte, error) {
	want := filepath.Join(ownedDir, "manifest.json")
	if reference.OpenPath != want || reference.Lifetime != "immutable" || reference.Slot == "" || reference.Purpose == "" ||
		!handoffDigestPattern.MatchString(reference.Digest) || reference.Bytes < 0 {
		return nil, fmt.Errorf("handoff manifest reference is incomplete or does not name %s", want)
	}
	rel, inside := relativePathInside(root, want)
	if !inside || reference.Path != filepath.ToSlash(rel) {
		return nil, fmt.Errorf("handoff manifest path %q does not name its owned copy", reference.Path)
	}
	data, mismatch := dispatch.ReadVerifiedReference(root, reference)
	if mismatch != nil {
		return nil, fmt.Errorf("handoff manifest does not verify: %s", mismatch.Line())
	}
	return data, nil
}

func validateHandoffJobs(root, ownedDir string, jobs []HandoffOpenJob) error {
	for _, job := range jobs {
		if job.ID == "" || job.Role == "" || job.Phase == "" || (job.Status != "pending" && job.Status != "running") {
			return fmt.Errorf("open job %q has invalid identity or lifecycle fields", job.ID)
		}
		if !job.Record.Required || job.Record.Status != "" {
			return fmt.Errorf("open job %s must carry a required verified record", job.ID)
		}
		if err := validateHandoffReference(root, ownedDir, job.Record); err != nil {
			return fmt.Errorf("open job %s: %w", job.ID, err)
		}
	}
	return nil
}

func validateHandoffLandings(landings HandoffLastLandings) error {
	for _, row := range landings.History {
		if row.At == "" || row.OpID == "" || (row.Verb != "land-ready" && row.Verb != "release") || row.Actor == "" {
			return fmt.Errorf("last landing history contains an incomplete row")
		}
	}
	for _, row := range landings.Receipts {
		if !handoffReceiptSource.MatchString(row.Receipt) || row.Type == "" || row.Outcome == "" {
			return fmt.Errorf("last landing receipt %q is incomplete", row.Receipt)
		}
	}
	return nil
}

func validateHandoffMessages(messages []HandoffMessage) error {
	for _, message := range messages {
		if message.Nonce == "" || message.Message == "" || message.DeliveryStatus != "pending" {
			return fmt.Errorf("messagesOwed contains an invalid pending notification")
		}
	}
	return nil
}

func validateHandoffLists(root, ownedDir string, jobs []HandoffOpenJob, landings HandoffLastLandings, scratch []HandoffReference, messages []HandoffMessage) error {
	if err := validateHandoffJobs(root, ownedDir, jobs); err != nil {
		return err
	}
	if err := validateHandoffLandings(landings); err != nil {
		return err
	}
	for _, reference := range scratch {
		if err := validateHandoffReference(root, ownedDir, reference); err != nil {
			return fmt.Errorf("scratch reference: %w", err)
		}
	}
	return validateHandoffMessages(messages)
}

func validateHandoffState(root, nonce, goalID string, binding HandoffBinding, state HandoffState, manifest HandoffManifest, hasManifest bool) error {
	if state.SchemaVersion != HandoffSchemaVersion {
		return fmt.Errorf("handoff state schemaVersion must be %d", HandoffSchemaVersion)
	}
	if state.WrittenAt.IsZero() || !state.WrittenAt.Equal(binding.RecordedAt) {
		return fmt.Errorf("handoff state writtenAt does not match the recorded binding time")
	}
	if state.Seat.Machine == "" || state.Seat.Runtime == "" || state.Seat.Session == "" || state.Seat.NormalizedSession == "" {
		return fmt.Errorf("handoff seat identity is incomplete")
	}
	if state.Seat.NormalizedSession != goal.NormalizeSession(state.Seat.Session) || state.Seat.NormalizedSession != binding.Session ||
		state.Seat.Runtime != binding.Runtime || state.Seat.MainID != binding.MainId || state.Seat.Identity != binding.Predecessor ||
		state.Seat.Tag != binding.PredecessorTag || state.Seat.JobID != binding.PredecessorJob {
		return fmt.Errorf("handoff seat does not match its launch binding")
	}
	if state.Seat.Identity.Mode() == identity.CompareInvalid {
		return fmt.Errorf("handoff seat predecessor identity is invalid")
	}
	if state.HeldGoal.ID != goalID || state.HeldGoal.ID == "" ||
		(state.HeldGoal.State != "claimed" && state.HeldGoal.State != "landing") || state.HeldGoal.AcceptedRevision == 0 ||
		state.HeldGoal.Claimant.Machine == "" || state.HeldGoal.Claimant.Lineage == "" || state.HeldGoal.Claimant.At == "" ||
		state.HeldGoal.Claimant.Revision == 0 {
		return fmt.Errorf("handoff heldGoal does not name the exact accepted claim")
	}
	if state.NextStep.Text == "" {
		return fmt.Errorf("handoff nextStep text is empty")
	}
	ownedDir := HandoffDir(root, nonce)
	for _, reference := range state.NextStep.References {
		if !reference.Required || reference.Status != "" {
			return fmt.Errorf("nextStep references must be required verified copies")
		}
		if err := validateHandoffReference(root, ownedDir, reference); err != nil {
			return fmt.Errorf("nextStep reference: %w", err)
		}
	}
	if state.Disposable != HandoffDisposable {
		return fmt.Errorf("handoff disposable declaration is missing or changed")
	}
	if hasManifest && manifest.SchemaVersion != HandoffSchemaVersion {
		return fmt.Errorf("handoff manifest schemaVersion must be %d", HandoffSchemaVersion)
	}
	if err := validateHandoffLists(root, ownedDir, state.OpenJobs, state.LastLandings, state.Scratch, state.MessagesOwed); err != nil {
		return err
	}
	if err := validateHandoffLists(root, ownedDir, manifest.OpenJobs, manifest.LastLandings, manifest.Scratch, manifest.MessagesOwed); err != nil {
		return fmt.Errorf("handoff manifest: %w", err)
	}
	if len(state.OpenJobs) > maxHandoffOpenJobs {
		return fmt.Errorf("handoff inline openJobs exceeds %d", maxHandoffOpenJobs)
	}
	if len(state.Scratch) > maxHandoffScratch {
		return fmt.Errorf("handoff inline scratch exceeds %d", maxHandoffScratch)
	}
	if len(state.MessagesOwed) > maxHandoffMessages {
		return fmt.Errorf("handoff inline messagesOwed exceeds %d", maxHandoffMessages)
	}
	if len(state.LastLandings.History)+len(manifest.LastLandings.History) > maxHandoffLandings ||
		len(state.LastLandings.Receipts)+len(manifest.LastLandings.Receipts) > maxHandoffLandings {
		return fmt.Errorf("handoff lastLandings exceeds %d history or receipt rows", maxHandoffLandings)
	}
	return nil
}

func validateHandoffBinding(root, nonce string, binding HandoffBinding) (string, error) {
	if !handoffNoncePattern.MatchString(nonce) {
		return "", fmt.Errorf("handoff nonce must be 16 lowercase hexadecimal characters")
	}
	canonicalRoot, err := canonicalExistingPath(root)
	if err != nil {
		return "", fmt.Errorf("resolve handoff state root: %w", err)
	}
	expectedPath := filepath.Join(HandoffDir(canonicalRoot, nonce), "state.json")
	if !filepath.IsAbs(binding.StatePath) || filepath.Clean(binding.StatePath) != binding.StatePath || binding.StatePath != expectedPath {
		return "", fmt.Errorf("handoff binding must name the canonical state path %s", expectedPath)
	}
	resolvedState, err := canonicalExistingPath(binding.StatePath)
	if err != nil {
		return "", fmt.Errorf("resolve handoff state path: %w", err)
	}
	if resolvedState != binding.StatePath {
		return "", fmt.Errorf("handoff binding state path resolves outside its canonical owned path: expected=%s found=%s", binding.StatePath, resolvedState)
	}
	if !handoffDigestPattern.MatchString(binding.StateDigest) {
		return "", fmt.Errorf("handoff state digest must be lowercase SHA-256 hex")
	}
	if binding.Runtime == "" || binding.Session == "" || binding.Session != goal.NormalizeSession(binding.Session) {
		return "", fmt.Errorf("handoff runtime and normalized session are required")
	}
	if binding.Predecessor.Pid <= 0 || binding.Predecessor.Mode() == identity.CompareInvalid {
		return "", fmt.Errorf("handoff predecessor identity is invalid")
	}
	if binding.RecordedAt.IsZero() {
		return "", fmt.Errorf("handoff recordedAt is required")
	}
	return canonicalRoot, nil
}

// verifyBoundHandoffState is the one read-side verifier used both when a
// staged authorization is created and immediately before it launches.
func verifyBoundHandoffState(root, nonce, goalID string, binding HandoffBinding) (string, error) {
	canonicalRoot, err := validateHandoffBinding(root, nonce, binding)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(binding.StatePath)
	if err != nil {
		return "", fmt.Errorf("handoff state is not a readable regular file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("handoff state is not a readable regular file: mode=%s", info.Mode())
	}
	if info.Size() >= int64(output.MaxInlineBytes) {
		return "", fmt.Errorf("handoff state must be smaller than %d bytes (found %d)", output.MaxInlineBytes, info.Size())
	}
	data, err := os.ReadFile(binding.StatePath)
	if err != nil {
		return "", fmt.Errorf("read handoff state: %w", err)
	}
	sum := sha256.Sum256(data)
	foundDigest := hex.EncodeToString(sum[:])
	if foundDigest != binding.StateDigest {
		return foundDigest, fmt.Errorf("handoff state digest mismatch expected=%s found=%s", binding.StateDigest, foundDigest)
	}
	var state HandoffState
	if err := decodeStrictHandoffJSON(data, &state); err != nil {
		return foundDigest, fmt.Errorf("decode handoff state: %w", err)
	}
	manifest := HandoffManifest{}
	if state.Manifest != nil {
		manifestData, err := validateManifestReference(canonicalRoot, HandoffDir(canonicalRoot, nonce), *state.Manifest)
		if err != nil {
			return foundDigest, err
		}
		if err := decodeStrictHandoffJSON(manifestData, &manifest); err != nil {
			return foundDigest, fmt.Errorf("decode handoff manifest: %w", err)
		}
	}
	if err := validateHandoffState(canonicalRoot, nonce, goalID, binding, state, manifest, state.Manifest != nil); err != nil {
		return foundDigest, err
	}
	return foundDigest, nil
}
