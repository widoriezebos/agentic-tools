# Agent adapters: adding an agent, overriding a built-in

MetaSystem ships four runtimes in Go. Each is a built-in you can override:

| Runtime | Built-in operations | Reserved lookalike (no other declaration may claim it) |
|---|---|---|
| `claude` | `internal/adapter/supervisor/claude.go` | `metasystem-claude-lookalike` |
| `codex` | `internal/adapter/supervisor/codex.go` | `metasystem-codex-lookalike` |
| `devin` | `internal/adapter/supervisor/devin.go` (legacy and ACP transports) | `devin acp`, the host CLI's own helper |
| `fake` | `internal/adapter/supervisor/fake.go` (fixtures only) | `metasystem-fake-lookalike` |

Two things need no Go change:

- **A new agent.** Install an executable at `<installation>/adapters/<name>`
  and name it in the configuration. The executable can be written in any
  language.
- **An override of a built-in.** Install an executable with the built-in's
  name. It implements only the operations it changes. For every other
  operation it exits 64, and the built-in performs that operation.

The shared layer stays in Go and is the same for every runtime: custody,
deadlines and kill domains, the permission-envelope comparison and refusal,
handshake bookkeeping, job records, return normalization, turn adjudication,
and the janitor's kill decision. An adapter only answers the operations below.
The shared layer judges and records everything an adapter reports.

## Installing and trusting an adapter

An external adapter is trusted code that a person installs. It runs only when
all of the following hold:

1. The configuration names it. Add `adapters.<name>.use=external` to
   `metasystem.conf`, or to `metasystem.conf.local` on one machine. The
   environment never names an adapter.
2. The file is owned by the installation's user.
3. The file is not group-writable or world-writable.
4. The name is a runtime name: a lowercase letter followed by up to 31
   lowercase letters, digits or dashes.

Discovery never executes a file that fails these checks. The refusal message
says what to fix, for example `fix it with: chmod go-w <path>` or
`add adapters.<name>.use=external to metasystem.conf`.

To use a new runtime, add its name to `metasystem.runtimes` and set the roles
and models as for any runtime. `settings check` accepts the name only when the
adapter is named and its file is safe. `settings show` and `system check` list
every external adapter and override, and every refused executable with the
reason.

## The registry: one declaration per runtime

The registry holds the built-ins plus every accepted external adapter, with
one effective declaration per runtime name. Every consumer reads it:
configuration validation, dispatch, probes, self-tests, the mission runner,
wait delivery, and the process recognizers (census, lease classification,
human authority, and the janitor's shapes).

Each `describe` declares a classification signature (`match` and `exclude`
patterns) and two vectors:

- `positive`: an argv that the signature must claim.
- `lookalike`: an argv that it must leave alone.

The registry refuses a declaration in any of these cases:

- its signature claims another runtime's positive vector;
- its signature claims another runtime's reserved lookalike vector;
- another runtime's signature already claims its own positive vector;
- it does not claim its own positive vector;
- it claims its own lookalike vector.

These checks prevent one runtime from classifying another runtime's
processes. For example, no adapter may claim Devin's `devin acp` helper.

An override of a built-in yields one effective declaration under the
built-in's name. The signature is the override's own, but the built-in's
exclusions are always added. The built-in's fallback operations rely on those
exclusions. The registry refuses an override whose own signature claims one of
the built-in's reserved lookalike vectors. For example, a Devin override must
keep excluding `devin acp`. Matching the built-in's own positive vector is not
a conflict.

For Devin this matters in practice. The Devin host CLI starts a raw
`devin acp` helper between the announced main session and every tool shell,
and the built-in excludes it, so a host's tool calls classify as the main
session and not as a delegate. A Devin override that answers `describe` must
therefore keep an exclusion for `devin acp`, for example:

```json
{"schemaVersion": 1, "name": "devin",
 "match": ["^([^[:space:]]*/)?devin([[:space:]]|$)", "^([^[:space:]]*/)?devin-delegate-acp([[:space:]]|$)"],
 "exclude": ["^([^[:space:]]*/)?devin[[:space:]]+acp([[:space:]]|$)"]}
```

The same holds for `claude` and `codex` with their reserved lookalikes. An
override that exits 64 on `describe` keeps the built-in's declaration
unchanged.

After a refused override, recognition keeps the built-in's declaration.
Running that runtime is refused, with the reason, until the file is fixed.
Every external declaration also gets the shared exclusions for the supervision
hook and the engine's supervisor process.

## Calling convention

For each operation the engine runs:

    <installation>/adapters/<name> OPERATION

- **stdin** receives one JSON request. It always has `schemaVersion` (1),
  `operation`, `runtime` (the name), and `root` (the installation root).
- **stdout** must contain the JSON response, which echoes
  `"schemaVersion": 1`. The exception is `observe`, which writes one JSON event
  per line.
- **Exit 0** means success.
- **Exit 64** is reserved. An override hands the operation to its built-in. A
  new runtime uses it to say that it does not implement the operation.
- **Any other exit status** fails the operation. Stderr is kept as the reason.

Each call is bounded by `exec.local-timeout-sec`. `describe` is memoized per
engine process for as long as the file stays unchanged. `observe` runs about
every 20 ms while the CLI runs and has not completed its handshake, so it must
be cheap.

An override is asked once per process for each operation. After an operation
exits 64, the built-in answers it for the rest of that process.

The adapter itself writes any files it needs, such as settings files and
callback channels, under the turn directory.

### Per-turn context: `turn`

`prepare`, `observe`, `finalize` and `repair` receive the turn in a `turn`
object:

| Field | Meaning |
|---|---|
| `role` | `delegate` (a dispatched job's round) or `host` (a mission's host turn) |
| `verb` | `dispatch` or `follow-up` for a delegate, `start-turn` for a host |
| `runtime`, `root` | the runtime name and the installation root |
| `workspace` | the directory the CLI runs in |
| `dir` | the round (delegate) or turn (host) directory; write your files here |
| `record` | the job record (delegate) or turn record (host), read-only |
| `job`, `tag`, `round`, `rootJob` | a delegate round's job, claim tag, round number and chain root |
| `mission`, `turnId`, `result` | a host turn's mission, turn and result envelope path |
| `prompt`, `schema` | the assembled prompt file and the return schema |
| `model` | the requested model |
| `resumeSession` | the session to resume, empty for a fresh turn |
| `requested`, `effective` | the requested envelope and the effective-envelope file |
| `events` | the round's events stream (delegate) |
| `handshakeDone`, `sessionId` | the shared handshake state so far |

`observe`, `finalize` and `repair` also receive `launch`, which holds
prepare's `argv`, `stdin` and `stdout`. When the adapter's own prepare made the
launch, they also receive `private`, the opaque value that prepare returned.

## Operations

### `describe`

Request: nothing beyond the common fields. Response:

```json
{
  "schemaVersion": 1,
  "name": "newagent",
  "cli": "newagent",
  "capabilities": {"resume": true, "followUp": true, "repair": false,
                   "waitDelivery": true, "host": true, "usage": "native"},
  "match": ["^([^[:space:]]*/)?newagent([[:space:]]|$)"],
  "exclude": ["newagent-helper"],
  "positive": "newagent -p task",
  "lookalike": "newagent-helper serve",
  "invocations": [{"includes": ["newagent", "-p"], "tagFlag": "--tag"}],
  "configPaths": [".newagent/config.json"],
  "enforcement": {"writeRoots": "mapped", "readRoots": "mapped", "network": "notEnforced"},
  "outputStream": "events.jsonl",
  "selftest": {"turnCeilingSec": 240, "denialEndsTurn": false,
               "probe": {"name": "tools", "behaviorLabels": ["tool-use"]}}
}
```

- `name` must equal the file name.
- The patterns use RE2 syntax, and POSIX classes such as `[[:space:]]` are
  allowed. An argv belongs to the runtime when some `match` pattern and no
  `exclude` pattern matches it.
- `cli` is the executable a host turn needs installed. Leave it empty when
  there is none.
- `capabilities.usage` is `native`, `metered` or `unavailable`.
- `invocations` are claim-bound invocation shapes. Each one lists the argv
  words the CLI carries and the flag whose value is the claim tag. `tagPrefix`
  marks a structured value (`key=TAG`). `tagPathBase` means the tag is the base
  name of a path value. The janitor uses these shapes to prove that an orphaned
  CLI belongs to a dead claim. A shape only says where the tag sits; it never
  authorizes a signal by itself.
- `enforcement` is needed for probes and contract snapshots. Each field is
  `mapped` or `notEnforced`.
- `outputStream` is the round-relative file that the CLI's stdout goes to.
  Prepare can override it for a single turn.
- An override may exit 64 on `describe`. It then keeps the built-in's
  declaration. If an override answers without `capabilities`, the built-in's
  capabilities apply.

### `probe`

Request: `stage` (`identity` or `snapshot`) and `args` (the probe verb's extra
arguments). Response:

```json
{
  "schemaVersion": 1,
  "installed": true,
  "version": "1.4.2",
  "configIdentity": {"cliVersion": "1.4.2", "configHash": "...", "configKeyHashes": {}, "runtime": "newagent"},
  "authenticated": true,
  "authFailure": "newagent is not logged in; run newagent login",
  "transports": ["stdin"],
  "capabilities": {"resume": true, "sessionEstablishedSignal": true, "sessionEstablishedTimeoutSec": 30},
  "permissions": {"unverified": []}
}
```

For `identity`, only `installed` and `configIdentity` matter. `configIdentity`
must change whenever the CLI's version or its configuration changes, because
dispatch selects capability snapshots by it.

For `snapshot`, the engine writes the capability snapshot from the response.
`"authenticated": false` refuses the probe with `authFailure`; this is where
the runtime's own permission and authentication check belongs.
`"installed": false` means the CLI is not installed.

### `selftest`

`selftest` is called only when `describe` declares `selftest.probe`. The
shared self-test dispatches real jobs through this runtime and runs the probe's
stages:

| `stage` | Request fields | Response |
|---|---|---|
| `prepare-scratch` | `probe`, `scratch`, `nonce` | exit 0 after preparing fixtures in the scratch repository |
| `prompt-text` | `probe`, `nonce` | `{"schemaVersion": 1, "text": "..."}`, extra goal instructions |
| `verify-evidence` | `probe`, `returnPath`, `nonce` | exit 0 when the returned evidence proves the probe |

When `verify-evidence` passes, the pass record earns the declared behavior
labels. An override that declares no `selftest` runs the built-in's self-test.

### `prepare`

Request: `turn`. Response:

```json
{
  "schemaVersion": 1,
  "argv": ["/usr/local/bin/newagent", "-p", "--tag", "TAG", "--model", "m"],
  "argv0": "",
  "env": ["NEWAGENT_HOME=/path"],
  "stdin": "/path/to/prompt.md",
  "stdout": "/path/to/round/events.jsonl",
  "truncateLog": false,
  "effective": {"readRoots": [], "writeRoots": ["/repo/package"], "network": "deny"},
  "private": {"anything": "the adapter wants back"}
}
```

- The engine launches `argv` under custody, in `turn.workspace`. `argv0`, if
  given, renames the process's argv[0]. `env` entries are added to the turn's
  environment.
- Put the claim tag (`turn.tag`) where your `invocations` shape says it sits.
- `effective` is the permission envelope that your command and settings
  actually grant. The shared layer compares it with the request and refuses a
  wider grant before anything starts. Map the request honestly: an exact
  subdirectory grant that your CLI enforces stays exact. If you omit
  `effective`, the request counts as the grant, so omit it only when your CLI
  enforces the request exactly.
- Answer `{"schemaVersion": 1, "earlyRefusal": {"error": "...", "phase": "handshake"}}`
  to end the turn before the envelope comparison. Answer with `refusal` to end
  it after the comparison but before the launch. Both are named failures on the
  record.

### `observe`

`observe` is called while the CLI runs, until it reports a handshake. Request:
`turn`, `launch`, `private`, and `running`. The response is zero or more JSON
events, one per line:

```
{"event": "handshake", "session": "S", "turn": "T", "model": "M"}
{"event": "line", "line": "{\"event\":\"session-established\",...}"}
{"event": "refusal", "error": "handshake_missing_session_id", "phase": "handshake"}
{"event": "record-patch", "patch": "/path/to/patch.json"}
```

No output means nothing has been observed yet. A `handshake` names the session
that the shared layer records, and the shared layer then stops calling
`observe`. A `line` is appended to the round's events stream as evidence.

### `finalize`

Request: `turn`, `launch`, `private`, `status` (the CLI's exit status), and
`usage` (the usage file to write). Response for a delegate round:

```json
{
  "schemaVersion": 1,
  "candidate": "/path/to/round/raw.out",
  "transcript": "",
  "session": "S", "turn": "T", "handshakeModel": "M", "resultModel": "M",
  "handshake": {"session": "S", "turn": "T", "model": "M"},
  "deliveryRepair": false,
  "refusal": null
}
```

- `candidate` is the return that adjudication validates against the schema.
- `session`, `turn` and `resultModel` are the identity the turn observed. The
  shared layer refuses completion when the identity disagrees with the
  handshake.
- `handshake` records a late handshake, for a session that only the finished
  output names.
- `deliveryRepair: true` asks the shared layer for a delivery repair when
  nothing was delivered and the same session can be asked again.
- Write `usage` whenever you can. It is in the shape of the built-ins' usage
  files: `availability`, `inputTokens`, `cachedInputTokens`, `outputTokens`,
  `reasoningTokens`, `cost` and `providerUnits`.

For a host turn, answer the finish fields instead: `hostSession`, `hostRaw`
(the raw reply), `hostReturn`, `hostAccepted`, `hostRequireReply`,
`hostTransport`, and `usage` (the usage file path). Use `hostExit` to end the
turn with a specific exit code. The shared layer writes the result envelope:
completed, failed, or unresumable when no session was reported.

### `repair`

`repair` is called only when `describe` declares `capabilities.repair`.
Request: `turn`, `launch`, `private`, `stage`, `promptFile`, `outputFile`,
`namedPath`, and `usage`. Response:
`{"schemaVersion": 1, "status": 0, "candidate": "", "violation": "", "namedPath": "", "settled": true, "model": "M"}`.

| `stage` | What it does |
|---|---|
| `turn` | Asks the same session again with `promptFile` and writes the reply to `outputFile` |
| `usage` | Recomputes `usage` so that it includes the repair's spend |
| `settle` | Re-certifies session and model from the repair (`settled`, `model`) |
| `named-path` | Names the file a delivery repair asks the session to write (`namedPath`) |
| `delivery` | Runs the delivery repair and returns the collected reply as `candidate`, or `violation` |

The shared layer decides whether a repair may run and pays for it through the
same custody. An adapter never repairs on its own initiative.

### `cancel`

Request: `job`. This is runtime-specific cancellation beyond the shared kill,
such as telling a server to end a session. Exit 64 means there is none. The
shared layer always kills the owned process groups afterwards.

## Overrides: which operations may fall back

A turn's operations must agree about who prepared the launch.

- If the override's `prepare` exits 64, the built-in prepares. The override can
  still answer `observe`, `finalize` or `repair`, without `private`, or exit 64
  on them.
- If the override's `prepare` answers, the launch is the override's. It must
  then answer `observe` and `finalize` itself, because a built-in cannot
  continue a launch it did not prepare. Exit 64 there fails the turn.

`probe`, `describe`, `selftest` and `cancel` fall back independently.

## Example: a partial override of Devin

To pin extra configuration into Devin's configuration identity and keep
everything else built in, answer only `probe` and exit 64 for every other
operation:

```sh
#!/bin/sh
case "$1" in
  probe) exec /usr/local/lib/devin-identity-wrapper ;;  # prints the probe answer
  *) exit 64 ;;
esac
```

Install it as `<installation>/adapters/devin` (mode 0755, owned by you) and set
`adapters.devin.use=external`. Devin's describe, prepare, observe, finalize,
repair (delivery repair), self-test and both transports, legacy and ACP, stay
the built-in's. The engine's test `TestDevinPartialOverrideFallsBackToTheBuiltIn`
runs a whole Devin round this way. Overrides of `claude` and `codex` work the
same way.

## A minimal new agent

A new runtime must implement `describe`, `probe`, `prepare`, `observe` and
`finalize`. `selftest`, `repair` and `cancel` may exit 64. The engine's own
tests include such an agent: `internal/adapter/supervisor/external_test.go`
for delegate rounds and `internal/missionrunner/hostturn/external_host_test.go`
for host turns. In them, a shell CLI prints its session, `observe` reports the
handshake, and `finalize` hands back the return and usage.
