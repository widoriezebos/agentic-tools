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
