package dispatch

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

const LaunchFingerprintVersion = 1

type DispatchMode string

const (
	DispatchModeFresh    DispatchMode = "fresh"
	DispatchModeFollowUp DispatchMode = "follow-up"
)

type LaunchMode string

const (
	LaunchModeWorktree       LaunchMode = "worktree"
	LaunchModeSharedCheckout LaunchMode = "shared-checkout"
)

// LaunchFingerprintRequest is the caller's complete process-creating
// request. A zero CapMinutes means the caller selected the supplied default;
// the default is resolved before the request reaches the wire encoder.
type LaunchFingerprintRequest struct {
	SessionKey               string
	DispatchMode             DispatchMode
	ResumedSessionID         *string
	Runtime                  string
	Model                    string
	Role                     string
	LaunchMode               LaunchMode
	PermissionEnvelopeDigest string
	ProductRoots             []string
	CapMinutes               int64
	InputHash                string
}

// CanonicalLaunchRequest is the v1 tuple after model, root, and cap
// normalization. Its JSON names are also the committed golden-vector shape.
type CanonicalLaunchRequest struct {
	SessionKey               string       `json:"sessionKey"`
	DispatchMode             DispatchMode `json:"dispatchMode"`
	ResumedSessionID         string       `json:"resumedSessionId"`
	Runtime                  string       `json:"runtime"`
	CanonicalModelKey        string       `json:"canonicalModelKey"`
	Role                     string       `json:"role"`
	LaunchMode               LaunchMode   `json:"launchMode"`
	PermissionEnvelopeDigest string       `json:"permissionEnvelopeDigest"`
	ProductRoots             []string     `json:"productRoots"`
	CapMinutes               int64        `json:"capMinutes"`
	InputHash                string       `json:"inputHash"`
}

type LaunchFingerprint struct {
	Version int                    `json:"fingerprintVersion"`
	Digest  string                 `json:"fingerprint"`
	Request CanonicalLaunchRequest `json:"request"`
}

type fingerprintWireField struct {
	Present bool
	Value   []byte
}

var launchFingerprintV1Preamble = []byte{'C', 'L', 'M', '-', 'F', 'P', 0, 1}

// CanonicalizeLaunchFingerprint resolves the whole request before hashing.
// Relative roots are based at the Git root supplied by the caller. Roots are
// resolved through their deepest existing ancestor, then sorted and
// deduplicated so aliases do not mint different operation identities.
func CanonicalizeLaunchFingerprint(gitRoot string, raw LaunchFingerprintRequest, defaultCapMinutes int64) (LaunchFingerprint, error) {
	if gitRoot == "" {
		return LaunchFingerprint{}, fmt.Errorf("claim-launch requires the Git root")
	}
	if raw.ResumedSessionID == nil {
		return LaunchFingerprint{}, fmt.Errorf("claim-launch resumed session must be explicitly present, including an explicit empty value for fresh dispatch")
	}
	resumed := *raw.ResumedSessionID
	if raw.DispatchMode != DispatchModeFresh && raw.DispatchMode != DispatchModeFollowUp {
		return LaunchFingerprint{}, fmt.Errorf("claim-launch dispatch mode must be fresh or follow-up")
	}
	if raw.DispatchMode == DispatchModeFresh && resumed != "" {
		return LaunchFingerprint{}, fmt.Errorf("a fresh dispatch must carry an explicit empty resumed session")
	}
	if raw.DispatchMode == DispatchModeFollowUp && resumed == "" {
		return LaunchFingerprint{}, fmt.Errorf("a follow-up dispatch requires the resumed runtime session id")
	}
	if raw.LaunchMode != LaunchModeWorktree && raw.LaunchMode != LaunchModeSharedCheckout {
		return LaunchFingerprint{}, fmt.Errorf("claim-launch launch mode must be worktree or shared-checkout")
	}
	modelKey := config.CanonicalModel(raw.Model)
	if raw.SessionKey == "" || raw.Runtime == "" || modelKey == "" || raw.Role == "" {
		return LaunchFingerprint{}, fmt.Errorf("claim-launch session, runtime, model, and role must be non-empty")
	}
	if !incarnationRe.MatchString(raw.PermissionEnvelopeDigest) {
		return LaunchFingerprint{}, fmt.Errorf("claim-launch permission envelope digest must be lowercase SHA-256")
	}
	if !incarnationRe.MatchString(raw.InputHash) {
		return LaunchFingerprint{}, fmt.Errorf("claim-launch input hash must be lowercase SHA-256")
	}
	capMinutes := raw.CapMinutes
	if capMinutes == 0 {
		capMinutes = defaultCapMinutes
	}
	if capMinutes < 1 {
		return LaunchFingerprint{}, fmt.Errorf("claim-launch cap request must resolve to a positive minute count")
	}
	roots, err := canonicalProductRoots(gitRoot, raw.ProductRoots)
	if err != nil {
		return LaunchFingerprint{}, err
	}
	request := CanonicalLaunchRequest{
		SessionKey:               raw.SessionKey,
		DispatchMode:             raw.DispatchMode,
		ResumedSessionID:         resumed,
		Runtime:                  raw.Runtime,
		CanonicalModelKey:        modelKey,
		Role:                     raw.Role,
		LaunchMode:               raw.LaunchMode,
		PermissionEnvelopeDigest: raw.PermissionEnvelopeDigest,
		ProductRoots:             roots,
		CapMinutes:               capMinutes,
		InputHash:                raw.InputHash,
	}
	return LaunchFingerprintV1(request)
}

func canonicalProductRoots(gitRoot string, roots []string) ([]string, error) {
	base := resolvePath(gitRoot)
	resolved := make([]string, 0, len(roots))
	for _, root := range roots {
		if root == "" || !utf8.ValidString(root) {
			return nil, fmt.Errorf("claim-launch product roots must be non-empty UTF-8 paths")
		}
		candidate := root
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(base, candidate)
		}
		candidate = resolvePath(candidate)
		if excludedProductRoot(base, candidate) {
			return nil, fmt.Errorf("claim-launch product root is operational state and cannot be attributed: %s", candidate)
		}
		resolved = append(resolved, candidate)
	}
	sort.Strings(resolved)
	deduplicated := make([]string, 0, len(resolved))
	for _, root := range resolved {
		if len(deduplicated) == 0 || deduplicated[len(deduplicated)-1] != root {
			deduplicated = append(deduplicated, root)
		}
	}
	return deduplicated, nil
}

func excludedProductRoot(gitRoot, root string) bool {
	if pathWithin(root, resolvePath(filepath.Join(gitRoot, ".git"))) {
		return true
	}
	agentState := resolvePath(filepath.Join(gitRoot, "artifacts", "agents"))
	if !pathWithin(root, agentState) {
		return false
	}
	worktrees := resolvePath(filepath.Join(agentState, "worktrees"))
	// The agents directory holds registries and control state, but each child
	// of worktrees is a delegate's product workspace. The shared worktrees
	// container stays excluded, and resolving the root before this decision
	// keeps links back into registry subtrees excluded.
	if root != worktrees && pathWithin(root, worktrees) {
		return false
	}
	return true
}

// LaunchFingerprintV1 hashes the pinned v1 wire tuple. Every scalar is a
// presence byte, an unsigned 64-bit big-endian byte length, and its UTF-8
// bytes. The roots field is present and carries an unsigned 64-bit count,
// followed by the same unsigned 64-bit length plus UTF-8 bytes for each root.
func LaunchFingerprintV1(request CanonicalLaunchRequest) (LaunchFingerprint, error) {
	if err := validateCanonicalLaunchRequest(request); err != nil {
		return LaunchFingerprint{}, err
	}
	capMinutes := strconv.FormatInt(request.CapMinutes, 10)
	roots, err := encodeFingerprintRoots(request.ProductRoots)
	if err != nil {
		return LaunchFingerprint{}, err
	}
	values := [][]byte{
		[]byte(request.SessionKey),
		[]byte(request.DispatchMode),
		[]byte(request.ResumedSessionID),
		[]byte(request.Runtime),
		[]byte(request.CanonicalModelKey),
		[]byte(request.Role),
		[]byte(request.LaunchMode),
		[]byte(request.PermissionEnvelopeDigest),
		roots,
		[]byte(capMinutes),
		[]byte(request.InputHash),
	}
	fields := make([]fingerprintWireField, 0, len(values))
	for _, value := range values {
		fields = append(fields, fingerprintWireField{Present: true, Value: value})
	}
	wire, err := encodeLaunchFingerprintV1(fields)
	if err != nil {
		return LaunchFingerprint{}, err
	}
	sum := sha256.Sum256(wire)
	return LaunchFingerprint{
		Version: LaunchFingerprintVersion,
		Digest:  hex.EncodeToString(sum[:]),
		Request: request,
	}, nil
}

func validateCanonicalLaunchRequest(request CanonicalLaunchRequest) error {
	if request.DispatchMode != DispatchModeFresh && request.DispatchMode != DispatchModeFollowUp {
		return fmt.Errorf("fingerprint v1 dispatch mode must be fresh or follow-up")
	}
	if request.DispatchMode == DispatchModeFresh && request.ResumedSessionID != "" {
		return fmt.Errorf("fingerprint v1 fresh dispatch must carry an explicit empty resumed session")
	}
	if request.DispatchMode == DispatchModeFollowUp && request.ResumedSessionID == "" {
		return fmt.Errorf("fingerprint v1 follow-up requires a resumed session")
	}
	if request.LaunchMode != LaunchModeWorktree && request.LaunchMode != LaunchModeSharedCheckout {
		return fmt.Errorf("fingerprint v1 launch mode is invalid")
	}
	if request.SessionKey == "" || request.Runtime == "" || request.CanonicalModelKey == "" || request.Role == "" || request.CapMinutes < 1 {
		return fmt.Errorf("fingerprint v1 required field is empty")
	}
	if config.CanonicalModel(request.CanonicalModelKey) != request.CanonicalModelKey {
		return fmt.Errorf("fingerprint v1 model key is not canonical")
	}
	if !incarnationRe.MatchString(request.PermissionEnvelopeDigest) || !incarnationRe.MatchString(request.InputHash) {
		return fmt.Errorf("fingerprint v1 digests must be lowercase SHA-256")
	}
	stringsToCheck := []string{
		request.SessionKey, string(request.DispatchMode), request.ResumedSessionID,
		request.Runtime, request.CanonicalModelKey, request.Role, string(request.LaunchMode),
		request.PermissionEnvelopeDigest, request.InputHash,
	}
	for _, value := range stringsToCheck {
		if !utf8.ValidString(value) {
			return fmt.Errorf("fingerprint v1 fields must be UTF-8")
		}
	}
	for index, root := range request.ProductRoots {
		if root == "" || !filepath.IsAbs(root) || !utf8.ValidString(root) {
			return fmt.Errorf("fingerprint v1 product roots must be non-empty absolute UTF-8 paths")
		}
		if index > 0 && request.ProductRoots[index-1] >= root {
			return fmt.Errorf("fingerprint v1 product roots must be sorted and deduplicated")
		}
	}
	return nil
}

func encodeFingerprintRoots(roots []string) ([]byte, error) {
	var buffer bytes.Buffer
	if err := binary.Write(&buffer, binary.BigEndian, uint64(len(roots))); err != nil {
		return nil, err
	}
	for _, root := range roots {
		if !utf8.ValidString(root) {
			return nil, fmt.Errorf("fingerprint v1 root is not UTF-8")
		}
		if err := binary.Write(&buffer, binary.BigEndian, uint64(len([]byte(root)))); err != nil {
			return nil, err
		}
		buffer.WriteString(root)
	}
	return buffer.Bytes(), nil
}

func encodeLaunchFingerprintV1(fields []fingerprintWireField) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.Write(launchFingerprintV1Preamble)
	for _, field := range fields {
		if !field.Present {
			buffer.WriteByte(0)
			if err := binary.Write(&buffer, binary.BigEndian, uint64(0)); err != nil {
				return nil, err
			}
			continue
		}
		buffer.WriteByte(1)
		if err := binary.Write(&buffer, binary.BigEndian, uint64(len(field.Value))); err != nil {
			return nil, err
		}
		buffer.Write(field.Value)
	}
	return buffer.Bytes(), nil
}
