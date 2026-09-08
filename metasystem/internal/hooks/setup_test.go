package hooks

import (
	"encoding/json"
	"strings"
	"testing"
)

const codexShipped = `{"hooks":{"SessionStart":[{"matcher":"startup|resume|clear|compact","hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh codex start","timeout":15}]}],"Stop":[{"hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh codex stop","timeout":60}]}],"SessionEnd":[{"hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh codex end","timeout":3}]}]}}`
const claudeShipped = `{"hooks":{"SessionStart":[{"matcher":"startup|resume|clear|compact","hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh claude start","timeout":15}]}],"Stop":[{"hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh claude receipt"},{"type":"command","command":"(bash scripts/agents/supervision-hook.sh claude stop) || printf '%s\\n' '{\"decision\":\"block\",\"reason\":\"Metasystem Stop hook launcher failed before a safe verdict; stopping is refused.\"}'","timeout":60}]}],"SessionEnd":[{"hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh claude end","timeout":3}]}]}}`
const devinShipped = `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh devin start","timeout":15}]}],"Stop":[{"hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh devin stop","timeout":60}]}],"SessionEnd":[{"hooks":[{"type":"command","command":"bash scripts/agents/supervision-hook.sh devin end","timeout":3}]}]}}`

func TestMergeSplitsOwnedHandlerFromForeignSiblingWhenMatcherChanges(t *testing.T) {
	live := []byte(`{"foreignTop":{"kept":true},"hooks":{"SessionStart":[{"matcher":"startup|resume|clear","groupField":"foreign","hooks":[{"type":"command","command":"cd \"$CLAUDE_PROJECT_DIR/metasystem\" && bash scripts/agents/supervision-hook.sh claude start","timeout":15},{"type":"command","command":"foreign-command","timeout":7}]}],"ForeignEvent":[{"matcher":"x","hooks":[{"command":"foreign-event"}]}]}}`)
	merged, err := MergeSettings(live, []byte(claudeShipped), "claude", "metasystem", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckSettings(merged, []byte(claudeShipped), "claude", "metasystem", false); err != nil {
		t.Fatalf("merged settings failed their structural check: %v", err)
	}
	text := string(merged)
	for _, fragment := range []string{`"groupField": "foreign"`, `"command": "foreign-command"`, `"matcher": "startup|resume|clear"`, `"matcher": "startup|resume|clear|compact"`, `"ForeignEvent"`, `unset GIT_DIR`, `$repo/metasystem`} {
		if !strings.Contains(text, fragment) {
			t.Errorf("merged settings lost %q: %s", fragment, text)
		}
	}
	if strings.Count(text, "supervision-hook.sh claude start") != 1 {
		t.Fatalf("owned start handler duplicated: %s", text)
	}
	again, err := MergeSettings(merged, []byte(claudeShipped), "claude", "metasystem", false)
	if err != nil || string(again) != string(merged) {
		t.Fatalf("merge is not byte-idempotent: %v", err)
	}
}

func TestMergePreservesUnrelatedJSONAndRejectsUncertainOrMalformedInput(t *testing.T) {
	foreign := []byte(`{"number":1.25,"hooks":{"Other":[{"hooks":[{"command":"foreign"}]}]}}`)
	merged, err := MergeSettings(foreign, []byte(codexShipped), "codex", ".", true)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if json.Unmarshal(merged, &value) != nil || value["number"].(float64) != 1.25 {
		t.Fatalf("unrelated JSON changed: %s", merged)
	}
	uncertain := []byte(`{"hooks":{"Stop":[{"hooks":[{"command":"env X=1 bash scripts/agents/supervision-hook.sh codex stop"}]}]}}`)
	if _, err := MergeSettings(uncertain, []byte(codexShipped), "codex", ".", true); err == nil || !strings.Contains(err.Error(), "unrecognized") {
		t.Fatalf("uncertain ownership did not conflict: %v", err)
	}
	if _, err := MergeSettings([]byte(`{"hooks":`), []byte(codexShipped), "codex", ".", true); err == nil {
		t.Fatal("malformed JSON was accepted")
	}
}

func TestCheckSettingsRejectsWrongMatcherActionRuntimeAndTimeout(t *testing.T) {
	valid, err := MergeSettings([]byte(`{}`), []byte(codexShipped), "codex", ".", true)
	if err != nil {
		t.Fatal(err)
	}
	for name, corrupt := range map[string]string{
		"matcher": strings.Replace(string(valid), "startup|resume|clear|compact", "startup|resume|compact", 1),
		"action":  strings.Replace(string(valid), "codex start", "codex end", 1),
		"runtime": strings.Replace(string(valid), "codex start", "devin start", 1),
		"timeout": strings.Replace(string(valid), `"timeout": 15`, `"timeout": 16`, 1),
		"event":   strings.Replace(string(valid), `"SessionStart"`, `"WrongEvent"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := CheckSettings([]byte(corrupt), []byte(codexShipped), "codex", ".", true); err == nil {
				t.Fatal("structural drift passed")
			}
		})
	}

	duplicate := strings.Replace(string(valid), `"hooks": [`, `"hooks": [`, 1)
	var duplicateObject map[string]any
	if err := json.Unmarshal([]byte(duplicate), &duplicateObject); err != nil {
		t.Fatal(err)
	}
	events := duplicateObject["hooks"].(map[string]any)
	startGroups := events["SessionStart"].([]any)
	startGroup := startGroups[0].(map[string]any)
	handlers := startGroup["hooks"].([]any)
	startGroup["hooks"] = append(handlers, handlers[0])
	duplicateJSON, _ := json.Marshal(duplicateObject)
	if err := CheckSettings(duplicateJSON, []byte(codexShipped), "codex", ".", true); err == nil {
		t.Fatal("duplicate owned handler passed")
	}

	unrecognized := strings.Replace(string(valid), "unset GIT_DIR", "env EXTRA=1; unset GIT_DIR", 1)
	if err := CheckSettings([]byte(unrecognized), []byte(codexShipped), "codex", ".", true); err == nil || !strings.Contains(err.Error(), "unrecognized") {
		t.Fatalf("unrecognized owned command did not produce a clear conflict: %v", err)
	}
}

func TestCheckSettingsAllowsForeignSiblingAndGroupMetadata(t *testing.T) {
	valid, err := MergeSettings([]byte(`{}`), []byte(codexShipped), "codex", "metasystem", false)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(valid, &object); err != nil {
		t.Fatal(err)
	}
	events := object["hooks"].(map[string]any)
	startGroup := events["SessionStart"].([]any)[0].(map[string]any)
	startGroup["providerMetadata"] = map[string]any{"kept": true}
	startGroup["hooks"] = append(startGroup["hooks"].([]any), map[string]any{
		"type": "command", "command": "foreign-command", "timeout": float64(7),
	})
	mixed, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckSettings(mixed, []byte(codexShipped), "codex", "metasystem", false); err != nil {
		t.Fatalf("foreign sibling and harmless group metadata failed readiness: %v", err)
	}
}

func TestCheckSettingsAcceptsEmptyDevinNonToolMatcher(t *testing.T) {
	valid, err := MergeSettings([]byte(`{}`), []byte(devinShipped), "devin", ".", true)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(valid, &object); err != nil {
		t.Fatal(err)
	}
	events := object["hooks"].(map[string]any)
	for _, rawGroups := range events {
		for _, rawGroup := range rawGroups.([]any) {
			rawGroup.(map[string]any)["matcher"] = ""
		}
	}
	withEmptyMatchers, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckSettings(withEmptyMatchers, []byte(devinShipped), "devin", ".", true); err != nil {
		t.Fatalf("empty Devin non-tool matcher failed readiness: %v", err)
	}
}

func TestClaudeMergePreservesReceiptNoticeAndFailClosedStop(t *testing.T) {
	live := []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"if ! cd \"$CLAUDE_PROJECT_DIR/metasystem\" 2>/dev/null; then echo '{\"systemMessage\":\"Harness hook could not resolve the project directory (CLAUDE_PROJECT_DIR).\"}'; else bash scripts/receipt.sh check >/dev/null 2>&1; rc=$?; if [ \"$rc\" -eq 1 ]; then echo '{\"systemMessage\":\"Harness retro due: run scripts/receipt.sh check for details, then skills/retro.\"}'; elif [ \"$rc\" -ne 0 ]; then echo '{\"systemMessage\":\"Harness receipt check errored; run scripts/receipt.sh check to see why.\"}'; fi; fi"},{"type":"command","command":"cd \"$CLAUDE_PROJECT_DIR/metasystem\" && bash scripts/agents/supervision-hook.sh claude stop","timeout":60}]}]}}`)
	merged, err := MergeSettings(live, []byte(claudeShipped), "claude", "metasystem", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckSettings(merged, []byte(claudeShipped), "claude", "metasystem", false); err != nil {
		t.Fatal(err)
	}
	text := string(merged)
	if strings.Count(text, "supervision-hook.sh claude receipt") != 1 || strings.Count(text, "supervision-hook.sh claude stop") != 1 || !strings.Contains(text, `\"decision\":\"block\"`) {
		t.Fatalf("Claude receipt or fail-closed Stop was lost: %s", text)
	}
}

func TestClaudeMergeMigratesActualLegacyAdoptedRootReceipt(t *testing.T) {
	live := []byte(`{"hooks":{"SessionStart":[{"matcher":"startup|resume|clear","hooks":[{"type":"command","command":"cd \"$CLAUDE_PROJECT_DIR\" && bash scripts/agents/supervision-hook.sh claude start","timeout":15}]}],"Stop":[{"hooks":[{"type":"command","command":"if ! cd \"$CLAUDE_PROJECT_DIR\" 2>/dev/null; then echo '{\"systemMessage\":\"Metasystem hook could not resolve the project directory (CLAUDE_PROJECT_DIR).\"}'; else bash scripts/receipt.sh check >/dev/null 2>&1; rc=$?; if [ \"$rc\" -eq 1 ]; then echo '{\"systemMessage\":\"Metasystem retro due: run scripts/receipt.sh check for details, then skills/retro.\"}'; elif [ \"$rc\" -ne 0 ]; then echo '{\"systemMessage\":\"Metasystem receipt check errored; run scripts/receipt.sh check to see why.\"}'; fi; fi"},{"type":"command","command":"(cd \"$CLAUDE_PROJECT_DIR\" && bash scripts/agents/supervision-hook.sh claude stop) || printf '%s\\n' '{\"decision\":\"block\",\"reason\":\"Metasystem Stop hook launcher failed before a safe verdict; stopping is refused.\"}'","timeout":60},{"type":"command","command":"foreign-stop-handler","foreign":true}],"groupMetadata":"kept"}],"SessionEnd":[{"hooks":[{"type":"command","command":"cd \"$CLAUDE_PROJECT_DIR\" && bash scripts/agents/supervision-hook.sh claude end","timeout":3}]}]}}`)
	merged, err := MergeSettings(live, []byte(claudeShipped), "claude", ".", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckSettings(merged, []byte(claudeShipped), "claude", ".", true); err != nil {
		t.Fatal(err)
	}
	text := string(merged)
	for _, fragment := range []string{"foreign-stop-handler", `"groupMetadata": "kept"`, `\"decision\":\"block\"`} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("legacy adopted-root migration lost %q: %s", fragment, text)
		}
	}
	if strings.Contains(text, `cd \"$CLAUDE_PROJECT_DIR\"`) || strings.Count(text, "supervision-hook.sh claude receipt") != 1 {
		t.Fatalf("legacy adopted-root handlers were not replaced exactly once: %s", text)
	}
}

func TestMergeScopesOwnedCommandsToSelectedInstallation(t *testing.T) {
	cases := []struct {
		name                       string
		selected                   string
		registrationAtInstallation bool
		legacy                     string
		other                      string
	}{
		{name: "adopted-root", selected: ".", registrationAtInstallation: true, legacy: `bash scripts/agents/supervision-hook.sh codex start`, other: `cd "$CLAUDE_PROJECT_DIR/metasystem" && bash scripts/agents/supervision-hook.sh codex start`},
		{name: "nested-template", selected: "metasystem", registrationAtInstallation: false, legacy: `cd "$CLAUDE_PROJECT_DIR/metasystem" && bash scripts/agents/supervision-hook.sh codex start`, other: `bash scripts/agents/supervision-hook.sh codex start`},
		{name: "nested-adopted", selected: "vendor/metasystem", registrationAtInstallation: true, legacy: `bash scripts/agents/supervision-hook.sh codex start`, other: `cd "$CLAUDE_PROJECT_DIR/vendor/metasystem" && bash scripts/agents/supervision-hook.sh codex start`},
	}
	for _, test := range cases {
		t.Run(test.name+"-migrates-own-command", func(t *testing.T) {
			liveObject := map[string]any{"hooks": map[string]any{"SessionStart": []any{map[string]any{
				"matcher": "startup|resume", "groupMetadata": "kept", "hooks": []any{
					map[string]any{"type": "command", "command": test.legacy, "timeout": float64(15)},
					map[string]any{"type": "command", "command": "foreign-command", "timeout": float64(7)},
				},
			}}}}
			live, _ := json.Marshal(liveObject)
			merged, err := MergeSettings(live, []byte(codexShipped), "codex", test.selected, test.registrationAtInstallation)
			if err != nil {
				t.Fatal(err)
			}
			text := string(merged)
			for _, kept := range []string{"foreign-command", `"groupMetadata": "kept"`} {
				if !strings.Contains(text, kept) {
					t.Fatalf("foreign sibling state was lost: %s", text)
				}
			}
			if !strings.Contains(text, "unset GIT_DIR") || strings.Count(text, "supervision-hook.sh codex start") != 1 {
				t.Fatalf("selected installation command was not migrated exactly once: %s", text)
			}
		})

		t.Run(test.name+"-refuses-other-installation", func(t *testing.T) {
			liveObject := map[string]any{"hooks": map[string]any{"SessionStart": []any{map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": test.other, "timeout": float64(15)},
					map[string]any{"type": "command", "command": "foreign-command"},
				},
			}}}}
			live, _ := json.Marshal(liveObject)
			if _, err := MergeSettings(live, []byte(codexShipped), "codex", test.selected, test.registrationAtInstallation); err == nil || !strings.Contains(err.Error(), "unrecognized MetaSystem handler") {
				t.Fatalf("handler targeting another installation was silently owned: %v", err)
			}
		})
	}
}

func TestMergeMigratesGitRequiredNonblockingLauncher(t *testing.T) {
	var live map[string]any
	if err := json.Unmarshal([]byte(codexShipped), &live); err != nil {
		t.Fatal(err)
	}
	events := live["hooks"].(map[string]any)
	start := events["SessionStart"].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)
	previous := renderGitRequiredCommand("codex", "start", ".", false)
	start["command"] = previous
	encoded, err := json.Marshal(live)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := MergeSettings(encoded, []byte(codexShipped), "codex", ".", true)
	if err != nil {
		t.Fatalf("previous generated launcher was not recognized for migration: %v", err)
	}
	if err := CheckSettings(merged, []byte(codexShipped), "codex", ".", true); err != nil {
		t.Fatalf("migrated launcher failed readiness: %v", err)
	}
	var ready map[string]any
	if err := json.Unmarshal(merged, &ready); err != nil {
		t.Fatal(err)
	}
	readyEvents := ready["hooks"].(map[string]any)
	command := readyEvents["SessionStart"].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"].(string)
	want := renderCommand("codex", "start", ".", false)
	if command != want || command == previous {
		t.Fatalf("launcher migration = %q, want %q and not previous form", command, want)
	}
}

func TestStopLaunchersRemainGitRequiredForEveryRuntime(t *testing.T) {
	for _, runtime := range []string{"claude", "codex", "devin"} {
		failClosed := runtime == "claude"
		got := renderCommand(runtime, "stop", "metasystem", failClosed)
		want := renderGitRequiredCommand(runtime, "stop", "metasystem", failClosed)
		if got != want || strings.Contains(got, `exit 0`) {
			t.Fatalf("%s Stop launcher gained non-Git no-op behavior: %s", runtime, got)
		}
	}
}

func TestMergePreservesShippedDefaultsAndRejectsMalformedLiveStructures(t *testing.T) {
	shippedWithDefaults := strings.Replace(codexShipped, `{"hooks":`, `{"_comment":"shipped only","provider":{"enabled":true},"hooks":`, 1)
	merged, err := MergeSettings([]byte(`{"local":true}`), []byte(shippedWithDefaults), "codex", ".", true)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(merged, &object); err != nil {
		t.Fatal(err)
	}
	provider, ok := object["provider"].(map[string]any)
	if !ok || provider["enabled"] != true || object["local"] != true {
		t.Fatalf("shipped defaults or local settings were lost: %s", merged)
	}
	if _, present := object["_comment"]; present {
		t.Fatalf("shipped-only comment was installed: %s", merged)
	}

	foreign, err := MergeSettings([]byte(`{"provider":{"owner":"foreign"}}`), []byte(shippedWithDefaults), "codex", ".", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(foreign), `"owner": "foreign"`) || strings.Contains(string(foreign), `"enabled": true`) {
		t.Fatalf("foreign top-level setting was overwritten: %s", foreign)
	}

	for name, test := range map[string]struct {
		live string
		want string
	}{
		"null top level":     {`null`, "top level is not an object"},
		"trailing value":     {`{} {}`, "trailing JSON values"},
		"hooks not object":   {`{"hooks":[]}`, "hooks is not an object"},
		"event not array":    {`{"hooks":{"SessionStart":{}}}`, "event SessionStart is not an array"},
		"group not object":   {`{"hooks":{"SessionStart":["group"]}}`, "non-object group"},
		"handlers missing":   {`{"hooks":{"SessionStart":[{}]}}`, "no handler array"},
		"handler not object": {`{"hooks":{"SessionStart":[{"hooks":["handler"]}]}}`, "non-object handler"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := MergeSettings([]byte(test.live), []byte(codexShipped), "codex", ".", true)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("malformed live structure did not produce %q: %v", test.want, err)
			}
		})
	}
}

func TestMergeRejectsMalformedShippedStructures(t *testing.T) {
	for name, test := range map[string]struct {
		shipped string
		want    string
	}{
		"malformed JSON":     {`{"hooks":`, "shipped hook configuration is invalid"},
		"null top level":     {`null`, "top level is not an object"},
		"hooks missing":      {`{}`, "has no hooks object"},
		"hooks not object":   {`{"hooks":[]}`, "has no hooks object"},
		"event not array":    {`{"hooks":{"Stop":{}}}`, "shipped event Stop is not an array"},
		"group not object":   {`{"hooks":{"Stop":["group"]}}`, "non-object group"},
		"handlers missing":   {`{"hooks":{"Stop":[{}]}}`, "has no handlers"},
		"handler not object": {`{"hooks":{"Stop":[{"hooks":["handler"]}]}}`, "non-object handler"},
		"wrong runtime":      {devinShipped, "without the expected runtime action"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := MergeSettings([]byte(`{}`), []byte(test.shipped), "codex", ".", true)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("malformed shipped structure did not produce %q: %v", test.want, err)
			}
		})
	}
}

func TestCheckSettingsRejectsMalformedLiveStructuresAndExtraOwnership(t *testing.T) {
	valid, err := MergeSettings([]byte(`{}`), []byte(codexShipped), "codex", ".", true)
	if err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		live string
		want string
	}{
		"null top level":     {`null`, "cannot read hook configuration"},
		"hooks missing":      {`{}`, "has no hooks object"},
		"event not array":    {`{"hooks":{"SessionStart":{}}}`, "event SessionStart is not an array"},
		"group not object":   {`{"hooks":{"SessionStart":["group"]}}`, "non-object group"},
		"handlers missing":   {`{"hooks":{"SessionStart":[{}]}}`, "no handler array"},
		"handler not object": {`{"hooks":{"SessionStart":[{"hooks":["handler"]}]}}`, "non-object handler"},
	} {
		t.Run(name, func(t *testing.T) {
			err := CheckSettings([]byte(test.live), []byte(codexShipped), "codex", ".", true)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("malformed live structure did not produce %q: %v", test.want, err)
			}
		})
	}

	var object map[string]any
	if err := json.Unmarshal(valid, &object); err != nil {
		t.Fatal(err)
	}
	events := object["hooks"].(map[string]any)
	startGroup := events["SessionStart"].([]any)[0].(map[string]any)
	startHandler := startGroup["hooks"].([]any)[0].(map[string]any)
	events["ForeignEvent"] = []any{map[string]any{"hooks": []any{startHandler}}}
	extraOwned, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckSettings(extraOwned, []byte(codexShipped), "codex", ".", true); err == nil || !strings.Contains(err.Error(), "owned handlers; want") {
		t.Fatalf("recognized handler under a foreign event escaped duplicate ownership detection: %v", err)
	}
}

func TestCheckSettingsEnforcesSynchronousHandlerContract(t *testing.T) {
	valid, err := MergeSettings([]byte(`{}`), []byte(codexShipped), "codex", ".", true)
	if err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		async   any
		wantErr bool
	}{
		"explicit false": {false, false},
		"true":           {true, true},
		"non-boolean":    {"false", true},
	} {
		t.Run(name, func(t *testing.T) {
			var object map[string]any
			if err := json.Unmarshal(valid, &object); err != nil {
				t.Fatal(err)
			}
			events := object["hooks"].(map[string]any)
			group := events["SessionStart"].([]any)[0].(map[string]any)
			handler := group["hooks"].([]any)[0].(map[string]any)
			handler["async"] = test.async
			encoded, err := json.Marshal(object)
			if err != nil {
				t.Fatal(err)
			}
			err = CheckSettings(encoded, []byte(codexShipped), "codex", ".", true)
			if (err != nil) != test.wantErr {
				t.Fatalf("async=%v readiness error = %v, wantErr=%v", test.async, err, test.wantErr)
			}
		})
	}
}
