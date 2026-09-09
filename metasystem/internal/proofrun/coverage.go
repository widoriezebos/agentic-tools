package proofrun

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
)

type CoverageBeginOptions struct {
	ControlRoot, ExecutionRoot, AttemptID, BaselinePath, ProducerClass string
	ProducerPID, CallerPID                                             int64
}

type CoverageCompleteOptions struct {
	CoverageBeginOptions
	CoverageLog, PackageInventory, ModulePrefix string
}

// CoverageProducerEligible authenticates the descendant first, then decides
// whether it is the full producer for the admitted source identity. A foreign
// nested fixture remains inside the parent proof deadline and budget but does
// not claim or complete the parent's coverage slot.
func CoverageProducerEligible(options CoverageBeginOptions) (bool, error) {
	attempt, _, err := authenticateCoverageCustody(options)
	if err != nil {
		return false, err
	}
	manifestDigest, err := FullDigest(options.ExecutionRoot)
	if err != nil {
		return false, err
	}
	if manifestDigest != attempt.ProofIdentity.ManifestDigest {
		return false, nil
	}
	if err := validateCoverageProducerInputs(options, attempt); err != nil {
		return false, err
	}
	return true, nil
}

func BeginCoverage(options CoverageBeginOptions) error {
	attempt, producer, err := authenticateCoverageProducer(options)
	if err != nil {
		return err
	}
	if options.ProducerClass != "full" || attempt.ProofIdentity.RatchetDigest == "" {
		return fmt.Errorf("only an unseeded full proof can claim coverage production")
	}
	lock, err := AcquireMutation(options.ControlRoot)
	if err != nil {
		return err
	}
	defer lock.Release()
	attempt, err = ReadAttempt(options.ControlRoot, options.AttemptID)
	if err != nil || attempt.Terminal != nil || attempt.CancellationIntent != "" {
		return fmt.Errorf("coverage producer lost its live proof attempt")
	}
	if attempt.PendingCoverage != nil {
		if attempt.PendingCoverage.Producer.Ref() == producer.Ref() && attempt.PendingCoverage.ProducerClass == options.ProducerClass {
			return nil
		}
		return fmt.Errorf("coverage producer slot is already owned")
	}
	attempt.PendingCoverage = &PendingCoverage{Producer: producer, ProducerClass: options.ProducerClass, Expected: attempt.ProofIdentity}
	return writeAttempt(attempt)
}

func CompleteCoverage(options CoverageCompleteOptions) (CoverageEvidence, error) {
	attempt, producer, err := authenticateCoverageProducer(options.CoverageBeginOptions)
	if err != nil {
		return CoverageEvidence{}, err
	}
	baseline, err := audit.ReadCoverageBaseline(options.BaselinePath)
	if err != nil {
		return CoverageEvidence{}, err
	}
	logBytes, err := os.ReadFile(options.CoverageLog)
	if err != nil {
		return CoverageEvidence{}, fmt.Errorf("coverage output unreadable: %w", err)
	}
	inventory, err := readCoverageInventory(options.PackageInventory, options.ModulePrefix)
	if err != nil {
		return CoverageEvidence{}, err
	}
	measured := audit.ParseCoverage(string(logBytes), options.ModulePrefix)
	if violations := audit.CheckCoverage(baseline, measured, inventory); len(violations) > 0 {
		return CoverageEvidence{}, fmt.Errorf("coverage evidence refused: %s", strings.Join(violations, "; "))
	}
	// The complete frozen export is authenticated against the attempt before
	// and after measurement. Reusable legacy ENGINE coverage deliberately uses
	// the narrower behavior projection so unrelated non-engine records do not
	// invalidate a measurement of identical executable source.
	engineDigest, err := Digest(options.ExecutionRoot)
	if err != nil {
		return CoverageEvidence{}, err
	}
	evidence := CoverageEvidence{SchemaVersion: 1, Producer: producer, ProducerClass: options.ProducerClass,
		AttemptID: attempt.AttemptID, PackageInventory: inventory, Measurements: measured,
		EngineDigest: engineDigest, EngineManifest: engineDigest, BehaviorPolicy: attempt.ProofIdentity.BehaviorPolicy,
		Platform: attempt.ProofIdentity.Platform, Toolchain: attempt.ProofIdentity.Toolchain,
		RatchetDigest: attempt.ProofIdentity.RatchetDigest}
	lock, err := AcquireMutation(options.ControlRoot)
	if err != nil {
		return CoverageEvidence{}, err
	}
	defer lock.Release()
	attempt, err = ReadAttempt(options.ControlRoot, options.AttemptID)
	if err != nil || attempt.Terminal != nil || attempt.CancellationIntent != "" || attempt.PendingCoverage == nil ||
		attempt.PendingCoverage.Producer.Ref() != producer.Ref() || attempt.PendingCoverage.ProducerClass != options.ProducerClass {
		return CoverageEvidence{}, fmt.Errorf("coverage producer no longer owns the live pending slot")
	}
	current, err := BuildProofIdentity(options.ExecutionRoot, filepath.Join(options.ExecutionRoot, "metasystem.conf"),
		attempt.ProofIdentity.ScopeClass, attempt.ProofIdentity.CommandClass, attempt.ProofIdentity.Sections, attempt.ProofIdentity.BehaviorPolicy)
	if err != nil || current.IdentityDigest != attempt.PendingCoverage.Expected.IdentityDigest {
		return CoverageEvidence{}, fmt.Errorf("proof inputs changed while coverage was measured")
	}
	evidence.CompletedAt = nowStamp()
	attempt.PendingCoverage.Evidence = &evidence
	if err := writeAttempt(attempt); err != nil {
		return CoverageEvidence{}, err
	}
	return evidence, nil
}

func authenticateCoverageProducer(options CoverageBeginOptions) (Attempt, ProcessIdentity, error) {
	attempt, producer, err := authenticateCoverageCustody(options)
	if err != nil {
		return Attempt{}, ProcessIdentity{}, err
	}
	if err := validateCoverageProducerInputs(options, attempt); err != nil {
		return Attempt{}, ProcessIdentity{}, err
	}
	return attempt, producer, nil
}

func authenticateCoverageCustody(options CoverageBeginOptions) (Attempt, ProcessIdentity, error) {
	if options.ControlRoot == "" || options.ExecutionRoot == "" || options.AttemptID == "" || options.BaselinePath == "" ||
		options.ProducerPID < 1 || options.CallerPID < 1 {
		return Attempt{}, ProcessIdentity{}, fmt.Errorf("coverage producer context is incomplete")
	}
	attempt, err := AuthenticateContext(options.ControlRoot, options.AttemptID, options.CallerPID)
	if err != nil {
		return Attempt{}, ProcessIdentity{}, err
	}
	producer, err := ProcessIdentityForPID(options.ProducerPID, nil)
	if err != nil {
		return Attempt{}, ProcessIdentity{}, err
	}
	if err := AuthenticateAncestor(options.CallerPID, producer); err != nil {
		return Attempt{}, ProcessIdentity{}, fmt.Errorf("coverage handler is outside the producer process: %w", err)
	}
	if err := AuthenticateAncestor(options.ProducerPID, attempt.Launcher); err != nil {
		return Attempt{}, ProcessIdentity{}, fmt.Errorf("coverage producer is outside the proof launcher: %w", err)
	}
	return attempt, producer, nil
}

func validateCoverageProducerInputs(options CoverageBeginOptions, attempt Attempt) error {
	baselineDigest, err := fileSHA256(options.BaselinePath)
	if err != nil || baselineDigest != attempt.ProofIdentity.RatchetDigest {
		return fmt.Errorf("coverage baseline does not match the admitted ratchet bytes")
	}
	current, err := BuildProofIdentity(options.ExecutionRoot, filepath.Join(options.ExecutionRoot, "metasystem.conf"),
		attempt.ProofIdentity.ScopeClass, attempt.ProofIdentity.CommandClass, attempt.ProofIdentity.Sections, attempt.ProofIdentity.BehaviorPolicy)
	if err != nil || current.IdentityDigest != attempt.ProofIdentity.IdentityDigest {
		return fmt.Errorf("coverage producer source identity does not match the admitted proof")
	}
	return nil
}

func ReusableCoverage(root, executionRoot, baselinePath string, packages []string) (*CoverageEvidence, bool, error) {
	baseline, err := audit.ReadCoverageBaseline(baselinePath)
	if err != nil {
		return nil, false, err
	}
	attempts, err := ReadAttempts(root)
	if err != nil {
		return nil, false, err
	}
	sort.Slice(attempts, func(i, j int) bool {
		left, _ := time.Parse(time.RFC3339Nano, attempts[i].EndedAt)
		right, _ := time.Parse(time.RFC3339Nano, attempts[j].EndedAt)
		return left.After(right)
	})
	for _, attempt := range attempts {
		evidence, found, matchErr := reusableCoverageForAttempt(attempt, executionRoot, baselinePath, baseline, packages)
		if matchErr != nil {
			return nil, false, matchErr
		}
		if found {
			return evidence, true, nil
		}
	}
	return nil, false, nil
}

// ReusableCoverageForAttempt validates one exact receipt-bound producer. It
// does not substitute a newer proof when the named retained attempt is stale.
func ReusableCoverageForAttempt(root, executionRoot, baselinePath, attemptID string, packages []string) (*CoverageEvidence, bool, error) {
	baseline, err := audit.ReadCoverageBaseline(baselinePath)
	if err != nil {
		return nil, false, err
	}
	attempt, err := ReadAttempt(root, attemptID)
	if err != nil {
		return nil, false, err
	}
	return reusableCoverageForAttempt(attempt, executionRoot, baselinePath, baseline, packages)
}

func reusableCoverageForAttempt(attempt Attempt, executionRoot, baselinePath string, baseline *audit.CoverageBaseline, packages []string) (*CoverageEvidence, bool, error) {
	if attempt.Terminal == nil || attempt.Terminal.Result != TerminalSuccess || len(CommittedDeliveryReceipt(attempt)) == 0 ||
		attempt.PendingCoverage == nil || attempt.PendingCoverage.Evidence == nil {
		return nil, false, nil
	}
	engineDigest, err := Digest(executionRoot)
	if err != nil {
		return nil, false, err
	}
	toolchain, err := CompleteToolchainIdentityAtWithEnvironment(executionRoot, os.Environ())
	if err != nil {
		return nil, false, err
	}
	ratchet, err := fileSHA256(baselinePath)
	if err != nil {
		return nil, false, err
	}
	evidence := attempt.PendingCoverage.Evidence
	if evidence.SchemaVersion != 1 || evidence.AttemptID != attempt.AttemptID || evidence.ProducerClass != "full" ||
		evidence.EngineDigest != engineDigest || evidence.EngineManifest != engineDigest ||
		evidence.BehaviorPolicy != behaviorsurface.SupportedVersion ||
		evidence.Platform != runtime.GOOS+"/"+runtime.GOARCH || evidence.Toolchain != toolchain || evidence.RatchetDigest != ratchet {
		return nil, false, nil
	}
	if violations := audit.CheckCoverage(baseline, evidence.Measurements, evidence.PackageInventory); len(violations) > 0 {
		return nil, false, nil
	}
	available := make(map[string]bool, len(evidence.PackageInventory))
	for _, pkg := range evidence.PackageInventory {
		available[pkg] = true
	}
	for _, pkg := range packages {
		if !available[pkg] {
			return nil, false, nil
		}
	}
	copy := *evidence
	return &copy, true, nil
}

func readCoverageInventory(path, modulePrefix string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("package inventory unreadable: %w", err)
	}
	defer file.Close()
	var inventory []string
	seen := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		pkg := strings.TrimSpace(scanner.Text())
		pkg = strings.TrimPrefix(pkg, modulePrefix)
		if pkg != "" && !seen[pkg] {
			inventory = append(inventory, pkg)
			seen[pkg] = true
		}
	}
	if err := scanner.Err(); err != nil || len(inventory) == 0 {
		return nil, fmt.Errorf("package inventory is empty or unreadable")
	}
	sort.Strings(inventory)
	return inventory, nil
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func nowStamp() string {
	return timeNow().UTC().Format(time.RFC3339Nano)
}

var timeNow = func() time.Time { return time.Now() }
