package testimpact

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	CodeUsage           = "TEST_IMPACT_USAGE"
	CodeConfigInvalid   = "TEST_IMPACT_CONFIG_INVALID"
	CodeInputInvalid    = "TEST_IMPACT_INPUT_INVALID"
	CodeResultInvalid   = "TEST_IMPACT_RESULT_INVALID"
	CodeExecutionFailed = "TEST_IMPACT_EXECUTION_FAILED"
)

type Implementation struct {
	Kind string   `json:"kind"`
	CWD  string   `json:"cwd"`
	Argv []string `json:"argv"`
}
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Envelope struct {
	SchemaVersion  int             `json:"schemaVersion"`
	Request        *Request        `json:"request"`
	Implementation *Implementation `json:"implementation"`
	Fallback       bool            `json:"fallback"`
	FallbackReason string          `json:"fallbackReason"`
	Result         *Result         `json:"result"`
	Error          *Failure        `json:"error"`
}
type Options struct {
	Start, Base, Mode string
	Executor          Executor
	Stderr            io.Writer
}

func Run(ctx context.Context, options Options) (Envelope, int) {
	envelope := Envelope{SchemaVersion: SchemaVersion}
	request, err := BuildRequest(ctx, options.Start, options.Base, options.Mode)
	if err != nil {
		return fail(envelope, CodeInputInvalid, err), 2
	}
	envelope.Request = &request
	contract, missing, err := loadContract(request.RepositoryRoot)
	if err != nil {
		return fail(envelope, CodeConfigInvalid, err), 2
	}
	reason := ""
	if contract.TailoringRequired {
		reason = "tailoring-required"
	} else if missing || contract.Impacted == nil {
		reason = "implementation-undeclared"
	}
	if len(contract.Groups) == 0 {
		reason = "no-tests-declared"
	}
	if reason != "" {
		envelope.Fallback, envelope.FallbackReason = true, reason
		envelope.Implementation = &Implementation{Kind: "full-test-fallback", CWD: ".", Argv: []string{}}
		if reason == "no-tests-declared" {
			fmt.Fprintln(options.Stderr, "FULL TEST FALLBACK: NO TESTS DECLARED")
			result := Result{SchemaVersion: SchemaVersion, Mode: request.Mode, Binding: request.Binding, Status: StatusNotRun,
				Selections: []Selection{}, Uncertainty: []Uncertainty{}}
			envelope.Result = &result
			return envelope, 0
		}
		fmt.Fprintf(options.Stderr, "FULL TEST FALLBACK: %s\n", reason)
		outcome := RunFallback(ctx, request.RepositoryRoot, request.Mode, contract, options.Executor, options.Stderr)
		if ctx.Err() != nil {
			return fail(envelope, CodeExecutionFailed, ctx.Err()), 2
		}
		result := fallbackResult(request, outcome)
		if err := ValidateResult(request, result); err != nil {
			return fail(envelope, CodeResultInvalid, err), 2
		}
		envelope.Result = &result
		announceResult(options.Stderr, result)
		return envelope, resultExit(result)
	}
	provider, err := RunProvider(ctx, *contract.Impacted, request.RepositoryRoot, request, options.Executor, options.Stderr)
	envelope.Implementation = &Implementation{Kind: "provider", CWD: contract.Impacted.CWD, Argv: provider.Argv}
	if err != nil {
		code := CodeExecutionFailed
		if !provider.Run.Cancelled && provider.Run.StartErr == nil && provider.Run.ExitCode == 0 {
			code = CodeResultInvalid
		}
		return fail(envelope, code, err), 2
	}
	envelope.Result = &provider.Result
	announceResult(options.Stderr, provider.Result)
	return envelope, resultExit(provider.Result)
}
func loadContract(root string) (testpolicy.Contract, bool, error) {
	path := filepath.Join(root, "metasystem", "testing.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return testpolicy.Contract{SchemaVersion: testpolicy.SchemaVersion, Groups: []testpolicy.Group{}}, true, nil
	}
	if err != nil {
		return testpolicy.Contract{}, false, fmt.Errorf("read %s: %w", path, err)
	}
	contract, err := testpolicy.Parse(data)
	if err != nil {
		return testpolicy.Contract{}, false, err
	}
	if contract.TailoringRequired {
		err = contract.Validate()
		if err != nil && strings.Contains(err.Error(), "TEST_CONTRACT_REQUIRED:") {
			err = contract.ValidateExecutableGroups()
		}
	} else {
		err = contract.Validate()
	}
	if err != nil {
		return testpolicy.Contract{}, false, err
	}
	if contract.Impacted != nil {
		err = testpolicy.ValidateImpactedCWD(root, contract.Impacted.CWD)
	}
	if err != nil {
		return testpolicy.Contract{}, false, err
	}
	return contract, false, nil
}
func fail(envelope Envelope, code string, err error) Envelope {
	envelope.Error = &Failure{Code: code, Message: err.Error()}
	return envelope
}
func resultExit(result Result) int {
	if result.Status == StatusFailed || result.Status == StatusIncomplete {
		return 1
	}
	return 0
}
func announceResult(stderr io.Writer, result Result) {
	if result.Status == StatusNothingSelected {
		fmt.Fprintln(stderr, "IMPACTED TESTS: NOTHING SELECTED")
	}
	if result.Status == StatusIncomplete {
		fmt.Fprintln(stderr, "IMPACTED TESTS: INCOMPLETE")
	}
}
func WriteEnvelope(writer io.Writer, envelope Envelope) error {
	return json.NewEncoder(writer).Encode(envelope)
}
func WriteSummary(writer io.Writer, envelope Envelope) {
	if envelope.Error != nil {
		fmt.Fprintf(writer, "IMPACTED TESTS ERROR code=%s message=%s\n", envelope.Error.Code, envelope.Error.Message)
		return
	}
	request, result := envelope.Request, envelope.Result
	fmt.Fprintf(writer, "IMPACTED TESTS mode=%s base=%s binding=%s fallback=%t reason=%s status=%s\n",
		request.Mode, request.Base, request.Binding, envelope.Fallback, envelope.FallbackReason, result.Status)
	for _, selection := range result.Selections {
		fmt.Fprintf(writer, "  %s scope=%s tests=%s result=%s seconds=%.3f argv=%s\n", selection.Label, selection.Scope,
			strings.Join(selection.Tests, ","), selection.Result, selection.Seconds, strings.Join(selection.Argv, " "))
		for _, reason := range selection.Reasons {
			fmt.Fprintf(writer, "    reason %s: %s\n", reason.Test, reason.Because)
		}
	}
	for _, uncertainty := range result.Uncertainty {
		fmt.Fprintf(writer, "  uncertainty paths=%s because=%s action=%s\n", strings.Join(uncertainty.Paths, ","), uncertainty.Because, uncertainty.Action)
	}
}
