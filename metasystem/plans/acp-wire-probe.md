# ACP wire probe (P1, first capture)

Captured 2026-08-16 against devin 3000.4.25 (7e8e528a), macOS.
Request: initialize, protocolVersion 1, minimal client caps.

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": 1,
    "agentCapabilities": {
      "loadSession": true,
      "promptCapabilities": {
        "image": true,
        "audio": false,
        "embeddedContext": true
      },
      "mcpCapabilities": {
        "http": false,
        "sse": false
      },
      "sessionCapabilities": {
        "list": {},
        "delete": {},
        "additionalDirectories": {}
      },
      "auth": {},
      "_meta": {
        "cognition.ai/multiRootWorkspace": true,
        "cognition.ai/sessionRename": true,
        "cognition.ai/sessionShare": true,
        "cognition.ai/documentLifecycle": true,
        "cognition.ai/userEdits": true,
        "cognition.ai/terminalLifecycle": true,
        "cognition.ai/userConfig": true,
        "cognition.ai/userShellCommand": true,
        "cognition.ai/editableCommands": true,
        "cognition.ai/commandRevision": true,
        "cognition.ai/megaplan": true,
        "cognition.ai/ruleMentions": true
      }
    },
    "authMethods": [
      {
        "id": "devin-browser",
        "name": "Log in with browser",
        "description": "Sign in via your browser"
      }
    ],
    "agentInfo": {
      "name": "affogato",
      "title": "Devin Agent",
      "version": "0.0.0-dev"
    },
    "_meta": {
      "mcpConfigPath": "/Users/wido/.config/devin/mcp_config.json"
    }
  }
}
```

Facts the design must absorb:
- protocolVersion 1 confirmed; agentInfo name=affogato.
- authMethods is non-empty (devin-browser): session establishment
  must handle an unauthenticated server — the draft assumed none.
- loadSession=true: resumable sessions exist at the protocol level.
- The cognition.ai _meta extension set (terminalLifecycle,
  userShellCommand, userConfig, megaplan...) is large and
  UNVERSIONED here — the client must treat extensions as opaque
  unless declared.

---

# P1 step A (2026-08-16, implementation-first under D81)

Full turn against devin 3000.4.25: initialize → session/new
(UNAUTHENTICATED) → one tool-free prompt → wind-down. Raw
newline-delimited JSON captured with timestamps (39 frames);
probe source: scratchpad acp-p1/main.go (throwaway).

## Facts established

1. **Unauthenticated session/new WORKS** (question 1): the CLI's
   stored login covers the ACP server. `authMethods` is
   advertised but nothing demanded auth; session `marvelous-answer`
   was established ~1.3s after initialize. Sessions get
   human-readable slug IDs and a server-assigned title
   (`session_info_update: "Pong Reply Request"`).
2. **Framing** (question 9, partial): newline-delimited JSON, no
   length headers; handshake round-trip under 100ms; the largest
   observed frame (the config-option catalog) is tens of KB on
   one line — the client's scanner ceiling must be generous.
3. **Update kinds observed** (question 3): agent_message_chunk
   (exactly one, the candidate: `{"content":{"text":"pong","type":"text"}}`),
   agent_thought_chunk (23 of them — the THOUGHT stream is
   verbose even for a pong; delivery assembly must consume only
   message chunks, exactly as the design says),
   available_commands_update, config_option_update,
   current_mode_update, session_info_update, usage_update. Plus
   extension notifications `_cognition.ai/mcp/serversChanged` and
   `_cognition.ai/agent_stopped` (cause "complete", stats:
   commandsRun, filesChanged, tokens, modelLabel "SWE-1.7 Max",
   requestId, responseDimensions…).
4. **Stop reason**: `end_turn`, with
   `_meta cognition.ai/userMessageId` on the PromptResponse.
5. **Usage** (question 5, major): BOTH candidate sources are real
   on this wire. `usage_update` carries context accounting
   (`used`/`size` = 11695/262000) plus
   `_meta cognition.ai/inputTokens|outputTokens` per turn; AND
   `PromptResponse.usage` = {inputTokens: 11605, outputTokens:
   90, totalTokens: 11695} arrives WITH the completion signal —
   r6 finding 5 confirmed: PromptResponse.usage exists and is
   naturally bounded by the completion boundary, making it the
   leading live-source candidate. The agent_stopped extension
   repeats the totals with the model label.
6. **Launch identity** (question 7, partial): session/new's
   response carries CONFIG OPTIONS — `mode` (default
   `accept-edits`; values accept-edits/smart/ask/plan/bypass,
   where "Bypass Permissions" is today's dangerous mode as a
   session mode) and `model` (default `swe-1-7`, full catalog).
   Model and permission grade are SESSION CONFIG, not argv —
   the adapter's launch table must set them through the config
   surface, and the permission-mode mapping (envelope grade →
   session mode) is now a concrete design input.
7. **Wind-down** (question 8, partial): a LATE FRAME
   (session_info_update title) arrived after the PromptResponse.
   Closing stdin did NOT terminate the server — it survived a
   60-second grace window and died only to SIGKILL. Teardown is
   the kill path's job, exactly as the sealed-custody design
   assumes; EOF is not a shutdown contract.

## Consequences for the spec

- The permission story gains a second leg: per-request
  `session/request_permission` answering AND the mode config
  (envelope grade → mode). Step C must establish when requests
  actually fire (mode `ask`?) versus what each mode auto-approves
  server-side.
- The usage owner's leading branch: PromptResponse.usage totals
  (turn-bounded); usage_update as journal evidence.
- Candidate assembly confirmed: exactly agent_message_chunk, in a
  turn that also emitted 23 thought chunks.
