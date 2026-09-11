package proofrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ExecutionContext freezes the source and effective environment facts used by
// admission. Callers must launch the proof with the same Environment bytes.
type ExecutionContext struct {
	ExecutionRoot     string
	ConfigurationPath string
	Environment       []string
	ManifestDigest    string
	Configuration     string
	Platform          string
	Toolchain         string
	RatchetDigest     string
}

// CaptureSharedExecutionContext binds a schema-2 shared test attempt to the
// prepared project bytes, effective configuration, installed engine, and
// runtime platform without requiring a Go installation. Language adapters
// identify their own tools only when selected.
func CaptureSharedExecutionContext(executionRoot, configurationPath string, environment []string, enginePath, preparedManifest string) (ExecutionContext, error) {
	if len(environment) == 0 {
		environment = os.Environ()
	}
	context := ExecutionContext{ExecutionRoot: executionRoot, ConfigurationPath: configurationPath,
		Environment: append([]string(nil), environment...), Platform: runtime.GOOS + "/" + runtime.GOARCH}
	var err error
	if preparedManifest != "" {
		if len(preparedManifest) != sha256.Size*2 {
			return ExecutionContext{}, fmt.Errorf("prepared testing manifest must be a SHA-256 digest")
		}
		if _, err := hex.DecodeString(preparedManifest); err != nil {
			return ExecutionContext{}, fmt.Errorf("prepared testing manifest must be a SHA-256 digest")
		}
		context.ManifestDigest = preparedManifest
	} else if context.ManifestDigest, err = FullDigest(executionRoot); err != nil {
		return ExecutionContext{}, err
	}
	if context.Configuration, err = effectiveProofConfigurationDigest(configurationPath, context.Environment); err != nil {
		return ExecutionContext{}, err
	}
	engine, err := os.Open(enginePath)
	if err != nil {
		return ExecutionContext{}, fmt.Errorf("open testing engine: %w", err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, engine)
	closeErr := engine.Close()
	if copyErr != nil || closeErr != nil {
		return ExecutionContext{}, fmt.Errorf("hash testing engine: %v", errors.Join(copyErr, closeErr))
	}
	context.Toolchain = hex.EncodeToString(hash.Sum(nil))
	return context, nil
}

func CaptureExecutionContext(executionRoot, configurationPath string, environment []string) (ExecutionContext, error) {
	if len(environment) == 0 {
		environment = os.Environ()
	}
	context := ExecutionContext{
		ExecutionRoot: executionRoot, ConfigurationPath: configurationPath,
		Environment: append([]string(nil), environment...), Platform: runtime.GOOS + "/" + runtime.GOARCH,
	}
	var err error
	if context.ManifestDigest, err = FullDigest(executionRoot); err != nil {
		return ExecutionContext{}, err
	}
	if context.Configuration, err = effectiveProofConfigurationDigest(configurationPath, context.Environment); err != nil {
		return ExecutionContext{}, err
	}
	if context.Toolchain, err = CompleteToolchainIdentityAtWithEnvironment(executionRoot, context.Environment); err != nil {
		return ExecutionContext{}, err
	}
	if context.RatchetDigest, err = coverageRatchetDigest(executionRoot); err != nil {
		return ExecutionContext{}, err
	}
	return context, nil
}

func CompleteToolchainIdentityAtWithEnvironment(root string, environment []string) (string, error) {
	versionCommand, err := explicitEnvironmentCommand(context.Background(), root, environment, []string{"go", "version"})
	if err != nil {
		return "", fmt.Errorf("resolve go from prepared environment: %w", err)
	}
	environmentCommand, err := explicitEnvironmentCommand(context.Background(), root, environment,
		[]string{"go", "env", "GOOS", "GOARCH", "GOFLAGS", "GOWORK", "GOEXPERIMENT", "CGO_ENABLED", "GOTOOLCHAIN"})
	if err != nil {
		return "", fmt.Errorf("resolve go from prepared environment: %w", err)
	}
	version, err := versionCommand.Output()
	if err != nil {
		return "", fmt.Errorf("read go version: %w", err)
	}
	values, err := environmentCommand.Output()
	if err != nil {
		return "", fmt.Errorf("read complete go environment: %w", err)
	}
	digest := sha256.Sum256(append(version, values...))
	return hex.EncodeToString(digest[:]), nil
}

// ToolchainClosureIdentity binds the selected Go front-end bytes and release
// VERSION file beside the reported version and effective Go environment.
func ToolchainClosureIdentity(root string, environment []string) (string, error) {
	reported, err := CompleteToolchainIdentityAtWithEnvironment(root, environment)
	if err != nil {
		return "", err
	}
	goCommand, err := explicitEnvironmentCommand(context.Background(), root, environment, []string{"go", "version"})
	if err != nil {
		return "", fmt.Errorf("resolve go from prepared environment: %w", err)
	}
	goPath, err := filepath.EvalSymlinks(goCommand.Path)
	if err != nil {
		return "", fmt.Errorf("resolve selected go executable: %w", err)
	}
	goBytes, err := os.ReadFile(goPath)
	if err != nil {
		return "", fmt.Errorf("read selected go executable: %w", err)
	}
	goDigest := sha256.Sum256(goBytes)
	gorootCommand, err := explicitEnvironmentCommand(context.Background(), root, environment, []string{"go", "env", "GOROOT"})
	if err != nil {
		return "", fmt.Errorf("resolve go from prepared environment: %w", err)
	}
	gorootBytes, err := gorootCommand.Output()
	if err != nil {
		return "", fmt.Errorf("read selected Go root: %w", err)
	}
	goroot := strings.TrimSpace(string(gorootBytes))
	if goroot == "" {
		return "", fmt.Errorf("selected Go root is empty")
	}
	versionBytes, err := os.ReadFile(filepath.Join(goroot, "VERSION"))
	if err != nil {
		return "", fmt.Errorf("read selected Go root VERSION: %w", err)
	}
	payload := make([]byte, 0, len(reported)+len(goDigest)+len(versionBytes)+2)
	payload = append(payload, reported...)
	payload = append(payload, 0)
	payload = append(payload, goDigest[:]...)
	payload = append(payload, 0)
	payload = append(payload, versionBytes...)
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func environmentLookup(environment []string) func(string) (string, bool) {
	values := make(map[string]string, len(environment))
	present := make(map[string]bool, len(environment))
	for _, entry := range environment {
		name, value, found := strings.Cut(entry, "=")
		if found {
			values[name], present[name] = value, true
		}
	}
	return func(name string) (string, bool) { return values[name], present[name] }
}
