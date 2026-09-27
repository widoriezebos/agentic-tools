package supervisor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes/external"
)

// ExternalOutputFile is the round file an external runtime's CLI stdout is
// captured in; the output-stream operation translates it into events.
const ExternalOutputFile = "runtime-output.jsonl"

// externalRuntime runs the runtime layer through an adapter executable
// (design 3.5): `<executable> OPERATION`, JSON request on stdin, JSON
// response on stdout. docs/agent-adapters.md is the operation reference.
type externalRuntime struct{ a external.Adapter }

func (e externalRuntime) Name() string { return e.a.Name }

func (e externalRuntime) call(operation string, request map[string]any, into any) error {
	response, err := e.a.Call(operation, request, nil)
	if err != nil {
		return err
	}
	return external.Decode(response, into)
}

func (e externalRuntime) Signature(Deps) (string, error) { return e.a.SignatureText() }

func (e externalRuntime) ConfigIdentity(d Deps) (string, error) {
	response, err := e.a.Call("config-identity", map[string]any{"root": d.Root}, nil)
	if err != nil {
		return "", err
	}
	var identity map[string]any
	if err := external.Decode(response, &identity); err != nil {
		return "", err
	}
	delete(identity, "schemaVersion")
	for _, member := range []string{"cliVersion", "configHash", "configKeyHashes"} {
		if _, ok := identity[member]; !ok {
			return "", fmt.Errorf("external adapter %s config-identity lacks %s", e.a.Name, member)
		}
	}
	identity["runtime"] = e.a.Name
	return canonicalObject(identity)
}

func (e externalRuntime) LocalConfigPaths(Deps) ([]string, error) {
	var response struct {
		Paths []string `json:"paths"`
	}
	if err := e.call("local-config-paths", nil, &response); err != nil {
		return nil, err
	}
	for _, path := range response.Paths {
		if path == "" || filepath.IsAbs(path) || strings.Contains(path, "..") {
			return nil, fmt.Errorf("external adapter %s declares an unsafe local config path %q", e.a.Name, path)
		}
	}
	return response.Paths, nil
}

func (e externalRuntime) EnforcementMap(Deps) (string, bool, error) {
	var response map[string]any
	if err := e.call("enforcement-map", nil, &response); err != nil {
		return "", false, err
	}
	parts := make([]string, 0, len(runtimes.EnforcementFields))
	for _, field := range runtimes.EnforcementFields {
		value, _ := response[field].(string)
		if value != string(runtimes.Mapped) && value != string(runtimes.NotEnforced) {
			return "", false, fmt.Errorf("external adapter %s enforcement-map %s=%v is outside mapped|notEnforced", e.a.Name, field, response[field])
		}
		parts = append(parts, fmt.Sprintf("%q:%q", field, value))
	}
	return "{" + strings.Join(parts, ",") + "}", true, nil
}

// snapshotFacts are the probe and contract responses: what the shared layer
// writes into a capability snapshot through the real construction path.
type snapshotFacts struct {
	CLIVersion          string          `json:"cliVersion"`
	ConfigHash          string          `json:"configHash"`
	ConfigKeyHashes     json.RawMessage `json:"configKeyHashes"`
	Transports          json.RawMessage `json:"transports"`
	Capabilities        json.RawMessage `json:"capabilities"`
	Permissions         json.RawMessage `json:"permissions"`
	EnvelopeEnforcement json.RawMessage `json:"envelopeEnforcement"`
}

func rawOr(value json.RawMessage, fallback string) string {
	if len(bytes.TrimSpace(value)) == 0 {
		return fallback
	}
	return string(value)
}

func (e externalRuntime) Contract(Deps) ([]byte, error) {
	var facts snapshotFacts
	if err := e.call("contract", nil, &facts); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "metasystem-contract.")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path, err := adapter.WriteCapabilitySnapshot(dir, e.a.Name, "0.0.0-contract", "contract0",
		rawOr(facts.Transports, "[]"), rawOr(facts.Capabilities, `{"sessionEstablishedTimeoutSec":1}`),
		rawOr(facts.Permissions, `{"unverified":[]}`), string(facts.EnvelopeEnforcement), "{}")
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (e externalRuntime) probe(d Deps, args []string) (int, error) {
	var facts snapshotFacts
	if err := e.call("probe", map[string]any{"root": d.Root, "args": args}, &facts); err != nil {
		return 1, err
	}
	if err := d.writeCapabilitySnapshot(e.a.Name, facts.CLIVersion, facts.ConfigHash,
		rawOr(facts.Transports, "[]"), string(facts.Capabilities), rawOr(facts.Permissions, `{"unverified":[]}`),
		string(facts.EnvelopeEnforcement), rawOr(facts.ConfigKeyHashes, "{}")); err != nil {
		return 1, err
	}
	return 0, nil
}

func (e externalRuntime) Probe(d Deps, args []string) int {
	code, err := e.probe(d, args)
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
	}
	return code
}

func (e externalRuntime) OutputStream(_ Deps, roundDir string) (string, error) {
	return roundDir + "/" + ExternalOutputFile, nil
}

// declaresCommand asks an executable whether it runs the round itself: the
// command operation with {"declare":true}. Exit 64 is no.
func (e externalRuntime) declaresCommand() (bool, error) {
	_, err := e.a.Call("command", map[string]any{"declare": true}, nil)
	if delegated(err) {
		return false, nil
	}
	return err == nil, err
}

func (e externalRuntime) Supervise(s *Supervision, args []string) int {
	return superviseExternal(s, args, e, "")
}

func (e externalRuntime) cancelHook(d Deps, job string) (bool, error) {
	_, err := e.a.Call("cancel", map[string]any{"root": d.Root, "job": job}, nil)
	return err == nil, err
}

// Cancel runs the runtime's own cancellation, then the shared one.
func (e externalRuntime) Cancel(d Deps, job string) int {
	if _, err := e.cancelHook(d, job); err != nil && !delegated(err) {
		fmt.Fprintln(d.Stderr, err)
	}
	return d.Dispatch.Run(d.Stdout, d.Stderr, "__cancel-owned", "--job", job)
}

func (e externalRuntime) WaitDelivery(_ Deps, waitID, nonce, deadline, session string) (bool, error) {
	if !WaitDeliveryAccepted(waitID, nonce, deadline, session) {
		return false, nil
	}
	var response struct {
		Answer string `json:"answer"`
	}
	if err := e.call("wait-delivery", map[string]any{"waitId": waitID, "nonce": nonce, "deadline": deadline, "session": session}, &response); err != nil {
		return false, err
	}
	return response.Answer == "blocking", nil
}

func (e externalRuntime) Selftest(d Deps) int {
	fmt.Fprintf(d.Stderr, "external runtime %s has no built-in self-test; drive it with a delegate round\n", e.a.Name)
	return 2
}

func (e externalRuntime) Usage(d Deps) {
	writeUsage(d.Stderr, usageLines(e.a.Name, "probe --root ROOT", true))
}

func canonicalObject(object map[string]any) (string, error) {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteByte('{')
	for index, key := range keys {
		if index > 0 {
			buf.WriteByte(',')
		}
		name, _ := json.Marshal(key)
		value, err := json.Marshal(object[key])
		if err != nil {
			return "", err
		}
		buf.Write(name)
		buf.WriteByte(':')
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.String(), nil
}

// commandResponse is the command operation's answer: how to start one turn.
type commandResponse struct {
	Argv  []string          `json:"argv"`
	Env   map[string]string `json:"env"`
	Stdin string            `json:"stdin"` // "prompt" or "none"
}

// streamEvent is one output-stream event line.
type streamEvent struct {
	Type      string          `json:"type"` // session, model, usage, result, violation
	Session   string          `json:"session"`
	Turn      string          `json:"turn"`
	Model     string          `json:"model"`
	Usage     json.RawMessage `json:"usage"`
	Candidate *string         `json:"candidate"`
	Detail    string          `json:"detail"`
}

// streamFacts folds the events of one translation.
type streamFacts struct {
	session, turn, model string
	usage                json.RawMessage
	candidate            *string
	violation            string
}

// translate runs the output-stream operation over the CLI output so far.
// builtinName is the overridden built-in, which offers no Go translation of
// its own; a delegated translation is therefore an error.
func translate(e externalRuntime, outputPath, builtinName string) (streamFacts, error) {
	output, err := os.ReadFile(outputPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return streamFacts{}, err
	}
	response, err := e.a.Call("output-stream", nil, bytes.NewReader(output))
	if delegated(err) {
		return streamFacts{}, fmt.Errorf("built-in %s offers no output-stream translation to compose with an external command", builtinName)
	}
	if err != nil {
		return streamFacts{}, err
	}
	var facts streamFacts
	for _, line := range strings.Split(string(response), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event streamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return streamFacts{}, fmt.Errorf("external adapter %s output-stream event is not JSON: %w", e.a.Name, err)
		}
		switch event.Type {
		case "session":
			if facts.session == "" {
				facts.session, facts.turn = event.Session, event.Turn
				if event.Model != "" {
					facts.model = event.Model
				}
			}
		case "model":
			facts.model = event.Model
		case "usage":
			facts.usage = event.Usage
		case "result":
			facts.candidate = event.Candidate
			if event.Model != "" {
				facts.model = event.Model
			}
		case "violation":
			facts.violation = event.Detail
		}
	}
	return facts, nil
}

// superviseExternal is one round of a runtime whose command and
// output-stream operations are an executable's: the shared lifecycle with
// the runtime layer called out.
func superviseExternal(s *Supervision, args []string, e externalRuntime, builtinName string) int {
	if !s.prepareOrUsage(args) {
		return 2
	}
	usageFile := filepath.Join(s.roundDir, "usage.json")
	outputPath := filepath.Join(s.roundDir, ExternalOutputFile)
	if err := s.recordWorkspaceWriteScope(); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	if !s.failIfEffectiveWider() {
		return 1
	}
	if err := truncate(s.events); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	var command commandResponse
	if err := e.call("command", map[string]any{
		"root": s.d.Root, "verb": s.verb, "job": s.job, "record": s.record, "roundDir": s.roundDir,
		"workspace": s.workspace, "prompt": s.prompt, "schema": s.schema, "model": s.requestedModel,
		"session": s.requestedSession, "instanceTag": s.tag, "effectivePermissions": s.effective,
	}, &command); err != nil || len(command.Argv) == 0 {
		if err == nil {
			err = fmt.Errorf("external adapter %s command returned no argv", e.a.Name)
		}
		s.logf("%v\n", err)
		s.failPending("runtime_error", "handshake", "")
		return 1
	}
	if !s.verifyReferences() {
		return 1
	}
	if err := s.markPrefork(); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		s.failPending("prefork_marker", "handshake", "")
		return 1
	}
	env := withEnv(s.childEnv, jobGitQuarantineEnv(s.d.git(), s.workspace)...)
	names := make([]string, 0, len(command.Env))
	for name := range command.Env {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		env = withEnv(env, name+"="+command.Env[name])
	}
	stdin := ""
	if command.Stdin != "none" {
		stdin = s.prompt
	}
	cli, err := s.launch(command.Argv, env, stdin, outputPath, nil)
	if err != nil {
		s.failPending("custody_registration", "handshake", "")
		return 1
	}
	if err := s.registerCustody(cli); err != nil {
		cli.terminate()
		s.failPending("custody_registration", "handshake", "")
		return 1
	}
	for cli.alive() {
		facts, err := translate(e, outputPath, builtinName)
		if err != nil {
			s.logf("%v\n", err)
			cli.terminate()
			s.failPending("runtime_error", "handshake", "")
			return 1
		}
		if facts.session != "" {
			if !s.recordHandshake(facts.session, facts.turn, facts.model) {
				cli.terminate()
				return 1
			}
			break
		}
		touch(s.heartbeat)
		s.d.Clock.Sleep(pollTick)
	}
	status, err := s.waitForCLI(cli)
	if err != nil {
		return exitCodeOf(err, 1)
	}
	facts, err := translate(e, outputPath, builtinName)
	if err != nil {
		s.logf("%v\n", err)
		facts = streamFacts{}
	}
	if len(bytes.TrimSpace(facts.usage)) > 0 {
		if err := os.WriteFile(usageFile, append(bytes.TrimSpace(facts.usage), '\n'), 0o644); err != nil {
			fmt.Fprintln(s.d.Stderr, err)
			return 1
		}
	} else if err := adapter.WriteUnavailableUsage(usageFile); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	if facts.violation != "" {
		s.logf("%s\n", facts.violation)
	}
	candidate := filepath.Join(s.roundDir, "runtime-reply.txt")
	reply := ""
	if facts.candidate != nil {
		reply = *facts.candidate
	}
	if err := os.WriteFile(candidate, []byte(reply), 0o644); err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	if data, err := os.ReadFile(outputPath); err == nil {
		_ = os.WriteFile(s.raw, data, 0o644)
	}
	if !s.settleResultIdentity(facts.session, facts.turn, s.requestedModel, facts.model, usageFile) {
		return 1
	}
	return terminal(s.completeFromCLI(status, usageFile, candidate, "", repairHooks{}))
}
