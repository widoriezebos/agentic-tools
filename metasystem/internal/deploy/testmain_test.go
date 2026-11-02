package deploy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// runnerChildEnv makes this test binary a runner in a process of its own,
// configured by the JSON the variable holds, so a test can end the process
// in the middle of a run as a crash would.
const runnerChildEnv = "DEPLOY_TEST_RUNNER_CHILD"

type childConfig struct {
	Dir  string   `json:"dir"`
	Argv []string `json:"argv"`
	Tip  string   `json:"tip"`
	// Act is run or rollback.
	Act string `json:"act"`
	// Gated, when set, is an operation whose adapter request the runner
	// writes only after it read a line from the FIFO Gate, so a test decides
	// when the request reaches an adapter whose process has started.
	Gated string `json:"gated,omitempty"`
	Gate  string `json:"gate,omitempty"`
}

func TestMain(m *testing.M) {
	if config := os.Getenv(runnerChildEnv); config != "" {
		os.Exit(runRunnerChild(config))
	}
	code := testenv.Main(m)
	if fixtureDir != "" {
		_ = os.RemoveAll(fixtureDir)
	}
	os.Exit(code)
}

func runRunnerChild(raw string) int {
	var config childConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		fmt.Println(err)
		return 1
	}
	at := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	runner := &Runner{Dir: config.Dir, Project: "fixture", Installation: ".",
		Git: Git{FetchMain: func() (string, error) { return config.Tip, nil }, AddTree: fakeTree(config.Argv), RemoveTree: os.RemoveAll},
		By:  "child", Now: func() time.Time { return at }}
	if config.Gated != "" {
		runner.Transport = func(command *exec.Cmd) (Process, error) {
			if command.Args[len(command.Args)-1] == config.Gated {
				command.Stdin = io.MultiReader(gate{path: config.Gate}, command.Stdin)
			}
			return startProcess(command)
		}
	}
	var err error
	if config.Act == "rollback" {
		_, err = runner.Rollback()
	} else {
		_, err = runner.Run()
	}
	if err != nil {
		fmt.Println(err)
		return 1
	}
	return 0
}

// gate is a reader that yields nothing, after reading one line from the FIFO
// at path.
type gate struct{ path string }

func (g gate) Read([]byte) (int, error) {
	file, err := os.Open(g.path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	if _, err := bufio.NewReader(file).ReadString('\n'); err != nil {
		return 0, err
	}
	return 0, io.EOF
}

// fakeTree stands in for a detached worktree: a directory naming its
// commit, which the fixture adapter's build requires, whose installation is
// its root and whose deploy.json names argv.
func fakeTree(argv []string) func(dir, commit string) error {
	return func(dir, commit string) error {
		contract, err := json.Marshal(Contract{Schema: 1, Adapter: Adapter{Argv: argv}})
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		for name, data := range map[string][]byte{"COMMIT": []byte(commit + "\n"), "metasystem.conf": nil, "deploy.json": contract} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
				return err
			}
		}
		return nil
	}
}
