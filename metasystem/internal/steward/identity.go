package steward

// The steward's installation identity: a record minted by the
// human-run install, authenticated by ownership, mode, and content.
// It accident-proofs recovery at the repository's trust level: scheduled
// recovery executes only enrolled bytes, while ordinary session arming may
// replace those bytes only when their build stamp proves they were landed.
// A same-user adversary is out of scope repo-wide.

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// ErrEnrollmentDrift marks a recovery refusal caused by changed, incomplete,
// or missing enrolled engine bytes.
var ErrEnrollmentDrift = errors.New("ENROLLMENT_DRIFT")

// ErrEngineRebuilt distinguishes changed bytes at the enrolled path from
// every other enrollment failure. Verifiers refuse both; ordinary up may
// resolve this one cause after proving the new bytes came from a landed tree.
var ErrEngineRebuilt = errors.New("enrolled engine digest changed")

const (
	EnrollmentHumanTerminal = "human-terminal"
	EnrollmentTemporaryWord = "temporary-word"
	EnrollmentFixture       = "fixture"
)

// InstallIdentity is the minted record's content.
type InstallIdentity struct {
	// RepoIdentity pins which repository this installation serves.
	RepoIdentity string `json:"repoIdentity"`
	// Generation increments at every reinstall; intents bind to it,
	// so records from a superseded installation stop authorizing.
	Generation int `json:"generation"`
	// InstallPath is where the installed steward bytes live.
	InstallPath string `json:"installPath"`
	// InstallDigest binds recovery to the exact metasystem engine enrolled by
	// ordinary up. The steward and supervision owner use the same binary.
	InstallDigest string `json:"installDigest,omitempty"`
	MintedAt      string `json:"mintedAt"`
	// Enrollment records the authority that admitted this installation. An
	// absent value is a human-terminal enrollment created before this field
	// existed.
	Enrollment string `json:"enrollment,omitempty"`
	// TemporaryHumanWord records a remote human authorization for an
	// enrollment performed without an agent-free terminal (the human was
	// away from the machine and spoke through an agent session). It is
	// durable provenance, not a weaker enrollment: health and every
	// reader see the temporary state until a terminal re-arm replaces
	// this generation. ReviewBy is the human's own re-approval date.
	TemporaryHumanWord string `json:"temporaryHumanWord,omitempty"`
	ReviewBy           string `json:"reviewBy,omitempty"`
	// MintedBy names the act that published this generation. Its absence marks
	// an enrollment created before provenance stamping existed.
	MintedBy                 string `json:"mintedBy,omitempty"`
	HumanWitnessedGeneration int    `json:"humanWitnessedGeneration,omitempty"`
	HumanWitnessedAt         string `json:"humanWitnessedAt,omitempty"`
	// EngineBuild is read from the enrolled bytes, never from the process that
	// performs the enrollment.
	EngineBuild string `json:"engineBuild,omitempty"`
	// Machine rebuilds record the landed commit and remote-tracking ref that
	// admitted the bytes. Human enrollments leave both empty.
	LandedCommit string `json:"landedCommit,omitempty"`
	LandingRef   string `json:"landingRef,omitempty"`
}

func identityDurabilityPendingPath(path string) string { return path + ".durability-pending" }

var identityWriter = atomicfile.WriteText

// publishIdentity replaces the record through the repository's durable
// publication contract. A durable marker remains beside a visible record
// whenever the rename landed but its directory sync could not be confirmed.
func publishIdentity(path string, id InstallIdentity) (bool, error) {
	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return false, err
	}
	marker := identityDurabilityPendingPath(path)
	markerDurable, err := identityWriter(marker, "pending\n", id.RepoIdentity)
	if err != nil {
		return false, err
	}
	durable, err := identityWriter(path, string(append(data, '\n')), id.RepoIdentity)
	if err != nil {
		return false, err
	}
	if !markerDurable || !durable {
		return false, nil
	}
	if err := os.Remove(marker); err != nil && !os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}

// MintIdentity writes the record with owner-only permissions and reports
// success only after its publication is confirmed durable.
func MintIdentity(path string, id InstallIdentity) error {
	durable, err := publishIdentity(path, id)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("steward identity generation %d is visible but its durability is pending", id.Generation)
	}
	return nil
}

// VerifyIdentity authenticates the record for a repository: it must
// exist, be owned by the calling user, carry owner-only permissions,
// and name the expected repository. Every failure is named — the
// classifier refuses on any of them rather than guessing.
func VerifyIdentity(path, wantRepoIdentity string) (InstallIdentity, error) {
	var id InstallIdentity
	info, err := os.Stat(path)
	if err != nil {
		return id, fmt.Errorf("steward identity absent: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return id, fmt.Errorf("steward identity %s is not owner-only (mode %o)", path, info.Mode().Perm())
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return id, fmt.Errorf("steward identity %s is owned by uid %d, not the calling user", path, st.Uid)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return id, fmt.Errorf("steward identity unreadable: %w", err)
	}
	if err := json.Unmarshal(data, &id); err != nil {
		return id, fmt.Errorf("steward identity malformed: %w", err)
	}
	if id.RepoIdentity != wantRepoIdentity {
		return id, fmt.Errorf("steward identity serves repository %q, not %q", id.RepoIdentity, wantRepoIdentity)
	}
	if id.Generation < 1 {
		return id, fmt.Errorf("steward identity carries no valid generation")
	}
	switch id.Enrollment {
	case "":
		id.Enrollment = EnrollmentHumanTerminal
	case EnrollmentHumanTerminal, EnrollmentTemporaryWord, EnrollmentFixture:
	default:
		return id, fmt.Errorf("steward identity carries unknown enrollment %q", id.Enrollment)
	}
	return id, nil
}

func installDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return digestOpenFile(file)
}

func digestOpenFile(file *os.File) (string, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil)), nil
}

func buildStampFromOpenFile(file *os.File) string {
	info, err := buildinfo.Read(file)
	if err != nil {
		return ""
	}
	const assignment = "supervise.BuildStamp="
	for _, setting := range info.Settings {
		if setting.Key != "-ldflags" {
			continue
		}
		at := strings.Index(setting.Value, assignment)
		if at < 0 {
			continue
		}
		value := setting.Value[at+len(assignment):]
		if end := strings.IndexAny(value, " \t\r\n\"'"); end >= 0 {
			value = value[:end]
		}
		return strings.TrimSpace(value)
	}
	return ""
}

// enrolledBytes is one locked observation of an installation path. The same
// open descriptor supplies both its digest and its linked build stamp.
type enrolledBytes struct {
	File   *os.File
	Digest string
	Stamp  string
	Err    error
}

func readEnrolledBytes(installed InstallIdentity) enrolledBytes {
	if installed.InstallPath == "" || installed.InstallDigest == "" {
		return enrolledBytes{Err: fmt.Errorf("%w: enrolled engine path or digest is absent", ErrEnrollmentDrift)}
	}
	canonicalInstallPath := canonicalPath(installed.InstallPath)
	if installed.InstallPath != canonicalInstallPath {
		return enrolledBytes{Err: fmt.Errorf("%w: enrolled engine path %q is not canonical (%q)",
			ErrEnrollmentDrift, installed.InstallPath, canonicalInstallPath)}
	}
	file, err := os.Open(installed.InstallPath)
	if err != nil {
		return enrolledBytes{Err: fmt.Errorf("%w: open enrolled engine %q: %v", ErrEnrollmentDrift, installed.InstallPath, err)}
	}
	fail := func(err error) enrolledBytes {
		_ = file.Close()
		return enrolledBytes{Err: err}
	}
	openedInfo, err := file.Stat()
	if err != nil {
		return fail(fmt.Errorf("%w: inspect opened engine %q: %v", ErrEnrollmentDrift, installed.InstallPath, err))
	}
	pathInfo, err := os.Stat(installed.InstallPath)
	if err != nil {
		return fail(fmt.Errorf("%w: inspect enrolled engine path %q: %v", ErrEnrollmentDrift, installed.InstallPath, err))
	}
	if !os.SameFile(openedInfo, pathInfo) {
		return fail(fmt.Errorf("%w: enrolled engine path changed while it was being pinned", ErrEnrollmentDrift))
	}
	if !openedInfo.Mode().IsRegular() || openedInfo.Mode()&0o111 == 0 {
		return fail(fmt.Errorf("%w: enrolled engine %q is not an executable regular file", ErrEnrollmentDrift, installed.InstallPath))
	}
	digest, err := digestOpenFile(file)
	if err != nil {
		return fail(fmt.Errorf("%w: read enrolled engine %q: %v", ErrEnrollmentDrift, installed.InstallPath, err))
	}
	return enrolledBytes{File: file, Digest: digest, Stamp: buildStampFromOpenFile(file)}
}

// EnrolledBinary pins the verified engine inode open. Commands execute that
// descriptor, so replacing the path after verification cannot change the
// bytes that become a supervisor or steward process.
type EnrolledBinary struct {
	Install  InstallIdentity
	repoRoot string
	file     *os.File
	execPath string
	execFile *os.File
}

// BuildStamp returns the linker-stamped source identity from the same open
// executable descriptor whose digest enrollment authenticated. Callers that
// need source-bound policy decisions must compare this value with their
// captured destination commit instead of trusting a path or record label.
func (b *EnrolledBinary) BuildStamp() string {
	if b == nil || b.file == nil {
		return ""
	}
	return buildStampFromOpenFile(b.file)
}

// Close releases the pinned engine descriptor.
func (b *EnrolledBinary) Close() error {
	if b == nil || b.file == nil {
		return nil
	}
	var first error
	if b.execFile != nil {
		first = b.execFile.Close()
	}
	if err := b.file.Close(); first == nil {
		first = err
	}
	return first
}

// PrepareForExecution copies the already verified open descriptor into an
// owner-only executable snapshot. Atomic replacement of the enrolled path
// cannot change this snapshot, and the snapshot remains available to the
// detached owner and runner after the arming command exits.
func (b *EnrolledBinary) PrepareForExecution() error {
	if b == nil || b.file == nil {
		return fmt.Errorf("the enrolled engine is not open")
	}
	directory := filepath.Join(b.repoRoot, "artifacts", "agents", "steward", "engine-pins")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	// One preparer owns publication through the descriptor open, so another
	// process can reuse but never replace a valid pin retained by this caller.
	prepareLockPath := filepath.Join(directory, ".prepare.flock")
	prepareLock, err := os.OpenFile(prepareLockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open engine pin preparation lock %s: %w", prepareLockPath, err)
	}
	if err := unix.Flock(int(prepareLock.Fd()), unix.LOCK_EX); err != nil {
		_ = prepareLock.Close()
		return fmt.Errorf("take engine pin preparation lock %s: %w", prepareLockPath, err)
	}
	defer func() {
		_ = unix.Flock(int(prepareLock.Fd()), unix.LOCK_UN)
		_ = prepareLock.Close()
	}()
	finalPath := EnrolledExecutionPath(b.repoRoot, b.Install)
	if existing, err := os.Open(finalPath); err == nil {
		if digest, digestErr := digestOpenFile(existing); digestErr == nil && digest == b.Install.InstallDigest {
			if info, statErr := existing.Stat(); statErr == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
				b.execPath, b.execFile = finalPath, existing
				return nil
			}
		}
		_ = existing.Close()
	}
	temporary, err := os.CreateTemp(directory, ".engine-pin-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	keep := false
	defer func() {
		_ = temporary.Close()
		if !keep {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := b.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.Copy(temporary, b.file); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Chmod(0o500); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if digest, err := installDigest(temporaryPath); err != nil || digest != b.Install.InstallDigest {
		return fmt.Errorf("%w: verified engine changed while its executable snapshot was prepared", ErrEnrollmentDrift)
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return err
	}
	keep = true
	execFile, err := os.Open(finalPath)
	if err != nil {
		return err
	}
	b.execPath, b.execFile = finalPath, execFile
	return nil
}

// EnrolledExecutionPath is the deterministic owner-only snapshot path for one
// accepted generation. Classifiers use it beside InstallPath so a pinned
// steward process remains attributable to its enrollment.
func EnrolledExecutionPath(repoRoot string, installed InstallIdentity) string {
	digestName := strings.TrimPrefix(installed.InstallDigest, "sha256:")
	return filepath.Join(canonicalPath(repoRoot), "artifacts", "agents", "steward", "engine-pins",
		fmt.Sprintf("generation-%d-%s", installed.Generation, digestName))
}

// Command re-verifies that the executable path still names the prepared
// inode immediately before the caller starts it.
func (b *EnrolledBinary) Command(args ...string) (*exec.Cmd, error) {
	if b == nil || b.execFile == nil || b.execPath == "" {
		return nil, fmt.Errorf("the enrolled engine has no prepared executable snapshot")
	}
	openedInfo, err := b.execFile.Stat()
	if err != nil {
		return nil, err
	}
	digest, err := digestOpenFile(b.execFile)
	if err != nil || digest != b.Install.InstallDigest {
		return nil, fmt.Errorf("%w: prepared engine bytes changed before execution", ErrEnrollmentDrift)
	}
	pathInfo, err := os.Stat(b.execPath)
	if err != nil || !os.SameFile(openedInfo, pathInfo) {
		return nil, fmt.Errorf("%w: prepared engine inode changed before execution", ErrEnrollmentDrift)
	}
	return exec.Command(b.execPath, args...), nil
}

// OpenEnrolledBinary authenticates the enrollment, opens its engine, proves
// the path still names that same inode, and verifies the digest through the
// open descriptor.
func OpenEnrolledBinary(repoRoot string) (*EnrolledBinary, error) {
	top := canonicalPath(repoRoot)
	installed, err := VerifyIdentity(RepoIdentityPath(top), top)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEnrollmentDrift, err)
	}
	bytes := readEnrolledBytes(installed)
	if bytes.Err != nil {
		return nil, bytes.Err
	}
	if bytes.Digest != installed.InstallDigest {
		_ = bytes.File.Close()
		return nil, fmt.Errorf("%w: %w (recorded %s, current %s)",
			ErrEnrollmentDrift, ErrEngineRebuilt, installed.InstallDigest, bytes.Digest)
	}
	return &EnrolledBinary{Install: installed, repoRoot: top, file: bytes.File}, nil
}

// VerifyEnrolledBinary authenticates the repository enrollment and proves
// that its canonical engine path still contains the exact enrolled bytes.
// Recovery uses this before it starts any repository ring.
func VerifyEnrolledBinary(repoRoot string) (InstallIdentity, error) {
	pinned, err := OpenEnrolledBinary(repoRoot)
	if err != nil {
		return InstallIdentity{}, err
	}
	defer pinned.Close()
	return pinned.Install, nil
}
