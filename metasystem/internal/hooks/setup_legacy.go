package hooks

// The tails earlier Claude Stop launchers shipped. A live settings file may
// still carry a launcher rendered with one, so setup recognizes each byte for
// byte as this installation's and replaces it. They are matched, never
// printed: their words are the old ones on purpose.

// legacyBlockFallback is the tail the Claude Stop launcher shipped before a
// launcher failure became a degraded allowance. A live settings file may
// still carry a launcher rendered with it; it is recognized as this
// installation's so setup replaces it.
const legacyBlockFallback = `printf '%s\n' '{"decision":"block","reason":"Metasystem Stop hook launcher failed before a safe verdict; stopping is refused."}'`
const legacyDegradedFallback = `printf '%s\n' '{"systemMessage":"Metasystem Stop hook launcher failed in its own infrastructure; stopping is allowed with degraded supervision. Cause: hook-bootstrap-failed. Component: hook-launcher. The steward owns repair."}'`

// legacyStartEngineMissingNotice is the SessionStart launcher's answer with
// no engine installed before it took the two-line wording.
const legacyStartEngineMissingNotice = `{"systemMessage":"Metasystem engine missing: this session received no role context; if this checkout is a declared brain it is uninstructed until the engine is rebuilt: run go run ./cmd/devgate build in the metasystem installation, then start a new session"}`
