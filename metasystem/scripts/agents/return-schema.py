#!/usr/bin/env python3
"""Materialize the versioned role-return schemas without changing frozen v1 files."""

from __future__ import annotations

import argparse
import json
from pathlib import Path


ROLES = {"behavior-judge", "code-critic", "design-critic", "implementer", "investigator", "verifier"}


def version_two(schema: dict) -> dict:
    value = json.loads(json.dumps(schema))
    value["$comment"] = "metasystem.version=2"
    value["title"] = value.get("title", "Agent return") + " version 2"
    # Every property is listed in `required`, which is both this family's own
    # convention and what a provider's structured output enforces. "Nothing
    # claimed" is expressed by null members, not by an absent object.
    value["required"] = ["schemaVersion", "claimed", *value["required"]]
    properties = value["properties"]
    # `type` alongside the value: a bare `const` is rejected by OpenAI
    # structured output ("In context=('properties', 'schemaVersion'), schema
    # must have a 'type' key").
    properties["schemaVersion"] = {"type": "integer", "enum": [2]}
    # Both members are listed in `required` and typed nullable, the same shape
    # every other object in these schemas uses. An object schema without
    # `required` is rejected outright by OpenAI structured output
    # ("In context=('properties', 'claimed'), 'required' is required to be
    # supplied"), which failed every codex delegate dispatch before the model
    # produced a single token. Null means the agent claimed nothing for that
    # member; the key's absence is not how this schema family says that.
    properties["claimed"] = {
        "type": "object",
        "additionalProperties": False,
        "required": ["sessionId", "model"],
        "properties": {
            "sessionId": {"type": ["string", "null"]},
            "model": {"type": ["string", "null"]},
        },
    }
    properties["sessionId"] = {"type": "string"}
    properties["model"]["properties"]["effective"] = {"type": "string"}
    return value


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--role", choices=sorted(ROLES), required=True)
    parser.add_argument("--version", type=int, choices=[1, 2], required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    source = args.root / "scripts/agents/schemas" / f"{args.role}.schema.json"
    schema = json.loads(source.read_text(encoding="utf-8"))
    if args.version == 2:
        schema = version_two(schema)
    args.output.write_text(json.dumps(schema, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
