package proofrun

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const rerunCap = 5

func rerunFailedTests(ctx context.Context, request TestRunRequest, group testpolicy.Group, cwd string, environment []string,
	limits supervisorLimits, sampleInterval time.Duration, result *GroupResult,
) {
	if group.Adapter != "go" || result.Status != "failed" {
		return
	}
	failed := make([]NativeTestIdentity, 0, len(result.Observed))
	for _, identity := range result.Observed {
		if identity.Status == "failed" {
			failed = append(failed, identity)
		}
	}
	if len(failed) == 0 {
		return
	}
	sort.Slice(failed, func(i, j int) bool {
		if failed[i].Classname == failed[j].Classname {
			return failed[i].Name < failed[j].Name
		}
		return failed[i].Classname < failed[j].Classname
	})
	failedLoad := sampleLoad(request.ControlRoot, request.AttemptID, int64(os.Getpid()), time.Now().UTC(), request.loadOptions...)

	for index, identity := range failed {
		if index == rerunCap {
			result.RerunNote = fmt.Sprintf("cap %d: %d failed tests not rerun", rerunCap, len(failed)-index)
			return
		}
		if ctx.Err() != nil {
			result.RerunNote = fmt.Sprintf("context done: %d not rerun", len(failed)-index)
			return
		}
		finding := runFailedTestAgain(ctx, request, group, cwd, environment, limits, sampleInterval, identity, index+1, failedLoad)
		result.Reruns = append(result.Reruns, finding)
		if finding.Second == "passed" {
			result.NotRunReason += fmt.Sprintf("; fail-then-pass under rerun: %s.%s (failed run load: %s)",
				finding.Package, finding.Test, finding.FailedLoad.Describe())
		}
	}
}

func runFailedTestAgain(ctx context.Context, request TestRunRequest, group testpolicy.Group, cwd string, environment []string,
	limits supervisorLimits, sampleInterval time.Duration, identity NativeTestIdentity, number int, failedLoad LoadSample,
) RerunFinding {
	logPath := filepath.Join(request.LogRoot, fmt.Sprintf("%s.rerun-%d.log", group.ID, number))
	argv := []string{"go", "test", "-json", "-count=1", "-timeout", "0"}
	if len(group.BuildTags) != 0 {
		argv = append(argv, "-tags", strings.Join(group.BuildTags, ","))
	}
	if group.Race {
		argv = append(argv, "-race")
	}
	argv = append(argv, "-run", "^"+regexp.QuoteMeta(identity.Name)+"$", identity.Classname)

	var output synchronizedBuffer
	second := "missing-terminal"
	command, commandErr := explicitEnvironmentCommand(ctx, cwd, environment, argv)
	logFile, logErr := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if commandErr == nil && logErr == nil {
		activity := newOutputActivity(time.Now())
		tee := &activityWriter{activity: activity, writer: io.MultiWriter(&output, logFile)}
		command.Stdout, command.Stderr = tee, tee
		outcome := superviseCommand(command, supervisorOptions{Context: ctx, Limits: limits, SampleInterval: sampleInterval, Activity: activity})
		closeErr := logFile.Close()
		if outcome.Verdict != "" {
			second = outcome.Verdict
		} else if closeErr == nil {
			observed, _, _, _ := parseGoJSON(output.Bytes(), []NativeTestIdentity{{Classname: identity.Classname, Name: identity.Name, Status: "expected"}})
			for _, rerun := range observed {
				if rerun.Classname == identity.Classname && rerun.Name == identity.Name && (rerun.Status == "passed" || rerun.Status == "failed") {
					second = rerun.Status
					break
				}
			}
		}
	} else {
		if logFile != nil {
			_ = logFile.Close()
		}
	}

	ended := time.Now().UTC()
	return RerunFinding{Package: identity.Classname, Test: identity.Name, First: "failed", Second: second,
		FailedLoad: failedLoad, RerunLoad: sampleLoad(request.ControlRoot, request.AttemptID, int64(os.Getpid()), ended, request.loadOptions...),
		LogPath: logPath, At: ended.Format(time.RFC3339Nano)}
}
