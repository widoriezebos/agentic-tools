package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
)

type surfaceDigestReport struct {
	PolicyVersion int                        `json:"policyVersion"`
	Projection    behaviorsurface.Projection `json:"projection"`
	Endpoint      string                     `json:"endpoint"`
	Digest        string                     `json:"surfaceDigest"`
}

func writeBehaviorSurfaceJSON(value any) error {
	return json.NewEncoder(os.Stdout).Encode(value)
}

func runBehaviorSurfacePolicy(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem behavior-surface policy")
		return 2
	}
	if _, err := behaviorsurface.Load(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := os.Stdout.Write(behaviorsurface.Bytes()); err != nil {
		fmt.Fprintln(os.Stderr, "behavior-surface output:", err)
		return 1
	}
	return 0
}

func runBehaviorSurfaceClassify(args []string) int {
	flags := flag.NewFlagSet("behavior-surface classify", flag.ContinueOnError)
	path := flags.String("path", "", "Git-toplevel-relative path")
	prefix := flags.String("prefix", "", "metasystem prefix relative to the Git toplevel")
	projectionName := flags.String("projection", "", "optional projection membership to report")
	if flags.Parse(args) != nil || *path == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem behavior-surface classify --path PATH [--prefix PREFIX] [--projection ENGINE|LANDING|PAYLOAD]")
		return 2
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	class, err := policy.Classify(*path, *prefix)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	response := map[string]any{"path": *path, "class": class, "policyVersion": policy.Version}
	if *projectionName != "" {
		projection, err := behaviorsurface.ParseProjection(*projectionName)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		included, err := policy.Includes(projection, *path, *prefix)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		response["projection"] = projection
		response["included"] = included
	}
	if err := writeBehaviorSurfaceJSON(response); err != nil {
		fmt.Fprintln(os.Stderr, "behavior-surface output:", err)
		return 1
	}
	return 0
}

func runBehaviorSurfaceSelect(args []string) int {
	flags := flag.NewFlagSet("behavior-surface select", flag.ContinueOnError)
	projectionName := flags.String("projection", "", "ENGINE, LANDING, or PAYLOAD")
	prefix := flags.String("prefix", "", "metasystem prefix relative to the Git toplevel")
	nul := flags.Bool("nul", false, "read and write NUL-terminated paths")
	if flags.Parse(args) != nil || *projectionName == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem behavior-surface select --projection ENGINE|LANDING|PAYLOAD [--prefix PREFIX] [--nul]")
		return 2
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	projection, err := behaviorsurface.ParseProjection(*projectionName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	separator := byte('\n')
	if *nul {
		separator = 0
	}
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	for {
		value, readErr := reader.ReadString(separator)
		if len(value) > 0 {
			if value[len(value)-1] == separator {
				value = value[:len(value)-1]
			}
			included, err := policy.Includes(projection, value, *prefix)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			if included {
				if _, err := writer.WriteString(value); err != nil {
					fmt.Fprintln(os.Stderr, "behavior-surface output:", err)
					return 1
				}
				if err := writer.WriteByte(separator); err != nil {
					fmt.Fprintln(os.Stderr, "behavior-surface output:", err)
					return 1
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			fmt.Fprintln(os.Stderr, readErr)
			return 1
		}
	}
	if err := writer.Flush(); err != nil {
		fmt.Fprintln(os.Stderr, "behavior-surface output:", err)
		return 1
	}
	return 0
}

func runBehaviorSurfaceDigest(args []string) int {
	flags := flag.NewFlagSet("behavior-surface digest", flag.ContinueOnError)
	root := flags.String("root", "", "root whose bytes are projected")
	projectionName := flags.String("projection", "", "ENGINE, LANDING, or PAYLOAD")
	endpoint := flags.String("endpoint", "", "human-readable endpoint identity")
	prefix := flags.String("prefix", "", "optional metasystem prefix below --root")
	pathsFrom := flags.String("paths-from", "", "optional NUL-delimited source projection manifest")
	if flags.Parse(args) != nil || *root == "" || *projectionName == "" || *endpoint == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem behavior-surface digest --root DIR --projection ENGINE|LANDING|PAYLOAD --endpoint NAME [--prefix PREFIX] [--paths-from NUL-MANIFEST]")
		return 2
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	projection, err := behaviorsurface.ParseProjection(*projectionName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	var digest string
	if *pathsFrom == "" {
		digest, err = policy.DigestWithPrefix(*root, projection, *prefix)
	} else {
		data, readErr := os.ReadFile(*pathsFrom)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, readErr)
			return 1
		}
		if len(data) > 0 && data[len(data)-1] != 0 {
			fmt.Fprintln(os.Stderr, "behavior-surface path manifest is not NUL-terminated")
			return 1
		}
		parts := bytes.Split(data, []byte{0})
		if len(parts) > 0 && len(parts[len(parts)-1]) == 0 {
			parts = parts[:len(parts)-1]
		}
		paths := make([]string, len(parts))
		for index := range parts {
			paths[index] = string(parts[index])
		}
		digest, err = policy.DigestListed(*root, projection, paths)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := writeBehaviorSurfaceJSON(surfaceDigestReport{PolicyVersion: policy.Version, Projection: projection, Endpoint: *endpoint, Digest: digest}); err != nil {
		fmt.Fprintln(os.Stderr, "behavior-surface output:", err)
		return 1
	}
	return 0
}

func runBehaviorSurfaceList(args []string) int {
	flags := flag.NewFlagSet("behavior-surface list", flag.ContinueOnError)
	root := flags.String("root", "", "root whose existing paths are projected")
	projectionName := flags.String("projection", "", "ENGINE, LANDING, or PAYLOAD")
	nul := flags.Bool("nul", false, "write NUL-terminated paths")
	if flags.Parse(args) != nil || *root == "" || *projectionName == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem behavior-surface list --root DIR --projection ENGINE|LANDING|PAYLOAD [--nul]")
		return 2
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	projection, err := behaviorsurface.ParseProjection(*projectionName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	paths, err := policy.ListPaths(*root, projection)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	separator := byte('\n')
	if *nul {
		separator = 0
	}
	writer := bufio.NewWriter(os.Stdout)
	for _, path := range paths {
		if _, err := writer.WriteString(path); err != nil {
			fmt.Fprintln(os.Stderr, "behavior-surface output:", err)
			return 1
		}
		if err := writer.WriteByte(separator); err != nil {
			fmt.Fprintln(os.Stderr, "behavior-surface output:", err)
			return 1
		}
	}
	if err := writer.Flush(); err != nil {
		fmt.Fprintln(os.Stderr, "behavior-surface output:", err)
		return 1
	}
	return 0
}

func runBehaviorSurfaceSkipAllowed(args []string) int {
	flags := flag.NewFlagSet("behavior-surface skip-allowed", flag.ContinueOnError)
	family := flags.String("family", "", "validation family name")
	if flags.Parse(args) != nil || *family == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem behavior-surface skip-allowed --family NAME")
		return 2
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !policy.SkipAllowed(*family) {
		fmt.Fprintf(os.Stderr, "behavior-surface policy version %d does not authorize skip family %q\n", policy.Version, *family)
		return 3
	}
	return 0
}
