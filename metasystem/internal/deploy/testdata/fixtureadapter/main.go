// Command fixtureadapter is the deploy tests' adapter: a real executable
// called as `fixtureadapter STATE OPERATION` with the request on standard
// input. It keeps what is active in STATE/active.json, builds artifacts as
// files under STATE/artifacts, logs each call to STATE/calls (and, with the
// name it was called by and the commit of the tree its source names, to
// STATE/via), and answers each operation as the test scripted it in
// STATE/script/OPERATION.json: a list of steps, one per call, the last
// repeating. Called by a name that begins with "broken", it fails every call.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type request struct {
	Operation string `json:"operation"`
	Commit    string `json:"commit"`
	Source    string `json:"source"`
	Artifact  string `json:"artifact"`
	Previous  *struct {
		Version  string `json:"version"`
		Artifact string `json:"artifact"`
		Digest   string `json:"digest"`
	} `json:"previous"`
}

type deployed struct {
	Version  string `json:"version"`
	Artifact string `json:"artifact"`
	Digest   string `json:"digest"`
}

// step is one scripted call. HoldBefore and HoldAfter name a FIFO the call
// opens and reads to its end before, or after, it does its work: the test
// releases it by opening the FIFO for writing and closing it. Exit other
// than 0 skips the work unless Apply is set. Raw replaces the response.
type step struct {
	Exit       int    `json:"exit"`
	Apply      bool   `json:"apply"`
	Reason     string `json:"reason"`
	Raw        string `json:"raw"`
	Artifact   string `json:"artifact"`
	HoldBefore string `json:"holdBefore"`
	HoldAfter  string `json:"holdAfter"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: fixtureadapter STATE OPERATION")
		os.Exit(2)
	}
	state, operation := os.Args[1], os.Args[2]
	var req request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fail(err)
	}
	if req.Operation != operation {
		fail(fmt.Errorf("the request names %q, the argument %q", req.Operation, operation))
	}
	appendLine(filepath.Join(state, "calls"), operation+" "+req.Commit)
	tree, _ := os.ReadFile(filepath.Join(req.Source, "COMMIT"))
	appendLine(filepath.Join(state, "via"), filepath.Base(os.Args[0])+" "+operation+" "+req.Commit+" in "+strings.TrimSpace(string(tree)))
	if strings.HasPrefix(filepath.Base(os.Args[0]), "broken") {
		writeJSON(os.Stdout, map[string]string{"outcome": "failed", "reason": "this adapter fails every call"})
		os.Exit(1)
	}
	current := next(state, operation)
	hold(current.HoldBefore, operation+" "+req.Commit)
	fmt.Fprintf(os.Stderr, "%s of %s in %s\n", operation, req.Commit, req.Source)
	var response any
	if current.Exit == 0 || current.Apply {
		response = work(state, req)
	}
	hold(current.HoldAfter, operation+" "+req.Commit)
	switch {
	case current.Raw != "":
		fmt.Print(current.Raw)
	case current.Exit == 1:
		writeJSON(os.Stdout, map[string]string{"outcome": "failed", "reason": current.Reason})
	case current.Exit == 0:
		if current.Artifact != "" {
			answer := response.(map[string]string)
			answer["artifact"] = current.Artifact
		}
		writeJSON(os.Stdout, response)
	}
	os.Exit(current.Exit)
}

func work(state string, req request) any {
	activePath := filepath.Join(state, "active.json")
	switch req.Operation {
	case "build":
		if _, err := os.Stat(filepath.Join(req.Source, "COMMIT")); err != nil {
			fail(fmt.Errorf("the clean tree is missing: %w", err))
		}
		artifact := filepath.Join(state, "artifacts", req.Commit)
		if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
			fail(err)
		}
		if err := os.WriteFile(artifact, []byte("artifact of "+req.Commit+"\n"), 0o644); err != nil {
			fail(err)
		}
		return map[string]string{"outcome": "built", "version": "v-" + req.Commit, "artifact": artifact, "digest": digest(artifact)}
	case "activate":
		version := "v-" + filepath.Base(req.Artifact)
		save(activePath, deployed{Version: version, Artifact: req.Artifact, Digest: digest(req.Artifact)})
		return map[string]string{"outcome": "active", "version": version}
	case "rollback":
		save(activePath, deployed{Version: req.Previous.Version, Artifact: req.Previous.Artifact, Digest: req.Previous.Digest})
		return map[string]string{"outcome": "active", "version": req.Previous.Version}
	case "version":
		data, err := os.ReadFile(activePath)
		if os.IsNotExist(err) {
			return map[string]string{"outcome": "none"}
		}
		var active deployed
		if err != nil || json.Unmarshal(data, &active) != nil {
			fail(fmt.Errorf("active.json is unreadable: %v", err))
		}
		return map[string]string{"outcome": "active", "version": active.Version, "artifact": active.Artifact, "digest": active.Digest}
	}
	fail(fmt.Errorf("unknown operation %q", req.Operation))
	return nil
}

// next is this call's step: the script's entry at the count of earlier
// calls of the operation, the last entry once the script runs out.
func next(state, operation string) step {
	script := filepath.Join(state, "script", operation+".json")
	data, err := os.ReadFile(script)
	if os.IsNotExist(err) {
		return step{}
	}
	var steps []step
	if err != nil || json.Unmarshal(data, &steps) != nil || len(steps) == 0 {
		fail(fmt.Errorf("the script %s is unreadable: %v", script, err))
	}
	countPath := filepath.Join(state, "script", operation+".count")
	count := 0
	if raw, err := os.ReadFile(countPath); err == nil {
		count, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
	}
	if err := os.WriteFile(countPath, []byte(strconv.Itoa(count+1)), 0o644); err != nil {
		fail(err)
	}
	return steps[min(count, len(steps)-1)]
}

func hold(fifo, call string) {
	if fifo == "" {
		return
	}
	// A pause ends only a call that has taken its scripted step. Mark
	// that call before it waits on the FIFO.
	if err := os.WriteFile(fifo+".held", []byte(call), 0o600); err != nil {
		fail(err)
	}
	defer os.Remove(fifo + ".held")
	file, err := os.Open(fifo)
	if err != nil {
		fail(err)
	}
	_, _ = io.Copy(io.Discard, file)
	_ = file.Close()
}

func digest(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func save(path string, value deployed) {
	file, err := os.Create(path)
	if err != nil {
		fail(err)
	}
	writeJSON(file, value)
	if err := file.Close(); err != nil {
		fail(err)
	}
}

func appendLine(path, line string) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fail(err)
	}
	_, _ = file.WriteString(line + "\n")
	_ = file.Close()
}

func writeJSON(out io.Writer, value any) {
	if err := json.NewEncoder(out).Encode(value); err != nil {
		fail(err)
	}
}

// fail is a broken fixture: exit 70 with no response, which the runner
// reads as an unknown state, and the test sees the reason in the log.
func fail(err error) {
	fmt.Fprintln(os.Stderr, "fixtureadapter:", err)
	os.Exit(70)
}
