# stop-response-delivery-moves-to-a-parent: build design (fixture)

This page is a test fixture for the moved-effects check in the design critic packet. It moves an owner and carries no moved-effects inventory.

## 1. Grounding

Today the supervision hook's Stop worker composes the Stop response and, in `emit_stop_payload` (`metasystem/scripts/agents/supervision-hook.sh`), stages the response bytes in a temporary file, writes them to the provider's standard output, measures the elapsed seconds, appends `<timestamp> stop response decision=<decision> elapsed=<n>s` to the supervision hooks log, and removes the temporary file.

## 2. Change

A deadline parent process becomes the only writer of the provider's standard output. The worker publishes its composed response to the parent over a private pipe and exits; the parent delivers the bytes before the runtime's cutoff. `emit_stop_payload` returns as soon as the response is published to the parent.

## 3. Fixtures

- A Stop whose worker publishes within the cutoff delivers the worker's response byte for byte.
- A Stop whose worker publishes nothing delivers the fixed degraded allowance before the cutoff.
